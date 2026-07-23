package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"go.uber.org/fx"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

// bootstrapConfig is the first-admin seed, read from the environment in the
// spirit of Keycloak's KEYCLOAK_ADMIN/KEYCLOAK_ADMIN_PASSWORD: when an admin
// email + password are set, Aegis ensures (idempotently, every boot) an admin
// realm, that admin account, a public PKCE OIDC client for Foundry, and any
// declared organizations (owned by the admin, the first one set active).
type bootstrapConfig struct {
	realm            string
	realmDisplayName string
	adminEmail       string
	adminPassword    string
	adminName        string
	clientID         string
	redirectURIs     []string
	orgs             []bootstrapOrg
}

// bootstrapOrg is one entry of AEGIS_BOOTSTRAP_ORGS: a JSON array
// (`[{"name":"Acme","slug":"acme"}]`, what the helm chart renders from its
// structured values).
type bootstrapOrg struct {
	name string
	slug string
}

func newBootstrapConfig() bootstrapConfig {
	return bootstrapConfig{
		realm:            envOrDefault("AEGIS_BOOTSTRAP_REALM", "master"),
		realmDisplayName: envOrDefault("AEGIS_BOOTSTRAP_REALM_DISPLAY_NAME", "Master"),
		adminEmail:       os.Getenv("AEGIS_BOOTSTRAP_ADMIN_EMAIL"),
		adminPassword:    os.Getenv("AEGIS_BOOTSTRAP_ADMIN_PASSWORD"),
		adminName:        envOrDefault("AEGIS_BOOTSTRAP_ADMIN_NAME", "Administrator"),
		clientID:         envOrDefault("AEGIS_BOOTSTRAP_CLIENT_ID", "foundry"),
		redirectURIs:     splitAndTrim(envOrDefault("AEGIS_BOOTSTRAP_CLIENT_REDIRECT_URIS", "http://localhost:8080/auth/callback")),
	}
}

// parseBootstrapOrgs parses the JSON array of {name, slug}. Slug is required;
// name defaults to the slug.
func parseBootstrapOrgs(raw string) ([]bootstrapOrg, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var entries []struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return nil, fmt.Errorf("AEGIS_BOOTSTRAP_ORGS: %w", err)
	}
	out := make([]bootstrapOrg, 0, len(entries))
	for _, e := range entries {
		slug := strings.TrimSpace(e.Slug)
		if slug == "" {
			return nil, fmt.Errorf("AEGIS_BOOTSTRAP_ORGS: entry %q has no slug", e.Name)
		}
		name := strings.TrimSpace(e.Name)
		if name == "" {
			name = slug
		}
		out = append(out, bootstrapOrg{name: name, slug: slug})
	}
	return out, nil
}

// enabled gates the whole bootstrap on admin credentials being present, so a
// deployment that doesn't want it simply leaves them unset.
func (c bootstrapConfig) enabled() bool {
	return c.adminEmail != "" && c.adminPassword != ""
}

func splitAndTrim(csv string) []string {
	var out []string
	for _, p := range strings.Split(csv, ",") {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// registerBootstrap runs the idempotent admin seed once on startup. Runtime
// seed failures are logged, not fatal — a transient error shouldn't crash-loop
// the server, and the ensure retries cleanly on the next boot. A malformed
// declaration, by contrast, is a config bug and fails the boot.
func registerBootstrap(lc fx.Lifecycle, realms app.RealmUsecase, clients app.ClientUsecase, authx app.AuthxUsecase, orgs app.OrganizationUsecase) error {
	cfg := newBootstrapConfig()
	if !cfg.enabled() {
		return nil
	}
	// A malformed org declaration is a config bug, not a transient failure —
	// fail the boot instead of seeding a partial deployment.
	seed, err := parseBootstrapOrgs(os.Getenv("AEGIS_BOOTSTRAP_ORGS"))
	if err != nil {
		return err
	}
	cfg.orgs = seed
	log := logger.New()
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			if err := ensureBootstrap(context.Background(), cfg, realms, clients, authx, orgs, log); err != nil {
				log.Error("bootstrap seed failed", "error", err)
			}
			return nil
		},
	})
	return nil
}

func ensureBootstrap(ctx context.Context, cfg bootstrapConfig, realms app.RealmUsecase, clients app.ClientUsecase, authx app.AuthxUsecase, orgs app.OrganizationUsecase, log logger.Logger) error {
	realmID, err := ensureRealm(ctx, cfg, realms, log)
	if err != nil {
		return err
	}
	adminID, err := ensureAdmin(ctx, cfg, realmID, authx, log)
	if err != nil {
		return err
	}
	if err := ensureClient(ctx, cfg, realmID, clients, log); err != nil {
		return err
	}
	return ensureOrgs(ctx, cfg, realmID, adminID, orgs, log)
}

func ensureRealm(ctx context.Context, cfg bootstrapConfig, realms app.RealmUsecase, log logger.Logger) (string, error) {
	if existing, err := realms.Get(ctx, app.RealmByName(cfg.realm)); err == nil {
		return existing.ID(), nil
	} else if !apierrors.Is(err, apierrors.CodeNotFound) {
		return "", err
	}
	created, err := realms.Create(ctx, domain.NewRealm(cfg.realm, domain.WithRealmDisplayName(cfg.realmDisplayName)))
	if err != nil {
		return "", err
	}
	log.Info("bootstrap created admin realm", "realm", cfg.realm, "realmId", created.ID())
	return created.ID(), nil
}

func ensureAdmin(ctx context.Context, cfg bootstrapConfig, realmID string, authx app.AuthxUsecase, log logger.Logger) (string, error) {
	acct, err := authx.Register(ctx, app.RegisterInput{
		RealmID:     realmID,
		Email:       cfg.adminEmail,
		Password:    cfg.adminPassword,
		DisplayName: cfg.adminName,
	})
	if err == nil {
		log.Info("bootstrap created admin account", "email", cfg.adminEmail, "realm", cfg.realm)
		return acct.ID(), nil
	}
	if apierrors.Is(err, apierrors.CodeAlreadyExists) {
		// The account exists from an earlier boot; log in with the bootstrap
		// credentials to recover its id (needed to own the seeded orgs).
		acct, err := authx.Login(ctx, app.LoginInput{RealmID: realmID, Email: cfg.adminEmail, Password: cfg.adminPassword})
		if err != nil {
			return "", err
		}
		return acct.ID(), nil
	}
	return "", err
}

func ensureClient(ctx context.Context, cfg bootstrapConfig, realmID string, clients app.ClientUsecase, log logger.Logger) error {
	if _, err := clients.Get(ctx, app.ClientByRealmAndClientID(realmID, cfg.clientID)); err == nil {
		return nil
	} else if !apierrors.Is(err, apierrors.CodeNotFound) {
		return err
	}
	_, err := clients.Create(ctx, domain.NewClient(realmID, cfg.clientID, domain.ClientTypePublic, cfg.clientID,
		domain.WithClientGrantTypes([]string{"authorization_code", "refresh_token"}),
		domain.WithClientScopes([]string{"openid", "profile", "email"}),
		domain.WithClientRedirectURIs(cfg.redirectURIs),
		domain.WithClientPKCERequired(true),
	))
	if err != nil {
		return err
	}
	log.Info("bootstrap registered OIDC client", "clientId", cfg.clientID, "realm", cfg.realm, "redirectUris", cfg.redirectURIs)
	return nil
}

// ensureOrgs seeds the declared organizations with the admin as owner. The
// first org is set as the admin's active org when none is stored yet, so a
// fresh deployment logs in with a workspace already selected.
func ensureOrgs(ctx context.Context, cfg bootstrapConfig, realmID, adminID string, orgs app.OrganizationUsecase, log logger.Logger) error {
	for i, o := range cfg.orgs {
		id, err := ensureOrg(ctx, orgs, realmID, adminID, o, log)
		if err != nil {
			return err
		}
		if i == 0 {
			if err := ensureActiveOrg(ctx, orgs, adminID, id); err != nil {
				return err
			}
		}
	}
	return nil
}

func ensureOrg(ctx context.Context, orgs app.OrganizationUsecase, realmID, adminID string, o bootstrapOrg, log logger.Logger) (string, error) {
	res, err := orgs.List(ctx, search.WithQueryOpts(
		query.FilterBy(filter.OpEq, fields.RealmID, realmID),
		query.FilterBy(filter.OpEq, fields.Slug, o.slug),
	))
	if err != nil {
		return "", err
	}
	if existing := res.Results(); len(existing) > 0 {
		return existing[0].ID(), nil
	}
	created, err := orgs.Create(ctx, domain.NewOrganization(realmID, o.name, o.slug,
		domain.WithOrganizationOwnerID(adminID),
	))
	if err != nil {
		return "", err
	}
	log.Info("bootstrap created organization", "slug", o.slug, "orgId", created.ID(), "realmId", realmID)
	return created.ID(), nil
}

// ensureActiveOrg sets the admin's active org, unless one is already resolved
// (stored, or a sole membership) — a manual workspace switch survives reboots.
func ensureActiveOrg(ctx context.Context, orgs app.OrganizationUsecase, adminID, orgID string) error {
	active, _, err := orgs.ActiveOrg(ctx, adminID)
	if err != nil {
		return err
	}
	if active != "" {
		return nil
	}
	return orgs.Activate(ctx, adminID, orgID)
}

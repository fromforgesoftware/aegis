package internal

import (
	"context"
	"os"
	"strings"

	"go.uber.org/fx"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/monitoring/logger"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

// bootstrapConfig is the first-admin seed, read from the environment in the
// spirit of Keycloak's KEYCLOAK_ADMIN/KEYCLOAK_ADMIN_PASSWORD: when an admin
// email + password are set, Aegis ensures (idempotently, every boot) an admin
// realm, that admin account, and a public PKCE OIDC client for Foundry.
type bootstrapConfig struct {
	realm            string
	realmDisplayName string
	adminEmail       string
	adminPassword    string
	adminName        string
	clientID         string
	redirectURIs     []string
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

// registerBootstrap runs the idempotent admin seed once on startup. Failures
// are logged, not fatal: a transient error shouldn't crash-loop the server, and
// the ensure retries cleanly on the next boot.
func registerBootstrap(lc fx.Lifecycle, realms app.RealmUsecase, clients app.ClientUsecase, authx app.AuthxUsecase) {
	cfg := newBootstrapConfig()
	if !cfg.enabled() {
		return
	}
	log := logger.New()
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			if err := ensureBootstrap(context.Background(), cfg, realms, clients, authx, log); err != nil {
				log.Error("bootstrap seed failed", "error", err)
			}
			return nil
		},
	})
}

func ensureBootstrap(ctx context.Context, cfg bootstrapConfig, realms app.RealmUsecase, clients app.ClientUsecase, authx app.AuthxUsecase, log logger.Logger) error {
	realmID, err := ensureRealm(ctx, cfg, realms, log)
	if err != nil {
		return err
	}
	if err := ensureAdmin(ctx, cfg, realmID, authx, log); err != nil {
		return err
	}
	return ensureClient(ctx, cfg, realmID, clients, log)
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

func ensureAdmin(ctx context.Context, cfg bootstrapConfig, realmID string, authx app.AuthxUsecase, log logger.Logger) error {
	_, err := authx.Register(ctx, app.RegisterInput{
		RealmID:     realmID,
		Email:       cfg.adminEmail,
		Password:    cfg.adminPassword,
		DisplayName: cfg.adminName,
	})
	if err == nil {
		log.Info("bootstrap created admin account", "email", cfg.adminEmail, "realm", cfg.realm)
		return nil
	}
	if apierrors.Is(err, apierrors.CodeAlreadyExists) {
		return nil
	}
	return err
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

package http

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/auth"
	"github.com/fromforgesoftware/go-kit/auth/jwt"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/jsonapi"
	"github.com/fromforgesoftware/go-kit/openapi"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/fields"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

// OrganizationController exposes /api/organizations as JSON:API plus a
// /api/organizations/{id}/members sub-resource backed by authz bindings.
type OrganizationController struct {
	orgs   app.OrganizationUsecase
	realms app.RealmUsecase
	tokens app.TokenIssuer
	// gateway validates the shared-secret token a trusted SERVICE presents. Nil when
	// FORGE_GATEWAY_SECRET is unset, which is what stops a deployment that has configured no service
	// identity from acquiring one by accident.
	gateway jwt.Validator
}

func NewOrganizationController(orgs app.OrganizationUsecase, realms app.RealmUsecase, tokens app.TokenIssuer) kitrest.Controller {
	c := &OrganizationController{orgs: orgs, realms: realms, tokens: tokens}
	// The same secret the gateway middleware gates /api with, read the same way. A malformed one is
	// treated as absent rather than fatal: that middleware would already be refusing every /api
	// request, and failing startup here would turn a misconfiguration into an outage.
	if secret := os.Getenv("FORGE_GATEWAY_SECRET"); secret != "" {
		if validator, err := jwt.NewHMACIssuer(secret); err == nil {
			c.gateway = validator
		}
	}
	return c
}

func (c *OrganizationController) Routes(r kitrest.Router) {
	r.Route("/api/organizations", func(r kitrest.Router) {
		// Self-service: a realm end-user creates their own org. Authenticated
		// directly by the caller's realm token (not the forge gateway).
		r.Post("", c.requireRealmToken(kitrest.NewJsonApiCommandHandler(
			c.create, decodeOrganization, api.OrganizationToDTO,
			kitrest.HandlerWithOpenAPI(
				openapi.Summary("Create an organization"),
				openapi.Description("The caller becomes the organization owner unless an owner relationship is supplied."),
				openapi.Tags("tenancy"), openapi.Errors(400, 401, 409),
			),
		)))
		// Listing the collection has two legitimate callers, and answers each differently.
		//
		// A realm END-USER gets their own memberships. This route was once anonymous, which returned
		// every tenant in every realm — name, slug, owner account id and realm id — to anyone who
		// could reach it.
		//
		// A trusted SERVICE holding FORGE_GATEWAY_SECRET gets the whole collection, filtered by the
		// query. That is not a reopening of the above: the secret is a deployment credential, it is
		// the identity every service-to-service call in the platform already presents, and the
		// gateway middleware gates this same /api surface with it. Closing the anonymous path left
		// back-office callers — a seeder, a migration tool — with no door at all, because they have
		// no user to borrow a token from. The answer to that is a service identity, not a public
		// endpoint.
		r.Get("", c.requireRealmTokenOrService(
			http.HandlerFunc(c.listMine), http.HandlerFunc(c.listAll)))
		r.Route("/{id}", func(r kitrest.Router) {
			r.Get("", c.requireOrgMembership(kitrest.NewJsonApiGetHandler(
				c.orgs, api.OrganizationToDTO, []query.ParseOpt{},
				kitrest.HandlerWithOpenAPI(openapi.Summary("Get an organization"), openapi.Tags("tenancy"), openapi.Errors(401, 404)),
			)))
			r.Patch("", c.requireOrgOwner(http.HandlerFunc(c.patch)))
			r.Delete("", c.requireOrgOwner(kitrest.NewJsonApiDeleteHandler(
				c.orgs, repository.DeleteTypeSoft,
				kitrest.HandlerWithOpenAPI(openapi.Summary("Delete an organization"), openapi.Tags("tenancy"), openapi.Errors(401, 403, 404)),
			)))
			r.Post("/activate", c.requireRealmToken(http.HandlerFunc(c.activate))) // self-service
			r.Get("/members", c.requireOrgMembership(http.HandlerFunc(c.listMembers)))
			r.Post("/members", c.requireOrgOwner(http.HandlerFunc(c.addMember)))
			r.Delete("/members/{accountId}", c.requireOrgOwner(http.HandlerFunc(c.removeMember)))
		})
	})
	r.Get("/api/me/organizations", c.requireRealmToken(http.HandlerFunc(c.listMine))) // self-service
}

// requireRealmToken authenticates a realm end-user's bearer token directly
// (validating its signature against the issuing realm's keys) and injects it
// into the context, so the self-service tenancy endpoints (create org, activate,
// me/organizations) work for an SPA calling aegis without the forge gateway.
// The realm is identified by the token's issuer (.../realms/<name>).
// requireRealmTokenOrService routes a request by WHO is asking.
//
// A valid gateway token means a service, which gets `service`. Anything else falls through to the
// realm-token check and gets `user`. The gateway branch is tried first, and only when a secret is
// configured — so a deployment without one behaves exactly as it did before this existed.
func (c *OrganizationController) requireRealmTokenOrService(user, service http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.gateway != nil {
			if raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "); raw != "" {
				if _, err := c.gateway.Validate(r.Context(), raw); err == nil {
					service.ServeHTTP(w, r)
					return
				}
			}
		}
		c.requireRealmToken(user).ServeHTTP(w, r)
	})
}

// listAll answers a SERVICE with the whole collection, honouring the request's filters.
//
// Unscoped because a service has no memberships to scope to: it acts for the platform rather than
// for a person, which is precisely why it had to prove it holds the deployment secret to get here.
func (c *OrganizationController) listAll(w http.ResponseWriter, r *http.Request) {
	opts, err := query.ParseOptsFromHTTPReq(r)
	if err != nil {
		writeAPIError(r.Context(), w, err)
		return
	}
	found, err := c.orgs.List(r.Context(), search.WithQueryOpts(opts...))
	if err != nil {
		writeAPIError(r.Context(), w, err)
		return
	}
	list := resource.ListResponseToDTO(api.OrganizationToDTO)(found)
	w.Header().Set("Content-Type", "application/vnd.api+json")
	_ = jsonapi.MarshalManyPayloads(w, list)
}

func (c *OrganizationController) requireRealmToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			writeAPIError(r.Context(), w, apierrors.Unauthorized("authentication required"))
			return
		}
		raw := strings.TrimPrefix(h, "Bearer ")
		tok, err := auth.NewToken(raw, auth.TokenType("Bearer"), nil)
		if err != nil {
			writeAPIError(r.Context(), w, apierrors.Unauthorized("invalid token"))
			return
		}
		name := realmNameFromIssuer(tok.Claims().Get("iss"))
		if name == "" {
			writeAPIError(r.Context(), w, apierrors.Unauthorized("token is not realm-scoped"))
			return
		}
		realm, err := c.realms.Get(r.Context(), app.RealmByName(name))
		if err != nil || realm == nil {
			writeAPIError(r.Context(), w, apierrors.Unauthorized("unknown realm"))
			return
		}
		if _, err := c.tokens.VerifyAccessToken(r.Context(), realm.ID(), raw); err != nil {
			writeAPIError(r.Context(), w, apierrors.Unauthorized("invalid token"))
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.InjectTokenInCtx(r.Context(), tok)))
	})
}

// requireOrgOwner authenticates the caller and requires it to OWN the {id} organization.
//
// It gates every write: rename, re-slug, delete, and membership changes. The owner rule is not
// invented here — it is the same rule the avatar controller already applies to the organization
// logo, and a workspace whose name any member could change but whose logo only the owner could
// would be incoherent.
//
// A missing organization is reported as 404 before ownership is considered, matching the avatar
// controller. That does leak existence to an authenticated caller, which is a deliberate trade:
// the alternative is answering 403 for ids that do not exist, which makes a genuine typo
// indistinguishable from a permission problem.
func (c *OrganizationController) requireOrgOwner(next http.Handler) http.Handler {
	return c.requireRealmToken(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := auth.TokenFromCtx(r.Context())
		if tok == nil {
			writeAPIError(r.Context(), w, apierrors.Unauthorized("authentication required"))
			return
		}
		orgID := r.PathValue("id")
		org, err := c.orgs.Get(r.Context(),
			search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.ID, orgID)))
		if err != nil || org == nil {
			writeAPIError(r.Context(), w, apierrors.NotFound("organization", orgID))
			return
		}
		owner := org.Owner()
		if owner == nil || owner.ID() != tok.Claims().Subject() {
			writeAPIError(r.Context(), w, apierrors.Forbidden("only the workspace owner can change this workspace"))
			return
		}
		next.ServeHTTP(w, r)
	}))
}

// requireOrgMembership authenticates the caller and requires it to BELONG to the {id}
// organization, which the owner necessarily does.
//
// It gates the reads of a single organization and its member list. Membership is answered by
// asking which organizations the caller belongs to rather than by a dedicated lookup: that is the
// same question /api/me/organizations answers, so there is one implementation of "is this account
// in this workspace" and no second one to disagree with it.
func (c *OrganizationController) requireOrgMembership(next http.Handler) http.Handler {
	return c.requireRealmToken(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := auth.TokenFromCtx(r.Context())
		if tok == nil {
			writeAPIError(r.Context(), w, apierrors.Unauthorized("authentication required"))
			return
		}
		orgID := r.PathValue("id")
		mine, err := c.orgs.ListForAccount(r.Context(), tok.Claims().Subject())
		if err != nil {
			writeAPIError(r.Context(), w, err)
			return
		}
		for _, org := range mine {
			if org.ID() == orgID {
				next.ServeHTTP(w, r)
				return
			}
		}
		// 404 rather than 403: to someone who is not a member, a workspace they cannot see should
		// not be confirmed to exist. Unlike the owner check above there is no typo to diagnose —
		// the caller was never going to be allowed to read it either way.
		writeAPIError(r.Context(), w, apierrors.NotFound("organization", orgID))
	}))
}

func patchOrganization(id string, dto *api.OrganizationDTO) []repository.PatchOption {
	opts := []repository.PatchOption{
		repository.PatchSearchOpts(search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.ID, id))),
	}
	if dto.RName != "" {
		opts = append(opts, repository.PatchField(fields.Name, dto.RName))
	}
	if dto.RSlug != "" {
		opts = append(opts, repository.PatchField(fields.Slug, dto.RSlug))
	}
	if dto.RStatus != "" {
		opts = append(opts, repository.PatchField(fields.Status, dto.RStatus))
	}
	return opts
}

// maxOrgNameLen bounds a workspace name: long enough for any real one, short enough that the
// column is not somewhere to store a paragraph.
const maxOrgNameLen = 128

// orgSlugPattern is a DNS-label-shaped slug: lowercase alphanumerics separated by single hyphens.
//
// A slug is an identifier, not a label — it is half of UNIQUE (realm_id, slug) and the natural
// key for any future /w/{slug} route. Accepting "My Workspace" would put a space in a URL and let
// two visually distinct slugs normalise to one.
var orgSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// validateOrgPatch rejects what the columns and the URL grammar cannot represent.
//
// The DTO's omitempty means an absent field arrives as "", which patchOrganization already treats
// as "leave alone" — so each rule applies only to a field actually being set.
func validateOrgPatch(dto *api.OrganizationDTO) error {
	if dto.RName != "" {
		name := strings.TrimSpace(dto.RName)
		if name == "" {
			return apierrors.InvalidArgument("name cannot be blank")
		}
		if len(name) > maxOrgNameLen {
			return apierrors.InvalidArgument("name is longer than 128 characters")
		}
		// Control characters reach a UI as broken markup or an injected header line, and no real
		// workspace name contains one. Same rule as an account's name parts.
		for _, ch := range name {
			if ch < 0x20 || ch == 0x7f {
				return apierrors.InvalidArgument("name contains a control character")
			}
		}
		dto.RName = name
	}
	if dto.RSlug != "" {
		if len(dto.RSlug) > 63 {
			return apierrors.InvalidArgument("slug is longer than 63 characters")
		}
		if !orgSlugPattern.MatchString(dto.RSlug) {
			return apierrors.InvalidArgument(
				"slug must be lowercase letters, digits and single hyphens, e.g. acme-trading")
		}
	}
	if dto.RStatus != "" && !domain.OrgStatus(dto.RStatus).Valid() {
		// Without this any string reaches the status column, and a typo like "ACITVE" would
		// silently produce a workspace in a state nothing knows how to interpret.
		return apierrors.InvalidArgument("status must be ACTIVE, SUSPENDED or ARCHIVED")
	}
	return nil
}

func (c *OrganizationController) patch(w http.ResponseWriter, r *http.Request) {
	dto, err := kitrest.UnmarshalPayloadFromRequest[*api.OrganizationDTO](r)
	if err != nil {
		writeAPIError(r.Context(), w, apierrors.InvalidArgument("invalid request body"))
		return
	}
	if err := validateOrgPatch(dto); err != nil {
		writeAPIError(r.Context(), w, err)
		return
	}
	patched, err := c.orgs.Patch(r.Context(), patchOrganization(r.PathValue("id"), dto)...)
	if err != nil {
		writeAPIError(r.Context(), w, err)
		return
	}
	if len(patched) == 0 {
		writeAPIError(r.Context(), w, apierrors.NotFound("organization", r.PathValue("id")))
		return
	}
	w.Header().Set("Content-Type", "application/vnd.api+json")
	_ = jsonapi.MarshalPayload(w, api.OrganizationToDTO(patched[0]))
}

func (c *OrganizationController) activate(w http.ResponseWriter, r *http.Request) {
	tok := auth.TokenFromCtx(r.Context())
	if tok == nil {
		writeAPIError(r.Context(), w, apierrors.Unauthorized("authentication required"))
		return
	}
	if err := c.orgs.Activate(r.Context(), tok.Claims().Subject(), r.PathValue("id")); err != nil {
		writeAPIError(r.Context(), w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *OrganizationController) listMine(w http.ResponseWriter, r *http.Request) {
	tok := auth.TokenFromCtx(r.Context())
	if tok == nil {
		writeAPIError(r.Context(), w, apierrors.Unauthorized("authentication required"))
		return
	}
	orgs, err := c.orgs.ListForAccount(r.Context(), tok.Claims().Subject())
	if err != nil {
		writeAPIError(r.Context(), w, err)
		return
	}
	list := resource.ListResponseToDTO(api.OrganizationToDTO)(resource.NewListResponse(orgs, len(orgs)))
	w.Header().Set("Content-Type", "application/vnd.api+json")
	_ = jsonapi.MarshalManyPayloads(w, list)
}

func (c *OrganizationController) create(ctx context.Context, org domain.Organization) (domain.Organization, error) {
	tok := auth.TokenFromCtx(ctx)

	realmID := ""
	if r := org.Realm(); r != nil {
		realmID = r.ID()
	}
	// A realm-scoped end-user token identifies its realm by issuer
	// (.../realms/<name>), so an SPA can create an org with just {name, slug}:
	// infer the realm from the caller's token when the client didn't supply a
	// realmId. Mirrors the owner-from-token inference below.
	if realmID == "" && tok != nil {
		if name := realmNameFromIssuer(tok.Claims().Get("iss")); name != "" {
			if realm, err := c.realms.Get(ctx, app.RealmByName(name)); err == nil && realm != nil {
				realmID = realm.ID()
			}
		}
	}

	ownerID := ""
	if o := org.Owner(); o != nil {
		ownerID = o.ID()
	} else if tok != nil {
		ownerID = tok.Claims().Subject()
	}

	// settings is a NOT NULL jsonb column; default to {} so a client that omits
	// it (e.g. an SPA creating a workspace) doesn't hit a null-constraint error.
	settings := org.Settings()
	if settings == nil {
		settings = map[string]any{}
	}
	opts := []domain.OrganizationOption{
		domain.WithOrganizationStatus(org.Status()),
		domain.WithOrganizationSettings(settings),
	}
	if ownerID != "" {
		opts = append(opts, domain.WithOrganizationOwnerID(ownerID))
	}
	return c.orgs.Create(ctx, domain.NewOrganization(realmID, org.Name(), org.Slug(), opts...))
}

// realmNameFromIssuer extracts the realm name from an OIDC issuer claim of the
// form ".../realms/<name>". Returns "" when the claim isn't a realm issuer.
func realmNameFromIssuer(iss any) string {
	s, _ := iss.(string)
	const marker = "/realms/"
	i := strings.LastIndex(s, marker)
	if i < 0 {
		return ""
	}
	name := s[i+len(marker):]
	if j := strings.IndexByte(name, '/'); j >= 0 {
		name = name[:j]
	}
	return name
}

func decodeOrganization(req *http.Request) (domain.Organization, error) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.OrganizationDTO](req)
	if err != nil {
		return nil, err
	}
	return api.OrganizationFromDTO(body), nil
}

type addOrgMemberRequest struct {
	AccountID string `json:"accountId"`
	Role      string `json:"role"`
}

func (c *OrganizationController) listMembers(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	members, err := c.orgs.ListMembers(r.Context(), orgID)
	if err != nil {
		writeAPIError(r.Context(), w, err)
		return
	}
	list := resource.ListResponseToDTO(func(b domain.Binding) *api.MembershipDTO {
		return api.MembershipToDTO(b, orgID)
	})(resource.NewListResponse(members, len(members)))
	w.Header().Set("Content-Type", "application/vnd.api+json")
	_ = jsonapi.MarshalManyPayloads(w, list)
}

func (c *OrganizationController) addMember(w http.ResponseWriter, r *http.Request) {
	var body addOrgMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(r.Context(), w, apierrors.InvalidArgument("invalid request body"))
		return
	}
	if body.AccountID == "" || body.Role == "" {
		writeAPIError(r.Context(), w, apierrors.InvalidArgument("accountId and role are required"))
		return
	}
	if err := c.orgs.AddMember(r.Context(), r.PathValue("id"), body.AccountID, body.Role); err != nil {
		writeAPIError(r.Context(), w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *OrganizationController) removeMember(w http.ResponseWriter, r *http.Request) {
	if err := c.orgs.RemoveMember(r.Context(), r.PathValue("id"), r.PathValue("accountId")); err != nil {
		writeAPIError(r.Context(), w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

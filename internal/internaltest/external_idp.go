package internaltest

import (
	"github.com/fromforgesoftware/aegis/internal/domain"
)

type ExternalIDPOption func(*externalIDPOpts)

type externalIDPOpts struct {
	id           string
	realmID      string
	kind         domain.ExternalIDPKind
	name         string
	enabled      *bool
	clientID     string
	discoveryURL string
	issuer       string
	scopes       []string
	config       map[string]string
}

func defaultExternalIDPOptions() []ExternalIDPOption {
	return []ExternalIDPOption{
		WithExternalIDPRealmID("realm-test"),
		WithExternalIDPKind(domain.ExternalIDPKindOIDCCustom),
		WithExternalIDPName("idp-test"),
	}
}

func WithExternalIDPID(id string) ExternalIDPOption {
	return func(o *externalIDPOpts) { o.id = id }
}
func WithExternalIDPRealmID(id string) ExternalIDPOption {
	return func(o *externalIDPOpts) { o.realmID = id }
}
func WithExternalIDPKind(k domain.ExternalIDPKind) ExternalIDPOption {
	return func(o *externalIDPOpts) { o.kind = k }
}
func WithExternalIDPName(n string) ExternalIDPOption {
	return func(o *externalIDPOpts) { o.name = n }
}
func WithExternalIDPEnabled(v bool) ExternalIDPOption {
	return func(o *externalIDPOpts) { o.enabled = &v }
}
func WithExternalIDPClientID(id string) ExternalIDPOption {
	return func(o *externalIDPOpts) { o.clientID = id }
}
func WithExternalIDPDiscoveryURL(u string) ExternalIDPOption {
	return func(o *externalIDPOpts) { o.discoveryURL = u }
}
func WithExternalIDPIssuer(i string) ExternalIDPOption {
	return func(o *externalIDPOpts) { o.issuer = i }
}
func WithExternalIDPScopes(s []string) ExternalIDPOption {
	return func(o *externalIDPOpts) { o.scopes = s }
}
func WithExternalIDPConfig(cfg map[string]string) ExternalIDPOption {
	return func(o *externalIDPOpts) { o.config = cfg }
}

// NewExternalIDP builds an ExternalIDPConfig fixture from defaults overridden by opts.
func NewExternalIDP(opts ...ExternalIDPOption) domain.ExternalIDPConfig {
	o := &externalIDPOpts{}
	for _, opt := range append(defaultExternalIDPOptions(), opts...) {
		opt(o)
	}
	domainOpts := []domain.ExternalIDPConfigOption{
		domain.WithExternalIDPClientID(o.clientID),
		domain.WithExternalIDPDiscoveryURL(o.discoveryURL),
		domain.WithExternalIDPIssuer(o.issuer),
		domain.WithExternalIDPScopes(o.scopes),
		domain.WithExternalIDPConfig(o.config),
	}
	if o.id != "" {
		domainOpts = append(domainOpts, domain.WithExternalIDPID(o.id))
	}
	if o.enabled != nil {
		domainOpts = append(domainOpts, domain.WithExternalIDPEnabled(*o.enabled))
	}
	return domain.NewExternalIDPConfig(o.realmID, o.kind, o.name, domainOpts...)
}

// MatchExternalIDP compares realm + kind + name, ignoring id/timestamps.
func MatchExternalIDP(want domain.ExternalIDPConfig) func(domain.ExternalIDPConfig) bool {
	return func(got domain.ExternalIDPConfig) bool {
		if want == nil {
			return got == nil
		}
		if got == nil {
			return false
		}
		return want.RealmID() == got.RealmID() &&
			want.Kind() == got.Kind() &&
			want.Name() == got.Name()
	}
}

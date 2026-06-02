package internaltest

import (
	"github.com/fromforgesoftware/aegis/internal/domain"
)

type ClientOption func(*clientOpts)

type clientOpts struct {
	realmID      string
	clientID     string
	clientType   domain.ClientType
	name         string
	grantTypes   []string
	scopes       []string
	redirectURIs []string
	pkceRequired *bool
	secretHash   string
}

func defaultClientOptions() []ClientOption {
	return []ClientOption{
		WithClientRealmID("realm-test"),
		WithClientID("client-test"),
		WithClientType(domain.ClientTypePublic),
		WithClientName("Test Client"),
	}
}

func WithClientRealmID(realmID string) ClientOption {
	return func(o *clientOpts) { o.realmID = realmID }
}
func WithClientID(id string) ClientOption {
	return func(o *clientOpts) { o.clientID = id }
}
func WithClientType(t domain.ClientType) ClientOption {
	return func(o *clientOpts) { o.clientType = t }
}
func WithClientName(n string) ClientOption {
	return func(o *clientOpts) { o.name = n }
}
func WithClientGrantTypes(g []string) ClientOption {
	return func(o *clientOpts) { o.grantTypes = g }
}
func WithClientScopes(s []string) ClientOption {
	return func(o *clientOpts) { o.scopes = s }
}
func WithClientRedirectURIs(u []string) ClientOption {
	return func(o *clientOpts) { o.redirectURIs = u }
}
func WithClientPKCERequired(v bool) ClientOption {
	return func(o *clientOpts) { o.pkceRequired = &v }
}
func WithClientSecretHash(h string) ClientOption {
	return func(o *clientOpts) { o.secretHash = h }
}

// NewClient builds a domain.Client fixture from defaults overridden by opts.
func NewClient(opts ...ClientOption) domain.Client {
	o := &clientOpts{}
	for _, opt := range append(defaultClientOptions(), opts...) {
		opt(o)
	}
	domainOpts := []domain.ClientOption{}
	if o.grantTypes != nil {
		domainOpts = append(domainOpts, domain.WithClientGrantTypes(o.grantTypes))
	}
	if o.scopes != nil {
		domainOpts = append(domainOpts, domain.WithClientScopes(o.scopes))
	}
	if o.redirectURIs != nil {
		domainOpts = append(domainOpts, domain.WithClientRedirectURIs(o.redirectURIs))
	}
	if o.pkceRequired != nil {
		domainOpts = append(domainOpts, domain.WithClientPKCERequired(*o.pkceRequired))
	}
	if o.secretHash != "" {
		domainOpts = append(domainOpts, domain.WithClientSecretHash(o.secretHash))
	}
	return domain.NewClient(o.realmID, o.clientID, o.clientType, o.name, domainOpts...)
}

// MatchClient compares the identifying fields (realm, client_id, type, name),
// ignoring the generated id/secret.
func MatchClient(want domain.Client) func(domain.Client) bool {
	return func(got domain.Client) bool {
		if want == nil {
			return got == nil
		}
		if got == nil {
			return false
		}
		return want.RealmID() == got.RealmID() &&
			want.ClientID() == got.ClientID() &&
			want.ClientType() == got.ClientType() &&
			want.Name() == got.Name()
	}
}

package domain

import (
	"time"

	"github.com/fromforgesoftware/go-kit/resource"
)

const ResourceTypeServiceAccount resource.Type = "service-accounts"

// ServiceAccount is a non-human identity: a SERVICE-type account (so it can be
// granted roles like a user) paired with machine credentials. The resource id
// is the account id, so a binding can name it directly as a subject.
type ServiceAccount interface {
	resource.Resource
	RealmID() string
	Name() string
	ClientID() string
	Scopes() []string
	LastUsedAt() *time.Time
}

type serviceAccount struct {
	resource.Resource

	realmID    string
	name       string
	clientID   string
	scopes     []string
	lastUsedAt *time.Time
}

type ServiceAccountOption func(*serviceAccount)

func WithServiceAccountID(id string) ServiceAccountOption {
	return func(s *serviceAccount) { s.Resource = resource.Update(s.Resource, resource.WithID(id)) }
}
func WithServiceAccountScopes(scopes []string) ServiceAccountOption {
	return func(s *serviceAccount) { s.scopes = scopes }
}
func WithServiceAccountLastUsedAt(t *time.Time) ServiceAccountOption {
	return func(s *serviceAccount) { s.lastUsedAt = t }
}

func NewServiceAccount(realmID, name, clientID string, opts ...ServiceAccountOption) ServiceAccount {
	s := &serviceAccount{
		Resource: resource.New(resource.WithType(ResourceTypeServiceAccount)),
		realmID:  realmID,
		name:     name,
		clientID: clientID,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *serviceAccount) RealmID() string        { return s.realmID }
func (s *serviceAccount) Name() string           { return s.name }
func (s *serviceAccount) ClientID() string       { return s.clientID }
func (s *serviceAccount) Scopes() []string       { return s.scopes }
func (s *serviceAccount) LastUsedAt() *time.Time { return s.lastUsedAt }

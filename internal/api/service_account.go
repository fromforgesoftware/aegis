package api

import (
	"time"

	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

const (
	ResourceTypeServiceAccount      resource.Type = "service-accounts"
	ResourceTypeServiceAccountToken resource.Type = "serviceAccountTokens"
)

// ServiceAccountDTO is the read/list shape. The secret is never part of it —
// it's surfaced once, only on create.
type ServiceAccountDTO struct {
	resource.RestDTO

	RRealmID    string     `jsonapi:"attr,realmId"`
	RName       string     `jsonapi:"attr,name"`
	RClientID   string     `jsonapi:"attr,clientId"`
	RScopes     []string   `jsonapi:"attr,scopes,omitempty"`
	RLastUsedAt *time.Time `jsonapi:"attr,lastUsedAt,omitempty"`
	RCreatedAt  time.Time  `jsonapi:"attr,createdAt,omitempty"`
}

func ServiceAccountToDTO(sa domain.ServiceAccount) *ServiceAccountDTO {
	if sa == nil {
		return nil
	}
	dto := &ServiceAccountDTO{
		RestDTO:     resource.ToRestDTO(sa),
		RRealmID:    sa.RealmID(),
		RName:       sa.Name(),
		RClientID:   sa.ClientID(),
		RScopes:     sa.Scopes(),
		RLastUsedAt: sa.LastUsedAt(),
		RCreatedAt:  sa.CreatedAt(),
	}
	dto.RType = ResourceTypeServiceAccount
	return dto
}

// ServiceAccountCreateDTO is the create request.
type ServiceAccountCreateDTO struct {
	resource.RestDTO

	RRealmID string   `jsonapi:"attr,realmId"`
	RName    string   `jsonapi:"attr,name"`
	RScopes  []string `jsonapi:"attr,scopes,omitempty"`
}

// ServiceAccountCredentialsDTO is the one-time create response carrying the raw
// secret.
type ServiceAccountCredentialsDTO struct {
	resource.RestDTO

	RAccountID    string   `jsonapi:"attr,accountId"`
	RRealmID      string   `jsonapi:"attr,realmId"`
	RName         string   `jsonapi:"attr,name"`
	RClientID     string   `jsonapi:"attr,clientId"`
	RClientSecret string   `jsonapi:"attr,clientSecret"`
	RScopes       []string `jsonapi:"attr,scopes,omitempty"`
}

func ServiceAccountCredentialsToDTO(sa domain.ServiceAccount, clientID, clientSecret string) *ServiceAccountCredentialsDTO {
	dto := &ServiceAccountCredentialsDTO{
		RAccountID:    sa.ID(),
		RRealmID:      sa.RealmID(),
		RName:         sa.Name(),
		RClientID:     clientID,
		RClientSecret: clientSecret,
		RScopes:       sa.Scopes(),
	}
	dto.RType = ResourceTypeServiceAccount
	dto.RID = sa.ID()
	return dto
}

// ServiceAccountTokenRequestDTO is the client_credentials request for a service
// account.
type ServiceAccountTokenRequestDTO struct {
	resource.RestDTO

	RRealmID      string `jsonapi:"attr,realmId"`
	RClientID     string `jsonapi:"attr,clientId"`
	RClientSecret string `jsonapi:"attr,clientSecret"`
}

// ServiceAccountTokenDTO is the minted access token.
type ServiceAccountTokenDTO struct {
	resource.RestDTO

	RAccessToken string `jsonapi:"attr,accessToken"`
	RTokenType   string `jsonapi:"attr,tokenType"`
	RExpiresIn   int64  `jsonapi:"attr,expiresIn"`
}

func ServiceAccountTokenToDTO(accessToken, tokenType string, expiresIn int64) *ServiceAccountTokenDTO {
	dto := &ServiceAccountTokenDTO{RAccessToken: accessToken, RTokenType: tokenType, RExpiresIn: expiresIn}
	dto.RType = ResourceTypeServiceAccountToken
	return dto
}

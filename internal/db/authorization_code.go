package db

import (
	"context"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
	"github.com/fromforgesoftware/go-kit/search/query"
	"gorm.io/gorm/clause"

	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

var authorizationCodeFieldMapping = map[string]string{
	fields.Code:       "code",
	fields.ConsumedAt: "consumed_at",
	fields.ExpiresAt:  "expires_at",
}

type authorizationCodeEntity struct {
	ECode          string     `gorm:"column:code;primaryKey"`
	ECreatedAt     time.Time  `gorm:"column:created_at;autoCreateTime:true"`
	ERealmID       string     `gorm:"column:realm_id;type:uuid"`
	EClientID      string     `gorm:"column:client_id"`
	EAccountID     string     `gorm:"column:account_id;type:uuid"`
	ESessionID     *string    `gorm:"column:session_id;type:uuid"`
	ERedirectURI   string     `gorm:"column:redirect_uri"`
	EScopes        []string   `gorm:"column:scopes;type:jsonb;serializer:json"`
	EPKCEChallenge string     `gorm:"column:pkce_challenge"`
	ENonce         string     `gorm:"column:nonce"`
	EExpiresAt     time.Time  `gorm:"column:expires_at"`
	EConsumedAt    *time.Time `gorm:"column:consumed_at"`
}

func (authorizationCodeEntity) TableName() string { return "aegis.authorization_code" }

func authCodeToDomain(e *authorizationCodeEntity) domain.AuthorizationCode {
	var sessionID string
	if e.ESessionID != nil {
		sessionID = *e.ESessionID
	}
	return domain.AuthorizationCode{
		Code:          e.ECode,
		RealmID:       e.ERealmID,
		ClientID:      e.EClientID,
		AccountID:     e.EAccountID,
		SessionID:     sessionID,
		RedirectURI:   e.ERedirectURI,
		Scopes:        e.EScopes,
		PKCEChallenge: e.EPKCEChallenge,
		Nonce:         e.ENonce,
		ExpiresAt:     e.EExpiresAt,
	}
}

type authorizationCodeRepo struct {
	*postgres.Repo
}

func NewAuthorizationCodeRepository(db *gormdb.DBClient) (*authorizationCodeRepo, error) {
	r, err := postgres.NewRepo(db, authorizationCodeFieldMapping)
	if err != nil {
		return nil, err
	}
	return &authorizationCodeRepo{Repo: r}, nil
}

func (r *authorizationCodeRepo) Create(ctx context.Context, c domain.AuthorizationCode) error {
	e := &authorizationCodeEntity{
		ECode:          c.Code,
		ERealmID:       c.RealmID,
		EClientID:      c.ClientID,
		EAccountID:     c.AccountID,
		ESessionID:     nilIfEmpty(c.SessionID),
		ERedirectURI:   c.RedirectURI,
		EScopes:        orEmptySlice(c.Scopes),
		EPKCEChallenge: c.PKCEChallenge,
		ENonce:         c.Nonce,
		EExpiresAt:     c.ExpiresAt,
	}
	if err := r.DB.WithContext(ctx).Create(e).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// Consume atomically marks a valid (unconsumed, unexpired) code consumed and
// returns it; the single UPDATE ... WHERE consumed_at IS NULL prevents code
// replay. NotFound when no valid code matches.
func (r *authorizationCodeRepo) Consume(ctx context.Context, code string, now time.Time) (domain.AuthorizationCode, error) {
	q := query.New(
		query.FilterBy(filter.OpEq, fields.Code, code),
		query.FilterBy(filter.OpIsNull, fields.ConsumedAt, nil),
		query.FilterBy(filter.OpGT, fields.ExpiresAt, now),
	)
	e := &authorizationCodeEntity{}
	res := r.QueryApply(ctx, q).
		Model(e).
		Clauses(clause.Returning{}).
		Update(r.FMapper()[fields.ConsumedAt], now)
	if res.Error != nil {
		return domain.AuthorizationCode{}, postgres.NewErrUnknown(res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.AuthorizationCode{}, apierrors.NotFound("authorization code", "")
	}
	return authCodeToDomain(e), nil
}

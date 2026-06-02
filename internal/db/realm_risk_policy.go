package db

import (
	"context"
	"errors"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/search/query"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

var realmRiskPolicyFieldMapping = map[string]string{
	fields.ID:      "id",
	fields.RealmID: "realm_id",
}

type realmRiskPolicyEntity struct {
	postgres.Model

	ERealmID         string `gorm:"column:realm_id;type:uuid"`
	ENewIPWeight     int    `gorm:"column:new_ip_weight"`
	ENewDeviceWeight int    `gorm:"column:new_device_weight"`
	EFailureWeight   int    `gorm:"column:failure_weight"`
	EStepUpThreshold int    `gorm:"column:step_up_threshold"`
	EDenyThreshold   int    `gorm:"column:deny_threshold"`
}

func (e *realmRiskPolicyEntity) TableName() string   { return "aegis.realm_risk_policy" }
func (e *realmRiskPolicyEntity) Type() resource.Type { return domain.ResourceTypeRealmRiskPolicy }
func (e *realmRiskPolicyEntity) RealmID() string     { return e.ERealmID }
func (e *realmRiskPolicyEntity) Policy() domain.RiskPolicy {
	return domain.RiskPolicy{
		NewIPWeight:     e.ENewIPWeight,
		NewDeviceWeight: e.ENewDeviceWeight,
		FailureWeight:   e.EFailureWeight,
		StepUpThreshold: e.EStepUpThreshold,
		DenyThreshold:   e.EDenyThreshold,
	}
}

type realmRiskPolicyRepo struct {
	*postgres.Repo
}

func NewRealmRiskPolicyRepository(db *gormdb.DBClient) (*realmRiskPolicyRepo, error) {
	r, err := postgres.NewRepo(db, realmRiskPolicyFieldMapping)
	if err != nil {
		return nil, err
	}
	return &realmRiskPolicyRepo{Repo: r}, nil
}

// Upsert sets the realm's risk policy, replacing any existing one (one per realm).
func (r *realmRiskPolicyRepo) Upsert(ctx context.Context, p domain.RealmRiskPolicy) (domain.RealmRiskPolicy, error) {
	pol := p.Policy()
	row := &realmRiskPolicyEntity{
		Model:            postgres.ModelFromResource(p),
		ERealmID:         p.RealmID(),
		ENewIPWeight:     pol.NewIPWeight,
		ENewDeviceWeight: pol.NewDeviceWeight,
		EFailureWeight:   pol.FailureWeight,
		EStepUpThreshold: pol.StepUpThreshold,
		EDenyThreshold:   pol.DenyThreshold,
	}
	if err := r.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "realm_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"new_ip_weight", "new_device_weight", "failure_weight", "step_up_threshold", "deny_threshold", "updated_at",
		}),
	}).Create(row).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return row, nil
}

func (r *realmRiskPolicyRepo) GetByRealm(ctx context.Context, realmID string) (domain.RealmRiskPolicy, error) {
	q := query.New(query.FilterBy(filter.OpEq, fields.RealmID, realmID))
	var e realmRiskPolicyEntity
	if err := r.QueryApply(ctx, q).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.NotFound("realm risk policy", realmID)
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return &e, nil
}

package app

import (
	"context"

	apierrors "github.com/fromforgesoftware/go-kit/errors"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// RealmACRPolicyRepository persists per-realm MFA / assurance policy.
type RealmACRPolicyRepository interface {
	Upsert(ctx context.Context, p domain.RealmACRPolicy) (domain.RealmACRPolicy, error)
	GetByRealm(ctx context.Context, realmID string) (domain.RealmACRPolicy, error)
}

// MFAPolicyUsecase is the admin surface for per-realm MFA requirements.
type MFAPolicyUsecase interface {
	SetPolicy(ctx context.Context, p domain.RealmACRPolicy) (domain.RealmACRPolicy, error)
	GetPolicy(ctx context.Context, realmID string) (domain.RealmACRPolicy, error)
}

type mfaPolicyUsecase struct {
	policies RealmACRPolicyRepository
}

func NewMFAPolicyUsecase(policies RealmACRPolicyRepository) MFAPolicyUsecase {
	return &mfaPolicyUsecase{policies: policies}
}

func (uc *mfaPolicyUsecase) SetPolicy(ctx context.Context, p domain.RealmACRPolicy) (domain.RealmACRPolicy, error) {
	if p.RealmID() == "" {
		return nil, apierrors.InvalidArgument("realm_id is required")
	}
	return uc.policies.Upsert(ctx, p)
}

// GetPolicy returns the realm's policy, or a permissive default when none is
// set so callers can treat "no policy" as "MFA optional".
func (uc *mfaPolicyUsecase) GetPolicy(ctx context.Context, realmID string) (domain.RealmACRPolicy, error) {
	p, err := uc.policies.GetByRealm(ctx, realmID)
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return domain.NewRealmACRPolicy(realmID, false, ""), nil
		}
		return nil, err
	}
	return p, nil
}

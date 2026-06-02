package app

import (
	"context"

	"github.com/fromforgesoftware/go-kit/application/repository"
	apierrors "github.com/fromforgesoftware/go-kit/errors"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// PasswordPolicyRepository reads the per-realm password policy through the
// kit's generic Getter. A realm with no row returns NotFound; the usecase
// falls back to the default policy.
type PasswordPolicyRepository interface {
	repository.Getter[domain.PasswordPolicy]
}

// validatePassword checks a password against the realm's policy (or the
// default when the realm has none), mapping a violation to INVALID_ARGUMENT.
// Shared by every flow that sets a password.
func validatePassword(ctx context.Context, policies PasswordPolicyRepository, realmID, password string) error {
	policy, err := policies.Get(ctx, byRealm(realmID))
	if err != nil {
		if !apierrors.Is(err, apierrors.CodeNotFound) {
			return err
		}
		policy = domain.DefaultPasswordPolicy()
	}
	if err := domain.ValidatePassword(policy, password); err != nil {
		return apierrors.InvalidArgument(err.Error())
	}
	return nil
}

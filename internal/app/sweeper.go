package app

import (
	"context"
	"time"
)

// GrantSweeper removes expired bindings and refreshes the projection so the
// closure drops them. It's driven both by an interval ticker and the admin
// sweep endpoint.
type GrantSweeper interface {
	Sweep(ctx context.Context) (int64, error)
}

type grantSweeper struct {
	bindings BindingRepository
	authz    AuthorizationUsecase
	now      func() time.Time
}

func NewGrantSweeper(bindings BindingRepository, authz AuthorizationUsecase) GrantSweeper {
	return &grantSweeper{bindings: bindings, authz: authz, now: time.Now}
}

// Sweep hard-deletes expired bindings; if any were removed it refreshes the
// projection so they leave the closure too. A no-op sweep skips the refresh.
func (s *grantSweeper) Sweep(ctx context.Context) (int64, error) {
	removed, err := s.bindings.DeleteExpired(ctx, s.now())
	if err != nil {
		return 0, err
	}
	if removed == 0 {
		return 0, nil
	}
	if err := s.authz.Refresh(ctx); err != nil {
		return removed, err
	}
	return removed, nil
}

package app

import (
	"context"
	"time"
)

// GrantSweeper removes expired bindings and keeps the projection published: it
// refreshes whenever the projection is behind the write_version, so an authz
// write whose explicit Refresh failed is republished within one sweep interval.
// It's driven both by an interval ticker and the admin sweep endpoint.
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

// Sweep hard-deletes expired bindings, then refreshes the projection when it
// is behind — because bindings were just removed, or because earlier writes
// were never published (their writer's Refresh failed). A sweep with nothing
// expired and nothing pending skips the refresh.
func (s *grantSweeper) Sweep(ctx context.Context) (int64, error) {
	removed, err := s.bindings.DeleteExpired(ctx, s.now())
	if err != nil {
		return 0, err
	}
	if removed == 0 {
		write, projection, err := s.authz.Version(ctx)
		if err != nil {
			return 0, err
		}
		if projection >= write {
			return 0, nil
		}
	}
	if err := s.authz.Refresh(ctx); err != nil {
		return removed, err
	}
	return removed, nil
}

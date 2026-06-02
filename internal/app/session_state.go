package app

import (
	"context"
	"time"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/application/usecase"
	apierrors "github.com/fromforgesoftware/go-kit/errors"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// SessionStateRepository persists live session topology.
type SessionStateRepository interface {
	repository.Getter[domain.SessionState]
	repository.Lister[domain.SessionState]
	Upsert(ctx context.Context, s domain.SessionState) (domain.SessionState, error)
	Touch(ctx context.Context, sessionID string, at time.Time) error
	PurgeIdle(ctx context.Context, before time.Time) (int64, error)
}

// SessionStateUsecase is the opt-in stateful-session surface: track where a
// session is, keep it alive, and reclaim idle rows.
type SessionStateUsecase interface {
	repository.Getter[domain.SessionState]
	repository.Lister[domain.SessionState]
	Track(ctx context.Context, s domain.SessionState) (domain.SessionState, error)
	Touch(ctx context.Context, sessionID string) error
	PurgeIdle(ctx context.Context, idleFor time.Duration) (int64, error)
}

type sessionStateUsecase struct {
	usecase.Getter[domain.SessionState]
	usecase.Lister[domain.SessionState]

	states SessionStateRepository
	now    func() time.Time
}

func NewSessionStateUsecase(states SessionStateRepository) SessionStateUsecase {
	return &sessionStateUsecase{
		Getter: usecase.NewGetter(states, domain.ResourceTypeSessionState),
		Lister: usecase.NewLister(states),
		states: states,
		now:    time.Now,
	}
}

func (uc *sessionStateUsecase) Track(ctx context.Context, s domain.SessionState) (domain.SessionState, error) {
	if s.ID() == "" {
		return nil, apierrors.InvalidArgument("session id is required")
	}
	if s.AccountID() == "" {
		return nil, apierrors.InvalidArgument("account_id is required")
	}
	return uc.states.Upsert(ctx, s)
}

func (uc *sessionStateUsecase) Touch(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return apierrors.InvalidArgument("session id is required")
	}
	return uc.states.Touch(ctx, sessionID, uc.now())
}

func (uc *sessionStateUsecase) PurgeIdle(ctx context.Context, idleFor time.Duration) (int64, error) {
	return uc.states.PurgeIdle(ctx, uc.now().Add(-idleFor))
}

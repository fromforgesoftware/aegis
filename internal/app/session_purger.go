package app

import (
	"context"
	"time"
)

// SessionPurger reclaims session-state rows idle beyond a threshold. Wired to
// an interval ticker only when stateful sessions are enabled.
type SessionPurger interface {
	Purge(ctx context.Context) (int64, error)
}

type sessionPurger struct {
	states  SessionStateUsecase
	idleFor time.Duration
}

func NewSessionPurger(states SessionStateUsecase, idleFor time.Duration) SessionPurger {
	return &sessionPurger{states: states, idleFor: idleFor}
}

func (p *sessionPurger) Purge(ctx context.Context) (int64, error) {
	return p.states.PurgeIdle(ctx, p.idleFor)
}

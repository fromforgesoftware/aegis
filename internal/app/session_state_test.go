package app_test

import (
	"context"
	"testing"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

func TestSessionStateTrack_Upserts(t *testing.T) {
	states := apptest.NewSessionStateRepository(t)
	uc := app.NewSessionStateUsecase(states)
	want := internaltest.NewSessionState(
		internaltest.WithSessionStateSessionID("sess-1"),
		internaltest.WithSessionStateAccountID("acc-1"),
		internaltest.WithSessionStateCurrentShard("silvermoon"),
	)
	states.EXPECT().Upsert(mock.Anything, mock.MatchedBy(internaltest.MatchSessionState(want))).
		Return(want, nil)

	got, err := uc.Track(context.Background(), want)
	require.NoError(t, err)
	assert.Equal(t, "silvermoon", got.CurrentShard())
}

func TestSessionStateTrack_Validates(t *testing.T) {
	states := apptest.NewSessionStateRepository(t)
	uc := app.NewSessionStateUsecase(states)
	_, err := uc.Track(context.Background(), internaltest.NewSessionState(
		internaltest.WithSessionStateAccountID("")))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestSessionStatePurgeIdle_PassesCutoff(t *testing.T) {
	states := apptest.NewSessionStateRepository(t)
	uc := app.NewSessionStateUsecase(states)
	states.EXPECT().PurgeIdle(mock.Anything, mock.MatchedBy(func(before time.Time) bool {
		// cutoff is roughly now-idleFor; just assert it's in the past.
		return before.Before(time.Now())
	})).Return(int64(4), nil)

	n, err := uc.PurgeIdle(context.Background(), 30*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(4), n)
}

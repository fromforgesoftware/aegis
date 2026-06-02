package app_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/audit"

	"github.com/fromforgesoftware/aegis/internal/app"
)

type captureSink struct {
	events []audit.Event
}

func (s *captureSink) Emit(_ context.Context, e audit.Event) error {
	s.events = append(s.events, e)
	return nil
}

func TestAuditor_RecordBuildsAndEmits(t *testing.T) {
	sink := &captureSink{}
	auditor := app.NewAuditor(sink)

	auditor.Record(context.Background(), "binding.grant", "binding", "bind-1", map[string]any{"roleId": "role-1"})

	require.Len(t, sink.events, 1)
	e := sink.events[0]
	assert.Equal(t, "binding.grant", e.Action)
	assert.Equal(t, "binding", e.ResourceType)
	assert.Equal(t, "bind-1", e.ResourceID)
	assert.Equal(t, "role-1", e.Changes["roleId"])
}

type failingSink struct{}

func (failingSink) Emit(context.Context, audit.Event) error {
	return assert.AnError
}

func TestAuditor_SwallowsSinkErrors(t *testing.T) {
	// A sink failure must not panic or surface — auditing is best-effort.
	auditor := app.NewAuditor(failingSink{})
	assert.NotPanics(t, func() {
		auditor.Record(context.Background(), "binding.grant", "binding", "bind-1", nil)
	})
}

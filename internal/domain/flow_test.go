package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

func TestRequiredFields(t *testing.T) {
	names := func(fs []domain.FlowField) []string {
		out := make([]string, len(fs))
		for i, f := range fs {
			out[i] = f.Name
		}
		return out
	}

	assert.Equal(t, []string{"email", "password"}, names(domain.RequiredFields(domain.FlowTypeLogin)))
	assert.Equal(t, []string{"email", "password", "displayName"}, names(domain.RequiredFields(domain.FlowTypeRegistration)))
	assert.Equal(t, []string{"email"}, names(domain.RequiredFields(domain.FlowTypeRecovery)))
	assert.Equal(t, []string{"token"}, names(domain.RequiredFields(domain.FlowTypeVerification)))
	assert.Nil(t, domain.RequiredFields(domain.FlowType("bogus")))
}

func TestFlowType_Valid(t *testing.T) {
	for _, ft := range []domain.FlowType{
		domain.FlowTypeLogin, domain.FlowTypeRegistration, domain.FlowTypeRecovery, domain.FlowTypeVerification,
	} {
		assert.True(t, ft.Valid(), ft)
	}
	assert.False(t, domain.FlowType("OTHER").Valid())
}

func TestFlowExpired(t *testing.T) {
	now := time.Now()
	future := domain.NewFlow("r", domain.FlowTypeLogin, now.Add(time.Minute))
	past := domain.NewFlow("r", domain.FlowTypeLogin, now.Add(-time.Minute))

	assert.False(t, domain.FlowExpired(future, now))
	assert.True(t, domain.FlowExpired(past, now))
}

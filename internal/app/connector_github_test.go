package app_test

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

const githubUserURL = "https://api.github.com/user"

func githubConfig() domain.ExternalIDPConfig {
	return internaltest.NewExternalIDP(
		internaltest.WithExternalIDPRealmID("r"),
		internaltest.WithExternalIDPKind(domain.ExternalIDPKindOAuthGitHub),
		internaltest.WithExternalIDPName("github-prod"),
	)
}

func TestGitHubConnector_Verify_Success(t *testing.T) {
	http := &fakeHTTP{responses: map[string]string{
		githubUserURL: `{"id":42,"login":"octocat","name":"Octo Cat","email":"o@example.com"}`,
	}}
	user, err := app.NewGitHubConnector(http).Verify(t.Context(), githubConfig(), "ghp_TOKEN")
	require.NoError(t, err)
	assert.Equal(t, "42", user.ID)
	assert.Equal(t, "o@example.com", user.Email)
	assert.Equal(t, "Octo Cat", user.Name)
	require.NotNil(t, http.lastReq)
	assert.Equal(t, "Bearer ghp_TOKEN", http.lastReq.Header.Get("Authorization"))
}

func TestGitHubConnector_Verify_FallsBackToLoginAsName(t *testing.T) {
	http := &fakeHTTP{responses: map[string]string{
		githubUserURL: `{"id":7,"login":"octocat","email":null}`,
	}}
	user, err := app.NewGitHubConnector(http).Verify(t.Context(), githubConfig(), "tok")
	require.NoError(t, err)
	assert.Equal(t, "octocat", user.Name)
	assert.Empty(t, user.Email)
}

func TestGitHubConnector_Verify_Unauthorized(t *testing.T) {
	doer := &fakeHTTPStatus{status: http.StatusUnauthorized, body: ""}
	_, err := app.NewGitHubConnector(doer).Verify(t.Context(), githubConfig(), "bad")
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated))
}

func TestGitHubConnector_Verify_EmptyToken(t *testing.T) {
	_, err := app.NewGitHubConnector(&fakeHTTP{}).Verify(t.Context(), githubConfig(), "")
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated))
}

func TestGitHubConnector_Verify_TransportError(t *testing.T) {
	doer := &fakeHTTP{errs: map[string]error{githubUserURL: errors.New("dial tcp")}}
	_, err := app.NewGitHubConnector(doer).Verify(t.Context(), githubConfig(), "tok")
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInternalError))
}

// fakeHTTPStatus returns a fixed status code regardless of URL — useful for
// asserting how the connector classifies non-200 responses.
type fakeHTTPStatus struct {
	status int
	body   string
}

func (f *fakeHTTPStatus) Do(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: f.status, Body: io.NopCloser(strings.NewReader(f.body))}, nil
}

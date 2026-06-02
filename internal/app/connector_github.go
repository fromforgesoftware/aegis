package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	apierrors "github.com/fromforgesoftware/go-kit/errors"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// githubUserURL is the canonical "who is the bearer of this token?" endpoint.
const githubUserURL = "https://api.github.com/user"

// GitHubConnector verifies a GitHub OAuth2 access token by hitting /user with
// the token as a Bearer credential — GitHub doesn't issue OIDC ID tokens, so
// "verify" here means "the token is valid for this user." The caller is
// expected to have already completed the OAuth code-exchange and to pass us
// the access_token; the redirect-and-exchange flow lands with Wave 4 slice 6.
type GitHubConnector struct {
	http HTTPDoer
}

func NewGitHubConnector(http HTTPDoer) *GitHubConnector {
	if http == nil {
		http = defaultHTTPClient()
	}
	return &GitHubConnector{http: http}
}

func (c *GitHubConnector) Kind() domain.ExternalIDPKind { return domain.ExternalIDPKindOAuthGitHub }

type githubUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (c *GitHubConnector) Verify(ctx context.Context, _ domain.ExternalIDPConfig, accessToken string) (ExternalUser, error) {
	if accessToken == "" {
		return ExternalUser{}, apierrors.Unauthenticated("missing GitHub access token")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubUserURL, nil)
	if err != nil {
		return ExternalUser{}, apierrors.InternalError("failed to build GitHub user request")
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.http.Do(req)
	if err != nil {
		return ExternalUser{}, apierrors.InternalError("failed to reach GitHub")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnauthorized {
		return ExternalUser{}, apierrors.Unauthenticated("GitHub rejected the access token")
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return ExternalUser{}, apierrors.InternalError("GitHub returned non-2xx")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ExternalUser{}, apierrors.InternalError("failed to read GitHub response")
	}

	var u githubUser
	if err := json.Unmarshal(body, &u); err != nil {
		return ExternalUser{}, apierrors.InternalError("GitHub response is not valid JSON")
	}
	if u.ID == 0 {
		return ExternalUser{}, apierrors.Unauthenticated("GitHub response has no user id")
	}
	name := u.Name
	if name == "" {
		name = u.Login
	}
	return ExternalUser{
		ID:    strconv.FormatInt(u.ID, 10),
		Email: u.Email,
		// GitHub's /user endpoint doesn't carry verified-email status; querying
		// /user/emails for that is a future enhancement. Default to false.
		EmailVerified: false,
		Name:          name,
	}, nil
}

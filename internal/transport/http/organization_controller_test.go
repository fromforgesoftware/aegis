package http_test

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/fromforgesoftware/go-kit/transport/rest/restest"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
	aegishttp "github.com/fromforgesoftware/aegis/internal/transport/http"
)

// These tests exist because every route below was, at one point, reachable with no credential at
// all. The ingress publishes /api/organizations straight to this service and aegis installs no
// global authentication middleware, so a missing check on a handler is a missing check on the
// internet: an anonymous GET returned every tenant in every realm, and an anonymous PATCH reached
// the database. The assertions are therefore about status codes on the unhappy paths — a 404 where
// a 401 belongs is exactly what that bug looked like.

const (
	testOrgID    = "org-1"
	testOwnerID  = "acct-owner"
	testOtherID  = "acct-other"
	testRealmID  = "realm-1"
	testRealmNam = "trading-bot"
)

// bearerFor builds an UNSIGNED three-part token whose payload carries the issuer and subject.
//
// The controller parses claims with auth.NewToken (which only base64-decodes the payload) and
// verifies the signature separately through TokenIssuer, which is mocked here. So a real signature
// would prove nothing this test cares about: what is under test is the authorization decision made
// after verification succeeds.
func bearerFor(subject string) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(
		`{"iss":"https://aegis.test/realms/` + testRealmNam + `","sub":"` + subject + `"}`))
	return "eyJhbGciOiJub25lIn0." + payload + ".sig"
}

// orgControllerFor wires the controller with the realm/token mocks already satisfied, so each test
// only has to state the organization facts that drive the decision under test.
func orgControllerFor(t *testing.T, orgs *apptest.OrganizationUsecase) http.Handler {
	t.Helper()
	realms := apptest.NewRealmUsecase(t)
	realms.EXPECT().Get(mock.Anything, mock.Anything).
		Return(domain.NewRealm(testRealmNam, domain.WithRealmID(testRealmID)), nil).Maybe()

	tokens := apptest.NewTokenIssuer(t)
	tokens.EXPECT().VerifyAccessToken(mock.Anything, mock.Anything, mock.Anything).
		Return(app.AccessClaims{}, nil).Maybe()

	return internaltest.NewRESTHandler(aegishttp.NewOrganizationController(orgs, realms, tokens))
}

// ownedOrg is the organization under test, owned by testOwnerID.
func ownedOrg() domain.Organization {
	return internaltest.NewOrganization(
		internaltest.WithOrganizationID(testOrgID),
		internaltest.WithOrganizationRealmID(testRealmID),
		internaltest.WithOrganizationOwnerID(testOwnerID),
		internaltest.WithOrganizationName("Acme"),
		internaltest.WithOrganizationSlug("acme"),
	)
}

func authedReq(t *testing.T, method, path, subject, body string) *http.Request {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = restest.NewReq(t, context.Background(), method, path, http.NoBody)
	} else {
		req = jsonapiReq(t, method, path, body)
	}
	req.Header.Set("Authorization", "Bearer "+bearerFor(subject))
	return req
}

func TestOrganizationController_WritesRequireAuthentication(t *testing.T) {
	// No mock expectations: an unauthenticated request must be rejected BEFORE the usecase is
	// consulted. mockery fails the test if a call arrives, so this also proves nothing touched
	// the database — which is what the 404 in the original bug proved had happened.
	orgs := apptest.NewOrganizationUsecase(t)
	handler := orgControllerFor(t, orgs)

	restest.NewHandlerSuite(
		restest.NewHandlerTest("anonymous PATCH is 401, not 404",
			handler,
			jsonapiReq(t, http.MethodPatch, "/api/organizations/"+testOrgID,
				`{"data":{"type":"organizations","id":"org-1","attributes":{"name":"Renamed"}}}`),
			restest.AssertResponseStatus(http.StatusUnauthorized),
		),
		restest.NewHandlerTest("anonymous DELETE is 401",
			handler,
			restest.NewReq(t, context.Background(), http.MethodDelete, "/api/organizations/"+testOrgID, http.NoBody),
			restest.AssertResponseStatus(http.StatusUnauthorized),
		),
		restest.NewHandlerTest("anonymous add-member is 401",
			handler,
			jsonapiReq(t, http.MethodPost, "/api/organizations/"+testOrgID+"/members",
				`{"accountId":"acct-other","role":"member"}`),
			restest.AssertResponseStatus(http.StatusUnauthorized),
		),
		restest.NewHandlerTest("anonymous remove-member is 401",
			handler,
			restest.NewReq(t, context.Background(), http.MethodDelete,
				"/api/organizations/"+testOrgID+"/members/"+testOtherID, http.NoBody),
			restest.AssertResponseStatus(http.StatusUnauthorized),
		),
	).Exec(t)
}

func TestOrganizationController_ReadsRequireAuthentication(t *testing.T) {
	orgs := apptest.NewOrganizationUsecase(t)
	handler := orgControllerFor(t, orgs)

	restest.NewHandlerSuite(
		// This one returned 200 with every tenant's name, slug, owner id and realm id.
		restest.NewHandlerTest("anonymous collection listing is 401, not a tenant dump",
			handler,
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/organizations", http.NoBody),
			restest.AssertResponseStatus(http.StatusUnauthorized),
		),
		restest.NewHandlerTest("anonymous read of one organization is 401",
			handler,
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/organizations/"+testOrgID, http.NoBody),
			restest.AssertResponseStatus(http.StatusUnauthorized),
		),
		restest.NewHandlerTest("anonymous member listing is 401",
			handler,
			restest.NewReq(t, context.Background(), http.MethodGet,
				"/api/organizations/"+testOrgID+"/members", http.NoBody),
			restest.AssertResponseStatus(http.StatusUnauthorized),
		),
	).Exec(t)
}

func TestOrganizationController_WritesRequireOwnership(t *testing.T) {
	orgs := apptest.NewOrganizationUsecase(t)
	// Authenticated, real organization, but the caller is not its owner.
	orgs.EXPECT().Get(mock.Anything, mock.Anything).Return(ownedOrg(), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest("a non-owner cannot rename the workspace",
			orgControllerFor(t, orgs),
			authedReq(t, http.MethodPatch, "/api/organizations/"+testOrgID, testOtherID,
				`{"data":{"type":"organizations","id":"org-1","attributes":{"name":"Hostile"}}}`),
			restest.AssertResponseStatus(http.StatusForbidden),
		),
	).Exec(t)
}

func TestOrganizationController_ReadsRequireMembership(t *testing.T) {
	orgs := apptest.NewOrganizationUsecase(t)
	// The caller belongs to a DIFFERENT organization, so this one must not be confirmed to exist.
	orgs.EXPECT().ListForAccount(mock.Anything, testOtherID).Return([]domain.Organization{
		internaltest.NewOrganization(internaltest.WithOrganizationID("org-elsewhere")),
	}, nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest("a non-member reading another workspace gets 404, not 403",
			orgControllerFor(t, orgs),
			authedReq(t, http.MethodGet, "/api/organizations/"+testOrgID, testOtherID, ""),
			restest.AssertResponseStatus(http.StatusNotFound),
		),
	).Exec(t)
}

func TestOrganizationController_OwnerCanPatch(t *testing.T) {
	orgs := apptest.NewOrganizationUsecase(t)
	orgs.EXPECT().Get(mock.Anything, mock.Anything).Return(ownedOrg(), nil)
	// Three variadic matchers, one per PatchOption: the id filter plus the two fields being set.
	// mockery matches variadic arguments positionally, so this arity also asserts that BOTH name
	// and slug reached the repository — a validator that silently dropped one would fail here.
	orgs.EXPECT().Patch(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]domain.Organization{
		internaltest.NewOrganization(
			internaltest.WithOrganizationID(testOrgID),
			internaltest.WithOrganizationName("Renamed"),
			internaltest.WithOrganizationSlug("renamed"),
		),
	}, nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest("the owner can rename and re-slug",
			orgControllerFor(t, orgs),
			authedReq(t, http.MethodPatch, "/api/organizations/"+testOrgID, testOwnerID,
				`{"data":{"type":"organizations","id":"org-1","attributes":{"name":"Renamed","slug":"renamed"}}}`),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestOrganizationController_MemberCanRead(t *testing.T) {
	orgs := apptest.NewOrganizationUsecase(t)
	orgs.EXPECT().ListForAccount(mock.Anything, testOwnerID).
		Return([]domain.Organization{ownedOrg()}, nil)
	orgs.EXPECT().Get(mock.Anything, mock.Anything).Return(ownedOrg(), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest("a member can read their own workspace",
			orgControllerFor(t, orgs),
			authedReq(t, http.MethodGet, "/api/organizations/"+testOrgID, testOwnerID, ""),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestOrganizationController_PatchValidation(t *testing.T) {
	// Each case is rejected before the patch is applied, so Patch is never expected. Get is, since
	// ownership is established first.
	// `wantMsg` is asserted as well as the status, because a malformed request body ALSO produces
	// 400 ("invalid request body"). Without pinning the message, a case that failed to parse would
	// pass this test while proving nothing about the validator.
	cases := []struct {
		name    string
		body    string
		wantMsg string
	}{
		{"a blank name is refused", `{"name":"   "}`, "name cannot be blank"},
		{"a name with a control character is refused", `{"name":"Ac\nme"}`, "control character"},
		{"a name over 128 characters is refused", `{"name":"` + longName() + `"}`, "longer than 128"},
		{"a slug with a space is refused", `{"slug":"My Workspace"}`, "slug must be lowercase"},
		{"an uppercase slug is refused", `{"slug":"Acme"}`, "slug must be lowercase"},
		{"a slug with a leading hyphen is refused", `{"slug":"-acme"}`, "slug must be lowercase"},
		{"a slug with a double hyphen is refused", `{"slug":"ac--me"}`, "slug must be lowercase"},
		{"an unknown status is refused", `{"status":"ACITVE"}`, "status must be ACTIVE"},
	}

	tests := make([]*restest.HandlerTest, 0, len(cases))
	for _, tc := range cases {
		orgs := apptest.NewOrganizationUsecase(t)
		orgs.EXPECT().Get(mock.Anything, mock.Anything).Return(ownedOrg(), nil)
		wantMsg := tc.wantMsg
		tests = append(tests, restest.NewHandlerTest(tc.name,
			orgControllerFor(t, orgs),
			authedReq(t, http.MethodPatch, "/api/organizations/"+testOrgID, testOwnerID,
				`{"data":{"type":"organizations","id":"org-1","attributes":`+tc.body+`}}`),
			restest.AssertResponseStatus(http.StatusBadRequest),
			restest.AssertResponse(func(t *testing.T, res *http.Response) {
				body, err := io.ReadAll(res.Body)
				if err != nil {
					t.Fatalf("read body: %v", err)
				}
				if !strings.Contains(string(body), wantMsg) {
					t.Errorf("status=%d want the error to mention %q, got %s", res.StatusCode, wantMsg, body)
				}
			}),
		))
	}
	restest.NewHandlerSuite(tests...).Exec(t)
}

func longName() string {
	out := make([]byte, 129)
	for i := range out {
		out[i] = 'a'
	}
	return string(out)
}

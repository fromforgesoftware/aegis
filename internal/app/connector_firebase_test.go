package app_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/cryptox"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

const firebaseCertsURL = "https://www.googleapis.com/robot/v1/metadata/x509/securetoken@system.gserviceaccount.com"

// firebaseCertJSON mints a Firebase-style certs response: a JSON object
// mapping kid → PEM-encoded x509 self-signed cert wrapping pub.
func firebaseCertJSON(t *testing.T, key *rsa.PrivateKey, kid string) string {
	t.Helper()
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "firebase-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	body, err := json.Marshal(map[string]string{kid: string(pemBytes)})
	require.NoError(t, err)
	return string(body)
}

func mintFirebaseIDToken(t *testing.T, key *rsa.PrivateKey, kid, projectID, sub, email string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":            "https://securetoken.google.com/" + projectID,
		"aud":            projectID,
		"sub":            sub,
		"email":          email,
		"email_verified": true,
		"name":           "Firebase User",
		"iat":            time.Now().Unix(),
		"exp":            time.Now().Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = kid
	s, err := tok.SignedString(key)
	require.NoError(t, err)
	return s
}

func firebaseConfig(projectID string) domain.ExternalIDPConfig {
	return internaltest.NewExternalIDP(
		internaltest.WithExternalIDPRealmID("r"),
		internaltest.WithExternalIDPKind(domain.ExternalIDPKindFirebase),
		internaltest.WithExternalIDPName("firebase-prod"),
		internaltest.WithExternalIDPConfig(map[string]string{"project_id": projectID}),
	)
}

func TestFirebaseConnector_Verify_Success(t *testing.T) {
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	const projectID = "trading-bot-prod"
	idToken := mintFirebaseIDToken(t, key, "fb-kid", projectID, "fb-uid-1", "f@b.com")

	http := &fakeHTTP{responses: map[string]string{
		firebaseCertsURL: firebaseCertJSON(t, key, "fb-kid"),
	}}
	user, err := app.NewFirebaseConnector(http).Verify(t.Context(), firebaseConfig(projectID), idToken)
	require.NoError(t, err)
	assert.Equal(t, "fb-uid-1", user.ID)
	assert.Equal(t, "f@b.com", user.Email)
	assert.True(t, user.EmailVerified)
}

func TestFirebaseConnector_Verify_WrongProjectID(t *testing.T) {
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	idToken := mintFirebaseIDToken(t, key, "fb-kid", "wrong-project", "u", "f@b.com")
	http := &fakeHTTP{responses: map[string]string{
		firebaseCertsURL: firebaseCertJSON(t, key, "fb-kid"),
	}}
	_, err = app.NewFirebaseConnector(http).Verify(t.Context(), firebaseConfig("expected-project"), idToken)
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated))
}

func TestFirebaseConnector_Verify_UnknownKid(t *testing.T) {
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	idToken := mintFirebaseIDToken(t, key, "rogue-kid", "p", "u", "f@b.com")
	http := &fakeHTTP{responses: map[string]string{
		firebaseCertsURL: firebaseCertJSON(t, key, "fb-kid"),
	}}
	_, err = app.NewFirebaseConnector(http).Verify(t.Context(), firebaseConfig("p"), idToken)
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated))
}

func TestFirebaseConnector_Verify_MissingProjectID(t *testing.T) {
	cfg := internaltest.NewExternalIDP(
		internaltest.WithExternalIDPRealmID("r"),
		internaltest.WithExternalIDPKind(domain.ExternalIDPKindFirebase),
		internaltest.WithExternalIDPName("firebase-prod"),
	)
	_, err := app.NewFirebaseConnector(&fakeHTTP{}).Verify(t.Context(), cfg, "x")
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

// Sanity check on the cert-PEM round-trip itself — if the helper mints
// garbage, the connector tests would fail mysteriously.
func TestFirebaseConnector_TestHelperCertParses(t *testing.T) {
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	body := firebaseCertJSON(t, key, "fb-kid")
	require.True(t, len(body) > 0)
	require.Contains(t, body, `"fb-kid":"-----BEGIN CERTIFICATE-----`)
}

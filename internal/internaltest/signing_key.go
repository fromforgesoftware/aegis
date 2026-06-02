package internaltest

import (
	"github.com/fromforgesoftware/aegis/internal/domain"
)

// MatchSigningKey compares the identifying fields (realm, algorithm,
// status), ignoring the generated kid/id and key material. Use inside
// mock.MatchedBy for repo Create arg assertions.
func MatchSigningKey(want domain.SigningKey) func(domain.SigningKey) bool {
	return func(got domain.SigningKey) bool {
		if want == nil {
			return got == nil
		}
		if got == nil {
			return false
		}
		return want.RealmID() == got.RealmID() &&
			want.Algorithm() == got.Algorithm() &&
			want.Status() == got.Status()
	}
}

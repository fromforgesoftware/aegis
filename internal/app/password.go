package app

import "github.com/fromforgesoftware/go-kit/auth/password"

// Password hashing moved to the shared kit (go/kit/auth/password) so Aegis and
// the Foundry control plane share one implementation. These aliases keep the
// existing app.* call sites unchanged.
type (
	HashedPassword = password.Hashed
	PasswordHasher = password.Hasher
	Argon2idHasher = password.Argon2id
)

func NewArgon2idHasher() *password.Argon2id { return password.NewArgon2id() }

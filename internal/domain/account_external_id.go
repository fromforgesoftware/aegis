package domain

import "time"

// AccountExternalID binds an Aegis account to its identity at an upstream
// IdP. The pair (kind, external_id) is globally unique so the same Google
// account can't be linked to two Aegis accounts; an Aegis account may carry
// many links across kinds (Wave 5 account linking).
type AccountExternalID struct {
	AccountID  string
	Kind       ExternalIDPKind
	ExternalID string
	CreatedAt  time.Time
}

package domain

// PermissionCheck is a single (resource, permission) question for an account.
// It is a comparable value so callers can use it as a map key when collating
// batch results.
type PermissionCheck struct {
	ResourceID   string
	PermissionID string
}

// PermissionDecision is the answer to a PermissionCheck.
type PermissionDecision struct {
	ResourceID   string
	PermissionID string
	Allowed      bool
}

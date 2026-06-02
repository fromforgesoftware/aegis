package domain

// CompositionOperator folds a component role's permission set into the
// composite role's set, applied in ordinal order.
type CompositionOperator string

const (
	// CompositionUnion adds the component's permissions.
	CompositionUnion CompositionOperator = "UNION"
	// CompositionIntersect keeps only permissions also in the component.
	CompositionIntersect CompositionOperator = "INTERSECT"
	// CompositionExclude removes the component's permissions.
	CompositionExclude CompositionOperator = "EXCLUDE"
)

func (o CompositionOperator) Valid() bool {
	switch o {
	case CompositionUnion, CompositionIntersect, CompositionExclude:
		return true
	}
	return false
}

// RoleComponent is one entry in a composite role: which role to fold in, with
// what operator, at what position.
type RoleComponent struct {
	ComponentRoleID string
	Operator        CompositionOperator
	Ordinal         int
}

// PermissionImplication is a single edge in the permission-inheritance DAG:
// holding PermissionID implies holding ImpliedPermissionID.
type PermissionImplication struct {
	PermissionID        string
	ImpliedPermissionID string
}

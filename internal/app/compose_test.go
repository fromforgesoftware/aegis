package app_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

func union(id string, ord int) domain.RoleComponent {
	return domain.RoleComponent{ComponentRoleID: id, Operator: domain.CompositionUnion, Ordinal: ord}
}
func intersect(id string, ord int) domain.RoleComponent {
	return domain.RoleComponent{ComponentRoleID: id, Operator: domain.CompositionIntersect, Ordinal: ord}
}
func exclude(id string, ord int) domain.RoleComponent {
	return domain.RoleComponent{ComponentRoleID: id, Operator: domain.CompositionExclude, Ordinal: ord}
}

func TestCompose_DirectGrantsPassThrough(t *testing.T) {
	got := app.ComposeEffectivePermissions(
		map[string][]string{"r1": {"doc.read", "doc.write"}}, nil, nil)
	assert.Equal(t, []string{"doc.read", "doc.write"}, got["r1"])
}

func TestCompose_Union(t *testing.T) {
	got := app.ComposeEffectivePermissions(
		map[string][]string{
			"base": {"doc.read"},
			"comp": {"doc.write"},
		},
		map[string][]domain.RoleComponent{"base": {union("comp", 0)}},
		nil,
	)
	assert.Equal(t, []string{"doc.read", "doc.write"}, got["base"])
}

func TestCompose_Intersect(t *testing.T) {
	got := app.ComposeEffectivePermissions(
		map[string][]string{
			"base": {"doc.read", "doc.write"},
			"comp": {"doc.write", "doc.delete"},
		},
		map[string][]domain.RoleComponent{"base": {intersect("comp", 0)}},
		nil,
	)
	assert.Equal(t, []string{"doc.write"}, got["base"])
}

func TestCompose_Exclude(t *testing.T) {
	got := app.ComposeEffectivePermissions(
		map[string][]string{
			"base": {"doc.read", "doc.write"},
			"comp": {"doc.write"},
		},
		map[string][]domain.RoleComponent{"base": {exclude("comp", 0)}},
		nil,
	)
	assert.Equal(t, []string{"doc.read"}, got["base"])
}

func TestCompose_OrderedFold(t *testing.T) {
	// union(a) then exclude(b) differs from the reverse: here we add a's perms
	// then strip b's, leaving only what a had that b lacks.
	got := app.ComposeEffectivePermissions(
		map[string][]string{
			"base": {},
			"a":    {"doc.read", "doc.write"},
			"b":    {"doc.write"},
		},
		map[string][]domain.RoleComponent{"base": {union("a", 0), exclude("b", 1)}},
		nil,
	)
	assert.Equal(t, []string{"doc.read"}, got["base"])
}

func TestCompose_NestedComponents(t *testing.T) {
	// base unions mid, which itself unions leaf — resolution recurses.
	got := app.ComposeEffectivePermissions(
		map[string][]string{
			"leaf": {"doc.read"},
			"mid":  {"doc.write"},
			"base": {"doc.admin"},
		},
		map[string][]domain.RoleComponent{
			"base": {union("mid", 0)},
			"mid":  {union("leaf", 0)},
		},
		nil,
	)
	assert.Equal(t, []string{"doc.admin", "doc.read", "doc.write"}, got["base"])
}

func TestCompose_BreaksCycles(t *testing.T) {
	// a composes b composes a: the in-progress role resolves to empty, so the
	// fold terminates instead of looping. Each role always retains its own
	// direct grant; the exact union across the cycle is resolution-order
	// dependent, so we only assert termination + own-grant retention.
	got := app.ComposeEffectivePermissions(
		map[string][]string{
			"a": {"doc.read"},
			"b": {"doc.write"},
		},
		map[string][]domain.RoleComponent{
			"a": {union("b", 0)},
			"b": {union("a", 0)},
		},
		nil,
	)
	assert.Contains(t, got["a"], "doc.read")
	assert.Contains(t, got["b"], "doc.write")
}

func TestCompose_InheritanceExpandsTransitively(t *testing.T) {
	// doc.admin implies doc.write implies doc.read.
	got := app.ComposeEffectivePermissions(
		map[string][]string{"r1": {"doc.admin"}},
		nil,
		map[string][]string{
			"doc.admin": {"doc.write"},
			"doc.write": {"doc.read"},
		},
	)
	assert.Equal(t, []string{"doc.admin", "doc.read", "doc.write"}, got["r1"])
}

func TestCompose_InheritanceCycleTerminates(t *testing.T) {
	got := app.ComposeEffectivePermissions(
		map[string][]string{"r1": {"a"}},
		nil,
		map[string][]string{"a": {"b"}, "b": {"a"}},
	)
	assert.Equal(t, []string{"a", "b"}, got["r1"])
}

func TestCompose_EmptyRoleOmitted(t *testing.T) {
	got := app.ComposeEffectivePermissions(
		map[string][]string{
			"base":  {"doc.read"},
			"empty": {},
		},
		map[string][]domain.RoleComponent{"base": {intersect("empty", 0)}},
		nil,
	)
	_, baseHasRows := got["base"]
	assert.False(t, baseHasRows, "intersect with empty leaves no rows")
}

package app

import (
	"sort"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// ComposeEffectivePermissions resolves every role's effective permission set
// from its direct grants, its composition (component roles folded with
// UNION/INTERSECT/EXCLUDE in ordinal order), and the permission-inheritance
// DAG. The result is the cache the projection reads.
//
// Folds run in ordinal order starting from the role's direct grants.
// Composition cycles are broken by treating an in-progress role as the empty
// set. Permission inheritance is expanded transitively on the final composed
// set, so an effective grant of a permission also grants everything it
// implies.
func ComposeEffectivePermissions(
	direct map[string][]string,
	components map[string][]domain.RoleComponent,
	implications map[string][]string,
) map[string][]string {
	r := &resolver{
		direct:     direct,
		components: components,
		implied:    implications,
		memo:       map[string]map[string]struct{}{},
		visiting:   map[string]bool{},
	}

	roles := map[string]struct{}{}
	for id := range direct {
		roles[id] = struct{}{}
	}
	for id := range components {
		roles[id] = struct{}{}
	}

	out := make(map[string][]string, len(roles))
	for id := range roles {
		composed := r.resolve(id)
		expanded := r.expandInheritance(composed)
		if len(expanded) == 0 {
			continue
		}
		out[id] = sortedKeys(expanded)
	}
	return out
}

type resolver struct {
	direct     map[string][]string
	components map[string][]domain.RoleComponent
	implied    map[string][]string
	memo       map[string]map[string]struct{}
	visiting   map[string]bool
}

func (r *resolver) resolve(roleID string) map[string]struct{} {
	if cached, ok := r.memo[roleID]; ok {
		return cached
	}
	if r.visiting[roleID] {
		// Composition cycle: treat the in-progress role as empty to break it.
		return map[string]struct{}{}
	}
	r.visiting[roleID] = true

	eff := toSet(r.direct[roleID])
	for _, c := range sortByOrdinal(r.components[roleID]) {
		comp := r.resolve(c.ComponentRoleID)
		switch c.Operator {
		case domain.CompositionUnion:
			for p := range comp {
				eff[p] = struct{}{}
			}
		case domain.CompositionIntersect:
			for p := range eff {
				if _, ok := comp[p]; !ok {
					delete(eff, p)
				}
			}
		case domain.CompositionExclude:
			for p := range comp {
				delete(eff, p)
			}
		}
	}

	delete(r.visiting, roleID)
	r.memo[roleID] = eff
	return eff
}

// expandInheritance returns the transitive closure of set over the implication
// DAG, with its own visited guard against inheritance cycles.
func (r *resolver) expandInheritance(set map[string]struct{}) map[string]struct{} {
	out := map[string]struct{}{}
	var walk func(string)
	walk = func(p string) {
		if _, seen := out[p]; seen {
			return
		}
		out[p] = struct{}{}
		for _, implied := range r.implied[p] {
			walk(implied)
		}
	}
	for p := range set {
		walk(p)
	}
	return out
}

func toSet(ids []string) map[string]struct{} {
	s := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		s[id] = struct{}{}
	}
	return s
}

func sortByOrdinal(cs []domain.RoleComponent) []domain.RoleComponent {
	out := make([]domain.RoleComponent, len(cs))
	copy(out, cs)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Ordinal < out[j].Ordinal })
	return out
}

func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

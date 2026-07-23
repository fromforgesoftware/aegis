package domain

import (
	"fmt"
	"sort"
	"strings"
)

// CatalogDocument declares one resource type's complete authz vocabulary —
// its permissions (with the implication tree) and its SYSTEM roles (with the
// composition tree) — as provisioning data. Aegis reconciles the stored
// catalog to the document: rows the catalog manages that leave the document
// are pruned. Permission and role references inside the document may be bare
// ("read", "reader") or fully qualified ("strategies.read"); both normalize
// to the slug ids "<resourceType>.<verb>" / "<resourceType>.<name>".
type CatalogDocument struct {
	ResourceType string                           `json:"resourceType"`
	Permissions  map[string]CatalogPermissionSpec `json:"permissions"`
	Roles        map[string]CatalogRoleSpec       `json:"roles"`
	// Force names the specific managed rows this document may prune
	// DESTRUCTIVELY — a role pruned despite live bindings, or a permission
	// pruned despite attachments from roles outside the catalog. Scoped by id
	// (bare or qualified) rather than a blanket flag so a leftover override
	// can't silently disable prune safety for everything; every apply that
	// carries force entries is logged loudly.
	Force []string `json:"force,omitempty"`
}

// Forces reports whether the document authorizes a destructive prune of the
// given slug id.
func (d CatalogDocument) Forces(id string) bool {
	for _, ref := range d.Force {
		if d.ResourceType+"."+d.Normalize(ref) == id {
			return true
		}
	}
	return false
}

// CatalogPermissionSpec declares one permission. The "create" verb is special
// by convention: creation is checked against the PARENT workspace (the child
// doesn't exist yet), and every workspace member holds every create permission
// on it — aegis's equivalent of Zanzibar's `permission create_x = member`,
// built into the projection rather than declared per catalog.
type CatalogPermissionSpec struct {
	Implies     []string `json:"implies,omitempty"`
	Description string   `json:"description,omitempty"`
}

type CatalogRoleSpec struct {
	Permissions []string `json:"permissions,omitempty"`
	Composes    []string `json:"composes,omitempty"`
	Description string   `json:"description,omitempty"`
}

// PermissionID returns the slug id for a verb of this catalog's type.
func (d CatalogDocument) PermissionID(verb string) string {
	return d.ResourceType + "." + verb
}

// RoleID returns the slug id for a role name of this catalog's type.
func (d CatalogDocument) RoleID(name string) string {
	return d.ResourceType + "." + name
}

// Normalize strips this catalog's "<resourceType>." prefix so references may
// be written bare or fully qualified.
func (d CatalogDocument) Normalize(ref string) string {
	return strings.TrimPrefix(ref, d.ResourceType+".")
}

// Validate reports the first structural problem: bad identifiers, dangling
// references, self-references, or cycles in either tree.
func (d CatalogDocument) Validate() error {
	if d.ResourceType == "" {
		return fmt.Errorf("catalog: resourceType is required")
	}
	if strings.ContainsAny(d.ResourceType, ". \t\n") {
		return fmt.Errorf("catalog %q: resourceType cannot contain '.' or whitespace", d.ResourceType)
	}
	if len(d.Permissions) == 0 {
		return fmt.Errorf("catalog %q: at least one permission is required", d.ResourceType)
	}
	for verb, spec := range d.Permissions {
		if verb == "" || strings.ContainsAny(verb, ". \t\n") {
			return fmt.Errorf("catalog %q: invalid permission verb %q", d.ResourceType, verb)
		}
		for _, ref := range spec.Implies {
			implied := d.Normalize(ref)
			if implied == verb {
				return fmt.Errorf("catalog %q: permission %q implies itself", d.ResourceType, verb)
			}
			if _, ok := d.Permissions[implied]; !ok {
				return fmt.Errorf("catalog %q: permission %q implies unknown permission %q", d.ResourceType, verb, ref)
			}
		}
	}
	for name, spec := range d.Roles {
		if name == "" || strings.ContainsAny(name, ". \t\n") {
			return fmt.Errorf("catalog %q: invalid role name %q", d.ResourceType, name)
		}
		// Role slugs and permission slugs share the "<type>.<name>" shape; a
		// role named like a verb would mint an id indistinguishable from the
		// permission's to a human reader.
		if _, ok := d.Permissions[name]; ok {
			return fmt.Errorf("catalog %q: role %q collides with the permission verb %q", d.ResourceType, name, name)
		}
		if len(spec.Permissions) == 0 && len(spec.Composes) == 0 {
			return fmt.Errorf("catalog %q: role %q grants nothing", d.ResourceType, name)
		}
		for _, ref := range spec.Permissions {
			if _, ok := d.Permissions[d.Normalize(ref)]; !ok {
				return fmt.Errorf("catalog %q: role %q references unknown permission %q", d.ResourceType, name, ref)
			}
		}
		for _, ref := range spec.Composes {
			comp := d.Normalize(ref)
			if comp == name {
				return fmt.Errorf("catalog %q: role %q composes itself", d.ResourceType, name)
			}
			if _, ok := d.Roles[comp]; !ok {
				return fmt.Errorf("catalog %q: role %q composes unknown role %q", d.ResourceType, name, ref)
			}
		}
	}
	for _, ref := range d.Force {
		if ref == "" || strings.ContainsAny(d.Normalize(ref), ". \t\n") {
			return fmt.Errorf("catalog %q: invalid force entry %q (name a role or permission of this catalog)", d.ResourceType, ref)
		}
	}
	if cycle := findCycle(keysOf(d.Permissions), func(v string) []string {
		return normalizeAll(d, d.Permissions[v].Implies)
	}); cycle != "" {
		return fmt.Errorf("catalog %q: permission implication cycle through %q", d.ResourceType, cycle)
	}
	if cycle := findCycle(keysOf(d.Roles), func(n string) []string {
		return normalizeAll(d, d.Roles[n].Composes)
	}); cycle != "" {
		return fmt.Errorf("catalog %q: role composition cycle through %q", d.ResourceType, cycle)
	}
	return nil
}

func normalizeAll(d CatalogDocument, refs []string) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, d.Normalize(r))
	}
	return out
}

func keysOf[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys) // deterministic error reporting
	return keys
}

// findCycle DFS-colors the graph and returns a node on the first cycle found,
// or "" when the graph is a DAG.
func findCycle(nodes []string, edges func(string) []string) string {
	const (
		white = 0
		grey  = 1
		black = 2
	)
	color := make(map[string]int, len(nodes))
	var visit func(string) string
	visit = func(n string) string {
		color[n] = grey
		for _, next := range edges(n) {
			switch color[next] {
			case grey:
				return next
			case white:
				if c := visit(next); c != "" {
					return c
				}
			}
		}
		color[n] = black
		return ""
	}
	for _, n := range nodes {
		if color[n] == white {
			if c := visit(n); c != "" {
				return c
			}
		}
	}
	return ""
}

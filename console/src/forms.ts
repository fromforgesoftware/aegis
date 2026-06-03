// Pure payload builders for the admin create forms. Copied verbatim from the
// forge host (features/console/application/forms.ts) so this remote owns its
// bespoke screens with no host-internal imports — the host file is the source
// of these attribute shapes the aegis backend expects.

export interface RoleForm {
	realmId: string;
	name: string;
	resourceType: string;
	kind: string;
}

// roleAttributes maps the builder form + selected permission ids to the role
// create attributes. The backend reads the synthetic `permissions` attribute.
export function roleAttributes(f: RoleForm, permissionIds: string[]): Record<string, unknown> {
	return {
		realmId: f.realmId,
		name: f.name,
		resourceType: f.resourceType,
		kind: f.kind,
		permissions: permissionIds,
	};
}

export interface BindingForm {
	resourceId: string;
	roleId: string;
	subjectType: string;
	subjectId: string;
}

export function bindingAttributes(f: BindingForm): Record<string, unknown> {
	return {
		resourceId: f.resourceId,
		roleId: f.roleId,
		subjectType: f.subjectType,
		subjectId: f.subjectId,
	};
}

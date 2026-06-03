import { ShieldCheck } from '@lucide/vue';
import type { ForgeConsolePlugin, ForgeConsolePage } from '@fromforgesoftware/forge-console-plugin';
import {
	ResourceListView,
	ResourceCreateForm,
	ActionForm,
} from '@fromforgesoftware/forge-console-plugin/ui';
import RoleBuilder from './components/RoleBuilder.vue';
import BindingForm from './components/BindingForm.vue';
import AegisOverview from './components/AegisOverview.vue';

// PluginContext is what the forge-console-plugin loader passes to a remote
// module's default-export factory. apiBase is resolved at RUNTIME from the
// backend /apps descriptor (descriptor.apiBase), not at build time — this
// module.js is built once, before any deployment knows its gateway base.
export interface PluginContext {
	apiBase: string;
}

// The Aegis console plugin: an overview dashboard plus the admin surface over
// the JSON:API admin API — list pages via the generic renderer, plus the
// custom-role builder and binding editor.
function list(apiBase: string, type: string, name: string, columns: string[]): ForgeConsolePage {
	return {
		path: type,
		name,
		component: ResourceListView,
		props: { apiBase, type, title: name, columns },
	};
}

// aegisPlugin builds the ForgeConsolePlugin for a given apiBase. In the forge
// host this used to call apiBaseFor('aegis') at construction; in the remote
// module the apiBase is injected by the loader via the factory below.
export function aegisPlugin(apiBase: string): ForgeConsolePlugin {
	return {
		serviceId: 'aegis',
		type: 'app',
		title: 'Aegis',
		basePath: '/aegis',
		apiBase,
		icon: ShieldCheck,
		order: 1,
		pages: [
			{ path: 'overview', name: 'Overview', component: AegisOverview, props: { apiBase } },

			list(apiBase, 'realms', 'Realms', ['name', 'displayName']),
			list(apiBase, 'organizations', 'Organizations', ['name', 'slug', 'status']),
			list(apiBase, 'invitations', 'Invitations', ['email', 'status', 'roleId']),
			list(apiBase, 'session-states', 'Sessions', ['accountId', 'currentShard', 'region']),
			list(apiBase, 'service-accounts', 'Service accounts', ['name', 'clientId', 'lastUsedAt']),
			list(apiBase, 'roles', 'Roles', ['name', 'resourceType', 'kind']),
			list(apiBase, 'permissions', 'Permissions', ['resourceType', 'verb']),
			list(apiBase, 'resources', 'Resources', ['type', 'parentId', 'visibility']),
			list(apiBase, 'bindings', 'Bindings', ['resourceId', 'roleId', 'subjectType', 'subjectId']),
			list(apiBase, 'clients', 'OIDC clients', ['name', 'type']),
			list(apiBase, 'audit-events', 'Audit log', ['action', 'actorId', 'resourceType']),
			{
				path: 'realms/new',
				name: 'New realm',
				component: ResourceCreateForm,
				props: {
					apiBase,
					type: 'realms',
					title: 'New realm',
					fields: [
						{ name: 'name', label: 'Name', required: true },
						{ name: 'displayName', label: 'Display name' },
					],
				},
			},
			{
				path: 'organizations/new',
				name: 'New organization',
				component: ResourceCreateForm,
				props: {
					apiBase,
					type: 'organizations',
					title: 'New organization',
					fields: [
						{ name: 'realmId', label: 'Realm ID', required: true },
						{ name: 'name', label: 'Name', required: true },
						{ name: 'slug', label: 'Slug', required: true },
					],
				},
			},
			{ path: 'roles/new', name: 'New role', component: RoleBuilder, props: { apiBase } },
			{ path: 'bindings/new', name: 'New binding', component: BindingForm, props: { apiBase } },
			{
				path: 'accounts/ban',
				name: 'Ban account',
				component: ActionForm,
				props: {
					apiBase,
					path: '/api/accounts/ban',
					type: 'accountBans',
					title: 'Ban account',
					submitLabel: 'Ban',
					fields: [
						{ name: 'accountId', label: 'Account ID', required: true },
						{ name: 'reason', label: 'Reason' },
						{ name: 'until', label: 'Until (RFC3339, blank = permanent)' },
					],
				},
			},
			{
				path: 'accounts/unban',
				name: 'Unban account',
				component: ActionForm,
				props: {
					apiBase,
					path: '/api/accounts/unban',
					type: 'accountBans',
					title: 'Unban account',
					submitLabel: 'Unban',
					fields: [{ name: 'accountId', label: 'Account ID', required: true }],
				},
			},
			{
				path: 'accounts/merge',
				name: 'Merge accounts',
				component: ActionForm,
				props: {
					apiBase,
					path: '/api/accounts/merge',
					type: 'accountMerges',
					title: 'Merge accounts',
					submitLabel: 'Merge',
					fields: [
						{ name: 'sourceId', label: 'Source account ID', required: true },
						{ name: 'targetId', label: 'Target (survivor) account ID', required: true },
					],
				},
			},
			{
				path: 'risk-policy',
				name: 'Risk policy',
				component: ActionForm,
				props: {
					apiBase,
					path: '/api/realm-risk-policies',
					type: 'realmRiskPolicies',
					title: 'Realm risk policy',
					submitLabel: 'Save',
					fields: [
						{ name: 'realmId', label: 'Realm ID', required: true },
						{ name: 'newIpWeight', label: 'New IP weight', type: 'number' },
						{ name: 'newDeviceWeight', label: 'New device weight', type: 'number' },
						{ name: 'failureWeight', label: 'Failure weight', type: 'number' },
						{ name: 'stepUpThreshold', label: 'Step-up threshold', type: 'number' },
						{ name: 'denyThreshold', label: 'Deny threshold', type: 'number' },
					],
				},
			},
			{
				path: 'mfa-policy',
				name: 'MFA policy',
				component: ActionForm,
				props: {
					apiBase,
					path: '/api/realm-acr-policies',
					type: 'realmAcrPolicies',
					title: 'Realm MFA policy',
					submitLabel: 'Save',
					fields: [
						{ name: 'realmId', label: 'Realm ID', required: true },
						{ name: 'mfaRequired', label: 'Require MFA', type: 'checkbox' },
						{ name: 'requiredAcr', label: 'Required ACR (e.g. aal2)' },
					],
				},
			},
			{
				path: 'service-accounts/new',
				name: 'New service account',
				component: ResourceCreateForm,
				props: {
					apiBase,
					type: 'service-accounts',
					title: 'New service account',
					fields: [
						{ name: 'realmId', label: 'Realm ID', required: true },
						{ name: 'name', label: 'Name', required: true },
					],
				},
			},
		],
	};
}

// Default export: the apiBase-injection FACTORY. A remote module.js is built
// once, before apiBase is known — apiBase only exists at runtime, from the
// backend /apps descriptor. The forge-console-plugin loader calls this factory
// with `{ apiBase: descriptor.apiBase }` and registers the returned plugin.
//
// The factory is also tolerant of being called with no context (the loader's
// zero-arg path) — it falls back to the gateway proxy base the host uses by
// convention so the descriptor fallback in loadConsolePlugins still applies.
export default function createPlugin(ctx?: PluginContext): ForgeConsolePlugin {
	return aegisPlugin(ctx?.apiBase ?? '/api/proxy/aegis');
}

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import {
	StatCard,
	StatCardGroup,
	DonutChart,
	BarChart,
	Alert,
	AlertDescription,
	Card,
	CardHeader,
	CardTitle,
	CardContent,
	type DonutSegment,
	type BarChartData,
} from '@fromforgesoftware/vue-kit';
import {
	ResourceListView,
	fetchCollection,
	type JsonApiResource,
} from '@fromforgesoftware/forge-console-plugin/ui';

// Aegis overview: headline KPIs above the fold, composition + comparison
// charts in the middle, recent audit activity for drill-down. Counts are
// derived client-side from the JSON:API collections (no stats endpoint yet).
//
// Remote-decoupled: JSON:API + ResourceListView come from forge-console-plugin
// /ui (NOT the host). The gateway authenticates the console user via the
// session cookie (`credentials: 'include'` inside the helper), so there is no
// bearer token to thread through — pass null.
const props = defineProps<{ apiBase: string }>();

const router = useRouter();
const loading = ref(true);
const error = ref<string | null>(null);

const realms = ref<JsonApiResource[]>([]);
const organizations = ref<JsonApiResource[]>([]);
const roles = ref<JsonApiResource[]>([]);
const permissions = ref<JsonApiResource[]>([]);
const bindings = ref<JsonApiResource[]>([]);
const clients = ref<JsonApiResource[]>([]);
const resources = ref<JsonApiResource[]>([]);

const CHART = [
	'var(--color-chart-1)',
	'var(--color-chart-2)',
	'var(--color-chart-3)',
	'var(--color-chart-4)',
	'var(--color-chart-5)',
];

async function load<T extends JsonApiResource>(type: string): Promise<T[]> {
	try {
		return (await fetchCollection(props.apiBase, type, null)) as T[];
	} catch {
		return [];
	}
}

function groupCounts(rows: JsonApiResource[], key: string): Map<string, number> {
	const out = new Map<string, number>();
	for (const r of rows) {
		const v = String(r.attributes[key] ?? '—');
		out.set(v, (out.get(v) ?? 0) + 1);
	}
	return out;
}

const bindingsBySubject = computed<DonutSegment[]>(() => {
	const m = groupCounts(bindings.value, 'subjectType');
	const label = (k: string) => (k === 'ACTOR_SET' ? 'Groups' : k === 'ACCOUNT' ? 'Accounts' : k);
	return [...m.entries()].map(([k, v], i) => ({
		label: label(k),
		value: v,
		color: CHART[i % CHART.length],
	}));
});

const resourcesByVisibility = computed<DonutSegment[]>(() => {
	const m = groupCounts(resources.value, 'visibility');
	return [...m.entries()].map(([k, v], i) => ({
		label: k.charAt(0) + k.slice(1).toLowerCase(),
		value: v,
		color: CHART[i % CHART.length],
	}));
});

const rolesByType = computed<BarChartData>(() => {
	const m = groupCounts(roles.value, 'resourceType');
	const entries = [...m.entries()].sort((a, b) => b[1] - a[1]).slice(0, 8);
	return {
		categories: entries.map(([k]) => k),
		datasets: [{ label: 'Roles', data: entries.map(([, v]) => v), color: 'var(--color-chart-1)' }],
	};
});

const kpis = computed(() => [
	{
		label: 'Realms',
		value: realms.value.length,
		description: 'Identity domains',
		to: '/aegis/realms',
	},
	{
		label: 'Organizations',
		value: organizations.value.length,
		description: 'Tenants',
		to: '/aegis/organizations',
	},
	{ label: 'Roles', value: roles.value.length, description: 'Access roles', to: '/aegis/roles' },
	{
		label: 'Permissions',
		value: permissions.value.length,
		description: 'Catalog',
		to: '/aegis/permissions',
	},
	{
		label: 'Bindings',
		value: bindings.value.length,
		description: 'ACL grants',
		to: '/aegis/bindings',
	},
	{
		label: 'OIDC clients',
		value: clients.value.length,
		description: 'Applications',
		to: '/aegis/clients',
	},
]);

onMounted(async () => {
	try {
		[
			realms.value,
			organizations.value,
			roles.value,
			permissions.value,
			bindings.value,
			clients.value,
			resources.value,
		] = await Promise.all([
			load('realms'),
			load('organizations'),
			load('roles'),
			load('permissions'),
			load('bindings'),
			load('clients'),
			load('resources'),
		]);
	} catch (e) {
		error.value = e instanceof Error ? e.message : 'request failed';
	} finally {
		loading.value = false;
	}
});
</script>

<template>
	<section class="space-y-6">
		<header>
			<h1 class="text-xl font-semibold">Aegis</h1>
			<p class="text-sm text-muted-foreground">Identity, access &amp; tenancy</p>
		</header>

		<Alert v-if="error" variant="destructive">
			<AlertDescription>Failed to load overview: {{ error }}</AlertDescription>
		</Alert>

		<StatCardGroup :columns="3">
			<StatCard
				v-for="k in kpis"
				:key="k.label"
				:label="k.label"
				:value="k.value"
				:description="k.description"
				:loading="loading"
				interactive
				@click="router.push(k.to)"
			/>
		</StatCardGroup>

		<div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
			<Card>
				<CardHeader>
					<CardTitle class="text-sm">Bindings by subject</CardTitle>
				</CardHeader>
				<CardContent>
					<DonutChart
						:segments="bindingsBySubject"
						:center-text="String(bindings.length)"
						center-label="Bindings"
						variant="legend-bottom"
					/>
				</CardContent>
			</Card>
			<Card>
				<CardHeader>
					<CardTitle class="text-sm">Resources by visibility</CardTitle>
				</CardHeader>
				<CardContent>
					<DonutChart
						:segments="resourcesByVisibility"
						:center-text="String(resources.length)"
						center-label="Resources"
						variant="legend-bottom"
					/>
				</CardContent>
			</Card>
			<Card>
				<CardHeader>
					<CardTitle class="text-sm">Roles by resource type</CardTitle>
				</CardHeader>
				<CardContent>
					<BarChart
						:data="rolesByType"
						:height="240"
						horizontal
						aria-label="Roles per resource type"
					/>
				</CardContent>
			</Card>
		</div>

		<Card>
			<CardHeader>
				<CardTitle class="text-sm">Recent activity</CardTitle>
			</CardHeader>
			<CardContent>
				<ResourceListView
					:api-base="apiBase"
					type="audit-events"
					title=""
					:columns="['action', 'actorId', 'resourceType']"
				/>
			</CardContent>
		</Card>
	</section>
</template>

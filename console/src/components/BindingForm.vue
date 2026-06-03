<script setup lang="ts">
import { reactive, ref } from 'vue';
import {
	Alert,
	AlertDescription,
	Button,
	FormField,
	Input,
	Select,
	SelectTrigger,
	SelectValue,
	SelectContent,
	SelectItem,
} from '@fromforgesoftware/vue-kit';
import { createResource } from '@fromforgesoftware/forge-console-plugin/ui';
import { bindingAttributes } from '../forms';

// Binding editor: grant a subject (account or group) a role on a resource.
//
// Remote-decoupled: createResource comes from forge-console-plugin /ui (cookie
// auth, so token is null); bindingAttributes is inlined in ./forms.
const props = defineProps<{ apiBase: string }>();

const form = reactive({ resourceId: '', roleId: '', subjectType: 'ACCOUNT', subjectId: '' });
const error = ref<string | null>(null);
const createdId = ref<string | null>(null);
const submitting = ref(false);

async function submit() {
	error.value = null;
	if (!form.resourceId || !form.roleId || !form.subjectId) {
		error.value = 'resource, role and subject are required';
		return;
	}
	submitting.value = true;
	try {
		const created = await createResource(props.apiBase, 'bindings', bindingAttributes(form), null);
		createdId.value = created.id;
	} catch (e) {
		error.value = e instanceof Error ? e.message : 'create failed';
	} finally {
		submitting.value = false;
	}
}
</script>

<template>
	<section class="mx-auto w-full max-w-2xl space-y-4">
		<h1 class="text-xl font-semibold">New binding</h1>

		<Alert v-if="createdId" variant="success">
			<AlertDescription>
				Granted binding <span class="font-mono">{{ createdId }}</span
				>.
			</AlertDescription>
		</Alert>

		<form class="space-y-4" @submit.prevent="submit">
			<FormField label="Resource ID" for="binding-resource">
				<Input id="binding-resource" v-model="form.resourceId" />
			</FormField>
			<FormField label="Role ID" for="binding-role">
				<Input id="binding-role" v-model="form.roleId" />
			</FormField>
			<FormField label="Subject type" for="binding-subject-type">
				<Select
					:model-value="form.subjectType"
					@update:model-value="(v) => (form.subjectType = v as string)"
				>
					<SelectTrigger id="binding-subject-type"><SelectValue /></SelectTrigger>
					<SelectContent>
						<SelectItem value="ACCOUNT">ACCOUNT</SelectItem>
						<SelectItem value="ACTOR_SET">ACTOR_SET (group)</SelectItem>
					</SelectContent>
				</Select>
			</FormField>
			<FormField label="Subject ID" for="binding-subject">
				<Input id="binding-subject" v-model="form.subjectId" />
			</FormField>

			<Alert v-if="error" variant="destructive">
				<AlertDescription>{{ error }}</AlertDescription>
			</Alert>

			<div class="flex justify-end">
				<Button type="submit" :disabled="submitting">
					{{ submitting ? 'Saving…' : 'Grant binding' }}
				</Button>
			</div>
		</form>
	</section>
</template>

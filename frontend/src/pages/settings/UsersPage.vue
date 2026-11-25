<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { t } from '../../i18n'
import BaseButton from '../../components/BaseButton.vue'
import FormField from '../../components/FormField.vue'
import { listProjects } from '../../services/projects'
import {
  listUsers,
  createUser,
  updateUserProjects,
  deleteUser,
  type User
} from '../../services/auth'
import type { Project } from '../../mock/data'

const users = ref<User[]>([])
const projects = ref<Project[]>([])
const email = ref('')
const password = ref('')
const selectedProjects = ref<string[]>([])
const error = ref('')
const success = ref('')
const busy = ref(false)

onMounted(async () => {
  await reload()
})

async function reload() {
  error.value = ''
  try {
    const [loadedUsers, loadedProjects] = await Promise.all([listUsers(), listProjects()])
    users.value = loadedUsers
    projects.value = loadedProjects
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

function assignments(user: User): string[] {
  return user.projectIds || []
}

async function handleCreate() {
  error.value = ''
  success.value = ''
  if (!email.value.trim() || password.value.length < 8) {
    error.value = t('Enter an email and a password of at least 8 characters.')
    return
  }
  busy.value = true
  try {
    await createUser(email.value.trim(), password.value, selectedProjects.value)
    email.value = ''
    password.value = ''
    selectedProjects.value = []
    success.value = t('User created.')
    await reload()
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  } finally {
    busy.value = false
  }
}

async function handleSaveAssignments(user: User, event: Event) {
  const checked = Array.from(
    (event.target as HTMLElement).closest('td')?.querySelectorAll<HTMLInputElement>('input[type=checkbox]:checked') || []
  ).map((input) => input.value)
  try {
    await updateUserProjects(user.id, checked)
    success.value = t('Projects updated for {{email}}.', { email: user.email })
    await reload()
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function handleDelete(user: User) {
  if (!confirm(t('Delete account {{email}}?', { email: user.email }))) return
  try {
    await deleteUser(user.id)
    await reload()
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

</script>

<template>
  <div class="users-page">
    <h2 class="section-title">{{ t('Users & project access') }}</h2>
    <p class="section-hint">{{ t('Sub-users can only manage the projects you assign to them.') }}</p>

    <div v-if="error" class="users-error">{{ error }}</div>
    <div v-if="success" class="users-success">{{ success }}</div>

    <section class="users-card">
      <h3 class="card-title">{{ t('Accounts') }}</h3>
      <div v-if="users.length === 0" class="users-empty">{{ t('No accounts yet.') }}</div>
      <div v-else class="users-table-wrap">
        <table class="users-table">
          <thead>
            <tr>
              <th>{{ t('auth.email') }}</th>
              <th>{{ t('Role') }}</th>
              <th>{{ t('Assigned projects') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="user in users" :key="user.id">
              <td class="mono">{{ user.email }}</td>
              <td>{{ t(user.role) }}</td>
              <td>
                <div v-if="user.role === 'user'" class="assignment-cell">
                  <label
                    v-for="project in projects"
                    :key="project.id"
                    class="assignment-option"
                  >
                    <input type="checkbox" :value="project.id" :checked="assignments(user).includes(project.id)" />
                    {{ project.name }}
                  </label>
                  <BaseButton variant="outline" size="sm" type="button" @click="handleSaveAssignments(user, $event)">
                    {{ t('Save access') }}
                  </BaseButton>
                </div>
                <span v-else class="muted">—</span>
              </td>
              <td>
                <BaseButton
                  v-if="user.role === 'user'"
                  variant="danger"
                  size="sm"
                  type="button"
                  @click="handleDelete(user)"
                >
                  {{ t('Delete') }}
                </BaseButton>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="users-card">
      <h3 class="card-title">{{ t('Add sub-user') }}</h3>
      <div class="create-grid">
        <FormField :label="t('auth.email')" id="new-user-email">
          <input id="new-user-email" v-model="email" type="email" :placeholder="t('name@company.com')" />
        </FormField>
        <FormField :label="t('auth.password')" id="new-user-password">
          <input id="new-user-password" v-model="password" type="password" :placeholder="t('At least 8 characters')" />
        </FormField>
      </div>
      <div class="project-options">
        <label v-for="project in projects" :key="project.id" class="assignment-option">
          <input v-model="selectedProjects" type="checkbox" :value="project.id" />
          {{ project.name }}
        </label>
      </div>
      <BaseButton variant="primary" type="button" :disabled="busy" @click="handleCreate">
        {{ busy ? t('Creating…') : t('Create user') }}
      </BaseButton>
    </section>

  </div>
</template>

<style scoped>
.users-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-24);
}

.section-title {
  font-size: var(--font-size-lg);
  margin: 0;
}

.section-hint,
.muted {
  color: var(--color-text-muted);
  font-size: var(--font-size-sm);
  margin: 0;
}

.users-error,
.users-success {
  padding: var(--space-8) var(--space-12);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
}

.users-error {
  border: 1px solid var(--color-danger);
  color: var(--color-danger);
}

.users-success {
  border: 1px solid var(--color-success);
  color: var(--color-success);
}

.users-card {
  padding: var(--space-24);
  background-color: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  display: flex;
  flex-direction: column;
  gap: var(--space-16);
}

.card-title {
  font-size: var(--font-size-base);
  margin: 0;
}

.users-empty {
  color: var(--color-text-muted);
  font-size: var(--font-size-sm);
}

.users-table-wrap {
  overflow-x: auto;
}

.users-table {
  width: 100%;
  min-width: 560px;
  border-collapse: collapse;
  font-size: var(--font-size-sm);
}

.users-table th,
.users-table td {
  padding: var(--space-12);
  border-bottom: 1px solid var(--color-border);
  text-align: left;
  vertical-align: top;
}

.mono {
  font-family: var(--font-mono);
}

.assignment-cell,
.project-options {
  display: flex;
  flex-direction: column;
  gap: var(--space-8);
}

.assignment-option,
.guest-toggle {
  display: flex;
  align-items: center;
  gap: var(--space-8);
  font-size: var(--font-size-sm);
}

.create-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--space-16);
}

@media (max-width: 600px) {
  .create-grid {
    grid-template-columns: 1fr;
  }
}
</style>

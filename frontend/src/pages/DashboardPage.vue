<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import BaseButton from '../components/BaseButton.vue'
import BaseDialog from '../components/BaseDialog.vue'
import FormField from '../components/FormField.vue'
import EmptyState from '../components/EmptyState.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import { mockStore, type Project } from '../mock/data'
import { listProjects, createProject, updateProject, deleteProject } from '../services/projects'
import { t } from '../i18n'
import { isAdmin } from '../services/auth'

const router = useRouter()
const isDialogOpen = ref(false)
const isEditDialogOpen = ref(false)
const isConfirmDeleteOpen = ref(false)
const projectToDelete = ref<string>('')
const isSubmitting = ref(false)
const isLoading = ref(true)
const projectDescription = ref('')
const nameError = ref('')
const projectName = ref<string>('')
const editingProjectId = ref<string>('')

onMounted(async () => {
  try {
    const list = await listProjects()
    mockStore.projects = list
  } catch (err) {
    console.error('Failed to load projects:', err)
  } finally {
    isLoading.value = false
  }
})

const formatInput = (event: Event): void => {
  const target = event.target as HTMLInputElement
  const formatted = target.value.replace(/\s+/g, '_')
  projectName.value = formatted
  target.value = formatted
}

function openNewProjectDialog() {
  projectName.value = ''
  projectDescription.value = ''
  nameError.value = ''
  isDialogOpen.value = true
}

async function handleCreateProject() {
  if (!projectName.value.trim()) {
    nameError.value = t('Project name is required.')
    return
  }
  isSubmitting.value = true
  try {
    const created = await createProject(projectName.value.trim(), projectDescription.value.trim())
    isDialogOpen.value = false
    router.push(`/projects/${created.id}`)
  } catch (err: any) {
    nameError.value = err?.message || t('Failed to create project.')
  } finally {
    isSubmitting.value = false
  }
}

function openEditProjectDialog(event: Event, project: Project) {
  event.stopPropagation()
  editingProjectId.value = project.id
  projectName.value = project.name
  projectDescription.value = project.description
  nameError.value = ''
  isEditDialogOpen.value = true
}

async function handleUpdateProject() {
  if (!projectName.value.trim()) {
    nameError.value = t('Project name is required.')
    return
  }
  isSubmitting.value = true
  try {
    await updateProject(editingProjectId.value, projectName.value.trim(), projectDescription.value.trim())
    isEditDialogOpen.value = false
  } catch (err: any) {
    nameError.value = err?.message || t('Failed to update project.')
  } finally {
    isSubmitting.value = false
  }
}

function promptDeleteProject(event: Event, id: string) {
  event.stopPropagation()
  projectToDelete.value = id
  isConfirmDeleteOpen.value = true
}

async function confirmDeleteProject() {
  if (!projectToDelete.value) return
  isConfirmDeleteOpen.value = false
  try {
    await deleteProject(projectToDelete.value)
  } catch (err: any) {
    alert(err?.message || t('Failed to delete project.'))
  } finally {
    projectToDelete.value = ''
  }
}

function navigateToProject(id: string) {
  router.push(`/projects/${id}`)
}

// Kept out of the template: literal "{{...}}" inside a mustache would end it early.
const fileCountLabel = (count: number) => t('{{count}} files', { count })

</script>

<template>
  <div class="page-container">
    <!-- Standard Page Header -->
    <header class="page-header">
      <div class="header-titles">
        <h1 class="page-title">{{ t('dashboard.title') }}</h1>
        <p class="page-description">{{ t('Manage RAG knowledge bases, retrieval settings, and personas.') }}</p>
      </div>
      <div class="header-actions">
        <BaseButton v-if="isAdmin" variant="primary" @click="openNewProjectDialog">{{ t('dashboard.new') }}</BaseButton>
      </div>
    </header>

    <!-- Content Area -->
    <div class="page-content">
      <div v-if="isLoading" class="loading-state">
        {{ t('Loading projects...') }}
      </div>
      <div v-else-if="mockStore.projects.length > 0" class="projects-table-card">
        <table class="projects-table">
          <thead>
            <tr>
              <th scope="col" class="th-main">{{ t('Project') }}</th>
              <th scope="col">{{ t('Knowledge files') }}</th>
              <th scope="col" class="col-optional">{{ t('Persona') }}</th>
              <th scope="col" class="col-optional">{{ t('Last updated') }}</th>
              <th scope="col" class="th-action"><span class="sr-only">{{ t('Actions') }}</span></th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="project in mockStore.projects"
              :key="project.id"
              class="project-row"
              @click="navigateToProject(project.id)"
            >
              <td class="td-main">
                <div class="project-info">
                  <span class="project-name">{{ project.name }}</span>
                  <span class="project-desc">{{ project.description || t('No description provided.') }}</span>
                </div>
              </td>
              <td>
                <span class="badge-count">{{ fileCountLabel(project.knowledgeFilesCount) }}</span>
              </td>
              <td class="col-optional">
                <span class="persona-name">{{ project.personaName }}</span>
              </td>
              <td class="col-optional">
                <time class="updated-time">{{ project.lastUpdated }}</time>
              </td>
              <td class="td-action">
                <div class="actions-group">
                  <button
                    type="button"
                    class="action-icon-btn"
                    :title="t('Edit project')"
                    @click="openEditProjectDialog($event, project)"
                  >
                    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path>
                      <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path>
                    </svg>
                  </button>
                  <button
                    type="button"
                    class="action-icon-btn action-icon-btn--danger"
                    :title="t('Delete project')"
                    @click="promptDeleteProject($event, project.id)"
                  >
                    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <polyline points="3 6 5 6 21 6"></polyline>
                      <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
                    </svg>
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Empty State -->
      <EmptyState
        v-else
        :title="t('No projects yet')"
        :description="t('No projects are visible yet. Projects assigned to your account will appear here.')"
      />
    </div>

    <!-- New Project Dialog -->
    <BaseDialog
      :open="isDialogOpen"
      :title="t('Create new project')"
      @close="isDialogOpen = false"
    >
      <form id="new-project-form" @submit.prevent="handleCreateProject">
        <FormField
          :label="t('Project name')"
          id="project-name"
          required
          :error="nameError"
          @input="formatInput"
          :hint="t('A clear identifier for this retrieval workspace.')"
        >
          <input
            id="project-name"
            v-model="projectName"
            type="text"
            :placeholder="t('e.g. Legal_Contracts_Assistant')"
            autofocus
          />
        </FormField>

        <FormField
          :label="t('Description')"
          id="project-desc"
          :hint="t('Optional summary of the knowledge domain or purpose.')"
        >
          <textarea
            id="project-desc"
            v-model="projectDescription"
            :placeholder="t('Describe what documents will be indexed and who uses this...')"
          />
        </FormField>
      </form>

      <template #footer>
        <BaseButton variant="outline" type="button" :disabled="isSubmitting" @click="isDialogOpen = false">
          {{ t('common.cancel') }}
        </BaseButton>
        <BaseButton variant="primary" type="submit" form="new-project-form" :disabled="isSubmitting">
          {{ isSubmitting ? t('Creating...') : t('Create project') }}
        </BaseButton>
      </template>
    </BaseDialog>

    <!-- Edit Project Dialog -->
    <BaseDialog
      :open="isEditDialogOpen"
      :title="t('Edit project')"
      @close="isEditDialogOpen = false"
    >
      <form id="edit-project-form" @submit.prevent="handleUpdateProject">
        <FormField
          :label="t('Project name')"
          id="edit-project-name"
          required
          :error="nameError"
          @input="formatInput"
          :hint="t('A clear identifier for this retrieval workspace.')"
        >
          <input
            id="edit-project-name"
            v-model="projectName"
            type="text"
            :placeholder="t('e.g. Legal_Contracts_Assistant')"
            autofocus
          />
        </FormField>

        <FormField
          :label="t('Description')"
          id="edit-project-desc"
          :hint="t('Optional summary of the knowledge domain or purpose.')"
        >
          <textarea
            id="edit-project-desc"
            v-model="projectDescription"
            :placeholder="t('Describe what documents will be indexed and who uses this...')"
          />
        </FormField>
      </form>

      <template #footer>
        <BaseButton variant="outline" type="button" :disabled="isSubmitting" @click="isEditDialogOpen = false">
          {{ t('common.cancel') }}
        </BaseButton>
        <BaseButton variant="primary" type="submit" form="edit-project-form" :disabled="isSubmitting">
          {{ isSubmitting ? t('Saving...') : t('Save changes') }}
        </BaseButton>
      </template>
    </BaseDialog>

    <!-- Confirm Delete Modal -->
    <ConfirmModal
      :open="isConfirmDeleteOpen"
      :title="t('Delete Project')"
      :message="t('Are you sure you want to delete this project? All associated documents and embeddings will be permanently removed.')"
      :confirm-text="t('Delete project')"
      :cancel-text="t('common.cancel')"
      variant="danger"
      @confirm="confirmDeleteProject"
      @cancel="isConfirmDeleteOpen = false"
    />
  </div>
</template>

<style scoped>
.page-container {
  display: flex;
  flex-direction: column;
  min-height: 100%;
}

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-16);
  padding: var(--space-24) var(--space-32);
  background-color: var(--color-surface);
  border-bottom: 1px solid var(--color-border);
}

.header-titles {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.page-title {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-medium);
  color: var(--color-text);
  letter-spacing: -0.01em;
}

.page-description {
  font-size: var(--font-size-base);
  color: var(--color-text-muted);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: var(--space-8);
}

.page-content {
  flex: 1;
  padding: var(--space-32);
}

.loading-state {
  color: var(--color-text-muted);
  font-size: var(--font-size-base);
}

.projects-table-card {
  background-color: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  overflow-x: auto;
}

.projects-table {
  width: 100%;
  min-width: 560px;
  border-collapse: collapse;
  text-align: left;
}

@media (max-width: 900px) {
  .page-header,
  .page-content {
    padding-left: var(--space-16);
    padding-right: var(--space-16);
  }
}

@media (max-width: 700px) {
  .col-optional {
    display: none;
  }

  .projects-table {
    min-width: 480px;
  }
}

@media (max-width: 600px) {
  .page-header {
    flex-wrap: wrap;
  }

  .page-content {
    padding: var(--space-16);
  }
}

.projects-table th {
  padding: var(--space-12) var(--space-16);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--color-text-muted);
  background-color: var(--color-surface-subtle);
  user-select: none;
}

.projects-table td {
  padding: var(--space-16);
  border-bottom: 1px solid var(--color-border);
  vertical-align: middle;
  font-size: var(--font-size-base);
  color: var(--color-text);
}

.project-row {
  cursor: pointer;
  transition: background-color var(--transition-fast);
}

.project-row:hover {
  background-color: var(--color-surface-subtle);
}

.project-row:last-child td {
  border-bottom: none;
}

.project-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.td-main {
  flex-direction: column;
  gap: 2px;
}

.project-name {
  font-weight: var(--font-weight-medium);
  color: var(--color-text);
}

.project-desc {
  font-size: var(--font-size-sm);
  color: var(--color-text-muted);
  max-width: 440px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.badge-count {
  font-family: var(--font-mono);
  font-size: var(--font-size-sm);
  color: var(--color-text-muted);
}

.persona-name {
  color: var(--color-text);
  font-size: var(--font-size-sm);
}

.updated-time {
  font-family: var(--font-mono);
  font-size: var(--font-size-sm);
  color: var(--color-text-muted);
}

.td-action {
  text-align: right;
  width: 80px;
}

.actions-group {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: var(--space-8);
}

.action-icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--color-border);
  background-color: var(--color-surface-subtle);
  color: var(--color-text-muted);
  cursor: pointer;
  transition: all var(--transition-fast);
}

.action-icon-btn:hover {
  color: var(--color-text);
  border-color: var(--color-border-strong);
  background-color: var(--color-surface);
}

.action-icon-btn--danger:hover {
  color: var(--color-danger);
  border-color: var(--color-danger);
  background-color: var(--color-danger-subtle);
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  border: 0;
}
</style>

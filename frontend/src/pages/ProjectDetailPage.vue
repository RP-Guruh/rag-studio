<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Events } from '@wailsio/runtime'
import BaseButton from '../components/BaseButton.vue'
import EvaluationPanel from './EvaluationPanel.vue'
import TabBar from '../components/TabBar.vue'
import FormField from '../components/FormField.vue'
import StatusBadge from '../components/StatusBadge.vue'
import EmptyState from '../components/EmptyState.vue'
import {
  mockStore,
  type PersonaConfig
} from '../mock/data'
import {
  getProject,
  saveProjectPersona,
  importDocuments,
  deleteDocument,
  trainProject,
  getTrainingStatus,
  reindexDocument,
  cancelTraining,
  exportProject,
  listPromptTemplates,
  type PromptTemplate,
  type TrainingProgress
} from '../services/projects'
import { isWailsRuntime, waitForWailsRuntime } from '../services/settings'
import { isAdmin } from '../services/auth'

const route = useRoute()
const router = useRouter()

const projectId = computed(() => route.params.id as string)
const project = computed(() => mockStore.projects.find((p) => p.id === projectId.value))

const activeTab = ref('knowledge')
const tabs = [
  { id: 'knowledge', label: 'Knowledge Base' },
  { id: 'persona', label: 'Persona' },
  { id: 'evaluation', label: 'Evaluation' }
]

let unsubscribeProgress: (() => void) | undefined
let unsubscribeFinished: (() => void) | undefined
let trainingPollInterval: ReturnType<typeof setInterval> | null = null
let isPollingStatus = false

onMounted(async () => {
  await loadProject()
  try {
    promptTemplates.value = await listPromptTemplates()
  } catch {
    promptTemplates.value = []
  }

  const runtimeReady = await waitForWailsRuntime()
  if (runtimeReady && isWailsRuntime()) {
    unsubscribeProgress = Events.On('training:progress', (event) => {
      applyTrainingProgress(event.data as TrainingProgress)
    })
    unsubscribeFinished = Events.On('training:finished', (event) => {
      void handleTrainingFinished(event.data as TrainingProgress)
    })

    try {
      const status = await getTrainingStatus(projectId.value)
      if (status) {
        applyTrainingProgress(status)
      }
    } catch (err) {
      console.error('Failed to load training status:', err)
      startPollingStatus()
    }
  } else {
    startPollingStatus()
  }
})

onUnmounted(() => {
  unsubscribeProgress?.()
  unsubscribeFinished?.()
  if (trainingPollInterval) {
    clearInterval(trainingPollInterval)
    trainingPollInterval = null
  }
  if (isPollingStatus) {
    isPollingStatus = false
  }
})

function startPollingStatus() {
  if (isPollingStatus) return
  isPollingStatus = true
  trainingPollInterval = setInterval(async () => {
    try {
      const status = await getTrainingStatus(projectId.value)
      if (status) {
        applyTrainingProgress(status)
        if (status.phase === 'finished') {
          clearInterval(trainingPollInterval!)
          trainingPollInterval = null
          isPollingStatus = false
        }
      }
    } catch (err) {
      console.error('Training status polling error:', err)
    }
  }, 5000) // poll every 5 seconds
}

async function loadProject() {
  try {
    const p = await getProject(projectId.value)
    if (p) {
      const idx = mockStore.projects.findIndex((item) => item.id === p.id)
      if (idx !== -1) {
        mockStore.projects[idx] = p
      } else {
        mockStore.projects.push(p)
      }
    }
  } catch (err) {
    console.error('Failed to load project details:', err)
  }
}

// Training state
const isTraining = computed(() => project.value?.isTraining || false)
const trainingProgress = computed(() => project.value?.trainingProgress || 0)
const trainingStep = computed(() => project.value?.trainingStep || '')
const lastIndexedAt = computed(() => project.value?.lastIndexedAt)
const trainingErrors = ref<string[]>([])
const totalChunks = computed(() =>
  project.value?.files.reduce((sum, file) => sum + (file.chunksCount || 0), 0) || 0
)

const hasUntrainedFiles = computed(() => {
  if (!project.value) return false
  return project.value.files.some(
    (f) => f.status === 'Untrained' || f.status === 'Processing' || f.status === 'Failed'
  )
})

const hasStaleChunks = computed(() =>
  project.value?.files.some((f) => f.status === 'Chunked') || false
)

const isReadOnly = computed(() => !isAdmin.value)
const promptTemplates = ref<PromptTemplate[]>([])

function applyPromptTemplate(name: string) {
  const template = promptTemplates.value.find((item) => item.name === name)
  if (template) personaForm.value.instructions = template.instructions
}

function applyTrainingProgress(progress: TrainingProgress) {
  const proj = mockStore.projects.find((item) => item.id === progress.projectId)
  if (!proj) return

  const running = progress.phase === 'starting' || progress.phase === 'processing'
  proj.isTraining = running
  proj.trainingProgress = progress.percent
  proj.trainingStep = progress.message
  trainingErrors.value = progress.errors || []
}

async function handleTrainingFinished(progress: TrainingProgress) {
  const proj = mockStore.projects.find((item) => item.id === progress.projectId)
  if (proj && progress.finishedAt) {
    proj.lastIndexedAt = new Date(progress.finishedAt)
      .toISOString()
      .replace('T', ' ')
      .slice(0, 16)
  }
  trainingErrors.value = progress.errors || []
  await loadProject()
}

function refreshTrainingStatus() {
  try {
    getTrainingStatus(projectId.value).then((status) => {
      if (status) {
        applyTrainingProgress(status)
      }
    })
  } catch (err) {
    console.error('Failed to refresh training status:', err)
  }
}

async function handleExportProject() {
  try {
    const payload = await exportProject(projectId.value)
    if (!payload) return
    const blob = new Blob([payload], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `${project.value?.name || 'project'}.json`
    link.click()
    URL.revokeObjectURL(url)
  } catch (err: any) {
    trainingErrors.value = [err?.message || 'Failed to export project.']
  }
}

async function handleReindexDocument(fileId: string) {
  if (isReadOnly.value) return
  try {
    await reindexDocument(projectId.value, fileId)
    await loadProject()
  } catch (err: any) {
    trainingErrors.value = [err?.message || 'Failed to re-index document.']
  }
}

async function handleCancelTraining() {
  if (isReadOnly.value) return
  try {
    await cancelTraining(projectId.value)
  } catch (err: any) {
    trainingErrors.value = [err?.message || 'Failed to stop training.']
  }
}

async function handleTrainKnowledge() {
  if (isReadOnly.value || !project.value || isTraining.value || project.value.files.length === 0) return

  trainingErrors.value = []
  try {
    const progress = await trainProject(projectId.value)
    if (progress) {
      applyTrainingProgress(progress)
    }
  } catch (err: any) {
    trainingErrors.value = [err?.message || 'Failed to start training.']
  }
}

// Persona local form state
const personaForm = ref<PersonaConfig>({
  name: '',
  role: '',
  instructions: '',
  tone: 'Neutral',
  answerLanguage: 'English',
  strictKnowledge: true
})

const isSavingPersona = ref(false)
const saveMessage = ref('')
const fileInputRef = ref<HTMLInputElement | null>(null)
const isDragging = ref(false)
const isUploading = ref(false)
const uploadMessage = ref('')
const uploadErrors = ref<string[]>([])

watch(
  project,
  (p) => {
    if (p) {
      personaForm.value = { ...p.persona }
    }
  },
  { immediate: true }
)

async function importFiles(files: File[]) {
  if (isReadOnly.value || files.length === 0 || isUploading.value) return

  isUploading.value = true
  uploadMessage.value = ''
  uploadErrors.value = []

  try {
    const result = await importDocuments(projectId.value, files)
    if (result.imported.length > 0) {
      uploadMessage.value =
        result.imported.length === 1
          ? '1 document uploaded.'
          : `${result.imported.length} documents uploaded.`
    }
    uploadErrors.value = result.errors
  } catch (err: any) {
    uploadErrors.value = [err?.message || 'Upload failed.']
  } finally {
    isUploading.value = false
  }
}

function handleFileSelect(event: Event) {
  const target = event.target as HTMLInputElement
  const files = target.files ? Array.from(target.files) : []
  target.value = ''
  void importFiles(files)
}

function handleDrop(e: DragEvent) {
  isDragging.value = false
  const files = e.dataTransfer?.files ? Array.from(e.dataTransfer.files) : []
  void importFiles(files)
}

function triggerFileInput() {
  if (isReadOnly.value || isUploading.value) return
  fileInputRef.value?.click()
}

async function handleRemoveFile(fileId: string) {
  if (isReadOnly.value) return
  try {
    await deleteDocument(projectId.value, fileId)
  } catch (err: any) {
    alert(err?.message || 'Failed to delete document.')
  }
}

async function handleSavePersona() {
  if (isReadOnly.value) return
  isSavingPersona.value = true
  try {
    await saveProjectPersona(projectId.value, personaForm.value)
    saveMessage.value = 'Persona settings saved successfully.'
    setTimeout(() => {
      saveMessage.value = ''
    }, 3000)
  } catch (err: any) {
    alert(err?.message || 'Failed to save persona.')
  } finally {
    isSavingPersona.value = false
  }
}
</script>

<template>
  <div v-if="project" class="project-detail-container">
    <!-- Header with Back Link -->
    <header class="detail-header">
      <div class="breadcrumb-row">
        <router-link to="/dashboard" class="back-link">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="15 18 9 12 15 6"></polyline>
          </svg>
          <span>Projects</span>
        </router-link>
      </div>

      <div class="title-row">
        <div class="title-details">
          <h1 class="project-name">{{ project.name }}</h1>
          <p class="project-description">{{ project.description || 'No description provided.' }}</p>
        </div>
        <BaseButton v-if="!isReadOnly" variant="outline" size="sm" type="button" @click="handleExportProject">
          Export project
        </BaseButton>
      </div>
    </header>

    <div v-if="isReadOnly" class="readonly-banner">
      Read-only access — only the administrator can modify projects and settings.
    </div>

    <!-- Main Tabs & Body -->
    <div class="detail-content">
      <TabBar v-model="activeTab" :tabs="tabs" />

      <!-- Tab 1: Knowledge -->
      <section v-if="activeTab === 'knowledge'" class="tab-panel">
        <div
          v-if="!isReadOnly"
          class="upload-dropzone"
          :class="{ 'upload-dropzone--drag': isDragging, 'upload-dropzone--busy': isUploading }"
          @dragover.prevent="isDragging = true"
          @dragleave.prevent="isDragging = false"
          @drop.prevent="handleDrop"
          @click="triggerFileInput"
        >
          <input
            ref="fileInputRef"
            type="file"
            multiple
            accept=".pdf,.md,.txt,.doc,.docx"
            class="hidden-file-input"
            @change="handleFileSelect"
          />

          <div class="dropzone-content">
            <div class="dropzone-icon">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path>
                <polyline points="17 8 12 3 7 8"></polyline>
                <line x1="12" y1="3" x2="12" y2="15"></line>
              </svg>
            </div>
            <div class="dropzone-text">
              <span class="primary-text">
                {{ isUploading ? 'Uploading and saving documents...' : 'Click to upload or drag files here' }}
              </span>
              <span class="secondary-text">Supported document formats: PDF, Markdown (.md), Plain Text (.txt), and Word (.doc, .docx) — up to 50 MB each</span>
            </div>
          </div>
        </div>

        <!-- Upload feedback -->
        <p v-if="uploadMessage" class="upload-feedback upload-feedback--success">{{ uploadMessage }}</p>
        <ul v-if="uploadErrors.length > 0" class="upload-feedback upload-feedback--error">
          <li v-for="message in uploadErrors" :key="message">{{ message }}</li>
        </ul>

        <!-- Training Progress Banner -->
        <div v-if="isTraining" class="training-banner">
          <div class="training-banner-top">
            <div class="training-status-label">
              <span class="spinner-dot"></span>
              <span class="training-title">Building RAG Knowledge Base...</span>
            </div>
            <span class="training-percent">{{ trainingProgress }}%</span>
            <span class="training-action-text" @click="refreshTrainingStatus">Refresh</span>
          </div>
          <div class="progress-track">
            <div class="progress-bar" :style="{ width: `${trainingProgress}%` }"></div>
          </div>
          <div class="training-step-text">{{ trainingStep }}</div>
          <BaseButton variant="secondary" size="sm" type="button" class="training-cancel" @click="handleCancelTraining">
            Stop training
          </BaseButton>
        </div>

        <!-- Training failures -->
        <div v-if="trainingErrors.length > 0" class="training-errors">
          <p class="training-errors-title">
            {{ isTraining ? 'Documents that failed so far:' : 'Some documents could not be processed:' }}
          </p>
          <ul class="training-errors-list">
            <li v-for="message in trainingErrors" :key="message">{{ message }}</li>
          </ul>
        </div>

        <!-- Files Header & Train Action -->
        <div class="files-header-row">
          <div class="files-header-left">
            <div class="files-heading-title">
              <h3 class="files-title">Uploaded documents</h3>
              <span class="files-count-badge">{{ project.files.length }} files</span>
              <span v-if="totalChunks > 0" class="chunks-total-badge">{{ totalChunks }} indexed chunks</span>
            </div>
            <span v-if="lastIndexedAt" class="indexed-at-hint">Last trained: {{ lastIndexedAt }}</span>
          </div>

          <div class="files-header-actions">
            <p v-if="hasStaleChunks" class="stale-hint">
              Vectors are not stored yet. Re-index to enable chat.
            </p>
            <BaseButton
              v-if="!isReadOnly && project.files.length > 0"
              variant="primary"
              :disabled="isTraining"
              class="train-btn"
              @click="handleTrainKnowledge"
            >
              <svg v-if="!isTraining" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polygon points="5 3 19 12 5 21 5 3"></polygon>
              </svg>
              <span>{{ isTraining ? 'Training in progress...' : (hasUntrainedFiles || hasStaleChunks ? 'Re-index knowledge base' : 'Train knowledge base') }}</span>
            </BaseButton>
          </div>
        </div>

        <!-- Document Table -->
        <div v-if="project.files.length > 0" class="files-table-card">
          <table class="files-table">
            <colgroup>
              <col class="col-name" />
              <col class="col-type col-optional" />
              <col class="col-size" />
              <col class="col-date col-optional" />
              <col class="col-chunks" />
              <col class="col-status" />
              <col class="col-actions" />
            </colgroup>
            <thead>
              <tr>
                <th scope="col" class="th-name">File name</th>
                <th scope="col" class="col-optional">Type</th>
                <th scope="col">Size</th>
                <th scope="col" class="col-optional">Added date</th>
                <th scope="col">Chunks</th>
                <th scope="col">Status</th>
                <th scope="col" class="th-actions"><span class="sr-only">Actions</span></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="file in project.files" :key="file.id" class="file-row">
                <td class="td-name">
                  <div class="file-cell">
                    <svg class="file-type-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path>
                      <polyline points="14 2 14 8 20 8"></polyline>
                      <line x1="16" y1="13" x2="8" y2="13"></line>
                      <line x1="16" y1="17" x2="8" y2="17"></line>
                      <polyline points="10 9 9 9 8 9"></polyline>
                    </svg>
                    <span class="file-name-text" :title="file.name">{{ file.name }}</span>
                  </div>
                </td>
                <td class="col-optional">
                  <span class="file-type-tag">{{ file.type }}</span>
                </td>
                <td>
                  <span class="file-size-text font-mono">{{ file.size }}</span>
                </td>
                <td class="col-optional">
                  <time class="file-date font-mono">{{ file.addedDate }}</time>
                </td>
                <td>
                  <span v-if="file.chunksCount" class="chunks-count-badge">{{ file.chunksCount }} chunks</span>
                  <span v-else class="chunks-empty-dash">—</span>
                </td>
                <td>
                  <StatusBadge :status="file.status" :title="file.errorMessage || undefined" />
                </td>
                <td class="td-actions">
                  <button
                    v-if="!isReadOnly"
                    type="button"
                    class="file-reindex-btn"
                    title="Re-index this document"
                    :disabled="isTraining"
                    @click="handleReindexDocument(file.id)"
                  >
                    ↻
                  </button>
                  <button
                    v-if="!isReadOnly"
                    type="button"
                    class="file-delete-btn"
                    title="Remove file"
                    :disabled="isTraining"
                    @click="handleRemoveFile(file.id)"
                  >
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <line x1="18" y1="6" x2="6" y2="18"></line>
                      <line x1="6" y1="6" x2="18" y2="18"></line>
                    </svg>
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <p v-else class="files-empty-note">
          No documents yet. Upload files above to build this project's knowledge base.
        </p>
      </section>

      <!-- Tab 2: Persona -->
      <section v-else-if="activeTab === 'persona'" class="tab-panel persona-panel">
        <form class="persona-form" @submit.prevent="handleSavePersona">
          <FormField
            label="Persona name"
            id="persona-name"
            required
            hint="Display name for this assistant personality."
          >
            <input
              id="persona-name"
              v-model="personaForm.name"
              type="text"
              :disabled="isReadOnly"
              placeholder="e.g. Legal Compliance Assistant"
            />
          </FormField>

          <FormField
            label="Role"
            id="persona-role"
            hint="One-line definition of this persona's specialty."
          >
            <input
              id="persona-role"
              v-model="personaForm.role"
              type="text"
              :disabled="isReadOnly"
              placeholder="e.g. Corporate contract reviewer"
            />
          </FormField>

          <FormField
            v-if="promptTemplates.length > 0"
            label="Instruction template"
            id="persona-template"
            hint="Start from a saved template, then edit as needed."
          >
            <select id="persona-template" :disabled="isReadOnly" @change="applyPromptTemplate(($event.target as HTMLSelectElement).value)">
              <option value="">Choose a template…</option>
              <option v-for="template in promptTemplates" :key="template.id" :value="template.name">
                {{ template.name }}
              </option>
            </select>
          </FormField>

          <FormField
            label="Instructions"
            id="persona-instructions"
            hint="System prompt guiding formatting, boundaries, and tone."
          >
            <textarea
              id="persona-instructions"
              v-model="personaForm.instructions"
              rows="5"
              :disabled="isReadOnly"
              placeholder="Provide exact guidelines on how the persona should respond..."
            />
          </FormField>

          <div class="form-row-grid">
            <FormField label="Tone" id="persona-tone">
              <select id="persona-tone" :disabled="isReadOnly" v-model="personaForm.tone">
                <option value="Neutral">Neutral</option>
                <option value="Friendly">Friendly</option>
                <option value="Formal">Formal</option>
                <option value="Concise">Concise</option>
              </select>
            </FormField>

            <FormField label="Answer language" id="persona-lang">
              <select id="persona-lang" :disabled="isReadOnly" v-model="personaForm.answerLanguage">
                <option value="auto">Auto (match question)</option>
                <option value="English">English</option>
                <option value="Indonesian">Indonesian</option>
                <option value="Spanish">Spanish</option>
                <option value="German">German</option>
                <option value="French">French</option>
                <option value="Japanese">Japanese</option>
              </select>
            </FormField>
          </div>

          <div class="toggle-control-row">
            <label class="toggle-checkbox-label">
              <input
                type="checkbox"
                v-model="personaForm.strictKnowledge"
                class="toggle-checkbox"
              />
              <span class="toggle-label-text">Strict knowledge grounding</span>
            </label>
            <span class="toggle-hint">
              If enabled, the assistant will explicitly state that it does not know if the answer cannot be found in the knowledge files.
            </span>
          </div>

          <div class="form-actions-row">
            <BaseButton v-if="!isReadOnly" variant="primary" type="submit" :disabled="isSavingPersona">
              {{ isSavingPersona ? 'Saving...' : 'Save persona' }}
            </BaseButton>
            <span v-if="saveMessage" class="save-success-msg">{{ saveMessage }}</span>
          </div>
        </form>
      </section>

      <section v-else-if="activeTab === 'evaluation'" class="tab-panel">
        <EvaluationPanel :project-id="projectId" :readonly="isReadOnly" />
      </section>
    </div>
  </div>

  <div v-else class="page-container">
    <EmptyState
      title="Project not found"
      description="The requested RAG project does not exist or was removed."
    >
      <template #action>
        <BaseButton variant="primary" @click="router.push('/dashboard')">
          Back to projects
        </BaseButton>
      </template>
    </EmptyState>
  </div>
</template>

<style scoped>
.project-detail-container {
  display: flex;
  flex-direction: column;
  min-height: 100%;
}

.detail-header {
  display: flex;
  flex-direction: column;
  gap: var(--space-8);
  padding: var(--space-20) var(--space-32);
  background-color: var(--color-surface);
  border-bottom: 1px solid var(--color-border);
}

.back-link {
  display: inline-flex;
  align-items: center;
  gap: var(--space-8);
  font-size: var(--font-size-sm);
  color: var(--color-text-muted);
  text-decoration: none;
  font-weight: var(--font-weight-medium);
  transition: color var(--transition-fast);
}

.back-link:hover {
  color: var(--color-text);
}

.title-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
}

.project-name {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-medium);
  color: var(--color-text);
  letter-spacing: -0.01em;
}

.project-description {
  font-size: var(--font-size-base);
  color: var(--color-text-muted);
  margin-top: 2px;
}

.readonly-banner {
  padding: var(--space-12) var(--space-32);
  background-color: var(--color-accent-subtle);
  border-bottom: 1px solid var(--color-border);
  color: var(--color-text-muted);
  font-size: var(--font-size-sm);
}

.detail-content {
  flex: 1;
  padding: var(--space-24) var(--space-32);
}

.tab-panel {
  margin-top: var(--space-24);
}

/* Upload Dropzone */
.upload-dropzone {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-32);
  border: 2px dashed var(--color-border);
  border-radius: var(--radius-md);
  background-color: var(--color-surface);
  cursor: pointer;
  transition: all var(--transition-fast);
}

.upload-dropzone:hover,
.upload-dropzone--drag {
  border-color: var(--color-accent);
  background-color: var(--color-surface-subtle);
}

.upload-dropzone--busy {
  cursor: progress;
  opacity: 0.75;
  pointer-events: none;
}

.upload-feedback {
  margin-top: var(--space-12);
  font-size: var(--font-size-sm);
  line-height: var(--line-height-normal);
}

.upload-feedback--success {
  color: var(--color-success);
}

.upload-feedback--error {
  margin: var(--space-12) 0 0;
  padding-left: var(--space-20);
  color: var(--color-danger);
  list-style: disc;
}

.hidden-file-input {
  display: none;
}

.dropzone-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-8);
  text-align: center;
}

.dropzone-icon {
  color: var(--color-text-muted);
}

.dropzone-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.primary-text {
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-medium);
  color: var(--color-text);
}

.secondary-text {
  font-size: var(--font-size-sm);
  color: var(--color-text-muted);
}

/* Training Banner */
.training-banner {
  margin-top: var(--space-20);
  padding: var(--space-16);
  background-color: var(--color-surface-subtle);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  display: flex;
  flex-direction: column;
  gap: var(--space-8);
}

.training-banner-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.training-status-label {
  display: flex;
  align-items: center;
  gap: var(--space-8);
}

.spinner-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background-color: var(--color-accent);
  animation: pulse 1.5s infinite;
}

@keyframes pulse {
  0% { transform: scale(0.95); opacity: 0.6; }
  50% { transform: scale(1.2); opacity: 1; }
  100% { transform: scale(0.95); opacity: 0.6; }
}

.training-title {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--color-text);
}

.training-percent {
  font-family: var(--font-mono);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--color-accent);
}

.progress-track {
  width: 100%;
  height: 6px;
  background-color: var(--color-border);
  border-radius: 3px;
  overflow: hidden;
}

.progress-bar {
  height: 100%;
  background-color: var(--color-accent);
  transition: width 0.3s ease;
}

.training-step-text {
  font-size: 12px;
  color: var(--color-text-muted);
  font-family: var(--font-mono);
}

.training-errors {
  margin-top: var(--space-12);
  padding: var(--space-12) var(--space-16);
  border: 1px solid rgba(220, 38, 38, 0.25);
  border-radius: var(--radius-sm);
  background-color: var(--color-danger-subtle);
  color: var(--color-danger);
  font-size: var(--font-size-sm);
}

.training-errors-title {
  font-weight: var(--font-weight-medium);
  margin-bottom: var(--space-4);
}

.training-cancel {
  align-self: flex-start;
  margin-top: var(--space-8);
}

.file-reindex-btn {
  width: 28px;
  height: 28px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--color-border);
  background-color: var(--color-surface-subtle);
  color: var(--color-text-muted);
  cursor: pointer;
  font-size: var(--font-size-md);
}

.file-reindex-btn:hover {
  color: var(--color-accent);
  border-color: var(--color-accent);
}

.training-errors-list {
  margin: 0;
  padding-left: var(--space-20);
  list-style: disc;
  line-height: var(--line-height-normal);
}

/* Files Header Row */
.files-header-row {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  justify-content: space-between;
  gap: var(--space-16);
  margin-top: var(--space-24);
  margin-bottom: var(--space-12);
}

.files-header-left {
  display: flex;
  flex-direction: column;
  gap: var(--space-8);
  min-width: 0;
}

.files-heading-title {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-12);
}

.files-title {
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-medium);
  color: var(--color-text);
  margin: 0;
}

.files-count-badge,
.chunks-total-badge {
  font-size: 12px;
  padding: 2px 8px;
  background-color: var(--color-surface-subtle);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  color: var(--color-text-muted);
  font-family: var(--font-mono);
}

.chunks-total-badge {
  color: var(--color-accent);
  border-color: rgba(37, 99, 235, 0.2);
}

.files-header-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: var(--space-12);
}

.stale-hint {
  font-size: var(--font-size-sm);
  color: var(--color-text-muted);
  max-width: 260px;
}

.indexed-at-hint {
  font-size: 12px;
  color: var(--color-text-muted);
  font-family: var(--font-mono);
}

.files-empty-note {
  margin-top: var(--space-16);
  text-align: center;
  font-size: var(--font-size-base);
  color: var(--color-text-muted);
}

/* Files Table */
.files-table-card {
  background-color: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  overflow-x: auto;
}

.files-table {
  width: 100%;
  min-width: 620px;
  table-layout: fixed;
  border-collapse: collapse;
  text-align: left;
}

@media (max-width: 700px) {
  .files-table .col-optional {
    display: none;
  }

  .files-table {
    min-width: 480px;
  }
}

@media (max-width: 600px) {
  .detail-content {
    padding: var(--space-16);
  }

  .files-header-actions {
    justify-content: flex-start;
  }
}

.col-name    { width: auto; }
.col-type    { width: 88px; }
.col-size    { width: 88px; }
.col-date    { width: 112px; }
.col-chunks  { width: 100px; }
.col-status  { width: 124px; }
.col-actions { width: 56px; }

.files-table th {
  padding: var(--space-12) var(--space-16);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--color-text-muted);
  background-color: var(--color-surface-subtle);
  white-space: nowrap;
}

.files-table td {
  padding: var(--space-12) var(--space-16);
  border-bottom: 1px solid var(--color-border);
  vertical-align: middle;
  font-size: var(--font-size-base);
}

.file-row:last-child td {
  border-bottom: none;
}

.file-cell {
  display: flex;
  align-items: center;
  gap: var(--space-8);
  min-width: 0;
}

.file-type-icon {
  color: var(--color-text-muted);
  flex-shrink: 0;
}

.td-name {
  overflow: hidden;
}

.file-name-text {
  font-weight: var(--font-weight-medium);
  color: var(--color-text);
  display: block;
  max-width: 100%;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.file-type-tag {
  font-size: 12px;
  padding: 2px 6px;
  border-radius: var(--radius-sm);
  background-color: var(--color-surface-subtle);
  border: 1px solid var(--color-border);
  color: var(--color-text-muted);
}

.file-size-text,
.file-date {
  font-size: 12px;
  color: var(--color-text-muted);
}

.chunks-count-badge {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--color-text);
}

.chunks-empty-dash {
  color: var(--color-text-muted);
}

.font-mono {
  font-family: var(--font-mono);
  font-size: 12px;
}

.th-actions,
.td-actions {
  text-align: right;
}

.file-delete-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--color-border);
  background-color: var(--color-surface-subtle);
  color: var(--color-text-muted);
  cursor: pointer;
  transition: all var(--transition-fast);
}

.file-delete-btn:hover:not(:disabled) {
  background-color: var(--color-danger-subtle);
  color: var(--color-danger);
  border-color: var(--color-danger);
}

.file-delete-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

/* Persona Panel */
.persona-panel {
  max-width: 640px;
}

.persona-form {
  display: flex;
  flex-direction: column;
  gap: var(--space-24);
}

.form-row-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--space-16);
}

.toggle-control-row {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  padding: var(--space-16);
  background-color: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
}

.toggle-checkbox-label {
  display: flex;
  align-items: center;
  gap: var(--space-12);
  cursor: pointer;
}

.toggle-checkbox {
  width: 18px;
  height: 18px;
  accent-color: var(--color-accent);
  cursor: pointer;
}

.toggle-label-text {
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-medium);
  color: var(--color-text);
}

.toggle-hint {
  font-size: var(--font-size-sm);
  color: var(--color-text-muted);
  margin-left: 30px;
}

.form-actions-row {
  display: flex;
  align-items: center;
  gap: var(--space-16);
  margin-top: var(--space-8);
}

.save-success-msg {
  font-size: var(--font-size-sm);
  color: var(--color-success);
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

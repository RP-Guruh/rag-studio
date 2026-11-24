<script setup lang="ts">
import { onMounted, ref } from 'vue'
import BaseButton from '../../components/BaseButton.vue'
import FormField from '../../components/FormField.vue'
import ConfirmModal from '../../components/ConfirmModal.vue'
import {
  defaultSettings,
  errorText,
  loadSettings,
  saveDatabase,
  sslModes,
  testDatabase,
  backupSettings,
  type Database,
  type TestResult
} from '../../services/settings'

const form = ref<Database>({ ...defaultSettings.database })
const showPassword = ref(false)
const isLoading = ref(true)
const isSaving = ref(false)
const isTesting = ref(false)
const isConfirmOpen = ref(false)
const loadError = ref('')
const formError = ref('')
const testResult = ref<TestResult | null>(null)
const saveSuccess = ref(false)
let saveTimer: ReturnType<typeof setTimeout> | undefined

onMounted(async () => {
  try {
    const settings = await loadSettings()
    form.value = { ...settings.database, password: '', clearPassword: false }
  } catch (error) {
    loadError.value = errorText(error)
  } finally {
    isLoading.value = false
  }
})

async function handleBackup() {
  try {
    const data = await backupSettings()
    if (!data) return
    const bytes = Uint8Array.from(atob(data), (char) => char.charCodeAt(0))
    const url = URL.createObjectURL(new Blob([bytes], { type: 'application/x-sqlite3' }))
    const link = document.createElement('a')
    link.href = url
    link.download = 'rag-studio-settings.db'
    link.click()
    URL.revokeObjectURL(url)
  } catch (error) {
    formError.value = errorText(error)
  }
}

async function handleTestConnection() {
  isTesting.value = true
  testResult.value = null
  formError.value = ''
  try {
    testResult.value = await testDatabase({
      ...form.value,
      password: form.value.password || ''
    })
  } catch (error) {
    testResult.value = { success: false, message: errorText(error), latencyMs: 0 }
  } finally {
    isTesting.value = false
  }
}

function promptSave() {
  isConfirmOpen.value = true
}

async function executeSave() {
  isConfirmOpen.value = false
  isSaving.value = true
  formError.value = ''
  saveSuccess.value = false
  try {
    const { database: saved } = await saveDatabase({
      ...form.value,
      password: form.value.password || ''
    })
    form.value = { ...saved, password: '', clearPassword: false }
    saveSuccess.value = true
    if (saveTimer) clearTimeout(saveTimer)
    saveTimer = setTimeout(() => {
      saveSuccess.value = false
    }, 3000)
  } catch (error) {
    formError.value = errorText(error)
  } finally {
    isSaving.value = false
  }
}
</script>

<template>
  <div class="settings-section">
    <div class="section-header">
      <h2 class="section-title">Database</h2>
      <p class="section-desc">PostgreSQL instance hosting vector embeddings and metadata tables.</p>
    </div>

    <div class="info-callout">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="callout-icon">
        <circle cx="12" cy="12" r="10"></circle>
        <line x1="12" y1="16" x2="12" y2="12"></line>
        <line x1="12" y1="8" x2="12.01" y2="8"></line>
      </svg>
      <div class="callout-content">
        <p class="callout-text">
          The connected PostgreSQL database must have the <code class="font-mono">pgvector</code> extension enabled (<code class="font-mono">CREATE EXTENSION IF NOT EXISTS vector;</code>).
        </p>
      </div>
    </div>

    <div v-if="isLoading" class="state-note">Loading settings...</div>
    <div v-else-if="loadError" class="feedback-banner feedback-banner--error" role="alert">
      {{ loadError }}
    </div>

    <form v-else class="settings-form" @submit.prevent="promptSave">
      <div class="form-grid-2">
        <FormField label="Host" id="db-host" hint="IP address or domain.">
          <input
            id="db-host"
            v-model="form.host"
            type="text"
            placeholder="localhost"
          />
        </FormField>

        <FormField label="Port" id="db-port" hint="Default: 5432.">
          <input
            id="db-port"
            v-model.number="form.port"
            type="number"
            min="1"
            max="65535"
            placeholder="5432"
          />
        </FormField>
      </div>

      <div class="form-grid-2">
        <FormField label="Database name" id="db-name">
          <input
            id="db-name"
            v-model="form.databaseName"
            type="text"
            placeholder="rag_db"
          />
        </FormField>

        <FormField label="Username" id="db-user">
          <input
            id="db-user"
            v-model="form.username"
            type="text"
            placeholder="postgres"
          />
        </FormField>
      </div>

      <div class="form-grid-2">
        <FormField
          label="Password"
          id="db-password"
          :hint="form.passwordSet
            ? `A password is stored (${form.passwordHint}). Leave blank to keep it.`
            : 'Stored in the operating system keychain, not in the database.'"
        >
          <div class="input-with-action">
            <input
              id="db-password"
              v-model="form.password"
              :type="showPassword ? 'text' : 'password'"
              :placeholder="form.passwordSet ? form.passwordHint : 'Database password'"
              autocomplete="off"
            />
            <button
              type="button"
              class="toggle-mask-btn"
              @click="showPassword = !showPassword"
            >
              {{ showPassword ? 'Hide' : 'Show' }}
            </button>
          </div>
        </FormField>

        <FormField label="SSL mode" id="db-ssl">
          <select id="db-ssl" v-model="form.sslMode">
            <option v-for="mode in sslModes" :key="mode" :value="mode">{{ mode }}</option>
          </select>
        </FormField>
      </div>

      <FormField
        label="Schema"
        id="db-schema"
        hint="Schema that holds the pgvector tables."
      >
        <input
          id="db-schema"
          v-model="form.schemaName"
          type="text"
          placeholder="public"
        />
      </FormField>

      <div
        v-if="testResult"
        class="feedback-banner"
        :class="{ 'feedback-banner--success': testResult.success, 'feedback-banner--error': !testResult.success }"
        role="status"
      >
        {{ testResult.message }}
      </div>

      <div v-if="formError" class="feedback-banner feedback-banner--error" role="alert">
        {{ formError }}
      </div>

      <div class="actions-row">
        <BaseButton
          variant="outline"
          type="button"
          :disabled="isTesting || isSaving"
          @click="handleTestConnection"
        >
          {{ isTesting ? 'Testing...' : 'Test connection' }}
        </BaseButton>

        <BaseButton variant="primary" type="submit" :disabled="isSaving">
          {{ isSaving ? 'Saving...' : 'Save' }}
        </BaseButton>

        <BaseButton variant="ghost" type="button" @click="handleBackup">
          Backup settings
        </BaseButton>

        <span v-if="saveSuccess" class="save-indicator">Settings saved.</span>
      </div>
    </form>

    <ConfirmModal
      :open="isConfirmOpen"
      title="Save Database Settings"
      message="Are you sure you want to save these database settings?

This action will also execute PostgreSQL migrations to create the required RAG schema tables (projects, project_personas, documents, chunks) if they do not exist yet."
      confirm-text="Save & Migrate"
      cancel-text="Cancel"
      variant="primary"
      @confirm="executeSave"
      @cancel="isConfirmOpen = false"
    />
  </div>
</template>

<style scoped>
.settings-section {
  display: flex;
  flex-direction: column;
  gap: var(--space-24);
}

.section-header {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.section-title {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-medium);
  color: var(--color-text);
}

.section-desc {
  font-size: var(--font-size-base);
  color: var(--color-text-muted);
}

.state-note {
  font-size: var(--font-size-base);
  color: var(--color-text-muted);
}

.info-callout {
  display: flex;
  align-items: flex-start;
  gap: var(--space-12);
  padding: var(--space-16);
  background-color: var(--color-surface-subtle);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
}

.callout-icon {
  color: var(--color-text-muted);
  flex-shrink: 0;
  margin-top: 2px;
}

.callout-text {
  font-size: var(--font-size-sm);
  color: var(--color-text-muted);
  line-height: var(--line-height-normal);
}

.callout-text code {
  background-color: var(--color-surface);
  padding: 2px 4px;
  border-radius: 4px;
  border: 1px solid var(--color-border);
  color: var(--color-text);
}

.settings-form {
  display: flex;
  flex-direction: column;
  gap: var(--space-16);
}

.form-grid-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--space-16);
}

.input-with-action {
  position: relative;
  display: flex;
  align-items: center;
}

.input-with-action input {
  padding-right: 64px;
}

.toggle-mask-btn {
  position: absolute;
  right: var(--space-8);
  font-size: var(--font-size-sm);
  color: var(--color-text-muted);
  background: none;
  border: none;
  cursor: pointer;
  padding: var(--space-4) var(--space-8);
  border-radius: var(--radius-sm);
}

.toggle-mask-btn:hover {
  color: var(--color-text);
  background-color: var(--color-surface-subtle);
}

.actions-row {
  display: flex;
  align-items: center;
  gap: var(--space-12);
  margin-top: var(--space-8);
}

.feedback-banner {
  padding: var(--space-12) var(--space-16);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
}

.feedback-banner--success {
  background-color: var(--color-success-subtle);
  color: var(--color-success);
  border: 1px solid rgba(22, 163, 74, 0.2);
}

.feedback-banner--error {
  background-color: var(--color-danger-subtle);
  color: var(--color-danger);
  border: 1px solid rgba(220, 38, 38, 0.2);
}

.save-indicator {
  font-size: var(--font-size-sm);
  color: var(--color-success);
}

.font-mono {
  font-family: var(--font-mono);
}
</style>

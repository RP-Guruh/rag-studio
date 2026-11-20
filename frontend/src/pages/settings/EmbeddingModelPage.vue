<script setup lang="ts">
import { onMounted, ref } from 'vue'
import BaseButton from '../../components/BaseButton.vue'
import FormField from '../../components/FormField.vue'
import {
  defaultSettings,
  embeddingProviders,
  errorText,
  loadSettings,
  saveEmbeddingModel,
  testEmbeddingModel,
  type EmbeddingModel,
  type TestResult
} from '../../services/settings'

const form = ref<EmbeddingModel>({ ...defaultSettings.embeddingModel })
const showApiKey = ref(false)
const isLoading = ref(true)
const isSaving = ref(false)
const isTesting = ref(false)
const loadError = ref('')
const formError = ref('')
const testResult = ref<TestResult | null>(null)
const saveSuccess = ref(false)
let saveTimer: ReturnType<typeof setTimeout> | undefined

onMounted(async () => {
  try {
    const settings = await loadSettings()
    form.value = { ...settings.embeddingModel, apiKey: '', clearApiKey: false }
  } catch (error) {
    loadError.value = errorText(error)
  } finally {
    isLoading.value = false
  }
})

async function handleTestConnection() {
  isTesting.value = true
  testResult.value = null
  formError.value = ''
  try {
    testResult.value = await testEmbeddingModel({
      ...form.value,
      apiKey: form.value.apiKey?.trim() || ''
    })
  } catch (error) {
    testResult.value = { success: false, message: errorText(error), latencyMs: 0 }
  } finally {
    isTesting.value = false
  }
}

async function handleSave() {
  isSaving.value = true
  formError.value = ''
  saveSuccess.value = false
  try {
    const saved = await saveEmbeddingModel({
      ...form.value,
      apiKey: form.value.apiKey?.trim() || ''
    })
    form.value = { ...saved, apiKey: '', clearApiKey: false }
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
      <h2 class="section-title">Embedding model</h2>
      <p class="section-desc">Vector embedding model used to vectorize and index text chunks.</p>
    </div>

    <div class="warning-callout">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="callout-icon">
        <circle cx="12" cy="12" r="10"></circle>
        <line x1="12" y1="8" x2="12" y2="12"></line>
        <line x1="12" y1="16" x2="12.01" y2="16"></line>
      </svg>
      <div class="callout-content">
        <p class="callout-text">
          Changing the embedding model or dimension later requires re-indexing every project's knowledge documents.
        </p>
      </div>
    </div>

    <div v-if="isLoading" class="state-note">Loading settings...</div>
    <div v-else-if="loadError" class="feedback-banner feedback-banner--error" role="alert">
      {{ loadError }}
    </div>

    <form v-else class="settings-form" @submit.prevent="handleSave">
      <FormField label="Provider" id="embed-provider">
        <select id="embed-provider" v-model="form.provider">
          <option v-for="option in embeddingProviders" :key="option.value" :value="option.value">
            {{ option.label }}
          </option>
        </select>
      </FormField>

      <FormField
        label="Base URL"
        id="embed-base-url"
        hint="Embedding API service endpoint."
      >
        <input
          id="embed-base-url"
          v-model="form.baseUrl"
          type="text"
          placeholder="https://api.openai.com/v1"
        />
      </FormField>

      <FormField
        label="API key"
        id="embed-api-key"
        :hint="form.apiKeySet
          ? `A key is stored (${form.apiKeyHint}). Leave blank to keep it.`
          : 'Stored in the operating system keychain, not in the database.'"
      >
        <div class="input-with-action">
          <input
            id="embed-api-key"
            v-model="form.apiKey"
            :type="showApiKey ? 'text' : 'password'"
            :placeholder="form.apiKeySet ? form.apiKeyHint : 'sk-...'"
            autocomplete="off"
          />
          <button
            type="button"
            class="toggle-mask-btn"
            @click="showApiKey = !showApiKey"
          >
            {{ showApiKey ? 'Hide' : 'Show' }}
          </button>
        </div>
      </FormField>

      <div class="form-grid-2">
        <FormField
          label="Model name"
          id="embed-model-name"
          hint="Model identifier."
        >
          <input
            id="embed-model-name"
            v-model="form.modelName"
            type="text"
            placeholder="text-embedding-3-small"
          />
        </FormField>

        <FormField
          label="Vector dimension"
          id="embed-dimension"
          hint="Output float array size (e.g. 768, 1536, 3072)."
        >
          <input
            id="embed-dimension"
            v-model.number="form.vectorDimension"
            type="number"
            step="1"
            min="1"
            max="65536"
          />
        </FormField>
      </div>

      <div class="form-grid-2">
        <FormField
          label="Chunk max tokens"
          id="embed-chunk-max-tokens"
          hint="Target token count per chunk produced by the document chunker."
        >
          <input
            id="embed-chunk-max-tokens"
            v-model.number="form.chunkMaxTokens"
            type="number"
            step="1"
            min="64"
            max="8192"
          />
        </FormField>

        <FormField
          label="Chunker tokenizer"
          id="embed-tokenizer"
          hint="Hugging Face tokenizer used to count tokens (e.g. BAAI/bge-m3)."
        >
          <input
            id="embed-tokenizer"
            v-model="form.tokenizer"
            type="text"
            placeholder="BAAI/bge-m3"
          />
        </FormField>
      </div>

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

        <span v-if="saveSuccess" class="save-indicator">Settings saved.</span>
      </div>
    </form>
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

.warning-callout {
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
</style>

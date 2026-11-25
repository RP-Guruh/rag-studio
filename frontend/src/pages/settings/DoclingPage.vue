<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { t } from '../../i18n'
import BaseButton from '../../components/BaseButton.vue'
import FormField from '../../components/FormField.vue'
import {
  defaultSettings,
  doclingModes,
  errorText,
  loadSettings,
  saveDocling,
  testDocling,
  type DoclingConnection,
  type TestResult
} from '../../services/settings'

const form = ref<DoclingConnection>({ ...defaultSettings.docling })
const showApiKey = ref(false)
const isLoading = ref(true)
const isSaving = ref(false)
const isTesting = ref(false)
const loadError = ref('')
const formError = ref('')
const testResult = ref<TestResult | null>(null)
const saveSuccess = ref(false)
let saveTimer: ReturnType<typeof setTimeout> | undefined

const isLocal = computed(() => form.value.mode === 'local')

onMounted(async () => {
  try {
    const settings = await loadSettings()
    form.value = { ...settings.docling, apiKey: '', clearApiKey: false }
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
    testResult.value = await testDocling({
      ...form.value,
      apiKey: form.value.apiKey || ''
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
    const saved = await saveDocling({
      ...form.value,
      apiKey: form.value.apiKey || ''
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
      <h2 class="section-title">{{ t('Document processing') }}</h2>
      <p class="section-desc">{{ t('Docling converts uploaded PDF, Word, Markdown and text files into parsed documents before they are indexed.') }}</p>
    </div>

    <div class="info-callout">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="callout-icon">
        <circle cx="12" cy="12" r="10"></circle>
        <line x1="12" y1="16" x2="12" y2="12"></line>
        <line x1="12" y1="8" x2="12.01" y2="8"></line>
      </svg>
      <div class="callout-content">
        <p class="callout-text">
          {{ t('Docling runs as an HTTP service. Use') }} <strong>{{ t('Local') }}</strong> {{ t('when it runs on this computer (for example') }} <code class="font-mono">http://localhost:5001</code> {{ t('), and') }} <strong>{{ t('Serve') }}</strong> {{ t('when it runs on another machine.') }}
        </p>
      </div>
    </div>

    <div v-if="isLoading" class="state-note">{{ t('Loading settings...') }}</div>
    <div v-else-if="loadError" class="feedback-banner feedback-banner--error" role="alert">
      {{ loadError }}
    </div>

    <form v-else class="settings-form" @submit.prevent="handleSave">
      <FormField
        :label="t('Mode')"
        id="docling-mode"
        :hint="t('Local means the service runs on this computer; Serve means it runs on another machine.')"
      >
        <select id="docling-mode" v-model="form.mode">
          <option v-for="mode in doclingModes" :key="mode.value" :value="mode.value">
            {{ mode.label }}
          </option>
        </select>
      </FormField>

      <FormField
        :label="t('Base URL')"
        id="docling-base-url"
        :hint="isLocal
          ? t('Address of the Docling service on this computer, for example http://localhost:5001.')
          : t('Address of the Docling service on the other machine, for example http://docling.internal:5001.')"
      >
        <input
          id="docling-base-url"
          v-model="form.baseUrl"
          type="url"
          :placeholder="t('http://localhost:5001')"
        />
      </FormField>

      <div class="form-grid-2">
        <FormField
          :label="t('API key')"
          id="docling-api-key"
          :hint="form.apiKeySet
            ? t('A key is stored ({{hint}}). Leave blank to keep it.', { hint: form.apiKeyHint ?? '' })
            : t('Optional. Only needed when the server requires authentication.')"
        >
          <div class="input-with-action">
            <input
              id="docling-api-key"
              v-model="form.apiKey"
              :type="showApiKey ? 'text' : 'password'"
              :placeholder="form.apiKeySet ? form.apiKeyHint : t('Optional')"
              autocomplete="off"
            />
            <button
              type="button"
              class="toggle-mask-btn"
              @click="showApiKey = !showApiKey"
            >
              {{ showApiKey ? t('Hide') : t('Show') }}
            </button>
          </div>
        </FormField>

        <FormField
          :label="t('Timeout (seconds)')"
          id="docling-timeout"
          :hint="t('How long a single document may take. Large scans can take several minutes.')"
        >
          <input
            id="docling-timeout"
            v-model.number="form.timeoutSeconds"
            type="number"
            min="5"
            max="3600"
            step="1"
            placeholder="300"
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
          {{ isTesting ? t('Testing...') : t('Test connection') }}
        </BaseButton>

        <BaseButton variant="primary" type="submit" :disabled="isSaving">
          {{ isSaving ? t('Saving...') : t('common.save') }}
        </BaseButton>

        <span v-if="saveSuccess" class="save-indicator">{{ t('Settings saved.') }}</span>
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

.font-mono {
  font-family: var(--font-mono);
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

.toggle-mask-btn:hover:not(:disabled) {
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

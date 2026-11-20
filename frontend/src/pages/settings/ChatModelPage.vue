<script setup lang="ts">
import { onMounted, ref } from 'vue'
import BaseButton from '../../components/BaseButton.vue'
import FormField from '../../components/FormField.vue'
import {
  chatProviders,
  defaultSettings,
  errorText,
  loadSettings,
  saveChatModel,
  testChatModel,
  type ChatModel,
  type TestResult
} from '../../services/settings'

const form = ref<ChatModel>({ ...defaultSettings.chatModel })
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
    form.value = { ...settings.chatModel, apiKey: '', clearApiKey: false }
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
    testResult.value = await testChatModel({
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
    const saved = await saveChatModel({
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
      <h2 class="section-title">Chat model</h2>
      <p class="section-desc">Configure the primary LLM used to synthesize RAG answers.</p>
    </div>

    <div v-if="isLoading" class="state-note">Loading settings...</div>
    <div v-else-if="loadError" class="feedback-banner feedback-banner--error" role="alert">
      {{ loadError }}
    </div>

    <form v-else class="settings-form" @submit.prevent="handleSave">
      <FormField label="Provider" id="chat-provider">
        <select id="chat-provider" v-model="form.provider">
          <option v-for="option in chatProviders" :key="option.value" :value="option.value">
            {{ option.label }}
          </option>
        </select>
      </FormField>

      <FormField
        label="Base URL"
        id="chat-base-url"
        hint="Root endpoint of the LLM API service."
      >
        <input
          id="chat-base-url"
          v-model="form.baseUrl"
          type="text"
          placeholder="https://api.openai.com/v1"
        />
      </FormField>

      <FormField
        label="API key"
        id="chat-api-key"
        :hint="form.apiKeySet
          ? `A key is stored (${form.apiKeyHint}). Leave blank to keep it.`
          : 'Stored in the operating system keychain, not in the database.'"
      >
        <div class="input-with-action">
          <input
            id="chat-api-key"
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

      <FormField
        label="Model name"
        id="chat-model-name"
        hint="Exact identifier expected by the provider API."
      >
        <input
          id="chat-model-name"
          v-model="form.modelName"
          type="text"
          placeholder="e.g. gpt-4o, claude-3-5-sonnet, llama3"
        />
      </FormField>

      <div class="form-grid-2">
        <FormField
          label="Temperature"
          id="chat-temperature"
          hint="Sampling randomness (0.0 - 2.0)."
        >
          <input
            id="chat-temperature"
            v-model.number="form.temperature"
            type="number"
            step="0.05"
            min="0"
            max="2"
          />
        </FormField>

        <FormField
          label="Max output tokens"
          id="chat-max-tokens"
          hint="Upper limit of response length."
        >
          <input
            id="chat-max-tokens"
            v-model.number="form.maxOutputTokens"
            type="number"
            step="128"
            min="1"
            max="1048576"
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

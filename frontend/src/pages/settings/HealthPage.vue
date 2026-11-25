<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { t } from '../../i18n'
import {
  loadSettings,
  errorText,
  restoreSettings,
  type Settings
} from '../../services/settings'
import {
  checkForUpdate,
  openReleasePage,
  downloadLatestInstaller,
  installUpdate,
  type UpdateInfo
} from '../../services/update'
import {
  stackStatus,
  startLocalStack,
  startLocalStackClean,
  stopLocalStack,
  applyLocalPreset,
  type StackStatus
} from '../../services/stack'

const settings = ref<Settings | null>(null)
const error = ref('')
const message = ref('')
const updateInfo = ref<UpdateInfo | null>(null)
const checking = ref(false)
const stack = ref<StackStatus | null>(null)
const downloadProgress = ref(0)
const downloadedPath = ref('')
const busyStack = ref(false)
const credentialIssue = ref(false)
const stackStage = ref('')

onMounted(async () => {
  try {
    settings.value = await loadSettings()
  } catch (err) {
    error.value = errorText(err)
  }
  await refreshStack()

  Events.On('update:progress', (event: { data: unknown }) => {
    const data = event.data as { percent?: number }
    if (typeof data?.percent === 'number') downloadProgress.value = data.percent
  })

  Events.On('stack:progress', (event: { data: unknown }) => {
    const data = event.data as { stage?: string; message?: string }
    if (data?.message) stackStage.value = data.message
  })
})

async function refreshStack() {
  try {
    stack.value = await stackStatus()
  } catch (err) {
    error.value = errorText(err)
  }
}

async function handleStartStack() {
  busyStack.value = true
  error.value = ''
  message.value = ''
  stackStage.value = t('Starting local stack...')
  try {
    credentialIssue.value = false
    const result = await startLocalStack()
    if (result) {
      if (result.success) message.value = result.message
      else {
        error.value = result.message
        credentialIssue.value = Boolean(result.credentialIssue)
      }
    }
    await refreshStack()
  } catch (err) {
    error.value = errorText(err)
  } finally {
    busyStack.value = false
    stackStage.value = ''
  }
}

async function handleStartStackClean() {
  busyStack.value = true
  error.value = ''
  message.value = ''
  stackStage.value = t('Starting local stack with a clean Docker config...')
  try {
    const result = await startLocalStackClean()
    if (result) {
      if (result.success) {
        message.value = result.message
        credentialIssue.value = false
      } else {
        error.value = result.message
        credentialIssue.value = Boolean(result.credentialIssue)
      }
    }
    await refreshStack()
  } catch (err) {
    error.value = errorText(err)
  } finally {
    busyStack.value = false
    stackStage.value = ''
  }
}

async function handleStopStack() {
  busyStack.value = true
  try {
    const result = await stopLocalStack()
    if (result) {
      if (result.success) message.value = result.message
      else error.value = result.message
    }
    await refreshStack()
  } catch (err) {
    error.value = errorText(err)
  } finally {
    busyStack.value = false
  }
}

async function handleApplyPreset() {
  busyStack.value = true
  error.value = ''
  try {
    const result = await applyLocalPreset()
    if (result) {
      if (result.success) message.value = result.message
      else error.value = result.message
    }
    settings.value = await loadSettings()
  } catch (err) {
    error.value = errorText(err)
  } finally {
    busyStack.value = false
  }
}

async function handleDownloadUpdate() {
  error.value = ''
  downloadProgress.value = 0
  try {
    const path = await downloadLatestInstaller()
    if (path) {
      downloadedPath.value = path
      message.value = t('Installer downloaded to {{path}}', { path })
    }
  } catch (err) {
    error.value = errorText(err)
  }
}

async function handleInstallUpdate() {
  if (!downloadedPath.value) return
  error.value = ''
  message.value = ''
  try {
    const result = await installUpdate(downloadedPath.value)
    if (result.success) message.value = result.message
    else error.value = result.message
  } catch (err) {
    error.value = errorText(err)
  }
}

async function handleRestoreSettings(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  try {
    await restoreSettings(await file.text())
    message.value = t('Settings restored. Restart the app to apply.')
  } catch (err) {
    error.value = errorText(err)
  } finally {
    input.value = ''
  }
}

async function handleCheckUpdate() {
  checking.value = true
  error.value = ''
  try {
    updateInfo.value = await checkForUpdate()
  } catch (err) {
    error.value = errorText(err)
  } finally {
    checking.value = false
  }
}

function detail(section: Settings[keyof Settings]): string {
  const value = section as unknown as Record<string, unknown>
  return String(value.modelName || value.databaseName || value.baseUrl || '—')
}

// Translated at render time through computed so locale changes take effect.
const items = computed(() => [
  { key: 'chatModel', label: t('Chat model'), path: '/settings/chat' },
  { key: 'embeddingModel', label: t('Embedding model'), path: '/settings/embedding' },
  { key: 'database', label: t('Database'), path: '/settings/database' },
  { key: 'docling', label: t('Document processing'), path: '/settings/docling' }
] as const)
</script>

<template>
  <div class="health-page">
    <h2 class="section-title">{{ t('settings.health') }}</h2>
    <p class="section-hint">{{ t('All four services must be configured before training a knowledge base.') }}</p>
    <div v-if="error" class="health-error">{{ error }}</div>
    <div v-if="settings" class="health-grid">
      <router-link
        v-for="item in items"
        :key="item.key"
        :to="item.path"
        class="health-card"
      >
        <span class="health-name">{{ item.label }}</span>
        <span :class="settings[item.key].configured ? 'health-ok' : 'health-missing'">
          {{ settings[item.key].configured ? t('Configured') : t('Not configured') }}
        </span>
        <span class="health-detail">{{ detail(settings[item.key]) }}</span>
      </router-link>
    </div>

    <div v-if="message" class="health-message">{{ message }}</div>

    <section class="health-card">
      <h3 class="card-title">{{ t('Local stack') }}</h3>
      <p class="section-hint">
        {{ t('PostgreSQL + pgvector and docling-serve run in Docker. Ollama is installed by you.') }}
      </p>
      <div v-if="stack" class="stack-status">
        <span>{{ t('Docker CLI') }}: {{ stack.dockerInstalled ? t('found') : t('missing') }}</span>
        <span>{{ t('Compose plugin') }}: {{ stack.composePlugin ? t('ready') : t('missing') }}</span>
        <span>{{ t('Daemon') }}: {{ stack.daemon ? t('running') : t('stopped') }}</span>
        <span>{{ t('Access') }}: {{ stack.permission ? t('ok') : t('denied') }}</span>
        <span>{{ t('PostgreSQL') }}: {{ stack.postgres ? t('ready') : t('stopped') }}</span>
        <span>{{ t('Docling') }}: {{ stack.docling ? t('ready') : t('stopped') }}</span>
        <span>{{ t('Ollama') }}: {{ stack.ollama ? t('detected') : t('not detected') }}</span>
      </div>
      <div v-if="stack?.detail" class="health-detail">{{ stack.detail }}</div>
      <div v-if="stackStage" class="stack-stage">{{ stackStage }}</div>
      <div class="row-actions">
        <button type="button" class="action-btn" :disabled="busyStack" @click="handleStartStack">
          {{ busyStack ? t('Starting…') : t('Start local stack') }}
        </button>
        <button
          v-if="credentialIssue"
          type="button"
          class="action-btn"
          :disabled="busyStack"
          @click="handleStartStackClean"
        >
          {{ busyStack ? t('Retrying…') : t('Retry with clean Docker config') }}
        </button>
        <button type="button" class="action-btn" :disabled="busyStack" @click="handleStopStack">
          {{ busyStack ? t('Stopping…') : t('Stop') }}
        </button>
        <button type="button" class="action-btn" :disabled="busyStack" @click="handleApplyPreset">{{ t('Use local preset') }}</button>
        <button type="button" class="action-btn" @click="refreshStack">{{ t('Refresh') }}</button>
      </div>
      <a class="action-btn" href="https://ollama.com/download" target="_blank" rel="noopener">{{ t('Install Ollama') }}</a>
    </section>

    <section class="health-card">
      <h3 class="card-title">{{ t('Restore settings') }}</h3>
      <div class="row-actions">
        <label class="action-btn">
          {{ t('Restore settings (.db)') }}
          <input type="file" accept=".db,.sqlite" @change="handleRestoreSettings" />
        </label>
      </div>
    </section>

    <section class="health-card">
      <h3 class="card-title">{{ t('Application update') }}</h3>
      <div class="row-actions">
        <button type="button" class="action-btn" :disabled="checking" @click="handleCheckUpdate">
          {{ checking ? t('Checking…') : t('Check for updates') }}
        </button>
        <button type="button" class="action-btn" @click="openReleasePage">{{ t('Open releases') }}</button>
        <button
          v-if="updateInfo?.updateAvailable && updateInfo.assetUrl"
          type="button"
          class="action-btn"
          @click="handleDownloadUpdate"
        >
          {{ t('Download update') }}
        </button>
        <button
          v-if="downloadedPath"
          type="button"
          class="action-btn"
          @click="handleInstallUpdate"
        >
          {{ t('Install update') }}
        </button>
      </div>
      <div v-if="downloadProgress > 0 && downloadProgress < 100" class="health-detail">
        {{ t('Downloading…') }} {{ downloadProgress }}%
      </div>
      <div v-if="updateInfo" class="health-detail">
        {{ t('Current') }} {{ updateInfo.currentVersion }} · {{ t('Latest') }} {{ updateInfo.latestVersion || t('unknown') }}
        <span v-if="updateInfo.updateAvailable" class="health-missing"> {{ t('— update available') }}</span>
        <span v-else class="health-ok"> {{ t('— up to date') }}</span>
      </div>
    </section>
  </div>
</template>

<style scoped>
.health-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-16);
}

.section-title {
  font-size: var(--font-size-lg);
  margin: 0;
}

.section-hint {
  color: var(--color-text-muted);
  font-size: var(--font-size-sm);
  margin: 0;
}

.health-error {
  padding: var(--space-8) var(--space-12);
  border: 1px solid var(--color-danger);
  color: var(--color-danger);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
}

.health-message {
  padding: var(--space-8) var(--space-12);
  border: 1px solid var(--color-success);
  border-radius: var(--radius-sm);
  color: var(--color-success);
  font-size: var(--font-size-sm);
}

.card-title {
  font-size: var(--font-size-base);
  margin: 0 0 var(--space-8);
}

.stack-stage {
  font-size: var(--font-size-sm);
  color: var(--color-accent);
  padding: var(--space-8) var(--space-12);
  background-color: var(--color-accent-subtle);
  border-radius: var(--radius-sm);
}

.stack-status {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-12);
  font-size: var(--font-size-sm);
  color: var(--color-text-muted);
}

.row-actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-8);
}

.action-btn {
  display: inline-flex;
  align-items: center;
  min-height: 36px;
  padding: 0 var(--space-12);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background-color: var(--color-surface-subtle);
  color: var(--color-text);
  font-size: var(--font-size-sm);
  cursor: pointer;
}

.action-btn input {
  display: none;
}

.health-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: var(--space-12);
}

.health-card {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  padding: var(--space-16);
  background-color: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  text-decoration: none;
  color: var(--color-text);
}

.health-card:hover {
  border-color: var(--color-accent);
  text-decoration: none;
}

.health-name {
  font-weight: var(--font-weight-medium);
  font-size: var(--font-size-sm);
}

.health-ok {
  color: var(--color-success);
  font-size: var(--font-size-sm);
}

.health-missing {
  color: var(--color-danger);
  font-size: var(--font-size-sm);
}

.health-detail {
  color: var(--color-text-muted);
  font-family: var(--font-mono);
  font-size: 12px;
  overflow-wrap: anywhere;
}
</style>

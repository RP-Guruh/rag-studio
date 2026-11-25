<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import {
  loadSettings,
  errorText,
  backupSettings,
  restoreSettings,
  type Settings
} from '../../services/settings'
import { backupAll, restoreAll } from '../../services/projects'
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
  try {
    const result = await startLocalStack()
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
      message.value = `Installer downloaded to ${path}`
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

function download(data: string, filename: string, type: string) {
  if (!data) return
  const bytes = Uint8Array.from(atob(data), (char) => char.charCodeAt(0))
  const url = URL.createObjectURL(new Blob([bytes], { type }))
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
}

async function handleBackupAll() {
  try {
    download(await backupAll(), 'rag-studio-backup.json', 'application/json')
    message.value = 'Full backup downloaded.'
  } catch (err) {
    error.value = errorText(err)
  }
}

async function handleRestoreFile(event: Event, kind: 'settings' | 'database') {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  try {
    const data = await file.text()
    if (kind === 'settings') await restoreSettings(data)
    else await restoreAll(data)
    message.value = kind === 'settings'
      ? 'Settings restored. Restart the app to apply.'
      : 'Knowledge bases restored.'
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

const items = [
  { key: 'chatModel', label: 'Chat model', path: '/settings/chat' },
  { key: 'embeddingModel', label: 'Embedding model', path: '/settings/embedding' },
  { key: 'database', label: 'Database', path: '/settings/database' },
  { key: 'docling', label: 'Document processing', path: '/settings/docling' }
] as const
</script>

<template>
  <div class="health-page">
    <h2 class="section-title">Setup & health</h2>
    <p class="section-hint">All four services must be configured before training a knowledge base.</p>
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
          {{ settings[item.key].configured ? 'Configured' : 'Not configured' }}
        </span>
        <span class="health-detail">{{ detail(settings[item.key]) }}</span>
      </router-link>
    </div>

    <div v-if="message" class="health-message">{{ message }}</div>

    <section class="health-card">
      <h3 class="card-title">Local stack</h3>
      <p class="section-hint">
        PostgreSQL + pgvector and docling-serve run in Docker. Ollama is installed by you.
      </p>
      <div v-if="stack" class="stack-status">
        <span>Docker CLI: {{ stack.dockerInstalled ? 'found' : 'missing' }}</span>
        <span>Compose plugin: {{ stack.composePlugin ? 'ready' : 'missing' }}</span>
        <span>Daemon: {{ stack.daemon ? 'running' : 'stopped' }}</span>
        <span>Access: {{ stack.permission ? 'ok' : 'denied' }}</span>
        <span>PostgreSQL: {{ stack.postgres ? 'ready' : 'stopped' }}</span>
        <span>Docling: {{ stack.docling ? 'ready' : 'stopped' }}</span>
        <span>Ollama: {{ stack.ollama ? 'detected' : 'not detected' }}</span>
      </div>
      <div v-if="stack?.detail" class="health-detail">{{ stack.detail }}</div>
      <div class="row-actions">
        <button type="button" class="action-btn" :disabled="busyStack" @click="handleStartStack">Start local stack</button>
        <button type="button" class="action-btn" :disabled="busyStack" @click="handleStopStack">Stop</button>
        <button type="button" class="action-btn" :disabled="busyStack" @click="handleApplyPreset">Use local preset</button>
        <button type="button" class="action-btn" @click="refreshStack">Refresh</button>
      </div>
      <a class="action-btn" href="https://ollama.com/download" target="_blank" rel="noopener">Install Ollama</a>
    </section>

    <section class="health-card">
      <h3 class="card-title">Backup & restore</h3>
      <div class="row-actions">
        <button type="button" class="action-btn" @click="handleBackupAll">Backup all projects</button>
        <label class="action-btn">
          Restore settings (.db)
          <input type="file" accept=".db,.sqlite" @change="handleRestoreFile($event, 'settings')" />
        </label>
        <label class="action-btn">
          Restore projects (.json)
          <input type="file" accept=".json" @change="handleRestoreFile($event, 'database')" />
        </label>
      </div>
    </section>

    <section class="health-card">
      <h3 class="card-title">Application update</h3>
      <div class="row-actions">
        <button type="button" class="action-btn" :disabled="checking" @click="handleCheckUpdate">
          {{ checking ? 'Checking…' : 'Check for updates' }}
        </button>
        <button type="button" class="action-btn" @click="openReleasePage">Open releases</button>
        <button
          v-if="updateInfo?.updateAvailable && updateInfo.assetUrl"
          type="button"
          class="action-btn"
          @click="handleDownloadUpdate"
        >
          Download update
        </button>
        <button
          v-if="downloadedPath"
          type="button"
          class="action-btn"
          @click="handleInstallUpdate"
        >
          Install update
        </button>
      </div>
      <div v-if="downloadProgress > 0 && downloadProgress < 100" class="health-detail">
        Downloading… {{ downloadProgress }}%
      </div>
      <div v-if="updateInfo" class="health-detail">
        Current {{ updateInfo.currentVersion }} · Latest {{ updateInfo.latestVersion || 'unknown' }}
        <span v-if="updateInfo.updateAvailable" class="health-missing"> — update available</span>
        <span v-else class="health-ok"> — up to date</span>
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

import * as SettingsService from '../../bindings/rag_project/internal/service/settingsservice'
import type {
  ChatModel,
  Database,
  DoclingConnection,
  EmbeddingModel,
  Settings,
  TestResult
} from '../../bindings/rag_project/internal/settings/models'
import { mockStore } from '../mock/data'

declare global {
  interface Window {
    _wails?: { environment?: unknown; invoke?: unknown; flags?: unknown }
    webkit?: { messageHandlers?: Record<string, unknown> }
    chrome?: { webview?: unknown }
  }
}

export type { ChatModel, Database, DoclingConnection, EmbeddingModel, Settings, TestResult }

/** True when the Wails runtime has been injected into the page. */
export function isWailsRuntime(): boolean {
  if (typeof window === 'undefined') return false
  const wails = window._wails
  return Boolean(
    wails?.invoke ||
    wails?.environment ||
    window.chrome?.webview ||
    window.webkit?.messageHandlers?.external
  )
}

let wailsRuntimeConfirmed = false
let wailsRuntimeSettled = false

if (typeof window !== 'undefined') {
  // The event fires when Wails injects its runtime, possibly after this module
  // was evaluated. Keep listening so a late injection is still recognised.
  window.addEventListener('wails:runtime-config-ready', () => {
    if (isWailsRuntime()) wailsRuntimeConfirmed = true
  })
}

/**
 * Wails injects its runtime on WindowLoadFinished, which can be later than
 * module evaluation. Wait briefly instead of silently falling back to mock
 * data before the bindings exist.
 */
export async function waitForWailsRuntime(timeoutMs = 2000): Promise<boolean> {
  if (wailsRuntimeConfirmed || isWailsRuntime()) {
    wailsRuntimeConfirmed = true
    return true
  }
  if (typeof window === 'undefined') return false
  if (wailsRuntimeSettled) return false

  await new Promise<void>((resolve) => {
    let done = false
    const finish = () => {
      if (done) return
      done = true
      window.removeEventListener('wails:runtime-config-ready', finish)
      window.clearInterval(poll)
      window.clearTimeout(timeout)
      resolve()
    }
    const poll = window.setInterval(() => {
      if (isWailsRuntime()) finish()
    }, 50)
    const timeout = window.setTimeout(finish, timeoutMs)
    window.addEventListener('wails:runtime-config-ready', finish)
  })

  wailsRuntimeSettled = true
  if (isWailsRuntime()) {
    wailsRuntimeConfirmed = true
    return true
  }
  return false
}

export const chatProviders = [
  { value: 'openai-compatible', label: 'OpenAI-compatible' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'ollama', label: 'Ollama' }
]

export const embeddingProviders = [
  { value: 'openai-compatible', label: 'OpenAI-compatible' },
  { value: 'ollama', label: 'Ollama' },
  { value: 'huggingface', label: 'HuggingFace' }
]

export const sslModes = [
  'disable',
  'allow',
  'prefer',
  'require',
  'verify-ca',
  'verify-full'
]

export const doclingModes = [
  { value: 'serve', label: 'Serve (Docling on another machine)' },
  { value: 'local', label: 'Local (Docling on this computer)' }
]

/**
 * Empty placeholder values for forms that have no saved configuration yet.
 * These are never used as runtime configuration; placeholders live in the
 * individual inputs instead.
 */
export const defaultSettings: Settings = {
  chatModel: {
    provider: '',
    baseUrl: '',
    modelName: '',
    temperature: 0,
    maxOutputTokens: 0,
    apiKeySet: false,
    apiKeyHint: '',
    configured: false
  },
  embeddingModel: {
    provider: '',
    baseUrl: '',
    modelName: '',
    vectorDimension: 0,
    chunkMaxTokens: 0,
    tokenizer: '',
    apiKeySet: false,
    apiKeyHint: '',
    configured: false
  },
  database: {
    host: '',
    port: 0,
    databaseName: '',
    username: '',
    sslMode: '',
    schemaName: '',
    passwordSet: false,
    passwordHint: '',
    configured: false
  },
  docling: {
    mode: '',
    baseUrl: '',
    timeoutSeconds: 0,
    apiKeySet: false,
    apiKeyHint: '',
    configured: false
  }
}

export async function backupSettings(): Promise<string> {
  if (!(await waitForWailsRuntime())) return ''
  return SettingsService.BackupSettings()
}

export async function restoreSettings(payload: string): Promise<void> {
  if (!(await waitForWailsRuntime())) return
  await SettingsService.RestoreSettings(payload)
}

export function errorText(error: unknown): string {
  if (error instanceof Error) return error.message
  if (typeof error === 'string') return error
  return 'The operation could not be completed.'
}

export async function loadSettings(): Promise<Settings> {
  if (!(await waitForWailsRuntime())) {
    return clone(mockStore.settings)
  }
  return SettingsService.GetSettings()
}

export async function saveChatModel(input: ChatModel): Promise<ChatModel> {
  if (!(await waitForWailsRuntime())) {
    const saved = stripSecret(input)
    mockStore.settings.chatModel = {
      ...applySecretState(saved, input.apiKey, input.clearApiKey, 'apiKeySet', 'apiKeyHint'),
      configured: true
    }
    return clone(mockStore.settings.chatModel)
  }
  return SettingsService.SaveChatModel(input)
}

export async function saveEmbeddingModel(input: EmbeddingModel): Promise<EmbeddingModel> {
  if (!(await waitForWailsRuntime())) {
    const saved = stripSecret(input)
    mockStore.settings.embeddingModel = {
      ...applySecretState(saved, input.apiKey, input.clearApiKey, 'apiKeySet', 'apiKeyHint'),
      configured: true
    }
    return clone(mockStore.settings.embeddingModel)
  }
  return SettingsService.SaveEmbeddingModel(input)
}

export async function saveDatabase(input: Database): Promise<{database: Database, migrated: boolean}> {
  if (!(await waitForWailsRuntime())) {
    const saved = stripSecret(input)
    mockStore.settings.database = {
      ...applySecretState(saved, input.password, input.clearPassword, 'passwordSet', 'passwordHint'),
      configured: true
    }
    return { database: clone(mockStore.settings.database), migrated: false }
  }
  const result = await SettingsService.SaveDatabase(input)
  return { database: result[0], migrated: result[1] }
}

export async function saveDocling(input: DoclingConnection): Promise<DoclingConnection> {
  if (!(await waitForWailsRuntime())) {
    const saved = stripSecret(input)
    mockStore.settings.docling = {
      ...applySecretState(saved, input.apiKey, input.clearApiKey, 'apiKeySet', 'apiKeyHint'),
      configured: true
    }
    return clone(mockStore.settings.docling)
  }
  return SettingsService.SaveDocling(input)
}

export async function testChatModel(input: ChatModel): Promise<TestResult> {
  if (!(await waitForWailsRuntime())) {
    return mockTest(`Chat provider ${input.provider} (${input.modelName})`)
  }
  return SettingsService.TestChatModel(input)
}

export async function testEmbeddingModel(input: EmbeddingModel): Promise<TestResult> {
  if (!(await waitForWailsRuntime())) {
    return mockTest(
      `Embedding provider ${input.provider} (${input.vectorDimension} dimensions)`
    )
  }
  return SettingsService.TestEmbeddingModel(input)
}

export async function testDatabase(input: Database): Promise<TestResult> {
  if (!(await waitForWailsRuntime())) {
    return mockTest(`PostgreSQL at ${input.host}:${input.port}`)
  }
  return SettingsService.TestDatabase(input)
}

export async function testDocling(input: DoclingConnection): Promise<TestResult> {
  if (!(await waitForWailsRuntime())) {
    const subject = input.mode === 'local'
      ? 'Local Docling engine'
      : `Docling at ${input.baseUrl}`
    return mockTest(subject)
  }
  return SettingsService.TestDocling(input)
}

function mockTest(subject: string): TestResult {
  return {
    success: true,
    message: `Mock mode: ${subject} was not contacted. Run the desktop app to test the real connection.`,
    latencyMs: 0
  }
}

function stripSecret<T extends { apiKey?: string; password?: string; clearApiKey?: boolean; clearPassword?: boolean }>(
  input: T
): T {
  const copy = { ...input }
  delete copy.apiKey
  delete copy.password
  delete copy.clearApiKey
  delete copy.clearPassword
  return copy
}

function applySecretState<T, K extends keyof T, H extends keyof T>(
  value: T,
  secret: string | undefined,
  clear: boolean | undefined,
  setKey: K,
  hintKey: H
): T {
  const next = { ...value }
  if (clear) {
    next[setKey] = false as T[K]
    next[hintKey] = '' as T[H]
  } else if (secret && secret.trim() !== '') {
    next[setKey] = true as T[K]
    next[hintKey] = secretHint(secret.trim()) as T[H]
  }
  return next
}

function secretHint(secret: string): string {
  if (secret.length <= 4) return '••••'
  return '…' + secret.slice(-4)
}

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T
}

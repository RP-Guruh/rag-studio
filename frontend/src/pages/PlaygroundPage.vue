<script setup lang="ts">
import { ref, computed, nextTick, onMounted } from 'vue'
import { Events } from '@wailsio/runtime'
import BaseButton from '../components/BaseButton.vue'
import EmptyState from '../components/EmptyState.vue'
import {
  mockStore,
  sendPlaygroundMessage,
  clearPlaygroundMessages,
  type ChatMessage,
  type RetrievedChunk
} from '../mock/data'
import {
  askProject,
  askProjectStream,
  cancelChat,
  listProjects,
  type ChunkHit,
  type ChatDebug
} from '../services/projects'
import { waitForWailsRuntime } from '../services/settings'
import { t } from '../i18n'
import { renderMarkdown } from '../utils/markdown'

const selectedProjectId = ref(mockStore.playground.selectedProjectId || mockStore.projects[0]?.id || '')
const inputQuery = ref('')
const isSidePanelOpen = ref(
  typeof window === 'undefined' || window.innerWidth > 1100
)
const isSending = ref(false)
const loadingAssistantId = ref('')
const sendError = ref('')
const debugInfo = ref<ChatDebug | null>(null)
const chatScrollContainer = ref<HTMLElement | null>(null)

const activeProject = computed(() =>
  mockStore.projects.find((p) => p.id === selectedProjectId.value)
)

const personaSummary = computed(() => {
  const persona = activeProject.value?.persona
  if (!persona) return ''
  const language = !persona.answerLanguage || persona.answerLanguage === 'auto'
    ? 'Auto'
    : persona.answerLanguage
  const grounding = persona.strictKnowledge ? 'strict' : 'general'
  return `${persona.name} · ${persona.tone} · ${language} · ${grounding}`
})

const currentMessages = computed(() => {
  return mockStore.playground.messages[selectedProjectId.value] || []
})

const currentChunks = computed(() => {
  return mockStore.playground.lastRetrievedChunks[selectedProjectId.value] || []
})

onMounted(async () => {
  const projects = await listProjects()
  mockStore.projects = projects
  if (!mockStore.projects.some((p) => p.id === selectedProjectId.value)) {
    selectedProjectId.value = mockStore.projects[0]?.id || ''
  }
  mockStore.playground.selectedProjectId = selectedProjectId.value
})

function handleProjectChange() {
  mockStore.playground.selectedProjectId = selectedProjectId.value
  sendError.value = ''
}

function handleClearChat() {
  clearPlaygroundMessages(selectedProjectId.value)
  sendError.value = ''
}

function timestamp(): string {
  return new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

function appendMessage(projectId: string, message: ChatMessage) {
  if (!mockStore.playground.messages[projectId]) {
    mockStore.playground.messages[projectId] = []
  }
  mockStore.playground.messages[projectId].push(message)
}

function mapChunk(chunk: ChunkHit): RetrievedChunk {
  const metadata = (chunk.metadata || {}) as Record<string, unknown>
  const headings = Array.isArray(metadata.headings)
    ? (metadata.headings as string[]).join(' › ')
    : ''
  const captions = Array.isArray(metadata.captions)
    ? (metadata.captions as string[]).join(', ')
    : ''
  const parts = [
    chunk.pageNumber > 0 ? `Page ${chunk.pageNumber}` : `Chunk ${chunk.chunkIndex}`,
    headings,
    captions
  ].filter(Boolean)
  const location = parts.join(' · ')
  const excerpt = chunk.content.length > 220
    ? `${chunk.content.slice(0, 220)}…`
    : chunk.content
  return {
    id: `chunk-${chunk.chunkId}`,
    fileName: chunk.documentName,
    location,
    excerpt
  }
}

async function scrollToBottom() {
  await nextTick()
  if (chatScrollContainer.value) {
    chatScrollContainer.value.scrollTop = chatScrollContainer.value.scrollHeight
  }
}

async function handleSendMessage() {
  const query = inputQuery.value.trim()
  if (!query || !selectedProjectId.value || isSending.value) return

  const projectId = selectedProjectId.value
  inputQuery.value = ''
  sendError.value = ''
  debugInfo.value = null

  if (!(await waitForWailsRuntime())) {
    sendPlaygroundMessage(projectId, query)
    await scrollToBottom()
    return
  }

  appendMessage(projectId, {
    id: `msg-user-${Date.now()}`,
    role: 'user',
    content: query,
    timestamp: timestamp()
  })

  const assistantId = `msg-assistant-${Date.now()}`
  appendMessage(projectId, {
    id: assistantId,
    role: 'assistant',
    content: '',
    timestamp: timestamp()
  })
  const placeholder = mockStore.playground.messages[projectId]?.find((msg) => msg.id === assistantId)
  loadingAssistantId.value = assistantId
  isSending.value = true
  await scrollToBottom()

  const unsubscribe = Events.On('chat:token', (event: { data: unknown }) => {
    const data = event.data as { projectId?: string; token?: string }
    if (data?.projectId === projectId && placeholder) {
      placeholder.content += data.token || ''
      void scrollToBottom()
    }
  })

  try {
    const answer = await askProjectStream(projectId, query)
    if (placeholder) {
      placeholder.content = answer.answer
      placeholder.sources = (answer.sources || []).map((source) => source.fileName)
    }
    mockStore.playground.lastRetrievedChunks[projectId] = (answer.chunks || []).map(mapChunk)
    debugInfo.value = answer.debug || null
  } catch (err) {
    sendError.value = err instanceof Error ? err.message : String(err)
    if (placeholder) {
      placeholder.content = placeholder.content || `Could not answer: ${sendError.value}`
    }
  } finally {
    unsubscribe()
    loadingAssistantId.value = ''
    isSending.value = false
    await scrollToBottom()
  }
}

function toggleSidePanel() {
  isSidePanelOpen.value = !isSidePanelOpen.value
}
</script>

<template>
  <div class="playground-layout">
    <!-- Top Control Bar -->
    <header class="playground-header">
      <div class="project-selector-group">
        <label for="project-select" class="selector-label">Project</label>
        <select
          id="project-select"
          v-model="selectedProjectId"
          class="project-select-input"
          @change="handleProjectChange"
        >
          <option
            v-for="p in mockStore.projects"
            :key="p.id"
            :value="p.id"
          >
            {{ p.name }}
          </option>
        </select>
        <span v-if="activeProject" class="persona-tag">
          {{ activeProject.personaName }}
        </span>
        <span v-if="personaSummary" class="persona-summary">
          {{ personaSummary }}
        </span>
        <span v-if="activeProject?.description" class="project-context-hint" :title="activeProject.description">
          {{ activeProject.description }}
        </span>
      </div>

      <div class="header-actions">
        <BaseButton
          variant="outline"
          size="sm"
          type="button"
          @click="toggleSidePanel"
        >
          {{ isSidePanelOpen ? 'Hide chunks' : 'Show chunks' }}
        </BaseButton>

        <BaseButton
          variant="ghost"
          size="sm"
          type="button"
          :disabled="currentMessages.length === 0"
          @click="handleClearChat"
        >
          Clear chat
        </BaseButton>
      </div>
    </header>

    <!-- Main Playground Workspace -->
    <div class="playground-workspace">
      <!-- Chat Conversation Column -->
      <section class="chat-column">
        <!-- Message History -->
        <div ref="chatScrollContainer" class="messages-container">
          <div v-if="currentMessages.length > 0" class="messages-list">
            <div
              v-for="msg in currentMessages"
              :key="msg.id"
              :class="['message-row', `message-row--${msg.role}`]"
            >
              <div class="message-bubble">
                <div class="message-meta">
                  <span class="message-author">
                    {{ msg.role === 'user' ? 'You' : (activeProject?.personaName || 'Assistant') }}
                  </span>
                  <span class="message-time">{{ msg.timestamp }}</span>
                </div>
                <div
                  v-if="msg.role === 'assistant' && !(msg.id === loadingAssistantId && !msg.content)"
                  class="message-body markdown-body"
                  v-html="renderMarkdown(msg.content)"
                />
                <div
                  v-else-if="msg.role === 'assistant'"
                  class="thinking-state"
                  role="status"
                >
                  <span class="thinking-dots" aria-hidden="true"><i /><i /><i /></span>
                  <span>{{ activeProject?.personaName || 'Assistant' }} sedang berpikir…</span>
                </div>
                <div v-else class="message-body">
                  {{ msg.content }}
                </div>
                <div v-if="msg.sources && msg.sources.length > 0" class="message-sources">
                  <span class="sources-label">Sources:</span>
                  <span class="sources-files">{{ msg.sources.join(', ') }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Empty State -->
          <EmptyState
            v-else
            title="Playground is empty"
            description="Train a project's knowledge base, then ask a question below to test its answer."
          />

        </div>

        <!-- Input Bar Pinned to Bottom -->
        <div class="input-bar-container">
          <div v-if="sendError" class="send-error">{{ sendError }}</div>
          <div v-if="isSending" class="stop-row">
            <BaseButton variant="secondary" size="sm" type="button" @click="cancelChat(selectedProjectId)">
              {{ t('playground.stop') }}
            </BaseButton>
          </div>
          <form class="input-bar-form" @submit.prevent="handleSendMessage">
            <input
              v-model="inputQuery"
              type="text"
              class="chat-input"
              placeholder="Ask a question about the project's knowledge documents..."
            />
            <BaseButton
              variant="primary"
              type="submit"
              :disabled="!inputQuery.trim() || !selectedProjectId || isSending"
            >
              {{ isSending ? '…' : t('playground.send') }}
            </BaseButton>
          </form>
        </div>
      </section>

      <!-- Right Collapsible Panel: Retrieved Chunks -->
      <aside v-if="isSidePanelOpen" class="chunks-panel">
        <div class="chunks-header">
          <h2 class="chunks-title">Retrieved chunks</h2>
          <span class="chunks-count font-mono">{{ currentChunks.length }}</span>
        </div>

        <div class="chunks-body">
          <div v-if="currentChunks.length > 0" class="chunks-list">
            <article
              v-for="chunk in currentChunks"
              :key="chunk.id"
              class="chunk-card"
            >
              <header class="chunk-meta">
                <span class="chunk-filename">{{ chunk.fileName }}</span>
                <span class="chunk-location font-mono">{{ chunk.location }}</span>
              </header>
              <p class="chunk-excerpt">
                {{ chunk.excerpt }}
              </p>
            </article>
          </div>
          <div v-else class="chunks-empty">
            <p>No chunks retrieved for this session yet.</p>
          </div>

          <div v-if="debugInfo" class="debug-panel">
            <h3 class="debug-title">Last request</h3>
            <p class="debug-meta">
              {{ debugInfo.latencyMs }} ms · top K {{ debugInfo.topK }}
            </p>
            <pre class="debug-prompt">{{ debugInfo.systemPrompt }}</pre>
          </div>
        </div>
      </aside>
    </div>
  </div>
</template>

<style scoped>
.playground-layout {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
  min-width: 0;
}

.playground-header {
  min-height: 56px;
  padding: var(--space-12) var(--space-24);
  background-color: var(--color-surface);
  border-bottom: 1px solid var(--color-border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: var(--space-12);
  min-width: 0;
}

.project-selector-group {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--space-12);
  min-width: 0;
}

.persona-summary {
  font-size: var(--font-size-sm);
  color: var(--color-text-muted);
  white-space: nowrap;
}

.project-context-hint {
  font-size: var(--font-size-sm);
  color: var(--color-text-muted);
  max-width: 320px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.selector-label {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--color-text-muted);
  white-space: nowrap;
}

.project-select-input {
  min-height: 36px;
  padding: 0 var(--space-12);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background-color: var(--color-surface);
  color: var(--color-text);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  max-width: 260px;
  min-width: 0;
}

.persona-tag {
  font-size: var(--font-size-sm);
  color: var(--color-text-muted);
  background-color: var(--color-surface-subtle);
  padding: 4px var(--space-8);
  border-radius: var(--radius-sm);
  border: 1px solid var(--color-border);
  white-space: nowrap;
  flex-shrink: 0;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: var(--space-8);
}

.playground-workspace {
  flex: 1;
  display: flex;
  overflow: hidden;
  min-width: 0;
  position: relative;
}

.chat-column {
  flex: 1;
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
  min-width: 0;
}

.messages-container {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  padding: var(--space-24);
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.messages-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-16);
  max-width: none;
  margin: 0;
  width: 100%;
  min-width: 0;
}

.message-row {
  display: flex;
  width: 100%;
}

.message-row--user {
  justify-content: flex-end;
}

.message-row--assistant {
  justify-content: flex-start;
}

.message-bubble {
  position: relative;
  max-width: 85%;
  width: fit-content;
  min-width: 0;
  padding: var(--space-16);
  border-radius: var(--radius-md);
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  word-break: break-word;
}

/* Comic-style tail: border triangle behind, surface triangle in front. */
.message-bubble::before,
.message-bubble::after {
  content: '';
  position: absolute;
  top: 18px;
  width: 0;
  height: 0;
  border-style: solid;
}

.message-row--user .message-bubble::before {
  left: 100%;
  border-width: 7px 0 7px 8px;
  border-color: transparent transparent transparent var(--color-border);
}

.message-row--user .message-bubble::after {
  left: calc(100% - 1px);
  border-width: 6px 0 6px 7px;
  border-color: transparent transparent transparent var(--color-surface);
}

.message-row--assistant .message-bubble::before {
  left: -8px;
  border-width: 7px 8px 7px 0;
  border-color: transparent var(--color-border) transparent transparent;
}

.message-row--assistant .message-bubble::after {
  left: -7px;
  border-width: 6px 7px 6px 0;
  border-color: transparent var(--color-surface) transparent transparent;
}

.message-row--user .message-bubble {
  background-color: var(--color-surface);
  border: 1px solid var(--color-border);
}

.message-row--assistant .message-bubble {
  background-color: var(--color-surface);
  border: 1px solid var(--color-border);
  width: fit-content;
  max-width: min(90%, 720px);
  padding: var(--space-20);
  min-height: 56px;
}

.message-row--assistant .message-meta {
  margin-bottom: var(--space-8);
}

.message-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-16);
  margin-bottom: var(--space-4);
}

.message-author {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--color-text);
}

.message-time {
  font-size: 11px;
  color: var(--color-text-muted);
  font-family: var(--font-mono);
}

.message-body {
  font-size: var(--font-size-base);
  line-height: var(--line-height-normal);
  color: var(--color-text);
  white-space: pre-wrap;
}

.markdown-body {
  white-space: normal;
  min-width: 0;
  line-height: 1.7;
  overflow-wrap: break-word;
}

.markdown-body > :first-child {
  margin-top: 0;
}

.markdown-body p {
  margin: 0 0 var(--space-16);
}

.markdown-body p:last-child {
  margin-bottom: 0;
}

.markdown-body strong {
  font-weight: var(--font-weight-medium);
}

.markdown-body ul,
.markdown-body ol {
  margin: var(--space-8) 0 var(--space-12);
  padding-left: var(--space-20);
}

.markdown-body ul:last-child,
.markdown-body ol:last-child {
  margin-bottom: 0;
}

.markdown-body li + li {
  margin-top: var(--space-8);
}

.markdown-body li::marker {
  color: var(--color-text-muted);
}

.markdown-body h1,
.markdown-body h2,
.markdown-body h3,
.markdown-body h4 {
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-medium);
  margin: 0 0 var(--space-8);
}

.markdown-body code {
  font-family: var(--font-mono);
  font-size: var(--font-size-sm);
  background-color: var(--color-surface-subtle);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  padding: 1px 4px;
}

.markdown-body pre {
  margin: var(--space-8) 0 0;
  padding: var(--space-12);
  background-color: var(--color-surface-subtle);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  overflow-x: auto;
}

.markdown-body pre code {
  padding: 0;
  border: none;
  background: transparent;
}

.markdown-body a {
  color: var(--color-accent);
  text-decoration: underline;
}

.markdown-body blockquote {
  margin: var(--space-8) 0 0;
  padding-left: var(--space-12);
  border-left: 1px solid var(--color-border-strong);
  color: var(--color-text-muted);
}

.markdown-body hr {
  border: none;
  border-top: 1px solid var(--color-border);
  margin: var(--space-12) 0;
}

.message-sources {
  margin-top: var(--space-8);
  padding-top: var(--space-8);
  border-top: 1px solid var(--color-border);
  display: flex;
  align-items: center;
  gap: var(--space-4);
  font-size: var(--font-size-sm);
}

.sources-label {
  color: var(--color-text-muted);
  font-weight: var(--font-weight-medium);
}

.sources-files {
  color: var(--color-accent);
  font-family: var(--font-mono);
  font-size: 12px;
}

.thinking-state {
  display: flex;
  align-items: center;
  gap: var(--space-8);
  min-height: 24px;
  font-size: var(--font-size-sm);
  color: var(--color-text-muted);
}

.thinking-dots {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.thinking-dots i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background-color: var(--color-accent);
  animation: thinking-pulse 1.2s infinite ease-in-out;
}

.thinking-dots i:nth-child(2) { animation-delay: 0.2s; }
.thinking-dots i:nth-child(3) { animation-delay: 0.4s; }

@keyframes thinking-pulse {
  0%, 60%, 100% { opacity: 0.25; transform: translateY(0); }
  30% { opacity: 1; transform: translateY(-3px); }
}

.send-error {
  max-width: none;
  margin: 0 0 var(--space-8);
  padding: var(--space-8) var(--space-12);
  border: 1px solid var(--color-danger);
  border-radius: var(--radius-sm);
  color: var(--color-danger);
  font-size: var(--font-size-sm);
}

.input-bar-container {
  padding: var(--space-16) var(--space-24);
  background-color: var(--color-surface);
  border-top: 1px solid var(--color-border);
}

.stop-row {
  max-width: none;
  margin-bottom: var(--space-8);
}

.input-bar-form {
  display: flex;
  gap: var(--space-8);
  max-width: none;
  margin: 0;
  width: 100%;
}

.chat-input {
  flex: 1;
  min-height: 40px;
  padding: 0 var(--space-16);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background-color: var(--color-bg);
  font-size: var(--font-size-base);
  color: var(--color-text);
}

.chat-input:focus {
  border-color: var(--color-accent);
  outline: none;
}

/* Chunks Panel */
.chunks-panel {
  width: 280px;
  min-width: 280px;
  max-width: 300px;
  flex-shrink: 0;
  border-left: 1px solid var(--color-border);
  background-color: var(--color-surface);
  display: flex;
  flex-direction: column;
  height: 100%;
}

.chunks-header {
  height: 56px;
  padding: 0 var(--space-16);
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--color-border);
}

.chunks-title {
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-medium);
  color: var(--color-text);
}

.chunks-count {
  font-size: var(--font-size-sm);
  color: var(--color-text-muted);
}

.chunks-body {
  flex: 1;
  overflow-y: auto;
  padding: var(--space-16);
}

.chunks-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-12);
}

.chunk-card {
  padding: var(--space-12);
  background-color: var(--color-bg);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  display: flex;
  flex-direction: column;
  gap: var(--space-8);
}

.chunk-meta {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.chunk-filename {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--color-text);
}

.chunk-location {
  font-size: 11px;
  color: var(--color-text-muted);
}

.chunk-excerpt {
  font-size: var(--font-size-sm);
  color: var(--color-text);
  line-height: var(--line-height-normal);
}

.chunks-empty {
  padding: var(--space-32) var(--space-16);
  text-align: center;
  font-size: var(--font-size-sm);
  color: var(--color-text-muted);
}

.debug-panel {
  margin-top: var(--space-16);
  padding-top: var(--space-16);
  border-top: 1px solid var(--color-border);
}

.debug-title {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  margin: 0 0 var(--space-4);
}

.debug-meta {
  font-size: var(--font-size-sm);
  color: var(--color-text-muted);
  margin: 0 0 var(--space-8);
}

.debug-prompt {
  margin: 0;
  padding: var(--space-8);
  background-color: var(--color-surface-subtle);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  font-family: var(--font-mono);
  font-size: 11px;
  line-height: 1.5;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.font-mono {
  font-family: var(--font-mono);
}

/* Below this width the retrieved-chunks panel overlays instead of taking
   space away from the chat column. */
@media (max-width: 1100px) {
  .chunks-panel {
    position: absolute;
    top: 0;
    right: 0;
    bottom: 0;
    z-index: 10;
    width: 260px;
    min-width: 260px;
    max-width: 260px;
    box-shadow: var(--shadow-floating);
  }
}

@media (max-width: 900px) {
  .playground-header {
    padding-left: var(--space-16);
    padding-right: var(--space-16);
  }

  .messages-container,
  .input-bar-container {
    padding-left: var(--space-16);
    padding-right: var(--space-16);
  }
}

@media (max-width: 600px) {
  .persona-summary,
  .project-context-hint {
    white-space: normal;
    width: 100%;
    max-width: none;
  }

  .chunks-panel {
    width: calc(100% - var(--space-32));
    min-width: 0;
    max-width: none;
  }

  .message-bubble {
    max-width: 100%;
  }

  .input-bar-form {
    flex-wrap: wrap;
  }

  .input-bar-form .btn {
    width: 100%;
  }
}
</style>

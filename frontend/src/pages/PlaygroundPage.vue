<script setup lang="ts">
import { ref, computed, nextTick } from 'vue'
import BaseButton from '../components/BaseButton.vue'
import EmptyState from '../components/EmptyState.vue'
import {
  mockStore,
  sendPlaygroundMessage,
  clearPlaygroundMessages
} from '../mock/data'

const selectedProjectId = ref(mockStore.playground.selectedProjectId || mockStore.projects[0]?.id || '')
const inputQuery = ref('')
const isSidePanelOpen = ref(true)
const chatScrollContainer = ref<HTMLElement | null>(null)

const activeProject = computed(() =>
  mockStore.projects.find((p) => p.id === selectedProjectId.value)
)

const currentMessages = computed(() => {
  return mockStore.playground.messages[selectedProjectId.value] || []
})

const currentChunks = computed(() => {
  return mockStore.playground.lastRetrievedChunks[selectedProjectId.value] || []
})

function handleProjectChange() {
  mockStore.playground.selectedProjectId = selectedProjectId.value
}

function handleClearChat() {
  clearPlaygroundMessages(selectedProjectId.value)
}

async function handleSendMessage() {
  const query = inputQuery.value.trim()
  if (!query || !selectedProjectId.value) return

  sendPlaygroundMessage(selectedProjectId.value, query)
  inputQuery.value = ''

  await nextTick()
  if (chatScrollContainer.value) {
    chatScrollContainer.value.scrollTop = chatScrollContainer.value.scrollHeight
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
                <div class="message-body">
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
            description="Send a message below to test retrieval answers and review retrieved document chunks."
          />
        </div>

        <!-- Input Bar Pinned to Bottom -->
        <div class="input-bar-container">
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
              :disabled="!inputQuery.trim() || !selectedProjectId"
            >
              Send
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
  height: 56px;
  min-height: 56px;
  padding: 0 var(--space-24);
  background-color: var(--color-surface);
  border-bottom: 1px solid var(--color-border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-width: 0;
}

.project-selector-group {
  display: flex;
  align-items: center;
  gap: var(--space-12);
  flex-wrap: nowrap;
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
  max-width: 840px;
  margin: 0 auto;
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
  max-width: 85%;
  width: fit-content;
  padding: var(--space-16);
  border-radius: var(--radius-md);
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  word-break: break-word;
}

.message-row--user .message-bubble {
  background-color: var(--color-surface);
  border: 1px solid var(--color-border);
}

.message-row--assistant .message-bubble {
  background-color: var(--color-surface);
  border: 1px solid var(--color-border);
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

.input-bar-container {
  padding: var(--space-16) var(--space-24);
  background-color: var(--color-surface);
  border-top: 1px solid var(--color-border);
}

.input-bar-form {
  display: flex;
  gap: var(--space-8);
  max-width: 840px;
  margin: 0 auto;
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

.font-mono {
  font-family: var(--font-mono);
}
</style>

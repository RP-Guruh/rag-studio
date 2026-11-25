<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { isAdmin } from '../../services/auth'
import { t } from '../../i18n'

const route = useRoute()

// Recomputed so a locale switch updates the navigation without a restart.
const navItems = computed(() => [
  { name: t('settings.health'), path: '/settings/health' },
  { name: t('Chat model'), path: '/settings/chat' },
  { name: t('Embedding model'), path: '/settings/embedding' },
  { name: t('Database'), path: '/settings/database' },
  { name: t('Document processing'), path: '/settings/docling' },
  { name: t('settings.users'), path: '/settings/users', adminOnly: true }
])
</script>

<template>
  <div class="settings-page">
    <header class="settings-header">
      <div class="header-titles">
        <h1 class="page-title">{{ t('settings.title') }}</h1>
        <p class="page-description">{{ t('Configure language models, embedding vectors, the vector database, and document processing.') }}</p>
      </div>
    </header>

    <div class="settings-body">
      <!-- Secondary Navigation on Left -->
      <aside class="settings-nav-pane">
        <nav class="settings-nav">
          <router-link
            v-for="item in navItems.filter((nav) => !nav.adminOnly || isAdmin)"
            :key="item.path"
            :to="item.path"
            class="settings-nav-item"
            :class="{ 'settings-nav-item--active': route.path === item.path }"
          >
            {{ item.name }}
          </router-link>
        </nav>
      </aside>

      <!-- Settings Content View -->
      <main class="settings-content-pane">
        <router-view />
      </main>
    </div>
  </div>
</template>

<style scoped>
.settings-page {
  display: flex;
  flex-direction: column;
  min-height: 100%;
}

.settings-header {
  padding: var(--space-24) var(--space-32);
  background-color: var(--color-surface);
  border-bottom: 1px solid var(--color-border);
}

.header-titles {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.page-title {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-medium);
  color: var(--color-text);
  letter-spacing: -0.01em;
}

.page-description {
  font-size: var(--font-size-base);
  color: var(--color-text-muted);
}

.settings-body {
  display: flex;
  flex: 1;
}

.settings-nav-pane {
  width: 220px;
  min-width: 220px;
  background-color: var(--color-surface);
  border-right: 1px solid var(--color-border);
  padding: var(--space-24) var(--space-12);
}

.settings-nav {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.settings-nav-item {
  display: flex;
  align-items: center;
  padding: 0 var(--space-12);
  min-height: 38px;
  border-radius: var(--radius-sm);
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-medium);
  color: var(--color-text-muted);
  text-decoration: none;
  transition: background-color var(--transition-fast), color var(--transition-fast);
}

.settings-nav-item:hover {
  background-color: var(--color-surface-subtle);
  color: var(--color-text);
  text-decoration: none;
}

.settings-nav-item--active {
  background-color: var(--color-surface-subtle);
  color: var(--color-accent);
}

.settings-content-pane {
  flex: 1;
  min-width: 0;
  padding: var(--space-32);
  max-width: 680px;
}

@media (max-width: 900px) {
  .settings-header,
  .settings-content-pane {
    padding-left: var(--space-16);
    padding-right: var(--space-16);
  }

  .settings-body {
    flex-direction: column;
  }

  .settings-nav-pane {
    width: 100%;
    min-width: 0;
    padding: var(--space-8) var(--space-16);
    border-right: none;
    border-bottom: 1px solid var(--color-border);
  }

  .settings-nav {
    flex-direction: row;
    flex-wrap: wrap;
    gap: var(--space-8);
  }

  .settings-nav-item {
    white-space: nowrap;
  }

  .settings-content-pane {
    max-width: none;
  }
}
</style>

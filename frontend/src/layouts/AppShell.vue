<script setup lang="ts">
import { computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { mockStore, signOut } from '../mock/data'
import { useTheme } from '../utils/theme'
import AppLogo from '../components/AppLogo.vue'

const router = useRouter()
const route = useRoute()
const { isDark, toggleTheme } = useTheme()

const currentUser = computed(() => mockStore.currentUser?.name || 'Guest')

const navItems = [
  {
    name: 'Dashboard',
    path: '/dashboard',
    icon: 'M3 3h7v9H3V3zm11 0h7v5h-7V3zm0 9h7v9h-7v-9zM3 16h7v5H3v-5z'
  },
  {
    name: 'Playground',
    path: '/playground',
    icon: 'M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z'
  },
  {
    name: 'Settings',
    path: '/settings',
    icon: 'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6zm0 7a10 10 0 1 1 0-20 10 10 0 0 1 0 20z'
  }
]

function handleSignOut() {
  signOut()
  router.push('/login')
}
</script>

<template>
  <div class="shell">
    <!-- Left Sidebar -->
    <aside class="sidebar">
      <div class="sidebar-header">
        <div class="brand">
          <AppLogo :size="24" />
          <span class="brand-title">RAG Studio</span>
        </div>
      </div>

      <nav class="sidebar-nav">
        <router-link
          v-for="item in navItems"
          :key="item.path"
          :to="item.path"
          class="nav-link"
          :class="{ 'nav-link--active': route.path.startsWith(item.path) }"
        >
          <svg class="nav-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path :d="item.icon" />
          </svg>
          <span>{{ item.name }}</span>
        </router-link>
      </nav>

      <div class="sidebar-footer">
        <button
          type="button"
          class="theme-toggle-btn"
          :aria-label="isDark ? 'Switch to light theme' : 'Switch to dark theme'"
          :title="isDark ? 'Switch to light theme' : 'Switch to dark theme'"
          @click="toggleTheme"
        >
          <svg v-if="isDark" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="12" r="5"></circle>
            <line x1="12" y1="1" x2="12" y2="3"></line>
            <line x1="12" y1="21" x2="12" y2="23"></line>
            <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"></line>
            <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"></line>
            <line x1="1" y1="12" x2="3" y2="12"></line>
            <line x1="21" y1="12" x2="23" y2="12"></line>
            <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"></line>
            <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"></line>
          </svg>
          <svg v-else width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path>
          </svg>
          <span>{{ isDark ? 'Light mode' : 'Dark mode' }}</span>
        </button>

        <div class="user-row">
          <div class="user-info">
            <div class="user-avatar" aria-hidden="true">
              {{ currentUser.charAt(0).toUpperCase() }}
            </div>
            <div class="user-details">
              <span class="user-name">{{ currentUser }}</span>
              <span class="user-role">Local user</span>
            </div>
          </div>
          <button
            type="button"
            class="signout-btn"
            aria-label="Sign out"
            title="Sign out"
            @click="handleSignOut"
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"></path>
              <polyline points="16 17 21 12 16 7"></polyline>
              <line x1="21" y1="12" x2="9" y2="12"></line>
            </svg>
          </button>
        </div>
      </div>
    </aside>

    <!-- Main Content Area -->
    <main class="main-content">
      <router-view />
    </main>
  </div>
</template>

<style scoped>
.shell {
  display: flex;
  height: 100vh;
  width: 100vw;
  overflow: hidden;
  background-color: var(--color-bg);
}

.sidebar {
  width: 240px;
  min-width: 240px;
  height: 100%;
  background-color: var(--color-surface);
  border-right: 1px solid var(--color-border);
  display: flex;
  flex-direction: column;
}

.sidebar-header {
  height: 56px;
  padding: 0 var(--space-16);
  display: flex;
  align-items: center;
  border-bottom: 1px solid var(--color-border);
}

.brand {
  display: flex;
  align-items: center;
  gap: var(--space-12);
  font-weight: var(--font-weight-medium);
  font-size: var(--font-size-md);
  color: var(--color-text);
  letter-spacing: -0.01em;
}

.brand-title {
  background: linear-gradient(135deg, var(--color-text) 0%, var(--color-text-muted) 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.sidebar-nav {
  flex: 1;
  padding: var(--space-16) var(--space-8);
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  overflow-y: auto;
}

.nav-link {
  display: flex;
  align-items: center;
  gap: var(--space-12);
  padding: 0 var(--space-12);
  min-height: 40px;
  border-radius: var(--radius-sm);
  color: var(--color-text-muted);
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-medium);
  text-decoration: none;
  transition: background-color var(--transition-fast), color var(--transition-fast);
}

.nav-link:hover {
  background-color: var(--color-surface-subtle);
  color: var(--color-text);
  text-decoration: none;
}

.nav-link--active {
  background-color: var(--color-accent-subtle);
  color: var(--color-accent);
}

.nav-link--active .nav-icon {
  color: var(--color-accent);
}

.sidebar-footer {
  padding: var(--space-16);
  border-top: 1px solid var(--color-border);
  display: flex;
  flex-direction: column;
  gap: var(--space-12);
  background-color: var(--color-surface);
}

.theme-toggle-btn {
  display: flex;
  align-items: center;
  gap: var(--space-8);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--color-text-muted);
  background-color: var(--color-surface-subtle);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  padding: var(--space-8) var(--space-12);
  cursor: pointer;
  width: 100%;
  transition: background-color var(--transition-fast), color var(--transition-fast);
}

.theme-toggle-btn:hover {
  background-color: var(--color-border);
  color: var(--color-text);
}

.user-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.user-info {
  display: flex;
  align-items: center;
  gap: var(--space-8);
}

.user-avatar {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background-color: var(--color-surface-subtle);
  border: 1px solid var(--color-border);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--color-text);
}

.user-details {
  display: flex;
  flex-direction: column;
}

.user-name {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--color-text);
  line-height: 1.2;
}

.user-role {
  font-size: 11px;
  color: var(--color-text-muted);
}

.signout-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
  color: var(--color-text-muted);
  cursor: pointer;
  border: 1px solid transparent;
  background: transparent;
  transition: all var(--transition-fast);
}

.signout-btn:hover {
  background-color: var(--color-danger-subtle);
  color: var(--color-danger);
  border-color: var(--color-danger);
}

.main-content {
  flex: 1;
  min-width: 0;
  height: 100%;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  background-color: var(--color-bg);
  background-image:
    radial-gradient(ellipse at 80% 0%, rgba(59, 130, 246, 0.05) 0%, transparent 50%),
    radial-gradient(ellipse at 10% 100%, rgba(99, 102, 241, 0.03) 0%, transparent 40%);
}
</style>

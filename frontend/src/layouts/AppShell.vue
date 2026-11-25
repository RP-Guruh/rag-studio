<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { currentUser, isAdmin, logout } from '../services/auth'
import { useTheme } from '../utils/theme'
import { locale, setLocale, t } from '../i18n'
import AppLogo from '../components/AppLogo.vue'

const router = useRouter()
const route = useRoute()
const { isDark, toggleTheme } = useTheme()

const displayName = computed(() => {
  const user = currentUser.value
  return user ? user.email.split('@')[0] : '—'
})
const roleLabel = computed(() => {
  const role = currentUser.value?.role
  return role === 'admin' ? t('Administrator') : role === 'user' ? t('Member') : ''
})
const isSidebarOpen = ref(false)

function closeSidebar() {
  isSidebarOpen.value = false
}

const navItems = computed(() => [
  {
    name: t('nav.dashboard'),
    path: '/dashboard',
    icon: 'M3 3h7v9H3V3zm11 0h7v5h-7V3zm0 9h7v9h-7v-9zM3 16h7v5H3v-5z'
  },
  {
    name: t('nav.playground'),
    path: '/playground',
    icon: 'M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z'
  },
  {
    name: t('nav.settings'),
    path: '/settings',
    adminOnly: true,
    icon: 'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6zm0 7a10 10 0 1 1 0-20 10 10 0 0 1 0 20z'
  }
])

async function handleSignOut() {
  await logout()
  router.push('/login')
}
</script>

<template>
  <div class="shell">
    <button
      type="button"
      class="mobile-nav-toggle"
      :aria-label="t('Open navigation')"
      @click="isSidebarOpen = !isSidebarOpen"
    >
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
        <line x1="3" y1="6" x2="21" y2="6" />
        <line x1="3" y1="12" x2="21" y2="12" />
        <line x1="3" y1="18" x2="21" y2="18" />
      </svg>
    </button>
    <div
      v-if="isSidebarOpen"
      class="sidebar-backdrop"
      @click="closeSidebar"
    />

    <!-- Left Sidebar -->
    <aside class="sidebar" :class="{ 'sidebar--open': isSidebarOpen }">
      <div class="sidebar-header">
        <div class="brand">
          <AppLogo :size="24" />
          <span class="brand-title">{{ t('RAG Studio') }}</span>
        </div>
      </div>

      <nav class="sidebar-nav">
        <router-link
          v-for="item in navItems.filter((nav) => !nav.adminOnly || isAdmin)"
          :key="item.path"
          :to="item.path"
          class="nav-link"
          :class="{ 'nav-link--active': route.path.startsWith(item.path) }"
          @click="closeSidebar"
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
          class="theme-toggle-btn locale-toggle-btn"
          @click="setLocale(locale === 'en' ? 'id' : 'en')"
        >
          <span>{{ locale === 'en' ? t('Bahasa Indonesia') : t('English') }}</span>
        </button>
        <button
          type="button"
          class="theme-toggle-btn"
          :aria-label="isDark ? t('Switch to light theme') : t('Switch to dark theme')"
          :title="isDark ? t('Switch to light theme') : t('Switch to dark theme')"
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
          <span>{{ isDark ? t('Light mode') : t('Dark mode') }}</span>
        </button>

        <div class="user-row">
          <div class="user-info">
            <div class="user-avatar" aria-hidden="true">
              {{ displayName.charAt(0).toUpperCase() }}
            </div>
            <div class="user-details">
              <span class="user-name">{{ displayName }}</span>
              <span class="user-role">{{ roleLabel }}</span>
            </div>
          </div>
          <button
            type="button"
            class="signout-btn"
            :aria-label="t('Sign out')"
            :title="t('Sign out')"
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

.mobile-nav-toggle,
.sidebar-backdrop {
  display: none;
}

@media (max-width: 900px) {
  .sidebar {
    width: 64px;
    min-width: 64px;
  }

  .sidebar-header,
  .sidebar-footer {
    padding-left: var(--space-8);
    padding-right: var(--space-8);
  }

  .brand {
    justify-content: center;
  }

  .brand-title,
  .nav-link span,
  .theme-toggle-btn span,
  .user-details,
  .signout-btn,
  .locale-toggle-btn {
    display: none;
  }

  .nav-link,
  .theme-toggle-btn {
    justify-content: center;
    padding-left: 0;
    padding-right: 0;
  }

  .user-row {
    justify-content: center;
  }
}

@media (max-width: 600px) {
  .sidebar {
    position: fixed;
    top: 0;
    left: 0;
    z-index: 60;
    width: 220px;
    min-width: 220px;
    transform: translateX(-100%);
    transition: transform var(--transition-fast);
    box-shadow: var(--shadow-floating);
  }

  .sidebar--open {
    transform: translateX(0);
  }

  .brand-title,
  .nav-link span,
  .theme-toggle-btn span,
  .user-details {
    display: block;
  }

  .nav-link,
  .theme-toggle-btn {
    justify-content: flex-start;
    padding-left: var(--space-12);
    padding-right: var(--space-12);
  }

  .brand {
    justify-content: flex-start;
  }

  .mobile-nav-toggle {
    position: sticky;
    top: var(--space-8);
    z-index: 55;
    align-self: flex-start;
    margin: var(--space-8);
    flex-shrink: 0;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 40px;
    height: 40px;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    background-color: var(--color-surface);
    color: var(--color-text);
  }

  .sidebar-backdrop {
    position: fixed;
    inset: 0;
    z-index: 50;
    display: block;
    background-color: rgba(15, 23, 42, 0.45);
  }
}
</style>

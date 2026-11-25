import { createRouter, createWebHashHistory, type RouteRecordRaw } from 'vue-router'
import { isAuthenticated, isAdmin, isBootstrap } from '../services/auth'

import LoginPage from '../pages/LoginPage.vue'
import AppShell from '../layouts/AppShell.vue'
import DashboardPage from '../pages/DashboardPage.vue'
import ProjectDetailPage from '../pages/ProjectDetailPage.vue'
import PlaygroundPage from '../pages/PlaygroundPage.vue'
import SettingsLayout from '../pages/settings/SettingsLayout.vue'
import UsersPage from '../pages/settings/UsersPage.vue'
import HealthPage from '../pages/settings/HealthPage.vue'
import ChatModelPage from '../pages/settings/ChatModelPage.vue'
import EmbeddingModelPage from '../pages/settings/EmbeddingModelPage.vue'
import DatabasePage from '../pages/settings/DatabasePage.vue'
import DoclingPage from '../pages/settings/DoclingPage.vue'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: LoginPage
  },
  {
    path: '/',
    component: AppShell,
    children: [
      {
        path: '',
        redirect: '/dashboard'
      },
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: DashboardPage
      },
      {
        path: 'projects/:id',
        name: 'ProjectDetail',
        component: ProjectDetailPage
      },
      {
        path: 'playground',
        name: 'Playground',
        component: PlaygroundPage
      },
      {
        path: 'settings',
        component: SettingsLayout,
        children: [
          {
            path: '',
            redirect: '/settings/health'
          },
          {
            path: 'chat',
            name: 'ChatModelSettings',
            component: ChatModelPage
          },
          {
            path: 'embedding',
            name: 'EmbeddingModelSettings',
            component: EmbeddingModelPage
          },
          {
            path: 'database',
            name: 'DatabaseSettings',
            component: DatabasePage
          },
          {
            path: 'docling',
            name: 'DoclingSettings',
            component: DoclingPage
          },
          {
            path: 'users',
            name: 'UsersSettings',
            component: UsersPage
          },
          {
            path: 'health',
            name: 'HealthSettings',
            component: HealthPage
          }
        ]
      }
    ]
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/login'
  }
]

export const router = createRouter({
  history: createWebHashHistory(),
  routes
})

// Setup is reachable only before the first account exists; afterwards only the
// administrator may enter settings. Everything else needs a signed-in user.
router.beforeEach((to, _from, next) => {
  const setupRoute = to.path.startsWith('/settings')

  if (to.path === '/login') {
    if (isAuthenticated.value && !isBootstrap.value) next('/dashboard')
    else next()
    return
  }

  if (!isAuthenticated.value) {
    if (isBootstrap.value && setupRoute) next()
    else next('/login')
    return
  }

  if (setupRoute && !isAdmin.value) next('/dashboard')
  else next()
})

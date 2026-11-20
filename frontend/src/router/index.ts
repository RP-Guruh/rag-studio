import { createRouter, createWebHashHistory, type RouteRecordRaw } from 'vue-router'
import { mockStore } from '../mock/data'

import LoginPage from '../pages/LoginPage.vue'
import AppShell from '../layouts/AppShell.vue'
import DashboardPage from '../pages/DashboardPage.vue'
import ProjectDetailPage from '../pages/ProjectDetailPage.vue'
import PlaygroundPage from '../pages/PlaygroundPage.vue'
import SettingsLayout from '../pages/settings/SettingsLayout.vue'
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
            redirect: '/settings/chat'
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

// Navigation guard to ensure login screen is shown first unless signed in
router.beforeEach((to, _from, next) => {
  if (to.path !== '/login' && !mockStore.currentUser) {
    next('/login')
  } else {
    next()
  }
})

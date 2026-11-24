import { ref, computed } from 'vue'
import * as AuthService from '../../bindings/rag_project/internal/auth/service'
import type { User } from '../../bindings/rag_project/internal/auth/models'
import { waitForWailsRuntime } from './settings'

export type { User }

export const currentUser = ref<User | null>(null)
export const hasUsers = ref(false)
export const authReady = ref(false)
export const demoMode = ref(false)

export const isAuthenticated = computed(() => currentUser.value !== null)
export const isGuest = computed(() => currentUser.value?.role === 'guest')
export const isAdmin = computed(() => currentUser.value?.role === 'admin')

const demoUser: User = {
  id: 'demo',
  email: 'demo@rag-studio.local',
  role: 'guest',
  projectIds: []
}

/** Enters frontend-only demo mode. No backend or configuration required. */
export function enterDemo(): User {
  demoMode.value = true
  currentUser.value = { ...demoUser }
  authReady.value = true
  return currentUser.value
}

export function leaveDemo() {
  demoMode.value = false
  currentUser.value = null
}

export async function initAuth(): Promise<void> {
  const runtime = await waitForWailsRuntime()
  if (!runtime) {
    authReady.value = true
    return
  }
  try {
    const [users, user] = await Promise.all([
      AuthService.HasUsers(),
      AuthService.CurrentUser()
    ])
    hasUsers.value = users
    currentUser.value = user
  } catch (error) {
    // A fresh install cannot query accounts before the database is configured.
    // Treat it as "no users yet" so first-run setup stays reachable.
    hasUsers.value = false
    currentUser.value = null
  } finally {
    authReady.value = true
  }
}

export async function register(email: string, password: string): Promise<User> {
  const user = await AuthService.Register(email, password)
  if (!user) throw new Error('Registration failed')
  currentUser.value = user
  hasUsers.value = true
  return user
}

export async function login(email: string, password: string): Promise<User> {
  const user = await AuthService.Login(email, password)
  if (!user) throw new Error('Invalid email or password')
  currentUser.value = user
  return user
}

export async function logout(): Promise<void> {
  await AuthService.Logout()
  currentUser.value = null
}

export async function listUsers(): Promise<User[]> {
  return (await AuthService.ListUsers()) || []
}

export async function createUser(email: string, password: string, projectIDs: string[]): Promise<User> {
  const user = await AuthService.CreateUser(email, password, projectIDs)
  if (!user) throw new Error('Failed to create user')
  return user
}

export async function updateUserProjects(userID: string, projectIDs: string[]): Promise<void> {
  await AuthService.UpdateUserProjects(userID, projectIDs)
}

export async function deleteUser(userID: string): Promise<void> {
  await AuthService.DeleteUser(userID)
}


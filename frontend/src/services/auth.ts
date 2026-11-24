import { ref, computed } from 'vue'
import * as AuthService from '../../bindings/rag_project/internal/auth/service'
import type { User } from '../../bindings/rag_project/internal/auth/models'
import { waitForWailsRuntime } from './settings'

export type { User }

export const currentUser = ref<User | null>(null)
export const hasUsers = ref(false)
export const guestAccess = ref(true)
export const authReady = ref(false)

export const isAuthenticated = computed(() => currentUser.value !== null)
export const isGuest = computed(() => currentUser.value?.role === 'guest')
export const isAdmin = computed(() => currentUser.value?.role === 'admin')

export async function initAuth(): Promise<void> {
  const runtime = await waitForWailsRuntime()
  if (!runtime) {
    authReady.value = true
    return
  }
  try {
    const [users, user, guest] = await Promise.all([
      AuthService.HasUsers(),
      AuthService.CurrentUser(),
      AuthService.GuestAccess()
    ])
    hasUsers.value = users
    currentUser.value = user
    guestAccess.value = guest
  } catch (error) {
    console.error('Failed to load auth state:', error)
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

export async function loginGuest(): Promise<User> {
  const user = await AuthService.LoginGuest()
  if (!user) throw new Error('Guest access is not available')
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

export async function setGuestAccess(enabled: boolean): Promise<void> {
  await AuthService.SetGuestAccess(enabled)
  guestAccess.value = enabled
}

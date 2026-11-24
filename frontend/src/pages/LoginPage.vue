<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import BaseButton from '../components/BaseButton.vue'
import FormField from '../components/FormField.vue'
import AppLogo from '../components/AppLogo.vue'
import { hasUsers, guestAccess, register, login, loginGuest } from '../services/auth'
import { t } from '../i18n'

const router = useRouter()
const email = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)

const isRegister = computed(() => !hasUsers.value)

async function submit() {
  if (!email.value.trim() || !password.value) return
  error.value = ''
  busy.value = true
  try {
    if (isRegister.value) {
      await register(email.value.trim(), password.value)
    } else {
      await login(email.value.trim(), password.value)
    }
    router.push('/dashboard')
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  } finally {
    busy.value = false
  }
}

async function handleContinueAsGuest() {
  error.value = ''
  busy.value = true
  try {
    await loginGuest()
    router.push('/dashboard')
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="login-wrapper">
    <!-- Ambient subtle background glow -->
    <div class="login-glow-bg" />

    <div class="login-card">
      <div class="login-header">
        <div class="login-logo-wrap">
          <AppLogo :size="44" :animated="true" />
        </div>
        <h1 class="login-title">{{ isRegister ? t('auth.register') : t('auth.signIn') }} — RAG Studio</h1>
        <p class="login-subtitle">
          {{ isRegister ? 'The first account becomes the administrator.' : 'Vector Knowledge & Neural Retrieval' }}
        </p>
      </div>

      <div v-if="error" class="login-error">{{ error }}</div>

      <form class="login-form" @submit.prevent="submit">
        <BaseButton
          v-if="guestAccess"
          variant="primary"
          type="button"
          class="guest-login-btn"
          :disabled="busy"
          @click="handleContinueAsGuest"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4"></path>
            <polyline points="10 17 15 12 10 7"></polyline>
            <line x1="15" y1="12" x2="3" y2="12"></line>
          </svg>
          <span>{{ t('auth.guest') }}</span>
        </BaseButton>

        <div class="divider-row">
          <span class="divider-line" />
          <span class="divider-text">{{ isRegister ? 'or create the admin account' : 'or sign in with credentials' }}</span>
          <span class="divider-line" />
        </div>

        <FormField :label="t('auth.email')" id="login-email">
          <input
            id="login-email"
            v-model="email"
            type="email"
            placeholder="name@company.com"
            autocomplete="email"
          />
        </FormField>

        <FormField :label="t('auth.password')" id="login-password">
          <input
            id="login-password"
            v-model="password"
            type="password"
            placeholder="Enter password"
            autocomplete="current-password"
          />
        </FormField>

        <div class="action-block">
          <BaseButton variant="primary" type="submit" :disabled="busy || !email.trim() || !password">
            {{ busy ? '…' : (isRegister ? t('auth.register') : t('auth.signIn')) }}
          </BaseButton>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.login-wrapper {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  padding: var(--space-24);
  background-color: var(--color-bg);
  background-image:
    radial-gradient(ellipse at 50% 15%, rgba(59, 130, 246, 0.12) 0%, transparent 55%),
    radial-gradient(circle at 85% 85%, rgba(99, 102, 241, 0.07) 0%, transparent 40%);
  overflow: hidden;
}

.login-glow-bg {
  position: absolute;
  top: 10%;
  left: 50%;
  transform: translateX(-50%);
  width: 500px;
  height: 400px;
  background: radial-gradient(circle, rgba(59, 130, 246, 0.15) 0%, transparent 70%);
  filter: blur(80px);
  pointer-events: none;
}

.login-error {
  padding: var(--space-8) var(--space-12);
  border: 1px solid var(--color-danger);
  border-radius: var(--radius-sm);
  color: var(--color-danger);
  font-size: var(--font-size-sm);
}

.login-card {
  position: relative;
  z-index: 2;
  width: 100%;
  max-width: 400px;
  background-color: var(--color-surface);
  background-image: linear-gradient(180deg, rgba(255, 255, 255, 0.04) 0%, rgba(255, 255, 255, 0) 100%);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: var(--space-32);
  display: flex;
  flex-direction: column;
  gap: var(--space-24);
  box-shadow: 0 20px 40px -15px rgba(0, 0, 0, 0.35), 0 0 0 1px rgba(59, 130, 246, 0.1);
  backdrop-filter: blur(12px);
}

.login-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  gap: var(--space-8);
}

.login-logo-wrap {
  margin-bottom: var(--space-4);
  filter: drop-shadow(0 4px 12px rgba(59, 130, 246, 0.3));
}

.login-title {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-medium);
  color: var(--color-text);
  letter-spacing: -0.01em;
  margin: 0;
}

.login-subtitle {
  font-size: var(--font-size-sm);
  color: var(--color-text-muted);
  margin: 0;
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: var(--space-16);
}

.action-block {
  display: flex;
  flex-direction: column;
  gap: var(--space-8);
  margin-top: var(--space-8);
}

.status-note {
  font-size: 11px;
  color: var(--color-text-muted);
  text-align: center;
}

.divider-row {
  display: flex;
  align-items: center;
  gap: var(--space-12);
  margin: var(--space-4) 0;
}

.divider-line {
  flex: 1;
  height: 1px;
  background-color: var(--color-border);
}

.divider-text {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--color-text-muted);
  font-weight: 500;
}
</style>

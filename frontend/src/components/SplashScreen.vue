<script setup lang="ts">
import { ref, onMounted } from 'vue'
import AppLogo from './AppLogo.vue'
import { t } from '../i18n'

const emit = defineEmits<{
  (e: 'finish'): void
}>()

const progress = ref(0)
const isExiting = ref(false)

onMounted(() => {
  const interval = setInterval(() => {
    if (progress.value < 100) {
      progress.value += 2
    } else {
      clearInterval(interval)
      triggerExit()
    }
  }, 40)
})

function triggerExit() {
  if (isExiting.value) return
  isExiting.value = true
  setTimeout(() => emit('finish'), 350)
}
</script>

<template>
  <div class="splash-container" :class="{ 'splash-exiting': isExiting }">
    <!-- One restrained wave rising with the progress value. -->
    <div class="liquid-wrapper" :style="{ height: `${progress}%` }">
      <div class="wave-container">
        <svg class="wave-svg" viewBox="0 0 1440 60" preserveAspectRatio="none" aria-hidden="true">
          <path
            d="M0,30 C360,60 720,0 1080,30 C1260,45 1380,15 1440,30 L1440,60 L0,60 Z"
            fill="var(--color-accent)"
          />
        </svg>
      </div>
      <div class="liquid-body" />
    </div>

    <div class="splash-content">
      <AppLogo :size="56" />
      <div class="brand-text-group">
        <h1 class="splash-brand-title">{{ t('RAG Studio') }}</h1>
        <p class="splash-brand-tagline">{{ t('Knowledge base workspace') }}</p>
      </div>
      <div class="splash-progress-track">
        <div class="splash-progress-bar" :style="{ width: `${progress}%` }" />
      </div>
      <span class="splash-percent">{{ progress }}%</span>
    </div>
  </div>
</template>

<style scoped>
.splash-container {
  position: fixed;
  inset: 0;
  z-index: 9999;
  background-color: #0b0f19;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  user-select: none;
  transition: opacity 350ms ease, transform 350ms ease;
}

.splash-exiting {
  opacity: 0;
  transform: scale(1.01);
  pointer-events: none;
}

.liquid-wrapper {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  display: flex;
  flex-direction: column;
  pointer-events: none;
  transition: height 60ms linear;
}

.wave-container {
  position: relative;
  width: 100%;
  height: 40px;
  margin-bottom: -1px;
  flex-shrink: 0;
}

.wave-svg {
  position: absolute;
  bottom: 0;
  left: 0;
  width: 100%;
  height: 100%;
  display: block;
  opacity: 0.85;
}

.liquid-body {
  flex: 1;
  background-color: var(--color-accent);
  opacity: 0.12;
}

.splash-content {
  position: relative;
  z-index: 2;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-16);
  width: min(400px, calc(100% - 32px));
  padding: var(--space-24);
  text-align: center;
}

.brand-text-group {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.splash-brand-title {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-medium);
  letter-spacing: 0.01em;
  color: #f8fafc;
  margin: 0;
}

.splash-brand-tagline {
  font-size: var(--font-size-sm);
  color: #94a3b8;
  margin: 0;
}

.splash-progress-track {
  width: 100%;
  height: 4px;
  background-color: rgba(248, 250, 252, 0.12);
  border-radius: 9999px;
  overflow: hidden;
}

.splash-progress-bar {
  height: 100%;
  background-color: var(--color-accent);
  border-radius: 9999px;
  transition: width 50ms linear;
}

.splash-percent {
  font-size: var(--font-size-sm);
  font-family: var(--font-mono);
  color: #cbd5e1;
}
</style>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import AppLogo from './AppLogo.vue'

const emit = defineEmits<{
  (e: 'finish'): void
}>()

const statusText = ref('Initializing vector neural engine...')
const progress = ref(0)
const isExiting = ref(false)

onMounted(() => {
  const interval = setInterval(() => {
    if (progress.value < 100) {
      progress.value += 2
      if (progress.value === 28) {
        statusText.value = 'Mounting knowledge retrieval pipelines...'
      } else if (progress.value === 64) {
        statusText.value = 'Configuring local pgvector indexes...'
      } else if (progress.value === 92) {
        statusText.value = 'Welcome to RAG Studio'
      }
    } else {
      clearInterval(interval)
      triggerExit()
    }
  }, 42) // ~2.1s
})

function triggerExit() {
  if (isExiting.value) return
  isExiting.value = true
  setTimeout(() => {
    emit('finish')
  }, 450)
}

function handleSkip() {
  triggerExit()
}
</script>

<template>
  <div class="splash-container" :class="{ 'splash-exiting': isExiting }">
    <!-- Ambient background glows -->
    <div class="ambient-glow glow-top" />
    <div class="ambient-glow glow-bottom" />

    <!-- Rising Liquid Fluid Engine -->
    <div class="liquid-wrapper" :style="{ height: `${Math.min(progress * 1.05, 100)}%` }">
      <!-- Top Wave Crest Layer (Double Layered SVG) -->
      <div class="wave-container">
        <!-- Back Wave: Cyan Highlight -->
        <svg class="wave-svg wave-back" viewBox="0 0 1440 90" preserveAspectRatio="none">
          <path
            d="M0,45 C360,90 720,0 1080,45 C1260,68 1380,20 1440,45 L1440,90 L0,90 Z"
            fill="rgba(56, 189, 248, 0.4)"
          />
        </svg>

        <!-- Front Wave: Electric Blue Crest -->
        <svg class="wave-svg wave-front" viewBox="0 0 1440 90" preserveAspectRatio="none">
          <path
            d="M0,35 C320,0 640,70 960,35 C1200,10 1360,60 1440,35 L1440,90 L0,90 Z"
            fill="#2563eb"
          />
        </svg>
      </div>

      <!-- Liquid Body Mass: Fills from bottom to wave crest -->
      <div class="liquid-body">
        <!-- Rising Fluid Bubbles -->
        <span class="bubble b1" />
        <span class="bubble b2" />
        <span class="bubble b3" />
        <span class="bubble b4" />
        <span class="bubble b5" />
        <span class="bubble b6" />
        <span class="bubble b7" />
        <span class="bubble b8" />
      </div>
    </div>

    <!-- Center Content: Brand & Loading Status -->
    <div class="splash-content">
      <div class="logo-box">
        <div class="logo-aura" />
        <AppLogo :size="64" :animated="true" />
      </div>

      <div class="brand-text-group">
        <h1 class="splash-brand-title">
          <span class="title-grad">RAG</span> STUDIO
        </h1>
        <p class="splash-brand-tagline">Vector Knowledge & Neural Retrieval</p>
      </div>

      <!-- Fluid Progress Bar -->
      <div class="splash-progress-track">
        <div class="splash-progress-liquid" :style="{ width: `${progress}%` }">
          <span class="liquid-glow-head" />
        </div>
      </div>

      <!-- Status indicator -->
      <div class="splash-status-row">
        <span class="splash-status-text font-mono">{{ statusText }}</span>
        <span class="splash-percent font-mono">{{ progress }}%</span>
      </div>
    </div>

    <!-- Skip button -->
    <button type="button" class="splash-skip-btn" @click="handleSkip">
      Skip
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <polygon points="5 4 15 12 5 20 5 4"></polygon>
        <line x1="19" y1="5" x2="19" y2="19"></line>
      </svg>
    </button>
  </div>
</template>

<style scoped>
.splash-container {
  position: fixed;
  inset: 0;
  z-index: 9999;
  background-color: #070a13;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  user-select: none;
  transition: opacity 450ms cubic-bezier(0.4, 0, 0.2, 1), transform 450ms cubic-bezier(0.4, 0, 0.2, 1);
}

.splash-exiting {
  opacity: 0;
  transform: scale(1.02);
  pointer-events: none;
}

/* Ambient background lighting */
.ambient-glow {
  position: absolute;
  border-radius: 50%;
  filter: blur(90px);
  pointer-events: none;
  opacity: 0.35;
}

.glow-top {
  top: -100px;
  left: 50%;
  transform: translateX(-50%);
  width: 500px;
  height: 350px;
  background: radial-gradient(circle, rgba(59, 130, 246, 0.4) 0%, rgba(99, 102, 241, 0.1) 70%, transparent 100%);
}

.glow-bottom {
  bottom: -150px;
  left: 50%;
  transform: translateX(-50%);
  width: 700px;
  height: 400px;
  background: radial-gradient(circle, rgba(37, 99, 235, 0.3) 0%, rgba(14, 165, 233, 0.08) 70%, transparent 100%);
}

/* Rising Liquid Engine */
.liquid-wrapper {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  pointer-events: none;
  transition: height 60ms linear;
  display: flex;
  flex-direction: column;
}

/* Wave container at top of rising liquid */
.wave-container {
  position: relative;
  width: 100%;
  height: 60px;
  margin-bottom: -1px; /* Seamless overlap with body */
  flex-shrink: 0;
}

.wave-svg {
  position: absolute;
  bottom: 0;
  left: 0;
  width: 200%;
  height: 100%;
  display: block;
}

.wave-front {
  z-index: 2;
  animation: wave-slide 4s linear infinite;
}

.wave-back {
  z-index: 1;
  bottom: 4px;
  animation: wave-slide-reverse 6s linear infinite;
}

@keyframes wave-slide {
  0% { transform: translateX(0); }
  100% { transform: translateX(-50%); }
}

@keyframes wave-slide-reverse {
  0% { transform: translateX(-50%); }
  100% { transform: translateX(0); }
}

/* Continuous deep oceanic fluid body */
.liquid-body {
  position: relative;
  width: 100%;
  flex: 1;
  background: linear-gradient(180deg, #2563eb 0%, #1e40af 15%, #1e3a8a 40%, #0f172a 75%, #070a13 100%);
  overflow: hidden;
}

/* Floating Fluid Bubbles */
.bubble {
  position: absolute;
  z-index: 5;
  border-radius: 50%;
  background: radial-gradient(circle at 30% 30%, rgba(255, 255, 255, 0.9), rgba(56, 189, 248, 0.4));
  box-shadow: 0 0 10px rgba(56, 189, 248, 0.7);
  pointer-events: none;
  animation: rise-bubble 2.8s infinite linear;
}

.b1 { left: 14%; width: 12px; height: 12px; bottom: 5%; animation-duration: 2.4s; animation-delay: 0s; }
.b2 { left: 26%; width: 8px; height: 8px; bottom: 25%; animation-duration: 2.8s; animation-delay: 0.4s; }
.b3 { left: 42%; width: 14px; height: 14px; bottom: 10%; animation-duration: 2.6s; animation-delay: 0.2s; }
.b4 { left: 60%; width: 10px; height: 10px; bottom: 35%; animation-duration: 2.5s; animation-delay: 0.6s; }
.b5 { left: 74%; width: 12px; height: 12px; bottom: 15%; animation-duration: 2.7s; animation-delay: 0.3s; }
.b6 { left: 88%; width: 7px; height: 7px; bottom: 30%; animation-duration: 2.2s; animation-delay: 0.8s; }
.b7 { left: 34%; width: 10px; height: 10px; bottom: 50%; animation-duration: 2.9s; animation-delay: 0.1s; }
.b8 { left: 52%; width: 6px; height: 6px; bottom: 45%; animation-duration: 2.3s; animation-delay: 0.5s; }

@keyframes rise-bubble {
  0% {
    transform: translateY(0) scale(0.85);
    opacity: 0.9;
  }
  50% {
    transform: translateY(-80px) scale(1);
    opacity: 0.7;
  }
  100% {
    transform: translateY(-160px) scale(1.15);
    opacity: 0;
  }
}

/* Center Content with Soft Ambient Radial Lighting */
.splash-content {
  position: relative;
  z-index: 10;
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  gap: var(--space-20, 20px);
  max-width: 440px;
  width: 90%;
  padding: 32px 24px;
  background: radial-gradient(ellipse at 50% 50%, rgba(7, 10, 19, 0.75) 0%, rgba(7, 10, 19, 0.3) 65%, transparent 100%);
  animation: content-pop-in 0.8s cubic-bezier(0.16, 1, 0.3, 1) forwards;
}

@keyframes content-pop-in {
  0% {
    opacity: 0;
    transform: translateY(16px) scale(0.96);
  }
  100% {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

.logo-box {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
}

.logo-aura {
  position: absolute;
  width: 90px;
  height: 90px;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(59, 130, 246, 0.5) 0%, rgba(37, 99, 235, 0) 70%);
  animation: aura-pulse 2s infinite ease-in-out;
}

@keyframes aura-pulse {
  0%, 100% { transform: scale(0.9); opacity: 0.5; }
  50% { transform: scale(1.25); opacity: 0.95; }
}

.brand-text-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.splash-brand-title {
  font-size: 26px;
  font-weight: 700;
  letter-spacing: 0.12em;
  color: #f8fafc;
  margin: 0;
  text-shadow: 0 2px 12px rgba(0, 0, 0, 0.6);
}

.title-grad {
  background: linear-gradient(135deg, #60a5fa 0%, #38bdf8 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.splash-brand-tagline {
  font-size: 13px;
  color: #94a3b8;
  margin: 0;
  letter-spacing: 0.04em;
  text-shadow: 0 2px 8px rgba(0, 0, 0, 0.7);
}

/* Fluid Progress Track */
.splash-progress-track {
  width: 100%;
  height: 4px;
  background-color: rgba(255, 255, 255, 0.12);
  border-radius: 9999px;
  overflow: hidden;
  position: relative;
  margin-top: 4px;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.4);
}

.splash-progress-liquid {
  height: 100%;
  background: linear-gradient(90deg, #2563eb 0%, #3b82f6 50%, #38bdf8 100%);
  border-radius: 9999px;
  position: relative;
  transition: width 50ms linear;
}

.liquid-glow-head {
  position: absolute;
  right: 0;
  top: 0;
  bottom: 0;
  width: 12px;
  background: #ffffff;
  filter: blur(3px);
  border-radius: 9999px;
}

.splash-status-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  font-size: 11px;
  color: #cbd5e1;
  text-shadow: 0 1px 4px rgba(0, 0, 0, 0.7);
}

.splash-status-text {
  letter-spacing: 0.02em;
}

.splash-percent {
  color: #60a5fa;
  font-weight: 600;
}

.font-mono {
  font-family: 'IBM Plex Mono', ui-monospace, monospace;
}

/* Skip button */
.splash-skip-btn {
  position: absolute;
  top: 24px;
  right: 24px;
  z-index: 20;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background-color: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.14);
  color: #cbd5e1;
  padding: 6px 14px;
  border-radius: 9999px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  backdrop-filter: blur(8px);
  transition: all 150ms ease;
}

.splash-skip-btn:hover {
  background-color: rgba(255, 255, 255, 0.16);
  color: #ffffff;
  border-color: rgba(255, 255, 255, 0.3);
}
</style>
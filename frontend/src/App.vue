<script setup lang="ts">
import { ref, onMounted } from 'vue'
import SplashScreen from './components/SplashScreen.vue'

const showSplash = ref(true)

function handleSplashFinish() {
  showSplash.value = false
}

// Allow re-triggering splash from console or testing
if (typeof window !== 'undefined') {
  ;(window as any).replaySplash = () => {
    showSplash.value = true
  }
}
</script>

<template>
  <div class="app-root">
    <transition name="splash-fade">
      <SplashScreen v-if="showSplash" @finish="handleSplashFinish" />
    </transition>
    <div :class="{ 'app-content-ready': !showSplash }">
      <router-view />
    </div>
  </div>
</template>

<style>
.app-root {
  width: 100%;
  height: 100vh;
  position: relative;
  overflow: hidden;
}

.splash-fade-enter-active,
.splash-fade-leave-active {
  transition: opacity 400ms ease, transform 400ms ease;
}

.splash-fade-enter-from,
.splash-fade-leave-to {
  opacity: 0;
  transform: scale(1.02);
}

.app-content-ready {
  width: 100%;
  height: 100%;
  animation: app-fade-in 350ms ease-out forwards;
}

@keyframes app-fade-in {
  from {
    opacity: 0;
    transform: scale(0.99);
  }
  to {
    opacity: 1;
    transform: scale(1);
  }
}
</style>
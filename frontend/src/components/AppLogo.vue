<script setup lang="ts">
interface Props {
  size?: number | string
  animated?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  size: 32,
  animated: false
})
</script>

<template>
  <div
    class="app-logo-wrapper"
    :class="{ 'app-logo-animated': animated }"
    :style="{ width: `${size}px`, height: `${size}px` }"
  >
    <svg
      :width="size"
      :height="size"
      viewBox="0 0 48 48"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      class="app-logo-svg"
    >
      <defs>
        <!-- Gradients -->
        <linearGradient id="ragLogoGrad1" x1="6" y1="8" x2="42" y2="40" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stop-color="#60a5fa" />
          <stop offset="50%" stop-color="#3b82f6" />
          <stop offset="100%" stop-color="#1d4ed8" />
        </linearGradient>

        <linearGradient id="ragLogoGrad2" x1="12" y1="6" x2="36" y2="42" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stop-color="#93c5fd" />
          <stop offset="100%" stop-color="#3b82f6" />
        </linearGradient>

        <linearGradient id="ragLiquidGrad" x1="0" y1="0" x2="0" y2="48" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stop-color="#38bdf8" />
          <stop offset="100%" stop-color="#2563eb" />
        </linearGradient>

        <!-- Drop Shadow Filter -->
        <filter id="ragGlow" x="-20%" y="-20%" width="140%" height="140%">
          <feGaussianBlur stdDeviation="3" result="blur" />
          <feComposite in="SourceGraphic" in2="blur" operator="over" />
        </filter>
      </defs>

      <!-- Outer Vector Shield / Layered Hexagon -->
      <path
        d="M24 4L42 14V34L24 44L6 34V14L24 4Z"
        stroke="url(#ragLogoGrad1)"
        stroke-width="2.5"
        stroke-linejoin="round"
        class="logo-outline"
      />

      <!-- Layer 1: Base Knowledge Foundation -->
      <path
        d="M12 30L24 36L36 30"
        stroke="url(#ragLogoGrad1)"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        opacity="0.6"
      />

      <!-- Layer 2: Vector Embedding Mesh -->
      <path
        d="M12 24L24 30L36 24"
        stroke="url(#ragLogoGrad2)"
        stroke-width="2.2"
        stroke-linecap="round"
        stroke-linejoin="round"
        opacity="0.85"
      />

      <!-- Layer 3: Top Generation Core (Fluid Prism) -->
      <polygon
        points="24 10 36 17 24 24 12 17"
        fill="url(#ragLiquidGrad)"
        fill-opacity="0.3"
        stroke="url(#ragLogoGrad2)"
        stroke-width="2"
        stroke-linejoin="round"
      />

      <!-- Central Neural Vector Node -->
      <circle cx="24" cy="24" r="3.5" fill="#60a5fa" filter="url(#ragGlow)" />
      <circle cx="24" cy="24" r="1.75" fill="#ffffff" />

      <!-- Vector Connection Rays -->
      <line x1="24" y1="10" x2="24" y2="20" stroke="#93c5fd" stroke-width="1.5" stroke-dasharray="2 2" />
      <line x1="24" y1="28" x2="24" y2="36" stroke="#93c5fd" stroke-width="1.5" stroke-dasharray="2 2" />
    </svg>
  </div>
</template>

<style scoped>
.app-logo-wrapper {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  position: relative;
  flex-shrink: 0;
}

.app-logo-svg {
  display: block;
  filter: drop-shadow(0 2px 8px rgba(59, 130, 246, 0.25));
  transition: transform var(--transition-fast, 150ms ease), filter var(--transition-fast, 150ms ease);
}

.app-logo-wrapper:hover .app-logo-svg {
  transform: scale(1.05);
  filter: drop-shadow(0 4px 14px rgba(59, 130, 246, 0.45));
}

.app-logo-animated .logo-outline {
  animation: outline-pulse 3s infinite ease-in-out;
}

@keyframes outline-pulse {
  0%, 100% {
    stroke-opacity: 0.8;
  }
  50% {
    stroke-opacity: 1;
    filter: drop-shadow(0 0 6px rgba(96, 165, 250, 0.6));
  }
}
</style>
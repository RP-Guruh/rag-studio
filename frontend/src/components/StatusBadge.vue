<script setup lang="ts">
import { t } from '../i18n'

interface Props {
  status: 'Ready' | 'Processing' | 'Untrained' | 'Training' | 'Chunked' | 'Failed'
}

defineProps<Props>()
</script>

<template>
  <span :class="['badge', `badge--${status.toLowerCase()}`]">
    <span class="badge-dot" aria-hidden="true" />
    {{ t(status) }}
  </span>
</template>

<style scoped>
.badge {
  display: inline-flex;
  align-items: center;
  gap: var(--space-8);
  padding: 4px var(--space-12);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  border-radius: var(--radius-sm);
  line-height: 1.25;
  width: fit-content;
  border: 1px solid transparent;
}

.badge-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}

.badge--ready {
  background-color: var(--color-success-subtle);
  color: var(--color-success);
  border-color: rgba(52, 211, 153, 0.25);
}

.badge--ready .badge-dot {
  background-color: var(--color-success);
}

.badge--training {
  background-color: var(--color-accent-subtle);
  color: var(--color-accent);
  border-color: rgba(59, 130, 246, 0.25);
}

.badge--training .badge-dot {
  background-color: var(--color-accent);
  animation: pulse-dot 1.2s infinite ease-in-out;
}

.badge--untrained {
  background-color: var(--color-surface-subtle);
  color: var(--color-text-muted);
  border-color: var(--color-border);
}

.badge--untrained .badge-dot {
  background-color: var(--color-text-muted);
}

.badge--processing {
  background-color: var(--color-surface-subtle);
  color: var(--color-text-muted);
  border-color: var(--color-border);
}

.badge--processing .badge-dot {
  background-color: var(--color-accent);
  animation: pulse-dot 1s infinite ease-in-out;
}

.badge--chunked {
  background-color: var(--color-accent-subtle);
  color: var(--color-accent);
  border-color: rgba(59, 130, 246, 0.25);
}

.badge--chunked .badge-dot {
  background-color: var(--color-accent);
}

.badge--failed {
  background-color: var(--color-danger-subtle);
  color: var(--color-danger);
  border-color: rgba(220, 38, 38, 0.25);
}

.badge--failed .badge-dot {
  background-color: var(--color-danger);
}

@keyframes pulse-dot {
  0%, 100% {
    opacity: 0.3;
    transform: scale(0.85);
  }
  50% {
    opacity: 1;
    transform: scale(1.15);
  }
}
</style>

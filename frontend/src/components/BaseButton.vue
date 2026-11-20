<script setup lang="ts">
interface Props {
  variant?: 'primary' | 'secondary' | 'outline' | 'danger' | 'ghost'
  type?: 'button' | 'submit' | 'reset'
  disabled?: boolean
  size?: 'default' | 'sm'
}

withDefaults(defineProps<Props>(), {
  variant: 'secondary',
  type: 'button',
  disabled: false,
  size: 'default'
})

defineEmits<{
  (e: 'click', event: MouseEvent): void
}>()
</script>

<template>
  <button
    :type="type"
    :disabled="disabled"
    :class="['btn', `btn--${variant}`, `btn--${size}`]"
    @click="$emit('click', $event)"
  >
    <slot />
  </button>
</template>

<style scoped>
.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-8);
  font-family: inherit;
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-medium);
  line-height: 1;
  border-radius: var(--radius-sm);
  border: 1px solid transparent;
  cursor: pointer;
  transition: background-color var(--transition-fast), border-color var(--transition-fast), color var(--transition-fast);
  white-space: nowrap;
  user-select: none;
}

.btn--default {
  min-height: 40px;
  padding: 0 var(--space-16);
}

.btn--sm {
  min-height: 32px;
  padding: 0 var(--space-12);
  font-size: var(--font-size-sm);
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  pointer-events: none;
}

/* Primary */
.btn--primary {
  background-image: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);
  background-color: var(--color-accent);
  color: #ffffff;
  border-color: rgba(255, 255, 255, 0.15);
  box-shadow: 0 2px 8px rgba(37, 99, 235, 0.25);
  transition: all var(--transition-fast);
}

.btn--primary:hover:not(:disabled) {
  background-image: linear-gradient(135deg, #60a5fa 0%, #2563eb 100%);
  box-shadow: 0 4px 14px rgba(37, 99, 235, 0.4);
  transform: translateY(-1px);
}

.btn--primary:active:not(:disabled) {
  transform: translateY(0);
}

/* Secondary */
.btn--secondary {
  background-color: var(--color-surface-subtle);
  color: var(--color-text);
  border-color: var(--color-border);
}

.btn--secondary:hover:not(:disabled) {
  background-color: var(--color-border);
}

/* Outline */
.btn--outline {
  background-color: var(--color-surface);
  color: var(--color-text);
  border-color: var(--color-border);
}

.btn--outline:hover:not(:disabled) {
  background-color: var(--color-surface-subtle);
  border-color: var(--color-border-strong);
}

/* Ghost */
.btn--ghost {
  background-color: transparent;
  color: var(--color-text-muted);
}

.btn--ghost:hover:not(:disabled) {
  background-color: var(--color-surface-subtle);
  color: var(--color-text);
}

/* Danger */
.btn--danger {
  background-color: var(--color-surface);
  color: var(--color-danger);
  border-color: var(--color-border);
}

.btn--danger:hover:not(:disabled) {
  background-color: var(--color-danger-subtle);
  border-color: var(--color-danger);
}
</style>

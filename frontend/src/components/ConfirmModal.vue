<script setup lang="ts">
import BaseDialog from './BaseDialog.vue'
import BaseButton from './BaseButton.vue'

interface Props {
  open: boolean
  title?: string
  message: string
  confirmText?: string
  cancelText?: string
  variant?: 'primary' | 'danger'
}

withDefaults(defineProps<Props>(), {
  title: 'Confirmation',
  confirmText: 'Confirm',
  cancelText: 'Cancel',
  variant: 'primary'
})

defineEmits<{
  (e: 'confirm'): void
  (e: 'cancel'): void
}>()
</script>

<template>
  <BaseDialog :open="open" :title="title" @close="$emit('cancel')">
    <div class="confirm-content">
      <div v-if="variant === 'danger'" class="icon-danger">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10"></circle>
          <line x1="12" y1="8" x2="12" y2="12"></line>
          <line x1="12" y1="16" x2="12.01" y2="16"></line>
        </svg>
      </div>
      <div v-else class="icon-info">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10"></circle>
          <line x1="12" y1="16" x2="12" y2="12"></line>
          <line x1="12" y1="8" x2="12.01" y2="8"></line>
        </svg>
      </div>
      <p class="confirm-message">{{ message }}</p>
    </div>

    <template #footer>
      <BaseButton variant="outline" type="button" @click="$emit('cancel')">
        {{ cancelText }}
      </BaseButton>
      <BaseButton
        :variant="variant === 'danger' ? 'danger' : 'primary'"
        type="button"
        @click="$emit('confirm')"
      >
        {{ confirmText }}
      </BaseButton>
    </template>
  </BaseDialog>
</template>

<style scoped>
.confirm-content {
  display: flex;
  align-items: flex-start;
  gap: var(--space-16);
}

.icon-danger {
  color: var(--color-danger);
  flex-shrink: 0;
  margin-top: 2px;
}

.icon-info {
  color: var(--color-accent);
  flex-shrink: 0;
  margin-top: 2px;
}

.confirm-message {
  font-size: var(--font-size-base);
  color: var(--color-text);
  line-height: var(--line-height-normal);
  white-space: pre-line;
}
</style>

<script setup lang="ts">
interface Props {
  label: string
  id?: string
  hint?: string
  error?: string
  required?: boolean
}

defineProps<Props>()
</script>

<template>
  <div class="form-field">
    <div class="label-row">
      <label v-if="label" :for="id" class="label">
        {{ label }}
        <span v-if="required" class="required-indicator" aria-hidden="true">*</span>
      </label>
    </div>
    
    <div class="control-wrapper">
      <slot :id="id" />
    </div>

    <p v-if="error" class="error-text" role="alert">{{ error }}</p>
    <p v-else-if="hint" class="hint-text">{{ hint }}</p>
  </div>
</template>

<style scoped>
.form-field {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  width: 100%;
}

.label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.label {
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-medium);
  color: var(--color-text);
  user-select: none;
}

.required-indicator {
  color: var(--color-danger);
  margin-left: var(--space-4);
}

.control-wrapper {
  display: flex;
  flex-direction: column;
}

:deep(input[type="text"]),
:deep(input[type="email"]),
:deep(input[type="url"]),
:deep(input[type="password"]),
:deep(input[type="number"]),
:deep(select),
:deep(textarea) {
  min-height: 40px;
  padding: var(--space-8) var(--space-12);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background-color: var(--color-surface);
  color: var(--color-text);
  font-family: inherit;
  font-size: var(--font-size-base);
  transition: border-color var(--transition-fast);
  width: 100%;
}

:deep(textarea) {
  min-height: 96px;
  resize: vertical;
  line-height: var(--line-height-normal);
}

:deep(input:focus),
:deep(select:focus),
:deep(textarea:focus) {
  border-color: var(--color-accent);
  outline: none;
}

:deep(input:disabled),
:deep(select:disabled),
:deep(textarea:disabled) {
  background-color: var(--color-surface-subtle);
  color: var(--color-text-muted);
  cursor: not-allowed;
}

.hint-text {
  font-size: var(--font-size-sm);
  color: var(--color-text-muted);
  line-height: var(--line-height-normal);
}

.error-text {
  font-size: var(--font-size-sm);
  color: var(--color-danger);
}
</style>

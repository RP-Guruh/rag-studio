<script setup lang="ts">
interface TabItem {
  id: string
  label: string
}

interface Props {
  tabs: TabItem[]
  modelValue: string
}

defineProps<Props>()
defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()
</script>

<template>
  <div class="tab-bar" role="tablist">
    <button
      v-for="tab in tabs"
      :key="tab.id"
      type="button"
      role="tab"
      :aria-selected="modelValue === tab.id"
      :class="['tab-btn', { 'tab-btn--active': modelValue === tab.id }]"
      @click="$emit('update:modelValue', tab.id)"
    >
      {{ tab.label }}
    </button>
  </div>
</template>

<style scoped>
.tab-bar {
  display: flex;
  align-items: center;
  gap: var(--space-4);
  border-bottom: 1px solid var(--color-border);
  margin-bottom: var(--space-24);
}

.tab-btn {
  padding: var(--space-8) var(--space-16);
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-medium);
  color: var(--color-text-muted);
  border: none;
  background: transparent;
  cursor: pointer;
  position: relative;
  transition: color var(--transition-fast);
  min-height: 40px;
}

.tab-btn:hover {
  color: var(--color-text);
}

.tab-btn--active {
  color: var(--color-accent);
}

.tab-btn--active::after {
  content: '';
  position: absolute;
  bottom: -1px;
  left: 0;
  right: 0;
  height: 2px;
  background-color: var(--color-accent);
}
</style>

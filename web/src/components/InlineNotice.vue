<script setup>
import { computed } from 'vue'
import ConsoleIcon from './ConsoleIcon.vue'

const props = defineProps({
  tone: { type: String, default: 'info' },
  title: { type: String, default: '' },
})

const icon = computed(() => ({
  success: 'success',
  warning: 'warning',
  danger: 'error',
  error: 'error',
  info: 'info',
}[props.tone] || 'info'))
</script>

<template>
  <div class="inline-notice" :class="`inline-notice-${tone}`" :role="tone === 'danger' || tone === 'error' ? 'alert' : 'status'">
    <ConsoleIcon :name="icon" class="inline-notice-icon" />
    <div class="min-w-0">
      <strong v-if="title" class="inline-notice-title">{{ title }}</strong>
      <div class="inline-notice-copy"><slot /></div>
    </div>
    <div v-if="$slots.actions" class="inline-notice-actions"><slot name="actions" /></div>
  </div>
</template>

<script setup>
import SignalDot from './SignalDot.vue'

defineProps({
  label: { type: String, default: '' },
  value: { type: [String, Number], default: '—' },
  unit: { type: String, default: '' },
  status: { type: String, default: '' },
  accent: { type: Boolean, default: false },
})
</script>

<template>
  <div class="card p-4 relative hover:border-primary/30 transition-colors">
    <div class="flex items-center justify-between mb-3">
      <span class="tick text-xs">{{ label }}</span>
      <SignalDot v-if="status" :status="status" />
    </div>
    <div class="flex items-baseline gap-1.5">
      <span class="text-2xl font-bold tabular-nums" :class="accent ? 'text-primary' : 'text-text'">{{ value }}</span>
      <span v-if="unit" class="text-xs text-text-muted">{{ unit }}</span>
    </div>
  </div>
</template>

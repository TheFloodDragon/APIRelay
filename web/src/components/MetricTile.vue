<script setup>
defineProps({
  label: { type: String, required: true },
  value: { type: [String, Number], default: '—' },
  hint: { type: String, default: '' },
  tone: { type: String, default: 'neutral' },
})
</script>

<template>
  <article class="metric-tile" :class="`metric-tile-${tone}`">
    <div class="metric-tile-label">{{ label }}</div>
    <div class="metric-tile-value">{{ value }}</div>
    <div v-if="hint || $slots.hint" class="metric-tile-hint"><slot name="hint">{{ hint }}</slot></div>
  </article>
</template>

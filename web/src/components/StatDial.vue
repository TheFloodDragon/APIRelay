<script setup>
// StatDial —— 仪表读数卡（配线架仪表面板）
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
  <div class="meter-card" :class="accent ? 'meter-card-accent' : ''">
    <div class="flex items-center justify-between mb-3">
      <span class="tick">{{ label }}</span>
      <SignalDot v-if="status" :status="status" :size="7" />
    </div>
    <div class="flex items-baseline gap-1.5">
      <span class="text-2xl font-semibold tabular-nums font-mono" :class="accent ? 'text-brass' : 'text-t1'">{{ value }}</span>
      <span v-if="unit" class="text-xs text-t3 font-mono">{{ unit }}</span>
    </div>
    <!-- 仪表基线刻度 -->
    <div class="mt-3 flex items-center gap-[3px]" aria-hidden="true">
      <span v-for="i in 12" :key="i" class="h-1 flex-1 rounded-full" :class="i <= 8 ? (accent ? 'bg-brass/40' : 'bg-line-2') : 'bg-line'"></span>
    </div>
  </div>
</template>

<script setup>
// 图签统计格：眉标 + 等宽大数 + 单位
defineProps({
  label: { type: String, required: true },
  value: { type: [String, Number], default: '—' },
  unit: { type: String, default: '' },
  hint: { type: String, default: '' },
})
</script>

<template>
  <div class="stat-cell" :title="hint">
    <div class="stat-cell-label">{{ label }}</div>
    <div class="mt-3 flex items-baseline gap-1.5">
      <span class="font-cond text-[30px] font-semibold leading-none tracking-[-.035em] text-ink">{{ value }}</span>
      <span v-if="unit" class="font-mono text-[9px] uppercase tracking-[.12em] text-faint">{{ unit }}</span>
    </div>
    <p v-if="hint" class="mt-2 text-[10px] leading-4 text-soft">{{ hint }}</p>
  </div>
</template>

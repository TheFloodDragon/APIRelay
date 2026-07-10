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
  <div class="border border-ink bg-white px-4 py-3" :title="hint">
    <div class="eyebrow">{{ label }}</div>
    <div class="mt-1 flex items-baseline gap-1">
      <span class="font-mono text-2xl leading-none text-ink">{{ value }}</span>
      <span v-if="unit" class="font-cond text-2xs font-medium tracking-wider text-faint">{{ unit }}</span>
    </div>
  </div>
</template>

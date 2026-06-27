<script setup>
// MeterBar —— 额度 / 占比细条（油量表）
import { computed } from 'vue'

const props = defineProps({
  value: { type: Number, default: 0 },
  max: { type: Number, default: 0 },        // 0 = 不限
  unlimited: { type: Boolean, default: false },
  tone: { type: String, default: 'signal' }, // signal | auto（按占比变色）
})

const pct = computed(() => {
  if (props.unlimited || !props.max) return 0
  return Math.min(100, Math.round((props.value / props.max) * 100))
})

const barColor = computed(() => {
  if (props.tone === 'signal') return 'var(--c-signal)'
  if (pct.value >= 90) return 'var(--c-down)'
  if (pct.value >= 70) return 'var(--c-warn)'
  return 'var(--c-signal)'
})
</script>

<template>
  <div class="w-full">
    <div class="h-1 rounded-full bg-line-strong overflow-hidden">
      <div
        v-if="!unlimited && max"
        class="h-full rounded-full transition-all duration-300"
        :style="{ width: pct + '%', backgroundColor: `rgb(${barColor})` }"
      ></div>
      <!-- 不限额度：信号色虚线脉冲表达 -->
      <div v-else class="h-full w-full opacity-30" style="background: repeating-linear-gradient(90deg, rgb(var(--c-signal)) 0 6px, transparent 6px 12px)"></div>
    </div>
  </div>
</template>

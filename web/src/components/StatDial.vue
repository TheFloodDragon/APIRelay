<script setup>
// StatDial —— 仪表读数卡
// 等宽大数字 + 刻度标签 + 可选单位后缀 + 可选状态点
import SignalDot from './SignalDot.vue'

defineProps({
  label: { type: String, default: '' },     // 刻度标签
  value: { type: [String, Number], default: '—' },
  unit: { type: String, default: '' },       // 弱文单位后缀
  status: { type: String, default: '' },     // 可选状态点
  accent: { type: Boolean, default: false },  // 是否用信号色高亮数字
})
</script>

<template>
  <div class="panel p-4 relative">
    <!-- active 时左侧信号条 -->
    <span v-if="accent" class="absolute left-0 top-3 bottom-3 w-[2px] rounded-r bg-signal"></span>
    <div class="flex items-center justify-between mb-2.5">
      <span class="tick">{{ label }}</span>
      <SignalDot v-if="status" :status="status" />
    </div>
    <div class="flex items-baseline gap-1">
      <span class="font-mono text-xl font-semibold tabular-nums" :class="accent ? 'text-signal' : 'text-t1'">{{ value }}</span>
      <span v-if="unit" class="font-mono text-2xs text-t3">{{ unit }}</span>
    </div>
  </div>
</template>

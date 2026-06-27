<script setup>
// SignalDot —— 状态脉冲点
// online: 在线（绿，脉冲扩散）  warn: 降级/冷却（琥珀，常亮）  down: 故障/禁用（红，常亮）  idle: 离线（灰）
const props = defineProps({
  status: { type: String, default: 'idle' }, // online | warn | down | idle
  size: { type: Number, default: 6 },
  pulse: { type: Boolean, default: true },
})

const colorVar = {
  online: 'var(--c-online)',
  warn: 'var(--c-warn)',
  down: 'var(--c-down)',
  idle: 'var(--c-t3)',
}
</script>

<template>
  <span
    class="inline-block rounded-full shrink-0 align-middle"
    :class="status === 'online' && pulse ? 'animate-signal-pulse' : ''"
    :style="{
      width: size + 'px',
      height: size + 'px',
      backgroundColor: `rgb(${colorVar[status] || colorVar.idle})`,
    }"
  ></span>
</template>

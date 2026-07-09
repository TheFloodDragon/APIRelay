<script setup>
// PriorityBar —— 优先级刻度条 (▮▮▮▯▯)
// 用填充段数表达优先级强度，越靠前优先级越高。
const props = defineProps({
  level: { type: Number, default: 0 },   // 当前序号（0 = 最高优先级）
  total: { type: Number, default: 5 },   // 总段数
  segments: { type: Number, default: 5 },
})

// level 越小填充越满
import { computed } from 'vue'
const filled = computed(() => {
  const max = Math.max(props.total, 1)
  const ratio = 1 - Math.min(props.level, max - 1) / max
  return Math.max(1, Math.round(ratio * props.segments))
})
</script>

<template>
  <span class="inline-flex items-center gap-[2px] align-middle" :title="`优先级序位 ${level + 1}`">
    <span
      v-for="i in segments" :key="i"
      class="w-[3px] rounded-[1px] transition-colors"
      :class="i <= filled ? 'bg-brass' : 'bg-line-2'"
      :style="{ height: 4 + i * 1.5 + 'px' }"
    ></span>
  </span>
</template>

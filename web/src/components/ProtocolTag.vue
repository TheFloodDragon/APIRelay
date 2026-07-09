<script setup>
// ProtocolTag —— 协议徽标（等宽小字）
const props = defineProps({
  protocol: { type: String, default: '' },
})

// 协议 → 信号色相位（用色相区分，但都保持低饱和、克制）
const styleMap = {
  openai:    { c: 'var(--jade)' },
  anthropic: { c: 'var(--brass)' },
  responses: { c: 'var(--electric)' },
  gemini:    { c: 'var(--electric)' },
}
import { computed } from 'vue'
const cssVar = computed(() => (styleMap[props.protocol?.toLowerCase()]?.c) || 'var(--t3)')
</script>

<template>
  <span
    v-if="protocol"
    class="inline-flex items-center px-1.5 py-0.5 rounded font-mono text-[10px] font-medium leading-none border"
    :style="{
      color: `rgb(${cssVar})`,
      borderColor: `rgb(${cssVar} / 0.35)`,
      backgroundColor: `rgb(${cssVar} / 0.08)`,
    }"
  >{{ protocol }}</span>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  status: { type: String, default: 'unknown' },
  label: { type: String, default: '' },
})

const config = computed(() => ({
  healthy: { text: '正常', cls: 'border-run/20 bg-run-wash text-run', dot: 'bg-run' },
  warning: { text: '观察中', cls: 'border-test/20 bg-test-wash text-test', dot: 'bg-test' },
  error: { text: '异常', cls: 'border-trip/20 bg-trip-wash text-trip', dot: 'bg-trip' },
  disabled: { text: '已停用', cls: 'border-line bg-ghost text-soft', dot: 'bg-faint' },
  unknown: { text: '未知', cls: 'border-line bg-white text-soft', dot: 'bg-faint' },
}[props.status] || {
  text: props.status || '未知', cls: 'border-line bg-white text-soft', dot: 'bg-faint',
}))
</script>

<template>
  <span class="inline-flex items-center gap-1.5 whitespace-nowrap rounded-full border px-2 py-0.5 text-xs font-medium" :class="config.cls">
    <span class="h-1.5 w-1.5 rounded-full" :class="config.dot" aria-hidden="true"></span>
    {{ label || config.text }}
  </span>
</template>

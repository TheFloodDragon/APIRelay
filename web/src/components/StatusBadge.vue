<script setup>
import { computed } from 'vue'

const props = defineProps({
  status: { type: String, default: 'unknown' },
  label: { type: String, default: '' },
})

const config = computed(() => ({
  healthy: { text: '正常', tone: 'success' },
  success: { text: '成功', tone: 'success' },
  warning: { text: '观察中', tone: 'warning' },
  error: { text: '异常', tone: 'danger' },
  danger: { text: '异常', tone: 'danger' },
  disabled: { text: '已停用', tone: 'neutral' },
  unknown: { text: '未知', tone: 'neutral' },
}[props.status] || { text: props.status || '未知', tone: 'neutral' }))
</script>

<template>
  <span class="status-badge" :class="`status-badge-${config.tone}`">
    <span class="status-badge-dot" aria-hidden="true"></span>
    {{ label || config.text }}
  </span>
</template>

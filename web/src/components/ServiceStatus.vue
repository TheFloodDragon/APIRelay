<script setup>
import { computed } from 'vue'

const props = defineProps({
  online: { type: Boolean, default: null },
  compact: { type: Boolean, default: false },
})

const label = computed(() => props.online === true ? '服务在线' : props.online === false ? '服务不可用' : '正在检查')
const state = computed(() => props.online === true ? 'online' : props.online === false ? 'offline' : 'checking')
</script>

<template>
  <span class="service-status" :class="[{ 'service-status-compact': compact }, `service-status-${state}`]" role="status">
    <i class="service-status-dot" aria-hidden="true"></i>
    <span>{{ label }}</span>
  </span>
</template>

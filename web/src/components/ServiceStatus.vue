<script setup>
import { computed } from 'vue'

const props = defineProps({
  online: { type: Boolean, default: null },
  compact: { type: Boolean, default: false },
})

const label = computed(() => props.online === true ? '服务在线' : props.online === false ? '状态未知' : '检查中')
</script>

<template>
  <span class="service-status" :class="{ 'service-status-compact': compact }" role="status">
    <i class="status-dot" :class="online === true ? 'status-dot-live' : online === false ? 'status-dot-off' : 'status-dot-idle'" aria-hidden="true"></i>
    <span>{{ label }}</span>
  </span>
</template>

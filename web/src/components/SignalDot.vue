<script setup>
import { computed } from 'vue'

const props = defineProps({
  status: { type: String, default: 'idle' },
  pulse: { type: Boolean, default: true },
  size: { type: [Number, String], default: 8 },
})

const classes = {
  online: 'status-dot-online',
  warn: 'status-dot-warning',
  down: 'status-dot-danger',
  idle: 'bg-t3',
}

const dim = computed(() => {
  const n = Number(props.size) || 8
  return { width: n + 'px', height: n + 'px' }
})
</script>

<template>
  <span
    class="rounded-full inline-block shrink-0"
    :style="dim"
    :class="[classes[status] || classes.idle, status === 'online' && pulse ? 'animate-pulse-glow' : '']"
  ></span>
</template>

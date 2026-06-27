<script setup>
const props = defineProps({
  status: { type: String, default: 'idle' },
  pulse: { type: Boolean, default: true },
})

const classes = {
  online: 'status-dot-online',
  warn: 'status-dot-warning',
  down: 'status-dot-danger',
  idle: 'bg-text-muted',
}
</script>

<template>
  <span
    class="status-dot"
    :class="[classes[status] || classes.idle, status === 'online' && pulse ? 'animate-pulse-glow' : '']"
  ></span>
</template>

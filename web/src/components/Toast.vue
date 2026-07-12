<script setup>
import { onBeforeUnmount, ref } from 'vue'
import ConsoleIcon from './ConsoleIcon.vue'

const toasts = ref([])
const timers = new Map()
let seq = 0

function remove(id) {
  const timer = timers.get(id)
  if (timer) {
    window.clearTimeout(timer)
    timers.delete(id)
  }
  toasts.value = toasts.value.filter((toast) => toast.id !== id)
}

function add(msg, type = 'info', duration = 3200) {
  const id = ++seq
  toasts.value.push({ id, msg: String(msg ?? ''), type })
  if (duration > 0) timers.set(id, window.setTimeout(() => remove(id), duration))
  return id
}

onBeforeUnmount(() => {
  timers.forEach((timer) => window.clearTimeout(timer))
  timers.clear()
})

const meta = {
  success: { label: '操作完成', icon: 'success', tone: 'success' },
  error: { label: '操作失败', icon: 'error', tone: 'danger' },
  warn: { label: '需要注意', icon: 'warning', tone: 'warning' },
  warning: { label: '需要注意', icon: 'warning', tone: 'warning' },
  info: { label: '系统消息', icon: 'info', tone: 'info' },
}

defineExpose({ add, remove })
</script>

<template>
  <div class="toast-stack" aria-live="polite" aria-label="系统提示">
    <TransitionGroup name="toast">
      <article
        v-for="toast in toasts"
        :key="toast.id"
        class="toast-item"
        :class="`toast-item-${(meta[toast.type] || meta.info).tone}`"
        :role="toast.type === 'error' ? 'alert' : 'status'"
      >
        <ConsoleIcon :name="(meta[toast.type] || meta.info).icon" class="toast-icon" />
        <div class="min-w-0 flex-1">
          <strong>{{ (meta[toast.type] || meta.info).label }}</strong>
          <p>{{ toast.msg }}</p>
        </div>
        <button class="toast-close" type="button" aria-label="关闭提示" @click="remove(toast.id)">
          <ConsoleIcon name="x" class="h-4 w-4" />
        </button>
      </article>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.toast-enter-active,
.toast-leave-active { transition: opacity 160ms ease, transform 180ms cubic-bezier(.2,.8,.2,1); }
.toast-enter-from,
.toast-leave-to { opacity: 0; transform: translateX(12px); }
@media (prefers-reduced-motion: reduce) {
  .toast-enter-active,
  .toast-leave-active { transition: none; }
  .toast-enter-from,
  .toast-leave-to { transform: none; }
}
</style>

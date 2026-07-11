<script setup>
import { onBeforeUnmount, ref } from 'vue'

const toasts = ref([])
const timers = new Map()
let seq = 0

const add = (msg, type = 'info', duration = 3200) => {
  const id = ++seq
  toasts.value.push({ id, msg: String(msg ?? ''), type })
  if (duration > 0) timers.set(id, window.setTimeout(() => remove(id), duration))
  return id
}

const remove = (id) => {
  const timer = timers.get(id)
  if (timer) {
    window.clearTimeout(timer)
    timers.delete(id)
  }
  toasts.value = toasts.value.filter((toast) => toast.id !== id)
}

onBeforeUnmount(() => {
  timers.forEach((timer) => window.clearTimeout(timer))
  timers.clear()
})

const meta = {
  success: { label: '成功', mark: '✓', cls: 'border-run/20 bg-run-wash', markCls: 'bg-run text-white' },
  error: { label: '失败', mark: '!', cls: 'border-trip/20 bg-trip-wash', markCls: 'bg-trip text-white' },
  warn: { label: '提醒', mark: '!', cls: 'border-test/20 bg-test-wash', markCls: 'bg-test text-white' },
  warning: { label: '提醒', mark: '!', cls: 'border-test/20 bg-test-wash', markCls: 'bg-test text-white' },
  info: { label: '消息', mark: 'i', cls: 'border-blue/20 bg-blue-wash', markCls: 'bg-blue text-white' },
}

defineExpose({ add, remove })
</script>

<template>
  <div class="pointer-events-none fixed inset-x-3 bottom-3 z-[90] flex flex-col items-end gap-2 sm:inset-x-auto sm:bottom-auto sm:right-4 sm:top-4 sm:w-96" aria-live="polite" aria-label="系统提示">
    <TransitionGroup name="toast">
      <div
        v-for="toast in toasts"
        :key="toast.id"
        class="pointer-events-auto flex w-full items-start gap-3 rounded-lg border p-3 shadow-lift"
        :class="(meta[toast.type] || meta.info).cls"
        :role="toast.type === 'error' ? 'alert' : 'status'"
      >
        <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-xs font-semibold" :class="(meta[toast.type] || meta.info).markCls" aria-hidden="true">
          {{ (meta[toast.type] || meta.info).mark }}
        </span>
        <div class="min-w-0 flex-1">
          <div class="text-sm font-medium text-ink">{{ (meta[toast.type] || meta.info).label }}</div>
          <p class="mt-0.5 whitespace-pre-line break-words text-sm leading-5 text-soft">{{ toast.msg }}</p>
        </div>
        <button class="-mr-1 -mt-1 flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-lg leading-none text-soft hover:bg-white/70 hover:text-ink" aria-label="关闭提示" @click="remove(toast.id)">×</button>
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition: opacity 160ms ease, transform 160ms ease;
}

.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(8px);
}

@media (min-width: 640px) {
  .toast-enter-from,
  .toast-leave-to {
    transform: translateX(10px);
  }
}

@media (prefers-reduced-motion: reduce) {
  .toast-enter-active,
  .toast-leave-active {
    transition: none;
  }

  .toast-enter-from,
  .toast-leave-to {
    transform: none;
  }
}
</style>

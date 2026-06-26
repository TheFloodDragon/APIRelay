<script setup>
import { ref, computed, watch } from 'vue'

const toasts = ref([])
let toastId = 0

const add = (msg, type = 'info', duration = 3000) => {
  const id = toastId++
  toasts.value.push({ id, msg, type, show: false })
  setTimeout(() => {
    const t = toasts.value.find(x => x.id === id)
    if (t) t.show = true
  }, 50)
  if (duration > 0) {
    setTimeout(() => remove(id), duration)
  }
}

const remove = (id) => {
  const idx = toasts.value.findIndex(x => x.id === id)
  if (idx >= 0) {
    toasts.value[idx].show = false
    setTimeout(() => {
      toasts.value = toasts.value.filter(x => x.id !== id)
    }, 200)
  }
}

const iconMap = {
  success: '✓',
  error: '✕',
  warning: '!',
  info: 'ℹ',
}

const colorMap = {
  success: 'bg-green-50 text-green-800 border-green-200 dark:bg-green-500/15 dark:text-green-300 dark:border-green-500/30',
  error: 'bg-red-50 text-red-800 border-red-200 dark:bg-red-500/15 dark:text-red-300 dark:border-red-500/30',
  warning: 'bg-amber-50 text-amber-800 border-amber-200 dark:bg-amber-500/15 dark:text-amber-300 dark:border-amber-500/30',
  info: 'bg-blue-50 text-blue-800 border-blue-200 dark:bg-blue-500/15 dark:text-blue-300 dark:border-blue-500/30',
}

defineExpose({ add, remove })
</script>

<template>
  <div class="fixed top-4 right-4 z-[100] flex flex-col gap-2 pointer-events-none">
    <div
      v-for="t in toasts" :key="t.id"
      :class="[
        'flex items-center gap-3 px-4 py-3 rounded-xl shadow-lg border pointer-events-auto transition-all duration-200 backdrop-blur whitespace-pre-line max-w-md',
        colorMap[t.type],
        t.show ? 'opacity-100 translate-x-0' : 'opacity-0 translate-x-8'
      ]"
    >
      <span class="font-semibold text-base shrink-0">{{ iconMap[t.type] }}</span>
      <span class="text-sm font-medium">{{ t.msg }}</span>
      <button @click="remove(t.id)" class="ml-2 text-current opacity-60 hover:opacity-100 shrink-0">✕</button>
    </div>
  </div>
</template>

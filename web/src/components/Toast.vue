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

// 信号语义色：左侧细条标识 + 等宽图标
const accentMap = {
  success: 'var(--c-online)',
  error: 'var(--c-down)',
  warning: 'var(--c-warn)',
  info: 'var(--c-signal)',
}

defineExpose({ add, remove })
</script>

<template>
  <div class="fixed top-4 right-4 z-[100] flex flex-col gap-2 pointer-events-none">
    <div
      v-for="t in toasts" :key="t.id"
      class="relative flex items-start gap-2.5 pl-4 pr-3 py-2.5 rounded-lg shadow-pop border border-line bg-panel pointer-events-auto transition-all duration-200 whitespace-pre-line max-w-md overflow-hidden"
      :class="t.show ? 'opacity-100 translate-x-0' : 'opacity-0 translate-x-6'"
    >
      <span class="absolute left-0 top-0 bottom-0 w-[3px]" :style="{ backgroundColor: `rgb(${accentMap[t.type]})` }"></span>
      <span class="font-mono font-semibold text-sm shrink-0 leading-5" :style="{ color: `rgb(${accentMap[t.type]})` }">{{ iconMap[t.type] }}</span>
      <span class="text-sm text-t1 leading-5">{{ t.msg }}</span>
      <button @click="remove(t.id)" class="ml-1 text-t3 hover:text-t1 shrink-0 leading-5">✕</button>
    </div>
  </div>
</template>

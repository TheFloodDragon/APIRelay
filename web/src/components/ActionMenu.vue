<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'

const open = ref(false)
const root = ref(null)

function closeSoon() {
  window.setTimeout(() => { open.value = false }, 0)
}

function onDocPointer(event) {
  if (open.value && root.value && !root.value.contains(event.target)) open.value = false
}

function onDocKeydown(event) {
  if (open.value && event.key === 'Escape') open.value = false
}

onMounted(() => {
  // 捕获阶段监听 pointerdown，兼容鼠标、触摸与内部组件阻止冒泡的场景。
  document.addEventListener('pointerdown', onDocPointer, true)
  document.addEventListener('keydown', onDocKeydown, true)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onDocPointer, true)
  document.removeEventListener('keydown', onDocKeydown, true)
})
</script>

<template>
  <div ref="root" class="relative inline-block text-left">
    <button
      type="button"
      class="btn btn-sm"
      aria-label="打开更多操作"
      aria-haspopup="menu"
      :aria-expanded="open"
      @click.stop="open = !open"
    >更多</button>
    <div
      v-if="open"
      class="absolute right-0 z-30 mt-1 min-w-36 border border-line bg-white p-1 shadow-lift"
      role="menu"
      @click="closeSoon"
    >
      <slot />
    </div>
  </div>
</template>

<style scoped>
:deep([role='menuitem']) {
  display: block;
  width: 100%;
  padding: 0.45rem 0.6rem;
  text-align: left;
  font-size: 0.75rem;
}

:deep([role='menuitem']:hover:not(:disabled)) {
  background: var(--color-panel, #f3f5f7);
}
</style>

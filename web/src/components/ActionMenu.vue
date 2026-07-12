<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
import ConsoleIcon from './ConsoleIcon.vue'

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
  document.addEventListener('pointerdown', onDocPointer, true)
  document.addEventListener('keydown', onDocKeydown, true)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onDocPointer, true)
  document.removeEventListener('keydown', onDocKeydown, true)
})
</script>

<template>
  <div ref="root" class="action-menu">
    <button
      type="button"
      class="icon-button"
      aria-label="打开更多操作"
      aria-haspopup="menu"
      :aria-expanded="open"
      @click.stop="open = !open"
    >
      <ConsoleIcon name="ellipsis" class="h-5 w-5" />
    </button>
    <div v-if="open" class="action-menu-panel" role="menu" @click="closeSoon"><slot /></div>
  </div>
</template>

<style scoped>
:deep([role='menuitem']) {
  display: flex;
  width: 100%;
  align-items: center;
  gap: .5rem;
  padding: .5rem .625rem;
  border-radius: .25rem;
  text-align: left;
  font-size: .75rem;
  color: rgb(var(--color-text-secondary));
}
:deep([role='menuitem']:hover:not(:disabled)) {
  background: rgb(var(--color-overlay));
  color: rgb(var(--color-text));
}
</style>

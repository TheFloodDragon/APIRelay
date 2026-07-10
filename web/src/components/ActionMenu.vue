<script setup>
import { ref } from 'vue'

const open = ref(false)

function closeSoon() {
  window.setTimeout(() => { open.value = false }, 0)
}
</script>

<template>
  <div class="relative inline-block text-left">
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

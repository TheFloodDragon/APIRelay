<script setup>
import { ref, useId } from 'vue'
import ConsoleIcon from './ConsoleIcon.vue'
import { useOverlayLayer } from '../composables/useOverlayLayer'

const props = defineProps({
  open: { type: Boolean, default: false },
  title: { type: String, default: '' },
  width: { type: String, default: 'max-w-2xl' },
  persistent: { type: Boolean, default: false },
})
const emit = defineEmits(['close'])

const panel = ref(null)
const titleId = useId()

function close() {
  if (!props.persistent) emit('close')
}

// 与 Modal 共用弹层栈：抽屉里再开确认框时，Esc 必须先关内层的确认框。
useOverlayLayer({
  panel,
  isOpen: () => props.open,
  isPersistent: () => props.persistent,
  onRequestClose: () => emit('close'),
})
</script>

<template>
  <Teleport to="body">
    <Transition name="drawer">
      <div v-if="open" class="drawer-layer" @mousedown.self="close">
        <aside
          ref="panel"
          class="drawer-panel"
          :class="width"
          role="dialog"
          aria-modal="true"
          :aria-labelledby="titleId"
          tabindex="-1"
        >
          <header class="drawer-header">
            <h2 :id="titleId" class="drawer-title">{{ title }}</h2>
            <button v-if="!persistent" class="icon-button" type="button" aria-label="关闭抽屉" @click="close">
              <ConsoleIcon name="x" class="h-5 w-5" />
            </button>
          </header>
          <div class="drawer-body"><slot /></div>
          <footer v-if="$slots.footer" class="drawer-footer"><slot name="footer" /></footer>
        </aside>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.drawer-enter-active,
.drawer-leave-active { transition: opacity 180ms ease; }
.drawer-enter-active .drawer-panel,
.drawer-leave-active .drawer-panel { transition: transform 220ms cubic-bezier(.2,.8,.2,1); }
.drawer-enter-from,
.drawer-leave-to { opacity: 0; }
.drawer-enter-from .drawer-panel,
.drawer-leave-to .drawer-panel { transform: translateX(100%); }
@media (prefers-reduced-motion: reduce) {
  .drawer-enter-active,
  .drawer-leave-active,
  .drawer-enter-active .drawer-panel,
  .drawer-leave-active .drawer-panel { transition: none; }
}
</style>

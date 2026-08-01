<script setup>
import { ref, useId } from 'vue'
import ConsoleIcon from './ConsoleIcon.vue'
import { useOverlayLayer } from '../composables/useOverlayLayer'

const props = defineProps({
  open: { type: Boolean, default: false },
  title: { type: String, default: '' },
  width: { type: String, default: 'max-w-xl' },
  persistent: { type: Boolean, default: false },
})
const emit = defineEmits(['close'])

const panel = ref(null)
const titleId = useId()

function requestClose() {
  if (!props.persistent) emit('close')
}

// Esc / Tab 陷阱 / 焦点恢复 / body 滚动锁统一由弹层栈接管，
// 保证嵌套弹层时只有栈顶响应 Esc。
useOverlayLayer({
  panel,
  isOpen: () => props.open,
  isPersistent: () => props.persistent,
  onRequestClose: () => emit('close'),
})
</script>

<template>
  <Teleport to="body">
    <Transition name="modal">
      <div v-if="open" class="modal-layer" @mousedown.self="requestClose">
        <section
          ref="panel"
          class="modal-panel"
          :class="width"
          role="dialog"
          aria-modal="true"
          :aria-labelledby="titleId"
          tabindex="-1"
        >
          <header class="modal-header">
            <h2 :id="titleId" class="modal-title">{{ title }}</h2>
            <button v-if="!persistent" class="icon-button" type="button" aria-label="关闭对话框" @click="requestClose">
              <ConsoleIcon name="x" class="h-5 w-5" />
            </button>
          </header>
          <div class="modal-body"><slot /></div>
          <footer v-if="$slots.footer" class="modal-footer"><slot name="footer" /></footer>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.modal-enter-active,
.modal-leave-active { transition: opacity 160ms ease; }
.modal-enter-active .modal-panel,
.modal-leave-active .modal-panel { transition: transform 200ms cubic-bezier(.2,.8,.2,1), opacity 160ms ease; }
.modal-enter-from,
.modal-leave-to { opacity: 0; }
.modal-enter-from .modal-panel { opacity: 0; transform: translateY(12px) scale(.99); }
.modal-leave-to .modal-panel { opacity: 0; transform: translateY(6px) scale(.995); }
@media (prefers-reduced-motion: reduce) {
  .modal-enter-active,
  .modal-leave-active,
  .modal-enter-active .modal-panel,
  .modal-leave-active .modal-panel { transition: none; }
}
</style>

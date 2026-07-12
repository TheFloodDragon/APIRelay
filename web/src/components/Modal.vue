<script setup>
import { nextTick, onBeforeUnmount, ref, useId, watch } from 'vue'
import ConsoleIcon from './ConsoleIcon.vue'

const props = defineProps({
  open: { type: Boolean, default: false },
  title: { type: String, default: '' },
  width: { type: String, default: 'max-w-xl' },
  persistent: { type: Boolean, default: false },
})
const emit = defineEmits(['close'])

const panel = ref(null)
const titleId = useId()
const focusSelector = 'button:not(:disabled), [href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])'
let previousFocus = null
let previousOverflow = ''

function requestClose() {
  if (!props.persistent) emit('close')
}

function onKeydown(event) {
  if (event.key === 'Escape') {
    if (!props.persistent) {
      event.stopPropagation()
      emit('close')
    }
    return
  }
  if (event.key !== 'Tab' || !panel.value) return
  const focusable = [...panel.value.querySelectorAll(focusSelector)].filter((element) => element.offsetParent !== null)
  if (!focusable.length) {
    event.preventDefault()
    panel.value.focus()
    return
  }
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}

watch(
  () => props.open,
  async (open) => {
    if (open) {
      previousFocus = document.activeElement
      previousOverflow = document.body.style.overflow
      document.body.style.overflow = 'hidden'
      document.addEventListener('keydown', onKeydown, true)
      await nextTick()
      const target = panel.value?.querySelector('[data-autofocus]') || panel.value?.querySelector(focusSelector)
      ;(target || panel.value)?.focus()
    } else {
      document.removeEventListener('keydown', onKeydown, true)
      document.body.style.overflow = previousOverflow
      previousFocus?.focus?.()
      previousFocus = null
    }
  },
  { immediate: true }
)

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown, true)
  document.body.style.overflow = previousOverflow
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

<script setup>
import { nextTick, onBeforeUnmount, ref, useId, watch } from 'vue'

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
let prevFocus = null
let previousOverflow = ''

const requestClose = () => {
  if (!props.persistent) emit('close')
}

const onKeydown = (event) => {
  if (event.key === 'Escape') {
    if (!props.persistent) {
      event.stopPropagation()
      emit('close')
    }
    return
  }
  if (event.key === 'Tab' && panel.value) {
    const focusables = [...panel.value.querySelectorAll(focusSelector)].filter((element) => element.offsetParent !== null)
    if (!focusables.length) {
      event.preventDefault()
      panel.value.focus()
      return
    }
    const first = focusables[0]
    const last = focusables[focusables.length - 1]
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault()
      last.focus()
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault()
      first.focus()
    }
  }
}

watch(
  () => props.open,
  async (open) => {
    if (open) {
      prevFocus = document.activeElement
      previousOverflow = document.body.style.overflow
      document.body.style.overflow = 'hidden'
      document.addEventListener('keydown', onKeydown, true)
      await nextTick()
      const target = panel.value?.querySelector('[data-autofocus]') || panel.value?.querySelector(focusSelector)
      ;(target || panel.value)?.focus()
    } else {
      document.removeEventListener('keydown', onKeydown, true)
      document.body.style.overflow = previousOverflow
      prevFocus?.focus?.()
      prevFocus = null
    }
  }
)

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown, true)
  document.body.style.overflow = previousOverflow
})
</script>

<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="open"
        class="fixed inset-0 z-[80] flex items-end justify-center bg-ink/35 sm:items-center sm:p-6"
        @mousedown.self="requestClose"
      >
        <div
          ref="panel"
          class="modal-panel flex h-[100dvh] w-full flex-col overflow-hidden bg-white shadow-lift sm:max-h-[calc(100vh-3rem)] sm:h-auto sm:rounded-xl sm:border sm:border-line"
          :class="width"
          role="dialog"
          aria-modal="true"
          :aria-labelledby="titleId"
          tabindex="-1"
        >
          <header class="flex shrink-0 items-center justify-between gap-3 border-b border-line bg-white px-4 py-3 sm:px-5">
            <h2 :id="titleId" class="min-w-0 truncate text-base font-semibold text-ink">{{ title }}</h2>
            <button v-if="!persistent" class="btn btn-ghost btn-sm shrink-0" aria-label="关闭对话框" @click="emit('close')">
              <span aria-hidden="true" class="text-lg leading-none">×</span>
              关闭
            </button>
          </header>
          <div class="min-h-0 flex-1 overflow-y-auto overscroll-contain p-4 sm:p-5">
            <slot />
          </div>
          <footer v-if="$slots.footer" class="sticky bottom-0 z-10 shrink-0 border-t border-line bg-white px-4 py-3 shadow-[0_-8px_20px_rgba(16,24,40,0.06)] sm:px-5">
            <div class="flex items-center justify-end gap-2">
              <slot name="footer" />
            </div>
          </footer>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.modal-enter-active,
.modal-leave-active { transition: opacity 180ms ease; }
.modal-enter-active .modal-panel,
.modal-leave-active .modal-panel { transition: transform 240ms cubic-bezier(.2,.8,.2,1), opacity 180ms ease; }
.modal-enter-from,
.modal-leave-to { opacity: 0; }
.modal-enter-from .modal-panel { opacity: 0; transform: translateY(18px) scale(.985); }
.modal-leave-to .modal-panel { opacity: 0; transform: translateY(8px) scale(.99); }

@media (prefers-reduced-motion: reduce) {
  .modal-enter-active,
  .modal-leave-active,
  .modal-enter-active .modal-panel,
  .modal-leave-active .modal-panel { transition: none; }
  .modal-enter-from .modal-panel,
  .modal-leave-to .modal-panel { transform: none; }
}
</style>

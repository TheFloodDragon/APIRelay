<script setup>
import { nextTick, onBeforeUnmount, ref, useId, watch } from 'vue'

const props = defineProps({
  open: { type: Boolean, default: false },
  title: { type: String, default: '' },
})
const emit = defineEmits(['close'])

const panel = ref(null)
const titleId = useId()
const focusSelector = 'button:not(:disabled), [href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])'
let previousFocus = null
let previousOverflow = ''

function close() {
  emit('close')
}

function onKeydown(event) {
  if (event.key === 'Escape') {
    event.stopPropagation()
    close()
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
  }
)

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown, true)
  document.body.style.overflow = previousOverflow
})
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-[80] bg-ink/30" @mousedown.self="close">
      <aside
        ref="panel"
        class="absolute inset-y-0 right-0 flex w-full max-w-2xl flex-col border-l border-line bg-white shadow-lift"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="titleId"
        tabindex="-1"
      >
        <header class="flex shrink-0 items-center justify-between gap-3 border-b border-line px-4 py-3 sm:px-5">
          <h2 :id="titleId" class="min-w-0 text-lg font-semibold text-ink">{{ title }}</h2>
          <button class="btn btn-sm shrink-0" type="button" aria-label="关闭详情" @click="close">关闭</button>
        </header>
        <div class="min-h-0 flex-1 overflow-y-auto p-4 sm:p-5">
          <slot />
        </div>
        <footer v-if="$slots.footer" class="shrink-0 border-t border-line bg-white px-4 py-3 sm:px-5">
          <slot name="footer" />
        </footer>
      </aside>
    </div>
  </Teleport>
</template>

import { readonly, ref } from 'vue'

const state = ref({ open: false, title: '', message: '', confirmLabel: '确认', tone: 'danger' })
let resolvePending = null

export function confirmAction(options = {}) {
  if (resolvePending) resolvePending(false)
  state.value = {
    open: true,
    title: options.title || '确认操作',
    message: options.message || '',
    confirmLabel: options.confirmLabel || '确认',
    tone: options.tone || 'danger',
  }
  return new Promise((resolve) => { resolvePending = resolve })
}

export function settleConfirm(result) {
  state.value = { ...state.value, open: false }
  resolvePending?.(result)
  resolvePending = null
}

export function useConfirmState() {
  return { confirmState: readonly(state), settleConfirm }
}

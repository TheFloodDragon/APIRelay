import { getCurrentInstance } from 'vue'

export function useToast() {
  const instance = getCurrentInstance()
  const toast = instance?.appContext.config.globalProperties.$toast

  return {
    success: (msg, duration) => toast?.add(msg, 'success', duration),
    error: (msg, duration) => toast?.add(msg, 'error', duration),
    warning: (msg, duration) => toast?.add(msg, 'warning', duration),
    info: (msg, duration) => toast?.add(msg, 'info', duration),
  }
}

import { getCurrentInstance } from 'vue'

export function useToast() {
  const instance = getCurrentInstance()
  const toast = instance?.appContext.config.globalProperties.$toast

  return {
    success: (msg) => toast?.add(msg, 'success'),
    error: (msg) => toast?.add(msg, 'error'),
    warning: (msg) => toast?.add(msg, 'warning'),
    info: (msg) => toast?.add(msg, 'info'),
  }
}

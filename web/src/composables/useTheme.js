import { ref } from 'vue'

const THEME_KEY = 'apirelay_theme'
const theme = ref('light')

function apply(value) {
  const root = document.documentElement
  if (value === 'dark') {
    root.classList.add('dark')
  } else {
    root.classList.remove('dark')
  }
  theme.value = value
}

// 初始化：读取本地偏好，否则跟随系统
export function initTheme() {
  const saved = localStorage.getItem(THEME_KEY)
  if (saved === 'dark' || saved === 'light') {
    apply(saved)
    return
  }
  const prefersDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches
  apply(prefersDark ? 'dark' : 'light')
}

export function useTheme() {
  function toggle() {
    const next = theme.value === 'dark' ? 'light' : 'dark'
    localStorage.setItem(THEME_KEY, next)
    apply(next)
  }
  function set(value) {
    localStorage.setItem(THEME_KEY, value)
    apply(value)
  }
  return { theme, toggle, set }
}

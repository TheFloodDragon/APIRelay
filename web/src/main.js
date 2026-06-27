import { createApp } from 'vue'
import App from './App.vue'
import router from './router'

// 自托管字体（仅打包 latin 子集 + 需要的字重，保证 embed 单文件、离线、精简）
import '@fontsource/ibm-plex-mono/latin-400.css'
import '@fontsource/ibm-plex-mono/latin-500.css'
import '@fontsource/ibm-plex-mono/latin-600.css'
import '@fontsource/ibm-plex-sans/latin-400.css'
import '@fontsource/ibm-plex-sans/latin-500.css'
import '@fontsource/ibm-plex-sans/latin-600.css'

import './style.css'
import { initTheme } from './composables/useTheme'

// 主题需在挂载前初始化，避免闪烁
initTheme()

const app = createApp(App)
app.use(router)

// 挂载 Toast
import Toast from './components/Toast.vue'
import { ref } from 'vue'
const toastRef = ref(null)
app.config.globalProperties.$toast = {
  add: (...args) => toastRef.value?.add(...args),
}
app.component('Toast', Toast)
app.provide('toastRef', toastRef)

app.mount('#app')

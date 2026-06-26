import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import './style.css'

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

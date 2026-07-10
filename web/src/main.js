import { createApp, ref } from 'vue'
import App from './App.vue'
import router from './router'

import '@fontsource/saira-semi-condensed/latin-500.css'
import '@fontsource/saira-semi-condensed/latin-600.css'
import '@fontsource/spline-sans-mono/latin-400.css'
import '@fontsource/spline-sans-mono/latin-500.css'
import './style.css'

import Toast from './components/Toast.vue'

const app = createApp(App)
app.use(router)

const toastRef = ref(null)
app.config.globalProperties.$toast = { add: (...args) => toastRef.value?.add(...args) }
app.component('Toast', Toast)
app.provide('toastRef', toastRef)
app.mount('#app')

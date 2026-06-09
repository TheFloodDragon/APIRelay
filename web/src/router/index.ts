import { createRouter, createWebHistory } from 'vue-router'
import Channels from '@/views/Channels.vue'
import Dashboard from '@/views/Dashboard.vue'
import Logs from '@/views/Logs.vue'
import Models from '@/views/Models.vue'
import Proxy from '@/views/Proxy.vue'
import Settings from '@/views/Settings.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/dashboard' },
    { path: '/dashboard', component: Dashboard },
    { path: '/channels', component: Channels },
    { path: '/models', component: Models },
    { path: '/proxy', component: Proxy },
    { path: '/settings', component: Settings },
    { path: '/logs', component: Logs }
  ]
})

export default router

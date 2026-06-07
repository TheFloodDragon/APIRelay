import { createRouter, createWebHistory } from 'vue-router'
import Channels from '@/views/Channels.vue'
import Dashboard from '@/views/Dashboard.vue'
import Logs from '@/views/Logs.vue'
import Models from '@/views/Models.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/dashboard' },
    { path: '/dashboard', component: Dashboard },
    { path: '/channels', component: Channels },
    { path: '/models', component: Models },
    { path: '/logs', component: Logs }
  ]
})

export default router

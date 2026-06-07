import { createRouter, createWebHistory } from 'vue-router'
import Dashboard from '@/views/Dashboard.vue'
import Channels from '@/views/Channels.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/channels' },
    { path: '/dashboard', component: Dashboard },
    { path: '/channels', component: Channels }
  ]
})

export default router

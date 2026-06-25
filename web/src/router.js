import { createRouter, createWebHashHistory } from 'vue-router'
import Login from './views/Login.vue'
import Dashboard from './views/Dashboard.vue'
import Channels from './views/Channels.vue'
import Tokens from './views/Tokens.vue'
import Logs from './views/Logs.vue'

const routes = [
  { path: '/login', name: 'login', component: Login, meta: { public: true } },
  { path: '/', redirect: '/dashboard' },
  { path: '/dashboard', name: 'dashboard', component: Dashboard },
  { path: '/channels', name: 'channels', component: Channels },
  { path: '/tokens', name: 'tokens', component: Tokens },
  { path: '/logs', name: 'logs', component: Logs },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

router.beforeEach((to) => {
  const authed = !!localStorage.getItem('session_token')
  if (!to.meta.public && !authed) {
    return { name: 'login' }
  }
  if (to.name === 'login' && authed) {
    return { name: 'dashboard' }
  }
})

export default router

import { createRouter, createWebHashHistory } from 'vue-router'
import Login from './views/Login.vue'
import Dashboard from './views/Dashboard.vue'
import Channels from './views/Channels.vue'
import Models from './views/Models.vue'
import Tokens from './views/Tokens.vue'
import Settings from './views/Settings.vue'
import Logs from './views/Logs.vue'

export const sheets = [
  { path: '/dashboard', name: 'dashboard', label: '总览', icon: 'M4 13h6V4H4v9Zm0 7h6v-4H4v4Zm10 0h6v-9h-6v9Zm0-16v4h6V4h-6Z', component: Dashboard },
  { path: '/channels', name: 'channels', label: '渠道', icon: 'M5 7h8m4 0h2M5 17h2m4 0h8M13 4v6M8 14v6', component: Channels },
  { path: '/models', name: 'models', label: '模型', icon: 'm12 3 8 4.5-8 4.5-8-4.5L12 3Zm-8 9 8 4.5 8-4.5M4 16.5l8 4.5 8-4.5', component: Models },
  { path: '/tokens', name: 'tokens', label: '令牌', icon: 'M14.5 8.5a4 4 0 1 1-3-3.87M14 10l7-7m-3 0h3v3M10.5 12.5 4 19v2h3l6.5-6.5', component: Tokens },
  { path: '/logs', name: 'logs', label: '日志', icon: 'M5 4h14v16H5zM8 8h8m-8 4h8m-8 4h5', component: Logs },
  { path: '/settings', name: 'settings', label: '设置', icon: 'M12 15.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7ZM19 13v-2l2-1-2-3-2 1a8 8 0 0 0-2-1l-.5-2h-5L9 7a8 8 0 0 0-2 1L5 7l-2 3 2 1v2l-2 1 2 3 2-1a8 8 0 0 0 2 1l.5 2h5l.5-2a8 8 0 0 0 2-1l2 1 2-3-2-1Z', component: Settings },
]

const routes = [
  { path: '/login', name: 'login', component: Login, meta: { public: true } },
  { path: '/', redirect: '/dashboard' },
  ...sheets.map((item) => ({ path: item.path, name: item.name, component: item.component })),
]

const router = createRouter({ history: createWebHashHistory(), routes })

router.beforeEach((to) => {
  const authed = !!localStorage.getItem('apirelay_session')
  if (!to.meta.public && !authed) return { name: 'login' }
  if (to.name === 'login' && authed) return { name: 'dashboard' }
})

export default router

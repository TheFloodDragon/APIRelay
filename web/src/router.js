import { createRouter, createWebHashHistory } from 'vue-router'
import Login from './views/Login.vue'
import Dashboard from './views/Dashboard.vue'
import Channels from './views/Channels.vue'
import Models from './views/Models.vue'
import Tokens from './views/Tokens.vue'
import Settings from './views/Settings.vue'
import Logs from './views/Logs.vue'

export const sheets = [
  { path: '/dashboard', name: 'dashboard', label: '运行总览', shortLabel: '总览', icon: 'dashboard', component: Dashboard },
  { path: '/channels', name: 'channels', label: '渠道工作台', shortLabel: '渠道', icon: 'server', component: Channels },
  { path: '/models', name: 'models', label: '模型库存', shortLabel: '模型', icon: 'models', component: Models },
  { path: '/tokens', name: 'tokens', label: '凭据管理', shortLabel: '令牌', icon: 'tokens', component: Tokens },
  { path: '/logs', name: 'logs', label: '诊断日志', shortLabel: '日志', icon: 'logs', component: Logs },
  { path: '/settings', name: 'settings', label: '配置中心', shortLabel: '设置', icon: 'settings', component: Settings },
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

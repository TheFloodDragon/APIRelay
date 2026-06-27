<script setup>
import { computed, inject } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { TOKEN_KEY } from './api'
import { useTheme } from './composables/useTheme'
import SignalDot from './components/SignalDot.vue'

const route = useRoute()
const router = useRouter()
const isLogin = computed(() => route.name === 'login')
const toastRef = inject('toastRef')
const { theme, toggle } = useTheme()

const VERSION = 'v0.1.0'

const nav = [
  { name: 'dashboard', label: '信号总览', icon: 'M3 13h8V3H3v10zm0 8h8v-6H3v6zm10 0h8V11h-8v10zm0-18v6h8V3h-8z', path: '/dashboard' },
  { name: 'channels', label: '路由表', icon: 'M4 6a2 2 0 012-2h12a2 2 0 012 2v2H4V6zm0 4h16v8a2 2 0 01-2 2H6a2 2 0 01-2-2v-8zm4 3h4v2H8v-2z', path: '/channels' },
  { name: 'models', label: '信号矩阵', icon: 'M12 2l9 5v10l-9 5-9-5V7l9-5zm0 2.3L5 8v8l7 3.9 7-3.9V8l-7-3.7z', path: '/models' },
  { name: 'tokens', label: '令牌', icon: 'M7 14a3 3 0 100-6 3 3 0 000 6zm5-3h9v2h-2v3h-2v-3h-2v3h-2v-5zm-5 1a1 1 0 110-2 1 1 0 010 2z', path: '/tokens' },
  { name: 'settings', label: '规则', icon: 'M12 8a4 4 0 100 8 4 4 0 000-8zm9 4l-2 1.5.3 2.5-2.4.8-1.2 2.2-2.5-.5L12 22l-1.7-1.7-2.5.5-1.2-2.2-2.4-.8.3-2.5L2 12l2-1.5L3.7 8l2.4-.8L7.3 5l2.5.5L12 2l1.7 1.7 2.5-.5 1.2 2.2 2.4.8-.3 2.5L21 12z', path: '/settings' },
  { name: 'logs', label: '信号流水', icon: 'M4 4h16v2H4V4zm0 5h16v2H4V9zm0 5h10v2H4v-2zm0 5h10v2H4v-2z', path: '/logs' },
]

const activeLabel = computed(() => nav.find(n => n.name === route.name)?.label || '')
const activeIdx = computed(() => {
  const i = nav.findIndex(n => n.name === route.name)
  return i < 0 ? 0 : i
})

function logout() {
  localStorage.removeItem(TOKEN_KEY)
  router.push('/login')
}
</script>

<template>
  <div>
    <Toast :ref="(el) => (toastRef = el)" />

    <div v-if="isLogin">
      <router-view />
    </div>

    <div v-else class="min-h-screen flex">
      <!-- 侧边栏 -->
      <aside class="w-64 shrink-0 flex flex-col bg-surface border-r border-border">
        <!-- Logo -->
        <div class="h-16 px-6 flex items-center gap-3 border-b border-border">
          <div class="w-10 h-10 rounded-lg bg-gradient-to-br from-primary to-accent flex items-center justify-center">
            <svg viewBox="0 0 24 24" class="w-6 h-6 text-white" fill="currentColor">
              <path d="M13 2L4.5 13.5h6L9 22l9.5-12h-6z"/>
            </svg>
          </div>
          <div>
            <div class="text-base font-semibold text-text">APIRelay</div>
            <div class="text-xs text-text-muted">信号路由</div>
          </div>
        </div>

        <!-- 导航 -->
        <nav class="flex-1 p-3 space-y-1 overflow-y-auto">
          <router-link
            v-for="n in nav" :key="n.name" :to="n.path"
            class="flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-all"
            :class="route.name === n.name
              ? 'bg-primary text-white shadow-lg shadow-primary/30'
              : 'text-text-dim hover:text-text hover:bg-elevated'"
          >
            <svg viewBox="0 0 24 24" class="w-5 h-5" fill="currentColor"><path :d="n.icon"/></svg>
            <span>{{ n.label }}</span>
          </router-link>
        </nav>

        <!-- 底部 -->
        <div class="p-3 border-t border-border space-y-2">
          <div class="flex items-center gap-2 px-3 py-2">
            <div class="status-dot status-dot-online"></div>
            <span class="text-xs text-text-muted">{{ VERSION }}</span>
          </div>
          <button @click="logout"
            class="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium text-text-dim hover:text-danger hover:bg-danger/10 transition-all">
            <svg viewBox="0 0 24 24" class="w-5 h-5" fill="currentColor">
              <path d="M16 17v-3H9v-4h7V7l5 5-5 5zM14 2a2 2 0 012 2v2h-2V4H5v16h9v-2h2v2a2 2 0 01-2 2H5a2 2 0 01-2-2V4a2 2 0 012-2h9z"/>
            </svg>
            <span>退出</span>
          </button>
        </div>
      </aside>

      <!-- 主内容 -->
      <main class="flex-1 flex flex-col overflow-hidden">
        <!-- 顶部栏 -->
        <header class="h-16 shrink-0 px-6 flex items-center justify-between bg-panel/50 backdrop-blur-sm border-b border-border">
          <h1 class="text-xl font-semibold text-text">{{ activeLabel }}</h1>
          <div class="flex items-center gap-4">
            <div class="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-elevated">
              <div class="w-8 h-8 rounded-lg bg-primary/20 flex items-center justify-center text-primary text-sm font-semibold">A</div>
              <span class="text-sm text-text">admin</span>
            </div>
          </div>
        </header>

        <div class="flex-1 overflow-auto p-6">
          <div class="animate-fade-in max-w-7xl mx-auto">
            <router-view />
          </div>
        </div>
      </main>
    </div>
  </div>
</template>

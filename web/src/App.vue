<script setup>
import { computed, inject } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { TOKEN_KEY } from './api'
import { useTheme } from './composables/useTheme'

const route = useRoute()
const router = useRouter()
const isLogin = computed(() => route.name === 'login')
const toastRef = inject('toastRef')
const { theme, toggle } = useTheme()

const nav = [
  { name: 'dashboard', label: '仪表盘', icon: 'M3 13h8V3H3v10zm0 8h8v-6H3v6zm10 0h8V11h-8v10zm0-18v6h8V3h-8z', path: '/' },
  { name: 'channels', label: '供应商', icon: 'M4 6a2 2 0 012-2h12a2 2 0 012 2v2H4V6zm0 4h16v8a2 2 0 01-2 2H6a2 2 0 01-2-2v-8zm4 3h4v2H8v-2z', path: '/channels' },
  { name: 'models', label: '模型', icon: 'M12 2l9 5v10l-9 5-9-5V7l9-5zm0 2.3L5 8v8l7 3.9 7-3.9V8l-7-3.7z', path: '/models' },
  { name: 'tokens', label: '令牌', icon: 'M7 14a3 3 0 100-6 3 3 0 000 6zm5-3h9v2h-2v3h-2v-3h-2v3h-2v-5zm-5 1a1 1 0 110-2 1 1 0 010 2z', path: '/tokens' },
  { name: 'settings', label: '设置', icon: 'M12 8a4 4 0 100 8 4 4 0 000-8zm9 4l-2 1.5.3 2.5-2.4.8-1.2 2.2-2.5-.5L12 22l-1.7-1.7-2.5.5-1.2-2.2-2.4-.8.3-2.5L2 12l2-1.5L3.7 8l2.4-.8L7.3 5l2.5.5L12 2l1.7 1.7 2.5-.5 1.2 2.2 2.4.8-.3 2.5L21 12z', path: '/settings' },
  { name: 'logs', label: '日志', icon: 'M4 4h16v2H4V4zm0 5h16v2H4V9zm0 5h10v2H4v-2zm0 5h10v2H4v-2z', path: '/logs' },
]

const breadcrumb = computed(() => {
  const item = nav.find(n => n.name === route.name)
  return item ? [{ label: '首页', path: '/' }, { label: item.label }] : []
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

    <div v-else class="min-h-screen flex bg-ink-50 dark:bg-ink-950 bg-mesh-light dark:bg-mesh-dark">
      <!-- 侧边栏 -->
      <aside class="w-64 shrink-0 flex flex-col border-r border-ink-100 dark:border-ink-800/80 bg-white/80 dark:bg-ink-900/60 backdrop-blur-xl">
        <div class="px-6 py-6">
          <div class="flex items-center gap-3">
            <div class="w-10 h-10 rounded-xl bg-brand-gradient flex items-center justify-center shadow-glow text-white">
              <svg viewBox="0 0 24 24" class="w-5 h-5" fill="currentColor"><path d="M13 2L4.5 13.5h6L9 22l9.5-12h-6z"/></svg>
            </div>
            <div>
              <h1 class="text-lg font-bold text-ink-900 dark:text-ink-50 leading-tight">APIRelay</h1>
              <p class="text-[11px] text-ink-400 dark:text-ink-500">AI 模型聚合中转</p>
            </div>
          </div>
        </div>

        <nav class="flex-1 px-3 space-y-1 overflow-y-auto">
          <router-link
            v-for="n in nav" :key="n.name" :to="n.path"
            class="group relative flex items-center gap-3 px-4 py-2.5 rounded-xl text-sm font-medium transition-all duration-200"
            :class="route.name === n.name
              ? 'text-brand-700 dark:text-white bg-brand-50 dark:bg-brand-600/20 shadow-sm'
              : 'text-ink-600 dark:text-ink-400 hover:bg-ink-100/70 dark:hover:bg-ink-800/60 hover:text-ink-900 dark:hover:text-ink-100'"
          >
            <span v-if="route.name === n.name" class="absolute left-0 top-1/2 -translate-y-1/2 h-5 w-1 rounded-r-full bg-brand-gradient"></span>
            <svg viewBox="0 0 24 24" class="w-5 h-5 shrink-0 transition-transform group-hover:scale-110" fill="currentColor"><path :d="n.icon"/></svg>
            <span>{{ n.label }}</span>
          </router-link>
        </nav>

        <div class="p-3 border-t border-ink-100 dark:border-ink-800/80">
          <button @click="logout"
            class="w-full flex items-center gap-3 px-4 py-2.5 rounded-xl text-sm font-medium text-ink-500 dark:text-ink-400 hover:bg-red-50 dark:hover:bg-red-500/10 hover:text-red-600 dark:hover:text-red-400 transition-all">
            <svg viewBox="0 0 24 24" class="w-5 h-5" fill="currentColor"><path d="M16 17v-3H9v-4h7V7l5 5-5 5zM14 2a2 2 0 012 2v2h-2V4H5v16h9v-2h2v2a2 2 0 01-2 2H5a2 2 0 01-2-2V4a2 2 0 012-2h9z"/></svg>
            <span>退出登录</span>
          </button>
        </div>
      </aside>

      <!-- 主内容区 -->
      <main class="flex-1 flex flex-col overflow-hidden min-w-0">
        <header class="px-6 py-4 border-b border-ink-100 dark:border-ink-800/80 bg-white/70 dark:bg-ink-900/50 backdrop-blur-xl">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2 text-sm">
              <template v-for="(crumb, i) in breadcrumb" :key="i">
                <router-link v-if="crumb.path" :to="crumb.path" class="text-ink-400 hover:text-brand-600 dark:hover:text-brand-400 transition-colors">
                  {{ crumb.label }}
                </router-link>
                <span v-else class="font-semibold text-ink-900 dark:text-ink-100">{{ crumb.label }}</span>
                <span v-if="i < breadcrumb.length - 1" class="text-ink-300 dark:text-ink-600">/</span>
              </template>
            </div>
            <div class="flex items-center gap-2">
              <!-- 主题切换 -->
              <button @click="toggle"
                class="w-9 h-9 flex items-center justify-center rounded-xl text-ink-500 dark:text-ink-400 hover:bg-ink-100 dark:hover:bg-ink-800 transition-colors"
                :title="theme === 'dark' ? '切换到亮色' : '切换到暗色'">
                <svg v-if="theme === 'dark'" viewBox="0 0 24 24" class="w-5 h-5" fill="currentColor"><path d="M12 7a5 5 0 100 10 5 5 0 000-10zM12 1v3m0 16v3M4.2 4.2l2.1 2.1m11.4 11.4l2.1 2.1M1 12h3m16 0h3M4.2 19.8l2.1-2.1M17.7 6.3l2.1-2.1" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round"/></svg>
                <svg v-else viewBox="0 0 24 24" class="w-5 h-5" fill="currentColor"><path d="M12 3a9 9 0 109 9c0-.46-.04-.92-.1-1.36a5.39 5.39 0 01-7.54-7.54c-.44-.06-.9-.1-1.36-.1z"/></svg>
              </button>
              <div class="flex items-center gap-2 pl-2 ml-1 border-l border-ink-200 dark:border-ink-700">
                <div class="w-8 h-8 rounded-full bg-brand-gradient flex items-center justify-center text-white text-xs font-semibold">A</div>
                <span class="text-sm text-ink-600 dark:text-ink-300 hidden sm:block">admin</span>
              </div>
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

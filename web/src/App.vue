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

    <div v-else class="min-h-screen flex bg-surface">
      <!-- ===== 左侧紧凑导航 ===== -->
      <aside class="w-[212px] shrink-0 flex flex-col border-r border-line bg-panel">
        <!-- 品牌 -->
        <div class="px-4 h-14 flex items-center gap-2.5 border-b border-line">
          <div class="w-7 h-7 rounded-md flex items-center justify-center text-surface" style="background-color: rgb(var(--c-signal))">
            <svg viewBox="0 0 24 24" class="w-4 h-4" fill="currentColor"><path d="M13 2L4.5 13.5h6L9 22l9.5-12h-6z"/></svg>
          </div>
          <div class="leading-none">
            <div class="font-mono text-sm font-semibold text-t1 tracking-tight">APIRelay</div>
            <div class="tick mt-0.5">SIGNAL ROUTER</div>
          </div>
        </div>

        <!-- 导航：图标 + 标签，active 信号色左条 + 等宽序号 -->
        <nav class="flex-1 p-2 space-y-0.5 overflow-y-auto">
          <router-link
            v-for="(n, i) in nav" :key="n.name" :to="n.path"
            class="group relative flex items-center gap-2.5 pl-3 pr-2 py-2 rounded-md text-sm transition-colors"
            :class="route.name === n.name
              ? 'bg-panel-2 text-t1'
              : 'text-t2 hover:text-t1 hover:bg-panel-2'"
          >
            <span v-if="route.name === n.name" class="absolute left-0 top-1.5 bottom-1.5 w-[2px] rounded-r bg-signal"></span>
            <span class="font-mono text-2xs w-4 text-center" :class="route.name === n.name ? 'text-signal' : 'text-t3'">{{ String(i + 1).padStart(2, '0') }}</span>
            <svg viewBox="0 0 24 24" class="w-4 h-4 shrink-0" fill="currentColor"><path :d="n.icon"/></svg>
            <span class="font-medium">{{ n.label }}</span>
          </router-link>
        </nav>

        <!-- 退出 -->
        <div class="p-2 border-t border-line">
          <button @click="logout"
            class="w-full flex items-center gap-2.5 pl-3 pr-2 py-2 rounded-md text-sm text-t2 hover:text-[rgb(var(--c-down))] hover:bg-[rgb(var(--c-down)/0.06)] transition-colors">
            <svg viewBox="0 0 24 24" class="w-4 h-4" fill="currentColor"><path d="M16 17v-3H9v-4h7V7l5 5-5 5zM14 2a2 2 0 012 2v2h-2V4H5v16h9v-2h2v2a2 2 0 01-2 2H5a2 2 0 01-2-2V4a2 2 0 012-2h9z"/></svg>
            <span>退出登录</span>
          </button>
        </div>
      </aside>

      <!-- ===== 主内容区 ===== -->
      <main class="flex-1 flex flex-col overflow-hidden min-w-0">
        <!-- 顶部细 header -->
        <header class="h-14 shrink-0 px-5 flex items-center justify-between border-b border-line bg-panel">
          <!-- 路径刻度面包屑 -->
          <div class="flex items-center gap-2 font-mono text-xs">
            <span class="text-t3">{{ String(activeIdx + 1).padStart(2, '0') }}</span>
            <span class="text-line-strong">/</span>
            <span class="text-t1 font-medium">{{ activeLabel }}</span>
          </div>

          <div class="flex items-center gap-3">
            <!-- 在线脉冲点 -->
            <div class="hidden sm:flex items-center gap-1.5">
              <SignalDot status="online" />
              <span class="font-mono text-2xs text-t3">{{ VERSION }}</span>
            </div>

            <!-- 主题切换 -->
            <button @click="toggle"
              class="w-8 h-8 flex items-center justify-center rounded-md text-t2 hover:text-t1 hover:bg-panel-2 transition-colors"
              :title="theme === 'dark' ? '切换到亮色' : '切换到暗色'">
              <svg v-if="theme === 'dark'" viewBox="0 0 24 24" class="w-4 h-4" fill="currentColor"><path d="M12 7a5 5 0 100 10 5 5 0 000-10zM12 1v3m0 16v3M4.2 4.2l2.1 2.1m11.4 11.4l2.1 2.1M1 12h3m16 0h3M4.2 19.8l2.1-2.1M17.7 6.3l2.1-2.1" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round"/></svg>
              <svg v-else viewBox="0 0 24 24" class="w-4 h-4" fill="currentColor"><path d="M12 3a9 9 0 109 9c0-.46-.04-.92-.1-1.36a5.39 5.39 0 01-7.54-7.54c-.44-.06-.9-.1-1.36-.1z"/></svg>
            </button>

            <!-- admin -->
            <div class="flex items-center gap-2 pl-3 border-l border-line">
              <div class="w-7 h-7 rounded-md border border-line bg-panel-2 flex items-center justify-center text-t1 text-xs font-mono font-semibold">A</div>
              <span class="text-xs text-t2 hidden sm:block font-mono">admin</span>
            </div>
          </div>
        </header>

        <div class="flex-1 overflow-auto p-5">
          <div class="animate-fade-in max-w-[1400px] mx-auto">
            <router-view />
          </div>
        </div>
      </main>
    </div>
  </div>
</template>

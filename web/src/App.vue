<script setup>
import { computed, inject } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { TOKEN_KEY } from './api'

const route = useRoute()
const router = useRouter()
const isLogin = computed(() => route.name === 'login')
const toastRef = inject('toastRef')

const VERSION = 'v0.1.0'

// 端口式导航（功能命名 + 等宽端口码）
const nav = [
  { name: 'dashboard', label: '总览', code: 'OVR', icon: 'M3 13h8V3H3v10zm0 8h8v-6H3v6zm10 0h8V11h-8v10zm0-18v6h8V3h-8z', path: '/dashboard' },
  { name: 'channels', label: '渠道', code: 'CHN', icon: 'M4 6a2 2 0 012-2h12a2 2 0 012 2v2H4V6zm0 4h16v8a2 2 0 01-2 2H6a2 2 0 01-2-2v-8zm4 3h4v2H8v-2z', path: '/channels' },
  { name: 'models', label: '模型', code: 'MDL', icon: 'M12 2l9 5v10l-9 5-9-5V7l9-5zm0 2.3L5 8v8l7 3.9 7-3.9V8l-7-3.7z', path: '/models' },
  { name: 'tokens', label: '令牌', code: 'KEY', icon: 'M7 14a3 3 0 100-6 3 3 0 000 6zm5-3h9v2h-2v3h-2v-3h-2v3h-2v-5zm-5 1a1 1 0 110-2 1 1 0 010 2z', path: '/tokens' },
  { name: 'logs', label: '日志', code: 'LOG', icon: 'M4 4h16v2H4V4zm0 5h16v2H4V9zm0 5h10v2H4v-2zm0 5h10v2H4v-2z', path: '/logs' },
  { name: 'settings', label: '设置', code: 'CFG', icon: 'M12 8a4 4 0 100 8 4 4 0 000-8zm9 4l-2 1.5.3 2.5-2.4.8-1.2 2.2-2.5-.5L12 22l-1.7-1.7-2.5.5-1.2-2.2-2.4-.8.3-2.5L2 12l2-1.5L3.7 8l2.4-.8L7.3 5l2.5.5L12 2l1.7 1.7 2.5-.5 1.2 2.2 2.4.8-.3 2.5L21 12z', path: '/settings' },
]

const activeLabel = computed(() => nav.find(n => n.name === route.name)?.label || '')
const activeCode = computed(() => nav.find(n => n.name === route.name)?.code || 'IR')

function setToastRef(el) {
  if (toastRef) toastRef.value = el
}

function logout() {
  localStorage.removeItem(TOKEN_KEY)
  router.push('/login')
}
</script>

<template>
  <div>
    <Toast :ref="setToastRef" />

    <div v-if="isLogin">
      <router-view />
    </div>

    <div v-else class="app-shell">
      <aside class="rack-sidebar">
        <div class="rack-brand">
          <div class="rack-logo">
            <svg viewBox="0 0 24 24" class="w-6 h-6" fill="currentColor">
              <path d="M13 2L4.5 13.5h6L9 22l9.5-12h-6z"/>
            </svg>
          </div>
          <div class="min-w-0">
            <div class="font-mono text-base font-semibold tracking-wide text-t1">APIRelay</div>
            <div class="mt-0.5 flex items-center gap-2 text-2xs font-mono uppercase tracking-[0.2em] text-t3">
              <span class="status-dot status-dot-online"></span>
              <span>Patchbay</span>
            </div>
          </div>
        </div>

        <nav class="rack-rail" aria-label="主导航">
          <router-link
            v-for="n in nav" :key="n.name" :to="n.path"
            class="rack-port"
            :class="route.name === n.name ? 'rack-port-active' : 'rack-port-idle'"
            :aria-current="route.name === n.name ? 'page' : undefined"
          >
            <svg viewBox="0 0 24 24" class="w-5 h-5 shrink-0" fill="currentColor"><path :d="n.icon"/></svg>
            <span class="flex-1">{{ n.label }}</span>
            <span class="font-mono text-2xs opacity-60">{{ n.code }}</span>
          </router-link>
        </nav>

        <div class="relative p-3 border-t border-line space-y-3">
          <div class="rounded-xl border border-line bg-ink/50 p-3">
            <div class="flex items-center justify-between">
              <span class="tick">IR CORE</span>
              <span class="badge badge-online">在线</span>
            </div>
            <div class="mt-3 grid grid-cols-9 gap-1">
              <span v-for="i in 9" :key="i" class="h-1.5 rounded-full" :class="i <= 6 ? 'bg-brass/80' : 'bg-line-2'"></span>
            </div>
            <div class="mt-3 font-mono text-2xs text-t3">{{ VERSION }} · 路由就绪</div>
          </div>
          <button @click="logout" class="w-full btn-ghost justify-start hover:text-rust hover:border-rust/25 hover:bg-rust/10">
            <svg viewBox="0 0 24 24" class="w-5 h-5" fill="currentColor">
              <path d="M16 17v-3H9v-4h7V7l5 5-5 5zM14 2a2 2 0 012 2v2h-2V4H5v16h9v-2h2v2a2 2 0 01-2 2H5a2 2 0 01-2-2V4a2 2 0 012-2h9z"/>
            </svg>
            <span>退出登录</span>
          </button>
        </div>
      </aside>

      <main class="main-area">
        <header class="console-topbar">
          <div>
            <div class="console-title">{{ activeCode }} · {{ activeLabel }}</div>
          </div>
          <div class="flex items-center gap-3">
            <div class="hidden sm:flex items-center gap-2 rounded-lg border border-line bg-ink/50 px-3 py-1.5">
              <span class="status-dot status-dot-online"></span>
              <span class="font-mono text-2xs text-t2">ADMIN</span>
            </div>
            <div class="w-9 h-9 rounded-lg border border-brass/30 bg-brass/10 flex items-center justify-center font-mono text-sm text-brass">A</div>
          </div>
        </header>

        <div class="console-frame">
          <div class="animate-fade-in max-w-7xl mx-auto">
            <router-view />
          </div>
        </div>
      </main>
    </div>
  </div>
</template>

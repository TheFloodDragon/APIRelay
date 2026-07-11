<script setup>
import { computed, inject, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { sheets } from './router'
import api, { logout } from './api'

const route = useRoute()
const toastRef = inject('toastRef')
const loggingOut = ref(false)
const navigationOpen = ref(false)
const serviceOnline = ref(true)
const isLogin = computed(() => route.name === 'login')
const username = computed(() => localStorage.getItem('apirelay_user') || '管理员')
const currentSheet = computed(() => sheets.find((item) => item.name === route.name))

watch(() => route.fullPath, () => {
  navigationOpen.value = false
  if (!isLogin.value) loadRuntimeState()
})

function bindToast(instance) {
  toastRef.value = instance
}

let runtimeSeq = 0
async function loadRuntimeState() {
  if (!localStorage.getItem('apirelay_session')) return
  const seq = ++runtimeSeq
  try {
    await api.get('/settings/logging')
    if (seq === runtimeSeq) serviceOnline.value = true
  } catch {
    if (seq === runtimeSeq) serviceOnline.value = false
  }
}

async function doLogout() {
  if (loggingOut.value) return
  loggingOut.value = true
  try {
    await logout()
  } finally {
    loggingOut.value = false
  }
}

onMounted(() => {
  if (!isLogin.value) loadRuntimeState()
})
</script>

<template>
  <Toast :ref="bindToast" />
  <RouterView v-if="isLogin" />

  <div v-else class="app-shell min-h-screen bg-canvas lg:grid lg:grid-cols-[248px_minmax(0,1fr)]">
    <header class="mobile-bar sticky top-0 z-40 flex h-16 items-center border-b border-line bg-white/95 px-4 backdrop-blur lg:hidden">
      <button class="icon-btn mr-3" type="button" aria-label="打开导航" :aria-expanded="navigationOpen" @click="navigationOpen = true">
        <svg viewBox="0 0 24 24" class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M4 7h16M4 12h16M4 17h16" /></svg>
      </button>
      <RouterLink to="/dashboard" class="brand-wordmark">API<span>Relay</span></RouterLink>
      <span class="ml-auto inline-flex items-center gap-2 text-xs text-soft"><i class="status-dot" :class="serviceOnline ? 'status-dot-live' : 'status-dot-off'"></i>{{ currentSheet?.label }}</span>
    </header>

    <div v-if="navigationOpen" class="fixed inset-0 z-50 bg-sidebar/55 backdrop-blur-sm lg:hidden" @mousedown.self="navigationOpen = false">
      <aside class="flex h-full w-[292px] max-w-[86vw] flex-col bg-sidebar text-white shadow-lift">
        <div class="flex h-20 items-center border-b border-white/10 px-5">
          <span class="brand-wordmark brand-wordmark-inverse">API<span>Relay</span></span>
          <button class="ml-auto rounded-lg p-2 text-white/60 transition hover:bg-white/10 hover:text-white" aria-label="关闭导航" @click="navigationOpen = false">
            <svg viewBox="0 0 24 24" class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="1.8"><path d="m6 6 12 12M18 6 6 18" /></svg>
          </button>
        </div>
        <nav class="route-nav relative flex-1 px-3 py-4" aria-label="主要导航">
          <div class="route-spine" aria-hidden="true"></div>
          <RouterLink v-for="item in sheets" :key="item.name" :to="item.path" class="route-link" :class="{ 'route-link-active': route.name === item.name }">
            <span class="route-node"><span></span></span><span>{{ item.label }}</span>
          </RouterLink>
        </nav>
        <div class="border-t border-white/10 p-4">
          <div class="mb-3 truncate text-sm text-white/65">{{ username }}</div>
          <button class="nav-account-button" :disabled="loggingOut" @click="doLogout">{{ loggingOut ? '退出中…' : '退出登录' }}</button>
        </div>
      </aside>
    </div>

    <aside class="sidebar fixed inset-y-0 left-0 z-30 hidden w-[248px] flex-col bg-sidebar text-white lg:flex">
      <div class="px-6 pb-5 pt-7">
        <RouterLink to="/dashboard" class="brand-wordmark brand-wordmark-inverse">API<span>Relay</span></RouterLink>
        <div class="mt-2 font-mono text-[10px] uppercase tracking-[0.2em] text-white/35">Routing control plane</div>
      </div>

      <div class="mx-4 mb-5 overflow-hidden rounded-xl border border-white/10 bg-white/[0.03] px-3 py-2.5">
        <div class="text-[10px] uppercase tracking-wider text-white/35">Service</div>
        <div class="mt-1 flex items-center gap-2 text-xs text-white/80"><i class="status-dot" :class="serviceOnline ? 'status-dot-live' : 'status-dot-off'"></i>{{ serviceOnline ? 'Online' : 'Unknown' }}</div>
      </div>

      <nav class="route-nav flex-1 px-3" aria-label="主要导航">
        <div class="route-spine" aria-hidden="true"></div>
        <RouterLink v-for="item in sheets" :key="item.name" :to="item.path" class="route-link" :class="{ 'route-link-active': route.name === item.name }">
          <span class="route-node"><span></span></span>
          <svg v-if="item.name === 'dashboard'" viewBox="0 0 24 24" aria-hidden="true"><path d="M4 13h6V4H4v9Zm0 7h6v-4H4v4Zm10 0h6v-9h-6v9Zm0-16v4h6V4h-6Z" /></svg>
          <svg v-else-if="item.name === 'channels'" viewBox="0 0 24 24" aria-hidden="true"><path d="M5 7h8m4 0h2M5 17h2m4 0h8M13 4v6M8 14v6" /></svg>
          <svg v-else-if="item.name === 'models'" viewBox="0 0 24 24" aria-hidden="true"><path d="m12 3 8 4.5-8 4.5-8-4.5L12 3Zm-8 9 8 4.5 8-4.5M4 16.5l8 4.5 8-4.5" /></svg>
          <svg v-else-if="item.name === 'tokens'" viewBox="0 0 24 24" aria-hidden="true"><path d="M14.5 8.5a4 4 0 1 1-3-3.87M14 10l7-7m-3 0h3v3M10.5 12.5 4 19v2h3l6.5-6.5" /></svg>
          <svg v-else-if="item.name === 'logs'" viewBox="0 0 24 24" aria-hidden="true"><path d="M5 4h14v16H5zM8 8h8m-8 4h8m-8 4h5" /></svg>
          <svg v-else viewBox="0 0 24 24" aria-hidden="true"><path d="M12 15.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7ZM19 13v-2l2-1-2-3-2 1a8 8 0 0 0-2-1l-.5-2h-5L9 7a8 8 0 0 0-2 1L5 7l-2 3 2 1v2l-2 1 2 3 2-1a8 8 0 0 0 2 1l.5 2h5l.5-2a8 8 0 0 0 2-1l2 1 2-3-2-1Z" /></svg>
          <span>{{ item.label }}</span>
        </RouterLink>
      </nav>

      <div class="border-t border-white/10 p-4">
        <div class="mb-3 flex items-center gap-3 px-1">
          <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-white/10 font-cond text-sm font-semibold text-white">{{ username.slice(0, 1).toUpperCase() }}</div>
          <div class="min-w-0"><div class="truncate text-sm text-white/85">{{ username }}</div><div class="text-[10px] uppercase tracking-wider text-white/35">Administrator</div></div>
        </div>
        <button class="nav-account-button" :disabled="loggingOut" @click="doLogout">{{ loggingOut ? '退出中…' : '退出登录' }}</button>
      </div>
    </aside>

    <section class="min-w-0 lg:col-start-2">
      <div class="desktop-status-rail hidden h-11 items-center border-b border-line bg-white/80 px-8 text-[11px] backdrop-blur lg:flex">
        <span class="font-mono uppercase tracking-[0.16em] text-faint">Control plane</span>
        <span class="mx-4 h-3 w-px bg-line"></span>
        <span class="text-soft">{{ currentSheet?.label || 'APIRelay' }}</span>
        <span class="ml-auto flex items-center gap-4">
          <span class="flex items-center gap-2 text-soft"><i class="status-dot" :class="serviceOnline ? 'status-dot-live' : 'status-dot-off'"></i>{{ serviceOnline ? '服务在线' : '状态未知' }}</span>
          <button class="text-blue transition hover:text-blue-deep" @click="loadRuntimeState">同步状态</button>
        </span>
      </div>
      <main class="min-w-0 px-4 py-6 sm:px-6 lg:px-8 lg:py-8 xl:px-10 xl:py-9">
        <div class="mx-auto w-full max-w-[1500px]">
          <RouterView v-slot="{ Component }"><component :is="Component" /></RouterView>
        </div>
      </main>
    </section>
  </div>
</template>

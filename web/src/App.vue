<script setup>
import { computed, inject, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { sheets } from './router'
import api, { logout } from './api'
import ServiceStatus from './components/ServiceStatus.vue'
import ConfirmDialog from './components/ConfirmDialog.vue'
import AppSidebar from './components/AppSidebar.vue'

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
  <ConfirmDialog />
  <a v-if="!isLogin" class="skip-link" href="#main-content">跳到主要内容</a>
  <RouterView v-if="isLogin" />

  <div v-else class="app-shell min-h-screen bg-canvas lg:grid lg:grid-cols-[104px_minmax(0,1fr)]">
    <header class="mobile-bar sticky top-0 z-40 flex h-16 items-center border-b border-line bg-white/95 px-4 backdrop-blur lg:hidden">
      <button class="icon-btn mr-3" type="button" aria-label="打开导航" :aria-expanded="navigationOpen" @click="navigationOpen = true">
        <svg viewBox="0 0 24 24" class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M4 7h16M4 12h16M4 17h16" /></svg>
      </button>
      <RouterLink to="/dashboard" class="brand-wordmark">API<span>Relay</span></RouterLink>
      <span class="ml-auto inline-flex items-center gap-3 text-xs text-soft"><ServiceStatus :online="serviceOnline" compact /><span class="hidden sm:inline">{{ currentSheet?.label }}</span></span>
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

    <AppSidebar
      :route-name="route.name"
      :username="username"
      :online="serviceOnline"
      :logging-out="loggingOut"
      @logout="doLogout"
    />

    <section class="min-w-0 lg:col-start-2">
      <div class="context-dock hidden lg:flex">
        <div class="context-dock-path"><span>APIRelay</span><i></i><strong>{{ currentSheet?.label || '控制台' }}</strong></div>
        <div class="context-dock-actions">
          <ServiceStatus :online="serviceOnline" />
          <button class="context-sync" type="button" @click="loadRuntimeState">同步状态</button>
        </div>
      </div>
      <main id="main-content" class="min-w-0 px-4 py-6 sm:px-7 lg:px-10 lg:pb-12 lg:pt-5 xl:px-14" tabindex="-1">
        <div class="mx-auto w-full max-w-[1680px]">
          <RouterView v-slot="{ Component }">
            <Transition name="page" mode="out-in">
              <component :is="Component" :key="route.name" />
            </Transition>
          </RouterView>
        </div>
      </main>
    </section>
  </div>
</template>

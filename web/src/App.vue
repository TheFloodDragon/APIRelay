<script setup>
import { computed, inject, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { sheets } from './router'
import { logout } from './api'

const route = useRoute()
const toastRef = inject('toastRef')
const loggingOut = ref(false)
const navigationOpen = ref(false)
const isLogin = computed(() => route.name === 'login')
const username = computed(() => localStorage.getItem('apirelay_user') || '管理员')

watch(() => route.fullPath, () => { navigationOpen.value = false })

function bindToast(instance) {
  toastRef.value = instance
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
</script>

<template>
  <Toast :ref="bindToast" />
  <RouterView v-if="isLogin" />

  <div v-else class="min-h-screen bg-canvas lg:grid lg:grid-cols-[220px_minmax(0,1fr)]">
    <header class="sticky top-0 z-40 flex h-16 items-center border-b border-line bg-white px-4 lg:hidden">
      <button class="icon-btn mr-3" type="button" aria-label="打开导航" :aria-expanded="navigationOpen" @click="navigationOpen = true">
        <svg viewBox="0 0 24 24" class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M4 7h16M4 12h16M4 17h16" /></svg>
      </button>
      <RouterLink to="/dashboard" class="font-cond text-xl font-semibold tracking-wide text-ink">APIRelay</RouterLink>
      <span class="ml-auto max-w-32 truncate text-sm text-soft">{{ username }}</span>
    </header>

    <div v-if="navigationOpen" class="fixed inset-0 z-50 bg-sidebar/45 lg:hidden" @mousedown.self="navigationOpen = false">
      <aside class="flex h-full w-[280px] max-w-[85vw] flex-col bg-sidebar text-white shadow-lift">
        <div class="flex h-16 items-center border-b border-white/10 px-5">
          <span class="font-cond text-xl font-semibold tracking-wide">APIRelay</span>
          <button class="ml-auto rounded-lg p-2 text-white/70 hover:bg-white/10 hover:text-white" aria-label="关闭导航" @click="navigationOpen = false">✕</button>
        </div>
        <nav class="flex-1 space-y-1 p-3" aria-label="主要导航">
          <RouterLink v-for="item in sheets" :key="item.name" :to="item.path" class="flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition" :class="route.name === item.name ? 'bg-white/12 text-white' : 'text-white/65 hover:bg-white/8 hover:text-white'">
            <span class="h-1.5 w-1.5 rounded-full" :class="route.name === item.name ? 'bg-blue-grid' : 'bg-white/25'"></span>{{ item.label }}
          </RouterLink>
        </nav>
        <div class="border-t border-white/10 p-4">
          <div class="mb-3 truncate text-sm text-white/65">{{ username }}</div>
          <button class="w-full rounded-lg border border-white/15 px-3 py-2 text-sm font-medium text-white/80 hover:bg-white/10" :disabled="loggingOut" @click="doLogout">{{ loggingOut ? '退出中…' : '退出登录' }}</button>
        </div>
      </aside>
    </div>

    <aside class="fixed inset-y-0 left-0 z-30 hidden w-[220px] flex-col bg-sidebar text-white lg:flex">
      <div class="flex h-20 items-center px-6">
        <RouterLink to="/dashboard" class="font-cond text-2xl font-semibold tracking-wide">APIRelay</RouterLink>
      </div>
      <nav class="flex-1 space-y-1 px-3" aria-label="主要导航">
        <RouterLink v-for="item in sheets" :key="item.name" :to="item.path" class="flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition" :class="route.name === item.name ? 'bg-white/12 text-white' : 'text-white/60 hover:bg-white/8 hover:text-white'">
          <span class="h-1.5 w-1.5 rounded-full" :class="route.name === item.name ? 'bg-blue-grid' : 'bg-white/25'"></span>{{ item.label }}
        </RouterLink>
      </nav>
      <div class="border-t border-white/10 p-4">
        <div class="mb-3">
          <div class="text-xs text-white/45">当前账户</div>
          <div class="mt-1 truncate text-sm text-white/80">{{ username }}</div>
        </div>
        <button class="w-full rounded-lg border border-white/15 px-3 py-2 text-sm font-medium text-white/75 transition hover:bg-white/10 hover:text-white" :disabled="loggingOut" @click="doLogout">{{ loggingOut ? '退出中…' : '退出登录' }}</button>
      </div>
    </aside>

    <main class="min-w-0 px-4 py-6 sm:px-6 lg:col-start-2 lg:px-8 lg:py-8 xl:px-10">
      <div class="mx-auto w-full max-w-[1440px]">
        <RouterView v-slot="{ Component }"><component :is="Component" /></RouterView>
      </div>
    </main>
  </div>
</template>

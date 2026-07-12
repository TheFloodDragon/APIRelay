<script setup>
import { computed, inject, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { sheets } from './router'
import api, { logout } from './api'
import AppSidebar from './components/AppSidebar.vue'
import ConfirmDialog from './components/ConfirmDialog.vue'
import ConsoleIcon from './components/ConsoleIcon.vue'
import Drawer from './components/Drawer.vue'
import ServiceStatus from './components/ServiceStatus.vue'

const route = useRoute()
const toastRef = inject('toastRef')
const loggingOut = ref(false)
const navigationOpen = ref(false)
const serviceOnline = ref(null)
const syncing = ref(false)
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
  syncing.value = true
  try {
    await api.get('/settings/logging')
    if (seq === runtimeSeq) serviceOnline.value = true
  } catch {
    if (seq === runtimeSeq) serviceOnline.value = false
  } finally {
    if (seq === runtimeSeq) syncing.value = false
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

  <div v-else class="app-shell">
    <AppSidebar
      class="app-sidebar-desktop"
      :route-name="route.name"
      :username="username"
      :online="serviceOnline"
      :logging-out="loggingOut"
      @logout="doLogout"
    />

    <section class="app-workspace">
      <header class="app-topbar">
        <button
          class="icon-button app-mobile-menu"
          type="button"
          aria-label="打开主导航"
          :aria-expanded="navigationOpen"
          @click="navigationOpen = true"
        >
          <ConsoleIcon name="bars" class="h-5 w-5" />
        </button>

        <div class="app-topbar-title">
          <span>APIRelay</span>
          <strong>{{ currentSheet?.label || '运维控制台' }}</strong>
        </div>

        <div class="app-topbar-actions">
          <button class="topbar-sync" type="button" :disabled="syncing" @click="loadRuntimeState">
            <ConsoleIcon name="arrowPath" class="h-4 w-4" :class="{ 'animate-spin': syncing }" />
            <span class="hidden sm:inline">{{ syncing ? '同步中' : '同步状态' }}</span>
          </button>
          <ServiceStatus class="hidden sm:inline-flex" :online="serviceOnline" compact />
          <div class="topbar-account" :title="`当前账户：${username}`">
            <ConsoleIcon name="user" class="h-4 w-4" />
            <span>{{ username }}</span>
          </div>
        </div>
      </header>

      <main id="main-content" class="main-stage" tabindex="-1">
        <div class="main-stage-inner">
          <RouterView v-slot="{ Component }">
            <Transition name="page" mode="out-in">
              <component :is="Component" :key="route.name" />
            </Transition>
          </RouterView>
        </div>
      </main>
    </section>

    <Drawer
      :open="navigationOpen"
      title="主导航"
      width="max-w-none sm:max-w-sm"
      @close="navigationOpen = false"
    >
      <AppSidebar
        mobile
        :route-name="route.name"
        :username="username"
        :online="serviceOnline"
        :logging-out="loggingOut"
        @logout="doLogout"
      />
    </Drawer>
  </div>
</template>

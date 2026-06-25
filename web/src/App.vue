<script setup>
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const isLogin = computed(() => route.name === 'login')

const nav = [
  { name: 'dashboard', label: '仪表盘', path: '/' },
  { name: 'channels', label: '渠道', path: '/channels' },
  { name: 'tokens', label: '令牌', path: '/tokens' },
  { name: 'logs', label: '日志', path: '/logs' },
]

function logout() {
  localStorage.removeItem('apirelay_session')
  router.push('/login')
}
</script>

<template>
  <div v-if="isLogin">
    <router-view />
  </div>
  <div v-else class="min-h-screen flex">
    <aside class="w-52 bg-slate-900 text-slate-200 flex flex-col">
      <div class="px-5 py-4 text-lg font-semibold border-b border-slate-700">APIRelay</div>
      <nav class="flex-1 p-2 space-y-1">
        <router-link
          v-for="n in nav" :key="n.name" :to="n.path"
          class="block px-3 py-2 rounded text-sm transition"
          :class="route.name === n.name ? 'bg-indigo-600 text-white' : 'hover:bg-slate-800'"
        >{{ n.label }}</router-link>
      </nav>
      <button @click="logout" class="m-2 px-3 py-2 text-sm text-slate-400 hover:text-white hover:bg-slate-800 rounded text-left">退出登录</button>
    </aside>
    <main class="flex-1 bg-slate-50 overflow-auto">
      <div class="max-w-6xl mx-auto p-6">
        <router-view />
      </div>
    </main>
  </div>
</template>

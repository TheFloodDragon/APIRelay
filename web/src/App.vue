<script setup>
import { computed, inject } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { TOKEN_KEY } from './api'

const route = useRoute()
const router = useRouter()
const isLogin = computed(() => route.name === 'login')
const toastRef = inject('toastRef')

const nav = [
  { name: 'dashboard', label: '仪表盘', icon: '📊', path: '/' },
  { name: 'channels', label: '渠道', icon: '🔗', path: '/channels' },
  { name: 'tokens', label: '令牌', icon: '🔑', path: '/tokens' },
  { name: 'logs', label: '日志', icon: '📝', path: '/logs' },
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
    
    <div v-else class="min-h-screen flex bg-gray-50">
      <!-- 侧边栏 -->
      <aside class="w-64 bg-white border-r border-gray-200 flex flex-col shadow-sm">
        <div class="px-6 py-5 border-b border-gray-200">
          <h1 class="text-xl font-bold text-brand-600 flex items-center gap-2">
            <span class="text-2xl">⚡</span>
            <span>APIRelay</span>
          </h1>
          <p class="text-xs text-gray-500 mt-1">AI API 中转聚合平台</p>
        </div>
        
        <nav class="flex-1 p-3 space-y-1">
          <router-link
            v-for="n in nav" :key="n.name" :to="n.path"
            class="group flex items-center gap-3 px-4 py-3 rounded-lg text-sm font-medium transition-all"
            :class="route.name === n.name 
              ? 'bg-brand-50 text-brand-700 shadow-sm' 
              : 'text-gray-700 hover:bg-gray-50 hover:text-brand-600'"
          >
            <span class="text-lg">{{ n.icon }}</span>
            <span>{{ n.label }}</span>
          </router-link>
        </nav>
        
        <div class="p-3 border-t border-gray-200">
          <button @click="logout" 
            class="w-full flex items-center gap-3 px-4 py-3 rounded-lg text-sm font-medium text-gray-600 hover:bg-gray-50 hover:text-red-600 transition-all">
            <span class="text-lg">🚪</span>
            <span>退出登录</span>
          </button>
        </div>
      </aside>

      <!-- 主内容区 -->
      <main class="flex-1 flex flex-col overflow-hidden">
        <!-- 顶栏 -->
        <header class="bg-white border-b border-gray-200 px-6 py-4 shadow-sm">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2 text-sm">
              <template v-for="(crumb, i) in breadcrumb" :key="i">
                <router-link v-if="crumb.path" :to="crumb.path" class="text-gray-500 hover:text-brand-600 transition-colors">
                  {{ crumb.label }}
                </router-link>
                <span v-else class="font-medium text-gray-900">{{ crumb.label }}</span>
                <span v-if="i < breadcrumb.length - 1" class="text-gray-300">/</span>
              </template>
            </div>
            <div class="flex items-center gap-3">
              <span class="text-sm text-gray-600">👤 admin</span>
            </div>
          </div>
        </header>

        <!-- 内容 -->
        <div class="flex-1 overflow-auto p-6">
          <router-view />
        </div>
      </main>
    </div>
  </div>
</template>

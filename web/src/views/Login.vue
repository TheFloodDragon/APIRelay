<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useToast } from '../composables/useToast'
import api from '../api'

const router = useRouter()
const toast = useToast()
const username = ref('admin')
const password = ref('')
const loading = ref(false)

async function login() {
  if (!username.value || !password.value) {
    toast.warning('请输入用户名和密码')
    return
  }
  loading.value = true
  try {
    const data = await api.post('/auth/login', {
      username: username.value,
      password: password.value,
    })
    localStorage.setItem('apirelay_session', data.token)
    toast.success('登录成功')
    router.push('/')
  } catch (e) {
    toast.error(e?.message || '登录失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center p-4 bg-ink-50 dark:bg-ink-950 bg-mesh-light dark:bg-mesh-dark relative overflow-hidden">
    <!-- 背景光晕 -->
    <div class="absolute -top-24 -left-24 w-96 h-96 rounded-full bg-brand-400/20 blur-3xl animate-float"></div>
    <div class="absolute -bottom-24 -right-24 w-96 h-96 rounded-full bg-purple-400/20 blur-3xl animate-float" style="animation-delay: -2s"></div>

    <div class="w-full max-w-md relative">
      <!-- 品牌 -->
      <div class="text-center mb-8">
        <div class="inline-flex items-center justify-center w-16 h-16 bg-brand-gradient rounded-2xl shadow-glow mb-4 animate-float text-white">
          <svg viewBox="0 0 24 24" class="w-8 h-8" fill="currentColor"><path d="M13 2L4.5 13.5h6L9 22l9.5-12h-6z"/></svg>
        </div>
        <h1 class="text-3xl font-bold text-ink-900 dark:text-ink-50 mb-2">APIRelay</h1>
        <p class="text-sm text-ink-500 dark:text-ink-400">AI 模型聚合中转平台</p>
      </div>

      <!-- 登录卡片 -->
      <div class="card animate-slide-in">
        <h2 class="text-lg font-semibold text-ink-900 dark:text-ink-100 mb-6">管理后台登录</h2>
        <form @submit.prevent="login" class="space-y-4">
          <div>
            <label class="label">用户名</label>
            <input v-model="username" class="input" placeholder="请输入用户名" autocomplete="username" />
          </div>
          <div>
            <label class="label">密码</label>
            <input v-model="password" type="password" class="input" placeholder="请输入密码" autocomplete="current-password" />
          </div>
          <button type="submit" :disabled="loading" class="btn-primary w-full mt-6">
            <span v-if="loading">登录中...</span>
            <span v-else>登录</span>
          </button>
        </form>
      </div>

      <div class="text-center mt-6 text-xs text-ink-400 dark:text-ink-600">
        默认账号: admin / admin123
      </div>
    </div>
  </div>
</template>

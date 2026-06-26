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
  <div class="min-h-screen flex items-center justify-center bg-gradient-to-br from-brand-50 via-white to-purple-50 p-4">
    <div class="w-full max-w-md">
      <!-- 品牌 -->
      <div class="text-center mb-8">
        <div class="inline-flex items-center justify-center w-16 h-16 bg-brand-600 rounded-2xl shadow-lg mb-4">
          <span class="text-3xl">⚡</span>
        </div>
        <h1 class="text-3xl font-bold text-gray-900 mb-2">APIRelay</h1>
        <p class="text-sm text-gray-500">AI API 中转聚合平台</p>
      </div>

      <!-- 登录卡片 -->
      <div class="card animate-fade-in">
        <h2 class="text-lg font-semibold text-gray-900 mb-6">管理后台登录</h2>
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

      <!-- 提示 -->
      <div class="text-center mt-6 text-xs text-gray-400">
        默认账号: admin / admin123
      </div>
    </div>
  </div>
</template>

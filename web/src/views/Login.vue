<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'

const router = useRouter()
const username = ref('admin')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function login() {
  error.value = ''
  loading.value = true
  try {
    const { data } = await api.post('/api/auth/login', {
      username: username.value,
      password: password.value,
    })
    localStorage.setItem('apirelay_session', data.data.token)
    router.push('/')
  } catch (e) {
    error.value = e?.response?.data?.message || '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-slate-100">
    <div class="bg-white rounded-xl shadow-lg p-8 w-80">
      <h1 class="text-xl font-semibold text-center mb-6">APIRelay 管理后台</h1>
      <form @submit.prevent="login" class="space-y-4">
        <input v-model="username" placeholder="用户名"
          class="w-full border rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400" />
        <input v-model="password" type="password" placeholder="密码"
          class="w-full border rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400" />
        <p v-if="error" class="text-red-500 text-xs">{{ error }}</p>
        <button :disabled="loading"
          class="w-full bg-indigo-600 text-white rounded py-2 text-sm hover:bg-indigo-700 disabled:opacity-50">
          {{ loading ? '登录中...' : '登录' }}
        </button>
      </form>
    </div>
  </div>
</template>

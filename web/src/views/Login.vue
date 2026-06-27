<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useToast } from '../composables/useToast'
import SignalDot from '../components/SignalDot.vue'
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
  <div class="min-h-screen flex items-center justify-center p-4 bg-gradient-to-br from-bg to-surface">
    <div class="w-full max-w-md">
      <!-- 品牌 -->
      <div class="flex items-center justify-center gap-3 mb-8">
        <div class="w-12 h-12 rounded-xl bg-gradient-to-br from-primary to-accent flex items-center justify-center shadow-lg shadow-primary/30">
          <svg viewBox="0 0 24 24" class="w-7 h-7 text-white" fill="currentColor">
            <path d="M13 2L4.5 13.5h6L9 22l9.5-12h-6z"/>
          </svg>
        </div>
        <div>
          <div class="text-xl font-bold text-text">APIRelay</div>
          <div class="text-sm text-text-muted">信号路由控制台</div>
        </div>
      </div>

      <!-- 登录卡片 -->
      <div class="card p-8 animate-fade-in">
        <div class="flex items-center gap-2 mb-6">
          <div class="status-dot status-dot-online"></div>
          <span class="text-sm text-text-dim">系统在线</span>
        </div>

        <form @submit.prevent="login" class="space-y-4">
          <div>
            <label class="label">用户名</label>
            <input v-model="username" class="input" placeholder="admin" autocomplete="username" />
          </div>
          <div>
            <label class="label">密码</label>
            <input v-model="password" type="password" class="input" placeholder="输入密码" autocomplete="current-password" @keyup.enter="login" />
          </div>
          <button type="submit" :disabled="loading" class="btn-primary w-full mt-6">
            {{ loading ? '连接中...' : '登录' }}
          </button>
        </form>

        <div class="mt-6 pt-6 border-t border-border text-xs text-text-muted text-center">
          默认账号：admin / admin123
        </div>
      </div>
    </div>
  </div>
</template>

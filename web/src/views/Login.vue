<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useToast } from '../composables/useToast'
import SignalDot from '../components/SignalDot.vue'
import api from '../api'

const router = useRouter()
const toast = useToast()
const showDevHint = import.meta.env.DEV
const username = ref(showDevHint ? 'admin' : '')
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
  <div class="min-h-screen flex items-center justify-center p-4">
    <div class="w-full max-w-md">
      <!-- 品牌 -->
      <div class="flex items-center justify-center gap-3 mb-8">
        <div class="rack-logo !w-12 !h-12 !rounded-xl">
          <svg viewBox="0 0 24 24" class="w-7 h-7" fill="currentColor">
            <path d="M13 2L4.5 13.5h6L9 22l9.5-12h-6z"/>
          </svg>
        </div>
        <div>
          <div class="text-xl font-semibold text-t1 font-mono tracking-wide">APIRelay</div>
          <div class="text-sm text-t3">配线架调度台</div>
        </div>
      </div>

      <!-- 登录卡片 -->
      <div class="panel p-8 animate-fade-in relative overflow-hidden">
        <!-- 配线架装饰：顶部端口条 -->
        <div class="absolute top-0 left-0 right-0 h-1 flex" aria-hidden="true">
          <span class="flex-1 bg-brass/70"></span>
          <span class="flex-1 bg-line-2"></span>
          <span class="flex-1 bg-electric/60"></span>
          <span class="flex-1 bg-line-2"></span>
        </div>

        <div class="flex items-center justify-between mb-6">
          <div class="flex items-center gap-2">
            <div class="status-dot status-dot-online"></div>
            <span class="text-sm text-t2">系统在线</span>
          </div>
          <span class="tick">IR · LOGIN</span>
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
            {{ loading ? '接入中…' : '登录' }}
          </button>
        </form>

        <div v-if="showDevHint" class="mt-6 pt-6 border-t border-line text-xs text-t3 text-center leading-relaxed">
          开发提示：未配置初始密码时可使用 admin / admin123。<br />生产环境请使用初始化时配置的管理员账号。
        </div>
      </div>
    </div>
  </div>
</template>

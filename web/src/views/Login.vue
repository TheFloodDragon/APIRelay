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
  <div class="min-h-screen flex items-center justify-center p-4 bg-surface relative">
    <div class="w-full max-w-sm relative">
      <!-- 品牌：开机感 -->
      <div class="flex items-center gap-2.5 mb-6">
        <div class="w-9 h-9 rounded-md flex items-center justify-center text-surface" style="background-color: rgb(var(--c-signal))">
          <svg viewBox="0 0 24 24" class="w-5 h-5" fill="currentColor"><path d="M13 2L4.5 13.5h6L9 22l9.5-12h-6z"/></svg>
        </div>
        <div class="leading-tight">
          <div class="font-mono text-base font-semibold text-t1 tracking-tight">APIRelay</div>
          <div class="tick mt-0.5">SIGNAL ROUTER CONSOLE</div>
        </div>
      </div>

      <!-- 终端卡片 -->
      <div class="panel overflow-hidden animate-pop-in">
        <!-- 开机扫描条 -->
        <div class="h-[2px] bg-line relative overflow-hidden">
          <div class="absolute inset-y-0 w-1/3 bg-signal animate-sweep"></div>
        </div>

        <div class="p-5">
          <div class="flex items-center justify-between mb-5">
            <span class="font-mono text-sm text-t1">// 管理后台登录</span>
            <div class="flex items-center gap-1.5">
              <SignalDot status="online" />
              <span class="tick">ONLINE</span>
            </div>
          </div>

          <form @submit.prevent="login" class="space-y-4">
            <div>
              <label class="label">用户名</label>
              <input v-model="username" class="input font-mono" placeholder="username" autocomplete="username" />
            </div>
            <div>
              <label class="label">密码</label>
              <input v-model="password" type="password" class="input font-mono" placeholder="password" autocomplete="current-password" />
            </div>
            <button type="submit" :disabled="loading" class="btn-primary w-full mt-5">
              <span v-if="loading" class="font-mono">连接中…</span>
              <span v-else>接入控制台</span>
            </button>
          </form>
        </div>

        <!-- 底部刻度 -->
        <div class="px-5 py-2.5 border-t border-line flex items-center justify-between font-mono text-2xs text-t3">
          <span>默认 admin / admin123</span>
          <span>v0.1.0</span>
        </div>
      </div>
    </div>
  </div>
</template>

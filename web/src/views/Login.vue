<script setup>
import { getCurrentInstance, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import api, { TOKEN_KEY } from '../api'

const router = useRouter()
const { proxy } = getCurrentInstance()
const isDev = import.meta.env.DEV

const username = ref(isDev ? 'admin' : '')
const password = ref('')
const loading = ref(false)
const serviceStatus = ref('checking')

const serviceMeta = {
  checking: { label: '正在确认运行状态', dot: 'bg-test', text: 'text-test' },
  online: { label: '服务运行正常', dot: 'bg-run', text: 'text-run' },
  unknown: { label: '暂时无法确认运行状态', dot: 'bg-faint', text: 'text-soft' },
}

async function checkService() {
  serviceStatus.value = 'checking'
  try {
    const response = await fetch('/healthz', { headers: { Accept: 'application/json' } })
    const data = response.ok ? await response.json() : null
    serviceStatus.value = response.ok && data?.status === 'ok' ? 'online' : 'unknown'
  } catch {
    serviceStatus.value = 'unknown'
  }
}

async function login() {
  if (!username.value || !password.value) {
    proxy.$toast.add('请输入用户名和密码', 'warn')
    return
  }
  loading.value = true
  try {
    const data = await api.post('/auth/login', {
      username: username.value,
      password: password.value,
    })
    localStorage.setItem(TOKEN_KEY, data.token)
    localStorage.setItem('apirelay_user', data.username || username.value)
    proxy.$toast.add('登录成功', 'success')
    router.push('/')
  } catch (error) {
    proxy.$toast.add(error.message || '登录失败', 'error')
  } finally {
    loading.value = false
  }
}

onMounted(checkService)
</script>

<template>
  <main class="flex min-h-screen items-center justify-center bg-canvas px-4 py-8">
    <section class="w-full max-w-sm rounded-xl border border-line bg-white shadow-lift" aria-labelledby="login-title">
      <div class="border-b border-line px-6 py-5">
        <div class="flex items-center gap-3">
          <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-blue font-mono text-base font-semibold text-white" aria-hidden="true">AR</div>
          <div>
            <div class="text-lg font-semibold tracking-[-0.02em] text-ink">APIRelay</div>
            <div class="text-xs text-soft">管理后台</div>
          </div>
        </div>
        <h1 id="login-title" class="mt-5 text-xl font-semibold text-ink">登录</h1>
        <p class="mt-1 text-sm leading-6 text-soft">使用管理员账户进入渠道、模型和调用记录管理。</p>
      </div>

      <form class="space-y-4 px-6 py-5" @submit.prevent="login">
        <div>
          <label class="field-label" for="lg-user">用户名</label>
          <input
            id="lg-user"
            v-model="username"
            class="input"
            placeholder="请输入用户名"
            autocomplete="username"
            data-autofocus
          />
        </div>
        <div>
          <label class="field-label" for="lg-pass">密码</label>
          <input
            id="lg-pass"
            v-model="password"
            type="password"
            class="input"
            placeholder="请输入密码"
            autocomplete="current-password"
          />
        </div>
        <button type="submit" class="btn btn-primary w-full" :disabled="loading">
          {{ loading ? '正在登录…' : '登录' }}
        </button>
        <p v-if="isDev" class="rounded-lg bg-ghost px-3 py-2 text-center text-xs text-soft">开发环境默认账户：admin / admin123</p>
      </form>

      <div class="flex items-center justify-between gap-3 border-t border-line px-6 py-3 text-xs">
        <span class="text-soft">运行状态</span>
        <button type="button" class="inline-flex items-center gap-2" :class="serviceMeta[serviceStatus].text" @click="checkService">
          <span class="h-2 w-2 rounded-full" :class="serviceMeta[serviceStatus].dot" aria-hidden="true"></span>
          {{ serviceMeta[serviceStatus].label }}
        </button>
      </div>
    </section>
  </main>
</template>

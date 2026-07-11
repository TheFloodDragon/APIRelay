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
  <main class="relative min-h-screen overflow-hidden bg-sidebar">
    <div class="absolute inset-0 opacity-20" style="background-image: linear-gradient(rgba(142,177,255,.16) 1px, transparent 1px), linear-gradient(90deg, rgba(142,177,255,.16) 1px, transparent 1px); background-size: 44px 44px;"></div>
    <div class="relative mx-auto grid min-h-screen max-w-[1440px] lg:grid-cols-[minmax(0,1.15fr)_minmax(420px,.85fr)]">
      <section class="hidden min-h-screen flex-col justify-between px-12 py-10 text-white lg:flex xl:px-20 xl:py-14">
        <div>
          <div class="brand-wordmark brand-wordmark-inverse text-[28px]">API<span>Relay</span></div>
          <div class="mt-2 font-mono text-[10px] uppercase tracking-[0.22em] text-white/35">Routing control plane</div>
        </div>

        <div class="max-w-2xl">
          <div class="mb-7 flex items-center gap-3 font-mono text-[10px] uppercase tracking-[0.18em] text-white/40">
            <span>Client</span><span class="h-px w-14 bg-white/15"></span><span class="text-blue-grid">Relay</span><span class="h-px w-14 bg-white/15"></span><span>Upstream</span>
          </div>
          <h1 class="font-cond text-5xl font-semibold leading-[1.04] tracking-[-0.035em] xl:text-6xl">让每一次 API 调用<br /><span class="text-blue-grid">都有清晰路径。</span></h1>
          <p class="mt-6 max-w-xl text-[15px] leading-7 text-white/55">统一维护上游渠道、模型映射、访问令牌和故障转移策略；从一个请求 ID 追踪调用链路。</p>

          <div class="route-timeline mt-10 space-y-6 border-white/15 text-sm">
            <div><div class="text-white/90">协议自适应路由</div><div class="mt-1 text-xs text-white/40">OpenAI · Anthropic · Responses</div></div>
            <div><div class="text-white/90">渠道健康与自动故障转移</div><div class="mt-1 text-xs text-white/40">优先级、权重、熔断与恢复检查</div></div>
            <div><div class="text-white/90">请求链路诊断</div><div class="mt-1 text-xs text-white/40">按请求 ID 定位上游响应与故障切换</div></div>
          </div>
        </div>

        <div class="flex items-center gap-3 text-xs text-white/35">
          <i class="status-dot" :class="serviceStatus === 'online' ? 'status-dot-live' : serviceStatus === 'checking' ? 'status-dot-capture' : 'status-dot-idle'"></i>
          {{ serviceMeta[serviceStatus].label }}
        </div>
      </section>

      <section class="flex min-h-screen items-center justify-center bg-canvas px-4 py-8 sm:px-8 lg:rounded-l-[32px] lg:px-12">
        <div class="w-full max-w-md">
          <div class="mb-8 lg:hidden">
            <div class="brand-wordmark text-[26px]">API<span>Relay</span></div>
            <div class="mt-2 font-mono text-[10px] uppercase tracking-[0.2em] text-faint">Routing control plane</div>
          </div>

          <div class="eyebrow">Administrator access</div>
          <h2 id="login-title" class="font-cond text-[36px] font-semibold leading-tight tracking-[-0.03em] text-ink">进入控制台</h2>
          <p class="mt-2 text-sm leading-6 text-soft">使用管理员账户管理路由和调用记录。</p>

          <form class="mt-8 space-y-5" aria-labelledby="login-title" @submit.prevent="login">
            <div>
              <label class="field-label" for="lg-user">用户名</label>
              <input id="lg-user" v-model="username" class="input min-h-11" placeholder="请输入用户名" autocomplete="username" data-autofocus />
            </div>
            <div>
              <label class="field-label" for="lg-pass">密码</label>
              <input id="lg-pass" v-model="password" type="password" class="input min-h-11" placeholder="请输入密码" autocomplete="current-password" />
            </div>
            <button type="submit" class="btn btn-primary min-h-11 w-full" :disabled="loading">
              {{ loading ? '正在验证…' : '登录控制台' }}
            </button>
          </form>

          <div v-if="isDev" class="mt-4 rounded-xl border border-line bg-white px-4 py-3 text-xs text-soft">
            开发环境账户 <code class="ml-1 text-ink">admin / admin123</code>
          </div>

          <button type="button" class="mt-8 flex w-full items-center justify-between rounded-xl border border-line bg-white px-4 py-3 text-left text-xs shadow-sheet lg:hidden" @click="checkService">
            <span class="text-soft">服务运行状态</span>
            <span class="inline-flex items-center gap-2" :class="serviceMeta[serviceStatus].text"><i class="status-dot" :class="serviceMeta[serviceStatus].dot"></i>{{ serviceMeta[serviceStatus].label }}</span>
          </button>
        </div>
      </section>
    </div>
  </main>
</template>

<script setup>
import { getCurrentInstance, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import api, { TOKEN_KEY } from '../api'
import ConsoleIcon from '../components/ConsoleIcon.vue'
import InlineNotice from '../components/InlineNotice.vue'

const router = useRouter()
const { proxy } = getCurrentInstance()
const isDev = import.meta.env.DEV

const username = ref('admin')
const password = ref('')
const showPassword = ref(false)
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
  <main class="login-page flex min-h-[100dvh] items-center bg-sidebar px-4 py-5 text-ink sm:px-6 lg:px-10">
    <div class="mx-auto grid w-full max-w-5xl overflow-hidden rounded-lg border border-line bg-paper shadow-lift lg:grid-cols-[minmax(0,.8fr)_minmax(380px,1fr)]">
      <section class="flex flex-col justify-between border-b border-line bg-sidebar p-6 sm:p-8 lg:min-h-[560px] lg:border-b-0 lg:border-r">
        <div class="flex items-center justify-between gap-4">
          <div class="flex items-center gap-3">
            <span class="flex h-9 w-9 items-center justify-center rounded-md border border-blue/30 bg-blue-wash text-blue-grid"><ConsoleIcon name="command" class="h-5 w-5" /></span>
            <div><div class="brand-wordmark brand-wordmark-inverse text-[22px]">API<span>Relay</span></div><div class="mt-1 font-mono text-[9px] uppercase tracking-[.16em] text-white/40">Internal control plane</div></div>
          </div>
          <button type="button" class="service-status service-status-compact text-white/55" :class="`service-status-${serviceStatus}`" @click="checkService">
            <span class="service-status-dot"></span><span>{{ serviceMeta[serviceStatus].label }}</span>
          </button>
        </div>

        <div class="hidden py-10 lg:block">
          <div class="font-mono text-[10px] uppercase tracking-[.16em] text-blue-grid">Administrator access</div>
          <h1 class="mt-3 max-w-sm text-2xl font-semibold leading-tight tracking-[-.02em] text-white">APIRelay 运维控制台</h1>
          <p class="mt-3 max-w-sm text-sm leading-6 text-white/50">用于维护上游连接、路由规则、运行记录与可靠性策略。仅限授权人员访问。</p>
          <dl class="mt-8 grid max-w-sm gap-3 text-xs">
            <div class="flex items-center justify-between border-b border-white/10 pb-3"><dt class="text-white/40">入口类型</dt><dd class="font-mono text-white/75">INTERNAL</dd></div>
            <div class="flex items-center justify-between border-b border-white/10 pb-3"><dt class="text-white/40">认证方式</dt><dd class="font-mono text-white/75">PASSWORD</dd></div>
            <div class="flex items-center justify-between"><dt class="text-white/40">会话范围</dt><dd class="font-mono text-white/75">CONTROL PLANE</dd></div>
          </dl>
        </div>

        <p class="hidden text-[11px] leading-5 text-white/35 lg:block">登录活动由当前 APIRelay 实例处理。请勿在共享设备上保存管理员密码。</p>
      </section>

      <section class="flex min-h-0 items-center bg-paper p-5 sm:p-8 lg:p-10">
        <div class="mx-auto w-full max-w-sm">
          <div class="mb-6 lg:mb-8">
            <div class="eyebrow">安全登录</div>
            <h2 id="login-title" class="text-2xl font-semibold tracking-[-.02em] text-ink">进入控制台</h2>
            <p class="mt-2 text-sm leading-6 text-soft">使用管理员凭据继续。</p>
          </div>

          <InlineNotice v-if="serviceStatus === 'unknown'" tone="warning" title="服务状态未确认">
            登录接口可能仍然可用；你也可以点击左侧状态重新检查。
          </InlineNotice>

          <form class="mt-5 space-y-4" aria-labelledby="login-title" @submit.prevent="login">
            <div>
              <label class="field-label" for="lg-user">用户名</label>
              <div class="relative">
                <ConsoleIcon name="user" class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-faint" />
                <input id="lg-user" v-model="username" class="input min-h-11 pl-10" placeholder="admin" autocomplete="username" autocapitalize="none" spellcheck="false" data-autofocus />
              </div>
            </div>
            <div>
              <div class="flex items-center justify-between gap-3"><label class="field-label" for="lg-pass">密码</label><button class="mb-1.5 text-xs text-blue hover:text-blue-deep" type="button" :aria-pressed="showPassword" @click="showPassword = !showPassword">{{ showPassword ? '隐藏密码' : '显示密码' }}</button></div>
              <div class="relative">
                <ConsoleIcon name="key" class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-faint" />
                <input id="lg-pass" v-model="password" :type="showPassword ? 'text' : 'password'" class="input min-h-11 pl-10" placeholder="输入管理员密码" autocomplete="current-password" />
              </div>
            </div>
            <button type="submit" class="btn btn-primary min-h-11 w-full" :disabled="loading"><ConsoleIcon :name="loading ? 'arrowPath' : 'arrowRightStart'" class="h-4 w-4" :class="{ 'animate-spin': loading }" />{{ loading ? '正在验证…' : '登录控制台' }}</button>
          </form>

          <InlineNotice v-if="isDev" class="mt-4" title="开发环境凭据">
            用户名与密码已允许浏览器自动填充。测试账户：<code class="font-mono text-ink">admin / admin123</code>
          </InlineNotice>

          <div class="mt-6 flex items-center justify-between border-t border-line pt-4 text-[11px] text-faint">
            <span>默认用户名 <code class="font-mono text-soft">admin</code></span>
            <span class="font-mono uppercase tracking-wider">APIRelay</span>
          </div>
        </div>
      </section>
    </div>
  </main>
</template>

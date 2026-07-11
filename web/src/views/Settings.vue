<script setup>
import { computed, getCurrentInstance, onMounted, ref } from 'vue'
import api from '../api'
import PageState from '../components/PageState.vue'

const { proxy } = getCurrentInstance()

const tabs = [
  { id: 'logging', label: '完整日志' },
  { id: 'network', label: '上游网络' },
  { id: 'testing', label: '模型测试' },
  { id: 'protocols', label: '协议规则' },
  { id: 'prices', label: '模型价格' },
  { id: 'breaker', label: '熔断器' },
]
const activeTab = ref('logging')
const rules = ref([])
const protocols = ref([])
const prices = ref([])
const testModel = ref('')
const loadingSettings = ref(true)
const settingsError = ref('')
const savingRules = ref(false)
const savingPrices = ref(false)
const savingBreaker = ref(false)
const savingLogging = ref(false)
const logging = ref({
  enabled: false,
  sanitized_header_keys: ['Authorization', 'Proxy-Authorization', 'Cookie', 'Set-Cookie', 'X-API-Key'],
  record_client_request: true,
  record_upstream_request: true,
  record_upstream_resp: true,
  record_client_resp: true,
})

const network = ref({ mode: 'system', manual_url: '', no_proxy: '', effective_source: '', effective_proxy_url: '' })
const networkTarget = ref('https://api.openai.com/v1/models')
const networkResult = ref(null)
const savingNetwork = ref(false)
const testingNetwork = ref(false)
const testPrompt = ref("Say 'hi' in one word.")
const savingTestPrompt = ref(false)

const defaultBreaker = {
  failure_threshold: 5,
  success_threshold: 2,
  timeout_seconds: 30,
  error_rate_threshold: 0.5,
  min_requests: 10,
  window_seconds: 60,
  channel_max_retries: 1,
}
const breaker = ref({ ...defaultBreaker })
const breakerError = ref('')

const testResult = computed(() => {
  const model = testModel.value.trim()
  if (!model) return { protocol: '', index: -1, invalid: false }
  for (let index = 0; index < rules.value.length; index += 1) {
    const rule = rules.value[index]
    if (!rule.pattern || !rule.protocol) continue
    try {
      if (new RegExp(rule.pattern).test(model)) return { protocol: rule.protocol, index, invalid: false }
    } catch {
      return { protocol: '', index, invalid: true }
    }
  }
  return { protocol: '', index: -1, invalid: false }
})

function notify(message, type = 'info') {
  proxy.$toast.add(message, type)
}

function selectTab(id) {
  activeTab.value = id
}

function onTabKeydown(event, index) {
  let target = index
  if (event.key === 'ArrowRight') target = (index + 1) % tabs.length
  else if (event.key === 'ArrowLeft') target = (index - 1 + tabs.length) % tabs.length
  else if (event.key === 'Home') target = 0
  else if (event.key === 'End') target = tabs.length - 1
  else return

  event.preventDefault()
  activeTab.value = tabs[target].id
  event.currentTarget.parentElement?.querySelectorAll('[role="tab"]')[target]?.focus()
}

function protocolName(value) {
  return protocols.value.find((item) => item.value === value)?.name || value || '未指定'
}

function addRule() {
  rules.value.push({ pattern: '', protocol: protocols.value[0]?.value || 'anthropic' })
}

function moveRule(index, direction) {
  const target = index + direction
  if (target < 0 || target >= rules.value.length) return
  const next = [...rules.value]
  ;[next[index], next[target]] = [next[target], next[index]]
  rules.value = next
}

function removeRule(index) {
  rules.value.splice(index, 1)
}

function addPrice() {
  prices.value.push({ model: '', input: 0, output: 0 })
}

async function loadSettings() {
  loadingSettings.value = true
  settingsError.value = ''
  try {
    const [ruleData, protocolData, priceData, breakerData, loggingData, networkData, promptData] = await Promise.all([
      api.get('/settings/protocol-rules'),
      api.get('/protocols'),
      api.get('/settings/model-prices'),
      api.get('/settings/circuit-breaker'),
      api.get('/settings/logging'),
      api.get('/settings/network'),
      api.get('/settings/test-prompt'),
    ])
    rules.value = (ruleData || []).map((item) => ({
      pattern: item.pattern || '',
      protocol: item.protocol || 'anthropic',
    }))
    protocols.value = protocolData || []
    prices.value = (priceData || []).map((item) => ({
      model: item.model || '',
      input: item.input || 0,
      output: item.output || 0,
    }))
    breaker.value = { ...defaultBreaker, ...(breakerData || {}) }
    logging.value = { ...logging.value, ...(loggingData || {}) }
    network.value = { ...network.value, ...(networkData || {}) }
    testPrompt.value = promptData?.prompt || testPrompt.value
  } catch (error) {
    settingsError.value = error.message || '设置初始化失败'
  } finally {
    loadingSettings.value = false
  }
}

async function saveRules() {
  savingRules.value = true
  try {
    const clean = rules.value
      .filter((rule) => rule.pattern.trim() && rule.protocol)
      .map((rule) => ({ pattern: rule.pattern.trim(), protocol: rule.protocol }))
    await api.put('/settings/protocol-rules', clean)
    rules.value = clean
    notify('全局协议规则已保存', 'success')
  } catch (error) {
    notify(`协议规则保存失败: ${error.message}`, 'error')
  } finally {
    savingRules.value = false
  }
}

async function savePrices() {
  savingPrices.value = true
  try {
    const clean = prices.value
      .filter((price) => price.model.trim())
      .map((price) => ({
        model: price.model.trim(),
        input: Number(price.input) || 0,
        output: Number(price.output) || 0,
      }))
    await api.put('/settings/model-prices', clean)
    prices.value = clean
    notify('全局模型价格已保存', 'success')
  } catch (error) {
    notify(`模型价格保存失败: ${error.message}`, 'error')
  } finally {
    savingPrices.value = false
  }
}

function resetBreakerDefaults() {
  breaker.value = { ...defaultBreaker }
  breakerError.value = ''
  notify('已恢复推荐默认值，保存后生效', 'info')
}

function validateBreaker() {
  const checks = [
    ['失败阈值', breaker.value.failure_threshold, 1],
    ['恢复阈值', breaker.value.success_threshold, 1],
    ['熔断超时', breaker.value.timeout_seconds, 1],
    ['最小请求数', breaker.value.min_requests, 1],
    ['统计窗口', breaker.value.window_seconds, 1],
    ['单渠道重试次数', breaker.value.channel_max_retries, 0],
  ]
  for (const [label, value, min] of checks) {
    const number = Number(value)
    if (!Number.isFinite(number) || number < min) return `${label}必须大于或等于 ${min}`
  }
  const rate = Number(breaker.value.error_rate_threshold)
  if (!Number.isFinite(rate) || rate < 0 || rate > 1) return '错误率阈值必须位于 0 到 1 之间'
  return ''
}

async function saveBreaker() {
  breakerError.value = validateBreaker()
  if (breakerError.value) {
    notify(breakerError.value, 'warn')
    return
  }
  savingBreaker.value = true
  try {
    const clean = {
      ...breaker.value,
      failure_threshold: Number(breaker.value.failure_threshold),
      success_threshold: Number(breaker.value.success_threshold),
      timeout_seconds: Number(breaker.value.timeout_seconds),
      error_rate_threshold: Number(breaker.value.error_rate_threshold),
      min_requests: Number(breaker.value.min_requests),
      window_seconds: Number(breaker.value.window_seconds),
      channel_max_retries: Number(breaker.value.channel_max_retries),
    }
    const response = await api.put('/settings/circuit-breaker', clean)
    breaker.value = { ...defaultBreaker, ...(response?.config || clean) }
    breakerError.value = ''
    notify('熔断器配置已保存', 'success')
  } catch (error) {
    breakerError.value = error.message || '熔断器配置保存失败'
    notify(`熔断器配置保存失败: ${breakerError.value}`, 'error')
  } finally {
    savingBreaker.value = false
  }
}

async function saveNetwork() {
  savingNetwork.value = true
  try {
    const payload = {
      mode: network.value.mode,
      manual_url: network.value.manual_url.trim(),
      no_proxy: network.value.no_proxy.trim(),
    }
    const response = await api.put('/settings/network', payload)
    network.value = { ...network.value, ...(response || payload) }
    notify('上游网络策略已热切换', 'success')
  } catch (error) {
    notify(`网络策略保存失败: ${error.message}`, 'error')
  } finally {
    savingNetwork.value = false
  }
}

async function runNetworkTest() {
  testingNetwork.value = true
  networkResult.value = null
  try {
    networkResult.value = await api.post('/settings/network/test', {
      mode: network.value.mode,
      manual_url: network.value.manual_url.trim(),
      no_proxy: network.value.no_proxy.trim(),
      target: networkTarget.value.trim(),
    })
  } catch (error) {
    networkResult.value = { success: false, stage: 'config', error: error.message }
  } finally {
    testingNetwork.value = false
  }
}

function stageLabel(stage) {
  return { config: '配置', dns: 'DNS', tcp: 'TCP', tcp_connected: 'TCP', tls: 'TLS', tls_connected: 'TLS', http: 'HTTP' }[stage] || stage || '等待'
}

async function saveTestPrompt() {
  const prompt = testPrompt.value.trim()
  if (!prompt) {
    notify('测试提示词不能为空', 'warn')
    return
  }
  savingTestPrompt.value = true
  try {
    const response = await api.put('/settings/test-prompt', { prompt })
    testPrompt.value = response?.prompt || prompt
    notify('全局测试提示词已保存', 'success')
  } catch (error) {
    notify(`测试提示词保存失败: ${error.message}`, 'error')
  } finally {
    savingTestPrompt.value = false
  }
}

async function saveLogging() {
  savingLogging.value = true
  try {
    const payload = {
      ...logging.value,
      sanitized_header_keys: ['Authorization', 'Proxy-Authorization', 'Cookie', 'Set-Cookie', 'X-API-Key'],
    }
    const response = await api.put('/settings/logging', payload)
    logging.value = { ...payload, ...(response || {}) }
    notify(logging.value.enabled ? '完整调用留痕已开启' : '完整调用留痕已关闭', 'success')
  } catch (error) {
    notify(`完整日志配置保存失败: ${error.message}`, 'error')
  } finally {
    savingLogging.value = false
  }
}

onMounted(() => {
  loadSettings()
})
</script>

<template>
  <div class="min-w-0 space-y-5">
    <header class="page-header">
      <div>
        <div class="eyebrow">Control plane settings</div>
        <h1 class="page-title">系统设置</h1>
        <p class="page-description">管理调用留痕、上游网络、模型测试、协议匹配、计价与熔断策略。</p>
      </div>
    </header>

    <nav class="min-w-0 overflow-x-auto" aria-label="设置分类">
      <div class="segmented min-w-max" role="tablist" aria-label="设置分类">
        <button
          v-for="(tab, index) in tabs"
          :id="`settings-tab-${tab.id}`"
          :key="tab.id"
          type="button"
          role="tab"
          :aria-selected="activeTab === tab.id"
          :aria-controls="`settings-panel-${tab.id}`"
          :tabindex="activeTab === tab.id ? 0 : -1"
          @click="selectTab(tab.id)"
          @keydown="onTabKeydown($event, index)"
        >
          {{ tab.label }}
        </button>
      </div>
    </nav>

    <section
      v-if="activeTab === 'logging'"
      id="settings-panel-logging"
      role="tabpanel"
      aria-labelledby="settings-tab-logging"
      tabindex="0"
      class="grid gap-5 xl:grid-cols-[minmax(0,1.35fr)_minmax(300px,0.65fr)]"
    >
      <div class="sheet overflow-hidden">
        <div class="sheet-head">
          <div>
            <div class="flex items-center gap-2">
              <span class="dim-title">完整调用留痕</span>
              <span class="chip" :class="logging.enabled ? 'chip-run' : ''">{{ logging.enabled ? '正在记录' : '仅记录摘要' }}</span>
            </div>
            <div class="mt-1 text-xs text-soft">保存后立即生效，无需重启服务</div>
          </div>
          <button class="btn btn-primary btn-sm" type="button" :disabled="savingLogging || loadingSettings" @click="saveLogging">
            {{ savingLogging ? '保存中…' : '保存日志策略' }}
          </button>
        </div>

        <PageState :loading="loadingSettings" :error="settingsError" @retry="loadSettings">
          <div class="space-y-5 p-4 sm:p-5">
            <button
              type="button"
              class="flex w-full items-center gap-4 rounded-xl border p-4 text-left transition"
              :class="logging.enabled ? 'border-blue/30 bg-blue-wash' : 'border-line bg-white hover:border-faint'"
              :aria-pressed="logging.enabled"
              @click="logging.enabled = !logging.enabled"
            >
              <span class="switch" :class="{ 'switch-on': logging.enabled }" aria-hidden="true"></span>
              <span class="min-w-0 flex-1">
                <span class="block font-medium text-ink">记录每次调用的完整链路</span>
                <span class="mt-1 block text-xs leading-5 text-soft">包含客户端请求、最终上游请求、上游响应、客户端响应和流式 SSE 内容，并自动使用 gzip 压缩。</span>
              </span>
              <span class="font-mono text-[11px] uppercase tracking-wider" :class="logging.enabled ? 'text-blue' : 'text-faint'">{{ logging.enabled ? 'ON' : 'OFF' }}</span>
            </button>

            <div>
              <div class="mb-3 flex items-center justify-between gap-3">
                <div>
                  <div class="dim-title">记录范围</div>
                  <p class="mt-1 text-xs text-soft">可按链路阶段减少敏感内容和存储占用。</p>
                </div>
                <span class="font-mono text-[10px] uppercase tracking-wider text-faint">gzip / json</span>
              </div>
              <div class="grid gap-3 sm:grid-cols-2">
                <label v-for="item in [
                  { key: 'record_client_request', title: '客户端请求', hint: '方法、路径、查询、请求头与原始正文' },
                  { key: 'record_upstream_request', title: '上游请求', hint: '最终 URL、请求头、模型映射后正文' },
                  { key: 'record_upstream_resp', title: '上游响应', hint: '状态、响应头、正文或原始 SSE' },
                  { key: 'record_client_resp', title: '客户端响应', hint: '最终状态、响应头与实际输出内容' },
                ]" :key="item.key" class="flex cursor-pointer items-start gap-3 rounded-xl border border-line bg-white p-3.5 hover:border-faint">
                  <input v-model="logging[item.key]" type="checkbox" class="mt-1 accent-blue" />
                  <span><span class="block text-[13px] font-medium text-ink">{{ item.title }}</span><span class="mt-1 block text-[11px] leading-5 text-soft">{{ item.hint }}</span></span>
                </label>
              </div>
            </div>
          </div>
        </PageState>
      </div>

      <aside class="space-y-4">
        <section class="rounded-xl border border-test/25 bg-test-wash p-4">
          <div class="font-cond text-base font-semibold text-test">数据与凭据安全</div>
          <p class="mt-2 text-xs leading-5 text-soft">完整正文可能包含提示词、文件内容和业务数据。只应在受控环境开启，并限制管理后台与数据库访问。</p>
          <div class="mt-4 border-t border-test/20 pt-3">
            <div class="font-mono text-[10px] uppercase tracking-wider text-test">始终脱敏的请求头</div>
            <div class="mt-2 flex flex-wrap gap-1.5">
              <code v-for="key in logging.sanitized_header_keys" :key="key" class="rounded border border-test/20 bg-white/70 px-1.5 py-0.5 text-[10px] text-test">{{ key }}</code>
            </div>
          </div>
        </section>
        <section class="sheet p-4">
          <div class="dim-title">存储方式</div>
          <div class="route-timeline mt-4 space-y-4 text-xs">
            <div><div class="font-medium text-ink">链路内采集</div><p class="mt-1 text-soft">流式内容边转发边记录，不改变 Flush 行为。</p></div>
            <div><div class="font-medium text-ink">异步压缩</div><p class="mt-1 text-soft">调用结束后序列化并以 gzip 压缩。</p></div>
            <div><div class="font-medium text-ink">按需解压</div><p class="mt-1 text-soft">日志列表只读摘要，打开详情时才加载完整内容。</p></div>
          </div>
        </section>
      </aside>
    </section>

    <section
      v-else-if="activeTab === 'network'"
      id="settings-panel-network"
      role="tabpanel"
      aria-labelledby="settings-tab-network"
      tabindex="0"
      class="grid gap-5 xl:grid-cols-[minmax(0,1.2fr)_minmax(320px,0.8fr)]"
    >
      <div class="sheet overflow-hidden">
        <div class="sheet-head">
          <div><div class="dim-title">上游连接策略</div><div class="mt-1 text-xs text-soft">保存后立即热切换，不中断进行中的请求</div></div>
          <button class="btn btn-primary btn-sm" type="button" :disabled="savingNetwork || loadingSettings" @click="saveNetwork">{{ savingNetwork ? '切换中…' : '保存并应用' }}</button>
        </div>
        <PageState :loading="loadingSettings" :error="settingsError" @retry="loadSettings">
          <div class="space-y-5 p-4 sm:p-5">
            <fieldset>
              <legend class="field-label">代理模式</legend>
              <div class="grid gap-2 sm:grid-cols-3">
                <label v-for="mode in [{ value: 'system', title: '跟随系统', hint: '读取 Windows 当前用户代理' }, { value: 'manual', title: '手动代理', hint: '显式指定代理 URL' }, { value: 'direct', title: '直接连接', hint: '绕过全部代理' }]" :key="mode.value" class="cursor-pointer rounded-xl border p-3" :class="network.mode === mode.value ? 'border-blue/35 bg-blue-wash' : 'border-line bg-white'">
                  <input v-model="network.mode" class="sr-only" type="radio" :value="mode.value" />
                  <span class="block text-sm font-medium text-ink">{{ mode.title }}</span><span class="mt-1 block text-xs text-soft">{{ mode.hint }}</span>
                </label>
              </div>
            </fieldset>
            <label v-if="network.mode === 'manual'"><span class="field-label">代理 URL</span><input v-model="network.manual_url" class="input input-mono" placeholder="http://127.0.0.1:7890 或 socks5://127.0.0.1:1080" /></label>
            <label><span class="field-label">不走代理</span><input v-model="network.no_proxy" class="input input-mono" placeholder="localhost,127.0.0.1,.internal.example" /><span class="field-help">逗号分隔主机、域名后缀或网段。</span></label>
            <div class="grid gap-3 sm:grid-cols-2">
              <div class="rounded-lg border border-line bg-ghost/40 p-3"><span class="field-label">实际来源</span><div class="font-mono text-sm text-ink">{{ network.effective_source || '等待加载' }}</div></div>
              <div class="rounded-lg border border-line bg-ghost/40 p-3"><span class="field-label">实际代理</span><div class="break-all font-mono text-xs text-ink">{{ network.effective_proxy_url || 'DIRECT' }}</div></div>
            </div>
          </div>
        </PageState>
      </div>
      <aside class="sheet overflow-hidden">
        <div class="sheet-head"><div><div class="dim-title">分段连通诊断</div><div class="mt-1 text-xs text-soft">使用左侧未保存的候选配置</div></div></div>
        <div class="space-y-4 p-4">
          <label><span class="field-label">测试地址</span><input v-model="networkTarget" class="input input-mono" type="url" /></label>
          <button class="btn w-full" type="button" :disabled="testingNetwork" @click="runNetworkTest">{{ testingNetwork ? '诊断中…' : '运行 DNS / TCP / TLS / HTTP 诊断' }}</button>
          <div v-if="networkResult" class="space-y-3" aria-live="polite">
            <div class="flex items-center justify-between rounded-lg border p-3" :class="networkResult.success ? 'border-run/30 bg-run-wash' : 'border-trip/30 bg-trip-wash'">
              <div><div class="text-sm font-medium" :class="networkResult.success ? 'text-run' : 'text-trip'">{{ networkResult.success ? '网络链路可用' : `停在 ${stageLabel(networkResult.stage)} 阶段` }}</div><div class="mt-1 font-mono text-xs text-soft">{{ networkResult.latency_ms || 0 }} ms · {{ networkResult.status_code || '—' }}</div></div>
              <span class="chip" :class="networkResult.success ? 'chip-run' : 'chip-trip'">{{ stageLabel(networkResult.stage) }}</span>
            </div>
            <div class="rounded-lg border border-line bg-white p-3"><span class="field-label">DNS 结果</span><div class="mt-2 flex flex-wrap gap-1"><code v-for="ip in networkResult.dns_results || []" :key="ip" class="chip">{{ ip }}</code><span v-if="!networkResult.dns_results?.length" class="text-xs text-soft">无</span></div></div>
            <div class="rounded-lg border border-line bg-white p-3 text-xs"><div><span class="text-soft">代理来源：</span>{{ networkResult.proxy_source || '—' }}</div><div class="mt-1 break-all font-mono">{{ networkResult.proxy_url || 'DIRECT' }}</div></div>
            <div v-if="networkResult.error" class="rounded-lg border border-trip/30 bg-trip-wash p-3 text-xs leading-5 text-trip" role="alert">{{ networkResult.error }}</div>
          </div>
        </div>
      </aside>
    </section>

    <section v-else-if="activeTab === 'testing'" id="settings-panel-testing" role="tabpanel" aria-labelledby="settings-tab-testing" tabindex="0" class="sheet overflow-hidden">
      <div class="sheet-head"><div><div class="dim-title">模型测试提示词</div><div class="mt-1 text-xs text-soft">渠道未设置覆盖值时使用此全局默认</div></div><button class="btn btn-primary btn-sm" type="button" :disabled="savingTestPrompt || loadingSettings" @click="saveTestPrompt">{{ savingTestPrompt ? '保存中…' : '保存提示词' }}</button></div>
      <PageState :loading="loadingSettings" :error="settingsError" @retry="loadSettings">
        <div class="grid gap-5 p-4 sm:p-5 lg:grid-cols-[minmax(0,1fr)_300px]">
          <label><span class="field-label">全局默认提示词</span><textarea v-model="testPrompt" class="input min-h-36 resize-y" maxlength="4000" placeholder="要求模型返回简短、可验证的回答"></textarea><span class="field-help">用于单模型测试与批量体检；建议保持输出短小以减少费用。</span></label>
          <aside class="rounded-xl border border-blue/20 bg-blue-wash p-4 text-xs leading-5 text-soft"><div class="font-medium text-blue">优先级</div><ol class="mt-2 space-y-2"><li>1. 本次测试显式提示词</li><li>2. 渠道自定义提示词</li><li>3. 此处全局默认提示词</li></ol></aside>
        </div>
      </PageState>
    </section>

    <section
      v-else-if="activeTab === 'protocols'"
      id="settings-panel-protocols"
      role="tabpanel"
      aria-labelledby="settings-tab-protocols"
      tabindex="0"
      class="sheet"
    >
      <div class="sheet-head">
        <div>
          <div class="dim-title">协议规则</div>
          <div class="mt-1 text-xs text-soft">正则首个命中，列表顺序即优先级</div>
        </div>
        <button class="btn btn-primary btn-sm" type="button" :disabled="savingRules || loadingSettings" @click="saveRules">
          {{ savingRules ? '保存中…' : '保存规则' }}
        </button>
      </div>

      <PageState :loading="loadingSettings" :error="settingsError" @retry="loadSettings">
        <div class="grid gap-5 p-4 xl:grid-cols-[minmax(0,1.35fr)_minmax(280px,0.65fr)]">
          <div class="min-w-0">
            <p class="mb-3 text-xs leading-5 text-soft">优先级：模型显式设置 → 渠道规则 → 全局规则 → 渠道默认。使用上移、下移按钮调整顺序。</p>
            <div class="space-y-3">
              <article v-for="(rule, index) in rules" :key="index" class="rounded-lg border border-line bg-white p-3">
                <div class="mb-3 flex items-center justify-between gap-2">
                  <span class="font-medium">规则 {{ index + 1 }}</span>
                  <div class="flex flex-wrap gap-1">
                    <button class="btn btn-sm" type="button" :disabled="index === 0" :aria-label="`上移规则 ${index + 1}`" @click="moveRule(index, -1)">上移</button>
                    <button class="btn btn-sm" type="button" :disabled="index === rules.length - 1" :aria-label="`下移规则 ${index + 1}`" @click="moveRule(index, 1)">下移</button>
                    <button class="btn btn-danger btn-sm" type="button" :aria-label="`删除规则 ${index + 1}`" @click="removeRule(index)">删除</button>
                  </div>
                </div>
                <div class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_180px]">
                  <label>
                    <span class="field-label">模型正则</span>
                    <input v-model="rule.pattern" class="input input-mono" placeholder="^claude- 或 gpt-.*" />
                  </label>
                  <label>
                    <span class="field-label">上游协议</span>
                    <select v-model="rule.protocol" class="input">
                      <option v-for="protocol in protocols" :key="protocol.value" :value="protocol.value">{{ protocol.name }}</option>
                    </select>
                  </label>
                </div>
              </article>
              <div v-if="!rules.length" class="rounded-lg border border-dashed border-line py-8 text-center text-sm text-soft">暂无全局协议规则</div>
            </div>
            <button class="btn mt-3" type="button" @click="addRule">添加规则</button>
          </div>

          <aside class="h-fit rounded-lg border border-line bg-ghost/40 p-4">
            <div class="dim-title mb-3">测试规则</div>
            <label>
              <span class="field-label">模型显示名</span>
              <input v-model="testModel" class="input input-mono" placeholder="claude-3-5-sonnet" />
            </label>
            <div class="mt-3 rounded-lg border border-line bg-white p-3">
              <span class="field-label">首个命中结果</span>
              <div v-if="!testModel.trim()" class="text-sm text-soft">输入模型名开始测试</div>
              <div v-else-if="testResult.invalid" class="flex flex-wrap items-center gap-2">
                <span class="chip chip-trip">正则无效</span>
                <span class="text-xs text-soft">规则 {{ testResult.index + 1 }} 无法编译</span>
              </div>
              <div v-else-if="testResult.protocol" class="flex flex-wrap items-center gap-2">
                <span class="chip chip-blue">{{ protocolName(testResult.protocol) }}</span>
                <span class="text-xs text-soft">规则 {{ testResult.index + 1 }}</span>
              </div>
              <span v-else class="chip">未命中，使用渠道默认</span>
            </div>
            <div class="mt-3">
              <span class="field-label">可用协议</span>
              <div class="flex flex-wrap gap-1">
                <span v-for="protocol in protocols" :key="protocol.value" class="chip chip-blue">{{ protocol.name }}</span>
              </div>
            </div>
          </aside>
        </div>
      </PageState>
    </section>

    <section
      v-else-if="activeTab === 'prices'"
      id="settings-panel-prices"
      role="tabpanel"
      aria-labelledby="settings-tab-prices"
      tabindex="0"
      class="sheet"
    >
      <div class="sheet-head">
        <div>
          <div class="dim-title">模型价格</div>
          <div class="mt-1 text-xs text-soft">单位为 USD / 1M tokens，default 可作默认价格</div>
        </div>
        <button class="btn btn-primary btn-sm" type="button" :disabled="savingPrices || loadingSettings" @click="savePrices">
          {{ savingPrices ? '保存中…' : '保存价格' }}
        </button>
      </div>

      <PageState :loading="loadingSettings" :error="settingsError" @retry="loadSettings">
        <div class="p-4">
          <p class="mb-3 text-xs leading-5 text-soft">优先级：渠道模型价格 → 全局价格 → 不计费。</p>
          <div class="space-y-3">
            <article v-for="(price, index) in prices" :key="index" class="rounded-lg border border-line bg-white p-3">
              <div class="mb-3 flex items-center justify-between gap-2">
                <span class="font-medium">价格 {{ index + 1 }}</span>
                <button class="btn btn-danger btn-sm" type="button" :aria-label="`删除价格 ${index + 1}`" @click="prices.splice(index, 1)">删除</button>
              </div>
              <div class="grid gap-3 md:grid-cols-[minmax(0,1fr)_160px_160px]">
                <label>
                  <span class="field-label">模型</span>
                  <input v-model="price.model" class="input input-mono" placeholder="模型名或 default" />
                </label>
                <label>
                  <span class="field-label">输入 $/1M</span>
                  <input v-model.number="price.input" class="input input-mono text-right" type="number" min="0" step="0.01" inputmode="decimal" placeholder="0" />
                </label>
                <label>
                  <span class="field-label">输出 $/1M</span>
                  <input v-model.number="price.output" class="input input-mono text-right" type="number" min="0" step="0.01" inputmode="decimal" placeholder="0" />
                </label>
              </div>
            </article>
            <div v-if="!prices.length" class="rounded-lg border border-dashed border-line py-8 text-center text-sm text-soft">暂无价格条目，未配置时不计费</div>
          </div>
          <button class="btn mt-3" type="button" @click="addPrice">添加价格</button>
        </div>
      </PageState>
    </section>

    <section
      v-else
      id="settings-panel-breaker"
      role="tabpanel"
      aria-labelledby="settings-tab-breaker"
      tabindex="0"
      class="sheet"
    >
      <div class="sheet-head">
        <div>
          <div class="dim-title">熔断器</div>
          <div class="mt-1 text-xs text-soft">设置失败判断、恢复条件、统计窗口和重试次数</div>
        </div>
        <div class="flex flex-wrap justify-end gap-2">
          <button class="btn btn-sm" type="button" :disabled="savingBreaker || loadingSettings" @click="resetBreakerDefaults">恢复默认</button>
          <button class="btn btn-primary btn-sm" type="button" :disabled="savingBreaker || loadingSettings" @click="saveBreaker">
            {{ savingBreaker ? '保存中…' : '保存配置' }}
          </button>
        </div>
      </div>

      <PageState :loading="loadingSettings" :error="settingsError" @retry="loadSettings">
        <div class="space-y-4 p-4">
          <p class="max-w-4xl text-xs leading-5 text-soft">错误率只统计滑动窗口内的请求。达到连续失败或错误率条件后暂停向该渠道发送请求；超时后进行恢复检查，连续成功达到恢复阈值后恢复正常。同渠道重试发生在切换渠道之前。</p>

          <div v-if="breakerError" class="rounded-lg border border-trip/30 bg-trip-wash px-3 py-2 text-sm text-trip" role="alert">{{ breakerError }}</div>

          <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <label class="rounded-lg border border-line bg-white p-3">
              <span class="field-label">失败阈值</span>
              <input v-model.number="breaker.failure_threshold" class="input input-mono" type="number" min="1" step="1" />
              <span class="field-help">连续失败达到此数量后暂停请求，至少为 1。</span>
            </label>
            <label class="rounded-lg border border-line bg-white p-3">
              <span class="field-label">恢复阈值</span>
              <input v-model.number="breaker.success_threshold" class="input input-mono" type="number" min="1" step="1" />
              <span class="field-help">恢复检查连续成功次数，至少为 1。</span>
            </label>
            <label class="rounded-lg border border-line bg-white p-3">
              <span class="field-label">熔断超时（秒）</span>
              <input v-model.number="breaker.timeout_seconds" class="input input-mono" type="number" min="1" step="1" />
              <span class="field-help">暂停后等待恢复检查的时间，至少为 1 秒。</span>
            </label>
            <label class="rounded-lg border border-line bg-white p-3">
              <span class="field-label">错误率阈值</span>
              <input v-model.number="breaker.error_rate_threshold" class="input input-mono" type="number" min="0" max="1" step="0.01" />
              <span class="field-help">允许范围为 0 到 1。</span>
            </label>
            <label class="rounded-lg border border-line bg-white p-3">
              <span class="field-label">最小请求数</span>
              <input v-model.number="breaker.min_requests" class="input input-mono" type="number" min="1" step="1" />
              <span class="field-help">启用错误率判断所需的最小样本数。</span>
            </label>
            <label class="rounded-lg border border-line bg-white p-3">
              <span class="field-label">统计窗口（秒）</span>
              <input v-model.number="breaker.window_seconds" class="input input-mono" type="number" min="1" step="1" />
              <span class="field-help">错误率统计的滑动窗口长度。</span>
            </label>
            <label class="rounded-lg border border-line bg-white p-3">
              <span class="field-label">单渠道重试次数</span>
              <input v-model.number="breaker.channel_max_retries" class="input input-mono" type="number" min="0" step="1" />
              <span class="field-help">临时错误在原渠道重试的次数，可为 0。</span>
            </label>
            <div class="rounded-lg border border-blue/20 bg-blue-wash p-3">
              <span class="field-label text-blue">推荐默认值</span>
              <dl class="grid grid-cols-[1fr_auto] gap-x-3 gap-y-1 text-xs text-blue">
                <dt>失败阈值</dt><dd class="font-mono">5</dd>
                <dt>恢复阈值</dt><dd class="font-mono">2</dd>
                <dt>超时</dt><dd class="font-mono">30s</dd>
                <dt>错误率</dt><dd class="font-mono">0.5</dd>
                <dt>最小请求数</dt><dd class="font-mono">10</dd>
                <dt>统计窗口</dt><dd class="font-mono">60s</dd>
                <dt>重试次数</dt><dd class="font-mono">1</dd>
              </dl>
            </div>
          </div>
        </div>
      </PageState>
    </section>
  </div>
</template>

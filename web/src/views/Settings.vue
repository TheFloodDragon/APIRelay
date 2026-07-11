<script setup>
import { computed, getCurrentInstance, nextTick, onMounted, ref, watch } from 'vue'
import api from '../api'
import PageState from '../components/PageState.vue'

const { proxy } = getCurrentInstance()

const tabs = [
  { id: 'logging', label: '完整日志' },
  { id: 'network', label: '上游网络' },
  { id: 'testing', label: '模型测试' },
  { id: 'protocols', label: '协议规则' },
  { id: 'prices', label: '模型价格' },
  { id: 'health', label: '模型健康' },
  { id: 'breaker', label: '熔断器' },
]
const activeTab = ref('logging')
const rules = ref([])
const protocols = ref([])
const prices = ref([])
const testModel = ref('')
const loadingSettings = ref(true)
const settingsError = ref('')
const hydrated = ref(false)
const saveState = ref(Object.fromEntries(tabs.map((tab) => [tab.id, { status: 'idle', error: '' }])))
const saveTimers = {}
const saveInFlight = {}
const savePending = {}
const skipDebounce = {}
const savedSnapshots = {}
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
const testingNetwork = ref(false)
const testPrompt = ref("Say 'hi' in one word.")
const modelHealth = ref({
  recent_count: 100,
  window_hours: 24,
  healthy_threshold: 95,
  warning_threshold: 70,
})
const modelHealthError = ref('')

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

function snapshot(value) {
  return JSON.stringify(value)
}

function setSaveState(section, status, error = '') {
  saveState.value[section] = { status, error }
}

function saveStateLabel(section) {
  const state = saveState.value[section]
  if (state.status === 'saving') return '正在保存…'
  if (state.status === 'saved') return '已保存'
  if (state.status === 'error') return '保存失败'
  if (state.status === 'invalid') return '请修正后自动保存'
  return '自动保存'
}

function saveStateClass(section) {
  const status = saveState.value[section].status
  if (status === 'saved') return 'text-run'
  if (status === 'error' || status === 'invalid') return 'text-trip'
  if (status === 'saving') return 'text-blue'
  return 'text-faint'
}

function sectionStatus(section) {
  return {
    text: saveStateLabel(section),
    title: saveState.value[section].error || saveStateLabel(section),
    class: saveStateClass(section),
  }
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
  saveImmediately('protocols')
}

function removeRule(index) {
  rules.value.splice(index, 1)
  saveImmediately('protocols')
}

function addPrice() {
  prices.value.push({ model: '', input: 0, output: 0 })
}

function removePrice(index) {
  prices.value.splice(index, 1)
  saveImmediately('prices')
}

async function loadSettings() {
  hydrated.value = false
  loadingSettings.value = true
  settingsError.value = ''
  try {
    const [ruleData, protocolData, priceData, breakerData, loggingData, networkData, promptData, healthData] = await Promise.all([
      api.get('/settings/protocol-rules'),
      api.get('/protocols'),
      api.get('/settings/model-prices'),
      api.get('/settings/circuit-breaker'),
      api.get('/settings/logging'),
      api.get('/settings/network'),
      api.get('/settings/test-prompt'),
      api.get('/settings/model-health'),
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
    modelHealth.value = { ...modelHealth.value, ...(healthData || {}) }
    savedSnapshots.logging = snapshot(loggingPayload())
    savedSnapshots.network = snapshot(networkPayload())
    savedSnapshots.testing = snapshot(testPromptPayload())
    savedSnapshots.protocols = snapshot(rulesPayload())
    savedSnapshots.prices = snapshot(pricesPayload())
    savedSnapshots.health = snapshot(modelHealthPayload())
    savedSnapshots.breaker = snapshot(breakerPayload())
    Object.keys(saveState.value).forEach((section) => setSaveState(section, 'idle'))
  } catch (error) {
    settingsError.value = error.message || '设置初始化失败'
  } finally {
    await nextTick()
    loadingSettings.value = false
    hydrated.value = !settingsError.value
  }
}

function loggingPayload() {
  return {
    ...logging.value,
    sanitized_header_keys: ['Authorization', 'Proxy-Authorization', 'Cookie', 'Set-Cookie', 'X-API-Key'],
  }
}

function networkPayload() {
  const mode = network.value.mode
  if (!['system', 'manual', 'direct'].includes(mode)) throw new Error('请选择有效的代理模式')
  const manualUrl = network.value.manual_url.trim()
  if (mode === 'manual') {
    if (!manualUrl) throw new Error('手动代理模式下必须填写代理 URL')
    try {
      const parsed = new URL(manualUrl)
      if (!['http:', 'https:', 'socks:', 'socks5:', 'socks5h:'].includes(parsed.protocol)) throw new Error()
    } catch {
      throw new Error('代理 URL 格式无效')
    }
  }
  return { mode, manual_url: manualUrl, no_proxy: network.value.no_proxy.trim() }
}

function testPromptPayload() {
  const prompt = testPrompt.value.trim()
  if (!prompt) throw new Error('测试提示词不能为空')
  return { prompt }
}

function rulesPayload() {
  const clean = []
  for (const rule of rules.value) {
    const pattern = rule.pattern.trim()
    if (!pattern) continue
    if (!rule.protocol) throw new Error('每条协议规则都必须选择上游协议')
    try {
      new RegExp(pattern)
    } catch {
      throw new Error(`正则表达式“${pattern}”无效`)
    }
    clean.push({ pattern, protocol: rule.protocol })
  }
  return clean
}

function pricesPayload() {
  const clean = []
  const models = new Set()
  for (const price of prices.value) {
    const model = price.model.trim()
    const input = Number(price.input)
    const output = Number(price.output)
    if (!model && (!input && !output)) continue
    if (!model) throw new Error('填写价格后必须指定模型名')
    if (!Number.isFinite(input) || input < 0 || !Number.isFinite(output) || output < 0) throw new Error(`模型“${model}”的价格必须是非负数`)
    if (models.has(model)) throw new Error(`模型“${model}”存在重复价格`)
    models.add(model)
    clean.push({ model, input, output })
  }
  return clean
}

function modelHealthPayload() {
  const payload = {
    recent_count: Number(modelHealth.value.recent_count),
    window_hours: Number(modelHealth.value.window_hours),
    healthy_threshold: Number(modelHealth.value.healthy_threshold),
    warning_threshold: Number(modelHealth.value.warning_threshold),
  }
  if (!Number.isInteger(payload.recent_count) || payload.recent_count < 1) throw new Error('最近请求数必须是大于或等于 1 的整数')
  if (!Number.isFinite(payload.window_hours) || payload.window_hours < 1) throw new Error('统计窗口必须大于或等于 1 小时')
  if (!Number.isFinite(payload.healthy_threshold) || payload.healthy_threshold < 0 || payload.healthy_threshold > 100) throw new Error('健康阈值必须位于 0% 到 100% 之间')
  if (!Number.isFinite(payload.warning_threshold) || payload.warning_threshold < 0 || payload.warning_threshold > 100) throw new Error('警告阈值必须位于 0% 到 100% 之间')
  if (payload.warning_threshold > payload.healthy_threshold) throw new Error('警告阈值不能高于健康阈值')
  return payload
}

function breakerPayload() {
  const error = validateBreaker()
  if (error) throw new Error(error)
  return {
    failure_threshold: Number(breaker.value.failure_threshold),
    success_threshold: Number(breaker.value.success_threshold),
    timeout_seconds: Number(breaker.value.timeout_seconds),
    error_rate_threshold: Number(breaker.value.error_rate_threshold),
    min_requests: Number(breaker.value.min_requests),
    window_seconds: Number(breaker.value.window_seconds),
    channel_max_retries: Number(breaker.value.channel_max_retries),
  }
}

const saveDefinitions = {
  logging: { endpoint: '/settings/logging', payload: loggingPayload },
  network: { endpoint: '/settings/network', payload: networkPayload },
  testing: { endpoint: '/settings/test-prompt', payload: testPromptPayload },
  protocols: { endpoint: '/settings/protocol-rules', payload: rulesPayload },
  prices: { endpoint: '/settings/model-prices', payload: pricesPayload },
  health: { endpoint: '/settings/model-health', payload: modelHealthPayload },
  breaker: { endpoint: '/settings/circuit-breaker', payload: breakerPayload },
}

async function persistSection(section) {
  if (!hydrated.value || loadingSettings.value) return
  if (saveInFlight[section]) {
    savePending[section] = true
    return
  }

  const definition = saveDefinitions[section]
  let payload
  try {
    payload = definition.payload()
  } catch (error) {
    if (section === 'breaker') breakerError.value = error.message
    if (section === 'health') modelHealthError.value = error.message
    setSaveState(section, 'invalid', error.message)
    return
  }

  const nextSnapshot = snapshot(payload)
  if (nextSnapshot === savedSnapshots[section]) {
    if (saveState.value[section].status === 'invalid') setSaveState(section, 'saved')
    return
  }

  saveInFlight[section] = true
  setSaveState(section, 'saving')
  try {
    const response = await api.put(definition.endpoint, payload)
    savedSnapshots[section] = nextSnapshot
    if (section === 'network' && response) {
      network.value.effective_source = response.effective_source || network.value.effective_source
      network.value.effective_proxy_url = response.effective_proxy_url ?? network.value.effective_proxy_url
    }
    if (section === 'breaker') breakerError.value = ''
    if (section === 'health') modelHealthError.value = ''
    setSaveState(section, 'saved')
  } catch (error) {
    const message = error.message || '保存失败'
    if (section === 'breaker') breakerError.value = message
    if (section === 'health') modelHealthError.value = message
    setSaveState(section, 'error', message)
  } finally {
    saveInFlight[section] = false
    if (savePending[section]) {
      savePending[section] = false
      persistSection(section)
    }
  }
}

function scheduleSave(section, delay = 450) {
  if (!hydrated.value || loadingSettings.value) return
  window.clearTimeout(saveTimers[section])
  saveTimers[section] = window.setTimeout(() => persistSection(section), delay)
}

function saveImmediately(section) {
  if (!hydrated.value || loadingSettings.value) return
  skipDebounce[section] = true
  window.clearTimeout(saveTimers[section])
  persistSection(section)
}

function watchSection(source, section) {
  watch(source, () => {
    if (skipDebounce[section]) {
      skipDebounce[section] = false
      return
    }
    scheduleSave(section)
  }, { deep: true })
}

function resetBreakerDefaults() {
  breaker.value = { ...defaultBreaker }
  breakerError.value = ''
  saveImmediately('breaker')
  notify('已恢复推荐默认值并自动保存', 'info')
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

watchSection(logging, 'logging')
watchSection(network, 'network')
watchSection(testPrompt, 'testing')
watchSection(rules, 'protocols')
watchSection(prices, 'prices')
watchSection(modelHealth, 'health')
watchSection(breaker, 'breaker')

onMounted(loadSettings)
</script>

<template>
  <div class="min-w-0 space-y-5">
    <header class="page-header">
      <div>
        <div class="eyebrow">Control plane settings</div>
        <h1 class="page-title">系统设置</h1>
        <p class="page-description">管理完整日志、上游网络、模型测试、协议匹配、计价、模型健康与熔断策略。</p>
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
            <div class="mt-1 text-xs text-soft">修改后自动保存并立即生效，无需重启服务</div>
          </div>
          <span class="font-mono text-[11px]" :class="sectionStatus('logging').class" :title="sectionStatus('logging').title" aria-live="polite">{{ sectionStatus('logging').text }}</span>
        </div>

        <PageState :loading="loadingSettings" :error="settingsError" @retry="loadSettings">
          <div class="space-y-5 p-4 sm:p-5">
            <button
              type="button"
              class="flex w-full items-center gap-4 rounded-xl border p-4 text-left transition"
              :class="logging.enabled ? 'border-blue/30 bg-blue-wash' : 'border-line bg-white hover:border-faint'"
              :aria-pressed="logging.enabled"
              @click="logging.enabled = !logging.enabled; saveImmediately('logging')"
            >
              <span class="switch" :class="{ 'switch-on': logging.enabled }" aria-hidden="true"></span>
              <span class="min-w-0 flex-1">
                <span class="block font-medium text-ink">记录每次调用的完整链路</span>
                <span class="mt-1 block text-xs leading-5 text-soft">包含客户端请求、最终上游请求、上游响应和客户端响应，并自动使用 gzip 压缩。</span>
              </span>
              <span class="font-mono text-[11px] uppercase tracking-wider" :class="logging.enabled ? 'text-blue' : 'text-faint'">{{ logging.enabled ? 'ON' : 'OFF' }}</span>
            </button>

            <div>
              <div class="mb-3 flex items-center justify-between gap-3">
                <div>
                  <div class="dim-title">记录范围</div>
                  <p class="mt-1 text-xs text-soft">按链路阶段控制记录范围与存储占用。</p>
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
                  <input v-model="logging[item.key]" type="checkbox" class="mt-1 accent-blue" @change="saveImmediately('logging')" />
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
            <div><div class="font-medium text-ink">按需解压</div><p class="mt-1 text-soft">打开日志详情时才加载完整内容。</p></div>
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
          <div><div class="dim-title">上游连接策略</div><div class="mt-1 text-xs text-soft">修改后自动热切换，不中断进行中的请求</div></div>
          <span class="font-mono text-[11px]" :class="sectionStatus('network').class" :title="sectionStatus('network').title" aria-live="polite">{{ sectionStatus('network').text }}</span>
        </div>
        <PageState :loading="loadingSettings" :error="settingsError" @retry="loadSettings">
          <div class="space-y-5 p-4 sm:p-5">
            <fieldset>
              <legend class="field-label">代理模式</legend>
              <div class="grid gap-2 sm:grid-cols-3">
                <label v-for="mode in [{ value: 'system', title: '跟随系统', hint: '读取 Windows 当前用户代理' }, { value: 'manual', title: '手动代理', hint: '显式指定代理 URL' }, { value: 'direct', title: '直接连接', hint: '绕过全部代理' }]" :key="mode.value" class="cursor-pointer rounded-xl border p-3" :class="network.mode === mode.value ? 'border-blue/35 bg-blue-wash' : 'border-line bg-white'">
                  <input v-model="network.mode" class="sr-only" type="radio" :value="mode.value" @change="saveImmediately('network')" />
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
        <div class="sheet-head"><div><div class="dim-title">分段连通诊断</div><div class="mt-1 text-xs text-soft">使用左侧当前候选配置</div></div></div>
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
      <div class="sheet-head"><div><div class="dim-title">模型测试提示词</div><div class="mt-1 text-xs text-soft">渠道未设置覆盖值时使用此全局默认</div></div><span class="font-mono text-[11px]" :class="sectionStatus('testing').class" :title="sectionStatus('testing').title" aria-live="polite">{{ sectionStatus('testing').text }}</span></div>
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
        <span class="font-mono text-[11px]" :class="sectionStatus('protocols').class" :title="sectionStatus('protocols').title" aria-live="polite">{{ sectionStatus('protocols').text }}</span>
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
                    <select v-model="rule.protocol" class="input" @change="saveImmediately('protocols')">
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
        <span class="font-mono text-[11px]" :class="sectionStatus('prices').class" :title="sectionStatus('prices').title" aria-live="polite">{{ sectionStatus('prices').text }}</span>
      </div>

      <PageState :loading="loadingSettings" :error="settingsError" @retry="loadSettings">
        <div class="p-4">
          <p class="mb-3 text-xs leading-5 text-soft">优先级：渠道模型价格 → 全局价格 → 不计费。</p>
          <div class="space-y-3">
            <article v-for="(price, index) in prices" :key="index" class="rounded-lg border border-line bg-white p-3">
              <div class="mb-3 flex items-center justify-between gap-2">
                <span class="font-medium">价格 {{ index + 1 }}</span>
                <button class="btn btn-danger btn-sm" type="button" :aria-label="`删除价格 ${index + 1}`" @click="removePrice(index)">删除</button>
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
      v-else-if="activeTab === 'health'"
      id="settings-panel-health"
      role="tabpanel"
      aria-labelledby="settings-tab-health"
      tabindex="0"
      class="sheet"
    >
      <div class="sheet-head">
        <div>
          <div class="dim-title">模型健康判定</div>
          <div class="mt-1 text-xs text-soft">按近期请求成功率划分健康、警告与异常状态</div>
        </div>
        <span class="font-mono text-[11px]" :class="sectionStatus('health').class" :title="sectionStatus('health').title" aria-live="polite">{{ sectionStatus('health').text }}</span>
      </div>
      <PageState :loading="loadingSettings" :error="settingsError" @retry="loadSettings">
        <div class="space-y-4 p-4 sm:p-5">
          <p class="max-w-4xl text-xs leading-5 text-soft">仅统计指定时间窗口内最近的请求。成功率达到健康阈值显示为健康；低于健康阈值但达到警告阈值时显示警告；更低则视为异常。</p>
          <div v-if="modelHealthError" class="rounded-lg border border-trip/30 bg-trip-wash px-3 py-2 text-sm text-trip" role="alert">{{ modelHealthError }}</div>
          <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <label class="rounded-lg border border-line bg-white p-3">
              <span class="field-label">最近请求数</span>
              <input v-model.number="modelHealth.recent_count" class="input input-mono" type="number" min="1" step="1" inputmode="numeric" />
              <span class="field-help">每个模型最多纳入判定的近期请求数量。</span>
            </label>
            <label class="rounded-lg border border-line bg-white p-3">
              <span class="field-label">统计窗口（小时）</span>
              <input v-model.number="modelHealth.window_hours" class="input input-mono" type="number" min="1" step="1" inputmode="numeric" />
              <span class="field-help">只使用此时间范围内的请求结果。</span>
            </label>
            <label class="rounded-lg border border-run/25 bg-run-wash p-3">
              <span class="field-label text-run">健康阈值</span>
              <input v-model.number="modelHealth.healthy_threshold" class="input input-mono" type="number" min="0" max="100" step="1" inputmode="decimal" />
              <span class="field-help">成功率达到该百分比时判定为健康。</span>
            </label>
            <label class="rounded-lg border border-test/25 bg-test-wash p-3">
              <span class="field-label text-test">警告阈值</span>
              <input v-model.number="modelHealth.warning_threshold" class="input input-mono" type="number" min="0" max="100" step="1" inputmode="decimal" />
              <span class="field-help">成功率低于该百分比时判定为异常，不得高于健康阈值。</span>
            </label>
          </div>
        </div>
      </PageState>
    </section>

    <section
      v-else-if="activeTab === 'breaker'"
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
        <div class="flex flex-wrap items-center justify-end gap-3">
          <span class="font-mono text-[11px]" :class="sectionStatus('breaker').class" :title="sectionStatus('breaker').title" aria-live="polite">{{ sectionStatus('breaker').text }}</span>
          <button class="btn btn-sm" type="button" :disabled="loadingSettings" @click="resetBreakerDefaults">恢复默认</button>
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

<script setup>
import { computed, getCurrentInstance, onMounted, ref } from 'vue'
import api from '../api'
import PageState from '../components/PageState.vue'

const { proxy } = getCurrentInstance()

const tabs = [
  { id: 'config', label: '配置文件' },
  { id: 'protocols', label: '协议规则' },
  { id: 'prices', label: '模型价格' },
  { id: 'breaker', label: '熔断器' },
]
const activeTab = ref('config')
const rules = ref([])
const protocols = ref([])
const prices = ref([])
const testModel = ref('')
const loadingSettings = ref(true)
const settingsError = ref('')
const savingRules = ref(false)
const savingPrices = ref(false)
const savingBreaker = ref(false)

const configFile = ref({ path: '', exists: false, content: '' })
const configDraft = ref('')
const loadingConfig = ref(true)
const configError = ref('')
const savingConfig = ref(false)

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

async function loadConfigFile() {
  loadingConfig.value = true
  configError.value = ''
  try {
    const data = await api.get('/settings/config-file')
    configFile.value = {
      path: data?.path || 'config.yaml',
      exists: !!data?.exists,
      content: data?.content || '',
    }
    configDraft.value = data?.content || ''
  } catch (error) {
    configError.value = error.message || '配置文件读取失败'
  } finally {
    loadingConfig.value = false
  }
}

async function saveConfigFile() {
  savingConfig.value = true
  try {
    const data = await api.put('/settings/config-file', { content: configDraft.value })
    configFile.value = {
      path: data?.path || configFile.value.path || 'config.yaml',
      exists: true,
      content: data?.content ?? configDraft.value,
    }
    configDraft.value = data?.content ?? configDraft.value
    notify(data?.message || '配置文件已写入，部分配置需要重启后生效', 'success')
  } catch (error) {
    notify(`配置文件保存失败: ${error.message}`, 'error')
  } finally {
    savingConfig.value = false
  }
}

async function loadSettings() {
  loadingSettings.value = true
  settingsError.value = ''
  try {
    const [ruleData, protocolData, priceData, breakerData] = await Promise.all([
      api.get('/settings/protocol-rules'),
      api.get('/protocols'),
      api.get('/settings/model-prices'),
      api.get('/settings/circuit-breaker'),
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

onMounted(() => {
  loadConfigFile()
  loadSettings()
})
</script>

<template>
  <div class="min-w-0 space-y-5">
    <header>
      <div class="eyebrow">系统设置</div>
      <h1 class="page-title">设置</h1>
      <p class="page-description">配置文件、协议规则、模型价格与熔断器分别保存。</p>
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
      v-if="activeTab === 'config'"
      id="settings-panel-config"
      role="tabpanel"
      aria-labelledby="settings-tab-config"
      tabindex="0"
      class="sheet"
    >
      <div class="sheet-head">
        <div>
          <div class="dim-title">配置文件</div>
          <div class="mt-1 text-xs text-soft">编辑服务启动配置 YAML</div>
        </div>
        <div class="flex flex-wrap justify-end gap-2">
          <button class="btn btn-sm" type="button" :disabled="loadingConfig || savingConfig" @click="loadConfigFile">
            {{ loadingConfig ? '读取中…' : '重新加载' }}
          </button>
          <button class="btn btn-primary btn-sm" type="button" :disabled="loadingConfig || savingConfig" @click="saveConfigFile">
            {{ savingConfig ? '保存中…' : '保存 YAML' }}
          </button>
        </div>
      </div>

      <PageState :loading="loadingConfig" :error="configError" @retry="loadConfigFile">
        <div class="space-y-4 p-4">
          <div class="grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(280px,0.65fr)]">
            <div class="rounded-lg border border-line bg-white p-3">
              <span class="field-label">当前配置文件路径</span>
              <div class="flex flex-wrap items-center gap-2">
                <code class="min-w-0 break-all text-sm">{{ configFile.path || 'config.yaml' }}</code>
                <span class="chip" :class="configFile.exists ? 'chip-run' : 'chip-test'">
                  {{ configFile.exists ? '文件存在' : '保存时创建' }}
                </span>
              </div>
            </div>
            <div class="rounded-lg border border-blue/20 bg-blue-wash p-3 text-xs leading-5 text-soft">
              保存前由服务端校验 YAML。写入成功不代表全部运行时参数立即更新；数据库、监听地址、认证初始化等配置通常需要重启进程后生效。
            </div>
          </div>
          <label>
            <span class="field-label">YAML 内容</span>
            <textarea
              v-model="configDraft"
              class="input input-mono min-h-[360px] resize-y whitespace-pre leading-5"
              spellcheck="false"
              placeholder="server:\n  port: 3000\n  host: 0.0.0.0\n"
            ></textarea>
          </label>
          <p class="text-xs text-soft">重新加载会覆盖尚未保存的编辑内容。</p>
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

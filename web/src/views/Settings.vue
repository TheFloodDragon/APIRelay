<template>
  <div>
    <div class="mb-5">
      <h2 class="page-title">设置</h2>
      <p class="page-subtitle">配置文件、全局协议映射、模型定价与熔断参数</p>
    </div>

    <!-- ===== 当前配置文件 ===== -->
    <div class="panel mb-4">
      <div class="px-4 h-12 flex items-center justify-between border-b border-line">
        <div class="min-w-0">
          <span class="font-mono text-sm font-medium text-t1">配置文件</span>
          <span class="tick ml-2">CONFIG.YAML</span>
        </div>
        <div class="flex items-center gap-2">
          <button class="btn-secondary btn-sm" :disabled="loadingConfig || savingConfig" @click="loadConfigFile">{{ loadingConfig ? '读取中…' : '重新加载文件' }}</button>
          <button class="btn-primary btn-sm" :disabled="savingConfig" @click="saveConfigFile">{{ savingConfig ? '写入中…' : '保存配置文件' }}</button>
        </div>
      </div>
      <div class="p-4 space-y-4">
        <div class="grid grid-cols-1 lg:grid-cols-[1fr_auto] gap-3 items-start">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <span class="tick">PATH</span>
              <span class="key-chip key-chip-full"><code>{{ configFile.path || 'config.yaml' }}</code></span>
              <span class="badge" :class="configFile.exists ? 'badge-online' : 'badge-warn'">{{ configFile.exists ? '已存在' : '保存时创建' }}</span>
            </div>
            <p class="hint">这里编辑的是当前进程启动时使用的配置文件路径；保存会写入文件，不代表所有运行时参数立即热更新。</p>
          </div>
          <div class="relay-callout">
            配置写入前会先按 APIRelay 配置结构解析 YAML。数据库、监听地址、认证初始化等参数通常需要重启后才会完整生效。
          </div>
        </div>
        <textarea
          v-model="configDraft"
          class="config-textarea"
          spellcheck="false"
          placeholder="server:\n  port: 3000\n  host: 0.0.0.0\n"
        ></textarea>
      </div>
    </div>

    <div class="grid grid-cols-1 xl:grid-cols-2 gap-4">
      <!-- ===== 全局协议规则 ===== -->
      <div class="panel">
        <div class="px-4 h-11 flex items-center justify-between border-b border-line">
          <div>
            <span class="font-mono text-sm font-medium text-t1">全局协议规则</span>
            <span class="tick ml-2">REGEX MATCH</span>
          </div>
          <button class="btn-primary btn-sm" :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存' }}</button>
        </div>

        <div class="p-4">
          <p class="hint mb-3">按模型显示名正则匹配上游协议。优先级：模型显式 &gt; 渠道规则 &gt; <span class="font-medium text-t1">全局规则</span> &gt; 渠道默认。</p>
          <div class="space-y-2">
            <div v-for="(r, i) in rules" :key="i" class="flex gap-2 items-center">
              <span class="font-mono text-2xs text-t3 w-6 text-center shrink-0">{{ String(i + 1).padStart(2, '0') }}</span>
              <input v-model="r.pattern" class="input font-mono text-xs flex-1" placeholder="正则，如 ^claude- 或 gpt-.*" />
              <select v-model="r.protocol" class="input text-xs w-36 shrink-0">
                <option v-for="p in protocols" :key="p.value" :value="p.value">{{ p.name }}</option>
              </select>
              <button class="text-t3 hover:text-[rgb(var(--rust))] px-2 shrink-0" @click="rules.splice(i, 1)">×</button>
            </div>
            <div v-if="!rules.length" class="empty-state inset !py-6">
              暂无全局规则
            </div>
          </div>
          <button class="btn-ghost btn-sm mt-3" @click="rules.push({ pattern: '', protocol: 'anthropic' })">+ 添加规则</button>
        </div>
      </div>

      <!-- ===== 规则测试器 ===== -->
      <div class="panel">
        <div class="px-4 h-11 flex items-center border-b border-line">
          <span class="font-mono text-sm font-medium text-t1">规则测试器</span>
          <span class="tick ml-2">DRY RUN</span>
        </div>
        <div class="p-4">
          <p class="hint mb-3">输入一个模型名，预览全局规则的首个命中结果（不含渠道级配置）</p>
          <div class="flex gap-2 items-center">
            <input v-model="testModel" class="input font-mono text-sm flex-1" placeholder="如 claude-3-5-sonnet" />
            <div class="shrink-0">
              <span v-if="testModel" class="badge font-mono" :class="testResult ? 'badge-signal' : 'badge-neutral'">
                {{ testResult || '未命中（用渠道默认）' }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ===== 全局模型价格 ===== -->
    <div class="panel mt-4">
      <div class="px-4 h-11 flex items-center justify-between border-b border-line">
        <div>
          <span class="font-mono text-sm font-medium text-t1">全局模型价格</span>
          <span class="tick ml-2">USD/1M TK</span>
        </div>
        <button class="btn-primary btn-sm" :disabled="savingPrices" @click="savePrices">{{ savingPrices ? '保存中…' : '保存' }}</button>
      </div>

      <div class="p-4">
        <p class="hint mb-3">单位：美元 / 100 万 tokens。模型名填 <code class="font-mono text-xs">default</code> 作为兜底价格。优先级：渠道模型价格 &gt; <span class="font-medium text-t1">全局价格</span> &gt; 不计费。</p>
        <div class="space-y-2">
          <div class="hidden lg:grid grid-cols-[40px_minmax(0,1fr)_120px_120px_32px] gap-2 items-center tick px-1">
            <span></span>
            <span>模型名</span>
            <span class="text-right pr-2">输入 $/1M</span>
            <span class="text-right pr-2">输出 $/1M</span>
            <span></span>
          </div>
          <div v-for="(p, i) in prices" :key="i" class="grid grid-cols-[40px_minmax(0,1fr)_120px_120px_32px] max-lg:grid-cols-[40px_minmax(0,1fr)_32px] gap-2 items-center">
            <span class="font-mono text-2xs text-t3 text-center">{{ String(i + 1).padStart(2, '0') }}</span>
            <input v-model="p.model" class="input font-mono text-xs" placeholder="模型名 或 default" />
            <input v-model.number="p.input" type="number" step="0.01" min="0" class="input text-xs text-right font-mono max-lg:hidden" placeholder="0" />
            <input v-model.number="p.output" type="number" step="0.01" min="0" class="input text-xs text-right font-mono max-lg:hidden" placeholder="0" />
            <button class="text-t3 hover:text-[rgb(var(--rust))] px-1" @click="prices.splice(i, 1)">×</button>
          </div>
          <div v-if="!prices.length" class="empty-state inset !py-6">
            暂无价格条目（未配置时不计费）
          </div>
        </div>
        <button class="btn-ghost btn-sm mt-3" @click="prices.push({ model: '', input: 0, output: 0 })">+ 添加价格</button>
      </div>
    </div>

    <!-- ===== 熔断器配置 ===== -->
    <div class="panel mt-4">
      <div class="px-4 h-11 flex items-center justify-between border-b border-line">
        <div>
          <span class="font-mono text-sm font-medium text-t1">熔断器配置</span>
          <span class="tick ml-2">CIRCUIT BREAKER</span>
        </div>
        <div class="flex items-center gap-2">
          <button class="btn-ghost btn-sm" :disabled="savingBreaker" @click="resetBreakerDefaults">恢复默认</button>
          <button class="btn-primary btn-sm" :disabled="savingBreaker" @click="saveBreaker">{{ savingBreaker ? '保存中…' : '保存' }}</button>
        </div>
      </div>

      <div class="p-4">
        <p class="hint mb-4">自动熔断故障渠道，防止级联失败。错误率只统计滑动窗口内的请求，过期失败不会长期误伤渠道；熔断后等待超时时间进入半开试探。</p>
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          <div>
            <label class="label">失败阈值</label>
            <input v-model.number="breaker.failure_threshold" type="number" min="1" class="input" />
            <p class="hint mt-1">连续失败多少次触发熔断</p>
          </div>
          <div>
            <label class="label">恢复阈值</label>
            <input v-model.number="breaker.success_threshold" type="number" min="1" class="input" />
            <p class="hint mt-1">半开状态连续成功多少次恢复</p>
          </div>
          <div>
            <label class="label">熔断超时（秒）</label>
            <input v-model.number="breaker.timeout_seconds" type="number" min="1" class="input" />
            <p class="hint mt-1">熔断后多久进入半开试探</p>
          </div>
          <div>
            <label class="label">错误率阈值</label>
            <input v-model.number="breaker.error_rate_threshold" type="number" min="0" max="1" step="0.01" class="input" />
            <p class="hint mt-1">错误率超过此值触发熔断（0-1）</p>
          </div>
          <div>
            <label class="label">最小请求数</label>
            <input v-model.number="breaker.min_requests" type="number" min="1" class="input" />
            <p class="hint mt-1">统计窗口最小请求数</p>
          </div>
          <div>
            <label class="label">统计窗口（秒）</label>
            <input v-model.number="breaker.window_seconds" type="number" min="1" class="input" />
            <p class="hint mt-1">仅统计最近 N 秒内的错误率，默认 60 秒</p>
          </div>
          <div>
            <label class="label">单渠道重试次数</label>
            <input v-model.number="breaker.channel_max_retries" type="number" min="0" class="input" />
            <p class="hint mt-1">同一渠道临时错误重试次数</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useToast } from '../composables/useToast'
import api from '../api'

const toast = useToast()
const rules = ref([])
const protocols = ref([])
const saving = ref(false)
const testModel = ref('')
const prices = ref([])
const savingPrices = ref(false)
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
const savingBreaker = ref(false)
const configFile = ref({ path: '', exists: false, content: '' })
const configDraft = ref('')
const loadingConfig = ref(false)
const savingConfig = ref(false)

const testResult = computed(() => {
  const m = testModel.value.trim()
  if (!m) return ''
  for (const r of rules.value) {
    if (!r.pattern || !r.protocol) continue
    try {
      if (new RegExp(r.pattern).test(m)) return r.protocol
    } catch {}
  }
  return ''
})

async function loadConfigFile() {
  loadingConfig.value = true
  try {
    const data = await api.get('/settings/config-file')
    configFile.value = { path: data?.path || 'config.yaml', exists: !!data?.exists, content: data?.content || '' }
    configDraft.value = data?.content || ''
  } catch (e) {
    toast.error('配置文件读取失败: ' + e.message)
  } finally {
    loadingConfig.value = false
  }
}

async function saveConfigFile() {
  savingConfig.value = true
  try {
    const data = await api.put('/settings/config-file', { content: configDraft.value })
    configFile.value = { path: data?.path || configFile.value.path || 'config.yaml', exists: true, content: data?.content ?? configDraft.value }
    configDraft.value = data?.content ?? configDraft.value
    toast.success(data?.message || '配置文件已写入，部分配置需要重启后生效')
  } catch (e) {
    toast.error('配置文件保存失败: ' + e.message)
  } finally {
    savingConfig.value = false
  }
}

async function load() {
  try {
    const [r, p, mp, cb] = await Promise.all([
      api.get('/settings/protocol-rules'),
      api.get('/protocols'),
      api.get('/settings/model-prices'),
      api.get('/settings/circuit-breaker'),
    ])
    rules.value = (r || []).map(x => ({ pattern: x.pattern || '', protocol: x.protocol || 'anthropic' }))
    protocols.value = p || []
    prices.value = (mp || []).map(x => ({ model: x.model || '', input: x.input || 0, output: x.output || 0 }))
    if (cb) breaker.value = { ...defaultBreaker, ...cb }
  } catch (e) {
    toast.error('加载失败: ' + e.message)
  }
}

async function save() {
  saving.value = true
  try {
    const clean = rules.value.filter(r => r.pattern.trim() && r.protocol)
    await api.put('/settings/protocol-rules', clean)
    rules.value = clean
    toast.success('规则已保存')
  } catch (e) {
    toast.error('保存失败: ' + e.message)
  } finally {
    saving.value = false
  }
}

async function savePrices() {
  savingPrices.value = true
  try {
    const clean = prices.value
      .filter(p => p.model.trim())
      .map(p => ({ model: p.model.trim(), input: Number(p.input) || 0, output: Number(p.output) || 0 }))
    await api.put('/settings/model-prices', clean)
    prices.value = clean
    toast.success('价格已保存')
  } catch (e) {
    toast.error('保存失败: ' + e.message)
  } finally {
    savingPrices.value = false
  }
}

function resetBreakerDefaults() {
  breaker.value = { ...defaultBreaker }
  toast.info('已填入推荐默认值，保存后生效')
}

function breakerNumberOrDefault(value, fallback, { min = 1, max = null } = {}) {
  if (value === '' || value === null || value === undefined) return fallback
  const next = Number(value)
  if (!Number.isFinite(next) || next < min) return fallback
  return max === null ? next : Math.min(next, max)
}

async function saveBreaker() {
  savingBreaker.value = true
  try {
    const clean = {
      ...breaker.value,
      failure_threshold: breakerNumberOrDefault(breaker.value.failure_threshold, defaultBreaker.failure_threshold),
      success_threshold: breakerNumberOrDefault(breaker.value.success_threshold, defaultBreaker.success_threshold),
      timeout_seconds: breakerNumberOrDefault(breaker.value.timeout_seconds, defaultBreaker.timeout_seconds),
      error_rate_threshold: breakerNumberOrDefault(breaker.value.error_rate_threshold, defaultBreaker.error_rate_threshold, { max: 1 }),
      min_requests: breakerNumberOrDefault(breaker.value.min_requests, defaultBreaker.min_requests),
      window_seconds: breakerNumberOrDefault(breaker.value.window_seconds, defaultBreaker.window_seconds),
      channel_max_retries: breakerNumberOrDefault(breaker.value.channel_max_retries, defaultBreaker.channel_max_retries, { min: 0 }),
    }
    const resp = await api.put('/settings/circuit-breaker', clean)
    breaker.value = { ...defaultBreaker, ...(resp?.config || clean) }
    toast.success('熔断器配置已保存')
  } catch (e) {
    toast.error('保存失败: ' + e.message)
  } finally {
    savingBreaker.value = false
  }
}

onMounted(() => {
  load()
  loadConfigFile()
})
</script>

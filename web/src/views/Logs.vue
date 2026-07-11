<script setup>
import { computed, getCurrentInstance, onMounted, ref } from 'vue'
import api, { copyText, takeLatest } from '../api'
import Drawer from '../components/Drawer.vue'
import PageState from '../components/PageState.vue'

const { proxy } = getCurrentInstance()
const logs = ref([])
const page = ref(1)
const pageSize = 20
const total = ref(0)
const loading = ref(true)
const error = ref('')
const selectedLog = ref(null)
const fullPayload = ref(null)
const detailLoading = ref(false)
const detailError = ref('')
const showMoreFilters = ref(false)
let loadSeq = 0

const filters = ref({
  type: '',
  model: '',
  token_name: '',
  channel_id: '',
  status: '',
  status_min: '',
  status_max: '',
  is_stream: '',
  has_full_record: '',
  request_id: '',
  upstream_request_id: '',
  range: '24h',
})

const logTypes = [
  { value: '', label: '全部类型' },
  { value: '1', label: '消费' },
  { value: '2', label: '错误' },
  { value: '3', label: '管理' },
]

const timeRanges = [
  { value: '', label: '全部时间', ms: 0 },
  { value: '1h', label: '最近 1 小时', ms: 60 * 60 * 1000 },
  { value: '24h', label: '最近 24 小时', ms: 24 * 60 * 60 * 1000 },
  { value: '7d', label: '最近 7 天', ms: 7 * 24 * 60 * 60 * 1000 },
]

const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))
const pageStart = computed(() => total.value ? (page.value - 1) * pageSize + 1 : 0)
const pageEnd = computed(() => Math.min(page.value * pageSize, total.value))
const activeFilterCount = computed(() => Object.entries(filters.value).filter(([key, value]) => {
  if (key === 'range') return value && value !== '24h'
  return String(value || '').trim()
}).length)
const moreFilterCount = computed(() => ['request_id', 'upstream_request_id', 'channel_id', 'status_min', 'status_max', 'is_stream', 'has_full_record']
  .filter((key) => String(filters.value[key] || '').trim()).length)

const fetchLogs = takeLatest((params) => api.get('/logs', { params }))

function fmt(ms) {
  if (!ms) return '—'
  const date = new Date(ms)
  const pad = (value) => String(value).padStart(2, '0')
  return `${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

function cost(micro) {
  return micro ? `$${(micro / 1_000_000).toFixed(4)}` : '—'
}

function formatBytes(value) {
  const bytes = Number(value) || 0
  if (!bytes) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`
  return `${(bytes / 1024 / 1024).toFixed(2)} MiB`
}

function parseFailoverChain(content) {
  if (!content || typeof content !== 'string') return []
  try {
    const parsed = JSON.parse(content)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

function typeName(type) {
  return { 1: '消费', 2: '错误', 3: '管理', 4: '系统' }[type] || '其他'
}

function typeChip(type) {
  if (type === 1) return 'chip-run'
  if (type === 2) return 'chip-trip'
  if (type === 3) return 'chip-blue'
  return ''
}

function statusChip(status) {
  if (status >= 500) return 'chip-trip'
  if (status >= 400) return 'chip-test'
  if (status > 0) return 'chip-run'
  return ''
}

function decisionName(decision) {
  return {
    success: '成功',
    retry_same_channel: '同渠道重试',
    switch_channel: '切换渠道',
    fatal: '终止',
  }[decision] || decision || '未记录'
}

function decisionChip(decision) {
  if (decision === 'success') return 'chip-run'
  if (decision === 'fatal') return 'chip-trip'
  if (decision === 'retry_same_channel') return 'chip-test'
  if (decision === 'switch_channel') return 'chip-blue'
  return ''
}

function logParams() {
  const params = { page: page.value, page_size: pageSize }
  for (const [key, value] of Object.entries(filters.value)) {
    if (key === 'range') continue
    const normalized = String(value || '').trim()
    if (normalized) params[key] = normalized
  }
  const range = timeRanges.find((item) => item.value === filters.value.range)
  if (range?.ms) {
    const end = Date.now()
    params.start_time = end - range.ms
    params.end_time = end
  }
  return params
}

async function load() {
  const seq = ++loadSeq
  loading.value = true
  error.value = ''
  try {
    const data = await fetchLogs(logParams())
    if (seq !== loadSeq || !data) return
    logs.value = (data.items || []).map((item) => ({
      ...item,
      _failover_chain: parseFailoverChain(item.content),
    }))
    total.value = data.total || 0
    selectedLog.value = null
    fullPayload.value = null
  } catch (err) {
    if (seq === loadSeq) error.value = err.message || '日志读取失败'
  } finally {
    if (seq === loadSeq) loading.value = false
  }
}

function applyFilters() {
  page.value = 1
  load()
}

function clearFilters() {
  filters.value = {
    type: '', model: '', token_name: '', channel_id: '', status: '', status_min: '', status_max: '',
    is_stream: '', has_full_record: '', request_id: '', upstream_request_id: '', range: '24h',
  }
  showMoreFilters.value = false
  page.value = 1
  load()
}

function applyQuick(type, status = '') {
  filters.value.type = type
  filters.value.status = status
  applyFilters()
}

async function openDetails(log) {
  selectedLog.value = log
  fullPayload.value = null
  detailError.value = ''
  if (!log.has_full_record) return
  detailLoading.value = true
  try {
    const data = await api.get(`/logs/${log.id}`)
    selectedLog.value = { ...log, ...(data?.log || {}) }
    fullPayload.value = data?.payload || null
  } catch (err) {
    detailError.value = err.message || '完整调用内容读取失败'
  } finally {
    detailLoading.value = false
  }
}

function prettyPayload(value) {
  if (!value) return ''
  try {
    const parsed = typeof value === 'string' ? JSON.parse(value) : value
    if (parsed && typeof parsed.body === 'string') {
      try { parsed.body = JSON.parse(parsed.body) } catch { /* 保留原始文本或 SSE */ }
    }
    return JSON.stringify(parsed, null, 2)
  } catch {
    return String(value)
  }
}

function payloadTitle(key) {
  return {
    client_request: '客户端请求',
    upstream_request: '上游请求',
    upstream_response: '上游响应',
    client_response: '客户端响应',
  }[key] || key
}

function previousPage() {
  if (page.value <= 1) return
  page.value -= 1
  load()
}

function nextPage() {
  if (page.value >= pageCount.value) return
  page.value += 1
  load()
}

function buildDiagnosticPackage(log) {
  const attempts = log._failover_chain || []
  const lines = [
    '# APIRelay 日志诊断包',
    '',
    `- 时间: ${fmt(log.created_at)}`,
    `- 日志 ID: ${log.id}`,
    `- 请求 ID: ${log.request_id || '—'}`,
    `- 上游请求 ID: ${log.upstream_request_id || '—'}`,
    `- 类型: ${typeName(log.type)}`,
    `- 状态: HTTP ${log.status || '—'}`,
    `- 分组: ${log.group || '—'}`,
    `- 令牌: ${log.token_name || '—'}`,
    `- 渠道: ${log.channel_name || (log.channel_id ? `#${log.channel_id}` : '—')}`,
    `- 协议: ${log.endpoint_type || '—'} -> ${log.api_type || '—'}`,
    `- 模型: ${log.src_model || '—'}${log.mapped_model && log.mapped_model !== log.src_model ? ` -> ${log.mapped_model}` : ''}`,
    `- 流式: ${log.is_stream ? 'yes' : 'no'}`,
    `- Tokens: prompt=${log.prompt_tokens || 0}, completion=${log.completion_tokens || 0}`,
    `- 费用: ${cost(log.quota)}`,
    `- 耗时: ${log.use_time_ms || 0}ms, 首字=${log.first_byte_ms || 0}ms`,
  ]
  if (log.error) lines.push('', '## Error', '```text', log.error, '```')
  if (attempts.length) {
    lines.push('', '## Failover Attempts')
    attempts.forEach((attempt, index) => {
      lines.push(`${index + 1}. [${decisionName(attempt.decision)}] ${attempt.channel_name || `#${attempt.channel_id || '—'}`} · HTTP ${attempt.status || '—'} · ${attempt.api_type || '—'}`)
      lines.push(`   retryable=${attempt.retryable ? 'true' : 'false'} · time=${fmt(attempt.at_ms)}`)
      if (attempt.error) lines.push(`   error: ${attempt.error_category ? `${attempt.error_category} · ` : ''}${attempt.error}`)
    })
    lines.push('', '```json', JSON.stringify(attempts, null, 2), '```')
  }
  if (fullPayload.value) {
    for (const key of ['client_request', 'upstream_request', 'upstream_response', 'client_response']) {
      if (!fullPayload.value[key]) continue
      lines.push('', `## ${payloadTitle(key)}`, '```json', prettyPayload(fullPayload.value[key]), '```')
    }
  }
  return lines.join('\n')
}

async function copyDiagnostic(log) {
  const copied = await copyText(buildDiagnosticPackage(log))
  proxy.$toast.add(copied ? '诊断包已复制' : '复制失败，请检查浏览器剪贴板权限', copied ? 'success' : 'error')
}

onMounted(load)
</script>

<template>
  <div class="min-w-0 space-y-5">
    <header class="page-header">
      <div>
        <div class="eyebrow">调用诊断中心</div>
        <h1 class="page-title">请求日志</h1>
        <p class="page-description">沿客户端、APIRelay 与上游渠道还原每次调用，集中查看路由、耗时、计费与错误。</p>
      </div>
      <div class="page-actions">
        <button class="btn" type="button" :disabled="loading" @click="load">刷新</button>
      </div>
    </header>

    <section class="sheet min-w-0">
      <div class="sheet-head">
        <div>
          <div class="dim-title">筛选日志</div>
          <div class="mt-1 text-xs text-soft">{{ activeFilterCount }} 个筛选条件已启用</div>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button class="btn btn-sm" type="button" @click="applyQuick('2')">异常</button>
          <button class="btn btn-sm" type="button" @click="applyQuick('2', '429')">429</button>
          <button class="btn btn-sm" type="button" @click="applyQuick('2', '504')">504</button>
          <button class="btn btn-sm" type="button" @click="clearFilters">清除</button>
        </div>
      </div>

      <form class="space-y-3 p-4" @submit.prevent="applyFilters">
        <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
          <label>
            <span class="field-label">类型</span>
            <select v-model="filters.type" class="input" @change="applyFilters">
              <option v-for="item in logTypes" :key="item.value" :value="item.value">{{ item.label }}</option>
            </select>
          </label>
          <label>
            <span class="field-label">时间范围</span>
            <select v-model="filters.range" class="input" @change="applyFilters">
              <option v-for="item in timeRanges" :key="item.value" :value="item.value">{{ item.label }}</option>
            </select>
          </label>
          <label>
            <span class="field-label">模型</span>
            <input v-model="filters.model" class="input input-mono" placeholder="gpt-4o" />
          </label>
          <label>
            <span class="field-label">令牌</span>
            <input v-model="filters.token_name" class="input input-mono" placeholder="token name" />
          </label>
          <label>
            <span class="field-label">状态码</span>
            <input v-model="filters.status" class="input input-mono" inputmode="numeric" placeholder="503" />
          </label>
        </div>

        <div>
          <button
            class="btn btn-sm"
            type="button"
            :aria-expanded="showMoreFilters"
            aria-controls="log-more-filters"
            @click="showMoreFilters = !showMoreFilters"
          >
            更多筛选<span v-if="moreFilterCount">（{{ moreFilterCount }}）</span>
          </button>
        </div>

        <div v-show="showMoreFilters" id="log-more-filters" class="grid gap-3 rounded-xl border border-line bg-ghost/40 p-3 sm:grid-cols-2 xl:grid-cols-4">
          <label>
            <span class="field-label">请求 ID</span>
            <input v-model="filters.request_id" class="input input-mono" placeholder="req..." />
          </label>
          <label>
            <span class="field-label">上游请求 ID</span>
            <input v-model="filters.upstream_request_id" class="input input-mono" placeholder="upstream..." />
          </label>
          <label>
            <span class="field-label">渠道 ID</span>
            <input v-model="filters.channel_id" class="input input-mono" inputmode="numeric" placeholder="42" />
          </label>
          <label>
            <span class="field-label">响应模式</span>
            <select v-model="filters.is_stream" class="input">
              <option value="">全部</option><option value="true">流式</option><option value="false">非流式</option>
            </select>
          </label>
          <label>
            <span class="field-label">完整记录</span>
            <select v-model="filters.has_full_record" class="input">
              <option value="">全部</option><option value="true">有完整内容</option><option value="false">仅摘要</option>
            </select>
          </label>
          <label>
            <span class="field-label">最低状态码</span>
            <input v-model="filters.status_min" class="input input-mono" inputmode="numeric" placeholder="400" />
          </label>
          <label>
            <span class="field-label">最高状态码</span>
            <input v-model="filters.status_max" class="input input-mono" inputmode="numeric" placeholder="599" />
          </label>
        </div>

        <div class="flex justify-end">
          <button class="btn btn-primary min-w-28" type="submit">查询</button>
        </div>
      </form>
    </section>

    <PageState :loading="loading" :error="error" @retry="load">
      <section class="sheet min-w-0 overflow-hidden">
        <div class="sheet-head">
          <span class="dim-title">日志记录</span>
          <span class="font-mono text-xs text-soft">{{ pageStart }}—{{ pageEnd }} / {{ total }}</span>
        </div>

        <div v-if="!logs.length" class="stamp-block">
          <div>当前范围没有日志</div>
          <p class="mt-2">可清除筛选，或重新读取最新记录。</p>
          <div class="mt-3 flex justify-center gap-2">
            <button class="btn" type="button" @click="clearFilters">清除筛选</button>
            <button class="btn" type="button" @click="load">重新读取</button>
          </div>
        </div>

        <div v-else>
          <div class="hidden lg:block">
            <table class="table-eng table-fixed">
              <thead>
                <tr>
                  <th class="w-[13%]">时间</th>
                  <th class="w-[9%]">类型</th>
                  <th class="w-[20%]">渠道 / 协议</th>
                  <th class="w-[16%]">模型 / 令牌</th>
                  <th class="w-[15%] text-right">Tokens / 费用</th>
                  <th class="w-[12%] text-right">耗时</th>
                  <th class="w-[9%]">状态</th>
                  <th class="w-[6%]">尝试</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="log in logs"
                  :key="log.id"
                  class="cursor-pointer focus-within:bg-canvas"
                  tabindex="0"
                  :aria-label="`查看日志 ${log.id} 详情`"
                  @click="openDetails(log)"
                  @keydown.enter.prevent="openDetails(log)"
                  @keydown.space.prevent="openDetails(log)"
                >
                  <td class="whitespace-nowrap font-mono text-xs">{{ fmt(log.created_at) }}</td>
                  <td><span class="chip" :class="typeChip(log.type)">{{ typeName(log.type) }}</span></td>
                  <td>
                    <div class="truncate text-sm">{{ log.channel_name || (log.channel_id ? `#${log.channel_id}` : '—') }}</div>
                    <div class="mt-1 truncate text-xs text-soft">{{ log.endpoint_type || '—' }}<template v-if="log.api_type && log.api_type !== log.endpoint_type"> → {{ log.api_type }}</template></div>
                  </td>
                  <td>
                    <div class="truncate font-mono text-xs">{{ log.src_model || '—' }}<template v-if="log.mapped_model && log.mapped_model !== log.src_model"> → {{ log.mapped_model }}</template></div>
                    <div class="mt-1 truncate font-mono text-xs text-soft">{{ log.token_name || '—' }}</div>
                  </td>
                  <td class="num">
                    <div>{{ log.prompt_tokens || 0 }} / {{ log.completion_tokens || 0 }}</div>
                    <div class="text-xs text-soft">{{ cost(log.quota) }}</div>
                  </td>
                  <td class="num">
                    <div>{{ log.use_time_ms || 0 }} ms</div>
                    <div class="text-xs text-soft">首字 {{ log.first_byte_ms || 0 }} ms</div>
                  </td>
                  <td>
                    <span class="chip" :class="statusChip(log.status)">HTTP {{ log.status || '—' }}</span>
                  </td>
                  <td><span class="chip" :class="log._failover_chain.length > 1 ? 'chip-test' : ''">{{ log._failover_chain.length }}</span></td>
                </tr>
              </tbody>
            </table>
          </div>

          <div class="space-y-3 p-3 lg:hidden">
            <article v-for="log in logs" :key="log.id" class="mobile-card">
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <div class="font-mono text-xs text-soft">{{ fmt(log.created_at) }}</div>
                  <div class="mt-1 truncate font-medium">{{ log.channel_name || (log.channel_id ? `#${log.channel_id}` : '未知渠道') }}</div>
                </div>
                <span class="chip shrink-0" :class="statusChip(log.status)">HTTP {{ log.status || '—' }}</span>
              </div>
              <dl class="mt-3 space-y-2">
                <div class="mobile-kv"><dt>类型</dt><dd><span class="chip" :class="typeChip(log.type)">{{ typeName(log.type) }}</span></dd></div>
                <div class="mobile-kv"><dt>协议</dt><dd class="break-all">{{ log.endpoint_type || '—' }}<template v-if="log.api_type && log.api_type !== log.endpoint_type"> → {{ log.api_type }}</template></dd></div>
                <div class="mobile-kv"><dt>模型</dt><dd class="break-all font-mono text-xs">{{ log.src_model || '—' }}<template v-if="log.mapped_model && log.mapped_model !== log.src_model"> → {{ log.mapped_model }}</template></dd></div>
                <div class="mobile-kv"><dt>令牌</dt><dd class="break-all font-mono text-xs">{{ log.token_name || '—' }}</dd></div>
                <div class="mobile-kv"><dt>Tokens / 费用</dt><dd class="font-mono text-xs">{{ log.prompt_tokens || 0 }} / {{ log.completion_tokens || 0 }} · {{ cost(log.quota) }}</dd></div>
                <div class="mobile-kv"><dt>耗时</dt><dd class="font-mono text-xs">{{ log.use_time_ms || 0 }} ms · 首字 {{ log.first_byte_ms || 0 }} ms</dd></div>
                <div class="mobile-kv"><dt>尝试</dt><dd>{{ log._failover_chain.length }} 次</dd></div>
              </dl>
              <div class="mt-4 flex flex-wrap justify-end gap-2">
                <button class="btn btn-sm" type="button" @click="copyDiagnostic(log)">复制诊断包</button>
                <button class="btn btn-primary btn-sm" type="button" @click="openDetails(log)">查看详情</button>
              </div>
            </article>
          </div>
        </div>
      </section>

      <nav class="flex flex-wrap items-center justify-between gap-3" aria-label="日志分页">
        <span class="font-mono text-xs text-soft">第 {{ page }} / {{ pageCount }} 页 · 共 {{ total }} 条</span>
        <div class="flex gap-2">
          <button class="btn" type="button" :disabled="page <= 1" @click="previousPage">上一页</button>
          <button class="btn" type="button" :disabled="page >= pageCount" @click="nextPage">下一页</button>
        </div>
      </nav>
    </PageState>

    <Drawer :open="!!selectedLog" title="调用诊断" @close="selectedLog = null; fullPayload = null">
      <div v-if="selectedLog" class="space-y-5">
        <section>
          <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
            <h3 class="text-base font-semibold">请求</h3>
            <div class="flex flex-wrap gap-2">
              <span class="chip" :class="typeChip(selectedLog.type)">{{ typeName(selectedLog.type) }}</span>
              <span class="chip" :class="statusChip(selectedLog.status)">HTTP {{ selectedLog.status || '—' }}</span>
            </div>
          </div>
          <dl class="grid gap-3 rounded-lg border border-line bg-ghost/40 p-3 sm:grid-cols-2">
            <div><dt class="field-label">时间</dt><dd class="font-mono text-xs">{{ fmt(selectedLog.created_at) }}</dd></div>
            <div><dt class="field-label">日志 ID</dt><dd class="break-all font-mono text-xs">{{ selectedLog.id || '—' }}</dd></div>
            <div><dt class="field-label">请求 ID</dt><dd class="break-all font-mono text-xs">{{ selectedLog.request_id || '—' }}</dd></div>
            <div><dt class="field-label">上游请求 ID</dt><dd class="break-all font-mono text-xs">{{ selectedLog.upstream_request_id || '—' }}</dd></div>
            <div><dt class="field-label">分组</dt><dd class="break-all font-mono text-xs">{{ selectedLog.group || '—' }}</dd></div>
            <div><dt class="field-label">流式</dt><dd>{{ selectedLog.is_stream ? '是' : '否' }}</dd></div>
            <div><dt class="field-label">渠道</dt><dd class="break-all">{{ selectedLog.channel_name || (selectedLog.channel_id ? `#${selectedLog.channel_id}` : '—') }}</dd></div>
            <div><dt class="field-label">协议</dt><dd class="break-all font-mono text-xs">{{ selectedLog.endpoint_type || '—' }} → {{ selectedLog.api_type || '—' }}</dd></div>
            <div><dt class="field-label">模型</dt><dd class="break-all font-mono text-xs">{{ selectedLog.src_model || '—' }}<template v-if="selectedLog.mapped_model && selectedLog.mapped_model !== selectedLog.src_model"> → {{ selectedLog.mapped_model }}</template></dd></div>
            <div><dt class="field-label">令牌</dt><dd class="break-all font-mono text-xs">{{ selectedLog.token_name || '—' }}</dd></div>
            <div><dt class="field-label">Tokens / 费用</dt><dd class="font-mono text-xs">{{ selectedLog.prompt_tokens || 0 }} / {{ selectedLog.completion_tokens || 0 }} · {{ cost(selectedLog.quota) }}</dd></div>
            <div><dt class="field-label">耗时</dt><dd class="font-mono text-xs">{{ selectedLog.use_time_ms || 0 }} ms · 首字 {{ selectedLog.first_byte_ms || 0 }} ms</dd></div>
          </dl>
        </section>

        <section>
          <h3 class="mb-3 text-base font-semibold">错误</h3>
          <pre v-if="selectedLog.error" class="max-h-64 overflow-auto whitespace-pre-wrap break-all rounded-lg border border-trip/30 bg-trip-wash p-3 text-xs text-trip">{{ selectedLog.error }}</pre>
          <div v-else class="rounded-lg border border-dashed border-line p-4 text-sm text-soft">该日志没有错误信息。</div>
        </section>

        <section>
          <div class="mb-3 flex items-center justify-between gap-2">
            <h3 class="text-base font-semibold">故障转移步骤</h3>
            <span class="chip">{{ selectedLog._failover_chain.length }} 次</span>
          </div>
          <ol v-if="selectedLog._failover_chain.length" class="space-y-3">
            <li v-for="(attempt, index) in selectedLog._failover_chain" :key="`${selectedLog.id}-${index}`" class="rounded-lg border border-line p-3">
              <div class="flex flex-wrap items-center justify-between gap-2">
                <span class="font-medium">步骤 {{ index + 1 }} · {{ attempt.channel_name || (attempt.channel_id ? `#${attempt.channel_id}` : '未知渠道') }}</span>
                <span class="chip" :class="decisionChip(attempt.decision)">{{ decisionName(attempt.decision) }}</span>
              </div>
              <div class="mt-2 flex flex-wrap gap-2">
                <span class="chip" :class="statusChip(attempt.status)">HTTP {{ attempt.status || '—' }}</span>
                <span class="chip chip-blue">{{ attempt.api_type || '—' }}</span>
                <span class="chip" :class="attempt.retryable ? 'chip-test' : ''">{{ attempt.retryable ? '可重试' : '不可重试' }}</span>
              </div>
              <dl class="mt-3 grid grid-cols-[88px_minmax(0,1fr)] gap-x-3 gap-y-2 text-xs">
                <dt class="text-soft">渠道 ID</dt><dd class="break-all font-mono">{{ attempt.channel_id || '—' }}</dd>
                <dt class="text-soft">时间</dt><dd class="font-mono">{{ fmt(attempt.at_ms) }}</dd>
                <dt class="text-soft">迭代 / 切换</dt><dd class="font-mono">{{ attempt.iter ?? '—' }} / {{ attempt.switches ?? '—' }}</dd>
                <dt class="text-soft">模型</dt><dd class="break-all font-mono">{{ attempt.origin_model || '—' }}<template v-if="attempt.upstream_model && attempt.upstream_model !== attempt.origin_model"> → {{ attempt.upstream_model }}</template></dd>
                <dt class="text-soft">错误</dt><dd class="break-all" :class="attempt.error ? 'text-trip' : 'text-soft'">{{ attempt.error_category ? `${attempt.error_category} · ` : '' }}{{ attempt.error || '—' }}</dd>
              </dl>
            </li>
          </ol>
          <div v-else class="rounded-lg border border-dashed border-line p-4 text-sm text-soft">该日志没有故障转移步骤。</div>
        </section>

        <section>
          <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
            <div>
              <h3 class="text-base font-semibold">完整调用内容</h3>
              <p class="mt-1 text-xs text-soft">客户端 → APIRelay → 上游 → 客户端</p>
            </div>
            <span class="chip" :class="selectedLog.has_full_record ? 'chip-blue' : ''">{{ selectedLog.has_full_record ? `gzip ${formatBytes(selectedLog.payload_original_size)} → ${formatBytes(selectedLog.payload_compressed_size)}` : '仅摘要' }}</span>
          </div>

          <div v-if="detailLoading" class="rounded-xl border border-line bg-ghost/40 p-5 text-center text-sm text-soft">正在解压完整调用内容…</div>
          <div v-else-if="detailError" class="rounded-xl border border-trip/25 bg-trip-wash p-4 text-sm text-trip">{{ detailError }}</div>
          <div v-else-if="fullPayload" class="route-timeline space-y-5">
            <article v-for="key in ['client_request', 'upstream_request', 'upstream_response', 'client_response']" :key="key">
              <div class="mb-2 flex items-center justify-between gap-2">
                <h4 class="font-cond text-sm font-semibold text-ink">{{ payloadTitle(key) }}</h4>
                <button v-if="fullPayload[key]" class="btn btn-sm" type="button" @click="copyText(prettyPayload(fullPayload[key])).then(ok => proxy.$toast.add(ok ? '内容已复制' : '复制失败', ok ? 'success' : 'error'))">复制</button>
              </div>
              <pre v-if="fullPayload[key]" class="log-code">{{ prettyPayload(fullPayload[key]) }}</pre>
              <div v-else class="rounded-xl border border-dashed border-line px-4 py-3 text-xs text-soft">此阶段未配置记录或没有可记录内容。</div>
            </article>
          </div>
          <div v-else class="rounded-xl border border-dashed border-line p-4 text-sm text-soft">
            该日志仅保留路由、计费、耗时和错误摘要。
          </div>
        </section>
      </div>
      <template #footer>
        <div class="flex justify-end">
          <button v-if="selectedLog" class="btn btn-primary" type="button" @click="copyDiagnostic(selectedLog)">复制诊断包</button>
        </div>
      </template>
    </Drawer>
  </div>
</template>

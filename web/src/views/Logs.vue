<script setup>
import { computed, getCurrentInstance, onMounted, ref } from 'vue'
import api, { copyText, takeLatest, fmtTime as fmt, cost } from '../api'
import PageState from '../components/PageState.vue'
import ConsoleIcon from '../components/ConsoleIcon.vue'
import LogFilterPanel from '../components/LogFilterPanel.vue'
import LogDetailDrawer from '../components/LogDetailDrawer.vue'

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
const moreFilterCount = computed(() => ['token_name', 'upstream_request_id', 'status_min', 'status_max', 'is_stream', 'has_full_record']
  .filter((key) => String(filters.value[key] || '').trim()).length)

const fetchLogs = takeLatest((params) => api.get('/logs', { params }))

// fmt（=api.js fmtTime 别名）与 cost 复用 api.js 公共实现。

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

function isModelMapped(log) {
  return Boolean(log?.mapped_model && log.mapped_model !== log.src_model)
}

function isFailed(log) {
  return log?.type === 2 || Number(log?.status) >= 400
}

function requestPath(log) {
  const capturedPath = log?.request_path || log?.client_path || log?.path
  if (capturedPath) return capturedPath
  return {
    openai: '/v1/chat/completions',
    anthropic: '/v1/messages',
    responses: '/v1/responses',
  }[log?.endpoint_type] || log?.endpoint_type || '—'
}

function statusTone(log) {
  if (Number(log?.status) >= 500 || log?.type === 2) return 'bg-trip'
  if (Number(log?.status) >= 400) return 'bg-test'
  if (Number(log?.status) > 0) return 'bg-run'
  return 'bg-faint'
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
    `- 客户端模型: ${log.src_model || '—'}`,
    ...(isModelMapped(log) ? [`- 实际请求模型: ${log.mapped_model}`] : []),
    `- 流式: ${log.is_stream ? 'yes' : 'no'}`,
    `- Tokens: prompt=${log.prompt_tokens || 0}, completion=${log.completion_tokens || 0}${log.usage_estimated ? ' (estimated)' : ''}`,
    `- Cache/Reasoning: write=${log.cache_creation_input_tokens || 0}, read=${log.cache_read_input_tokens || 0}, reasoning=${log.reasoning_tokens || 0}`,
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

async function copyPayload(value) {
  const copied = await copyText(prettyPayload(value))
  proxy.$toast.add(copied ? '内容已复制' : '复制失败', copied ? 'success' : 'error')
}

onMounted(load)
</script>

<template>
  <div class="page-workbench logs-page min-w-0 space-y-3">
    <header class="flex flex-col gap-3 border-b border-line pb-3 sm:flex-row sm:items-end sm:justify-between">
      <div class="min-w-0">
        <div class="flex items-center gap-2">
          <ConsoleIcon name="logs" class="h-5 w-5 text-blue-grid" />
          <h1 class="text-xl font-semibold tracking-tight text-ink">日志诊断</h1>
        </div>
        <p class="mt-1 text-xs text-soft">按请求定位路由、故障转移、延迟与计费。</p>
      </div>
      <button class="btn btn-sm" type="button" :disabled="loading" @click="load">
        <ConsoleIcon name="arrowPath" class="h-4 w-4" :class="{ 'animate-spin': loading }" />
        {{ loading ? '刷新中…' : '刷新' }}
      </button>
    </header>

    <LogFilterPanel
      v-model:expanded="showMoreFilters"
      :filters="filters"
      :log-types="logTypes"
      :time-ranges="timeRanges"
      :active-count="activeFilterCount"
      :more-count="moreFilterCount"
      @apply="applyFilters"
      @clear="clearFilters"
      @quick="applyQuick"
    />

    <PageState :loading="loading" :error="error" @retry="load">
      <section class="sheet flex min-h-[30rem] min-w-0 flex-col overflow-hidden">
        <div class="sheet-head !items-center !px-3 !py-2.5 sm:!px-4">
          <div class="flex min-w-0 items-center gap-2">
            <span class="dim-title">请求记录</span>
            <span v-if="activeFilterCount" class="font-mono text-[10px] text-blue-grid">{{ activeFilterCount }} FILTERS</span>
          </div>
          <span class="font-mono text-xs text-soft">{{ pageStart }}—{{ pageEnd }} / {{ total }}</span>
        </div>

        <div v-if="!logs.length" class="flex flex-1 items-center justify-center p-5">
          <div class="stamp-block !my-0">
            <div>当前范围没有日志</div>
            <p class="mt-2">可清除筛选，或重新读取最新记录。</p>
            <div class="mt-3 flex justify-center gap-2">
              <button class="btn btn-sm" type="button" @click="clearFilters">清除筛选</button>
              <button class="btn btn-sm" type="button" @click="load">重新读取</button>
            </div>
          </div>
        </div>

        <div v-else class="min-h-0 flex-1 overflow-auto">
          <div class="hidden min-w-[1120px] lg:block">
            <table class="table-eng table-fixed" aria-label="请求日志数据网格">
              <thead>
                <tr>
                  <th class="w-[13%]">时间</th>
                  <th class="w-[8%]">状态</th>
                  <th class="w-[17%]">请求路径</th>
                  <th class="w-[13%]">渠道</th>
                  <th class="w-[15%]">模型</th>
                  <th class="w-[10%] text-right">延迟</th>
                  <th class="w-[9%] text-right">Tokens</th>
                  <th class="w-[9%] text-right">费用</th>
                  <th class="w-[6%] text-right">尝试</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="log in logs"
                  :key="log.id"
                  class="cursor-pointer outline-none focus:bg-canvas"
                  :class="{ 'bg-trip-wash/30': isFailed(log) }"
                  tabindex="0"
                  :aria-label="`查看日志 ${log.id} 详情`"
                  @click="openDetails(log)"
                  @keydown.enter.prevent="openDetails(log)"
                  @keydown.space.prevent="openDetails(log)"
                >
                  <td class="whitespace-nowrap border-l-2 font-mono text-[11px]" :class="isFailed(log) ? '!border-l-trip' : '!border-l-transparent'">{{ fmt(log.created_at) }}</td>
                  <td>
                    <div class="flex items-center gap-2 font-mono text-[11px] font-medium text-ink">
                      <span class="h-2 w-2 shrink-0 rounded-full" :class="statusTone(log)" />
                      {{ log.status || '—' }}
                    </div>
                    <div class="mt-1 text-[10px] text-faint">{{ typeName(log.type) }}</div>
                  </td>
                  <td>
                    <div class="truncate font-mono text-[11px] font-medium text-ink" :title="requestPath(log)">{{ requestPath(log) }}</div>
                    <div class="mt-1 truncate text-[10px] text-faint">{{ log.request_id || `log:${log.id}` }}</div>
                  </td>
                  <td>
                    <div class="truncate text-xs text-ink">{{ log.channel_name || (log.channel_id ? `#${log.channel_id}` : '—') }}</div>
                    <div class="mt-1 truncate font-mono text-[10px] text-faint">{{ log.endpoint_type || '—' }}<template v-if="log.api_type && log.api_type !== log.endpoint_type"> → {{ log.api_type }}</template></div>
                  </td>
                  <td>
                    <div class="truncate font-mono text-[11px] font-medium text-ink">{{ log.src_model || '—' }}</div>
                    <div v-if="isModelMapped(log)" class="mt-1 truncate font-mono text-[10px] text-faint" :title="`实际请求模型：${log.mapped_model}`">→ {{ log.mapped_model }}</div>
                  </td>
                  <td class="num">
                    <div class="text-ink">{{ log.use_time_ms || 0 }} ms</div>
                    <div class="mt-1 text-[10px] text-faint">TTFB {{ log.first_byte_ms || 0 }} ms</div>
                  </td>
                  <td class="num">
                    <div class="text-ink">{{ log.total_tokens || ((log.prompt_tokens || 0) + (log.completion_tokens || 0)) }}</div>
                    <div class="mt-1 text-[10px] text-faint">{{ log.prompt_tokens || 0 }} + {{ log.completion_tokens || 0 }}<template v-if="log.usage_estimated"> · 估算</template></div>
                  </td>
                  <td class="num text-ink">{{ cost(log.quota) }}</td>
                  <td class="num text-ink">{{ log._failover_chain.length }}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <div class="divide-y divide-line lg:hidden">
            <article
              v-for="log in logs"
              :key="log.id"
              class="cursor-pointer border-l-2 px-3 py-3 outline-none focus:bg-canvas"
              :class="isFailed(log) ? 'border-l-trip bg-trip-wash/25' : 'border-l-transparent'"
              tabindex="0"
              @click="openDetails(log)"
              @keydown.enter.prevent="openDetails(log)"
              @keydown.space.prevent="openDetails(log)"
            >
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <div class="truncate font-mono text-xs font-medium text-ink">{{ requestPath(log) }}</div>
                  <div class="mt-1 truncate text-[11px] text-soft">{{ log.channel_name || (log.channel_id ? `#${log.channel_id}` : '未知渠道') }} · {{ log.src_model || '未知模型' }}</div>
                </div>
                <div class="flex shrink-0 items-center gap-1.5 font-mono text-xs font-semibold text-ink">
                  <span class="h-2 w-2 rounded-full" :class="statusTone(log)" />{{ log.status || '—' }}
                </div>
              </div>
              <div class="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 font-mono text-[10px] text-faint">
                <span>{{ fmt(log.created_at) }}</span>
                <span>{{ log.use_time_ms || 0 }} ms</span>
                <span>{{ log.total_tokens || ((log.prompt_tokens || 0) + (log.completion_tokens || 0)) }} tok</span>
                <span>{{ cost(log.quota) }}</span>
                <span>{{ log._failover_chain.length }} 次尝试</span>
              </div>
              <div class="mt-2 flex items-center justify-between gap-2">
                <span class="truncate font-mono text-[10px] text-faint">{{ log.request_id || `log:${log.id}` }}</span>
                <button class="btn btn-ghost btn-sm shrink-0" type="button" @click.stop="copyDiagnostic(log)">复制诊断</button>
              </div>
            </article>
          </div>
        </div>

        <nav class="sticky bottom-0 flex flex-wrap items-center justify-between gap-3 border-t border-line bg-surface px-3 py-2.5 sm:px-4" aria-label="日志分页">
          <span class="font-mono text-[11px] text-soft">第 {{ page }} / {{ pageCount }} 页 · 共 {{ total }} 条</span>
          <div class="flex gap-2">
            <button class="btn btn-sm" type="button" :disabled="page <= 1" @click="previousPage">
              <ConsoleIcon name="chevronLeft" class="h-4 w-4" />上一页
            </button>
            <button class="btn btn-sm" type="button" :disabled="page >= pageCount" @click="nextPage">
              下一页<ConsoleIcon name="chevronRight" class="h-4 w-4" />
            </button>
          </div>
        </nav>
      </section>
    </PageState>

    <LogDetailDrawer
      :log="selectedLog"
      :payload="fullPayload"
      :loading="detailLoading"
      :error="detailError"
      :helpers="{ typeChip, typeName, statusChip, isModelMapped, decisionChip, decisionName, formatBytes, payloadTitle, prettyPayload, fmt, cost, requestPath, isFailed, statusTone }"
      @close="selectedLog = null; fullPayload = null"
      @copy-diagnostic="copyDiagnostic"
      @copy-payload="copyPayload"
    />
  </div>
</template>

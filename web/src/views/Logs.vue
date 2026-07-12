<script setup>
import { computed, getCurrentInstance, onMounted, ref } from 'vue'
import api, { copyText, takeLatest, fmtTime as fmt, cost } from '../api'
import PageState from '../components/PageState.vue'
import PageHeader from '../components/PageHeader.vue'
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
const moreFilterCount = computed(() => ['request_id', 'upstream_request_id', 'channel_id', 'status_min', 'status_max', 'is_stream', 'has_full_record']
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
  <div class="page-workbench logs-page min-w-0 space-y-5">
    <PageHeader eyebrow="调用诊断中心" title="请求日志" description="沿客户端、APIRelay 与上游渠道还原每次调用，集中查看路由、耗时、计费与错误。">
      <template #actions>
        <button class="btn" type="button" :disabled="loading" @click="load">{{ loading ? '刷新中…' : '刷新' }}</button>
      </template>
    </PageHeader>

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
                  <th class="w-[16%]">客户端模型 / 令牌</th>
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
                    <div class="truncate font-mono text-xs font-medium">{{ log.src_model || '—' }}</div>
                    <div v-if="isModelMapped(log)" class="mt-1 truncate font-mono text-[11px] text-soft" :title="`实际请求模型：${log.mapped_model}`">↳ 实际 {{ log.mapped_model }}</div>
                    <div class="mt-1 truncate font-mono text-xs text-soft">{{ log.token_name || '—' }}</div>
                  </td>
                  <td class="num">
                    <div>{{ log.prompt_tokens || 0 }} / {{ log.completion_tokens || 0 }} <span v-if="log.usage_estimated" class="chip chip-test ml-1">估算</span></div>
                    <div v-if="log.cache_creation_input_tokens || log.cache_read_input_tokens || log.reasoning_tokens" class="text-[10px] text-soft">写 {{ log.cache_creation_input_tokens || 0 }} · 读 {{ log.cache_read_input_tokens || 0 }} · 推理 {{ log.reasoning_tokens || 0 }}</div>
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
                <div class="mobile-kv"><dt>客户端模型</dt><dd class="break-all font-mono text-xs font-medium">{{ log.src_model || '—' }}</dd></div>
                <div v-if="isModelMapped(log)" class="mobile-kv"><dt>实际请求模型</dt><dd class="break-all font-mono text-xs text-soft">{{ log.mapped_model }}</dd></div>
                <div class="mobile-kv"><dt>令牌</dt><dd class="break-all font-mono text-xs">{{ log.token_name || '—' }}</dd></div>
                <div class="mobile-kv"><dt>Tokens / 费用</dt><dd class="font-mono text-xs">{{ log.prompt_tokens || 0 }} / {{ log.completion_tokens || 0 }} · {{ cost(log.quota) }}<span v-if="log.usage_estimated" class="ml-1 text-test">估算</span></dd></div>
                <div v-if="log.cache_creation_input_tokens || log.cache_read_input_tokens || log.reasoning_tokens" class="mobile-kv"><dt>缓存 / 推理</dt><dd class="font-mono text-xs">写 {{ log.cache_creation_input_tokens || 0 }} · 读 {{ log.cache_read_input_tokens || 0 }} · 推理 {{ log.reasoning_tokens || 0 }}</dd></div>
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

    <LogDetailDrawer
      :log="selectedLog"
      :payload="fullPayload"
      :loading="detailLoading"
      :error="detailError"
      :helpers="{ typeChip, typeName, statusChip, isModelMapped, decisionChip, decisionName, formatBytes, payloadTitle, prettyPayload, fmt, cost }"
      @close="selectedLog = null; fullPayload = null"
      @copy-diagnostic="copyDiagnostic"
      @copy-payload="copyPayload"
    />
  </div>
</template>

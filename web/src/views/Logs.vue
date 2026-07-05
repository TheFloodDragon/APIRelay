<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'
import ProtocolTag from '../components/ProtocolTag.vue'
import SignalDot from '../components/SignalDot.vue'
import { useToast } from '../composables/useToast'

const toast = useToast()
const logs = ref([])
const page = ref(1)
const pageSize = 20
const total = ref(0)
const loading = ref(false)
const expandedId = ref(null) // 行内展开的日志 id
const filters = ref({
  type: '',
  model: '',
  token_name: '',
  channel_id: '',
  status: '',
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

function fmt(ms) {
  if (!ms) return '-'
  const d = new Date(ms)
  const p = (n) => String(n).padStart(2, '0')
  return `${p(d.getMonth() + 1)}/${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

const cost = (micro) => micro ? '$' + (micro / 1_000_000).toFixed(4) : '-'

function parseFailoverChain(content) {
  if (!content || typeof content !== 'string') return []
  try {
    const parsed = JSON.parse(content)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

function decisionName(d) {
  return {
    success: '命中',
    retry_same_channel: '同频重试',
    switch_channel: '切换航路',
    fatal: '终止',
  }[d] || d || '-'
}
function decisionBadge(d) {
  if (d === 'success') return 'badge-online'
  if (d === 'retry_same_channel') return 'badge-warn'
  if (d === 'switch_channel') return 'badge-signal'
  if (d === 'fatal') return 'badge-down'
  return 'badge-neutral'
}
function attemptStatusClass(status) {
  if (!status) return 'text-t3'
  if (status >= 500) return 'text-danger'
  if (status >= 400) return 'text-warning'
  return 'text-success'
}
function attemptTime(ms) {
  if (!ms) return '-'
  return fmt(ms)
}

function typeName(t) {
  return { 1: '消费', 2: '错误', 3: '管理', 4: '系统' }[t] || '其他'
}
function typeBadge(t) {
  if (t === 1) return 'badge-online'
  if (t === 2) return 'badge-down'
  return 'badge-neutral'
}
function statusState(s) {
  if (!s) return 'idle'
  if (s >= 500) return 'down'
  if (s >= 400) return 'warn'
  return 'online'
}

function toggle(l) {
  expandedId.value = expandedId.value === l.id ? null : l.id
}

function logParams() {
  const params = { page: page.value, page_size: pageSize }
  for (const [key, value] of Object.entries(filters.value)) {
    if (key === 'range') continue
    const v = String(value || '').trim()
    if (v) params[key] = v
  }
  const range = timeRanges.find((item) => item.value === filters.value.range)
  if (range?.ms) {
    const end = Date.now()
    params.start_time = end - range.ms
    params.end_time = end
  }
  return params
}
function applyFilters() {
  page.value = 1
  load()
}
function clearFilters() {
  filters.value = { type: '', model: '', token_name: '', channel_id: '', status: '', request_id: '', upstream_request_id: '', range: '24h' }
  page.value = 1
  load()
}
function quickErrors() {
  filters.value.type = '2'
  filters.value.status = ''
  applyFilters()
}
function quickRateLimit() {
  filters.value.type = '2'
  filters.value.status = '429'
  applyFilters()
}
function quickTimeouts() {
  filters.value.type = '2'
  filters.value.status = '504'
  applyFilters()
}
function filterSummary() {
  const parts = []
  const type = logTypes.find((item) => item.value === filters.value.type)
  const range = timeRanges.find((item) => item.value === filters.value.range)
  if (type?.value) parts.push(type.label)
  if (range?.value) parts.push(range.label)
  if (filters.value.model) parts.push(`模型 ${filters.value.model}`)
  if (filters.value.token_name) parts.push(`令牌 ${filters.value.token_name}`)
  if (filters.value.status) parts.push(`HTTP ${filters.value.status}`)
  if (filters.value.channel_id) parts.push(`节点 #${filters.value.channel_id}`)
  if (filters.value.request_id) parts.push(`请求 ${filters.value.request_id}`)
  if (filters.value.upstream_request_id) parts.push(`上游 ${filters.value.upstream_request_id}`)
  return parts.length ? parts.join(' · ') : '全部信号'
}
function buildDiagnosticPackage(l) {
  const chain = l._failover_chain || []
  const lines = [
    '# APIRelay 调用诊断包',
    '',
    `- 时间: ${fmt(l.created_at)}`,
    `- 日志 ID: ${l.id}`,
    `- 请求 ID: ${l.request_id || '-'}`,
    `- 上游请求 ID: ${l.upstream_request_id || '-'}`,
    `- 类型: ${typeName(l.type)}`,
    `- 状态: HTTP ${l.status || '-'}`,
    `- 分组: ${l.group || '-'}`,
    `- 令牌: ${l.token_name || '-'}`,
    `- 节点: ${l.channel_name || (l.channel_id ? '#' + l.channel_id : '-')}`,
    `- 协议: ${l.endpoint_type || '-'} -> ${l.api_type || '-'}`,
    `- 模型: ${l.src_model || '-'}${l.mapped_model && l.mapped_model !== l.src_model ? ' -> ' + l.mapped_model : ''}`,
    `- 流式: ${l.is_stream ? 'yes' : 'no'}`,
    `- Tokens: prompt=${l.prompt_tokens || 0}, completion=${l.completion_tokens || 0}`,
    `- 费用: ${cost(l.quota)}`,
    `- 耗时: ${l.use_time_ms || 0}ms, 首字=${l.first_byte_ms || 0}ms`,
  ]
  if (l.error) {
    lines.push('', '## Error', '```text', l.error, '```')
  }
  if (chain.length) {
    lines.push('', '## Failover Track')
    chain.forEach((a, idx) => {
      lines.push(`${idx + 1}. [${decisionName(a.decision)}] ${a.channel_name || '#' + a.channel_id} · HTTP ${a.status || '-'} · ${a.api_type || '-'}`)
      lines.push(`   model: ${a.origin_model || '-'}${a.upstream_model && a.upstream_model !== a.origin_model ? ' -> ' + a.upstream_model : ''}`)
      if (a.error) lines.push(`   error: ${a.error_category ? a.error_category + ' · ' : ''}${a.error}`)
    })
    lines.push('', '```json', JSON.stringify(chain, null, 2), '```')
  }
  return lines.join('\n')
}
async function copyDiagnostic(l) {
  const text = buildDiagnosticPackage(l)
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
    } else {
      const ta = document.createElement('textarea')
      ta.value = text
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
    }
    toast.success('诊断包已复制')
  } catch (e) {
    toast.error('复制失败: ' + (e?.message || '浏览器拒绝访问剪贴板'))
  }
}

async function load() {
  loading.value = true
  try {
    const data = await api.get('/logs', { params: logParams() })
    logs.value = (data.items || []).map((item) => ({
      ...item,
      _failover_chain: parseFailoverChain(item.content),
    }))
    total.value = data.total || 0
    expandedId.value = null
  } finally {
    loading.value = false
  }
}

function prev() { if (page.value > 1) { page.value--; load() } }
function next() { if (page.value * pageSize < total.value) { page.value++; load() } }

onMounted(load)
</script>

<template>
  <div>
    <div class="flex items-center justify-between mb-5">
      <div>
        <h2 class="page-title">信号流水</h2>
        <p class="page-subtitle">API 调用历史 · 点击行展开详情</p>
      </div>
      <button @click="load" class="btn-secondary">刷新</button>
    </div>

    <div class="panel p-4 mb-4">
      <div class="flex items-center justify-between gap-3 mb-3">
        <div>
          <span class="tick">FILTER RADAR</span>
          <p class="text-2xs text-t3 mt-0.5">缩小信号范围，快速定位异常节点与模型航迹</p>
        </div>
        <div class="flex flex-wrap items-center justify-end gap-2">
          <button @click="quickErrors" class="btn-ghost btn-sm">异常</button>
          <button @click="quickRateLimit" class="btn-ghost btn-sm">限流 429</button>
          <button @click="quickTimeouts" class="btn-ghost btn-sm">超时 504</button>
          <button @click="clearFilters" class="btn-ghost btn-sm">清除</button>
        </div>
      </div>
      <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 2xl:grid-cols-8 gap-3">
        <label>
          <span class="label !mb-1">类型</span>
          <select v-model="filters.type" class="input" @change="applyFilters">
            <option v-for="t in logTypes" :key="t.value" :value="t.value">{{ t.label }}</option>
          </select>
        </label>
        <label>
          <span class="label !mb-1">时间窗</span>
          <select v-model="filters.range" class="input" @change="applyFilters">
            <option v-for="r in timeRanges" :key="r.value" :value="r.value">{{ r.label }}</option>
          </select>
        </label>
        <label>
          <span class="label !mb-1">模型</span>
          <input v-model="filters.model" class="input font-mono" placeholder="gpt-4o" @keyup.enter="applyFilters" />
        </label>
        <label>
          <span class="label !mb-1">令牌</span>
          <input v-model="filters.token_name" class="input font-mono" placeholder="token name" @keyup.enter="applyFilters" />
        </label>
        <label>
          <span class="label !mb-1">状态码</span>
          <input v-model="filters.status" class="input font-mono" placeholder="503" inputmode="numeric" @keyup.enter="applyFilters" />
        </label>
        <label>
          <span class="label !mb-1">请求 ID</span>
          <input v-model="filters.request_id" class="input font-mono" placeholder="req..." @keyup.enter="applyFilters" />
        </label>
        <label>
          <span class="label !mb-1">上游 ID</span>
          <input v-model="filters.upstream_request_id" class="input font-mono" placeholder="upstream..." @keyup.enter="applyFilters" />
        </label>
        <label>
          <span class="label !mb-1">节点 ID</span>
          <input v-model="filters.channel_id" class="input font-mono" placeholder="42" inputmode="numeric" @keyup.enter="applyFilters" />
        </label>
        <label class="flex items-end">
          <button @click="applyFilters" class="btn-secondary w-full">扫描</button>
        </label>
      </div>
      <div class="mt-3 flex flex-wrap items-center justify-between gap-2 border-t border-border pt-3 font-mono text-2xs text-t3">
        <span>LOCKED ON · {{ filterSummary() }}</span>
        <span>{{ total }} 条匹配 · PAGE {{ page }}</span>
      </div>
    </div>

    <div class="panel overflow-hidden">
      <div class="overflow-x-auto">
        <table class="dtable">
          <thead>
            <tr>
              <th class="w-32">时间</th>
              <th>类型</th>
              <th>节点</th>
              <th>协议</th>
              <th>模型</th>
              <th class="text-center">流</th>
              <th class="text-right">Tokens</th>
              <th class="text-right">费用</th>
              <th class="text-right">耗时</th>
              <th class="text-right">首字</th>
              <th class="text-center">状态</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="l in logs" :key="l.id">
              <tr class="cursor-pointer" @click="toggle(l)">
                <td class="whitespace-nowrap font-mono text-2xs text-t2">{{ fmt(l.created_at) }}</td>
                <td><span class="badge" :class="typeBadge(l.type)">{{ typeName(l.type) }}</span></td>
                <td class="text-t2 text-xs truncate max-w-[120px]">{{ l.channel_name || (l.channel_id ? '#'+l.channel_id : '-') }}</td>
                <td>
                  <div class="flex gap-1">
                    <ProtocolTag :protocol="l.endpoint_type" />
                    <ProtocolTag v-if="l.api_type && l.api_type !== l.endpoint_type" :protocol="l.api_type" />
                  </div>
                </td>
                <td class="whitespace-nowrap font-mono text-2xs text-t1">
                  {{ l.src_model }}
                  <span v-if="l.mapped_model && l.mapped_model !== l.src_model" class="text-t3">→ {{ l.mapped_model }}</span>
                </td>
                <td class="text-center">
                  <span v-if="l.is_stream" class="badge badge-signal !px-1.5">流</span>
                  <span v-else class="text-t3">—</span>
                </td>
                <td class="text-right whitespace-nowrap font-mono text-2xs text-t2 tabular-nums">{{ l.prompt_tokens }}<span class="text-t3">/</span>{{ l.completion_tokens }}</td>
                <td class="text-right whitespace-nowrap font-mono text-2xs text-t1 tabular-nums">{{ cost(l.quota) }}</td>
                <td class="text-right font-mono text-2xs text-t2 tabular-nums">{{ l.use_time_ms }}ms</td>
                <td class="text-right font-mono text-2xs text-t3 tabular-nums">{{ l.first_byte_ms ? l.first_byte_ms + 'ms' : '-' }}</td>
                <td class="text-center">
                  <span class="font-mono text-2xs tabular-nums" :class="l.status >= 400 ? 'text-danger' : 'text-t2'">{{ l.status }}</span>
                </td>
              </tr>
              <!-- 行内展开详情 -->
              <tr v-if="expandedId === l.id">
                <td colspan="11" class="!p-0 border-b border-line">
                  <div class="bg-panel-2 px-4 py-3 animate-fade-in">
                    <div class="flex items-center justify-between gap-3 mb-3">
                      <span class="tick">DIAGNOSTIC PAYLOAD</span>
                      <button @click.stop="copyDiagnostic(l)" class="btn-secondary btn-sm">复制诊断包</button>
                    </div>
                    <div class="grid grid-cols-2 md:grid-cols-4 gap-x-6 gap-y-2 text-xs">
                      <div><span class="tick">GROUP</span><div class="font-mono text-t1 mt-0.5">{{ l.group || '-' }}</div></div>
                      <div><span class="tick">TOKEN</span><div class="font-mono text-t1 mt-0.5">{{ l.token_name || '-' }}</div></div>
                      <div><span class="tick">STATUS</span><div class="font-mono mt-0.5 flex items-center gap-1.5"><SignalDot :status="statusState(l.status)" :size="6" :pulse="false" />{{ l.status }}</div></div>
                      <div><span class="tick">TOTAL TK</span><div class="font-mono text-t1 mt-0.5">{{ (l.prompt_tokens || 0) + (l.completion_tokens || 0) }}</div></div>
                      <div v-if="l.request_id" class="col-span-2 md:col-span-2"><span class="tick">REQUEST ID</span><div class="font-mono text-t2 mt-0.5 break-all">{{ l.request_id }}</div></div>
                      <div v-if="l.upstream_request_id" class="col-span-2 md:col-span-2"><span class="tick">UPSTREAM ID</span><div class="font-mono text-t2 mt-0.5 break-all">{{ l.upstream_request_id }}</div></div>
                    </div>
                    <div v-if="l.error" class="mt-3">
                      <span class="tick">ERROR</span>
                      <pre class="mt-1 p-2.5 rounded-md border text-2xs whitespace-pre-wrap break-all max-h-56 overflow-auto font-mono text-danger border-danger/30 bg-danger/10">{{ l.error }}</pre>
                    </div>

                    <div v-if="l._failover_chain?.length" class="mt-4">
                      <div class="flex items-center justify-between gap-3 mb-2">
                        <div>
                          <span class="tick">FAILOVER TRACK</span>
                          <p class="text-2xs text-t3 mt-0.5">按实际尝试顺序记录渠道、状态与调度决策</p>
                        </div>
                        <span class="badge badge-neutral font-mono !text-2xs">{{ l._failover_chain.length }} HOPS</span>
                      </div>

                      <div class="rounded-lg border border-primary/20 bg-surface/70 overflow-hidden">
                        <div v-for="(a, idx) in l._failover_chain" :key="idx" class="relative grid grid-cols-[2rem_1fr] gap-3 px-3 py-3 border-b border-border/70 last:border-b-0">
                          <div class="relative flex justify-center">
                            <div v-if="idx < l._failover_chain.length - 1" class="absolute top-7 bottom-[-0.75rem] w-px bg-primary/25"></div>
                            <div class="relative z-10 h-7 w-7 rounded-full border flex items-center justify-center font-mono text-2xs"
                                 :class="a.decision === 'success' ? 'border-success/50 bg-success/15 text-success' : a.decision === 'fatal' ? 'border-danger/50 bg-danger/15 text-danger' : 'border-primary/40 bg-primary/10 text-primary'">
                              {{ idx + 1 }}
                            </div>
                          </div>

                          <div class="min-w-0">
                            <div class="flex flex-wrap items-center gap-2">
                              <span class="font-mono text-xs text-t1 truncate max-w-[180px]">{{ a.channel_name || ('#' + a.channel_id) }}</span>
                              <span class="badge !px-2 !py-0.5" :class="decisionBadge(a.decision)">{{ decisionName(a.decision) }}</span>
                              <span class="font-mono text-2xs tabular-nums" :class="attemptStatusClass(a.status)">HTTP {{ a.status || '-' }}</span>
                              <span v-if="a.retryable" class="badge badge-warning !px-2 !py-0.5">可重试</span>
                            </div>
                            <div class="mt-1.5 grid grid-cols-1 md:grid-cols-3 gap-1.5 text-2xs text-t3 font-mono">
                              <span>API {{ a.api_type || '-' }}</span>
                              <span>MODEL {{ a.origin_model || '-' }}<template v-if="a.upstream_model && a.upstream_model !== a.origin_model"> → {{ a.upstream_model }}</template></span>
                              <span>T+ {{ attemptTime(a.at_ms) }}</span>
                            </div>
                            <div v-if="a.error" class="mt-2 rounded-md border border-border/80 bg-bg/40 px-2 py-1.5 font-mono text-2xs text-t2 break-all">
                              <span v-if="a.error_category" class="text-t3">{{ a.error_category }} · </span>{{ a.error }}
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </td>
              </tr>
            </template>
            <tr v-if="!logs.length && !loading">
              <td colspan="11" class="empty-state">
                <span class="font-mono text-3xl text-t3">∅</span>
                <span>暂无日志</span>
              </td>
            </tr>
            <tr v-if="loading">
              <td colspan="11" class="p-4">
                <div class="space-y-2">
                  <div v-for="i in 6" :key="i" class="skeleton h-6"></div>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- 分页 -->
    <div class="flex items-center justify-end gap-3 mt-4 font-mono text-2xs text-t2">
      <button class="page-btn" :disabled="page<=1" @click="prev">上一页</button>
      <span>PAGE {{ page }} · {{ total }} 条</span>
      <button class="page-btn" :disabled="page*pageSize>=total" @click="next">下一页</button>
    </div>
  </div>
</template>


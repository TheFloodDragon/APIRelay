<template>
  <section class="page-hero">
    <div>
      <p class="eyebrow">Observability</p>
      <h1>请求日志</h1>
      <p>追踪每一次中转请求的协议、渠道、状态码、延迟和错误信息。</p>
    </div>
    <div class="page-actions toolbar-panel">
      <el-input
        v-model="searchKeyword"
        placeholder="搜索模型、渠道、错误..."
        :prefix-icon="Search"
        clearable
        style="width: 280px"
        @input="handleFilterChange"
        @clear="handleFilterChange"
      />
      <el-select v-model="statusFilter" placeholder="状态筛选" style="width: 140px" @change="handleFilterChange">
        <el-option label="全部" value="all" />
        <el-option label="成功" value="success" />
        <el-option label="失败" value="failed" />
      </el-select>
      <el-button :icon="Refresh" :loading="loading" @click="loadLogs">刷新</el-button>
    </div>
  </section>

  <div class="metric-grid compact">
    <div class="metric-card">
      <span class="metric-label">日志总数</span>
      <strong>{{ total }}</strong>
      <small>来自 /api/logs</small>
    </div>
    <div class="metric-card accent-red">
      <span class="metric-label">当前页失败</span>
      <strong>{{ failedCount }}</strong>
      <small>HTTP 4xx / 5xx 或错误</small>
    </div>
    <div class="metric-card accent-amber">
      <span class="metric-label">平均延迟</span>
      <strong>{{ averageLatency }}ms</strong>
      <small>当前页样本</small>
    </div>
  </div>

  <el-card class="table-card" shadow="never">
    <template #header>
      <div class="panel-header">
        <span>请求明细</span>
        <el-tag type="info" effect="plain">双击行查看详情</el-tag>
      </div>
    </template>

    <el-table
      v-loading="loading"
      :data="filteredLogs"
      class="admin-table log-table"
      empty-text="暂无请求日志"
      @row-dblclick="openDetail"
    >
      <el-table-column label="结果" width="110" fixed>
        <template #default="{ row }">
          <el-tag :type="resultTagType(row)" effect="dark" round>
            {{ resultText(row) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="HTTP" width="88">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status_code)" effect="light" round>
            {{ row.status_code || '-' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="model" label="请求模型" min-width="170" show-overflow-tooltip />
      <el-table-column prop="resolved_model" label="解析模型" min-width="170" show-overflow-tooltip>
        <template #default="{ row }">{{ row.resolved_model || '-' }}</template>
      </el-table-column>
      <el-table-column label="渠道" min-width="150" show-overflow-tooltip>
        <template #default="{ row }">
          <span class="channel-chip">{{ row.channel?.name || row.channel_id || '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="渠道类型" min-width="130">
        <template #default="{ row }">
          <el-tag effect="plain" size="small">{{ row.channel_type || row.channel?.type || '-' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="API 类型" min-width="110">
        <template #default="{ row }">{{ row.api_type || '-' }}</template>
      </el-table-column>
      <el-table-column label="模式" min-width="140">
        <template #default="{ row }">{{ row.relay_mode || '-' }}</template>
      </el-table-column>
      <el-table-column label="格式" min-width="140">
        <template #default="{ row }">{{ row.relay_format || '-' }}</template>
      </el-table-column>
      <el-table-column label="延迟" width="110" sortable>
        <template #default="{ row }">
          <span :class="latencyClass(row.latency)">{{ formatLatency(row.latency) }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="error" label="错误" min-width="240" show-overflow-tooltip>
        <template #default="{ row }">
          <span :class="row.error ? 'text-danger' : 'text-muted'">{{ row.error || '无' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="时间" width="180" sortable>
        <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="100" fixed="right" align="center">
        <template #default="{ row }">
          <el-button type="primary" text :icon="View" size="small" @click.stop="openDetail(row)">详情</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="table-footer">
      <span>当前页筛选后 {{ filteredLogs.length }} 条 / 每页 {{ pageSize }} 条</span>
      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :page-sizes="[10, 20, 50, 100]"
        :total="total"
        layout="sizes, prev, pager, next"
        background
        @current-change="loadLogs"
        @size-change="handleSizeChange"
      />
    </div>
  </el-card>

  <el-drawer v-model="detailVisible" title="请求详情" size="520px" class="detail-drawer">
    <template v-if="selectedLog">
      <div class="detail-status" :class="detailStatusClass(selectedLog)">
        <div>
          <el-tag :type="resultTagType(selectedLog)" effect="dark" round>
            {{ resultText(selectedLog) }}
          </el-tag>
          <el-tag :type="statusType(selectedLog.status_code)" effect="light" round>
            HTTP {{ selectedLog.status_code || '-' }}
          </el-tag>
          <strong>{{ selectedLog.model || '-' }}</strong>
        </div>
        <small>{{ selectedLog.channel?.name || selectedLog.channel_id || '未匹配渠道' }} · {{ formatLatency(selectedLog.latency) }}</small>
      </div>

      <el-alert
        v-if="selectedLog.error"
        class="error-alert"
        title="调用失败 / 上游错误"
        type="error"
        :closable="false"
        show-icon
      >
        <template #default>
          <pre class="error-detail">{{ selectedLog.error }}</pre>
          <el-button type="danger" text size="small" @click="copyText(selectedLog.error || '')">复制错误</el-button>
        </template>
      </el-alert>
      <el-alert
        v-else-if="isSuccessful(selectedLog)"
        class="error-alert"
        title="调用成功"
        type="success"
        :closable="false"
        show-icon
      >
        <template #default>本次请求没有记录错误信息。</template>
      </el-alert>
      <el-alert
        v-else
        class="error-alert"
        title="结果未知"
        type="warning"
        :closable="false"
        show-icon
      >
        <template #default>日志没有记录明确错误，但 HTTP 状态码不是 2xx/3xx，请结合响应状态继续排查。</template>
      </el-alert>

      <div class="detail-section-title">链路字段</div>
      <el-descriptions :column="1" border class="detail-descriptions">
        <el-descriptions-item label="请求 ID">
          <span>{{ selectedLog.request_id || '-' }}</span>
          <el-button v-if="selectedLog.request_id" type="primary" text size="small" @click="copyText(selectedLog.request_id || '')">
            复制
          </el-button>
        </el-descriptions-item>
        <el-descriptions-item label="请求模型">{{ selectedLog.model || '-' }}</el-descriptions-item>
        <el-descriptions-item label="解析模型">{{ selectedLog.resolved_model || '-' }}</el-descriptions-item>
        <el-descriptions-item label="渠道">{{ selectedLog.channel?.name || selectedLog.channel_id || '-' }}</el-descriptions-item>
        <el-descriptions-item label="渠道类型">{{ selectedLog.channel_type || selectedLog.channel?.type || '-' }}</el-descriptions-item>
        <el-descriptions-item label="API 类型">{{ selectedLog.api_type || '-' }}</el-descriptions-item>
        <el-descriptions-item label="转发模式">{{ selectedLog.relay_mode || '-' }}</el-descriptions-item>
        <el-descriptions-item label="转发格式">{{ selectedLog.relay_format || '-' }}</el-descriptions-item>
        <el-descriptions-item label="调用结果">
          <el-tag :type="resultTagType(selectedLog)" effect="dark" round>{{ resultText(selectedLog) }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="HTTP 状态码">{{ selectedLog.status_code || '-' }}</el-descriptions-item>
        <el-descriptions-item label="路径">{{ selectedLog.method }} {{ selectedLog.path }}</el-descriptions-item>
        <el-descriptions-item label="Token">
          输入 {{ selectedLog.request_tokens || 0 }} / 输出 {{ selectedLog.response_tokens || 0 }}
        </el-descriptions-item>
        <el-descriptions-item label="延迟">
          <span :class="latencyClass(selectedLog.latency)">{{ selectedLog.latency }}ms</span>
        </el-descriptions-item>
        <el-descriptions-item label="IP">{{ selectedLog.ip || '-' }}</el-descriptions-item>
        <el-descriptions-item label="时间">{{ formatDate(selectedLog.created_at) }}</el-descriptions-item>
        <el-descriptions-item label="错误">
          <span :class="selectedLog.error ? 'text-danger' : 'text-muted'">{{ selectedLog.error || '无' }}</span>
        </el-descriptions-item>
      </el-descriptions>
    </template>
  </el-drawer>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, Search, View } from '@element-plus/icons-vue'
import { useRoute } from 'vue-router'
import { getLogs, type RequestLog } from '@/api/logs'

const route = useRoute()
const loading = ref(false)
const logs = ref<RequestLog[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const searchKeyword = ref('')
const statusFilter = ref<'all' | 'success' | 'failed'>('all')
const detailVisible = ref(false)
const selectedLog = ref<RequestLog | null>(null)
let loadToken = 0

type TagType = 'success' | 'warning' | 'danger' | 'info'
type ResultState = 'success' | 'failed' | 'unknown'

const failedCount = computed(() => logs.value.filter((item) => resultState(item) === 'failed').length)
const averageLatency = computed(() => {
  if (logs.value.length === 0) return 0
  const totalLatency = logs.value.reduce((sum, item) => sum + (item.latency || 0), 0)
  return Math.round(totalLatency / logs.value.length)
})

const filteredLogs = computed(() => {
  let result = logs.value

  if (statusFilter.value === 'success') {
    result = result.filter((item) => resultState(item) === 'success')
  } else if (statusFilter.value === 'failed') {
    result = result.filter((item) => resultState(item) === 'failed')
  }

  if (searchKeyword.value.trim()) {
    const keyword = searchKeyword.value.trim().toLowerCase()
    result = result.filter((item) => {
      const fields = [
        item.request_id,
        item.model,
        item.resolved_model,
        item.channel?.name,
        item.channel_type,
        item.api_type,
        item.relay_mode,
        item.relay_format,
        item.error,
        item.path,
        item.ip
      ]
      return fields.some((field) => String(field || '').toLowerCase().includes(keyword))
    })
  }

  return result
})

watch(
  () => route.fullPath,
  () => loadLogs(),
  { immediate: true }
)

async function loadLogs() {
  const currentToken = ++loadToken
  loading.value = true
  try {
    const res = await getLogs({
      limit: pageSize.value,
      offset: (page.value - 1) * pageSize.value
    })
    if (currentToken === loadToken) {
      logs.value = normalizeLogs(res.data.data)
      total.value = Number(res.data.total || 0)
      clampPage()
    }
  } catch (error: any) {
    if (currentToken === loadToken) {
      ElMessage.error(error?.response?.data?.error || '加载请求日志失败')
    }
  } finally {
    if (currentToken === loadToken) {
      loading.value = false
    }
  }
}

function normalizeLogs(value?: RequestLog[] | null): RequestLog[] {
  return (value || []).map((log) => ({
    ...log,
    model: log.model || '',
    method: log.method || '',
    path: log.path || '',
    status_code: Number(log.status_code || 0),
    request_tokens: Number(log.request_tokens || 0),
    response_tokens: Number(log.response_tokens || 0),
    latency: Number(log.latency || 0),
    ip: log.ip || ''
  }))
}

function resetPage() {
  page.value = 1
}

function clampPage() {
  const maxPage = Math.max(1, Math.ceil(total.value / pageSize.value))
  if (page.value > maxPage) {
    page.value = maxPage
    loadLogs()
  }
}

function handleFilterChange() {
  resetPage()
  loadLogs()
}

function handleSizeChange() {
  page.value = 1
  loadLogs()
}

function openDetail(row: RequestLog) {
  selectedLog.value = row
  detailVisible.value = true
}

async function copyText(value: string) {
  try {
    if (navigator.clipboard?.writeText && window.isSecureContext) {
      await navigator.clipboard.writeText(value)
    } else {
      copyTextFallback(value)
    }
    ElMessage.success('已复制')
  } catch {
    ElMessage.warning('复制失败,请手动复制')
  }
}

function copyTextFallback(value: string) {
  const textarea = document.createElement('textarea')
  textarea.value = value
  textarea.style.position = 'fixed'
  textarea.style.left = '-9999px'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.focus()
  textarea.select()
  const copied = document.execCommand('copy')
  document.body.removeChild(textarea)
  if (!copied) {
    throw new Error('copy command failed')
  }
}

function resultState(log: RequestLog): ResultState {
  if (log.error || log.status_code >= 400) return 'failed'
  if (log.status_code >= 200 && log.status_code < 400) return 'success'
  return 'unknown'
}

function isSuccessful(log: RequestLog) {
  return resultState(log) === 'success'
}

function resultText(log: RequestLog) {
  const state = resultState(log)
  if (state === 'success') return '成功'
  if (state === 'failed') return '失败'
  return '未知'
}

function resultTagType(log: RequestLog): TagType {
  const state = resultState(log)
  if (state === 'success') return 'success'
  if (state === 'failed') return 'danger'
  return 'warning'
}

function detailStatusClass(log: RequestLog) {
  return `detail-status--${resultState(log)}`
}

function statusType(status: number): TagType {
  if (status >= 200 && status < 300) return 'success'
  if (status >= 400 && status < 500) return 'warning'
  if (status >= 500) return 'danger'
  return 'info'
}

function latencyClass(latency: number) {
  const value = Number(latency || 0)
  if (value > 5000) return 'text-danger'
  if (value > 2000) return 'text-warning'
  return ''
}

function formatLatency(latency?: number) {
  return `${Number(latency || 0)}ms`
}

function formatDate(value?: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  }).format(date)
}
</script>

<style scoped>
.log-table :deep(.el-table__row) {
  cursor: pointer;
}

.channel-chip {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: 8px;
  font-size: 13px;
  background: var(--primary-light);
  color: var(--primary);
}

.detail-status {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-bottom: 18px;
  padding: 16px;
  border: 1px solid rgba(37, 99, 235, 0.16);
  border-radius: var(--radius-lg);
  background: linear-gradient(135deg, var(--primary-light), #ffffff);
  box-shadow: var(--shadow-subtle);
}

.detail-status--success {
  border-color: rgba(22, 163, 74, 0.24);
  background: linear-gradient(135deg, rgba(22, 163, 74, 0.12), #ffffff);
}

.detail-status--failed {
  border-color: rgba(220, 38, 38, 0.24);
  background: linear-gradient(135deg, rgba(220, 38, 38, 0.12), #ffffff);
}

.detail-status--unknown {
  border-color: rgba(217, 119, 6, 0.24);
  background: linear-gradient(135deg, rgba(217, 119, 6, 0.12), #ffffff);
}

.error-alert {
  margin-bottom: 18px;
}

.error-detail {
  margin: 6px 0 8px;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', monospace;
}

.detail-status > div {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.detail-status strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.detail-status small {
  color: var(--muted);
  font-weight: 600;
}

.detail-section-title {
  margin: 18px 0 10px;
  color: var(--muted);
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.detail-descriptions :deep(.el-descriptions__label) {
  width: 122px;
  color: var(--muted);
  font-weight: 700;
}

.detail-descriptions :deep(.el-descriptions__content) {
  word-break: break-word;
}
</style>

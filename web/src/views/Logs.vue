<template>
  <section class="page-hero">
    <div>
      <p class="eyebrow">Observability</p>
      <h1>请求日志</h1>
      <p>追踪每一次中转请求的协议、渠道、状态码、延迟和错误信息。</p>
    </div>
    <div class="page-actions">
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
      <el-table-column label="状态" width="96" fixed>
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
      <div class="detail-status">
        <el-tag :type="statusType(selectedLog.status_code)" effect="light" round>
          {{ selectedLog.status_code || '-' }}
        </el-tag>
        <strong>{{ selectedLog.model || '-' }}</strong>
      </div>

      <el-descriptions :column="1" border>
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

const failedCount = computed(() => logs.value.filter((item) => item.status_code >= 400 || item.error).length)
const averageLatency = computed(() => {
  if (logs.value.length === 0) return 0
  const totalLatency = logs.value.reduce((sum, item) => sum + (item.latency || 0), 0)
  return Math.round(totalLatency / logs.value.length)
})

const filteredLogs = computed(() => {
  let result = logs.value

  if (statusFilter.value === 'success') {
    result = result.filter((item) => item.status_code >= 200 && item.status_code < 400 && !item.error)
  } else if (statusFilter.value === 'failed') {
    result = result.filter((item) => item.status_code >= 400 || !!item.error)
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
  align-items: center;
  gap: 12px;
  margin-bottom: 18px;
  padding: 14px;
  border-radius: var(--radius-md);
  background: var(--primary-light);
}

.detail-status strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>

<template>
  <section class="page-hero">
    <div>
      <p class="eyebrow">Observability</p>
      <h1>请求日志</h1>
      <p>追踪每一次中转请求的协议、渠道、状态码、延迟和错误信息。</p>
    </div>
    <div class="page-actions">
      <el-button :icon="Refresh" :loading="loading" @click="loadLogs">刷新</el-button>
    </div>
  </section>

  <div class="metric-grid compact">
    <div class="metric-card">
      <span class="metric-label">日志总数</span>
      <strong>{{ total }}</strong>
      <small>来自 /api/logs</small>
    </div>
    <div class="metric-card">
      <span class="metric-label">当前页失败</span>
      <strong>{{ failedCount }}</strong>
      <small>HTTP 4xx / 5xx</small>
    </div>
    <div class="metric-card">
      <span class="metric-label">平均延迟</span>
      <strong>{{ averageLatency }}ms</strong>
      <small>当前页样本</small>
    </div>
  </div>

  <el-card class="table-card" shadow="never">
    <el-table v-loading="loading" :data="logs" class="admin-table" empty-text="暂无请求日志">
      <el-table-column label="状态" width="96">
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
        <template #default="{ row }">{{ row.channel?.name || row.channel_id || '-' }}</template>
      </el-table-column>
      <el-table-column label="渠道类型" min-width="130">
        <template #default="{ row }">{{ row.channel_type || row.channel?.type || '-' }}</template>
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
      <el-table-column label="延迟" width="100">
        <template #default="{ row }">{{ row.latency }}ms</template>
      </el-table-column>
      <el-table-column prop="error" label="错误" min-width="220" show-overflow-tooltip>
        <template #default="{ row }">
          <span :class="row.error ? 'text-danger' : 'text-muted'">{{ row.error || '无' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="时间" width="180">
        <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
      </el-table-column>
    </el-table>

    <div class="table-footer">
      <span>每页展示 {{ pageSize }} 条</span>
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
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { getLogs, type RequestLog } from '@/api/logs'

const loading = ref(false)
const logs = ref<RequestLog[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)

type TagType = 'success' | 'warning' | 'danger' | 'info'

const failedCount = computed(() => logs.value.filter((item) => item.status_code >= 400 || item.error).length)
const averageLatency = computed(() => {
  if (logs.value.length === 0) return 0
  const totalLatency = logs.value.reduce((sum, item) => sum + (item.latency || 0), 0)
  return Math.round(totalLatency / logs.value.length)
})

onMounted(loadLogs)

async function loadLogs() {
  loading.value = true
  try {
    const res = await getLogs({
      limit: pageSize.value,
      offset: (page.value - 1) * pageSize.value
    })
    logs.value = res.data.data || []
    total.value = res.data.total || 0
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || '加载请求日志失败')
  } finally {
    loading.value = false
  }
}

function handleSizeChange() {
  page.value = 1
  loadLogs()
}

function statusType(status: number): TagType {
  if (status >= 200 && status < 300) return 'success'
  if (status >= 400 && status < 500) return 'warning'
  if (status >= 500) return 'danger'
  return 'info'
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

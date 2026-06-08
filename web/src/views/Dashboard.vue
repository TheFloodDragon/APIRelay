<template>
  <section class="page-hero">
    <div>
      <p class="eyebrow">Overview</p>
      <h1>仪表盘</h1>
      <p>聚合渠道、模型和最近请求日志,快速了解 APIRelay 当前运行状态。</p>
    </div>
    <div class="page-actions">
      <el-button :icon="Refresh" :loading="loading" @click="loadDashboard">刷新</el-button>
      <el-button type="primary" :icon="Plus" @click="router.push('/channels')">添加渠道</el-button>
    </div>
  </section>

  <div class="metric-grid">
    <div class="metric-card accent-blue" @click="router.push('/channels')">
      <span class="metric-label">渠道总数</span>
      <strong>{{ channels.length }}</strong>
      <small>{{ enabledChannels }} 个已启用</small>
      <div class="metric-progress">
        <div class="progress-bar" :style="{ width: channelEnabledPercentage + '%' }"></div>
      </div>
    </div>
    <div class="metric-card accent-green" @click="router.push('/models')">
      <span class="metric-label">模型总数</span>
      <strong>{{ models.length }}</strong>
      <small>{{ enabledModels }} 个可用模型</small>
      <div class="metric-progress">
        <div class="progress-bar" :style="{ width: modelEnabledPercentage + '%' }"></div>
      </div>
    </div>
    <div class="metric-card accent-purple" @click="router.push('/logs')">
      <span class="metric-label">请求总数</span>
      <strong>{{ logTotal }}</strong>
      <small>日志表记录总量</small>
    </div>
    <div class="metric-card accent-red">
      <span class="metric-label">近 {{ logs.length }} 条失败</span>
      <strong>{{ failedRequests }}</strong>
      <small>错误或 4xx / 5xx</small>
      <div class="metric-progress">
        <div
          class="progress-bar"
          :style="{ width: failureRate + '%', background: 'var(--danger)' }"
        ></div>
      </div>
    </div>
    <div class="metric-card accent-amber">
      <span class="metric-label">平均延迟</span>
      <strong>{{ averageLatency }}ms</strong>
      <small>最近请求样本</small>
      <el-tag
        v-if="averageLatency > 0"
        :type="latencyType"
        effect="plain"
        size="small"
        style="margin-top: 8px"
      >
        {{ latencyText }}
      </el-tag>
    </div>
  </div>

  <div class="dashboard-grid">
    <el-card class="panel-card" shadow="never">
      <template #header>
        <div class="panel-header">
          <span>渠道健康</span>
          <div style="display: flex; gap: 8px">
            <el-tag type="info" effect="plain">{{ channels.length }} 个渠道</el-tag>
            <el-button type="primary" text size="small" @click="router.push('/channels')">
              管理
            </el-button>
          </div>
        </div>
      </template>
      <div v-if="channels.length" class="health-list">
        <div v-for="item in healthStats" :key="item.label" class="health-row">
          <span class="health-dot" :class="item.className" />
          <span>{{ item.label }}</span>
          <strong>{{ item.count }}</strong>
        </div>
      </div>
      <el-empty v-else description="暂无渠道" :image-size="80">
        <el-button type="primary" @click="router.push('/channels')">添加渠道</el-button>
      </el-empty>
    </el-card>

    <el-card class="panel-card" shadow="never">
      <template #header>
        <div class="panel-header">
          <span>模型覆盖</span>
          <div style="display: flex; gap: 8px">
            <el-tag type="success" effect="plain">{{ uniqueModelNames }} 个唯一模型</el-tag>
            <el-button type="primary" text size="small" @click="router.push('/models')">
              查看
            </el-button>
          </div>
        </div>
      </template>
      <div v-if="topChannels.length" class="rank-list">
        <div v-for="channel in topChannels" :key="channel.id" class="rank-row">
          <div>
            <strong>{{ channel.name }}</strong>
            <small>{{ channel.type || 'openai_compatible' }}</small>
          </div>
          <el-progress
            :percentage="channelCoverage(channel.models?.length || 0)"
            :show-text="false"
            :stroke-width="8"
          />
          <span class="model-count">{{ channel.models?.length || 0 }} 个</span>
        </div>
      </div>
      <el-empty v-else description="暂无模型覆盖数据" :image-size="80" />
    </el-card>
  </div>

  <el-card class="table-card" shadow="never">
    <template #header>
      <div class="panel-header">
        <span>最近请求</span>
        <RouterLink to="/logs" class="text-link">查看全部 →</RouterLink>
      </div>
    </template>
    <el-table
      v-loading="loading"
      :data="logs.slice(0, 10)"
      class="admin-table"
      empty-text="暂无请求日志"
    >
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status_code)" effect="light" round>
            {{ row.status_code || '-' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="model" label="模型" min-width="180" show-overflow-tooltip />
      <el-table-column label="渠道" min-width="160" show-overflow-tooltip>
        <template #default="{ row }">{{ row.channel?.name || row.channel_id || '-' }}</template>
      </el-table-column>
      <el-table-column label="协议" width="140">
        <template #default="{ row }">{{ row.api_type || '-' }} / {{ row.relay_mode || '-' }}</template>
      </el-table-column>
      <el-table-column label="延迟" width="100" sortable>
        <template #default="{ row }">
          <span :class="{ 'text-danger': row.latency > 5000, 'text-warning': row.latency > 2000 }">
            {{ formatLatency(row.latency) }}
          </span>
        </template>
      </el-table-column>
      <el-table-column label="时间" width="170" sortable>
        <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
      </el-table-column>
    </el-table>
  </el-card>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Plus, Refresh } from '@element-plus/icons-vue'
import { getChannels, type Channel } from '@/api/channels'
import { getLogs, type RequestLog } from '@/api/logs'
import { getModels, type ModelRecord } from '@/api/models'

type TagType = 'success' | 'warning' | 'danger' | 'info'

const router = useRouter()
const route = useRoute()
const loading = ref(false)
const channels = ref<Channel[]>([])
const models = ref<ModelRecord[]>([])
const logs = ref<RequestLog[]>([])
const logTotal = ref(0)
let loadToken = 0

const enabledChannels = computed(() => channels.value.filter((item) => item.enabled).length)
const enabledModels = computed(() => models.value.filter((item) => item.enabled).length)
const failedRequests = computed(() => logs.value.filter((item) => item.status_code >= 400 || item.error).length)

const channelEnabledPercentage = computed(() =>
  channels.value.length > 0 ? Math.round((enabledChannels.value / channels.value.length) * 100) : 0
)

const modelEnabledPercentage = computed(() =>
  models.value.length > 0 ? Math.round((enabledModels.value / models.value.length) * 100) : 0
)

const failureRate = computed(() =>
  logs.value.length > 0 ? Math.round((failedRequests.value / logs.value.length) * 100) : 0
)

const averageLatency = computed(() => {
  if (logs.value.length === 0) return 0
  return Math.round(logs.value.reduce((sum, item) => sum + (item.latency || 0), 0) / logs.value.length)
})

const latencyType = computed<TagType>(() => {
  const latency = averageLatency.value
  if (latency < 1000) return 'success'
  if (latency < 3000) return 'warning'
  return 'danger'
})

const latencyText = computed(() => {
  const latency = averageLatency.value
  if (latency < 1000) return '表现良好'
  if (latency < 3000) return '略有延迟'
  return '延迟较高'
})

const uniqueModelNames = computed(() => {
  const names = models.value.map((item) => item.display_name || item.name)
  return new Set(names).size
})

const maxChannelModels = computed(() => Math.max(...channels.value.map((item) => item.models?.length || 0), 1))

const topChannels = computed(() =>
  [...channels.value].sort((a, b) => (b.models?.length || 0) - (a.models?.length || 0)).slice(0, 5)
)

const healthStats = computed(() => {
  const healthy = channels.value.filter((item) => item.health_status === 'healthy').length
  const unhealthy = channels.value.filter((item) => item.health_status === 'unhealthy').length
  const unknown = channels.value.length - healthy - unhealthy
  return [
    { label: '健康', count: healthy, className: 'is-healthy' },
    { label: '异常', count: unhealthy, className: 'is-unhealthy' },
    { label: '未知', count: unknown, className: 'is-unknown' }
  ]
})

watch(
  () => route.fullPath,
  () => loadDashboard(),
  { immediate: true }
)

async function loadDashboard() {
  const currentToken = ++loadToken
  loading.value = true
  try {
    const [channelRes, modelRes, logRes] = await Promise.all([
      getChannels(),
      getModels(),
      getLogs({ limit: 50, offset: 0 })
    ])
    if (currentToken === loadToken) {
      channels.value = normalizeChannels(channelRes.data.data)
      models.value = normalizeModels(modelRes.data.data)
      logs.value = normalizeLogs(logRes.data.data)
      logTotal.value = Number(logRes.data.total || 0)
    }
  } catch (error: any) {
    if (currentToken === loadToken) {
      ElMessage.error(error?.response?.data?.error || '加载仪表盘失败')
    }
  } finally {
    if (currentToken === loadToken) {
      loading.value = false
    }
  }
}

function normalizeChannels(value?: Channel[] | null): Channel[] {
  return (value || []).map((channel) => ({
    ...channel,
    models: Array.isArray(channel.models) ? channel.models : [],
    health_status: channel.health_status || 'unknown'
  }))
}

function normalizeModels(value?: ModelRecord[] | null): ModelRecord[] {
  return (value || []).map((model) => ({
    ...model,
    name: model.name || '',
    display_name: model.display_name || model.name || '',
    enabled: model.enabled ?? true
  }))
}

function normalizeLogs(value?: RequestLog[] | null): RequestLog[] {
  return (value || []).map((log) => ({
    ...log,
    status_code: Number(log.status_code || 0),
    latency: Number(log.latency || 0)
  }))
}

function channelCoverage(count: number) {
  return Math.round((count / maxChannelModels.value) * 100)
}

function statusType(status: number): TagType {
  if (status >= 200 && status < 300) return 'success'
  if (status >= 400 && status < 500) return 'warning'
  if (status >= 500) return 'danger'
  return 'info'
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
.metric-card {
  cursor: pointer;
}

.metric-progress {
  position: absolute;
  bottom: 0;
  left: 0;
  width: 100%;
  height: 3px;
  border-radius: 0 0 var(--radius-lg) var(--radius-lg);
  background: rgba(0, 0, 0, 0.05);
  overflow: hidden;
}

.progress-bar {
  height: 100%;
  background: var(--primary);
  transition: width 0.6s cubic-bezier(0.4, 0, 0.2, 1);
}

.model-count {
  font-weight: 600;
  color: var(--primary);
}

.text-warning {
  color: var(--warning);
}
</style>

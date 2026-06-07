<template>
  <section class="page-hero">
    <div>
      <p class="eyebrow">Overview</p>
      <h1>仪表盘</h1>
      <p>聚合渠道、模型和最近请求日志，快速了解 APIRelay 当前运行状态。</p>
    </div>
    <div class="page-actions">
      <el-button :icon="Refresh" :loading="loading" @click="loadDashboard">刷新</el-button>
    </div>
  </section>

  <div class="metric-grid">
    <div class="metric-card accent-blue">
      <span class="metric-label">渠道总数</span>
      <strong>{{ channels.length }}</strong>
      <small>{{ enabledChannels }} 个已启用</small>
    </div>
    <div class="metric-card accent-green">
      <span class="metric-label">模型总数</span>
      <strong>{{ models.length }}</strong>
      <small>{{ enabledModels }} 个可用模型</small>
    </div>
    <div class="metric-card accent-purple">
      <span class="metric-label">请求总数</span>
      <strong>{{ logTotal }}</strong>
      <small>日志表记录总量</small>
    </div>
    <div class="metric-card accent-red">
      <span class="metric-label">近 {{ logs.length }} 条失败</span>
      <strong>{{ failedRequests }}</strong>
      <small>错误或 4xx / 5xx</small>
    </div>
    <div class="metric-card accent-amber">
      <span class="metric-label">平均延迟</span>
      <strong>{{ averageLatency }}ms</strong>
      <small>最近请求样本</small>
    </div>
  </div>

  <div class="dashboard-grid">
    <el-card class="panel-card" shadow="never">
      <template #header>
        <div class="panel-header">
          <span>渠道健康</span>
          <el-tag type="info" effect="plain">{{ channels.length }} 个渠道</el-tag>
        </div>
      </template>
      <div v-if="channels.length" class="health-list">
        <div v-for="item in healthStats" :key="item.label" class="health-row">
          <span class="health-dot" :class="item.className" />
          <span>{{ item.label }}</span>
          <strong>{{ item.count }}</strong>
        </div>
      </div>
      <el-empty v-else description="暂无渠道" :image-size="80" />
    </el-card>

    <el-card class="panel-card" shadow="never">
      <template #header>
        <div class="panel-header">
          <span>模型覆盖</span>
          <el-tag type="success" effect="plain">{{ uniqueModelNames }} 个唯一模型</el-tag>
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
          <span>{{ channel.models?.length || 0 }} 个</span>
        </div>
      </div>
      <el-empty v-else description="暂无模型覆盖数据" :image-size="80" />
    </el-card>
  </div>

  <el-card class="table-card" shadow="never">
    <template #header>
      <div class="panel-header">
        <span>最近请求</span>
        <RouterLink to="/logs" class="text-link">查看全部</RouterLink>
      </div>
    </template>
    <el-table v-loading="loading" :data="logs.slice(0, 8)" class="admin-table" empty-text="暂无请求日志">
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status_code)" effect="light" round>{{ row.status_code || '-' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="model" label="模型" min-width="180" show-overflow-tooltip />
      <el-table-column label="渠道" min-width="160" show-overflow-tooltip>
        <template #default="{ row }">{{ row.channel?.name || row.channel_id || '-' }}</template>
      </el-table-column>
      <el-table-column label="协议" width="140">
        <template #default="{ row }">{{ row.api_type || '-' }} / {{ row.relay_mode || '-' }}</template>
      </el-table-column>
      <el-table-column label="延迟" width="100">
        <template #default="{ row }">{{ row.latency }}ms</template>
      </el-table-column>
      <el-table-column label="时间" width="170">
        <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
      </el-table-column>
    </el-table>
  </el-card>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { getChannels, type Channel } from '@/api/channels'
import { getLogs, type RequestLog } from '@/api/logs'
import { getModels, type ModelRecord } from '@/api/models'

type TagType = 'success' | 'warning' | 'danger' | 'info'

const loading = ref(false)
const channels = ref<Channel[]>([])
const models = ref<ModelRecord[]>([])
const logs = ref<RequestLog[]>([])
const logTotal = ref(0)

const enabledChannels = computed(() => channels.value.filter((item) => item.enabled).length)
const enabledModels = computed(() => models.value.filter((item) => item.enabled).length)
const failedRequests = computed(() => logs.value.filter((item) => item.status_code >= 400 || item.error).length)
const averageLatency = computed(() => {
  if (logs.value.length === 0) return 0
  return Math.round(logs.value.reduce((sum, item) => sum + (item.latency || 0), 0) / logs.value.length)
})
const uniqueModelNames = computed(() => {
  // 优先使用 display_name，为空时回退到 name
  const names = models.value.map((item) => item.display_name || item.name)
  return new Set(names).size
})
const maxChannelModels = computed(() => Math.max(...channels.value.map((item) => item.models?.length || 0), 1))
const topChannels = computed(() =>
  [...channels.value]
    .sort((a, b) => (b.models?.length || 0) - (a.models?.length || 0))
    .slice(0, 5)
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

onMounted(loadDashboard)

async function loadDashboard() {
  loading.value = true
  try {
    const [channelRes, modelRes, logRes] = await Promise.all([
      getChannels(),
      getModels(),
      getLogs({ limit: 50, offset: 0 })
    ])
    channels.value = channelRes.data.data || []
    models.value = modelRes.data.data || []
    logs.value = logRes.data.data || []
    logTotal.value = logRes.data.total || 0
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || '加载仪表盘失败')
  } finally {
    loading.value = false
  }
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

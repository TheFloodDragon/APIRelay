<script setup>
import { computed, onMounted, ref } from 'vue'
import api, { usd } from '../api'
import PageState from '../components/PageState.vue'
import PageHeader from '../components/PageHeader.vue'
import RouteHealthBar from '../components/RouteHealthBar.vue'
import StatusBadge from '../components/StatusBadge.vue'

const loading = ref(true)
const error = ref('')
const stat = ref({})
const dashboardChannelCount = ref(0)
const models = ref([])
const channels = ref([])
const channelTypes = ref([])
const channelHealth = ref({})
const anomalies = ref([])
const anomaliesError = ref('')

const fmt = (value) => (Number(value) || 0).toLocaleString()

const availableModelCount = computed(() => models.value.filter((model) => {
  if (!Array.isArray(model?.providers)) return true
  return model.providers.some((provider) => provider.enabled !== false)
}).length)

const channelCount = computed(() => dashboardChannelCount.value || channels.value.length)
const healthyChannelCount = computed(() => routeRows.value.filter((row) => row.status === 'healthy').length)

const typeMap = computed(() => Object.fromEntries(channelTypes.value.map((item) => [item.value, item.name])))
const latestErrorByChannel = computed(() => {
  const result = {}
  for (const item of anomalies.value) {
    if (item?.channel_id && !result[item.channel_id]) result[item.channel_id] = item
  }
  return result
})

function modelCount(channel) {
  if (channel.model_configs) {
    try {
      const list = JSON.parse(channel.model_configs)
      if (Array.isArray(list)) return list.filter((model) => model?.enabled !== false && String(model?.name || '').trim()).length
    } catch {
      // 兼容旧渠道数据。
    }
  }
  return String(channel.models || '').split(',').map((item) => item.trim()).filter(Boolean).length
}

function formatTime(value) {
  if (!value) return '时间未知'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '时间未知'
  const pad = (part) => String(part).padStart(2, '0')
  return `${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function shortText(value, fallback = '未提供错误详情') {
  const text = String(value || '').replace(/\s+/g, ' ').trim()
  if (!text) return fallback
  return text.length > 120 ? `${text.slice(0, 117)}…` : text
}

function stateOf(channel, health) {
  if (channel.status !== 1) return { status: 'disabled', label: '已停用' }
  if (!health) return { status: 'unknown', label: '状态未知' }
  if (health.circuit_state === 'open' || (channel.cooldown_until && channel.cooldown_until > Date.now())) {
    return { status: 'error', label: '已熔断' }
  }
  if (health.circuit_state === 'half_open') return { status: 'warning', label: '恢复检查' }
  if (Number(health.consecutive_failures) > 0) return { status: 'warning', label: '有失败' }
  return { status: 'healthy', label: '可用' }
}

function recentOf(channel, health) {
  const latestError = latestErrorByChannel.value[channel.id]
  if (latestError) {
    const latency = Number(latestError.use_time_ms) > 0 ? `${fmt(latestError.use_time_ms)} ms` : `HTTP ${latestError.status || '失败'}`
    return {
      primary: `${latency} · ${formatTime(latestError.created_at)}`,
      secondary: shortText(latestError.error),
    }
  }
  if (health?.last_error) {
    return {
      primary: `最近失败于 ${formatTime(health.last_failure_at || health.updated_at)}`,
      secondary: shortText(health.last_error),
    }
  }
  if (health?.last_success_at) {
    return {
      primary: '暂无延迟数据',
      secondary: `最近成功于 ${formatTime(health.last_success_at)}`,
    }
  }
  return { primary: '暂无延迟或错误记录', secondary: '' }
}

const routeRows = computed(() => channels.value.map((channel, index) => {
  const health = channelHealth.value[channel.id]
  const state = stateOf(channel, health)
  const recent = recentOf(channel, health)
  return {
    id: channel.id,
    priority: index + 1,
    name: channel.name || `渠道 ${channel.id}`,
    group: channel.group || 'default',
    protocol: typeMap.value[channel.type] || ({ 1: 'OpenAI', 2: 'Anthropic', 3: 'Responses' }[channel.type] || '未识别'),
    status: state.status,
    statusLabel: state.label,
    modelCount: modelCount(channel),
    recentPrimary: recent.primary,
    recentSecondary: recent.secondary,
  }
}))

function dashboardAnomalies(dashboard) {
  for (const key of ['anomalies', 'recent_anomalies', 'recent_errors', 'errors']) {
    if (!Object.prototype.hasOwnProperty.call(dashboard, key)) continue
    const value = dashboard[key]
    if (Array.isArray(value)) return value
    if (Array.isArray(value?.items)) return value.items
    return []
  }
  return null
}

function anomalyStatus(item) {
  const status = Number(item?.status)
  if (status >= 500) return { tone: 'error', label: `HTTP ${status}` }
  if (status >= 400) return { tone: 'warning', label: `HTTP ${status}` }
  return { tone: 'error', label: '请求失败' }
}

async function load() {
  loading.value = true
  error.value = ''
  anomaliesError.value = ''
  try {
    const [dashboard, modelList, channelList, typeList] = await Promise.all([
      api.get('/dashboard'),
      api.get('/models').catch(() => []),
      api.get('/channels').catch(() => []),
      api.get('/channel-types').catch(() => []),
    ])

    stat.value = dashboard?.stat || {}
    dashboardChannelCount.value = Number(dashboard?.channel_count) || 0
    models.value = Array.isArray(modelList) ? modelList : []
    channels.value = Array.isArray(channelList) ? channelList : []
    channelTypes.value = Array.isArray(typeList) ? typeList : []

    const suppliedAnomalies = dashboardAnomalies(dashboard || {})
    const anomalyRequest = suppliedAnomalies === null
      ? api.get('/logs', { params: { type: 2, page_size: 5 } })
          .then((data) => Array.isArray(data?.items) ? data.items : [])
          .catch((err) => {
            anomaliesError.value = err.message || '近期异常暂时无法读取'
            return []
          })
      : Promise.resolve(suppliedAnomalies.slice(0, 5))

    const healthRequest = Promise.all(channels.value.map(async (channel) => {
      try {
        return [channel.id, await api.get(`/channels/${channel.id}/health`)]
      } catch {
        return [channel.id, null]
      }
    }))

    const [recentItems, healthRows] = await Promise.all([anomalyRequest, healthRequest])
    anomalies.value = recentItems.slice(0, 5)
    channelHealth.value = Object.fromEntries(healthRows)
  } catch (err) {
    error.value = err.message || '总览加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="page-workbench dashboard-page space-y-5">
    <PageHeader eyebrow="实时路由概览" title="路由总览" description="监看模型供给、渠道健康与最近 7 天的调用态势。">
      <template #actions>
        <button class="btn" :disabled="loading" @click="load">{{ loading ? '刷新中…' : '刷新' }}</button>
      </template>
    </PageHeader>

    <PageState :loading="loading" :error="error" @retry="load">
      <div class="grid grid-cols-2 gap-3 lg:grid-cols-4" aria-label="核心指标">
        <article class="metric-card">
          <div class="metric-label">渠道</div>
          <div class="metric-value">{{ fmt(channelCount) }}</div>
          <div class="metric-hint">当前已配置渠道</div>
        </article>
        <article class="metric-card">
          <div class="metric-label">可用模型</div>
          <div class="metric-value">{{ fmt(availableModelCount) }}</div>
          <div class="metric-hint">至少有一个可用渠道</div>
        </article>
        <article class="metric-card">
          <div class="metric-label">近 7 日请求</div>
          <div class="metric-value">{{ fmt(stat.total_requests) }}</div>
          <div class="metric-hint">成功计费请求总数</div>
        </article>
        <article class="metric-card">
          <div class="metric-label">近 7 日费用</div>
          <div class="metric-value">{{ usd(stat.total_quota) }}</div>
          <div class="metric-hint">按当前计价汇总</div>
        </article>
      </div>

      <div class="mt-6 grid items-start gap-5 xl:grid-cols-[minmax(0,1.35fr)_minmax(340px,.65fr)]">
      <section class="sheet" aria-labelledby="route-health-title">
        <div class="sheet-head">
          <div>
            <h2 id="route-health-title" class="dim-title">路由健康</h2>
            <p class="mt-1 text-xs text-soft">按当前优先级展示渠道状态；延迟缺失时显示最近错误或成功时间。</p>
          </div>
          <span class="text-xs text-soft">{{ healthyChannelCount }} / {{ channels.length }} 个渠道可用</span>
        </div>
        <RouteHealthBar :rows="routeRows" />
      </section>

      <section class="sheet" aria-labelledby="recent-anomalies-title">
        <div class="sheet-head">
          <div>
            <h2 id="recent-anomalies-title" class="dim-title">近期异常</h2>
            <p class="mt-1 text-xs text-soft">最近记录的 5 条失败请求。</p>
          </div>
          <RouterLink class="btn btn-sm" to="/logs">查看全部日志</RouterLink>
        </div>

        <div v-if="anomalies.length" class="divide-y divide-line">
          <article v-for="item in anomalies" :key="item.id || item.request_id" class="grid gap-3 px-4 py-3 sm:grid-cols-[120px_minmax(0,1fr)_auto] sm:items-center">
            <div>
              <StatusBadge :status="anomalyStatus(item).tone" :label="anomalyStatus(item).label" />
              <div class="mt-1 font-mono text-xs text-soft">{{ formatTime(item.created_at) }}</div>
            </div>
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm">
                <span class="font-medium text-ink">{{ item.channel_name || (item.channel_id ? `渠道 ${item.channel_id}` : '未知渠道') }}</span>
                <span class="font-mono text-xs text-soft">{{ item.src_model || item.model || '未记录模型' }}</span>
              </div>
              <p class="mt-1 break-words text-xs leading-5 text-soft">{{ shortText(item.error) }}</p>
            </div>
            <div class="whitespace-nowrap text-right font-mono text-xs text-soft">
              {{ Number(item.use_time_ms) > 0 ? `${fmt(item.use_time_ms)} ms` : '耗时未知' }}
            </div>
          </article>
        </div>
        <div v-else class="px-4 py-10 text-center">
          <p class="text-sm font-medium text-ink">{{ anomaliesError ? '近期异常暂不可用' : '近期没有异常记录' }}</p>
          <p class="mt-1 text-xs text-soft">{{ anomaliesError || '系统暂未记录失败请求。' }}</p>
        </div>
      </section>
      </div>
    </PageState>
  </div>
</template>

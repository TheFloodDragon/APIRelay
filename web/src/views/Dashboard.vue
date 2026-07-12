<script setup>
import { computed, onMounted, ref } from 'vue'
import api, { usd } from '../api'
import ConsoleIcon from '../components/ConsoleIcon.vue'
import ConsoleSection from '../components/ConsoleSection.vue'
import InlineNotice from '../components/InlineNotice.vue'
import PageState from '../components/PageState.vue'
import RouteHealthBar from '../components/RouteHealthBar.vue'
import StatCell from '../components/StatCell.vue'
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
const unhealthyChannelCount = computed(() => routeRows.value.filter((row) => ['warning', 'error'].includes(row.status)).length)

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

function healthMetaOf(channel, health) {
  if (channel.status !== 1) return '不参与路由'
  if (!health) return '未返回健康数据'
  const failures = Number(health.consecutive_failures) || 0
  if (health.circuit_state === 'open') return failures ? `open · 连续失败 ${failures}` : 'circuit open'
  if (health.circuit_state === 'half_open') return 'half-open · 等待探测'
  if (failures) return `closed · 连续失败 ${failures}`
  return 'closed · 探测正常'
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
    healthMeta: healthMetaOf(channel, health),
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

function severityRank(item) {
  const status = Number(item?.status)
  if (status >= 500) return 3
  if (status === 429 || status === 408) return 2
  if (status >= 400) return 1
  return 2
}

function anomalyStatus(item) {
  const status = Number(item?.status)
  if (status >= 500) return { tone: 'error', label: `严重 · ${status}` }
  if (status === 429) return { tone: 'warning', label: '限流 · 429' }
  if (status === 408) return { tone: 'warning', label: '超时 · 408' }
  if (status >= 400) return { tone: 'warning', label: `HTTP ${status}` }
  return { tone: 'error', label: '请求失败' }
}

const anomalyQueue = computed(() => [...anomalies.value].sort((left, right) => {
  const severityDelta = severityRank(right) - severityRank(left)
  if (severityDelta) return severityDelta
  const timeDelta = new Date(right?.created_at || 0).getTime() - new Date(left?.created_at || 0).getTime()
  if (timeDelta) return timeDelta
  const leftChannel = String(left?.channel_name || left?.channel_id || '')
  const rightChannel = String(right?.channel_name || right?.channel_id || '')
  return leftChannel.localeCompare(rightChannel, 'zh-CN')
}))

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
  <div class="page-workbench dashboard-console">
    <header class="dashboard-commandbar">
      <div class="dashboard-heading">
        <div class="dashboard-heading-icon"><ConsoleIcon name="dashboard" /></div>
        <div class="min-w-0">
          <p class="dashboard-kicker">ROUTING CONTROL / 7D WINDOW</p>
          <h1>运行总览</h1>
          <p>渠道供给、路由健康与失败请求的实时工作面。</p>
        </div>
      </div>
      <button class="btn btn-sm" type="button" :disabled="loading" @click="load">
        <ConsoleIcon name="arrowPath" class="h-4 w-4" :class="{ 'animate-spin': loading }" />
        {{ loading ? '同步中' : '刷新数据' }}
      </button>
    </header>

    <div v-if="loading" class="dashboard-skeleton" role="status" aria-live="polite">
      <span class="sr-only">正在加载总览</span>
      <div class="dashboard-skeleton-metrics">
        <div v-for="index in 4" :key="`metric-${index}`" class="dashboard-skeleton-cell"><i></i><b></b><i></i></div>
      </div>
      <div class="dashboard-main-grid">
        <div class="dashboard-skeleton-panel dashboard-skeleton-routes">
          <div class="dashboard-skeleton-panel-head"><b></b><i></i></div>
          <div v-for="index in 6" :key="`route-${index}`" class="dashboard-skeleton-row"><i></i><b></b><i></i><i></i><b></b></div>
        </div>
        <div class="dashboard-skeleton-panel">
          <div class="dashboard-skeleton-panel-head"><b></b><i></i></div>
          <div v-for="index in 5" :key="`anomaly-${index}`" class="dashboard-skeleton-alert"><i></i><b></b><i></i></div>
        </div>
      </div>
      <div class="dashboard-skeleton-shortcuts"><i v-for="index in 4" :key="`shortcut-${index}`"></i></div>
    </div>

    <PageState v-else :error="error" @retry="load">
      <div class="dashboard-metric-strip" aria-label="核心指标">
        <StatCell label="渠道" :value="fmt(channelCount)" hint="当前已配置渠道" icon="server" />
        <StatCell label="可用模型" :value="fmt(availableModelCount)" hint="至少有一个可用渠道" icon="models" tone="success" />
        <StatCell label="7 日请求" :value="fmt(stat.total_requests)" hint="成功计费请求总数" icon="bolt" />
        <StatCell label="7 日费用" :value="usd(stat.total_quota)" hint="按当前计价汇总" icon="circleStack" />
      </div>

      <div class="dashboard-main-grid">
        <ConsoleSection
          title="路由健康矩阵"
          eyebrow="ROUTE MATRIX"
          description="按当前路由优先级排列；状态合并熔断器、连续失败与最近探测结果。"
          flush
        >
          <template #actions>
            <div class="dashboard-section-summary">
              <span><b class="text-run">{{ healthyChannelCount }}</b> 可用</span>
              <span><b :class="unhealthyChannelCount ? 'text-trip' : 'text-soft'">{{ unhealthyChannelCount }}</b> 异常</span>
              <span>{{ channels.length }} 总计</span>
            </div>
          </template>
          <RouteHealthBar :rows="routeRows" />
        </ConsoleSection>

        <ConsoleSection
          title="异常队列"
          eyebrow="INCIDENT QUEUE"
          description="按严重级别、发生时间与渠道排序的最近失败请求。"
          flush
        >
          <template #actions>
            <RouterLink class="btn btn-sm" to="/logs">全部日志</RouterLink>
          </template>

          <div v-if="anomaliesError" class="dashboard-anomaly-notice">
            <InlineNotice tone="warning" title="异常模块读取失败">
              {{ anomaliesError }}
              <template #actions><RouterLink class="btn btn-sm" to="/logs">前往日志</RouterLink></template>
            </InlineNotice>
          </div>

          <div v-if="anomalyQueue.length" class="anomaly-queue">
            <article v-for="item in anomalyQueue" :key="item.id || item.request_id" class="anomaly-row">
              <div class="anomaly-row-head">
                <StatusBadge :status="anomalyStatus(item).tone" :label="anomalyStatus(item).label" />
                <time>{{ formatTime(item.created_at) }}</time>
              </div>
              <div class="anomaly-channel">
                <strong>{{ item.channel_name || (item.channel_id ? `渠道 ${item.channel_id}` : '未知渠道') }}</strong>
                <span>{{ item.src_model || item.model || '未记录模型' }}</span>
              </div>
              <p>{{ shortText(item.error) }}</p>
              <div class="anomaly-row-foot">
                <span>{{ Number(item.use_time_ms) > 0 ? `${fmt(item.use_time_ms)} ms` : '耗时未知' }}</span>
                <RouterLink to="/logs">定位日志 <ConsoleIcon name="chevronRight" /></RouterLink>
              </div>
            </article>
          </div>
          <div v-else-if="!anomaliesError" class="dashboard-empty">
            <ConsoleIcon name="shield" />
            <strong>近期没有异常记录</strong>
            <span>系统暂未记录失败请求。</span>
          </div>
        </ConsoleSection>
      </div>

      <nav class="dashboard-shortcuts" aria-label="快捷入口">
        <RouterLink to="/channels" class="dashboard-shortcut">
          <ConsoleIcon name="plus" />
          <span><strong>新增渠道</strong><small>进入渠道工作台</small></span>
          <ConsoleIcon name="chevronRight" />
        </RouterLink>
        <RouterLink to="/logs" class="dashboard-shortcut">
          <ConsoleIcon name="logs" />
          <span><strong>查看日志</strong><small>检索请求与错误</small></span>
          <ConsoleIcon name="chevronRight" />
        </RouterLink>
        <RouterLink to="/models" class="dashboard-shortcut">
          <ConsoleIcon name="models" />
          <span><strong>模型目录</strong><small>检查模型供给</small></span>
          <ConsoleIcon name="chevronRight" />
        </RouterLink>
        <RouterLink to="/settings" class="dashboard-shortcut">
          <ConsoleIcon name="settings" />
          <span><strong>系统设置</strong><small>调整运行参数</small></span>
          <ConsoleIcon name="chevronRight" />
        </RouterLink>
      </nav>
    </PageState>
  </div>
</template>

<style scoped>
.dashboard-console { min-width: 0; }
.dashboard-commandbar {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1rem;
  padding-bottom: .9rem;
  border-bottom: 1px solid rgb(var(--color-border));
}
.dashboard-heading {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: .75rem;
}
.dashboard-heading-icon {
  display: grid;
  width: 2.25rem;
  height: 2.25rem;
  flex: 0 0 auto;
  place-items: center;
  border: 1px solid rgb(var(--color-border-strong));
  border-radius: .35rem;
  background: rgb(var(--color-surface-2));
  color: rgb(var(--color-accent-soft));
}
.dashboard-heading-icon svg { width: 1.05rem; height: 1.05rem; }
.dashboard-kicker {
  color: rgb(var(--color-accent-soft));
  font-family: 'Spline Sans Mono', monospace;
  font-size: .5rem;
  letter-spacing: .13em;
}
.dashboard-heading h1 {
  color: rgb(var(--color-text));
  font-size: 1.05rem;
  font-weight: 650;
  line-height: 1.35;
}
.dashboard-heading p:last-child {
  overflow: hidden;
  color: rgb(var(--color-text-secondary));
  font-size: .7rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.dashboard-metric-strip {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  min-width: 0;
  overflow: hidden;
  border: 1px solid rgb(var(--color-border));
  border-radius: .45rem;
  background: rgb(var(--color-surface-1));
}
.dashboard-main-grid {
  display: grid;
  min-width: 0;
  align-items: start;
  gap: 1rem;
  margin-top: 1rem;
}
.dashboard-main-grid > * { min-width: 0; }
.dashboard-section-summary {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: .35rem .75rem;
  color: rgb(var(--color-text-muted));
  font-family: 'Spline Sans Mono', monospace;
  font-size: .6rem;
}
.dashboard-section-summary b { font-weight: 650; }
.dashboard-anomaly-notice { padding: .75rem; border-bottom: 1px solid rgb(var(--color-border)); }
.anomaly-queue { min-width: 0; }
.anomaly-row {
  min-width: 0;
  padding: .78rem .85rem;
  border-bottom: 1px solid rgb(var(--color-border));
  transition: background-color 150ms ease;
}
.anomaly-row:last-child { border-bottom: 0; }
.anomaly-row:hover { background: rgb(var(--color-overlay) / .42); }
.anomaly-row-head,
.anomaly-row-foot,
.anomaly-channel {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: .6rem;
}
.anomaly-row-head time,
.anomaly-row-foot > span {
  flex: 0 0 auto;
  color: rgb(var(--color-text-muted));
  font-family: 'Spline Sans Mono', monospace;
  font-size: .6rem;
}
.anomaly-channel { margin-top: .55rem; justify-content: flex-start; }
.anomaly-channel strong {
  min-width: 0;
  overflow: hidden;
  color: rgb(var(--color-text));
  font-size: .72rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.anomaly-channel span {
  min-width: 0;
  overflow: hidden;
  color: rgb(var(--color-text-muted));
  font-family: 'Spline Sans Mono', monospace;
  font-size: .6rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.anomaly-row > p {
  margin-top: .35rem;
  color: rgb(var(--color-text-secondary));
  font-size: .67rem;
  line-height: 1.05rem;
  overflow-wrap: anywhere;
}
.anomaly-row-foot { margin-top: .5rem; }
.anomaly-row-foot a {
  display: inline-flex;
  align-items: center;
  gap: .15rem;
  color: rgb(var(--color-accent-soft));
  font-size: .65rem;
  font-weight: 600;
}
.anomaly-row-foot a svg { width: .75rem; height: .75rem; }
.dashboard-empty {
  display: flex;
  min-height: 14rem;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: .35rem;
  padding: 1.5rem;
  color: rgb(var(--color-text-muted));
  text-align: center;
}
.dashboard-empty svg { width: 1.4rem; height: 1.4rem; color: rgb(var(--color-success)); }
.dashboard-empty strong { color: rgb(var(--color-text)); font-size: .75rem; }
.dashboard-empty span { font-size: .65rem; }
.dashboard-shortcuts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  min-width: 0;
  gap: .55rem;
  margin-top: 1rem;
}
.dashboard-shortcut {
  display: grid;
  min-width: 0;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: .65rem;
  padding: .72rem .8rem;
  border: 1px solid rgb(var(--color-border));
  border-radius: .4rem;
  background: rgb(var(--color-surface-1));
  transition: border-color 150ms ease, background-color 150ms ease, transform 150ms ease;
}
.dashboard-shortcut:hover {
  border-color: rgb(var(--color-border-strong));
  background: rgb(var(--color-surface-2));
  transform: translateY(-1px);
}
.dashboard-shortcut > svg:first-child { width: 1rem; height: 1rem; color: rgb(var(--color-accent-soft)); }
.dashboard-shortcut > svg:last-child { width: .8rem; height: .8rem; color: rgb(var(--color-text-muted)); }
.dashboard-shortcut span { min-width: 0; }
.dashboard-shortcut strong,
.dashboard-shortcut small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.dashboard-shortcut strong { color: rgb(var(--color-text)); font-size: .7rem; }
.dashboard-shortcut small { margin-top: .1rem; color: rgb(var(--color-text-muted)); font-size: .58rem; }
.dashboard-skeleton { min-width: 0; }
.dashboard-skeleton-metrics,
.dashboard-skeleton-shortcuts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  overflow: hidden;
  border: 1px solid rgb(var(--color-border));
  border-radius: .45rem;
}
.dashboard-skeleton-cell { min-height: 5.4rem; padding: .85rem; border-right: 1px solid rgb(var(--color-border)); border-bottom: 1px solid rgb(var(--color-border)); }
.dashboard-skeleton-cell:nth-child(2n) { border-right: 0; }
.dashboard-skeleton-cell:nth-child(n + 3) { border-bottom: 0; }
.dashboard-skeleton-cell i,
.dashboard-skeleton-cell b,
.dashboard-skeleton-panel i,
.dashboard-skeleton-panel b,
.dashboard-skeleton-shortcuts i {
  display: block;
  border-radius: .2rem;
  background: linear-gradient(100deg, rgb(var(--color-surface-2)) 20%, rgb(var(--color-surface-3)) 42%, rgb(var(--color-surface-2)) 64%);
  background-size: 220% 100%;
  animation: dashboard-skeleton-flow 1.35s ease-in-out infinite;
}
.dashboard-skeleton-cell i { width: 42%; height: .45rem; }
.dashboard-skeleton-cell b { width: 60%; height: 1.2rem; margin-top: .65rem; }
.dashboard-skeleton-cell i:last-child { width: 72%; margin-top: .55rem; }
.dashboard-skeleton-panel { overflow: hidden; border: 1px solid rgb(var(--color-border)); border-radius: .45rem; }
.dashboard-skeleton-panel-head { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: .8rem; border-bottom: 1px solid rgb(var(--color-border)); }
.dashboard-skeleton-panel-head b { width: 8rem; height: .7rem; }
.dashboard-skeleton-panel-head i { width: 4rem; height: .55rem; }
.dashboard-skeleton-row { display: grid; grid-template-columns: 2rem minmax(5rem, 1fr) 3rem 4rem minmax(7rem, 1.3fr); gap: .65rem; padding: .8rem; border-bottom: 1px solid rgb(var(--color-border)); }
.dashboard-skeleton-row:last-child { border-bottom: 0; }
.dashboard-skeleton-row > * { height: .65rem; }
.dashboard-skeleton-alert { padding: .8rem; border-bottom: 1px solid rgb(var(--color-border)); }
.dashboard-skeleton-alert:last-child { border-bottom: 0; }
.dashboard-skeleton-alert i { width: 38%; height: .65rem; }
.dashboard-skeleton-alert b { width: 72%; height: .55rem; margin-top: .6rem; }
.dashboard-skeleton-alert i:last-child { width: 95%; height: .45rem; margin-top: .45rem; }
.dashboard-skeleton-shortcuts { gap: .55rem; margin-top: 1rem; border: 0; }
.dashboard-skeleton-shortcuts i { height: 3.75rem; border: 1px solid rgb(var(--color-border)); }
@keyframes dashboard-skeleton-flow { to { background-position-x: -220%; } }

@media (min-width: 640px) {
  .dashboard-metric-strip,
  .dashboard-skeleton-metrics { grid-template-columns: repeat(4, minmax(0, 1fr)); }
  .dashboard-skeleton-cell { border-bottom: 0; }
  .dashboard-skeleton-cell:nth-child(2n) { border-right: 1px solid rgb(var(--color-border)); }
  .dashboard-skeleton-cell:last-child { border-right: 0; }
  .dashboard-shortcuts,
  .dashboard-skeleton-shortcuts { grid-template-columns: repeat(4, minmax(0, 1fr)); }
}
@media (min-width: 1180px) {
  .dashboard-main-grid { grid-template-columns: minmax(0, 1.45fr) minmax(19rem, .75fr); }
  .anomaly-queue { max-height: 22rem; overflow-y: auto; }
}
@media (max-width: 479px) {
  .dashboard-commandbar { align-items: flex-start; }
  .dashboard-heading-icon { display: none; }
  .dashboard-heading p:last-child { max-width: 13rem; }
  .dashboard-commandbar .btn { width: 2.25rem; padding: 0; font-size: 0; }
  .dashboard-commandbar .btn svg { width: 1rem; height: 1rem; }
  .dashboard-shortcut { gap: .45rem; padding-inline: .65rem; }
  .dashboard-shortcut small { display: none; }
  .dashboard-skeleton-row { grid-template-columns: 2rem minmax(4rem, 1fr) 3rem; }
  .dashboard-skeleton-row > :nth-child(n + 4) { display: none; }
}
@media (prefers-reduced-motion: reduce) {
  .dashboard-skeleton-cell i,
  .dashboard-skeleton-cell b,
  .dashboard-skeleton-panel i,
  .dashboard-skeleton-panel b,
  .dashboard-skeleton-shortcuts i { animation: none; }
  .dashboard-shortcut { transition: none; }
}
</style>

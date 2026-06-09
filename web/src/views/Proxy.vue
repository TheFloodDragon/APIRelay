<template>
  <section class="page-hero">
    <div>
      <p class="eyebrow">Global Proxy</p>
      <h1>全局代理管理</h1>
      <p>统一配置代理开关、故障转移、重试超时与熔断器。所有协议入口共享这一套全局策略。</p>
    </div>
    <div class="page-actions">
      <el-button :icon="Refresh" :loading="loading" @click="loadProxy">刷新</el-button>
    </div>
  </section>

  <div class="metric-grid compact proxy-metrics">
    <div class="metric-card" :class="config?.enabled ? 'accent-green' : 'accent-red'">
      <span class="metric-label">代理状态</span>
      <strong>{{ config?.enabled ? '启用' : '关闭' }}</strong>
      <small>{{ config?.auto_failover_enabled ? '自动故障转移已启用' : '仅按最高优先级渠道' }}</small>
    </div>
    <div class="metric-card accent-purple">
      <span class="metric-label">队列渠道</span>
      <strong>{{ queue.length }}</strong>
      <small>全局故障转移顺序</small>
    </div>
    <div class="metric-card accent-amber">
      <span class="metric-label">熔断打开</span>
      <strong>{{ openCircuitCount }}</strong>
      <small>{{ halfOpenCircuitCount }} 个半开状态</small>
    </div>
  </div>

  <el-alert class="guide-card" type="info" show-icon :closable="false">
    <template #title>
      <span>当前页面只展示全局配置；熔断器与队列均以 channel_id 作为唯一 key。</span>
    </template>
  </el-alert>

  <ProxyToggle :config="config" :saving="savingConfig" @save="saveConfig" />

  <div class="dashboard-grid proxy-grid">
    <FailoverQueueManager
      :channels="channels"
      :queue="queue"
      :loading="loading"
      :saving="savingQueue"
      @save="saveQueue"
    />

    <el-card class="panel-card" shadow="never">
      <template #header>
        <div class="panel-header">
          <span>运行时摘要</span>
          <el-tag :type="config?.enabled ? 'success' : 'info'" effect="plain">
            {{ config?.enabled ? '代理可用' : '代理关闭' }}
          </el-tag>
        </div>
      </template>

      <div class="summary-list">
        <div class="summary-row">
          <span>最大重试次数</span>
          <strong>{{ config?.max_retries ?? '-' }}</strong>
        </div>
        <div class="summary-row">
          <span>非流式超时</span>
          <strong>{{ formatMS(config?.non_streaming_timeout_ms) }}</strong>
        </div>
        <div class="summary-row">
          <span>流式首包超时</span>
          <strong>{{ formatMS(config?.streaming_first_byte_timeout) }}</strong>
        </div>
        <div class="summary-row">
          <span>流式静默超时</span>
          <strong>{{ formatMS(config?.streaming_idle_timeout_ms) }}</strong>
        </div>
        <div class="summary-row">
          <span>失败阈值 / 恢复阈值</span>
          <strong>{{ config?.circuit_failure_threshold ?? '-' }} / {{ config?.circuit_success_threshold ?? '-' }}</strong>
        </div>
        <div class="summary-row">
          <span>熔断打开时间</span>
          <strong>{{ config?.circuit_open_seconds ?? '-' }}s</strong>
        </div>
      </div>
    </el-card>
  </div>

  <CircuitBreakerPanel
    class="circuit-panel"
    :circuits="circuits"
    :loading="loading"
    :resetting-id="resettingID"
    @refresh="loadProxy"
    @reset="handleResetCircuit"
  />
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { useRoute } from 'vue-router'
import { getChannels, type Channel } from '@/api/channels'
import {
  getProxyStatus,
  resetCircuit,
  updateFailoverQueue,
  updateProxyConfig,
  type CircuitStatus,
  type FailoverQueueItem,
  type ProxyConfig
} from '@/api/proxy'
import ProxyToggle from '@/components/proxy/ProxyToggle.vue'
import FailoverQueueManager from '@/components/proxy/FailoverQueueManager.vue'
import CircuitBreakerPanel from '@/components/proxy/CircuitBreakerPanel.vue'

const route = useRoute()
const loading = ref(false)
const savingConfig = ref(false)
const savingQueue = ref(false)
const resettingID = ref<number | null>(null)
const config = ref<ProxyConfig | null>(null)
const queue = ref<FailoverQueueItem[]>([])
const circuits = ref<CircuitStatus[]>([])
const channels = ref<Channel[]>([])
let loadToken = 0

const openCircuitCount = computed(() => circuits.value.filter((item) => item.circuit.state === 'open').length)
const halfOpenCircuitCount = computed(() => circuits.value.filter((item) => item.circuit.state === 'half_open').length)

watch(
  () => route.fullPath,
  () => loadProxy(),
  { immediate: true }
)

async function loadProxy() {
  const currentToken = ++loadToken
  loading.value = true
  try {
    const [statusRes, channelsRes] = await Promise.all([getProxyStatus(), getChannels()])
    if (currentToken === loadToken) {
      const status = statusRes.data.data
      config.value = normalizeConfig(status.config)
      queue.value = normalizeQueue(status.failover_queue)
      circuits.value = normalizeCircuits(status.circuits)
      channels.value = normalizeChannels(channelsRes.data.data)
    }
  } catch (error: any) {
    if (currentToken === loadToken) {
      ElMessage.error(error?.response?.data?.error || '加载代理配置失败')
    }
  } finally {
    if (currentToken === loadToken) {
      loading.value = false
    }
  }
}

async function saveConfig(nextConfig: Partial<ProxyConfig>) {
  savingConfig.value = true
  try {
    const res = await updateProxyConfig(nextConfig)
    config.value = normalizeConfig(res.data.data)
    ElMessage.success('全局代理配置已保存')
    await loadProxy()
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || '保存代理配置失败')
  } finally {
    savingConfig.value = false
  }
}

async function saveQueue(channelIDs: number[]) {
  savingQueue.value = true
  try {
    const res = await updateFailoverQueue(channelIDs)
    queue.value = normalizeQueue(res.data.data)
    ElMessage.success('全局故障转移队列已保存')
    await loadProxy()
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || '保存故障转移队列失败')
  } finally {
    savingQueue.value = false
  }
}

async function handleResetCircuit(channelID: number) {
  try {
    await ElMessageBox.confirm(`确定重置渠道 #${channelID} 的熔断器吗？`, '重置熔断器', { type: 'warning' })
  } catch {
    return
  }

  resettingID.value = channelID
  try {
    await resetCircuit(channelID)
    ElMessage.success('熔断器已重置')
    await loadProxy()
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || '重置熔断器失败')
  } finally {
    resettingID.value = null
  }
}

function normalizeConfig(value?: ProxyConfig | null): ProxyConfig {
  return {
    enabled: value?.enabled ?? true,
    auto_failover_enabled: value?.auto_failover_enabled ?? true,
    max_retries: Number(value?.max_retries ?? 2),
    non_streaming_timeout_ms: Number(value?.non_streaming_timeout_ms ?? 60000),
    streaming_first_byte_timeout: Number(value?.streaming_first_byte_timeout ?? 5000),
    streaming_idle_timeout_ms: Number(value?.streaming_idle_timeout_ms ?? 60000),
    circuit_failure_threshold: Number(value?.circuit_failure_threshold ?? 3),
    circuit_success_threshold: Number(value?.circuit_success_threshold ?? 1),
    circuit_open_seconds: Number(value?.circuit_open_seconds ?? 30),
    id: value?.id,
    created_at: value?.created_at,
    updated_at: value?.updated_at
  }
}

function normalizeQueue(value?: FailoverQueueItem[] | null): FailoverQueueItem[] {
  return (value || [])
    .map((item, index) => ({
      ...item,
      channel_id: Number(item.channel_id),
      position: Number(item.position || index + 1),
      channel: item.channel ? normalizeChannel(item.channel) : undefined
    }))
    .sort((a, b) => a.position - b.position || a.channel_id - b.channel_id)
}

function normalizeCircuits(value?: CircuitStatus[] | null): CircuitStatus[] {
  return (value || []).map((item) => ({
    ...item,
    channel_id: Number(item.channel_id),
    channel: item.channel ? normalizeChannel(item.channel as Channel) : undefined,
    circuit: {
      ...item.circuit,
      channel_id: Number(item.circuit.channel_id),
      consecutive_failures: Number(item.circuit.consecutive_failures || 0),
      consecutive_successes: Number(item.circuit.consecutive_successes || 0),
      failure_threshold: Number(item.circuit.failure_threshold || 0),
      success_threshold: Number(item.circuit.success_threshold || 0),
      open_duration_seconds: Number(item.circuit.open_duration_seconds || 0),
      half_open_permit_in_use: Boolean(item.circuit.half_open_permit_in_use)
    },
    health: item.health
      ? {
          ...item.health,
          channel_id: Number(item.health.channel_id),
          consecutive_failures: Number(item.health.consecutive_failures || 0),
          is_healthy: Boolean(item.health.is_healthy)
        }
      : undefined
  }))
}

function normalizeChannels(value?: Channel[] | null): Channel[] {
  return (value || []).map(normalizeChannel)
}

function normalizeChannel(channel: Channel): Channel {
  return {
    ...channel,
    models: Array.isArray(channel.models) ? channel.models : [],
    priority: Number(channel.priority || 0),
    weight: Number(channel.weight || 1),
    timeout: Number(channel.timeout || 60000),
    max_retries: Number(channel.max_retries || 0),
    enabled: Boolean(channel.enabled),
    health_status: channel.health_status || 'unknown'
  }
}

function formatMS(value?: number) {
  if (!value) return '-'
  if (value >= 1000) return `${Math.round(value / 100) / 10}s`
  return `${value}ms`
}
</script>

<style scoped>
.proxy-metrics .metric-card strong {
  font-size: 28px;
}

.proxy-grid {
  align-items: start;
  margin-bottom: 20px;
}

.summary-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.summary-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 14px;
  border: 1px solid var(--border-light);
  border-radius: 14px;
  background: linear-gradient(135deg, #ffffff, #f8fafc);
}

.summary-row:last-child {
  border-bottom: 0;
}

.summary-row span {
  color: var(--muted);
}

.summary-row strong {
  color: var(--text);
  font-size: 15px;
}

.circuit-panel {
  margin-top: 20px;
}
</style>

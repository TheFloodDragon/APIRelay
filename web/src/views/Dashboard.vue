<script setup>
import { ref, computed, onMounted } from 'vue'
import api from '../api'
import StatDial from '../components/StatDial.vue'
import SignalDot from '../components/SignalDot.vue'
import PriorityBar from '../components/PriorityBar.vue'

const data = ref({ channel_count: 0, stat: {} })
const modelCount = ref(0)
const channels = ref([])
const healthStats = ref({})
const loading = ref(true)

onMounted(async () => {
  try {
    const [dash, models, chs, health] = await Promise.all([
      api.get('/dashboard'),
      api.get('/models').catch(() => []),
      api.get('/channels').catch(() => []),
      api.get('/settings/health-stats').catch(() => ({})),
    ])
    data.value = dash
    modelCount.value = (models || []).length
    channels.value = chs || []
    healthStats.value = health || {}
  } finally {
    loading.value = false
  }
})

// ===== 仪表读数 =====
const stat = computed(() => data.value.stat || {})
const fmt = (n) => (n || 0).toLocaleString()
const usd = (micro) => '$' + ((micro || 0) / 1_000_000).toFixed(4)

const dials = computed(() => [
  { label: 'CHANNELS', value: fmt(data.value.channel_count), accent: true },
  { label: 'MODELS', value: fmt(modelCount.value), accent: true },
  { label: 'CIRCUIT OPEN', value: fmt(healthStats.value.open || 0), state: 'down' },
  { label: 'REQUESTS · 7D', value: fmt(stat.value.total_requests) },
  { label: 'PROMPT TK', value: fmt(stat.value.total_prompt_tokens), unit: 'tk' },
  { label: 'SPEND · 7D', value: usd(stat.value.total_quota) },
])

// ===== 路由概览：按优先级排序的供应商信号条 =====
const routeRows = computed(() => {
  return [...channels.value]
    .map(ch => ({
      ...ch,
      modelN: modelCountOf(ch),
      state: stateOf(ch),
    }))
    .sort((a, b) => (a.priority - b.priority) || (a.id - b.id))
})

function modelCountOf(ch) {
  if (Array.isArray(ch._models)) return ch._models.length
  return (ch.models || '').split(',').map(s => s.trim()).filter(Boolean).length
}
function stateOf(ch) {
  if (ch.status !== 1) return 'down'                    // 禁用
  if (ch.cooldown_until && ch.cooldown_until > Date.now()) return 'warn' // 冷却
  return 'online'
}
const stateLabel = { online: '在线', warn: '冷却', down: '禁用' }

const onlineCount = computed(() => routeRows.value.filter(r => r.state === 'online').length)
</script>

<template>
  <div>
    <div class="mb-5">
      <h2 class="page-title">信号总览</h2>
      <p class="page-subtitle">流量路由与近 7 天读数</p>
    </div>

    <!-- ===== 仪表读数 ===== -->
    <div v-if="loading" class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3 mb-6">
      <div v-for="i in 6" :key="i" class="panel p-4">
        <div class="skeleton h-3 w-16 mb-3"></div>
        <div class="skeleton h-6 w-20"></div>
      </div>
    </div>
    <div v-else class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3 mb-6">
      <StatDial v-for="d in dials" :key="d.label" :label="d.label" :value="d.value" :unit="d.unit" :accent="d.accent" :status="d.state" />
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
      <!-- ===== 路由概览（签名区） ===== -->
      <div class="lg:col-span-2 panel">
        <div class="px-4 h-11 flex items-center justify-between border-b border-line">
          <div class="flex items-center gap-2">
            <span class="font-mono text-sm font-medium text-t1">路由概览</span>
            <span class="tick">PRIORITY ROUTE</span>
          </div>
          <span class="font-mono text-2xs text-t3">{{ onlineCount }}/{{ routeRows.length }} 在线</span>
        </div>

        <div v-if="loading" class="p-4 space-y-2">
          <div v-for="i in 4" :key="i" class="skeleton h-10"></div>
        </div>

        <div v-else-if="routeRows.length" class="divide-y divide-line">
          <div v-for="(r, i) in routeRows" :key="r.id"
            class="flex items-center gap-3 px-4 py-2.5 hover:bg-[rgb(var(--c-signal)/0.04)] transition-colors">
            <!-- 优先级刻度 -->
            <span class="font-mono text-2xs text-t3 w-5 text-right">{{ String(i + 1).padStart(2, '0') }}</span>
            <PriorityBar :level="i" :total="routeRows.length" />
            <!-- 名称 + 状态 -->
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <SignalDot :status="r.state" />
                <span class="font-medium text-sm text-t1 truncate">{{ r.name }}</span>
                <span class="font-mono text-2xs text-t3">#{{ r.id }}</span>
              </div>
            </div>
            <!-- 模型数 -->
            <div class="text-right shrink-0">
              <div class="font-mono text-sm text-t1">{{ r.modelN }}</div>
              <div class="tick">models</div>
            </div>
            <!-- 权重 -->
            <div class="text-right shrink-0 w-12 hidden sm:block">
              <div class="font-mono text-sm text-t2">×{{ r.weight }}</div>
              <div class="tick">weight</div>
            </div>
            <!-- 状态标签 -->
            <span class="shrink-0 w-12 text-right">
              <span class="badge" :class="{ 'badge-online': r.state === 'online', 'badge-warn': r.state === 'warn', 'badge-down': r.state === 'down' }">{{ stateLabel[r.state] }}</span>
            </span>
          </div>
        </div>

        <div v-else class="empty-state">
          <span class="font-mono text-2xs">暂无供应商节点</span>
          <router-link to="/channels" class="btn-secondary btn-sm mt-1">配置路由表</router-link>
        </div>
      </div>

      <!-- ===== 系统状态 + 快捷 ===== -->
      <div class="space-y-4">
        <div class="panel">
          <div class="px-4 h-11 flex items-center border-b border-line">
            <span class="font-mono text-sm font-medium text-t1">系统状态</span>
          </div>
          <div class="p-4 space-y-3">
            <div class="flex items-center justify-between">
              <span class="tick">STATUS</span>
              <span class="flex items-center gap-1.5"><SignalDot status="online" /><span class="font-mono text-xs text-t1">运行中</span></span>
            </div>
            <div class="flex items-center justify-between">
              <span class="tick">VERSION</span>
              <span class="font-mono text-xs text-t1">v0.1.0</span>
            </div>
            <div class="flex items-center justify-between">
              <span class="tick">PROTOCOLS</span>
              <span class="font-mono text-2xs text-t2">OpenAI · Anthropic · Responses</span>
            </div>
            <div class="flex items-center justify-between">
              <span class="tick">GROUP</span>
              <span class="font-mono text-xs text-t1">default</span>
            </div>
          </div>
        </div>

        <div class="panel">
          <div class="px-4 h-11 flex items-center border-b border-line">
            <span class="font-mono text-sm font-medium text-t1">快捷操作</span>
          </div>
          <div class="p-3 grid grid-cols-2 gap-2">
            <router-link to="/channels" class="btn-secondary btn-sm justify-start">路由表</router-link>
            <router-link to="/models" class="btn-secondary btn-sm justify-start">信号矩阵</router-link>
            <router-link to="/tokens" class="btn-secondary btn-sm justify-start">新建令牌</router-link>
            <router-link to="/settings" class="btn-secondary btn-sm justify-start">协议规则</router-link>
            <router-link to="/logs" class="btn-secondary btn-sm justify-start col-span-2">信号流水</router-link>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'

const data = ref({ channel_count: 0, stat: {} })
const modelCount = ref(0)
const loading = ref(true)

onMounted(async () => {
  try {
    const [dash, models] = await Promise.all([
      api.get('/dashboard'),
      api.get('/models').catch(() => []),
    ])
    data.value = dash
    modelCount.value = (models || []).length
  } finally {
    loading.value = false
  }
})

const stats = [
  { label: '供应商', key: 'channel_count', icon: '🔗', tint: 'brand' },
  { label: '聚合模型', key: '_models', icon: '🧩', tint: 'purple' },
  { label: '总请求数', key: 'total_requests', icon: '📊', tint: 'blue', isStat: true },
  { label: 'Prompt Tokens', key: 'total_prompt_tokens', icon: '📝', tint: 'green', isStat: true },
  { label: 'Completion Tokens', key: 'total_completion_tokens', icon: '✨', tint: 'amber', isStat: true },
]

const tintMap = {
  brand: 'bg-brand-50 text-brand-600 dark:bg-brand-500/15 dark:text-brand-400',
  blue: 'bg-blue-50 text-blue-600 dark:bg-blue-500/15 dark:text-blue-400',
  green: 'bg-green-50 text-green-600 dark:bg-green-500/15 dark:text-green-400',
  purple: 'bg-purple-50 text-purple-600 dark:bg-purple-500/15 dark:text-purple-400',
  amber: 'bg-amber-50 text-amber-600 dark:bg-amber-500/15 dark:text-amber-400',
}

function valueOf(s) {
  if (s.key === '_models') return modelCount.value
  return s.isStat ? data.value.stat?.[s.key] : data.value[s.key]
}
const formatNumber = (n) => (n || 0).toLocaleString()
</script>

<template>
  <div>
    <div class="mb-6">
      <h2 class="page-title">仪表盘</h2>
      <p class="page-subtitle">运行概览与快捷操作</p>
    </div>

    <!-- 统计卡片 -->
    <div v-if="loading" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-4 mb-8">
      <div v-for="i in 5" :key="i" class="card-flat">
        <div class="skeleton h-4 w-20 mb-3"></div>
        <div class="skeleton h-8 w-24"></div>
      </div>
    </div>

    <div v-else class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-4 mb-8">
      <div v-for="s in stats" :key="s.key" class="card group cursor-default !p-5">
        <div class="flex items-center justify-between mb-3">
          <div :class="['w-10 h-10 rounded-xl flex items-center justify-center text-lg transition-transform group-hover:scale-110', tintMap[s.tint]]">
            {{ s.icon }}
          </div>
        </div>
        <div class="text-xs text-ink-500 mb-1">{{ s.label }}</div>
        <div class="text-2xl font-bold text-ink-900 dark:text-ink-50">{{ formatNumber(valueOf(s)) }}</div>
      </div>
    </div>

    <!-- 快捷操作 -->
    <div class="card-flat mb-6">
      <h3 class="text-base font-semibold text-ink-900 dark:text-ink-100 mb-4">快捷操作</h3>
      <div class="flex flex-wrap gap-3">
        <router-link to="/channels" class="btn-secondary">🔗 管理供应商</router-link>
        <router-link to="/models" class="btn-secondary">🧩 查看模型</router-link>
        <router-link to="/tokens" class="btn-secondary">🔑 创建令牌</router-link>
        <router-link to="/settings" class="btn-secondary">🧭 协议规则</router-link>
        <router-link to="/logs" class="btn-secondary">📝 查看日志</router-link>
      </div>
    </div>

    <!-- 系统信息 -->
    <div class="card-flat">
      <h3 class="text-base font-semibold text-ink-900 dark:text-ink-100 mb-4">系统信息</h3>
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
        <div>
          <div class="text-ink-500 mb-1">版本</div>
          <div class="font-medium text-ink-900 dark:text-ink-100">v0.1.0</div>
        </div>
        <div>
          <div class="text-ink-500 mb-1">状态</div>
          <div class="flex items-center gap-2">
            <span class="w-2 h-2 bg-green-500 rounded-full animate-pulse-ring"></span>
            <span class="font-medium text-green-600 dark:text-green-400">运行中</span>
          </div>
        </div>
        <div>
          <div class="text-ink-500 mb-1">协议支持</div>
          <div class="font-medium text-ink-900 dark:text-ink-100">OpenAI · Anthropic · Responses</div>
        </div>
        <div>
          <div class="text-ink-500 mb-1">默认分组</div>
          <div class="font-medium text-ink-900 dark:text-ink-100">default</div>
        </div>
      </div>
    </div>
  </div>
</template>

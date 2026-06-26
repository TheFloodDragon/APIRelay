<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'

const data = ref({ channel_count: 0, stat: {} })
const loading = ref(true)

onMounted(async () => {
  try {
    data.value = await api.get('/dashboard')
  } finally {
    loading.value = false
  }
})

const stats = [
  { label: '渠道数', key: 'channel_count', icon: '🔗', color: 'brand' },
  { label: '总请求数', key: 'total_requests', icon: '📊', color: 'blue', isStat: true },
  { label: 'Prompt Tokens', key: 'total_prompt_tokens', icon: '📝', color: 'green', isStat: true },
  { label: 'Completion Tokens', key: 'total_completion_tokens', icon: '✨', color: 'purple', isStat: true },
]

const formatNumber = (n) => (n || 0).toLocaleString()
</script>

<template>
  <div>
    <h2 class="text-2xl font-bold text-gray-900 mb-6">仪表盘</h2>

    <!-- 统计卡片 -->
    <div v-if="loading" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
      <div v-for="i in 4" :key="i" class="card-flat">
        <div class="skeleton h-4 w-20 mb-3"></div>
        <div class="skeleton h-8 w-24"></div>
      </div>
    </div>

    <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
      <div v-for="s in stats" :key="s.key" class="card group cursor-default">
        <div class="flex items-start justify-between mb-3">
          <div class="text-3xl">{{ s.icon }}</div>
          <div :class="['px-2 py-1 rounded-lg text-xs font-medium', 
            s.color === 'brand' ? 'bg-brand-50 text-brand-600' :
            s.color === 'blue' ? 'bg-blue-50 text-blue-600' :
            s.color === 'green' ? 'bg-green-50 text-green-600' :
            'bg-purple-50 text-purple-600']">
            实时
          </div>
        </div>
        <div class="text-xs text-gray-500 mb-1">{{ s.label }}</div>
        <div class="text-2xl font-bold text-gray-900">
          {{ formatNumber(s.isStat ? data.stat?.[s.key] : data[s.key]) }}
        </div>
      </div>
    </div>

    <!-- 快捷操作 -->
    <div class="card-flat mb-8">
      <h3 class="text-base font-semibold text-gray-900 mb-4">快捷操作</h3>
      <div class="flex flex-wrap gap-3">
        <router-link to="/channels" class="btn-secondary">
          <span>🔗</span>
          <span>管理渠道</span>
        </router-link>
        <router-link to="/tokens" class="btn-secondary">
          <span>🔑</span>
          <span>创建令牌</span>
        </router-link>
        <router-link to="/logs" class="btn-secondary">
          <span>📝</span>
          <span>查看日志</span>
        </router-link>
      </div>
    </div>

    <!-- 系统信息 -->
    <div class="card-flat">
      <h3 class="text-base font-semibold text-gray-900 mb-4">系统信息</h3>
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
        <div>
          <div class="text-gray-500 mb-1">版本</div>
          <div class="font-medium text-gray-900">v0.1.0</div>
        </div>
        <div>
          <div class="text-gray-500 mb-1">状态</div>
          <div class="flex items-center gap-2">
            <span class="w-2 h-2 bg-green-500 rounded-full"></span>
            <span class="font-medium text-green-600">运行中</span>
          </div>
        </div>
        <div>
          <div class="text-gray-500 mb-1">协议支持</div>
          <div class="font-medium text-gray-900">OpenAI, Anthropic, Gemini</div>
        </div>
        <div>
          <div class="text-gray-500 mb-1">默认分组</div>
          <div class="font-medium text-gray-900">default</div>
        </div>
      </div>
    </div>
  </div>
</template>

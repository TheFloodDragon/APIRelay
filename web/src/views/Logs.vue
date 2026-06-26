<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'

const logs = ref([])
const page = ref(1)
const pageSize = 20
const total = ref(0)
const loading = ref(false)

function fmt(ms) {
  if (!ms) return '-'
  return new Date(ms).toLocaleString()
}

// 微美元 -> 美元（费用列）
function cost(micro) {
  if (!micro) return '-'
  return '$' + (micro / 1_000_000).toFixed(4)
}

function typeName(t) {
  return { 1: '消费', 2: '错误', 3: '管理', 4: '系统' }[t] || '其他'
}

function typeClass(t) {
  if (t === 1) return 'badge-success'
  if (t === 2) return 'badge-error'
  return 'badge-neutral'
}

function protocolBadge(proto) {
  const map = {
    openai: 'badge-info',
    anthropic: 'badge-warning',
    gemini: 'badge-success',
  }
  return map[proto?.toLowerCase()] || 'badge-neutral'
}

async function load() {
  loading.value = true
  try {
    const data = await api.get('/logs', { params: { page: page.value, page_size: pageSize } })
    logs.value = data.items || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="flex items-center justify-between mb-6">
      <div>
        <h2 class="page-title">调用日志</h2>
        <p class="page-subtitle">查看 API 调用历史与统计</p>
      </div>
      <button @click="load" class="btn-secondary">🔄 刷新</button>
    </div>

    <div class="table-wrapper">
      <table class="table">
        <thead>
          <tr>
            <th>时间</th>
            <th>类型</th>
            <th>渠道</th>
            <th>对外/上游</th>
            <th>模型</th>
            <th>流</th>
            <th>Tokens</th>
            <th>费用</th>
            <th>耗时</th>
            <th>首字</th>
            <th>状态</th>
            <th>错误</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="l in logs" :key="l.id">
            <td class="whitespace-nowrap text-xs">{{ fmt(l.created_at) }}</td>
            <td><span :class="typeClass(l.type)">{{ typeName(l.type) }}</span></td>
            <td class="text-ink-600 dark:text-ink-300">{{ l.channel_name || l.channel_id || '-' }}</td>
            <td>
              <div class="flex gap-1">
                <span :class="protocolBadge(l.endpoint_type)">{{ l.endpoint_type }}</span>
                <span v-if="l.api_type" :class="protocolBadge(l.api_type)">{{ l.api_type }}</span>
              </div>
            </td>
            <td class="whitespace-nowrap text-xs font-mono text-ink-600 dark:text-ink-300">
              {{ l.src_model }}
              <span v-if="l.mapped_model && l.mapped_model !== l.src_model" class="text-ink-400">
                → {{ l.mapped_model }}
              </span>
            </td>
            <td class="text-center">{{ l.is_stream ? '✓' : '' }}</td>
            <td class="whitespace-nowrap text-xs text-ink-500">{{ l.prompt_tokens }}/{{ l.completion_tokens }}</td>
            <td class="whitespace-nowrap text-xs font-mono text-ink-600 dark:text-ink-300">{{ cost(l.quota) }}</td>
            <td class="text-ink-500">{{ l.use_time_ms }}ms</td>
            <td class="text-ink-500">{{ l.first_byte_ms ? l.first_byte_ms + 'ms' : '-' }}</td>
            <td>
              <span v-if="l.status >= 400" class="badge-error">{{ l.status }}</span>
              <span v-else class="badge-success">{{ l.status }}</span>
            </td>
            <td class="text-red-500 text-xs max-w-[160px] truncate" :title="l.error">{{ l.error }}</td>
          </tr>
          <tr v-if="!logs.length">
            <td colspan="12" class="empty-state">
              <div class="text-5xl mb-3 opacity-60">📝</div>
              <div>暂无日志</div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 分页 -->
    <div class="pagination mt-6">
      <button class="page-btn" :disabled="page<=1" @click="page--;load()">上一页</button>
      <span class="text-gray-600">第 {{ page }} 页 / 共 {{ total }} 条</span>
      <button class="page-btn" :disabled="page*pageSize>=total" @click="page++;load()">下一页</button>
    </div>
  </div>
</template>

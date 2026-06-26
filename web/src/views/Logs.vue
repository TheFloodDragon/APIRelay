<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'

const logs = ref([])
const page = ref(1)
const pageSize = 20
const total = ref(0)
const loading = ref(false)
const detail = ref(null) // 当前查看详情的日志

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

function openDetail(l) {
  detail.value = l
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
            <td class="text-center">
              <span v-if="l.is_stream" class="badge-info text-[10px] !px-1.5 !py-0.5">流</span>
              <span v-else class="text-ink-300 dark:text-ink-600">—</span>
            </td>
            <td class="whitespace-nowrap text-xs text-ink-500">{{ l.prompt_tokens }}/{{ l.completion_tokens }}</td>
            <td class="whitespace-nowrap text-xs font-mono text-ink-600 dark:text-ink-300">{{ cost(l.quota) }}</td>
            <td class="text-ink-500">{{ l.use_time_ms }}ms</td>
            <td class="text-ink-500">{{ l.first_byte_ms ? l.first_byte_ms + 'ms' : '-' }}</td>
            <td>
              <span v-if="l.status >= 400" class="badge-error">{{ l.status }}</span>
              <span v-else class="badge-success">{{ l.status }}</span>
            </td>
            <td class="max-w-[200px]">
              <div class="flex items-center gap-2">
                <span class="text-red-500 text-xs truncate flex-1" :title="l.error">{{ l.error || '-' }}</span>
                <button v-if="l.error" class="btn-ghost btn-sm shrink-0 !px-2" @click="openDetail(l)">详情</button>
              </div>
            </td>
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
      <span class="text-ink-500 dark:text-ink-400">第 {{ page }} 页 / 共 {{ total }} 条</span>
      <button class="page-btn" :disabled="page*pageSize>=total" @click="page++;load()">下一页</button>
    </div>

    <!-- 日志详情弹窗 -->
    <div v-if="detail" class="modal-backdrop" @click.self="detail=null">
      <div class="modal max-w-2xl">
        <div class="flex items-center justify-between mb-5 pb-4 border-b border-ink-100 dark:border-ink-800">
          <h3 class="text-lg font-semibold text-ink-900 dark:text-ink-100">日志详情</h3>
          <button @click="detail=null" class="text-ink-400 hover:text-ink-600 dark:hover:text-ink-200 text-2xl leading-none">&times;</button>
        </div>
        <div class="space-y-3 text-sm">
          <div class="grid grid-cols-2 gap-3">
            <div><span class="text-ink-400">时间</span><div class="font-medium">{{ fmt(detail.created_at) }}</div></div>
            <div><span class="text-ink-400">类型</span><div><span :class="typeClass(detail.type)">{{ typeName(detail.type) }}</span></div></div>
            <div><span class="text-ink-400">供应商</span><div class="font-medium">{{ detail.channel_name || (detail.channel_id ? '#'+detail.channel_id : '未选中渠道') }}</div></div>
            <div><span class="text-ink-400">分组</span><div class="font-medium">{{ detail.group || '-' }}</div></div>
            <div><span class="text-ink-400">对外协议</span><div><span :class="protocolBadge(detail.endpoint_type)">{{ detail.endpoint_type || '-' }}</span></div></div>
            <div><span class="text-ink-400">上游协议</span><div><span v-if="detail.api_type" :class="protocolBadge(detail.api_type)">{{ detail.api_type }}</span><span v-else>-</span></div></div>
            <div><span class="text-ink-400">请求模型</span><div class="font-mono text-xs">{{ detail.src_model || '-' }}</div></div>
            <div><span class="text-ink-400">上游模型</span><div class="font-mono text-xs">{{ detail.mapped_model || '-' }}</div></div>
            <div><span class="text-ink-400">状态码</span><div class="font-medium">{{ detail.status }}</div></div>
            <div><span class="text-ink-400">令牌</span><div class="font-medium">{{ detail.token_name || '-' }}</div></div>
            <div><span class="text-ink-400">耗时</span><div class="font-medium">{{ detail.use_time_ms }}ms</div></div>
            <div><span class="text-ink-400">费用</span><div class="font-mono text-xs">{{ cost(detail.quota) }}</div></div>
          </div>
          <div v-if="detail.request_id || detail.upstream_request_id" class="grid grid-cols-1 gap-2">
            <div v-if="detail.request_id"><span class="text-ink-400">请求 ID</span><div class="font-mono text-xs break-all">{{ detail.request_id }}</div></div>
            <div v-if="detail.upstream_request_id"><span class="text-ink-400">上游请求 ID</span><div class="font-mono text-xs break-all">{{ detail.upstream_request_id }}</div></div>
          </div>
          <div v-if="detail.error">
            <span class="text-ink-400">错误信息</span>
            <pre class="mt-1 p-3 rounded-xl bg-red-50 dark:bg-red-500/10 border border-red-200 dark:border-red-500/30 text-red-600 dark:text-red-400 text-xs whitespace-pre-wrap break-all max-h-64 overflow-auto">{{ detail.error }}</pre>
          </div>
        </div>
        <div class="flex justify-end mt-5 pt-4 border-t border-ink-100 dark:border-ink-800">
          <button class="btn-secondary" @click="detail=null">关闭</button>
        </div>
      </div>
    </div>
  </div>
</template>

<template>
  <div>
    <div class="flex items-center justify-between mb-4">
      <h2 class="text-lg font-semibold">调用日志</h2>
      <button class="px-3 py-1.5 text-sm rounded border" @click="load">刷新</button>
    </div>

    <div class="overflow-x-auto bg-white rounded-lg shadow">
      <table class="w-full text-xs">
        <thead class="bg-gray-100 text-gray-600 text-left">
          <tr>
            <th class="p-2">时间</th>
            <th class="p-2">类型</th>
            <th class="p-2">渠道</th>
            <th class="p-2">对外协议</th>
            <th class="p-2">上游协议</th>
            <th class="p-2">模型(请求→上游)</th>
            <th class="p-2">流</th>
            <th class="p-2">Tokens</th>
            <th class="p-2">耗时</th>
            <th class="p-2">首字</th>
            <th class="p-2">状态</th>
            <th class="p-2">错误</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="l in logs" :key="l.id" class="border-t hover:bg-gray-50">
            <td class="p-2 whitespace-nowrap">{{ fmt(l.created_at) }}</td>
            <td class="p-2"><span :class="typeClass(l.type)">{{ typeName(l.type) }}</span></td>
            <td class="p-2">{{ l.channel_name || l.channel_id || '-' }}</td>
            <td class="p-2">{{ l.endpoint_type }}</td>
            <td class="p-2">{{ l.api_type || '-' }}</td>
            <td class="p-2 whitespace-nowrap">{{ l.src_model }}<span v-if="l.mapped_model && l.mapped_model !== l.src_model" class="text-gray-400"> → {{ l.mapped_model }}</span></td>
            <td class="p-2">{{ l.is_stream ? '✓' : '' }}</td>
            <td class="p-2 whitespace-nowrap">{{ l.prompt_tokens }}/{{ l.completion_tokens }}</td>
            <td class="p-2">{{ l.use_time_ms }}ms</td>
            <td class="p-2">{{ l.first_byte_ms ? l.first_byte_ms + 'ms' : '-' }}</td>
            <td class="p-2"><span :class="l.status >= 400 ? 'text-red-500' : 'text-green-600'">{{ l.status }}</span></td>
            <td class="p-2 text-red-500 max-w-[180px] truncate" :title="l.error">{{ l.error }}</td>
          </tr>
          <tr v-if="!logs.length"><td colspan="12" class="p-6 text-center text-gray-400">暂无日志</td></tr>
        </tbody>
      </table>
    </div>

    <div class="flex items-center justify-end gap-3 mt-4 text-sm">
      <button class="px-3 py-1 rounded border disabled:opacity-40" :disabled="page<=1" @click="page--;load()">上一页</button>
      <span>第 {{ page }} 页 / 共 {{ total }} 条</span>
      <button class="px-3 py-1 rounded border disabled:opacity-40" :disabled="page*pageSize>=total" @click="page++;load()">下一页</button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'

const logs = ref([])
const page = ref(1)
const pageSize = 20
const total = ref(0)

function fmt(ms) {
  if (!ms) return '-'
  return new Date(ms).toLocaleString()
}
function typeName(t) {
  return { 1: '消费', 2: '错误', 3: '管理', 4: '系统' }[t] || '其他'
}
function typeClass(t) {
  return t === 2 ? 'text-red-500' : t === 1 ? 'text-green-600' : 'text-gray-500'
}

async function load() {
  const data = await api.get('/logs', { params: { page: page.value, page_size: pageSize } })
  logs.value = data.items || []
  total.value = data.total || 0
}
onMounted(load)
</script>

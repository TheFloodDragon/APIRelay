<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'
import ProtocolTag from '../components/ProtocolTag.vue'
import SignalDot from '../components/SignalDot.vue'

const logs = ref([])
const page = ref(1)
const pageSize = 20
const total = ref(0)
const loading = ref(false)
const expandedId = ref(null) // 行内展开的日志 id

function fmt(ms) {
  if (!ms) return '-'
  const d = new Date(ms)
  const p = (n) => String(n).padStart(2, '0')
  return `${p(d.getMonth() + 1)}/${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

const cost = (micro) => micro ? '$' + (micro / 1_000_000).toFixed(4) : '-'

function typeName(t) {
  return { 1: '消费', 2: '错误', 3: '管理', 4: '系统' }[t] || '其他'
}
function typeBadge(t) {
  if (t === 1) return 'badge-online'
  if (t === 2) return 'badge-down'
  return 'badge-neutral'
}
function statusState(s) {
  if (!s) return 'idle'
  if (s >= 500) return 'down'
  if (s >= 400) return 'warn'
  return 'online'
}

function toggle(l) {
  expandedId.value = expandedId.value === l.id ? null : l.id
}

async function load() {
  loading.value = true
  try {
    const data = await api.get('/logs', { params: { page: page.value, page_size: pageSize } })
    logs.value = data.items || []
    total.value = data.total || 0
    expandedId.value = null
  } finally {
    loading.value = false
  }
}

function prev() { if (page.value > 1) { page.value--; load() } }
function next() { if (page.value * pageSize < total.value) { page.value++; load() } }

onMounted(load)
</script>

<template>
  <div>
    <div class="flex items-center justify-between mb-5">
      <div>
        <h2 class="page-title">信号流水</h2>
        <p class="page-subtitle">API 调用历史 · 点击行展开详情</p>
      </div>
      <button @click="load" class="btn-secondary">刷新</button>
    </div>

    <div class="panel overflow-hidden">
      <div class="overflow-x-auto">
        <table class="dtable">
          <thead>
            <tr>
              <th class="w-32">时间</th>
              <th>类型</th>
              <th>节点</th>
              <th>协议</th>
              <th>模型</th>
              <th class="text-center">流</th>
              <th class="text-right">Tokens</th>
              <th class="text-right">费用</th>
              <th class="text-right">耗时</th>
              <th class="text-right">首字</th>
              <th class="text-center">状态</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="l in logs" :key="l.id">
              <tr class="cursor-pointer" @click="toggle(l)">
                <td class="whitespace-nowrap font-mono text-2xs text-t2">{{ fmt(l.created_at) }}</td>
                <td><span class="badge" :class="typeBadge(l.type)">{{ typeName(l.type) }}</span></td>
                <td class="text-t2 text-xs truncate max-w-[120px]">{{ l.channel_name || (l.channel_id ? '#'+l.channel_id : '-') }}</td>
                <td>
                  <div class="flex gap-1">
                    <ProtocolTag :protocol="l.endpoint_type" />
                    <ProtocolTag v-if="l.api_type && l.api_type !== l.endpoint_type" :protocol="l.api_type" />
                  </div>
                </td>
                <td class="whitespace-nowrap font-mono text-2xs text-t1">
                  {{ l.src_model }}
                  <span v-if="l.mapped_model && l.mapped_model !== l.src_model" class="text-t3">→ {{ l.mapped_model }}</span>
                </td>
                <td class="text-center">
                  <span v-if="l.is_stream" class="badge badge-signal !px-1.5">流</span>
                  <span v-else class="text-t3">—</span>
                </td>
                <td class="text-right whitespace-nowrap font-mono text-2xs text-t2 tabular-nums">{{ l.prompt_tokens }}<span class="text-t3">/</span>{{ l.completion_tokens }}</td>
                <td class="text-right whitespace-nowrap font-mono text-2xs text-t1 tabular-nums">{{ cost(l.quota) }}</td>
                <td class="text-right font-mono text-2xs text-t2 tabular-nums">{{ l.use_time_ms }}ms</td>
                <td class="text-right font-mono text-2xs text-t3 tabular-nums">{{ l.first_byte_ms ? l.first_byte_ms + 'ms' : '-' }}</td>
                <td class="text-center">
                  <span class="font-mono text-2xs tabular-nums" :class="l.status >= 400 ? 'text-[rgb(var(--c-down))]' : 'text-t2'">{{ l.status }}</span>
                </td>
              </tr>
              <!-- 行内展开详情 -->
              <tr v-if="expandedId === l.id">
                <td colspan="11" class="!p-0 border-b border-line">
                  <div class="bg-panel-2 px-4 py-3 animate-fade-in">
                    <div class="grid grid-cols-2 md:grid-cols-4 gap-x-6 gap-y-2 text-xs">
                      <div><span class="tick">GROUP</span><div class="font-mono text-t1 mt-0.5">{{ l.group || '-' }}</div></div>
                      <div><span class="tick">TOKEN</span><div class="font-mono text-t1 mt-0.5">{{ l.token_name || '-' }}</div></div>
                      <div><span class="tick">STATUS</span><div class="font-mono mt-0.5 flex items-center gap-1.5"><SignalDot :status="statusState(l.status)" :size="6" :pulse="false" />{{ l.status }}</div></div>
                      <div><span class="tick">TOTAL TK</span><div class="font-mono text-t1 mt-0.5">{{ (l.prompt_tokens || 0) + (l.completion_tokens || 0) }}</div></div>
                      <div v-if="l.request_id" class="col-span-2 md:col-span-2"><span class="tick">REQUEST ID</span><div class="font-mono text-t2 mt-0.5 break-all">{{ l.request_id }}</div></div>
                      <div v-if="l.upstream_request_id" class="col-span-2 md:col-span-2"><span class="tick">UPSTREAM ID</span><div class="font-mono text-t2 mt-0.5 break-all">{{ l.upstream_request_id }}</div></div>
                    </div>
                    <div v-if="l.error" class="mt-3">
                      <span class="tick">ERROR</span>
                      <pre class="mt-1 p-2.5 rounded-md border text-2xs whitespace-pre-wrap break-all max-h-56 overflow-auto font-mono text-[rgb(var(--c-down))] border-[rgb(var(--c-down)/0.28)] bg-[rgb(var(--c-down)/0.06)]">{{ l.error }}</pre>
                    </div>
                  </div>
                </td>
              </tr>
            </template>
            <tr v-if="!logs.length && !loading">
              <td colspan="11" class="empty-state">
                <span class="font-mono text-3xl text-t3">∅</span>
                <span>暂无日志</span>
              </td>
            </tr>
            <tr v-if="loading">
              <td colspan="11" class="p-4">
                <div class="space-y-2">
                  <div v-for="i in 6" :key="i" class="skeleton h-6"></div>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- 分页 -->
    <div class="flex items-center justify-end gap-3 mt-4 font-mono text-2xs text-t2">
      <button class="page-btn" :disabled="page<=1" @click="prev">上一页</button>
      <span>PAGE {{ page }} · {{ total }} 条</span>
      <button class="page-btn" :disabled="page*pageSize>=total" @click="next">下一页</button>
    </div>
  </div>
</template>


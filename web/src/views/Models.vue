<template>
  <div>
    <div class="flex items-center justify-between mb-5 gap-4">
      <div>
        <h2 class="page-title">模型</h2>
        <p class="page-subtitle">按模型显示名聚合的上游可达性与协议映射</p>
      </div>
      <div class="flex items-center gap-2">
        <div class="relative hidden sm:block">
          <svg viewBox="0 0 24 24" class="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-t3" fill="currentColor"><path d="M10 2a8 8 0 105.3 14l5.4 5.4 1.4-1.4-5.4-5.4A8 8 0 0010 2zm0 2a6 6 0 110 12 6 6 0 010-12z"/></svg>
          <input v-model="q" class="input !pl-9 w-64 font-mono" placeholder="search model..." />
        </div>
        <button @click="load" class="btn-secondary">刷新</button>
      </div>
    </div>

    <!-- 概览 -->
    <div class="grid grid-cols-2 md:grid-cols-3 gap-3 mb-5">
      <StatDial label="聚合模型" :value="models.length" accent />
      <StatDial label="可路由" :value="enabledModels" status="online" />
      <StatDial label="渠道绑定" :value="totalBindings" />
    </div>

    <div v-if="loading" class="space-y-2">
      <div v-for="i in 5" :key="i" class="skeleton h-16 rounded-lg"></div>
    </div>

    <div v-else class="panel overflow-hidden">
      <div class="h-10 px-4 border-b border-line flex items-center justify-between">
        <span class="font-mono text-sm font-medium text-t1">模型聚合矩阵</span>
        <span class="tick">{{ filtered.length }} / {{ models.length }}</span>
      </div>

      <div v-if="filtered.length" class="divide-y divide-line">
        <div v-for="m in filtered" :key="m.name" class="grid grid-cols-[minmax(180px,0.8fr)_1.6fr_70px] max-md:grid-cols-1 gap-3 px-4 py-3 hover:bg-[rgb(var(--brass)/0.04)] transition-colors">
          <div class="min-w-0 flex items-center gap-2">
            <SignalDot :status="anyEnabled(m) ? 'online' : 'idle'" />
            <span class="font-mono font-semibold text-sm text-t1 truncate" :title="m.name">{{ m.name }}</span>
          </div>

          <div class="flex flex-wrap gap-1.5 min-w-0">
            <div v-for="(p, i) in m.providers" :key="i"
              class="inline-flex items-center gap-1.5 pl-2 pr-2 py-1 rounded-md text-2xs border font-mono"
              :class="p.enabled
                ? 'border-line bg-panel-2 text-t1'
                : 'border-line bg-panel text-t3 opacity-65'">
              <SignalDot :status="p.enabled ? 'online' : 'idle'" :size="5" :pulse="false" />
              <span class="font-sans text-xs font-medium">{{ p.channel_name }}</span>
              <ProtocolTag :protocol="p.protocol" />
              <span v-if="p.upstream && p.upstream !== m.name" class="text-t3 truncate max-w-[160px]">→ {{ p.upstream }}</span>
            </div>
          </div>

          <div class="flex md:justify-end items-center gap-2">
            <span :class="anyEnabled(m) ? 'badge badge-online' : 'badge badge-neutral'">{{ anyEnabled(m) ? '可用' : '禁用' }}</span>
            <span class="badge badge-neutral font-mono">{{ m.providers.length }}</span>
          </div>
        </div>
      </div>

      <div v-else class="empty-state">
        <span class="font-mono text-3xl text-t3">∅</span>
        <span>{{ q ? '没有匹配的模型' : '暂无模型，先在渠道中添加' }}</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useToast } from '../composables/useToast'
import api from '../api'
import StatDial from '../components/StatDial.vue'
import SignalDot from '../components/SignalDot.vue'
import ProtocolTag from '../components/ProtocolTag.vue'

const toast = useToast()
const models = ref([])
const loading = ref(true)
const q = ref('')

const filtered = computed(() => {
  const kw = q.value.trim().toLowerCase()
  if (!kw) return models.value
  return models.value.filter(m => m.name.toLowerCase().includes(kw))
})
const enabledModels = computed(() => models.value.filter(anyEnabled).length)
const totalBindings = computed(() => models.value.reduce((s, m) => s + m.providers.length, 0))

function anyEnabled(m) { return m.providers.some(p => p.enabled) }

async function load() {
  loading.value = true
  try {
    models.value = (await api.get('/models')) || []
  } catch (e) {
    toast.error('加载失败: ' + e.message)
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

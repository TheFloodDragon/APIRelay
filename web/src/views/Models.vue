<template>
  <div>
    <div class="flex items-center justify-between mb-6">
      <div>
        <h2 class="page-title">模型聚合</h2>
        <p class="page-subtitle">全局可用模型，按显示名聚合（同名可由多个供应商提供）</p>
      </div>
      <div class="flex items-center gap-3">
        <div class="relative">
          <svg viewBox="0 0 24 24" class="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-ink-400" fill="currentColor"><path d="M10 2a8 8 0 105.3 14l5.4 5.4 1.4-1.4-5.4-5.4A8 8 0 0010 2zm0 2a6 6 0 110 12 6 6 0 010-12z"/></svg>
          <input v-model="q" class="input !pl-9 w-64" placeholder="搜索模型..." />
        </div>
        <button @click="load" class="btn-secondary">🔄 刷新</button>
      </div>
    </div>

    <!-- 概览 -->
    <div class="grid grid-cols-2 md:grid-cols-3 gap-4 mb-6">
      <div class="card-flat flex items-center gap-4">
        <div class="w-11 h-11 rounded-xl bg-brand-50 dark:bg-brand-500/15 flex items-center justify-center text-xl">🧩</div>
        <div>
          <div class="text-2xl font-bold text-ink-900 dark:text-ink-50">{{ models.length }}</div>
          <div class="text-xs text-ink-500">聚合模型</div>
        </div>
      </div>
      <div class="card-flat flex items-center gap-4">
        <div class="w-11 h-11 rounded-xl bg-green-50 dark:bg-green-500/15 flex items-center justify-center text-xl">✅</div>
        <div>
          <div class="text-2xl font-bold text-ink-900 dark:text-ink-50">{{ enabledModels }}</div>
          <div class="text-xs text-ink-500">有可用供应商</div>
        </div>
      </div>
      <div class="card-flat flex items-center gap-4">
        <div class="w-11 h-11 rounded-xl bg-blue-50 dark:bg-blue-500/15 flex items-center justify-center text-xl">🔗</div>
        <div>
          <div class="text-2xl font-bold text-ink-900 dark:text-ink-50">{{ totalBindings }}</div>
          <div class="text-xs text-ink-500">供应商绑定</div>
        </div>
      </div>
    </div>

    <div v-if="loading" class="space-y-3">
      <div v-for="i in 5" :key="i" class="skeleton h-16 rounded-2xl"></div>
    </div>

    <div v-else class="space-y-3">
      <div v-for="m in filtered" :key="m.name" class="card-flat hover:shadow-card-hover transition-all">
        <div class="flex items-start justify-between gap-4">
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2 mb-2">
              <span class="font-mono font-semibold text-ink-900 dark:text-ink-100 truncate">{{ m.name }}</span>
              <span v-if="anyEnabled(m)" class="badge-success">可用</span>
              <span v-else class="badge-neutral">全部禁用</span>
              <span class="badge-info">{{ m.providers.length }} 供应商</span>
            </div>
            <div class="flex flex-wrap gap-2">
              <div v-for="(p, i) in m.providers" :key="i"
                class="inline-flex items-center gap-1.5 pl-2 pr-2.5 py-1 rounded-lg text-xs border"
                :class="p.enabled
                  ? 'border-ink-200 dark:border-ink-700 bg-white dark:bg-ink-800/50'
                  : 'border-ink-100 dark:border-ink-800 bg-ink-50 dark:bg-ink-900/40 opacity-60'">
                <span class="w-1.5 h-1.5 rounded-full shrink-0" :class="p.enabled ? 'bg-green-500' : 'bg-ink-400'"></span>
                <span class="text-ink-700 dark:text-ink-300 font-medium">{{ p.channel_name }}</span>
                <span class="protocol-tag" :class="protoClass(p.protocol)">{{ p.protocol }}</span>
                <span v-if="p.upstream && p.upstream !== m.name" class="text-ink-400 font-mono">→ {{ p.upstream }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-if="!filtered.length" class="empty-state card-flat">
        <div class="text-5xl mb-3 opacity-60">🧩</div>
        <div>{{ q ? '没有匹配的模型' : '暂无模型，先在供应商中添加' }}</div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useToast } from '../composables/useToast'
import api from '../api'

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
function protoClass(p) {
  return {
    openai: 'bg-blue-50 text-blue-600 dark:bg-blue-500/10 dark:text-blue-400',
    anthropic: 'bg-amber-50 text-amber-600 dark:bg-amber-500/10 dark:text-amber-400',
    responses: 'bg-purple-50 text-purple-600 dark:bg-purple-500/10 dark:text-purple-400',
  }[p] || 'bg-ink-100 text-ink-500 dark:bg-ink-800 dark:text-ink-400'
}

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

<style scoped>
.protocol-tag {
  @apply px-1.5 py-0.5 rounded font-mono text-[10px] font-medium;
}
</style>

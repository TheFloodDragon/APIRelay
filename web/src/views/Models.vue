<script setup>
import { ref, computed, onMounted, getCurrentInstance } from 'vue'
import api from '../api'
import StatCell from '../components/StatCell.vue'
import PageState from '../components/PageState.vue'

const { proxy } = getCurrentInstance()
const models = ref([])
const loading = ref(true)
const error = ref('')
const q = ref('')
const expandedModels = ref(new Set())

const filtered = computed(() => {
  const keyword = q.value.trim().toLowerCase()
  if (!keyword) return models.value
  return models.value.filter((model) => {
    if (model.name.toLowerCase().includes(keyword)) return true
    return providersFor(model).some((provider) => [
      provider.channel_name,
      provider.group,
      provider.protocol,
      provider.upstream,
    ].some((value) => String(value || '').toLowerCase().includes(keyword)))
  })
})
const enabledModels = computed(() => models.value.filter(anyEnabled).length)
const totalBindings = computed(() => models.value.reduce((sum, model) => sum + providersFor(model).length, 0))

function providersFor(model) {
  return Array.isArray(model.providers) ? model.providers : []
}

function enabledProviders(model) {
  return providersFor(model).filter((provider) => provider.enabled)
}

function anyEnabled(model) {
  return enabledProviders(model).length > 0
}

function upstreamName(model, provider) {
  return provider.upstream || model.name
}

function protocolEntries(model) {
  const counts = new Map()
  providersFor(model).forEach((provider) => {
    const protocol = provider.protocol || '未指定'
    counts.set(protocol, (counts.get(protocol) || 0) + 1)
  })
  return [...counts.entries()].map(([name, count]) => ({ name, count }))
}

function protocolSummary(model) {
  const entries = protocolEntries(model)
  if (!entries.length) return '—'
  return entries.map(({ name, count }) => `${name} ${count}`).join(' · ')
}

function upstreamSummary(model) {
  const names = [...new Set(providersFor(model).map((provider) => upstreamName(model, provider)))]
  if (!names.length) return '—'
  const visible = names.slice(0, 3).join('、')
  return names.length > 3 ? `${visible} 等 ${names.length} 个` : visible
}

function isExpanded(model) {
  return expandedModels.value.has(model.name)
}

function toggleModel(model) {
  const next = new Set(expandedModels.value)
  if (next.has(model.name)) next.delete(model.name)
  else next.add(model.name)
  expandedModels.value = next
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    models.value = (await api.get('/models')) || []
  } catch (e) {
    error.value = e.message || '无法读取模型数据'
    proxy.$toast.add(`模型加载失败：${error.value}`, 'error')
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="space-y-6">
    <header class="page-header">
      <div>
        <div class="eyebrow">模型管理</div>
        <h1 class="page-title">模型与渠道绑定</h1>
        <p class="page-description">按模型查看可用渠道、协议分布和上游模型映射。</p>
      </div>
      <div class="flex w-full flex-col gap-2 sm:w-auto sm:flex-row sm:items-end">
        <label class="min-w-0 sm:w-64">
          <span class="field-label">搜索</span>
          <input
            v-model="q"
            class="input input-mono"
            type="search"
            placeholder="模型、渠道或协议"
            aria-label="搜索模型、渠道或协议"
          />
        </label>
        <button class="btn shrink-0" :disabled="loading" aria-label="刷新模型列表" @click="load">
          {{ loading ? '刷新中…' : '刷新' }}
        </button>
      </div>
    </header>

    <PageState :loading="loading" :error="error" @retry="load">
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <StatCell label="模型数" :value="models.length" unit="个" />
        <StatCell label="可用模型" :value="enabledModels" unit="个" hint="至少有一个可用渠道" />
        <StatCell label="渠道绑定" :value="totalBindings" unit="个" />
      </div>

      <section class="mt-6 sheet overflow-hidden">
        <div class="sheet-head">
          <span class="dim-title">模型列表</span>
          <span class="text-xs text-soft">显示 {{ filtered.length }} / {{ models.length }}</span>
        </div>

        <PageState
          :empty="filtered.length === 0"
          :empty-text="q.trim() ? '没有匹配的模型' : '暂无模型'"
          :empty-hint="q.trim() ? '请调整关键词后重试。' : '请先在渠道管理中配置模型。'"
        >
          <div class="hidden md:block">
            <table class="table-eng table-fixed">
              <thead>
                <tr>
                  <th class="w-[34%]">模型</th>
                  <th class="w-[16%]">可用渠道</th>
                  <th class="w-[24%]">协议分布</th>
                  <th class="w-[26%]">上游映射</th>
                </tr>
              </thead>
              <tbody>
                <template v-for="model in filtered" :key="model.name">
                  <tr>
                    <td>
                      <button
                        class="flex w-full min-w-0 items-center gap-3 text-left"
                        type="button"
                        :aria-expanded="isExpanded(model)"
                        :aria-label="`${isExpanded(model) ? '收起' : '展开'}模型 ${model.name} 的渠道绑定`"
                        @click="toggleModel(model)"
                      >
                        <span class="w-4 shrink-0 text-center text-soft" aria-hidden="true">{{ isExpanded(model) ? '−' : '+' }}</span>
                        <span class="min-w-0">
                          <span class="block break-words font-mono text-[13px] font-medium text-ink">{{ model.name }}</span>
                          <span class="mt-1 block text-xs text-soft">{{ providersFor(model).length }} 个渠道绑定</span>
                        </span>
                      </button>
                    </td>
                    <td>
                      <span class="chip" :class="anyEnabled(model) ? 'chip-run' : ''">
                        {{ enabledProviders(model).length }} / {{ providersFor(model).length }}
                      </span>
                    </td>
                    <td class="break-words text-sm text-ink">{{ protocolSummary(model) }}</td>
                    <td class="break-words font-mono text-xs text-ink" :title="providersFor(model).map((provider) => upstreamName(model, provider)).join('、')">
                      {{ upstreamSummary(model) }}
                    </td>
                  </tr>
                  <tr v-if="isExpanded(model)">
                    <td colspan="4" class="bg-canvas/60 !p-0">
                      <div v-if="providersFor(model).length" class="divide-y divide-line">
                        <div
                          v-for="(provider, index) in providersFor(model)"
                          :key="`${model.name}-${provider.channel_id}-${index}`"
                          class="grid grid-cols-12 gap-x-4 gap-y-2 px-6 py-3 text-sm"
                        >
                          <div class="col-span-4 min-w-0">
                            <div class="break-words font-medium text-ink">{{ provider.channel_name || `渠道 #${provider.channel_id}` }}</div>
                            <div class="mt-0.5 text-xs text-soft">#{{ provider.channel_id }} · {{ provider.group || 'default' }}</div>
                          </div>
                          <div class="col-span-2">
                            <span class="chip" :class="provider.enabled ? 'chip-run' : ''">{{ provider.enabled ? '可用' : '停用' }}</span>
                          </div>
                          <div class="col-span-2 min-w-0 break-words text-soft">{{ provider.protocol || '未指定' }}</div>
                          <div class="col-span-4 min-w-0 break-words font-mono text-xs text-ink">
                            {{ model.name }} → {{ upstreamName(model, provider) }}
                          </div>
                        </div>
                      </div>
                      <div v-else class="px-6 py-4 text-sm text-soft">暂无渠道绑定。</div>
                    </td>
                  </tr>
                </template>
              </tbody>
            </table>
          </div>

          <div class="space-y-3 p-3 md:hidden">
            <article v-for="model in filtered" :key="model.name" class="mobile-card !p-0 overflow-hidden">
              <button
                class="flex w-full min-w-0 items-start justify-between gap-3 p-4 text-left"
                type="button"
                :aria-expanded="isExpanded(model)"
                :aria-label="`${isExpanded(model) ? '收起' : '展开'}模型 ${model.name} 的渠道绑定`"
                @click="toggleModel(model)"
              >
                <span class="min-w-0">
                  <span class="block break-words font-mono text-[13px] font-medium text-ink">{{ model.name }}</span>
                  <span class="mt-1 block text-xs text-soft">{{ providersFor(model).length }} 个渠道绑定</span>
                </span>
                <span class="shrink-0 text-lg leading-5 text-soft" aria-hidden="true">{{ isExpanded(model) ? '−' : '+' }}</span>
              </button>

              <dl class="space-y-3 border-t border-line px-4 py-3">
                <div class="mobile-kv">
                  <dt>可用渠道</dt>
                  <dd><span class="chip" :class="anyEnabled(model) ? 'chip-run' : ''">{{ enabledProviders(model).length }} / {{ providersFor(model).length }}</span></dd>
                </div>
                <div class="mobile-kv">
                  <dt>协议分布</dt>
                  <dd class="break-words">{{ protocolSummary(model) }}</dd>
                </div>
                <div class="mobile-kv">
                  <dt>上游映射</dt>
                  <dd class="break-words font-mono text-xs">{{ upstreamSummary(model) }}</dd>
                </div>
              </dl>

              <div v-if="isExpanded(model)" class="border-t border-line bg-canvas/60">
                <div
                  v-for="(provider, index) in providersFor(model)"
                  :key="`${model.name}-${provider.channel_id}-${index}`"
                  class="border-b border-line p-4 last:border-b-0"
                >
                  <div class="flex min-w-0 items-start justify-between gap-3">
                    <div class="min-w-0">
                      <div class="break-words font-medium text-ink">{{ provider.channel_name || `渠道 #${provider.channel_id}` }}</div>
                      <div class="mt-0.5 text-xs text-soft">#{{ provider.channel_id }} · {{ provider.group || 'default' }}</div>
                    </div>
                    <span class="chip shrink-0" :class="provider.enabled ? 'chip-run' : ''">{{ provider.enabled ? '可用' : '停用' }}</span>
                  </div>
                  <dl class="mt-3 space-y-2">
                    <div class="mobile-kv">
                      <dt>协议</dt>
                      <dd class="break-words">{{ provider.protocol || '未指定' }}</dd>
                    </div>
                    <div class="mobile-kv">
                      <dt>上游映射</dt>
                      <dd class="break-words font-mono text-xs">{{ model.name }} → {{ upstreamName(model, provider) }}</dd>
                    </div>
                  </dl>
                </div>
                <div v-if="!providersFor(model).length" class="p-4 text-sm text-soft">暂无渠道绑定。</div>
              </div>
            </article>
          </div>
        </PageState>
      </section>
    </PageState>
  </div>
</template>

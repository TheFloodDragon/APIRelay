<script setup>
import { ref, computed, onMounted, getCurrentInstance } from 'vue'
import api, { fmtTime } from '../api'
import { DEFAULT_HEALTH_CONFIG, hasHealth, healthPercent, healthText, healthTitle } from '../health'
import PageState from '../components/PageState.vue'
import ConsoleSection from '../components/ConsoleSection.vue'
import DataToolbar from '../components/DataToolbar.vue'
import Drawer from '../components/Drawer.vue'
import StatusBadge from '../components/StatusBadge.vue'
import ConsoleIcon from '../components/ConsoleIcon.vue'

const { proxy } = getCurrentInstance()
const models = ref([])
const loading = ref(true)
const error = ref('')
const q = ref('')
const healthFilter = ref('all')
const protocolFilter = ref('all')
const availabilityFilter = ref('all')
const expandedModels = ref(new Set())
const selectedModel = ref(null)
const healthConfig = ref({ ...DEFAULT_HEALTH_CONFIG })

const protocols = computed(() => [...new Set(
  models.value.flatMap((model) => providersFor(model).map((provider) => provider.protocol).filter(Boolean))
)].sort((a, b) => a.localeCompare(b)))

const filtered = computed(() => {
  const keyword = q.value.trim().toLowerCase()
  return models.value.filter((model) => {
    if (keyword && ![
      model.name,
      ...providersFor(model).flatMap((provider) => [
        provider.channel_name,
        provider.group,
        provider.protocol,
        provider.upstream,
      ]),
    ].some((value) => String(value || '').toLowerCase().includes(keyword))) return false

    if (healthFilter.value !== 'all' && healthState(model.health) !== healthFilter.value) return false
    if (protocolFilter.value !== 'all' && !providersFor(model).some((provider) => provider.protocol === protocolFilter.value)) return false
    if (availabilityFilter.value !== 'all' && availabilityState(model) !== availabilityFilter.value) return false
    return true
  })
})

const totalBindings = computed(() => models.value.reduce((sum, model) => sum + providersFor(model).length, 0))
const availableModels = computed(() => models.value.filter((model) => availabilityState(model) === 'available').length)
const activeFilterCount = computed(() => [
  q.value.trim(),
  healthFilter.value !== 'all',
  protocolFilter.value !== 'all',
  availabilityFilter.value !== 'all',
].filter(Boolean).length)

function providersFor(model) {
  return Array.isArray(model?.providers) ? model.providers : []
}

function enabledProviders(model) {
  return providersFor(model).filter((provider) => provider.enabled)
}

function availabilityState(model) {
  const total = providersFor(model).length
  const enabled = enabledProviders(model).length
  if (!total || !enabled) return 'unavailable'
  if (enabled === total) return 'available'
  return 'partial'
}

function availabilityStatus(model) {
  return availabilityState(model) === 'available' ? 'healthy' : availabilityState(model) === 'partial' ? 'warning' : 'disabled'
}

function availabilityLabel(model) {
  return `${enabledProviders(model).length} / ${providersFor(model).length}`
}

function healthState(health) {
  if (!hasHealth(health)) return 'unknown'
  const percent = healthPercent(health)
  if (percent >= Number(healthConfig.value.healthy_threshold ?? 95)) return 'healthy'
  if (percent >= Number(healthConfig.value.warning_threshold ?? 70)) return 'warning'
  return 'error'
}

function upstreamName(model, provider) {
  return provider.upstream || model.name
}

function dominantProtocol(model) {
  const counts = new Map()
  providersFor(model).forEach((provider) => {
    const protocol = provider.protocol || '未指定'
    counts.set(protocol, (counts.get(protocol) || 0) + 1)
  })
  return [...counts.entries()].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))[0]?.[0] || '—'
}

function mappingSummary(model) {
  const mappings = [...new Set(providersFor(model).map((provider) => upstreamName(model, provider)))]
  if (!mappings.length) return '无映射'
  const visible = mappings.slice(0, 2).join('、')
  return mappings.length > 2 ? `${visible} 等 ${mappings.length} 个` : visible
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

function openModel(model) {
  selectedModel.value = model
}

function resetFilters() {
  q.value = ''
  healthFilter.value = 'all'
  protocolFilter.value = 'all'
  availabilityFilter.value = 'all'
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [modelData, healthData] = await Promise.all([
      api.get('/models'),
      api.get('/settings/model-health'),
    ])
    models.value = modelData || []
    healthConfig.value = { ...healthConfig.value, ...(healthData || {}) }
    if (selectedModel.value) {
      selectedModel.value = models.value.find((model) => model.name === selectedModel.value.name) || null
    }
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
  <div class="page-workbench models-page min-w-0 space-y-4">
    <div class="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
      <div class="min-w-0">
        <div class="eyebrow">模型目录</div>
        <h1 class="text-lg font-semibold text-ink">模型与渠道绑定</h1>
        <p class="mt-1 text-xs leading-5 text-soft">检查模型供给、真实调用健康、协议与上游映射。</p>
      </div>
      <span class="mt-2 font-mono text-[10px] text-faint sm:mt-0">{{ models.length }} 个模型 · {{ totalBindings }} 个绑定</span>
    </div>

    <DataToolbar label="模型筛选工具栏">
      <label class="min-w-0 basis-full sm:basis-64 sm:flex-1">
        <span class="field-label">搜索</span>
        <span class="relative block">
          <ConsoleIcon name="search" class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-faint" />
          <input v-model="q" class="input input-mono pl-9" type="search" placeholder="模型、渠道、分组或映射" aria-label="搜索模型" />
        </span>
      </label>
      <label class="min-w-0 flex-1 sm:max-w-40">
        <span class="field-label">健康</span>
        <select v-model="healthFilter" class="input" aria-label="按健康状态筛选">
          <option value="all">全部健康状态</option>
          <option value="healthy">正常</option>
          <option value="warning">观察中</option>
          <option value="error">异常</option>
          <option value="unknown">未调用</option>
        </select>
      </label>
      <label class="min-w-0 flex-1 sm:max-w-44">
        <span class="field-label">协议</span>
        <select v-model="protocolFilter" class="input input-mono" aria-label="按协议筛选">
          <option value="all">全部协议</option>
          <option v-for="protocol in protocols" :key="protocol" :value="protocol">{{ protocol }}</option>
        </select>
      </label>
      <label class="min-w-0 flex-1 sm:max-w-40">
        <span class="field-label">可用性</span>
        <select v-model="availabilityFilter" class="input" aria-label="按可用性筛选">
          <option value="all">全部可用性</option>
          <option value="available">全部可用</option>
          <option value="partial">部分可用</option>
          <option value="unavailable">不可用</option>
        </select>
      </label>
      <template #actions>
        <span class="chip">显示 {{ filtered.length }} / {{ models.length }}</span>
        <button v-if="activeFilterCount" class="btn btn-sm" type="button" @click="resetFilters">清除筛选</button>
        <button class="btn btn-sm" type="button" :disabled="loading" aria-label="刷新模型列表" @click="load">
          <ConsoleIcon name="arrowPath" class="h-4 w-4" />
          {{ loading ? '刷新中' : '刷新' }}
        </button>
      </template>
    </DataToolbar>

    <PageState :loading="loading" :error="error" @retry="load">
      <ConsoleSection
        title="模型列表"
        :description="`${availableModels} 个模型的全部渠道可用；点击模型查看结构化渠道绑定。`"
        flush
      >
        <PageState
          :empty="filtered.length === 0"
          :empty-text="models.length ? '没有匹配的模型' : '暂无模型'"
          :empty-hint="models.length ? '请调整搜索或筛选条件。' : '请先在渠道管理中配置模型。'"
        >
          <div class="hidden md:block">
            <table class="table-eng table-fixed">
              <thead>
                <tr>
                  <th class="w-[22%]">模型名</th>
                  <th class="w-[13%]">渠道可用度</th>
                  <th class="w-[16%]">健康</th>
                  <th class="w-[14%]">主要协议</th>
                  <th class="w-[14%]">最近调用</th>
                  <th class="w-[21%]">映射摘要</th>
                </tr>
              </thead>
              <tbody>
                <template v-for="model in filtered" :key="model.name">
                  <tr
                    class="cursor-pointer"
                    tabindex="0"
                    :aria-expanded="isExpanded(model)"
                    @click="toggleModel(model)"
                    @keydown.enter.prevent="toggleModel(model)"
                    @keydown.space.prevent="toggleModel(model)"
                  >
                    <td>
                      <div class="flex min-w-0 items-center gap-2">
                        <ConsoleIcon name="chevronRight" class="h-4 w-4 shrink-0 text-faint transition-transform" :class="{ 'rotate-90': isExpanded(model) }" />
                        <div class="min-w-0">
                          <div class="break-words font-mono text-[12px] font-medium text-ink">{{ model.name }}</div>
                          <div class="mt-0.5 text-[10px] text-faint">{{ providersFor(model).length }} 个绑定</div>
                        </div>
                      </div>
                    </td>
                    <td><StatusBadge :status="availabilityStatus(model)" :label="availabilityLabel(model)" /></td>
                    <td><StatusBadge :status="healthState(model.health)" :label="healthText(model.health)" :title="healthTitle(model.health)" /></td>
                    <td class="break-words font-mono text-xs text-ink">{{ dominantProtocol(model) }}</td>
                    <td class="font-mono text-[11px] text-soft" :title="model.last_used_at ? new Date(model.last_used_at).toLocaleString() : '尚未调用'">
                      {{ model.last_used_at ? fmtTime(model.last_used_at) : '未使用' }}
                    </td>
                    <td class="break-words font-mono text-[11px] text-ink" :title="providersFor(model).map((provider) => upstreamName(model, provider)).join('、')">
                      {{ mappingSummary(model) }}
                    </td>
                  </tr>
                  <tr v-if="isExpanded(model)">
                    <td colspan="6" class="!p-0">
                      <div class="border-y border-line bg-canvas/60">
                        <div class="grid grid-cols-[minmax(0,1.2fr)_90px_minmax(0,.8fr)_minmax(0,1fr)_minmax(0,1.35fr)] gap-3 border-b border-line px-5 py-2 font-mono text-[9px] uppercase tracking-wider text-faint">
                          <span>渠道 / 分组</span><span>可用性</span><span>健康</span><span>协议</span><span>模型映射</span>
                        </div>
                        <div
                          v-for="(provider, index) in providersFor(model)"
                          :key="`${model.name}-${provider.channel_id}-${index}`"
                          class="grid grid-cols-[minmax(0,1.2fr)_90px_minmax(0,.8fr)_minmax(0,1fr)_minmax(0,1.35fr)] items-center gap-3 border-b border-line px-5 py-3 last:border-b-0"
                        >
                          <div class="min-w-0">
                            <div class="break-words text-xs font-medium text-ink">{{ provider.channel_name || `渠道 #${provider.channel_id}` }}</div>
                            <div class="mt-0.5 font-mono text-[10px] text-faint">#{{ provider.channel_id }} · {{ provider.group || 'default' }}</div>
                          </div>
                          <StatusBadge :status="provider.enabled ? 'healthy' : 'disabled'" :label="provider.enabled ? '可用' : '停用'" />
                          <StatusBadge :status="healthState(provider.health)" :label="healthText(provider.health)" :title="healthTitle(provider.health)" />
                          <div class="min-w-0 break-words font-mono text-[11px] text-soft">{{ provider.protocol || '未指定' }}</div>
                          <div class="min-w-0 break-words font-mono text-[11px] text-ink">{{ model.name }} → {{ upstreamName(model, provider) }}</div>
                        </div>
                        <div v-if="!providersFor(model).length" class="px-5 py-4 text-xs text-soft">暂无渠道绑定。</div>
                      </div>
                    </td>
                  </tr>
                </template>
              </tbody>
            </table>
          </div>

          <div class="divide-y divide-line md:hidden">
            <button
              v-for="model in filtered"
              :key="model.name"
              type="button"
              class="block w-full min-w-0 px-4 py-3 text-left"
              :aria-label="`查看模型 ${model.name} 的渠道绑定`"
              @click="openModel(model)"
            >
              <span class="flex min-w-0 items-start justify-between gap-3">
                <span class="min-w-0">
                  <span class="block break-words font-mono text-xs font-medium text-ink">{{ model.name }}</span>
                  <span class="mt-1 block text-[10px] text-faint">{{ dominantProtocol(model) }} · {{ model.last_used_at ? fmtTime(model.last_used_at) : '未使用' }}</span>
                </span>
                <ConsoleIcon name="chevronRight" class="mt-0.5 h-4 w-4 shrink-0 text-faint" />
              </span>
              <span class="mt-2 flex min-w-0 flex-wrap items-center gap-2">
                <StatusBadge :status="availabilityStatus(model)" :label="`${availabilityLabel(model)} 渠道`" />
                <StatusBadge :status="healthState(model.health)" :label="healthText(model.health)" />
                <span class="min-w-0 truncate font-mono text-[10px] text-soft">{{ mappingSummary(model) }}</span>
              </span>
            </button>
          </div>
        </PageState>
      </ConsoleSection>
    </PageState>

    <Drawer
      :open="!!selectedModel"
      :title="selectedModel ? selectedModel.name : '模型详情'"
      width="max-w-xl"
      @close="selectedModel = null"
    >
      <template v-if="selectedModel">
        <div class="grid grid-cols-2 gap-3">
          <div class="stat-cell min-w-0">
            <div class="stat-cell-label">渠道可用度</div>
            <div class="mt-2"><StatusBadge :status="availabilityStatus(selectedModel)" :label="availabilityLabel(selectedModel)" /></div>
          </div>
          <div class="stat-cell min-w-0">
            <div class="stat-cell-label">调用健康</div>
            <div class="mt-2"><StatusBadge :status="healthState(selectedModel.health)" :label="healthText(selectedModel.health)" :title="healthTitle(selectedModel.health)" /></div>
          </div>
          <div class="stat-cell min-w-0">
            <div class="stat-cell-label">主要协议</div>
            <div class="mt-2 break-words font-mono text-xs text-ink">{{ dominantProtocol(selectedModel) }}</div>
          </div>
          <div class="stat-cell min-w-0">
            <div class="stat-cell-label">最近调用</div>
            <div class="mt-2 font-mono text-xs text-ink">{{ selectedModel.last_used_at ? fmtTime(selectedModel.last_used_at) : '未使用' }}</div>
          </div>
        </div>

        <section class="mt-5" aria-labelledby="mobile-bindings-title">
          <div class="mb-2 flex items-center justify-between gap-3">
            <h3 id="mobile-bindings-title" class="text-sm font-semibold text-ink">渠道绑定</h3>
            <span class="text-xs text-soft">{{ providersFor(selectedModel).length }} 个</span>
          </div>
          <div v-if="providersFor(selectedModel).length" class="divide-y divide-line overflow-hidden rounded-lg border border-line">
            <article v-for="(provider, index) in providersFor(selectedModel)" :key="`${selectedModel.name}-${provider.channel_id}-${index}`" class="min-w-0 bg-surface p-3">
              <div class="flex min-w-0 items-start justify-between gap-3">
                <div class="min-w-0">
                  <h4 class="break-words text-sm font-medium text-ink">{{ provider.channel_name || `渠道 #${provider.channel_id}` }}</h4>
                  <p class="mt-0.5 font-mono text-[10px] text-faint">#{{ provider.channel_id }} · {{ provider.group || 'default' }}</p>
                </div>
                <StatusBadge :status="provider.enabled ? 'healthy' : 'disabled'" :label="provider.enabled ? '可用' : '停用'" />
              </div>
              <dl class="mt-3 space-y-2">
                <div class="mobile-kv"><dt>健康</dt><dd><StatusBadge :status="healthState(provider.health)" :label="healthText(provider.health)" /></dd></div>
                <div class="mobile-kv"><dt>协议</dt><dd class="break-words font-mono text-xs">{{ provider.protocol || '未指定' }}</dd></div>
                <div class="mobile-kv"><dt>映射</dt><dd class="break-all font-mono text-[11px]">{{ selectedModel.name }} → {{ upstreamName(selectedModel, provider) }}</dd></div>
              </dl>
            </article>
          </div>
          <div v-else class="rounded-lg border border-dashed border-line bg-surface px-4 py-6 text-center text-xs text-soft">暂无渠道绑定。</div>
        </section>
      </template>
    </Drawer>
  </div>
</template>

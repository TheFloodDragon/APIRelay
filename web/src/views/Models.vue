<script setup>
import { ref, computed, onMounted, getCurrentInstance } from 'vue'
import api, { fmtTime } from '../api'
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
const calledModels = computed(() => models.value.filter((model) => hasHealth(model.health)).length)
const totalCalls = computed(() => models.value.reduce((sum, model) => sum + healthTotal(model.health), 0))

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

function healthTotal(health) {
  return Number(health?.total) || 0
}

function hasHealth(health) {
  return healthTotal(health) > 0
}

function healthPercent(health) {
  if (!hasHealth(health)) return 0
  return Math.round((Number(health.availability) || 0) * 10) / 10
}

function healthText(health) {
  if (!hasHealth(health)) return '未调用'
  return `${healthPercent(health)}% · ${Number(health.success) || 0}/${healthTotal(health)}`
}

function healthClass(health) {
  if (!hasHealth(health)) return ''
  const percent = healthPercent(health)
  if (percent >= 95) return 'chip-run'
  if (percent >= 70) return 'chip-test'
  return 'chip-trip'
}

function healthTitle(health) {
  if (!hasHealth(health)) return '尚无真实调用日志'
  const parts = [`成功 ${Number(health.success) || 0}`, `失败 ${Number(health.failed) || 0}`]
  if (health.last_failure_at) parts.push(`最近失败 ${fmtTime(health.last_failure_at)}`)
  if (health.last_error) parts.push(health.last_error)
  return parts.join(' · ')
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
        <p class="page-description">按最近使用时间排列，查看模型的可用渠道、真实调用健康、协议分布和上游映射。</p>
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
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <StatCell label="模型数" :value="models.length" unit="个" />
        <StatCell label="可用模型" :value="enabledModels" unit="个" hint="至少有一个可用渠道" />
        <StatCell label="有调用日志" :value="calledModels" unit="个" hint="来自真实转发日志" />
        <StatCell label="累计调用" :value="totalCalls" unit="次" />
      </div>

      <section class="mt-6 sheet overflow-hidden">
        <div class="sheet-head">
          <span class="dim-title">模型列表</span>
          <span class="text-xs text-soft">显示 {{ filtered.length }} / {{ models.length }} · {{ totalBindings }} 个渠道绑定</span>
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
                  <th class="w-[25%]">模型</th>
                  <th class="w-[13%]">最近使用</th>
                  <th class="w-[12%]">可用渠道</th>
                  <th class="w-[14%]">调用健康</th>
                  <th class="w-[17%]">协议分布</th>
                  <th class="w-[19%]">上游映射</th>
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
                    <td class="font-mono text-xs text-soft" :title="model.last_used_at ? new Date(model.last_used_at).toLocaleString() : '尚未调用'">
                      {{ model.last_used_at ? fmtTime(model.last_used_at) : '未使用' }}
                    </td>
                    <td>
                      <span class="chip" :class="anyEnabled(model) ? 'chip-run' : ''">
                        {{ enabledProviders(model).length }} / {{ providersFor(model).length }}
                      </span>
                    </td>
                    <td><span class="chip" :class="healthClass(model.health)" :title="healthTitle(model.health)">{{ healthText(model.health) }}</span></td>
                    <td class="break-words text-sm text-ink">{{ protocolSummary(model) }}</td>
                    <td class="break-words font-mono text-xs text-ink" :title="providersFor(model).map((provider) => upstreamName(model, provider)).join('、')">
                      {{ upstreamSummary(model) }}
                    </td>
                  </tr>
                  <tr v-if="isExpanded(model)">
                    <td colspan="6" class="bg-canvas/60 !p-0">
                      <div v-if="providersFor(model).length" class="divide-y divide-line">
                        <div
                          v-for="(provider, index) in providersFor(model)"
                          :key="`${model.name}-${provider.channel_id}-${index}`"
                          class="grid grid-cols-12 gap-x-4 gap-y-2 px-6 py-3 text-sm"
                        >
                          <div class="col-span-3 min-w-0">
                            <div class="break-words font-medium text-ink">{{ provider.channel_name || `渠道 #${provider.channel_id}` }}</div>
                            <div class="mt-0.5 text-xs text-soft">#{{ provider.channel_id }} · {{ provider.group || 'default' }}</div>
                          </div>
                          <div class="col-span-2">
                            <span class="chip" :class="provider.enabled ? 'chip-run' : ''">{{ provider.enabled ? '可用' : '停用' }}</span>
                          </div>
                          <div class="col-span-2"><span class="chip" :class="healthClass(provider.health)" :title="healthTitle(provider.health)">{{ healthText(provider.health) }}</span></div>
                          <div class="col-span-2 min-w-0 break-words text-soft">{{ provider.protocol || '未指定' }}</div>
                          <div class="col-span-3 min-w-0 break-words font-mono text-xs text-ink">
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
                  <dt>最近使用</dt>
                  <dd class="font-mono text-xs">{{ model.last_used_at ? fmtTime(model.last_used_at) : '未使用' }}</dd>
                </div>
                <div class="mobile-kv">
                  <dt>可用渠道</dt>
                  <dd><span class="chip" :class="anyEnabled(model) ? 'chip-run' : ''">{{ enabledProviders(model).length }} / {{ providersFor(model).length }}</span></dd>
                </div>
                <div class="mobile-kv">
                  <dt>调用健康</dt>
                  <dd><span class="chip" :class="healthClass(model.health)" :title="healthTitle(model.health)">{{ healthText(model.health) }}</span></dd>
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
                      <dt>调用健康</dt>
                      <dd><span class="chip" :class="healthClass(provider.health)" :title="healthTitle(provider.health)">{{ healthText(provider.health) }}</span></dd>
                    </div>
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

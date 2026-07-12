<script setup>
import { ref, computed, onMounted, getCurrentInstance } from 'vue'
import api, { copyText, usd } from '../api'
import { confirmAction } from '../composables/useConfirm'
import PageState from '../components/PageState.vue'
import ConsoleSection from '../components/ConsoleSection.vue'
import DataToolbar from '../components/DataToolbar.vue'
import InlineNotice from '../components/InlineNotice.vue'
import Drawer from '../components/Drawer.vue'
import Modal from '../components/Modal.vue'
import StatusBadge from '../components/StatusBadge.vue'
import ConsoleIcon from '../components/ConsoleIcon.vue'

const { proxy } = getCurrentInstance()
const tokens = ref([])
const loading = ref(true)
const error = ref('')
const deleteError = ref('')
const deletingId = ref(null)
const query = ref('')
const statusFilter = ref('all')
const createOpen = ref(false)
const saving = ref(false)
const formError = ref('')
const secretKey = ref('')
const secretOpen = ref(false)
const secretAcknowledged = ref(false)
const copied = ref(false)
const form = ref(emptyForm())

const filteredTokens = computed(() => {
  const keyword = query.value.trim().toLowerCase()
  return tokens.value.filter((token) => {
    if (statusFilter.value === 'enabled' && token.status !== 1) return false
    if (statusFilter.value === 'disabled' && token.status === 1) return false
    if (!keyword) return true
    return [token.name, keyPrefix(token), token.group || 'default', modelScope(token)]
      .some((value) => String(value || '').toLowerCase().includes(keyword))
  })
})

const enabledCount = computed(() => tokens.value.filter((token) => token.status === 1).length)
const limitedCount = computed(() => tokens.value.filter((token) => !token.unlimited).length)
const totalUsed = computed(() => tokens.value.reduce((sum, token) => sum + (Number(token.used_quota) || 0), 0))
const hasFilters = computed(() => !!query.value.trim() || statusFilter.value !== 'all')

function emptyForm() {
  return { name: '', group: 'default', models: '', unlimited: true, quota_usd: 0 }
}

function keyPrefix(token) {
  return token.key_prefix || '—'
}

function modelScope(token) {
  return token.models || '全部模型'
}

function quotaDisplay(token) {
  if (token.unlimited) return `${usd(token.used_quota)} / 不限额度`
  return `${usd(token.used_quota)} / ${usd(token.quota)}`
}

function quotaPercent(token) {
  if (token.unlimited || !token.quota) return 0
  return Math.min(100, Math.max(0, (Number(token.used_quota) / Number(token.quota)) * 100))
}

function quotaFillClass(token) {
  const percent = quotaPercent(token)
  if (percent >= 90) return 'quota-fill-trip'
  if (percent >= 70) return 'quota-fill-test'
  return 'quota-fill-run'
}

function clearFilters() {
  query.value = ''
  statusFilter.value = 'all'
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    tokens.value = (await api.get('/tokens')) || []
  } catch (e) {
    error.value = e.message || '无法读取令牌数据'
    proxy.$toast.add(`令牌加载失败：${error.value}`, 'error')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  form.value = emptyForm()
  formError.value = ''
  createOpen.value = true
}

function closeCreate() {
  if (!saving.value) createOpen.value = false
}

async function save() {
  formError.value = ''
  const name = form.value.name.trim()
  const quota = Number(form.value.quota_usd) || 0
  if (!name) {
    formError.value = '请填写令牌名称'
    return
  }
  if (!form.value.unlimited && quota <= 0) {
    formError.value = '限制额度必须大于 0 美元'
    return
  }

  saving.value = true
  try {
    const result = await api.post('/tokens', {
      name,
      group: form.value.group.trim() || 'default',
      models: form.value.models.trim(),
      unlimited: form.value.unlimited,
      quota_usd: form.value.unlimited ? 0 : quota,
    })
    if (!result?.key) throw new Error('服务端未返回一次性密钥')

    secretKey.value = result.key
    copied.value = false
    secretAcknowledged.value = false
    createOpen.value = false
    secretOpen.value = true
    proxy.$toast.add('令牌已创建，请立即保存一次性 Key', 'success')
    await load()
  } catch (e) {
    formError.value = e.message || '创建失败'
  } finally {
    saving.value = false
  }
}

async function copySecret() {
  const ok = await copyText(secretKey.value)
  copied.value = ok
  proxy.$toast.add(ok ? '完整 Key 已复制' : '自动复制失败，请手动选择并复制 Key', ok ? 'success' : 'warn')
}

function confirmSecretSaved() {
  if (!secretAcknowledged.value) return
  secretOpen.value = false
  secretKey.value = ''
  copied.value = false
  secretAcknowledged.value = false
}

function keepSecretOpen() {
  proxy.$toast.add('请先保存 Key，并勾选确认后再关闭', 'warn')
}

async function removeToken(token) {
  const confirmed = await confirmAction({
    title: '删除 API 令牌',
    message: `确认删除令牌「${token.name}」？删除后客户端将立即失去访问权限。`,
    confirmLabel: '删除令牌',
  })
  if (!confirmed) return
  deleteError.value = ''
  deletingId.value = token.id
  try {
    await api.delete(`/tokens/${token.id}`)
    proxy.$toast.add('令牌已删除', 'success')
    await load()
  } catch (e) {
    deleteError.value = `删除「${token.name}」失败：${e.message || '未知错误'}`
    proxy.$toast.add(deleteError.value, 'error')
  } finally {
    deletingId.value = null
  }
}

onMounted(load)
</script>

<template>
  <div class="page-workbench tokens-page min-w-0 space-y-4">
    <div class="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
      <div class="min-w-0">
        <div class="eyebrow">访问控制</div>
        <h1 class="text-lg font-semibold text-ink">API 令牌</h1>
        <p class="mt-1 text-xs leading-5 text-soft">管理客户端凭证、分组、模型范围与额度。</p>
      </div>
      <button class="btn btn-primary mt-2 w-full sm:mt-0 sm:w-auto" type="button" aria-label="创建 API 令牌" @click="openCreate">
        <ConsoleIcon name="plus" class="h-4 w-4" />
        创建令牌
      </button>
    </div>

    <InlineNotice v-if="deleteError" tone="danger" title="令牌删除失败">
      {{ deleteError }}
    </InlineNotice>

    <DataToolbar label="令牌摘要与筛选工具栏">
      <div class="flex basis-full flex-wrap gap-2" aria-label="令牌摘要">
        <span class="chip">总计 {{ tokens.length }}</span>
        <span class="chip chip-run">启用 {{ enabledCount }}</span>
        <span class="chip chip-blue">限额 {{ limitedCount }}</span>
        <span class="chip">累计使用 {{ usd(totalUsed) }}</span>
      </div>
      <label class="min-w-0 basis-full sm:basis-72 sm:flex-1">
        <span class="field-label">搜索</span>
        <span class="relative block">
          <ConsoleIcon name="search" class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-faint" />
          <input
            v-model="query"
            class="input input-mono pl-9"
            type="search"
            placeholder="名称、Key 前缀、分组或模型"
            aria-label="搜索令牌"
          />
        </span>
      </label>
      <label class="min-w-0 flex-1 sm:max-w-44">
        <span class="field-label">状态</span>
        <select v-model="statusFilter" class="input" aria-label="按令牌状态筛选">
          <option value="all">全部状态</option>
          <option value="enabled">启用</option>
          <option value="disabled">停用</option>
        </select>
      </label>
      <template #actions>
        <span class="chip">显示 {{ filteredTokens.length }} / {{ tokens.length }}</span>
        <button v-if="hasFilters" class="btn btn-sm" type="button" @click="clearFilters">清除筛选</button>
        <button class="btn btn-sm" type="button" :disabled="loading" aria-label="刷新令牌列表" @click="load">
          <ConsoleIcon name="arrowPath" class="h-4 w-4" />
          {{ loading ? '刷新中' : '刷新' }}
        </button>
      </template>
    </DataToolbar>

    <PageState :loading="loading" :error="error" @retry="load">
      <ConsoleSection
        title="令牌列表"
        description="列表仅保留 Key 前缀；完整 Key 只会在创建成功后展示一次。"
        flush
      >
        <PageState
          :empty="filteredTokens.length === 0"
          :empty-text="tokens.length ? '没有匹配的令牌' : '暂无 API 令牌'"
          :empty-hint="tokens.length ? '请调整搜索或状态筛选。' : '创建后请立即保存完整 Key；离开一次性展示窗口后无法再次查看。'"
        >
          <template #empty>
            <button v-if="!tokens.length" class="btn btn-primary" type="button" @click="openCreate">创建第一个令牌</button>
            <button v-else class="btn" type="button" @click="clearFilters">清除筛选</button>
          </template>

          <div class="hidden md:block">
            <table class="table-eng table-fixed">
              <thead>
                <tr>
                  <th class="w-[20%]">名称</th>
                  <th class="w-[15%]">Key 前缀</th>
                  <th class="w-[11%]">分组</th>
                  <th class="w-[20%]">模型范围</th>
                  <th class="w-[22%]">已用 / 总额度</th>
                  <th class="w-[7%]">状态</th>
                  <th class="w-[5%]"><span class="sr-only">操作</span></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="token in filteredTokens" :key="token.id">
                  <td>
                    <div class="break-words text-sm font-semibold text-ink">{{ token.name }}</div>
                    <div class="mt-0.5 font-mono text-[10px] text-faint">#{{ token.id }}</div>
                  </td>
                  <td><span class="break-all font-mono text-xs text-ink" title="完整 Key 仅在创建后显示一次">{{ keyPrefix(token) }}</span></td>
                  <td><span class="chip chip-blue">{{ token.group || 'default' }}</span></td>
                  <td><div class="break-words font-mono text-[11px] text-ink" :title="modelScope(token)">{{ modelScope(token) }}</div></td>
                  <td>
                    <div class="flex min-w-0 items-center gap-3">
                      <span class="min-w-0 flex-1 font-mono text-[11px] tabular-nums text-ink">{{ quotaDisplay(token) }}</span>
                      <div v-if="!token.unlimited" class="quota-track w-20 shrink-0" :aria-label="`额度已使用 ${quotaPercent(token).toFixed(1)}%`" role="img">
                        <div class="quota-fill" :class="quotaFillClass(token)" :style="{ width: `${quotaPercent(token)}%` }"></div>
                      </div>
                      <span v-else class="text-[10px] text-faint">∞</span>
                    </div>
                  </td>
                  <td><StatusBadge :status="token.status === 1 ? 'healthy' : 'disabled'" :label="token.status === 1 ? '启用' : '停用'" /></td>
                  <td class="text-right">
                    <button
                      class="icon-button h-8 w-8 border-trip/25 text-trip"
                      type="button"
                      :disabled="deletingId === token.id"
                      :aria-label="`删除令牌 ${token.name}`"
                      @click="removeToken(token)"
                    >
                      <ConsoleIcon name="trash" class="h-4 w-4" />
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <div class="divide-y divide-line md:hidden">
            <article v-for="token in filteredTokens" :key="token.id" class="min-w-0 px-4 py-3">
              <div class="flex min-w-0 items-start justify-between gap-3">
                <div class="min-w-0">
                  <h2 class="break-words text-sm font-semibold text-ink">{{ token.name }}</h2>
                  <p class="mt-1 break-all font-mono text-[11px] text-soft" title="完整 Key 仅在创建后显示一次">{{ keyPrefix(token) }}</p>
                </div>
                <StatusBadge class="shrink-0" :status="token.status === 1 ? 'healthy' : 'disabled'" :label="token.status === 1 ? '启用' : '停用'" />
              </div>

              <div class="mt-3 flex min-w-0 flex-wrap items-center gap-2">
                <span class="chip chip-blue">{{ token.group || 'default' }}</span>
                <span class="min-w-0 break-words font-mono text-[10px] text-soft">{{ modelScope(token) }}</span>
              </div>

              <div class="mt-3 rounded-md border border-line bg-surface px-3 py-2.5">
                <div class="flex min-w-0 items-center justify-between gap-3">
                  <span class="text-[10px] font-medium text-faint">已用 / 总额度</span>
                  <span class="min-w-0 text-right font-mono text-[11px] tabular-nums text-ink">{{ quotaDisplay(token) }}</span>
                </div>
                <div v-if="!token.unlimited" class="quota-track mt-2" :aria-label="`额度已使用 ${quotaPercent(token).toFixed(1)}%`" role="img">
                  <div class="quota-fill" :class="quotaFillClass(token)" :style="{ width: `${quotaPercent(token)}%` }"></div>
                </div>
              </div>

              <div class="mt-3 flex items-center justify-between gap-3">
                <span class="font-mono text-[10px] text-faint">令牌 #{{ token.id }}</span>
                <button
                  class="btn btn-danger btn-sm"
                  type="button"
                  :disabled="deletingId === token.id"
                  :aria-label="`删除令牌 ${token.name}`"
                  @click="removeToken(token)"
                >
                  <ConsoleIcon name="trash" class="h-4 w-4" />
                  {{ deletingId === token.id ? '删除中' : '删除' }}
                </button>
              </div>
            </article>
          </div>
        </PageState>
      </ConsoleSection>
    </PageState>

    <Drawer :open="createOpen" title="创建 API 令牌" width="max-w-lg" @close="closeCreate">
      <div class="space-y-4">
        <InlineNotice tone="info" title="一次性密钥">
          创建成功后只展示一次完整 Key，请准备好安全的保存位置。
        </InlineNotice>

        <label>
          <span class="field-label">名称</span>
          <input v-model="form.name" class="input" placeholder="例：测试环境" autocomplete="off" data-autofocus />
        </label>

        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <label class="min-w-0">
            <span class="field-label">分组</span>
            <input v-model="form.group" class="input input-mono" placeholder="default" autocomplete="off" />
          </label>
          <label class="min-w-0">
            <span class="field-label">额度模式</span>
            <span class="flex min-h-9 items-center gap-2 rounded-md border border-line bg-surface px-3 text-sm text-ink">
              <input v-model="form.unlimited" type="checkbox" aria-label="不限额度" />
              <span>{{ form.unlimited ? '不限额度' : '限制额度' }}</span>
            </span>
          </label>
        </div>

        <label>
          <span class="field-label">模型范围</span>
          <input v-model="form.models" class="input input-mono" placeholder="逗号分隔，留空为全部模型" autocomplete="off" />
          <span class="field-help">例如：gpt-4o, claude-3-5-sonnet</span>
        </label>

        <label v-if="!form.unlimited">
          <span class="field-label">总额度（USD）</span>
          <input v-model.number="form.quota_usd" class="input input-mono" type="number" min="0.000001" step="0.01" placeholder="10.00" />
        </label>

        <InlineNotice v-if="formError" tone="danger" title="无法创建令牌">
          {{ formError }}
        </InlineNotice>
      </div>
      <template #footer>
        <div class="flex w-full flex-wrap justify-end gap-2">
          <button class="btn" type="button" :disabled="saving" aria-label="取消创建令牌" @click="closeCreate">取消</button>
          <button class="btn btn-primary" type="button" :disabled="saving" aria-label="创建令牌" @click="save">
            {{ saving ? '创建中…' : '创建令牌' }}
          </button>
        </div>
      </template>
    </Drawer>

    <Modal :open="secretOpen" title="一次性 API Key" width="max-w-2xl" persistent @close="keepSecretOpen">
      <div class="space-y-4">
        <InlineNotice tone="warning" title="完整 Key 仅显示一次">
          关闭后无法再次查看，请立即复制并保存到安全位置。
        </InlineNotice>
        <label>
          <span class="field-label">完整 API Key</span>
          <textarea class="input input-mono min-h-24 resize-none break-all" :value="secretKey" readonly aria-label="完整一次性 API Key"></textarea>
        </label>
        <button class="btn w-full" type="button" aria-label="复制完整 API Key" @click="copySecret">
          <ConsoleIcon :name="copied ? 'success' : 'key'" class="h-4 w-4" />
          {{ copied ? '已复制完整 Key' : '复制完整 Key' }}
        </button>
        <p class="text-xs leading-5 text-soft">若浏览器不允许自动复制，请手动选择上方内容并复制。</p>
        <label class="flex min-w-0 items-start gap-3 rounded-lg border border-line bg-surface px-4 py-3 text-sm text-ink">
          <input v-model="secretAcknowledged" class="mt-1 shrink-0" type="checkbox" />
          <span class="min-w-0">我已将完整 Key 保存到安全位置，并理解关闭后无法恢复。</span>
        </label>
      </div>
      <template #footer>
        <span class="mr-auto min-w-0 text-xs text-soft">勾选确认后才能清除明文并关闭。</span>
        <button class="btn btn-primary" type="button" :disabled="!secretAcknowledged" aria-label="确认已保存一次性 API Key" @click="confirmSecretSaved">我已保存并清除</button>
      </template>
    </Modal>
  </div>
</template>

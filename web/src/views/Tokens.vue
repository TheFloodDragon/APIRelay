<script setup>
import { ref, onMounted, getCurrentInstance } from 'vue'
import api, { copyText, usd } from '../api'
import { confirmAction } from '../composables/useConfirm'
import PageState from '../components/PageState.vue'
import PageHeader from '../components/PageHeader.vue'
import Modal from '../components/Modal.vue'

const { proxy } = getCurrentInstance()
const tokens = ref([])
const loading = ref(true)
const error = ref('')
const deleteError = ref('')
const deletingId = ref(null)
const createOpen = ref(false)
const saving = ref(false)
const formError = ref('')
const secretKey = ref('')
const secretOpen = ref(false)
const secretAcknowledged = ref(false)
const copied = ref(false)
const form = ref(emptyForm())

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
  secretOpen.value = false
  secretKey.value = ''
  copied.value = false
}

function keepSecretOpen() {
  proxy.$toast.add('请先保存 Key，再点击“我已保存”', 'warn')
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
  <div class="page-workbench tokens-page space-y-6">
    <PageHeader eyebrow="访问控制" title="API 令牌" description="管理客户端访问凭证、额度、分组和可用模型范围。">
      <template #actions>
        <button class="btn btn-primary" aria-label="创建 API 令牌" @click="openCreate">创建令牌</button>
      </template>
    </PageHeader>

    <div v-if="deleteError" class="rounded-lg border border-trip/30 bg-trip-wash px-3 py-2 text-sm text-trip" role="alert">
      {{ deleteError }}
    </div>

    <PageState
      :loading="loading"
      :error="error"
      :empty="tokens.length === 0"
      empty-text="暂无 API 令牌"
      empty-hint="创建令牌后，客户端即可按配置的额度和模型范围访问 API。"
      @retry="load"
    >
      <template #empty>
        <button class="btn btn-primary" @click="openCreate">创建第一个令牌</button>
      </template>

      <div class="grid items-start gap-5 xl:grid-cols-[250px_minmax(0,1fr)]">
        <aside class="credential-brief xl:sticky xl:top-28">
          <div class="eyebrow">Credential vault</div>
          <div class="mt-4 font-cond text-4xl font-semibold tracking-[-.04em] text-ink">{{ tokens.length }}</div>
          <p class="mt-1 text-xs leading-5 text-soft">个客户端凭证正在由 APIRelay 管理。</p>
          <div class="mt-6 space-y-3 text-xs text-soft">
            <p><strong class="block text-ink">一次性明文</strong>创建后仅展示一次完整 Key。</p>
            <p><strong class="block text-ink">额度边界</strong>按凭证限制模型范围和总预算。</p>
          </div>
        </aside>

      <section class="sheet overflow-hidden">
        <div class="sheet-head">
          <span class="dim-title">令牌列表</span>
          <span class="text-xs text-soft">共 {{ tokens.length }} 个</span>
        </div>

        <div class="hidden md:block">
          <table class="table-eng table-fixed">
            <thead>
              <tr>
                <th class="w-[22%]">名称</th>
                <th class="w-[15%]">Key 前缀</th>
                <th class="w-[12%]">分组</th>
                <th class="w-[19%]">模型范围</th>
                <th class="w-[20%]">额度</th>
                <th class="w-[7%]">状态</th>
                <th class="w-[5%]"><span class="sr-only">操作</span></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="token in tokens" :key="token.id">
                <td>
                  <div class="break-words font-medium text-ink">{{ token.name }}</div>
                  <div class="mt-0.5 font-mono text-xs text-soft">#{{ token.id }}</div>
                </td>
                <td>
                  <span class="break-all font-mono text-xs text-ink" title="完整 Key 仅在创建后显示一次">{{ keyPrefix(token) }}</span>
                </td>
                <td><span class="chip chip-blue">{{ token.group || 'default' }}</span></td>
                <td>
                  <div class="break-words font-mono text-xs text-ink" :title="modelScope(token)">{{ modelScope(token) }}</div>
                </td>
                <td>
                  <div class="font-mono text-xs tabular-nums text-ink">{{ quotaDisplay(token) }}</div>
                  <div v-if="!token.unlimited" class="quota-track mt-2" :aria-label="`额度已使用 ${quotaPercent(token).toFixed(1)}%`" role="img">
                    <div class="quota-fill" :style="{ width: `${quotaPercent(token)}%` }"></div>
                  </div>
                  <div v-else class="mt-1 text-xs text-soft">无上限</div>
                </td>
                <td>
                  <span class="chip" :class="token.status === 1 ? 'chip-run' : 'chip-trip'">
                    {{ token.status === 1 ? '启用' : '停用' }}
                  </span>
                </td>
                <td class="text-right">
                  <button
                    class="btn btn-danger btn-sm"
                    :disabled="deletingId === token.id"
                    :aria-label="`删除令牌 ${token.name}`"
                    @click="removeToken(token)"
                  >
                    {{ deletingId === token.id ? '删除中' : '删除' }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="space-y-3 p-3 md:hidden">
          <article v-for="token in tokens" :key="token.id" class="mobile-card">
            <div class="flex min-w-0 items-start justify-between gap-3">
              <div class="min-w-0">
                <h2 class="break-words font-medium text-ink">{{ token.name }}</h2>
                <div class="mt-1 font-mono text-xs text-soft">#{{ token.id }}</div>
              </div>
              <span class="chip shrink-0" :class="token.status === 1 ? 'chip-run' : 'chip-trip'">
                {{ token.status === 1 ? '启用' : '停用' }}
              </span>
            </div>

            <dl class="mt-4 space-y-3 border-t border-line pt-4">
              <div class="mobile-kv">
                <dt>Key 前缀</dt>
                <dd class="break-all font-mono text-xs">{{ keyPrefix(token) }}</dd>
              </div>
              <div class="mobile-kv">
                <dt>分组</dt>
                <dd><span class="chip chip-blue">{{ token.group || 'default' }}</span></dd>
              </div>
              <div class="mobile-kv">
                <dt>模型范围</dt>
                <dd class="break-words font-mono text-xs">{{ modelScope(token) }}</dd>
              </div>
              <div>
                <div class="flex items-center justify-between gap-3 text-sm">
                  <span class="text-soft">额度</span>
                  <span class="text-right font-mono text-xs tabular-nums text-ink">{{ quotaDisplay(token) }}</span>
                </div>
                <div v-if="!token.unlimited" class="quota-track mt-2" :aria-label="`额度已使用 ${quotaPercent(token).toFixed(1)}%`" role="img">
                  <div class="quota-fill" :style="{ width: `${quotaPercent(token)}%` }"></div>
                </div>
              </div>
            </dl>

            <div class="mt-4 border-t border-line pt-3 text-right">
              <button
                class="btn btn-danger btn-sm"
                :disabled="deletingId === token.id"
                :aria-label="`删除令牌 ${token.name}`"
                @click="removeToken(token)"
              >
                {{ deletingId === token.id ? '删除中' : '删除令牌' }}
              </button>
            </div>
          </article>
        </div>
      </section>
      </div>
    </PageState>

    <Modal :open="createOpen" title="创建 API 令牌" width="max-w-lg" @close="closeCreate">
      <div class="space-y-4">
        <label>
          <span class="field-label">名称</span>
          <input v-model="form.name" class="input" placeholder="例：测试环境" autocomplete="off" data-autofocus />
        </label>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <label>
            <span class="field-label">分组</span>
            <input v-model="form.group" class="input input-mono" placeholder="default" autocomplete="off" />
          </label>
          <label>
            <span class="field-label">额度模式</span>
            <span class="flex min-h-10 items-center gap-2 rounded-lg border border-line bg-white px-3">
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
        <div v-if="formError" class="rounded-lg border border-trip/30 bg-trip-wash px-3 py-2 text-sm text-trip" role="alert">
          {{ formError }}
        </div>
      </div>
      <template #footer>
        <button class="btn" :disabled="saving" aria-label="取消创建令牌" @click="closeCreate">取消</button>
        <button class="btn btn-primary" :disabled="saving" aria-label="创建令牌" @click="save">
          {{ saving ? '创建中…' : '创建令牌' }}
        </button>
      </template>
    </Modal>

    <Modal :open="secretOpen" title="一次性 API Key" width="max-w-2xl" persistent @close="keepSecretOpen">
      <div class="space-y-4">
        <div class="rounded-lg border border-trip/30 bg-trip-wash px-4 py-3 text-sm text-trip" role="alert">
          完整 Key 仅显示一次。关闭后无法再次查看，请立即保存到安全位置。
        </div>
        <label>
          <span class="field-label">完整 API Key</span>
          <textarea class="input input-mono min-h-24 resize-none break-all" :value="secretKey" readonly aria-label="完整一次性 API Key"></textarea>
        </label>
        <button class="btn w-full" aria-label="复制完整 API Key" @click="copySecret">
          {{ copied ? '已复制完整 Key' : '复制完整 Key' }}
        </button>
        <p class="text-xs leading-5 text-soft">若浏览器不允许自动复制，请手动选择上方内容并复制。</p>
        <label class="flex items-start gap-3 rounded-xl border border-line bg-panel/60 px-4 py-3 text-sm text-ink">
          <input v-model="secretAcknowledged" class="mt-1" type="checkbox" />
          <span>我已将完整 Key 保存到安全位置，并理解关闭后无法恢复。</span>
        </label>
      </div>
      <template #footer>
        <span class="mr-auto text-xs text-soft">点击确认后，页面将立即清除明文。</span>
        <button class="btn btn-primary" :disabled="!secretAcknowledged" aria-label="确认已保存一次性 API Key" @click="confirmSecretSaved">我已保存并清除</button>
      </template>
    </Modal>
  </div>
</template>

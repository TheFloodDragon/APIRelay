<template>
  <div>
    <div class="flex items-center justify-between mb-6">
      <div>
        <h2 class="page-title">供应商</h2>
        <p class="page-subtitle">配置上游 AI 服务、模型与协议 · 拖动卡片调整优先级</p>
      </div>
      <button class="btn-primary" @click="openCreate">
        <svg viewBox="0 0 24 24" class="w-4 h-4" fill="currentColor"><path d="M11 11V5h2v6h6v2h-6v6h-2v-6H5v-2z"/></svg>
        <span>新建供应商</span>
      </button>
    </div>

    <!-- 提示条 -->
    <div v-if="channels.length > 1" class="flex items-center gap-2 mb-4 text-xs text-ink-500 dark:text-ink-400">
      <svg viewBox="0 0 24 24" class="w-4 h-4 shrink-0" fill="currentColor"><path d="M11 18a2 2 0 11-4 0 2 2 0 014 0zm0-6a2 2 0 11-4 0 2 2 0 014 0zm0-6a2 2 0 11-4 0 2 2 0 014 0zm6 12a2 2 0 11-4 0 2 2 0 014 0zm0-6a2 2 0 11-4 0 2 2 0 014 0zm0-6a2 2 0 11-4 0 2 2 0 014 0z"/></svg>
      <span>排在越上方优先级越高，请求会优先路由到靠前的供应商；同优先级按权重分配。</span>
      <span v-if="reordering" class="badge-brand ml-1">保存中…</span>
    </div>

    <!-- 供应商卡片列表（可拖动排序） -->
    <div class="space-y-3">
      <div
        v-for="(ch, idx) in channels" :key="ch.id"
        class="group card-flat flex items-center gap-4 !py-3.5 transition-all"
        :class="[
          dragIndex === idx ? 'opacity-40' : '',
          dropIndex === idx && dragIndex !== null && dragIndex !== idx ? 'ring-2 ring-brand-400 ring-offset-2 ring-offset-ink-50 dark:ring-offset-ink-950' : '',
          ch.status !== 1 ? 'opacity-70' : '',
        ]"
        draggable="true"
        @dragstart="onDragStart(idx, $event)"
        @dragover.prevent="onDragOver(idx)"
        @drop="onDrop(idx)"
        @dragend="onDragEnd"
      >
        <!-- 拖动手柄 -->
        <div class="cursor-grab active:cursor-grabbing text-ink-300 dark:text-ink-600 hover:text-ink-500 dark:hover:text-ink-400 shrink-0 select-none" title="拖动排序">
          <svg viewBox="0 0 24 24" class="w-5 h-5" fill="currentColor"><path d="M9 5a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0zm0 7a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0zm0 7a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0zm9-14a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0zm0 7a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0zm0 7a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0z"/></svg>
        </div>

        <!-- 排名徽标 -->
        <div class="shrink-0 w-8 h-8 rounded-lg flex items-center justify-center text-sm font-bold"
          :class="idx === 0 ? 'bg-brand-gradient text-white shadow-glow' : 'bg-ink-100 dark:bg-ink-800 text-ink-500 dark:text-ink-400'">
          {{ idx + 1 }}
        </div>

        <!-- 主信息 -->
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2 flex-wrap">
            <span class="font-semibold text-ink-900 dark:text-ink-100 truncate">{{ ch.name }}</span>
            <span class="badge-brand">{{ typeName(ch.type) }}</span>
            <span class="badge-neutral">{{ ch.group }}</span>
            <span v-if="ch.status === 1" class="badge-success"><span class="w-1.5 h-1.5 rounded-full bg-green-500"></span>启用</span>
            <span v-else class="badge-error">禁用</span>
          </div>
          <div class="flex items-center gap-3 mt-1.5 text-xs text-ink-400 dark:text-ink-500">
            <span class="font-mono">#{{ ch.id }}</span>
            <span class="truncate max-w-[260px]" :title="ch.base_url">{{ ch.base_url || '默认地址' }}</span>
            <span class="inline-flex items-center gap-1">
              <svg viewBox="0 0 24 24" class="w-3.5 h-3.5" fill="currentColor"><path d="M12 2l9 5v10l-9 5-9-5V7l9-5zm0 2.3L5 8v8l7 3.9 7-3.9V8l-7-3.7z"/></svg>
              {{ modelCount(ch) }} 模型
            </span>
          </div>
        </div>

        <!-- 权重 -->
        <div class="shrink-0 text-center hidden sm:block">
          <div class="text-xs text-ink-400">权重</div>
          <div class="text-sm font-semibold text-ink-700 dark:text-ink-300">{{ ch.weight }}</div>
        </div>

        <!-- 操作 -->
        <div class="shrink-0 flex gap-2">
          <button @click="openEdit(ch)" class="btn-ghost btn-sm">编辑</button>
          <button @click="remove(ch)" class="btn-danger btn-sm">删除</button>
        </div>
      </div>

      <div v-if="!channels.length" class="empty-state card-flat">
        <div class="text-5xl mb-3 opacity-60">🔗</div>
        <div>暂无供应商，点击右上角新建</div>
      </div>
    </div>

    <!-- 编辑/创建弹窗 -->
    <div v-if="showModal" class="modal-backdrop" @click.self="showModal=false">
      <div class="modal max-w-3xl">
        <div class="flex items-center justify-between mb-5 pb-4 border-b border-ink-100 dark:border-ink-800">
          <h3 class="text-lg font-semibold text-ink-900 dark:text-ink-100">{{ form.id ? '编辑供应商' : '新建供应商' }}</h3>
          <button @click="showModal=false" class="text-ink-400 hover:text-ink-600 dark:hover:text-ink-200 text-2xl leading-none">&times;</button>
        </div>

        <div class="space-y-5">
          <!-- 基本信息 -->
          <!-- 阻止浏览器把 API Key 当作登录密码弹出"保存账号密码"：
               不使用 type=password，改用可切换显隐的文本框 + autocomplete=off。 -->
          <div class="grid grid-cols-2 gap-4" autocomplete="off">
            <div class="col-span-2">
              <label class="label">供应商名称 <span class="text-red-500">*</span></label>
              <input v-model="form.name" class="input" placeholder="例：OpenAI 主账号" autocomplete="off" />
            </div>
            <div>
              <label class="label">默认协议 <span class="text-red-500">*</span></label>
              <select v-model.number="form.type" class="input" @change="onTypeChange">
                <option v-for="t in channelTypes" :key="t.value" :value="t.value">{{ t.name }}</option>
              </select>
            </div>
            <div>
              <label class="label">分组</label>
              <input v-model="form.group" class="input" placeholder="default" autocomplete="off" />
            </div>
            <div class="col-span-2">
              <label class="label">Base URL <span class="text-red-500">*</span></label>
              <input v-model="form.base_url" class="input" placeholder="https://api.openai.com" autocomplete="off" />
            </div>
            <div class="col-span-2">
              <label class="label">API Key <span class="text-red-500">*</span></label>
              <div class="relative">
                <input
                  v-model="form.key"
                  type="text"
                  class="input pr-10"
                  :class="{ 'key-mask': !showKey }"
                  placeholder="上游密钥"
                  name="apirelay-upstream-key"
                  autocomplete="off"
                  autocapitalize="off"
                  autocorrect="off"
                  spellcheck="false"
                  data-1p-ignore
                  data-lpignore="true"
                  data-form-type="other"
                />
                <button
                  type="button"
                  class="absolute right-2 top-1/2 -translate-y-1/2 text-ink-400 hover:text-ink-600 dark:hover:text-ink-300 p-1"
                  :title="showKey ? '隐藏' : '显示'"
                  @click="showKey = !showKey"
                >
                  <svg v-if="showKey" viewBox="0 0 24 24" class="w-4 h-4" fill="currentColor"><path d="M12 6c3.79 0 7.17 2.13 8.82 5.5C19.17 14.87 15.79 17 12 17s-7.17-2.13-8.82-5.5C4.83 8.13 8.21 6 12 6zm0 2a3.5 3.5 0 100 7 3.5 3.5 0 000-7zm0 1.5a2 2 0 110 4 2 2 0 010-4z"/></svg>
                  <svg v-else viewBox="0 0 24 24" class="w-4 h-4" fill="currentColor"><path d="M2 5.27L3.28 4 20 20.72 18.73 22l-3.08-3.08A11 11 0 0112 19c-5 0-9.27-3.11-11-7.5a11.8 11.8 0 014.17-5.06L2 5.27zM12 9a3 3 0 012.83 4L12 9zm0-3c5 0 9.27 3.11 11 7.5a11.8 11.8 0 01-2.18 3.36l-1.42-1.42A9.8 9.8 0 0020.82 13C19.17 9.63 15.79 7.5 12 7.5c-.74 0-1.46.09-2.16.26L8.36 6.28A11 11 0 0112 6z"/></svg>
                </button>
              </div>
              <p class="hint" v-if="form.id">留空表示不修改现有密钥。</p>
            </div>
          </div>

          <!-- 模型管理 -->
          <div class="surface p-4">
            <div class="flex items-center justify-between mb-3">
              <div>
                <label class="text-sm font-semibold text-ink-700 dark:text-ink-200">模型管理</label>
                <p class="hint">每个模型可单独启用、覆盖协议、映射上游名</p>
              </div>
              <button
                class="btn-secondary btn-sm"
                :disabled="probing || !form.base_url || !form.key"
                @click="fetchModels"
              >
                <span v-if="probing">⏳ 拉取中</span>
                <span v-else>🔄 拉取模型</span>
              </button>
            </div>

            <!-- 添加模型行 -->
            <div class="flex gap-2 mb-3">
              <input v-model="newModelName" class="input !py-2" placeholder="手动添加模型名（或填 * 通配）" @keyup.enter="addModel" />
              <button class="btn-secondary btn-sm shrink-0" @click="addModel">添加</button>
            </div>

            <div v-if="models.length" class="rounded-xl border border-ink-200 dark:border-ink-700 overflow-hidden">
              <div class="max-h-64 overflow-y-auto divide-y divide-ink-100 dark:divide-ink-800">
                <div v-for="(m, i) in models" :key="i"
                  class="flex flex-wrap items-center gap-2 px-3 py-2 bg-white dark:bg-ink-900/40 hover:bg-ink-50 dark:hover:bg-ink-800/40 transition-colors">
                  <!-- 启用开关 -->
                  <button type="button" class="toggle shrink-0" :class="{ 'toggle-on': m.enabled }" @click="m.enabled = !m.enabled">
                    <span class="toggle-knob"></span>
                  </button>
                  <!-- 模型名 -->
                  <input v-model="m.name" class="input !py-1.5 !rounded-lg font-mono text-xs flex-1 min-w-[120px]" placeholder="模型显示名" />
                  <!-- 协议覆盖 -->
                  <select v-model="m.protocol" class="input !py-1.5 !rounded-lg text-xs w-28 shrink-0">
                    <option value="">继承</option>
                    <option v-for="p in protocols" :key="p.value" :value="p.value">{{ p.name }}</option>
                  </select>
                  <!-- 上游名 -->
                  <input v-model="m.upstream" class="input !py-1.5 !rounded-lg font-mono text-xs w-32 shrink-0" placeholder="上游名(可选)" />
                  <!-- 价格 input/output（$/1M，留空=继承） -->
                  <input v-model.number="m.input" type="number" step="0.01" min="0" class="input !py-1.5 !rounded-lg text-xs w-20 shrink-0 text-right" placeholder="入价" title="输入价 $/1M（0=继承）" />
                  <input v-model.number="m.output" type="number" step="0.01" min="0" class="input !py-1.5 !rounded-lg text-xs w-20 shrink-0 text-right" placeholder="出价" title="输出价 $/1M（0=继承）" />
                  <button class="text-ink-300 hover:text-red-500 shrink-0 px-1" @click="models.splice(i, 1)">✕</button>
                </div>
              </div>
            </div>
            <div v-else class="text-center py-6 text-ink-400 text-sm">
              暂无模型，拉取或手动添加
            </div>
            <p class="hint mt-2">已启用 {{ enabledCount }} 个 · 协议「继承」走规则/供应商默认 · 价格留空（0）走全局价格表</p>
          </div>

          <!-- 协议规则 -->
          <details class="surface overflow-hidden" open>
            <summary class="px-4 py-3 cursor-pointer font-medium text-sm text-ink-700 dark:text-ink-200 select-none">
              🧭 供应商协议规则（正则）
            </summary>
            <div class="px-4 pb-4 space-y-2">
              <p class="hint mb-1">按显示名正则匹配，命中则用指定协议（优先级低于模型显式协议、高于供应商默认）</p>
              <div v-for="(r, i) in rules" :key="i" class="flex gap-2">
                <input v-model="r.pattern" class="input !py-2 font-mono text-xs flex-1" placeholder="正则，如 ^claude" />
                <select v-model="r.protocol" class="input !py-2 text-xs w-36 shrink-0">
                  <option v-for="p in protocols" :key="p.value" :value="p.value">{{ p.name }}</option>
                </select>
                <button class="text-ink-300 hover:text-red-500 px-1 shrink-0" @click="rules.splice(i, 1)">✕</button>
              </div>
              <button class="btn-ghost btn-sm" @click="rules.push({ pattern: '', protocol: 'anthropic' })">+ 添加规则</button>
            </div>
          </details>

          <!-- 高级配置 -->
          <details class="surface overflow-hidden">
            <summary class="px-4 py-3 cursor-pointer font-medium text-sm text-ink-700 dark:text-ink-200 select-none">
              ⚙️ 高级配置
            </summary>
            <div class="px-4 pb-4 space-y-4">
              <div>
                <label class="label">权重</label>
                <input v-model.number="form.weight" type="number" min="1" class="input" placeholder="1" />
                <p class="hint">同优先级下的负载分配比例。优先级请在列表中拖动调整。</p>
              </div>
              <div>
                <label class="label">请求头覆盖（JSON）</label>
                <input v-model="form.header_override" class="input font-mono text-xs" placeholder='{"User-Agent":"..."}' />
              </div>
            </div>
          </details>
        </div>

        <div class="mt-5 pt-4 border-t border-ink-100 dark:border-ink-800">
          <div v-if="err" class="mb-3 p-3 bg-red-50 dark:bg-red-500/10 border border-red-200 dark:border-red-500/30 rounded-xl text-sm text-red-600 dark:text-red-400">
            ⚠️ {{ err }}
          </div>
          <div class="flex justify-end gap-3">
            <button @click="showModal=false" class="btn-secondary">取消</button>
            <button class="btn-primary" :disabled="saving || !canSave" @click="save">
              {{ saving ? '保存中...' : '保存' }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useToast } from '../composables/useToast'
import api from '../api'

const toast = useToast()
const channels = ref([])
const channelTypes = ref([])
const protocols = ref([])
const showModal = ref(false)
const probing = ref(false)
const saving = ref(false)
const err = ref('')
const showKey = ref(false)

const models = ref([])   // [{name,enabled,protocol,upstream}]
const rules = ref([])    // [{pattern,protocol}]
const newModelName = ref('')

// 拖动排序状态
const dragIndex = ref(null)
const dropIndex = ref(null)
const reordering = ref(false)

const blank = () => ({
  id: 0, name: '', type: 1, base_url: '', key: '', group: 'default',
  header_override: '', priority: 0, weight: 1, status: 1,
})
const form = ref(blank())

const enabledCount = computed(() => models.value.filter(m => m.enabled && m.name.trim()).length)
// 编辑时密钥可留空（表示不修改）；新建时必填
const canSave = computed(() =>
  form.value.name &&
  form.value.base_url &&
  (form.value.id ? true : form.value.key) &&
  enabledCount.value > 0
)

function typeName(t) {
  const f = channelTypes.value.find(x => x.value === t)
  return f ? f.name : t
}
function modelCount(ch) {
  if (Array.isArray(ch._models)) return ch._models.length
  return (ch.models || '').split(',').map(s => s.trim()).filter(Boolean).length
}

async function load() {
  try {
    channels.value = (await api.get('/channels')) || []
  } catch (e) {
    toast.error('加载失败: ' + e.message)
  }
}
async function loadMeta() {
  try {
    const [types, protos] = await Promise.all([api.get('/channel-types'), api.get('/protocols')])
    channelTypes.value = types || []
    protocols.value = protos || []
  } catch (e) {
    toast.error('加载元数据失败: ' + e.message)
  }
}

function parseModels(ch) {
  // 优先 model_configs，回退 models 字符串
  if (ch.model_configs) {
    try {
      const arr = JSON.parse(ch.model_configs)
      if (Array.isArray(arr)) return arr.map(m => ({ name: m.name || '', enabled: m.enabled !== false, protocol: m.protocol || '', upstream: m.upstream || '', input: m.input || 0, output: m.output || 0 }))
    } catch {}
  }
  return (ch.models || '').split(',').map(s => s.trim()).filter(Boolean)
    .map(n => ({ name: n, enabled: true, protocol: '', upstream: '', input: 0, output: 0 }))
}
function parseRules(ch) {
  if (ch.protocol_rules) {
    try {
      const arr = JSON.parse(ch.protocol_rules)
      if (Array.isArray(arr)) return arr.map(r => ({ pattern: r.pattern || '', protocol: r.protocol || 'anthropic' }))
    } catch {}
  }
  return []
}

function openCreate() {
  form.value = blank()
  models.value = []
  rules.value = []
  err.value = ''
  showKey.value = false
  const t = channelTypes.value.find(x => x.value === form.value.type)
  if (t) form.value.base_url = t.default_base_url
  showModal.value = true
}
function openEdit(ch) {
  form.value = { ...blank(), ...ch }
  // 出于安全，编辑时不回填密钥；留空表示不修改
  form.value.key = ''
  models.value = parseModels(ch)
  rules.value = parseRules(ch)
  err.value = ''
  showKey.value = false
  showModal.value = true
}
function onTypeChange() {
  const t = channelTypes.value.find(x => x.value === form.value.type)
  if (t && !form.value.base_url) form.value.base_url = t.default_base_url
}

function addModel() {
  const n = newModelName.value.trim()
  if (!n) return
  if (models.value.some(m => m.name === n)) {
    toast.warning('模型已存在')
    return
  }
  models.value.unshift({ name: n, enabled: true, protocol: '', upstream: '', input: 0, output: 0 })
  newModelName.value = ''
}

async function fetchModels() {
  if (!form.value.base_url || !form.value.key) {
    err.value = '请先填写 Base URL 和 API Key'
    return
  }
  err.value = ''
  probing.value = true
  try {
    const data = await api.post('/channels/probe-models', {
      type: form.value.type, base_url: form.value.base_url, key: form.value.key,
    })
    const fetched = data.models || []
    const existing = new Set(models.value.map(m => m.name))
    let added = 0
    for (const name of fetched) {
      if (!existing.has(name)) {
        models.value.push({ name, enabled: true, protocol: '', upstream: '', input: 0, output: 0 })
        added++
      }
    }
    toast.success(`拉取到 ${fetched.length} 个模型，新增 ${added} 个`)
  } catch (e) {
    err.value = '拉取失败: ' + (e.message || '网络错误')
    toast.error(err.value)
  } finally {
    probing.value = false
  }
}

async function save() {
  if (!canSave.value) {
    err.value = '请填写必填项并至少启用一个模型'
    return
  }
  err.value = ''
  saving.value = true
  const cleanModels = models.value.filter(m => m.name.trim()).map(m => ({
    name: m.name.trim(),
    enabled: !!m.enabled,
    protocol: m.protocol || '',
    upstream: m.upstream || '',
    input: Number(m.input) || 0,
    output: Number(m.output) || 0,
  }))
  const cleanRules = rules.value.filter(r => r.pattern.trim() && r.protocol)
  const payload = {
    ...form.value,
    model_configs: JSON.stringify(cleanModels),
    protocol_rules: JSON.stringify(cleanRules),
    models: cleanModels.filter(m => m.enabled).map(m => m.name).join(','),
  }
  // 编辑时密钥留空表示不修改，删除该字段避免覆盖为空
  if (form.value.id && !form.value.key) {
    delete payload.key
  }
  try {
    if (form.value.id) {
      await api.put(`/channels/${form.value.id}`, payload)
      toast.success('供应商已更新')
    } else {
      await api.post('/channels', payload)
      toast.success('供应商已创建')
    }
    showModal.value = false
    await load()
  } catch (e) {
    err.value = e.message || '保存失败'
    toast.error(err.value)
  } finally {
    saving.value = false
  }
}

async function remove(ch) {
  if (!confirm(`确认删除供应商「${ch.name}」？\n\n此操作不可撤销。`)) return
  try {
    await api.delete(`/channels/${ch.id}`)
    toast.success('已删除')
    await load()
  } catch (e) {
    toast.error('删除失败: ' + e.message)
  }
}

// ---- 拖动排序 ----
function onDragStart(idx, e) {
  dragIndex.value = idx
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', String(idx)) // Firefox 需要
  }
}
function onDragOver(idx) {
  dropIndex.value = idx
}
function onDrop(idx) {
  const from = dragIndex.value
  if (from === null || from === idx) return
  const list = channels.value.slice()
  const [moved] = list.splice(from, 1)
  list.splice(idx, 0, moved)
  channels.value = list
  persistOrder()
}
function onDragEnd() {
  dragIndex.value = null
  dropIndex.value = null
}

async function persistOrder() {
  reordering.value = true
  try {
    await api.post('/channels/reorder', { ids: channels.value.map(c => c.id) })
    toast.success('优先级已更新')
    await load() // 重新拉取以同步后端计算的 priority 值
  } catch (e) {
    toast.error('排序保存失败: ' + e.message)
    await load() // 回滚到服务端状态
  } finally {
    reordering.value = false
  }
}

onMounted(async () => {
  await loadMeta()
  await load()
})
</script>

<style scoped>
/* 用文本框模拟密码遮罩，避免浏览器把 API Key 当登录密码弹出保存提示 */
.key-mask {
  -webkit-text-security: disc;
  text-security: disc;
  font-family: text-security-disc, ui-monospace, monospace;
}
</style>

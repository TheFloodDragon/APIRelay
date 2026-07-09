<template>
  <div>
    <div class="flex items-center justify-between mb-5">
      <div>
        <h2 class="page-title">渠道</h2>
        <p class="page-subtitle">上游渠道的优先级、权重与模型协议配置</p>
      </div>
      <button class="btn-primary" @click="openCreate">
        <svg viewBox="0 0 24 24" class="w-4 h-4" fill="currentColor"><path d="M11 11V5h2v6h6v2h-6v6h-2v-6H5v-2z"/></svg>
        <span>新建渠道</span>
      </button>
    </div>

    <div class="panel overflow-hidden">
      <div class="h-10 px-4 border-b border-line flex items-center justify-between">
        <div class="flex items-center gap-2">
          <span class="font-mono text-sm font-medium text-t1">渠道机架</span>
          <span class="tick">拖动排序</span>
        </div>
        <span v-if="reordering" class="badge badge-signal">保存中…</span>
      </div>

      <div v-if="channels.length" class="divide-y divide-line">
        <div
          v-for="(ch, idx) in sortedChannels" :key="ch.id"
          class="group grid grid-cols-[40px_54px_minmax(0,1fr)_82px_66px_120px] max-lg:grid-cols-[40px_54px_minmax(0,1fr)] gap-3 items-start px-4 py-3 transition-colors hover:bg-[rgb(var(--brass)/0.04)]"
          :class="[
            dragIndex === idx ? 'opacity-40' : '',
            dropIndex === idx && dragIndex !== null && dragIndex !== idx ? 'ring-1 ring-brass' : '',
            ch.status !== 1 ? 'opacity-60' : '',
          ]"
          draggable="true"
          @dragstart="onDragStart(idx, $event)"
          @dragover.prevent="onDragOver(idx)"
          @drop="onDrop(idx)"
          @dragend="onDragEnd"
        >
          <!-- 拖拽手柄 -->
          <button class="cursor-grab active:cursor-grabbing text-t3 hover:text-t1 transition-colors" title="拖动排序">
            <svg viewBox="0 0 24 24" class="w-4 h-4" fill="currentColor"><path d="M9 5a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0zm0 7a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0zm0 7a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0zm9-14a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0zm0 7a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0zm0 7a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0z"/></svg>
          </button>

          <!-- 序位 -->
          <div class="flex items-center gap-2">
            <span class="font-mono text-sm font-semibold" :class="idx === 0 ? 'text-brass' : 'text-t3'">{{ String(idx + 1).padStart(2, '0') }}</span>
            <PriorityBar :level="idx" :total="channels.length" />
          </div>

          <!-- 主信息 -->
          <div class="min-w-0">
            <div class="flex items-center gap-2 min-w-0">
              <SignalDot :status="stateOf(ch)" />
              <span class="font-semibold text-sm text-t1 truncate">{{ ch.name }}</span>
              <span class="badge badge-signal shrink-0">{{ typeName(ch.type) }}</span>
              <span class="badge badge-neutral shrink-0">{{ ch.group || 'default' }}</span>
            </div>
            <div class="mt-1 flex items-center gap-3 text-2xs text-t3 font-mono min-w-0">
              <span>#{{ ch.id }}</span>
              <span class="truncate" :title="ch.base_url">{{ ch.base_url || 'default-url' }}</span>
            </div>
            <div class="mt-2 flex items-start gap-2 min-w-0">
              <span class="tick shrink-0 pt-1">KEY</span>
              <span class="key-chip key-chip-full flex-1 min-w-0">
                <code :title="ch.key">{{ ch.key || '未配置' }}</code>
              </span>
              <button class="btn-secondary btn-sm !px-2 !py-1 shrink-0" :disabled="!ch.key" @click.stop="copyKey(ch)">复制</button>
            </div>
          </div>

          <!-- 模型数 -->
          <div class="text-right">
            <div class="font-mono text-sm text-t1">{{ modelCount(ch) }}</div>
            <div class="tick">models</div>
          </div>

          <!-- 权重 -->
          <div class="text-right max-lg:hidden">
            <div class="font-mono text-sm text-t1">×{{ ch.weight }}</div>
            <div class="tick">weight</div>
          </div>

          <!-- 操作 -->
          <div class="flex justify-end gap-2 max-lg:hidden">
            <button @click="resetBreaker(ch)" class="btn-ghost btn-sm" title="重置熔断器">
              <svg viewBox="0 0 24 24" class="w-3.5 h-3.5" fill="currentColor"><path d="M12 5V2L8 6l4 4V7c3.31 0 6 2.69 6 6 0 2.97-2.17 5.43-5 5.91v2.02c3.95-.49 7-3.85 7-7.93 0-4.42-3.58-8-8-8zm-6 8c0-2.97 2.17-5.43 5-5.91V5.07C7.05 5.56 4 8.92 4 13c0 4.42 3.58 8 8 8v-3l4 4-4 4v-3c-4.42 0-8-3.58-8-8z"/></svg>
            </button>
            <button @click="openEdit(ch)" class="btn-ghost btn-sm">配置</button>
            <button @click="remove(ch)" class="btn-danger btn-sm">删除</button>
          </div>
        </div>
      </div>

      <div v-if="!channels.length" class="empty-state">
        <span class="font-mono text-3xl text-t3">∅</span>
        <span>暂无渠道，点击右上角新建</span>
      </div>
    </div>

    <!-- 编辑/创建弹窗 -->
    <div v-if="showModal" class="modal-backdrop" @click.self="showModal=false">
      <div class="modal max-w-4xl">
        <div class="modal-header">
          <div>
            <h3 class="modal-title">{{ form.id ? '配置渠道' : '新建渠道' }}</h3>
            <p class="hint mt-0">模型、协议规则与上游凭据集中在此面板配置</p>
          </div>
          <button @click="showModal=false" class="text-t3 hover:text-t1 text-xl leading-none">×</button>
        </div>

        <div class="grid grid-cols-1 lg:grid-cols-[1fr_1.2fr] gap-4">
          <!-- 左：节点基础 -->
          <section class="space-y-4">
            <div class="panel p-4 space-y-3">
              <div class="flex items-center justify-between mb-1">
                <span class="font-mono text-sm text-t1">渠道参数</span>
                <span class="tick">CHANNEL</span>
              </div>
              <div>
                <label class="label">渠道名称 <span class="text-[rgb(var(--rust))]">*</span></label>
                <input v-model="form.name" class="input" placeholder="例：OpenAI 主账号" autocomplete="off" />
              </div>
              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label class="label">默认协议 <span class="text-[rgb(var(--rust))]">*</span></label>
                  <select v-model.number="form.type" class="input" @change="onTypeChange">
                    <option v-for="t in channelTypes" :key="t.value" :value="t.value">{{ t.name }}</option>
                  </select>
                </div>
                <div>
                  <label class="label">分组</label>
                  <input v-model="form.group" class="input font-mono" placeholder="default" autocomplete="off" />
                </div>
              </div>
              <div>
                <label class="label">Base URL <span class="text-[rgb(var(--rust))]">*</span></label>
                <input v-model="form.base_url" class="input font-mono" placeholder="https://api.openai.com" autocomplete="off" />
              </div>
              <div>
                <label class="label">API Key <span class="text-[rgb(var(--rust))]">*</span></label>
                <input
                  v-model="form.key"
                  type="text"
                  class="input font-mono"
                  placeholder="upstream-key"
                  name="apirelay-upstream-key"
                  autocomplete="off"
                  autocapitalize="off"
                  autocorrect="off"
                  spellcheck="false"
                  data-1p-ignore
                  data-lpignore="true"
                  data-form-type="other"
                />
                <p class="hint">上游站点 Key 会按明文保存并在渠道列表中直接显示。</p>
              </div>
            </div>

            <details class="panel overflow-hidden">
              <summary class="px-4 py-3 cursor-pointer font-medium text-sm text-t1 select-none flex items-center justify-between">
                <span>高级参数</span><span class="tick">ADVANCED</span>
              </summary>
              <div class="px-4 pb-4 space-y-3 border-t border-line">
                <div>
                  <label class="label">权重</label>
                  <input v-model.number="form.weight" type="number" min="1" class="input font-mono" placeholder="1" />
                  <p class="hint">同优先级下的负载分配比例。优先级请在列表中拖动调整。</p>
                </div>
                <div>
                  <label class="label">请求头覆盖（JSON）</label>
                  <input v-model="form.header_override" class="input font-mono text-xs" placeholder='{"User-Agent":"..."}' />
                </div>
              </div>
            </details>
          </section>

          <!-- 右：模型和规则 -->
          <section class="space-y-4">
            <div class="panel p-4">
              <div class="flex items-start justify-between mb-3">
                <div>
                  <div class="font-mono text-sm text-t1">模型映射</div>
                  <p class="hint mt-0">每个模型可单独启用、覆盖协议、映射上游名</p>
                </div>
                <button class="btn-secondary btn-sm" :disabled="probing || !form.base_url || !form.key" @click="fetchModels">
                  {{ probing ? '拉取中…' : '拉取模型' }}
                </button>
              </div>

              <div class="flex gap-2 mb-3">
                <input v-model="newModelName" class="input !py-1.5 font-mono" placeholder="模型名（或 * 通配）" @keyup.enter="addModel" />
                <button class="btn-secondary btn-sm shrink-0" @click="addModel">添加</button>
              </div>

              <div v-if="models.length" class="border border-line rounded-lg overflow-hidden">
                <div class="max-h-80 overflow-y-auto divide-y divide-line">
                  <div v-for="(m, i) in models" :key="i" class="p-2 bg-panel hover:bg-[rgb(var(--brass)/0.04)] transition-colors">
                    <div class="grid grid-cols-[40px_minmax(120px,1fr)_104px_118px_70px_70px_40px_24px] max-xl:grid-cols-[40px_minmax(120px,1fr)_104px_40px_24px] gap-2 items-center">
                      <button type="button" class="toggle shrink-0" :class="{ 'toggle-on': m.enabled }" @click="m.enabled = !m.enabled"><span class="toggle-knob"></span></button>
                      <input v-model="m.name" class="input !py-1.5 !rounded-md font-mono text-xs" placeholder="显示名" />
                      <select v-model="m.protocol" class="input !py-1.5 !rounded-md text-xs">
                        <option value="">继承</option>
                        <option v-for="p in protocols" :key="p.value" :value="p.value">{{ p.name }}</option>
                      </select>
                      <input v-model="m.upstream" class="input !py-1.5 !rounded-md font-mono text-xs max-xl:hidden" placeholder="上游名" />
                      <input v-model.number="m.input" type="number" step="0.01" min="0" class="input !py-1.5 !rounded-md text-xs text-right max-xl:hidden" placeholder="入价" title="输入价 $/1M" />
                      <input v-model.number="m.output" type="number" step="0.01" min="0" class="input !py-1.5 !rounded-md text-xs text-right max-xl:hidden" placeholder="出价" title="输出价 $/1M" />
                      <button type="button" class="btn-secondary btn-sm !px-2" :disabled="testing[m.name] || !m.name.trim()" @click="testModel(m)" title="发送一次测试对话">{{ testing[m.name] ? '…' : '测' }}</button>
                      <button class="text-t3 hover:text-[rgb(var(--rust))]" @click="removeModel(i, m)">×</button>
                    </div>
                    <div v-if="testResults[m.name]" class="mt-2 ml-10 text-xs rounded-md px-2 py-1 border"
                      :class="testResults[m.name].success ? 'text-[rgb(var(--jade))] border-[rgb(var(--jade)/0.28)] bg-[rgb(var(--jade)/0.08)]' : 'text-[rgb(var(--rust))] border-[rgb(var(--rust)/0.28)] bg-[rgb(var(--rust)/0.08)]'">
                      <template v-if="testResults[m.name].success">
                        ✓ {{ testResults[m.name].latency_ms }}ms · {{ testResults[m.name].protocol }}
                        <span v-if="testResults[m.name].reply" class="text-t3"> · {{ testResults[m.name].reply }}</span>
                      </template>
                      <template v-else>✕ {{ testResults[m.name].error }}</template>
                    </div>
                  </div>
                </div>
              </div>
              <div v-else class="empty-state !py-8 border border-line rounded-lg bg-panel-2">
                <span>暂无模型，拉取或手动添加</span>
              </div>
              <p class="hint">已启用 {{ enabledCount }} 个 · 协议「继承」走规则/渠道默认 · 价格 0 走全局价格表</p>
            </div>

            <details class="panel overflow-hidden" open>
              <summary class="px-4 py-3 cursor-pointer font-medium text-sm text-t1 select-none flex items-center justify-between">
                <span>渠道协议规则</span><span class="tick">REGEX</span>
              </summary>
              <div class="px-4 pb-4 pt-3 space-y-2 border-t border-line">
                <p class="hint mt-0">按显示名正则匹配，命中则用指定协议（优先级低于模型显式协议）</p>
                <div v-for="(r, i) in rules" :key="i" class="flex gap-2">
                  <input v-model="r.pattern" class="input !py-1.5 font-mono text-xs flex-1" placeholder="^claude" />
                  <select v-model="r.protocol" class="input !py-1.5 text-xs w-32 shrink-0">
                    <option v-for="p in protocols" :key="p.value" :value="p.value">{{ p.name }}</option>
                  </select>
                  <button class="text-t3 hover:text-[rgb(var(--rust))] px-1" @click="rules.splice(i, 1)">×</button>
                </div>
                <button class="btn-ghost btn-sm" @click="rules.push({ pattern: '', protocol: 'anthropic' })">+ 添加规则</button>
              </div>
            </details>
          </section>
        </div>

        <div class="mt-4 pt-4 border-t border-line">
          <div v-if="err" class="mb-3 p-3 rounded-lg text-sm border text-[rgb(var(--rust))] border-[rgb(var(--rust)/0.28)] bg-[rgb(var(--rust)/0.08)]">{{ err }}</div>
          <div class="flex justify-end gap-2">
            <button @click="showModal=false" class="btn-secondary">取消</button>
            <button class="btn-primary" :disabled="saving || !canSave" @click="save">{{ saving ? '保存中…' : '保存渠道' }}</button>
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
import SignalDot from '../components/SignalDot.vue'
import PriorityBar from '../components/PriorityBar.vue'

const toast = useToast()
const channels = ref([])
const channelTypes = ref([])
const protocols = ref([])
const showModal = ref(false)
const probing = ref(false)
const saving = ref(false)
const err = ref('')
const models = ref([])
const rules = ref([])
const newModelName = ref('')

const testing = ref({})
const testResults = ref({})

const dragIndex = ref(null)
const dropIndex = ref(null)
const reordering = ref(false)

const blank = () => ({
  id: 0, name: '', type: 1, base_url: '', key: '', group: 'default',
  header_override: '', priority: 0, weight: 1, status: 1,
})
const form = ref(blank())

const sortedChannels = computed(() => channels.value)
const enabledCount = computed(() => models.value.filter(m => m.enabled && m.name.trim()).length)
const canSave = computed(() =>
  form.value.name &&
  form.value.base_url &&
  String(form.value.key || '').trim() &&
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
function stateOf(ch) {
  if (ch.status !== 1) return 'down'
  if (ch.cooldown_until && ch.cooldown_until > Date.now()) return 'warn'
  return 'online'
}

async function copyKey(ch) {
  if (!ch.key) return
  try {
    await navigator.clipboard.writeText(ch.key)
    toast.success(`已复制「${ch.name}」的上游 Key`)
  } catch {
    toast.warning('浏览器阻止了剪贴板写入，请手动复制列表中的 Key')
  }
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
  testing.value = {}
  testResults.value = {}
  const t = channelTypes.value.find(x => x.value === form.value.type)
  if (t) form.value.base_url = t.default_base_url
  showModal.value = true
}
function openEdit(ch) {
  form.value = { ...blank(), ...ch, key: ch.key || '' }
  models.value = parseModels(ch)
  rules.value = parseRules(ch)
  err.value = ''
  testing.value = {}
  testResults.value = {}
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

function removeModel(i, m) {
  models.value.splice(i, 1)
  if (m && m.name) {
    delete testResults.value[m.name]
    delete testing.value[m.name]
  }
}

async function testModel(m) {
  const name = m.name.trim()
  if (!name) return
  if (!form.value.base_url) {
    toast.warning('请先填写 Base URL')
    return
  }
  if (!form.value.key) {
    toast.warning('请先填写 API Key')
    return
  }
  testing.value = { ...testing.value, [name]: true }
  try {
    const res = await api.post('/channels/test', {
      type: form.value.type,
      base_url: form.value.base_url,
      key: form.value.key,
      group: form.value.group || 'default',
      model_configs: JSON.stringify([{ name, enabled: true, protocol: m.protocol || '', upstream: m.upstream || '' }]),
      protocol_rules: JSON.stringify(rules.value.filter(r => r.pattern.trim() && r.protocol)),
      header_override: form.value.header_override || '',
      model: name,
    })
    testResults.value = { ...testResults.value, [name]: res }
    if (res.success) toast.success(`模型 ${name} 连通正常`)
    else toast.error(`模型 ${name} 测试失败`)
  } catch (e) {
    testResults.value = { ...testResults.value, [name]: { success: false, error: e.message || '请求失败' } }
    toast.error('测试失败: ' + e.message)
  } finally {
    testing.value = { ...testing.value, [name]: false }
  }
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
    err.value = '请填写渠道名称、Base URL、API Key，并至少启用一个模型'
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
  try {
    if (form.value.id) {
      await api.put(`/channels/${form.value.id}`, payload)
      toast.success('路由节点已更新')
    } else {
      await api.post('/channels', payload)
      toast.success('路由节点已创建')
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
  if (!confirm(`确认删除渠道「${ch.name}」？\n\n此操作不可撤销。`)) return
  try {
    await api.delete(`/channels/${ch.id}`)
    toast.success('已删除')
    await load()
  } catch (e) {
    toast.error('删除失败: ' + e.message)
  }
}

async function resetBreaker(ch) {
  try {
    await api.post(`/channels/${ch.id}/health/reset`)
    toast.success(`已重置渠道「${ch.name}」的熔断器`)
  } catch (e) {
    toast.error('重置失败: ' + e.message)
  }
}

function onDragStart(idx, e) {
  dragIndex.value = idx
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', String(idx))
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
    await load()
  } catch (e) {
    toast.error('排序保存失败: ' + e.message)
    await load()
  } finally {
    reordering.value = false
  }
}

onMounted(async () => {
  await loadMeta()
  await load()
})
</script>


<template>
  <div>
    <div class="flex items-center justify-between mb-6">
      <div>
        <h2 class="page-title">供应商</h2>
        <p class="page-subtitle">配置上游 AI 服务、模型与协议</p>
      </div>
      <button class="btn-primary" @click="openCreate">
        <svg viewBox="0 0 24 24" class="w-4 h-4" fill="currentColor"><path d="M11 11V5h2v6h6v2h-6v6h-2v-6H5v-2z"/></svg>
        <span>新建供应商</span>
      </button>
    </div>

    <div class="table-wrapper">
      <table class="table">
        <thead>
          <tr>
            <th>ID</th>
            <th>名称</th>
            <th>默认协议</th>
            <th>分组</th>
            <th>模型</th>
            <th>优先级</th>
            <th>权重</th>
            <th>状态</th>
            <th class="text-right">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="ch in channels" :key="ch.id">
            <td class="font-mono text-xs text-ink-400">#{{ ch.id }}</td>
            <td class="font-medium">{{ ch.name }}</td>
            <td><span class="badge-brand">{{ typeName(ch.type) }}</span></td>
            <td><span class="badge-neutral">{{ ch.group }}</span></td>
            <td>
              <span class="badge-info">{{ modelCount(ch) }} 个</span>
            </td>
            <td class="text-center text-ink-500">{{ ch.priority }}</td>
            <td class="text-center text-ink-500">{{ ch.weight }}</td>
            <td>
              <span v-if="ch.status === 1" class="badge-success"><span class="w-1.5 h-1.5 rounded-full bg-green-500"></span>启用</span>
              <span v-else class="badge-error">禁用</span>
            </td>
            <td>
              <div class="flex gap-2 justify-end">
                <button @click="openEdit(ch)" class="btn-ghost btn-sm">编辑</button>
                <button @click="remove(ch)" class="btn-danger btn-sm">删除</button>
              </div>
            </td>
          </tr>
          <tr v-if="!channels.length">
            <td colspan="9" class="empty-state">
              <div class="text-5xl mb-3 opacity-60">🔗</div>
              <div>暂无供应商，点击右上角新建</div>
            </td>
          </tr>
        </tbody>
      </table>
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
          <div class="grid grid-cols-2 gap-4">
            <div class="col-span-2">
              <label class="label">供应商名称 <span class="text-red-500">*</span></label>
              <input v-model="form.name" class="input" placeholder="例：OpenAI 主账号" />
            </div>
            <div>
              <label class="label">默认协议 <span class="text-red-500">*</span></label>
              <select v-model.number="form.type" class="input" @change="onTypeChange">
                <option v-for="t in channelTypes" :key="t.value" :value="t.value">{{ t.name }}</option>
              </select>
            </div>
            <div>
              <label class="label">分组</label>
              <input v-model="form.group" class="input" placeholder="default" />
            </div>
            <div class="col-span-2">
              <label class="label">Base URL <span class="text-red-500">*</span></label>
              <input v-model="form.base_url" class="input" placeholder="https://api.openai.com" />
            </div>
            <div class="col-span-2">
              <label class="label">API Key <span class="text-red-500">*</span></label>
              <input v-model="form.key" class="input" type="password" placeholder="上游密钥" />
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
                  class="flex items-center gap-2 px-3 py-2 bg-white dark:bg-ink-900/40 hover:bg-ink-50 dark:hover:bg-ink-800/40 transition-colors">
                  <!-- 启用开关 -->
                  <button type="button" class="toggle shrink-0" :class="{ 'toggle-on': m.enabled }" @click="m.enabled = !m.enabled">
                    <span class="toggle-knob"></span>
                  </button>
                  <!-- 模型名 -->
                  <input v-model="m.name" class="input !py-1.5 !rounded-lg font-mono text-xs flex-1" placeholder="模型显示名" />
                  <!-- 协议覆盖 -->
                  <select v-model="m.protocol" class="input !py-1.5 !rounded-lg text-xs w-32 shrink-0">
                    <option value="">继承</option>
                    <option v-for="p in protocols" :key="p.value" :value="p.value">{{ p.name }}</option>
                  </select>
                  <!-- 上游名 -->
                  <input v-model="m.upstream" class="input !py-1.5 !rounded-lg font-mono text-xs w-36 shrink-0" placeholder="上游名(可选)" />
                  <button class="text-ink-300 hover:text-red-500 shrink-0 px-1" @click="models.splice(i, 1)">✕</button>
                </div>
              </div>
            </div>
            <div v-else class="text-center py-6 text-ink-400 text-sm">
              暂无模型，拉取或手动添加
            </div>
            <p class="hint mt-2">已启用 {{ enabledCount }} 个 · 协议「继承」表示走规则或供应商默认协议</p>
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
              <div class="grid grid-cols-2 gap-4">
                <div>
                  <label class="label">优先级</label>
                  <input v-model.number="form.priority" type="number" class="input" placeholder="0" />
                  <p class="hint">数字越大越优先</p>
                </div>
                <div>
                  <label class="label">权重</label>
                  <input v-model.number="form.weight" type="number" class="input" placeholder="1" />
                  <p class="hint">同优先级下的负载比例</p>
                </div>
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

const models = ref([])   // [{name,enabled,protocol,upstream}]
const rules = ref([])    // [{pattern,protocol}]
const newModelName = ref('')

const blank = () => ({
  id: 0, name: '', type: 1, base_url: '', key: '', group: 'default',
  header_override: '', priority: 0, weight: 1, status: 1,
})
const form = ref(blank())

const enabledCount = computed(() => models.value.filter(m => m.enabled && m.name.trim()).length)
const canSave = computed(() => form.value.name && form.value.base_url && form.value.key && enabledCount.value > 0)

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
      if (Array.isArray(arr)) return arr.map(m => ({ name: m.name || '', enabled: m.enabled !== false, protocol: m.protocol || '', upstream: m.upstream || '' }))
    } catch {}
  }
  return (ch.models || '').split(',').map(s => s.trim()).filter(Boolean)
    .map(n => ({ name: n, enabled: true, protocol: '', upstream: '' }))
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
  const t = channelTypes.value.find(x => x.value === form.value.type)
  if (t) form.value.base_url = t.default_base_url
  showModal.value = true
}
function openEdit(ch) {
  form.value = { ...blank(), ...ch }
  models.value = parseModels(ch)
  rules.value = parseRules(ch)
  err.value = ''
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
  models.value.unshift({ name: n, enabled: true, protocol: '', upstream: '' })
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
        models.value.push({ name, enabled: true, protocol: '', upstream: '' })
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
  const cleanModels = models.value.filter(m => m.name.trim())
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

onMounted(async () => {
  await loadMeta()
  await load()
})
</script>

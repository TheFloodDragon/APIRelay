<template>
  <div>
    <div class="flex items-center justify-between mb-4">
      <h2 class="text-lg font-semibold">渠道管理</h2>
      <button class="btn-primary" @click="openCreate">+ 新建渠道</button>
    </div>

    <table class="w-full text-sm bg-white rounded-lg shadow overflow-hidden">
      <thead class="bg-gray-100 text-gray-600 text-left">
        <tr>
          <th class="p-3">ID</th>
          <th class="p-3">名称</th>
          <th class="p-3">协议</th>
          <th class="p-3">Base URL</th>
          <th class="p-3">分组</th>
          <th class="p-3">模型</th>
          <th class="p-3">状态</th>
          <th class="p-3">操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="ch in channels" :key="ch.id" class="border-t">
          <td class="p-3">{{ ch.id }}</td>
          <td class="p-3">{{ ch.name }}</td>
          <td class="p-3">{{ typeName(ch.type) }}</td>
          <td class="p-3 text-gray-500 max-w-[180px] truncate">{{ ch.base_url }}</td>
          <td class="p-3">{{ ch.group }}</td>
          <td class="p-3 text-gray-500 max-w-[220px] truncate">{{ ch.models }}</td>
          <td class="p-3">
            <span :class="ch.status === 1 ? 'text-green-600' : 'text-red-500'">
              {{ ch.status === 1 ? '启用' : '禁用' }}
            </span>
          </td>
          <td class="p-3 space-x-2">
            <button class="text-blue-600" @click="openEdit(ch)">编辑</button>
            <button class="text-red-500" @click="remove(ch)">删除</button>
          </td>
        </tr>
        <tr v-if="!channels.length"><td colspan="8" class="p-6 text-center text-gray-400">暂无渠道</td></tr>
      </tbody>
    </table>

    <!-- 编辑/创建弹窗 -->
    <div v-if="showModal" class="fixed inset-0 bg-black/40 flex items-center justify-center z-50" @click.self="showModal=false">
      <div class="bg-white rounded-lg shadow-lg w-[560px] max-h-[90vh] overflow-auto p-6">
        <h3 class="text-base font-semibold mb-4">{{ form.id ? '编辑渠道' : '新建渠道' }}</h3>
        <div class="space-y-3">
          <div>
            <label class="lbl">名称</label>
            <input v-model="form.name" class="inp" placeholder="渠道名称" />
          </div>
          <div>
            <label class="lbl">协议类型</label>
            <select v-model.number="form.type" class="inp" @change="onTypeChange">
              <option v-for="t in channelTypes" :key="t.value" :value="t.value">{{ t.name }}</option>
            </select>
          </div>
          <div>
            <label class="lbl">Base URL</label>
            <input v-model="form.base_url" class="inp" placeholder="https://api.openai.com" />
          </div>
          <div>
            <label class="lbl">API Key</label>
            <input v-model="form.key" class="inp" type="password" placeholder="上游密钥" />
          </div>
          <div>
            <label class="lbl">分组</label>
            <input v-model="form.group" class="inp" placeholder="default" />
          </div>
          <div>
            <div class="flex items-center justify-between">
              <label class="lbl">支持模型（逗号分隔）</label>
              <button class="text-xs text-blue-600 disabled:text-gray-300" :disabled="probing" @click="fetchModels">
                {{ probing ? '拉取中...' : '↻ 按协议拉取模型' }}
              </button>
            </div>
            <textarea v-model="form.models" class="inp h-20" placeholder="gpt-4o, gpt-4o-mini"></textarea>
            <p class="text-xs text-gray-400 mt-1">提示：填写 <code>*</code> 表示该渠道支持任意模型（通配）。</p>
            <div v-if="probedModels.length" class="mt-2 flex flex-wrap gap-1">
              <span v-for="m in probedModels" :key="m"
                    class="px-2 py-0.5 text-xs rounded cursor-pointer"
                    :class="isSelected(m) ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-600'"
                    @click="toggleModel(m)">{{ m }}</span>
            </div>
          </div>
          <div class="grid grid-cols-2 gap-3">
            <div><label class="lbl">优先级</label><input v-model.number="form.priority" type="number" class="inp" /></div>
            <div><label class="lbl">权重</label><input v-model.number="form.weight" type="number" class="inp" /></div>
          </div>
          <div>
            <label class="lbl">模型重定向（JSON，可选）</label>
            <input v-model="form.model_mapping" class="inp" placeholder='{"gpt-4":"gpt-4o"}' />
          </div>
          <div>
            <label class="lbl">请求头覆盖（JSON，可选）</label>
            <input v-model="form.header_override" class="inp" placeholder='{"User-Agent":"..."}' />
          </div>
        </div>
        <div v-if="err" class="text-red-500 text-sm mt-3">{{ err }}</div>
        <div class="flex justify-end gap-2 mt-5">
          <button class="px-4 py-2 text-sm rounded border" @click="showModal=false">取消</button>
          <button class="btn-primary" :disabled="saving" @click="save">{{ saving ? '保存中...' : '保存' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'

const channels = ref([])
const channelTypes = ref([])
const showModal = ref(false)
const probing = ref(false)
const saving = ref(false)
const err = ref('')
const probedModels = ref([])

const blank = () => ({
  id: 0, name: '', type: 1, base_url: '', key: '', group: 'default',
  models: '', model_mapping: '', header_override: '', priority: 0, weight: 1, status: 1,
})
const form = ref(blank())

function typeName(t) {
  const f = channelTypes.value.find(x => x.value === t)
  return f ? f.name : t
}

async function load() {
  const { data } = await api.get('/channels')
  channels.value = data.data || []
}
async function loadTypes() {
  const { data } = await api.get('/channel-types')
  channelTypes.value = data.data || []
}

function openCreate() {
  form.value = blank()
  probedModels.value = []
  err.value = ''
  // 默认填充协议的 base_url
  const t = channelTypes.value.find(x => x.value === form.value.type)
  if (t) form.value.base_url = t.default_base_url
  showModal.value = true
}
function openEdit(ch) {
  form.value = { ...ch }
  probedModels.value = []
  err.value = ''
  showModal.value = true
}
function onTypeChange() {
  const t = channelTypes.value.find(x => x.value === form.value.type)
  if (t && !form.value.base_url) form.value.base_url = t.default_base_url
  probedModels.value = []
}

// 核心：按协议拉取上游模型列表
async function fetchModels() {
  err.value = ''
  probing.value = true
  try {
    const { data } = await api.post('/channels/probe-models', {
      type: form.value.type,
      base_url: form.value.base_url,
      key: form.value.key,
    })
    probedModels.value = data.data.models || []
    if (!probedModels.value.length) err.value = '上游未返回模型'
  } catch (e) {
    err.value = e.response?.data?.message || '拉取失败'
  } finally {
    probing.value = false
  }
}

function selectedSet() {
  return new Set(form.value.models.split(',').map(s => s.trim()).filter(Boolean))
}
function isSelected(m) {
  return selectedSet().has(m)
}
function toggleModel(m) {
  const set = selectedSet()
  if (set.has(m)) set.delete(m); else set.add(m)
  form.value.models = [...set].join(',')
}

async function save() {
  err.value = ''
  saving.value = true
  try {
    if (form.value.id) {
      await api.put(`/channels/${form.value.id}`, form.value)
    } else {
      await api.post('/channels', form.value)
    }
    showModal.value = false
    await load()
  } catch (e) {
    err.value = e.response?.data?.message || '保存失败'
  } finally {
    saving.value = false
  }
}

async function remove(ch) {
  if (!confirm(`确认删除渠道「${ch.name}」？`)) return
  await api.delete(`/channels/${ch.id}`)
  await load()
}

onMounted(async () => {
  await loadTypes()
  await load()
})
</script>

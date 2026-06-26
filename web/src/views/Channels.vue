<template>
  <div>
    <div class="flex items-center justify-between mb-6">
      <div>
        <h2 class="text-2xl font-bold text-gray-900">渠道管理</h2>
        <p class="text-sm text-gray-500 mt-1">配置上游 AI API 服务渠道</p>
      </div>
      <button class="btn-primary" @click="openCreate">
        <span>➕</span>
        <span>新建渠道</span>
      </button>
    </div>

    <div class="table-wrapper">
      <table class="table">
        <thead>
          <tr>
            <th>ID</th>
            <th>名称</th>
            <th>协议</th>
            <th>分组</th>
            <th>模型</th>
            <th>优先级</th>
            <th>权重</th>
            <th>状态</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="ch in channels" :key="ch.id">
            <td class="font-mono text-xs text-gray-500">#{{ ch.id }}</td>
            <td class="font-medium">{{ ch.name }}</td>
            <td><span class="badge-info">{{ typeName(ch.type) }}</span></td>
            <td><span class="badge-neutral">{{ ch.group }}</span></td>
            <td class="max-w-[200px]">
              <div class="text-xs text-gray-600 font-mono truncate" :title="ch.models">
                {{ ch.models || '全部' }}
              </div>
            </td>
            <td class="text-gray-600 text-center">{{ ch.priority }}</td>
            <td class="text-gray-600 text-center">{{ ch.weight }}</td>
            <td>
              <span v-if="ch.status === 1" class="badge-success">启用</span>
              <span v-else class="badge-error">禁用</span>
            </td>
            <td>
              <div class="flex gap-2">
                <button @click="openEdit(ch)" class="text-brand-600 hover:text-brand-700 font-medium text-sm">编辑</button>
                <button @click="remove(ch)" class="text-red-600 hover:text-red-700 font-medium text-sm">删除</button>
              </div>
            </td>
          </tr>
          <tr v-if="!channels.length">
            <td colspan="9" class="empty-state">
              <div class="text-4xl mb-2">🔗</div>
              <div>暂无渠道，点击右上角新建</div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 编辑/创建弹窗 -->
    <div v-if="showModal" class="fixed inset-0 bg-black/40 flex items-center justify-center z-50 p-4" @click.self="showModal=false">
      <div class="bg-white rounded-xl shadow-2xl w-full max-w-2xl max-h-[90vh] overflow-auto">
        <!-- Header -->
        <div class="sticky top-0 bg-white border-b border-gray-100 px-6 py-4 flex items-center justify-between">
          <h3 class="text-lg font-semibold text-gray-900">{{ form.id ? '编辑渠道' : '新建渠道' }}</h3>
          <button @click="showModal=false" class="text-gray-400 hover:text-gray-600 text-2xl leading-none">&times;</button>
        </div>

        <!-- Body -->
        <div class="px-6 py-4 space-y-5">
          <!-- 基本信息 -->
          <div class="space-y-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1.5">渠道名称 <span class="text-red-500">*</span></label>
              <input v-model="form.name" class="input" placeholder="例：OpenAI 主账号" />
            </div>
            
            <div class="grid grid-cols-2 gap-4">
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1.5">协议类型 <span class="text-red-500">*</span></label>
                <select v-model.number="form.type" class="input" @change="onTypeChange">
                  <option v-for="t in channelTypes" :key="t.value" :value="t.value">{{ t.name }}</option>
                </select>
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1.5">分组</label>
                <input v-model="form.group" class="input" placeholder="default" />
              </div>
            </div>

            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1.5">Base URL <span class="text-red-500">*</span></label>
              <input v-model="form.base_url" class="input" placeholder="https://api.openai.com" />
            </div>

            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1.5">API Key <span class="text-red-500">*</span></label>
              <input v-model="form.key" class="input" type="password" placeholder="上游密钥" />
            </div>
          </div>

          <!-- 模型选择区域 -->
          <div class="border border-gray-200 rounded-lg p-4 bg-gray-50">
            <div class="flex items-center justify-between mb-3">
              <label class="text-sm font-medium text-gray-700">支持模型 <span class="text-red-500">*</span></label>
              <button 
                class="px-3 py-1.5 text-sm rounded-lg font-medium transition-colors"
                :class="probing ? 'bg-gray-300 text-gray-500 cursor-not-allowed' : 'bg-brand-600 text-white hover:bg-brand-700'"
                :disabled="probing || !form.base_url || !form.key"
                @click="fetchModels"
              >
                <span v-if="probing">⏳ 拉取中...</span>
                <span v-else>🔄 从上游拉取模型</span>
              </button>
            </div>

            <!-- 模型列表 -->
            <div v-if="probedModels.length" class="space-y-2">
              <div class="text-xs text-gray-500 mb-2">点击模型名称选择/取消（已选 {{ selectedCount }} 个）</div>
              <div class="flex flex-wrap gap-2 max-h-48 overflow-y-auto p-2 bg-white rounded border border-gray-200">
                <button 
                  v-for="m in probedModels" :key="m"
                  class="px-3 py-1.5 text-sm rounded-lg font-mono transition-all"
                  :class="isSelected(m) 
                    ? 'bg-brand-600 text-white hover:bg-brand-700 shadow-sm' 
                    : 'bg-gray-100 text-gray-700 hover:bg-gray-200'"
                  @click="toggleModel(m)"
                >
                  {{ m }}
                </button>
              </div>
            </div>
            <div v-else class="text-center py-8 text-gray-400">
              <div class="text-3xl mb-2">📋</div>
              <div class="text-sm">填写上方信息后，点击「从上游拉取模型」</div>
            </div>

            <div class="mt-3 text-xs text-gray-500 bg-blue-50 border border-blue-100 rounded p-2">
              💡 提示：也可填写 <code class="bg-white px-1 rounded">*</code> 表示该渠道支持任意模型（通配）
            </div>
          </div>

          <!-- 高级配置 -->
          <details class="border border-gray-200 rounded-lg overflow-hidden">
            <summary class="px-4 py-3 bg-gray-50 cursor-pointer hover:bg-gray-100 font-medium text-sm text-gray-700">
              ⚙️ 高级配置（可选）
            </summary>
            <div class="p-4 space-y-4 bg-white">
              <div class="grid grid-cols-2 gap-4">
                <div>
                  <label class="block text-sm font-medium text-gray-700 mb-1.5">优先级</label>
                  <input v-model.number="form.priority" type="number" class="input" placeholder="0" />
                  <p class="text-xs text-gray-500 mt-1">数字越大优先级越高</p>
                </div>
                <div>
                  <label class="block text-sm font-medium text-gray-700 mb-1.5">权重</label>
                  <input v-model.number="form.weight" type="number" class="input" placeholder="1" />
                  <p class="text-xs text-gray-500 mt-1">同优先级下的负载比例</p>
                </div>
              </div>
              
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1.5">模型重定向（JSON）</label>
                <input v-model="form.model_mapping" class="input font-mono text-xs" placeholder='{"gpt-4":"gpt-4o"}' />
                <p class="text-xs text-gray-500 mt-1">将请求的模型名映射为上游模型名</p>
              </div>
              
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1.5">请求头覆盖（JSON）</label>
                <input v-model="form.header_override" class="input font-mono text-xs" placeholder='{"User-Agent":"..."}' />
                <p class="text-xs text-gray-500 mt-1">额外添加或覆盖的 HTTP 请求头</p>
              </div>
            </div>
          </details>
        </div>

        <!-- Footer -->
        <div class="sticky bottom-0 bg-gray-50 border-t border-gray-100 px-6 py-4">
          <div v-if="err" class="mb-3 p-3 bg-red-50 border border-red-200 rounded text-sm text-red-600">
            ⚠️ {{ err }}
          </div>
          <div class="flex justify-end gap-3">
            <button @click="showModal=false" class="px-4 py-2 text-sm rounded-lg border border-gray-300 hover:bg-gray-50 font-medium">取消</button>
            <button 
              class="btn-primary"
              :disabled="saving || !canSave"
              @click="save"
            >
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

const selectedCount = computed(() => {
  return form.value.models.split(',').map(s => s.trim()).filter(Boolean).length
})

const canSave = computed(() => {
  return form.value.name && form.value.base_url && form.value.key && form.value.models
})

function typeName(t) {
  const f = channelTypes.value.find(x => x.value === t)
  return f ? f.name : t
}

async function load() {
  try {
    channels.value = (await api.get('/channels')) || []
  } catch (e) {
    toast.error('加载渠道失败: ' + e.message)
  }
}

async function loadTypes() {
  try {
    channelTypes.value = (await api.get('/channel-types')) || []
  } catch (e) {
    toast.error('加载协议类型失败: ' + e.message)
  }
}

function openCreate() {
  form.value = blank()
  probedModels.value = []
  err.value = ''
  const t = channelTypes.value.find(x => x.value === form.value.type)
  if (t) form.value.base_url = t.default_base_url
  showModal.value = true
}

function openEdit(ch) {
  form.value = { ...ch }
  probedModels.value = []
  err.value = ''
  // 编辑时预填已选模型
  if (ch.models) {
    probedModels.value = ch.models.split(',').map(s => s.trim()).filter(Boolean)
  }
  showModal.value = true
}

function onTypeChange() {
  const t = channelTypes.value.find(x => x.value === form.value.type)
  if (t && !form.value.base_url) form.value.base_url = t.default_base_url
  probedModels.value = []
  form.value.models = ''
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
      type: form.value.type,
      base_url: form.value.base_url,
      key: form.value.key,
    })
    probedModels.value = data.models || []
    if (!probedModels.value.length) {
      err.value = '上游未返回模型列表'
    } else {
      toast.success(`成功拉取 ${probedModels.value.length} 个模型`)
    }
  } catch (e) {
    err.value = '拉取失败: ' + (e.message || '网络错误')
    toast.error(err.value)
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
  if (set.has(m)) {
    set.delete(m)
  } else {
    set.add(m)
  }
  form.value.models = [...set].sort().join(',')
}

async function save() {
  if (!canSave.value) {
    err.value = '请填写必填项并选择至少一个模型'
    return
  }

  err.value = ''
  saving.value = true
  try {
    if (form.value.id) {
      await api.put(`/channels/${form.value.id}`, form.value)
      toast.success('渠道已更新')
    } else {
      await api.post('/channels', form.value)
      toast.success('渠道已创建')
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
    toast.success('渠道已删除')
    await load()
  } catch (e) {
    toast.error('删除失败: ' + e.message)
  }
}

onMounted(async () => {
  await loadTypes()
  await load()
})
</script>

<style scoped>
.input {
  @apply w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-brand-500 focus:border-transparent transition-all;
}

details summary::-webkit-details-marker {
  display: none;
}

code {
  @apply font-mono text-xs;
}
</style>

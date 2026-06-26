<script setup>
import { ref, onMounted } from 'vue'
import { useToast } from '../composables/useToast'
import api from '../api'

const toast = useToast()
const tokens = ref([])
const showModal = ref(false)
const err = ref('')
const form = ref({ name: '', group: 'default', allowed_models: '' })

function mask(k) {
  if (!k) return ''
  return k.slice(0, 7) + '...' + k.slice(-4)
}

async function copy(k) {
  try {
    await navigator.clipboard.writeText(k)
    toast.success('已复制到剪贴板')
  } catch {
    toast.error('复制失败')
  }
}

async function load() {
  tokens.value = (await api.get('/tokens')) || []
}

function openCreate() {
  form.value = { name: '', group: 'default', allowed_models: '' }
  err.value = ''
  showModal.value = true
}

async function save() {
  err.value = ''
  try {
    const res = await api.post('/tokens', form.value)
    showModal.value = false
    if (res && res.key) {
      // 显示完整 key（仅此一次）并自动复制
      try {
        await navigator.clipboard.writeText(res.key)
        toast.success(`✅ 令牌已创建并复制到剪贴板\n\n🔑 ${res.key}\n\n⚠️ 请妥善保存，此密钥仅显示一次`, 10000)
      } catch {
        // 复制失败，显示 key 让用户手动复制
        toast.success(`✅ 令牌已创建\n\n🔑 ${res.key}\n\n⚠️ 请立即复制保存，此密钥仅显示一次`, 15000)
      }
    }
    await load()
  } catch (e) {
    err.value = e.message || '保存失败'
  }
}

async function remove(t) {
  if (!confirm(`确认删除令牌「${t.name}」？`)) return
  try {
    await api.delete(`/tokens/${t.id}`)
    toast.success('令牌已删除')
    await load()
  } catch (e) {
    toast.error(e.message || '删除失败')
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="flex items-center justify-between mb-6">
      <div>
        <h2 class="text-2xl font-bold text-gray-900">令牌管理</h2>
        <p class="text-sm text-gray-500 mt-1">管理对外暴露的 API Key</p>
      </div>
      <button class="btn-primary" @click="openCreate">
        <span>➕</span>
        <span>新建令牌</span>
      </button>
    </div>

    <div class="table-wrapper">
      <table class="table">
        <thead>
          <tr>
            <th>ID</th>
            <th>名称</th>
            <th>Key</th>
            <th>分组</th>
            <th>允许模型</th>
            <th>状态</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="t in tokens" :key="t.id">
            <td class="font-mono text-xs">#{{ t.id }}</td>
            <td class="font-medium">{{ t.name }}</td>
            <td class="font-mono text-xs">
              <span class="text-gray-500" :title="`完整 key 仅创建时可见\n前缀: ${t.key_prefix}`">
                {{ mask(t.key_prefix) }}
              </span>
              <span class="ml-2 text-gray-400 text-[10px]">（仅创建时可见）</span>
            </td>
            <td><span class="badge-neutral">{{ t.group }}</span></td>
            <td class="text-gray-600 text-xs max-w-[180px] truncate">{{ t.models || '全部' }}</td>
            <td>
              <span v-if="t.status === 1" class="badge-success">启用</span>
              <span v-else class="badge-error">禁用</span>
            </td>
            <td>
              <button @click="remove(t)" class="text-red-600 hover:text-red-700 font-medium text-sm">删除</button>
            </td>
          </tr>
          <tr v-if="!tokens.length">
            <td colspan="7" class="empty-state">
              <div class="text-4xl mb-2">🔑</div>
              <div>暂无令牌，点击右上角新建</div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 创建模态 -->
    <div v-if="showModal" class="modal-backdrop" @click.self="showModal=false">
      <div class="modal">
        <h3 class="modal-header">新建令牌</h3>
        <div class="space-y-4">
          <div>
            <label class="label">令牌名称</label>
            <input v-model="form.name" class="input" placeholder="例：测试环境" />
          </div>
          <div>
            <label class="label">分组</label>
            <input v-model="form.group" class="input" placeholder="default" />
          </div>
          <div>
            <label class="label">允许模型（逗号分隔，留空=全部）</label>
            <input v-model="form.allowed_models" class="input" placeholder="gpt-4o,claude-3-5-sonnet" />
          </div>
          <div v-if="err" class="text-red-500 text-sm">{{ err }}</div>
        </div>
        <div class="flex justify-end gap-3 mt-6 pt-4 border-t border-gray-100">
          <button @click="showModal=false" class="btn-secondary">取消</button>
          <button @click="save" class="btn-primary">创建</button>
        </div>
      </div>
    </div>
  </div>
</template>

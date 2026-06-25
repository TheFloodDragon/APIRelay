<template>
  <div>
    <div class="flex items-center justify-between mb-4">
      <h2 class="text-lg font-semibold">令牌管理</h2>
      <button class="btn-primary" @click="openCreate">+ 新建令牌</button>
    </div>

    <table class="w-full text-sm bg-white rounded-lg shadow overflow-hidden">
      <thead class="bg-gray-100 text-gray-600 text-left">
        <tr>
          <th class="p-3">ID</th>
          <th class="p-3">名称</th>
          <th class="p-3">Key</th>
          <th class="p-3">分组</th>
          <th class="p-3">允许模型</th>
          <th class="p-3">状态</th>
          <th class="p-3">操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="t in tokens" :key="t.id" class="border-t">
          <td class="p-3">{{ t.id }}</td>
          <td class="p-3">{{ t.name }}</td>
          <td class="p-3 font-mono text-xs">
            <span>{{ mask(t.key) }}</span>
            <button class="ml-2 text-blue-600" @click="copy(t.key)">复制</button>
          </td>
          <td class="p-3">{{ t.group }}</td>
          <td class="p-3 text-gray-500 max-w-[200px] truncate">{{ t.allowed_models || '全部' }}</td>
          <td class="p-3">
            <span :class="t.status === 1 ? 'text-green-600' : 'text-red-500'">
              {{ t.status === 1 ? '启用' : '禁用' }}
            </span>
          </td>
          <td class="p-3">
            <button class="text-red-500" @click="remove(t)">删除</button>
          </td>
        </tr>
        <tr v-if="!tokens.length"><td colspan="7" class="p-6 text-center text-gray-400">暂无令牌</td></tr>
      </tbody>
    </table>

    <div v-if="showModal" class="fixed inset-0 bg-black/40 flex items-center justify-center z-50" @click.self="showModal=false">
      <div class="bg-white rounded-lg shadow-lg w-[440px] p-6">
        <h3 class="text-base font-semibold mb-4">新建令牌</h3>
        <div class="space-y-3">
          <div><label class="lbl">名称</label><input v-model="form.name" class="inp" /></div>
          <div><label class="lbl">分组</label><input v-model="form.group" class="inp" placeholder="default" /></div>
          <div><label class="lbl">允许模型（逗号分隔，留空=全部）</label><input v-model="form.allowed_models" class="inp" /></div>
        </div>
        <div v-if="err" class="text-red-500 text-sm mt-3">{{ err }}</div>
        <div class="flex justify-end gap-2 mt-5">
          <button class="px-4 py-2 text-sm rounded border" @click="showModal=false">取消</button>
          <button class="btn-primary" @click="save">保存</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'

const tokens = ref([])
const showModal = ref(false)
const err = ref('')
const form = ref({ name: '', group: 'default', allowed_models: '' })

function mask(k) {
  if (!k) return ''
  return k.slice(0, 7) + '...' + k.slice(-4)
}
function copy(k) {
  navigator.clipboard?.writeText(k)
}

async function load() {
  const { data } = await api.get('/tokens')
  tokens.value = data.data || []
}
function openCreate() {
  form.value = { name: '', group: 'default', allowed_models: '' }
  err.value = ''
  showModal.value = true
}
async function save() {
  err.value = ''
  try {
    await api.post('/tokens', form.value)
    showModal.value = false
    await load()
  } catch (e) {
    err.value = e.response?.data?.message || '保存失败'
  }
}
async function remove(t) {
  if (!confirm(`确认删除令牌「${t.name}」？`)) return
  await api.delete(`/tokens/${t.id}`)
  await load()
}
onMounted(load)
</script>

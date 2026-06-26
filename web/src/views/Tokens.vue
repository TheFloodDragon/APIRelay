<script setup>
import { ref, onMounted } from 'vue'
import { useToast } from '../composables/useToast'
import api from '../api'

const toast = useToast()
const tokens = ref([])
const showModal = ref(false)
const err = ref('')
const form = ref({ name: '', group: 'default', models: '', unlimited: true, quota_usd: 0 })

function mask(k) {
  if (!k) return ''
  return k.slice(0, 7) + '...' + k.slice(-4)
}

// 微美元 -> 美元显示
function usd(micro) {
  return '$' + ((micro || 0) / 1_000_000).toFixed(4)
}

function quotaDisplay(t) {
  if (t.unlimited) return `${usd(t.used_quota)} / 不限`
  return `${usd(t.used_quota)} / ${usd(t.quota)}`
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
  form.value = { name: '', group: 'default', models: '', unlimited: true, quota_usd: 0 }
  err.value = ''
  showModal.value = true
}

async function save() {
  err.value = ''
  if (!form.value.name.trim()) {
    err.value = '请填写令牌名称'
    return
  }
  try {
    const payload = {
      name: form.value.name.trim(),
      group: form.value.group || 'default',
      models: form.value.models || '',
      unlimited: form.value.unlimited,
      quota_usd: form.value.unlimited ? 0 : (Number(form.value.quota_usd) || 0),
    }
    const res = await api.post('/tokens', payload)
    showModal.value = false
    if (res && res.key) {
      // 显示完整 key（仅此一次）并自动复制
      try {
        await navigator.clipboard.writeText(res.key)
        toast.success(`✅ 令牌已创建并复制到剪贴板\n\n🔑 ${res.key}\n\n⚠️ 请妥善保存，此密钥仅显示一次`, 10000)
      } catch {
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
        <h2 class="page-title">令牌管理</h2>
        <p class="page-subtitle">管理对外暴露的 API Key</p>
      </div>
      <button class="btn-primary" @click="openCreate">
        <svg viewBox="0 0 24 24" class="w-4 h-4" fill="currentColor"><path d="M11 11V5h2v6h6v2h-6v6h-2v-6H5v-2z"/></svg>
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
            <th>额度(已用/总)</th>
            <th>状态</th>
            <th class="text-right">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="t in tokens" :key="t.id">
            <td class="font-mono text-xs text-ink-400">#{{ t.id }}</td>
            <td class="font-medium">{{ t.name }}</td>
            <td class="font-mono text-xs">
              <span class="text-ink-500" :title="`完整 key 仅创建时可见\n前缀: ${t.key_prefix}`">
                {{ mask(t.key_prefix) }}
              </span>
              <span class="ml-2 text-ink-400 text-[10px]">（仅创建时可见）</span>
            </td>
            <td><span class="badge-neutral">{{ t.group }}</span></td>
            <td class="text-ink-500 text-xs max-w-[180px] truncate">{{ t.models || '全部' }}</td>
            <td class="text-xs font-mono text-ink-600 dark:text-ink-300">{{ quotaDisplay(t) }}</td>
            <td>
              <span v-if="t.status === 1" class="badge-success"><span class="w-1.5 h-1.5 rounded-full bg-green-500"></span>启用</span>
              <span v-else class="badge-error">禁用</span>
            </td>
            <td>
              <div class="flex justify-end">
                <button @click="remove(t)" class="btn-danger btn-sm">删除</button>
              </div>
            </td>
          </tr>
          <tr v-if="!tokens.length">
            <td colspan="8" class="empty-state">
              <div class="text-5xl mb-3 opacity-60">🔑</div>
              <div>暂无令牌，点击右上角新建</div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 创建模态 -->
    <div v-if="showModal" class="modal-backdrop" @click.self="showModal=false">
      <div class="modal max-w-lg">
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
            <input v-model="form.models" class="input" placeholder="gpt-4o,claude-3-5-sonnet" />
          </div>
          <div>
            <label class="label">额度</label>
            <div class="flex items-center gap-3">
              <button type="button" class="toggle shrink-0" :class="{ 'toggle-on': form.unlimited }" @click="form.unlimited = !form.unlimited">
                <span class="toggle-knob"></span>
              </button>
              <span class="text-sm text-ink-600 dark:text-ink-300">{{ form.unlimited ? '不限额度' : '限制额度' }}</span>
              <div v-if="!form.unlimited" class="flex items-center gap-1 ml-auto">
                <span class="text-ink-400 text-sm">$</span>
                <input v-model.number="form.quota_usd" type="number" step="0.01" min="0" class="input !py-1.5 w-32 text-right" placeholder="0.00" />
              </div>
            </div>
            <p class="hint">额度按模型价格（美元）扣减；不限额度仅统计用量不拦截。</p>
          </div>
          <div v-if="err" class="text-red-500 text-sm">{{ err }}</div>
        </div>
        <div class="flex justify-end gap-3 mt-6 pt-4 border-t border-ink-100 dark:border-ink-800">
          <button @click="showModal=false" class="btn-secondary">取消</button>
          <button @click="save" class="btn-primary">创建</button>
        </div>
      </div>
    </div>
  </div>
</template>

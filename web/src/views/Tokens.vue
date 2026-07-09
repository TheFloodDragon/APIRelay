<script setup>
import { ref, computed, onMounted } from 'vue'
import { useToast } from '../composables/useToast'
import api from '../api'
import SignalDot from '../components/SignalDot.vue'
import MeterBar from '../components/MeterBar.vue'

const toast = useToast()
const tokens = ref([])
const showModal = ref(false)
const err = ref('')
const form = ref({ name: '', group: 'default', models: '', unlimited: true, quota_usd: 0 })

function mask(k) {
  return k || '—'
}

const usd = (micro) => '$' + ((micro || 0) / 1_000_000).toFixed(4)

function quotaDisplay(t) {
  if (t.unlimited) return `${usd(t.used_quota)} / ∞`
  return `${usd(t.used_quota)} / ${usd(t.quota)}`
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
      try {
        await navigator.clipboard.writeText(res.key)
        toast.success(`✓ 令牌已创建并复制到剪贴板\n\n${res.key}\n\n⚠ 请妥善保存，此密钥仅显示一次`, 10000)
      } catch {
        toast.success(`✓ 令牌已创建\n\n${res.key}\n\n⚠ 请立即复制保存，此密钥仅显示一次`, 15000)
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

const tokenStatus = (t) => t.status === 1 ? 'online' : 'down'

onMounted(load)
</script>

<template>
  <div>
    <div class="flex items-center justify-between mb-5">
      <div>
        <h2 class="page-title">令牌管理</h2>
        <p class="page-subtitle">对外暴露的 API Key 与额度追踪</p>
      </div>
      <button class="btn-primary" @click="openCreate">
        <svg viewBox="0 0 24 24" class="w-4 h-4" fill="currentColor"><path d="M11 11V5h2v6h6v2h-6v6h-2v-6H5v-2z"/></svg>
        <span>新建令牌</span>
      </button>
    </div>

    <div class="panel overflow-hidden">
      <table class="dtable">
        <thead>
          <tr>
            <th class="w-12">ID</th>
            <th>名称</th>
            <th>Key 前缀</th>
            <th>分组</th>
            <th>允许模型</th>
            <th>额度使用</th>
            <th>状态</th>
            <th class="text-right w-20">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="t in tokens" :key="t.id">
            <td class="font-mono text-2xs text-t3">#{{ t.id }}</td>
            <td class="font-medium text-t1">{{ t.name }}</td>
            <td>
              <div class="flex items-center gap-2">
                <span class="key-chip"><code :title="`完整 key 仅创建时可见 · 前缀: ${t.key_prefix}`">{{ mask(t.key_prefix) }}</code></span>
                <span class="tick">hash only</span>
              </div>
            </td>
            <td><span class="badge badge-neutral font-mono">{{ t.group }}</span></td>
            <td class="text-xs text-t2 font-mono max-w-[200px] truncate">{{ t.models || '全部' }}</td>
            <td>
              <div class="space-y-1">
                <div class="font-mono text-xs text-t1 tabular-nums">{{ quotaDisplay(t) }}</div>
                <MeterBar :value="t.used_quota" :max="t.quota" :unlimited="t.unlimited" tone="auto" />
              </div>
            </td>
            <td>
              <div class="flex items-center gap-1.5">
                <SignalDot :status="tokenStatus(t)" />
                <span class="text-xs text-t2">{{ t.status === 1 ? '启用' : '禁用' }}</span>
              </div>
            </td>
            <td>
              <div class="flex justify-end">
                <button @click="remove(t)" class="btn-danger btn-sm">删除</button>
              </div>
            </td>
          </tr>
          <tr v-if="!tokens.length">
            <td colspan="8" class="empty-state">
              <span class="font-mono text-3xl text-t3">∅</span>
              <span>暂无令牌，点击右上角新建</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 创建模态 -->
    <div v-if="showModal" class="modal-backdrop" @click.self="showModal=false">
      <div class="modal max-w-lg">
        <div class="modal-header">
          <h3 class="modal-title">新建令牌</h3>
          <button @click="showModal=false" class="text-t3 hover:text-t1 text-xl leading-none">×</button>
        </div>
        <div class="space-y-4">
          <div>
            <label class="label">令牌名称</label>
            <input v-model="form.name" class="input" placeholder="例：测试环境" />
          </div>
          <div>
            <label class="label">分组</label>
            <input v-model="form.group" class="input font-mono" placeholder="default" />
          </div>
          <div>
            <label class="label">允许模型（逗号分隔，留空=全部）</label>
            <input v-model="form.models" class="input font-mono text-xs" placeholder="gpt-4o,claude-3-5-sonnet" />
          </div>
          <div>
            <label class="label">额度</label>
            <div class="inset p-3 space-y-2">
              <div class="flex items-center gap-3">
                <button type="button" class="toggle shrink-0" :class="{ 'toggle-on': form.unlimited }" @click="form.unlimited = !form.unlimited">
                  <span class="toggle-knob"></span>
                </button>
                <span class="text-sm text-t1">{{ form.unlimited ? '不限额度' : '限制额度' }}</span>
                <div v-if="!form.unlimited" class="flex items-center gap-1 ml-auto">
                  <span class="text-t2 text-sm font-mono">$</span>
                  <input v-model.number="form.quota_usd" type="number" step="0.01" min="0" class="input !py-1.5 w-28 text-right font-mono" placeholder="0.00" />
                </div>
              </div>
              <p class="hint">额度按模型价格（美元）扣减；不限额度仅统计用量不拦截。</p>
            </div>
          </div>
          <div v-if="err" class="text-sm border rounded-lg px-3 py-2 text-[rgb(var(--rust))] border-[rgb(var(--rust)/0.28)] bg-[rgb(var(--rust)/0.08)]">{{ err }}</div>
        </div>
        <div class="flex justify-end gap-2 mt-5 pt-4 border-t border-line">
          <button @click="showModal=false" class="btn-secondary">取消</button>
          <button @click="save" class="btn-primary">创建</button>
        </div>
      </div>
    </div>
  </div>
</template>

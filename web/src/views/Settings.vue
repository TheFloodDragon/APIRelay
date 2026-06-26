<template>
  <div>
    <div class="mb-6">
      <h2 class="page-title">设置</h2>
      <p class="page-subtitle">全局协议规则与系统配置</p>
    </div>

    <div class="card max-w-3xl">
      <div class="flex items-start gap-3 mb-5">
        <div class="w-10 h-10 rounded-xl bg-brand-50 dark:bg-brand-500/15 flex items-center justify-center text-lg shrink-0">🧭</div>
        <div>
          <h3 class="text-base font-semibold text-ink-900 dark:text-ink-100">全局协议规则</h3>
          <p class="hint">按模型显示名正则匹配上游协议。优先级：模型显式协议 &gt; 供应商规则 &gt; <span class="font-medium">全局规则</span> &gt; 供应商默认协议。</p>
        </div>
      </div>

      <div class="space-y-2">
        <div v-for="(r, i) in rules" :key="i" class="flex gap-2 items-center">
          <span class="text-xs text-ink-400 font-mono w-6 text-center shrink-0">{{ i + 1 }}</span>
          <input v-model="r.pattern" class="input font-mono text-xs flex-1" placeholder="正则，如 ^claude- 或 gpt-.*" />
          <select v-model="r.protocol" class="input text-xs w-40 shrink-0">
            <option v-for="p in protocols" :key="p.value" :value="p.value">{{ p.name }}</option>
          </select>
          <button class="text-ink-300 hover:text-red-500 px-2 shrink-0" @click="rules.splice(i, 1)">✕</button>
        </div>
        <div v-if="!rules.length" class="text-center py-8 text-ink-400 text-sm surface">
          暂无全局规则
        </div>
      </div>

      <div class="flex items-center justify-between mt-4 pt-4 border-t border-ink-100 dark:border-ink-800">
        <button class="btn-ghost btn-sm" @click="rules.push({ pattern: '', protocol: 'anthropic' })">
          + 添加规则
        </button>
        <button class="btn-primary" :disabled="saving" @click="save">
          {{ saving ? '保存中...' : '保存规则' }}
        </button>
      </div>
    </div>

    <!-- 规则测试 -->
    <div class="card max-w-3xl mt-6">
      <h3 class="text-base font-semibold text-ink-900 dark:text-ink-100 mb-3">规则测试</h3>
      <p class="hint mb-3">输入一个模型名，预览全局规则的首个命中结果（不含供应商级配置）</p>
      <div class="flex gap-2 items-center">
        <input v-model="testModel" class="input font-mono text-sm flex-1" placeholder="如 claude-3-5-sonnet" />
        <div class="shrink-0">
          <span v-if="testModel" class="badge-brand">{{ testResult || '未命中（用供应商默认）' }}</span>
        </div>
      </div>
    </div>

    <!-- 全局模型价格 -->
    <div class="card max-w-3xl mt-6">
      <div class="flex items-start gap-3 mb-5">
        <div class="w-10 h-10 rounded-xl bg-green-50 dark:bg-green-500/15 flex items-center justify-center text-lg shrink-0">💲</div>
        <div>
          <h3 class="text-base font-semibold text-ink-900 dark:text-ink-100">全局模型价格</h3>
          <p class="hint">单位：美元 / 100 万 tokens。模型名填 <code class="font-mono">default</code> 作为兜底价格。优先级：供应商模型价格 &gt; <span class="font-medium">全局价格</span> &gt; 不计费。</p>
        </div>
      </div>

      <div class="space-y-2">
        <div class="hidden sm:flex gap-2 items-center text-xs text-ink-400 px-1">
          <span class="w-6 shrink-0"></span>
          <span class="flex-1">模型名</span>
          <span class="w-28 shrink-0 text-right pr-2">输入 $/1M</span>
          <span class="w-28 shrink-0 text-right pr-2">输出 $/1M</span>
          <span class="w-6 shrink-0"></span>
        </div>
        <div v-for="(p, i) in prices" :key="i" class="flex gap-2 items-center">
          <span class="text-xs text-ink-400 font-mono w-6 text-center shrink-0">{{ i + 1 }}</span>
          <input v-model="p.model" class="input font-mono text-xs flex-1" placeholder="模型名 或 default" />
          <input v-model.number="p.input" type="number" step="0.01" min="0" class="input text-xs w-28 shrink-0 text-right" placeholder="0" />
          <input v-model.number="p.output" type="number" step="0.01" min="0" class="input text-xs w-28 shrink-0 text-right" placeholder="0" />
          <button class="text-ink-300 hover:text-red-500 px-2 shrink-0" @click="prices.splice(i, 1)">✕</button>
        </div>
        <div v-if="!prices.length" class="text-center py-8 text-ink-400 text-sm surface">
          暂无价格条目（未配置时不计费）
        </div>
      </div>

      <div class="flex items-center justify-between mt-4 pt-4 border-t border-ink-100 dark:border-ink-800">
        <button class="btn-ghost btn-sm" @click="prices.push({ model: '', input: 0, output: 0 })">
          + 添加价格
        </button>
        <button class="btn-primary" :disabled="savingPrices" @click="savePrices">
          {{ savingPrices ? '保存中...' : '保存价格' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useToast } from '../composables/useToast'
import api from '../api'

const toast = useToast()
const rules = ref([])
const protocols = ref([])
const saving = ref(false)
const testModel = ref('')
const prices = ref([])
const savingPrices = ref(false)

const testResult = computed(() => {
  const m = testModel.value.trim()
  if (!m) return ''
  for (const r of rules.value) {
    if (!r.pattern || !r.protocol) continue
    try {
      if (new RegExp(r.pattern).test(m)) return r.protocol
    } catch {}
  }
  return ''
})

async function load() {
  try {
    const [r, p, mp] = await Promise.all([
      api.get('/settings/protocol-rules'),
      api.get('/protocols'),
      api.get('/settings/model-prices'),
    ])
    rules.value = (r || []).map(x => ({ pattern: x.pattern || '', protocol: x.protocol || 'anthropic' }))
    protocols.value = p || []
    prices.value = (mp || []).map(x => ({ model: x.model || '', input: x.input || 0, output: x.output || 0 }))
  } catch (e) {
    toast.error('加载失败: ' + e.message)
  }
}

async function save() {
  saving.value = true
  try {
    const clean = rules.value.filter(r => r.pattern.trim() && r.protocol)
    await api.put('/settings/protocol-rules', clean)
    rules.value = clean
    toast.success('规则已保存')
  } catch (e) {
    toast.error('保存失败: ' + e.message)
  } finally {
    saving.value = false
  }
}

async function savePrices() {
  savingPrices.value = true
  try {
    const clean = prices.value
      .filter(p => p.model.trim())
      .map(p => ({ model: p.model.trim(), input: Number(p.input) || 0, output: Number(p.output) || 0 }))
    await api.put('/settings/model-prices', clean)
    prices.value = clean
    toast.success('价格已保存')
  } catch (e) {
    toast.error('保存失败: ' + e.message)
  } finally {
    savingPrices.value = false
  }
}

onMounted(load)
</script>

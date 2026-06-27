<template>
  <div>
    <div class="mb-5">
      <h2 class="page-title">规则配置</h2>
      <p class="page-subtitle">全局协议映射与模型定价表</p>
    </div>

    <div class="grid grid-cols-1 xl:grid-cols-2 gap-4">
      <!-- ===== 全局协议规则 ===== -->
      <div class="panel">
        <div class="px-4 h-11 flex items-center justify-between border-b border-line">
          <div>
            <span class="font-mono text-sm font-medium text-t1">全局协议规则</span>
            <span class="tick ml-2">REGEX MATCH</span>
          </div>
          <button class="btn-primary btn-sm" :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存' }}</button>
        </div>

        <div class="p-4">
          <p class="hint mb-3">按模型显示名正则匹配上游协议。优先级：模型显式 &gt; 供应商规则 &gt; <span class="font-medium text-t1">全局规则</span> &gt; 供应商默认。</p>
          <div class="space-y-2">
            <div v-for="(r, i) in rules" :key="i" class="flex gap-2 items-center">
              <span class="font-mono text-2xs text-t3 w-6 text-center shrink-0">{{ String(i + 1).padStart(2, '0') }}</span>
              <input v-model="r.pattern" class="input font-mono text-xs flex-1" placeholder="正则，如 ^claude- 或 gpt-.*" />
              <select v-model="r.protocol" class="input text-xs w-36 shrink-0">
                <option v-for="p in protocols" :key="p.value" :value="p.value">{{ p.name }}</option>
              </select>
              <button class="text-t3 hover:text-[rgb(var(--c-down))] px-2 shrink-0" @click="rules.splice(i, 1)">×</button>
            </div>
            <div v-if="!rules.length" class="empty-state inset !py-6">
              暂无全局规则
            </div>
          </div>
          <button class="btn-ghost btn-sm mt-3" @click="rules.push({ pattern: '', protocol: 'anthropic' })">+ 添加规则</button>
        </div>
      </div>

      <!-- ===== 规则测试器 ===== -->
      <div class="panel">
        <div class="px-4 h-11 flex items-center border-b border-line">
          <span class="font-mono text-sm font-medium text-t1">规则测试器</span>
          <span class="tick ml-2">DRY RUN</span>
        </div>
        <div class="p-4">
          <p class="hint mb-3">输入一个模型名，预览全局规则的首个命中结果（不含供应商级配置）</p>
          <div class="flex gap-2 items-center">
            <input v-model="testModel" class="input font-mono text-sm flex-1" placeholder="如 claude-3-5-sonnet" />
            <div class="shrink-0">
              <span v-if="testModel" class="badge font-mono" :class="testResult ? 'badge-signal' : 'badge-neutral'">
                {{ testResult || '未命中（用供应商默认）' }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ===== 全局模型价格 ===== -->
    <div class="panel mt-4">
      <div class="px-4 h-11 flex items-center justify-between border-b border-line">
        <div>
          <span class="font-mono text-sm font-medium text-t1">全局模型价格</span>
          <span class="tick ml-2">USD/1M TK</span>
        </div>
        <button class="btn-primary btn-sm" :disabled="savingPrices" @click="savePrices">{{ savingPrices ? '保存中…' : '保存' }}</button>
      </div>

      <div class="p-4">
        <p class="hint mb-3">单位：美元 / 100 万 tokens。模型名填 <code class="font-mono text-xs">default</code> 作为兜底价格。优先级：供应商模型价格 &gt; <span class="font-medium text-t1">全局价格</span> &gt; 不计费。</p>
        <div class="space-y-2">
          <div class="hidden lg:grid grid-cols-[40px_minmax(0,1fr)_120px_120px_32px] gap-2 items-center tick px-1">
            <span></span>
            <span>模型名</span>
            <span class="text-right pr-2">输入 $/1M</span>
            <span class="text-right pr-2">输出 $/1M</span>
            <span></span>
          </div>
          <div v-for="(p, i) in prices" :key="i" class="grid grid-cols-[40px_minmax(0,1fr)_120px_120px_32px] max-lg:grid-cols-[40px_minmax(0,1fr)_32px] gap-2 items-center">
            <span class="font-mono text-2xs text-t3 text-center">{{ String(i + 1).padStart(2, '0') }}</span>
            <input v-model="p.model" class="input font-mono text-xs" placeholder="模型名 或 default" />
            <input v-model.number="p.input" type="number" step="0.01" min="0" class="input text-xs text-right font-mono max-lg:hidden" placeholder="0" />
            <input v-model.number="p.output" type="number" step="0.01" min="0" class="input text-xs text-right font-mono max-lg:hidden" placeholder="0" />
            <button class="text-t3 hover:text-[rgb(var(--c-down))] px-1" @click="prices.splice(i, 1)">×</button>
          </div>
          <div v-if="!prices.length" class="empty-state inset !py-6">
            暂无价格条目（未配置时不计费）
          </div>
        </div>
        <button class="btn-ghost btn-sm mt-3" @click="prices.push({ model: '', input: 0, output: 0 })">+ 添加价格</button>
      </div>
    </div>

    <!-- ===== 熔断器配置 ===== -->
    <div class="panel mt-4">
      <div class="px-4 h-11 flex items-center justify-between border-b border-line">
        <div>
          <span class="font-mono text-sm font-medium text-t1">熔断器配置</span>
          <span class="tick ml-2">CIRCUIT BREAKER</span>
        </div>
        <button class="btn-primary btn-sm" :disabled="savingBreaker" @click="saveBreaker">{{ savingBreaker ? '保存中…' : '保存' }}</button>
      </div>

      <div class="p-4">
        <p class="hint mb-4">自动熔断故障渠道，防止级联失败。熔断后等待超时时间进入半开试探。</p>
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          <div>
            <label class="label">失败阈值</label>
            <input v-model.number="breaker.failure_threshold" type="number" min="1" class="input" />
            <p class="hint mt-1">连续失败多少次触发熔断</p>
          </div>
          <div>
            <label class="label">恢复阈值</label>
            <input v-model.number="breaker.success_threshold" type="number" min="1" class="input" />
            <p class="hint mt-1">半开状态连续成功多少次恢复</p>
          </div>
          <div>
            <label class="label">熔断超时（秒）</label>
            <input v-model.number="breaker.timeout_seconds" type="number" min="1" class="input" />
            <p class="hint mt-1">熔断后多久进入半开试探</p>
          </div>
          <div>
            <label class="label">错误率阈值</label>
            <input v-model.number="breaker.error_rate_threshold" type="number" min="0" max="1" step="0.01" class="input" />
            <p class="hint mt-1">错误率超过此值触发熔断（0-1）</p>
          </div>
          <div>
            <label class="label">最小请求数</label>
            <input v-model.number="breaker.min_requests" type="number" min="1" class="input" />
            <p class="hint mt-1">统计窗口最小请求数</p>
          </div>
          <div>
            <label class="label">单渠道重试次数</label>
            <input v-model.number="breaker.channel_max_retries" type="number" min="0" class="input" />
            <p class="hint mt-1">同一渠道临时错误重试次数</p>
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
const rules = ref([])
const protocols = ref([])
const saving = ref(false)
const testModel = ref('')
const prices = ref([])
const savingPrices = ref(false)
const breaker = ref({
  failure_threshold: 5,
  success_threshold: 2,
  timeout_seconds: 30,
  error_rate_threshold: 0.5,
  min_requests: 10,
  channel_max_retries: 1,
})
const savingBreaker = ref(false)

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
    const [r, p, mp, cb] = await Promise.all([
      api.get('/settings/protocol-rules'),
      api.get('/protocols'),
      api.get('/settings/model-prices'),
      api.get('/settings/circuit-breaker'),
    ])
    rules.value = (r || []).map(x => ({ pattern: x.pattern || '', protocol: x.protocol || 'anthropic' }))
    protocols.value = p || []
    prices.value = (mp || []).map(x => ({ model: x.model || '', input: x.input || 0, output: x.output || 0 }))
    if (cb) breaker.value = cb
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

async function saveBreaker() {
  savingBreaker.value = true
  try {
    await api.put('/settings/circuit-breaker', breaker.value)
    toast.success('熔断器配置已保存')
  } catch (e) {
    toast.error('保存失败: ' + e.message)
  } finally {
    savingBreaker.value = false
  }
}

onMounted(load)
</script>

<script setup>
import Drawer from './Drawer.vue'

const props = defineProps({
  log: { type: Object, default: null },
  payload: { type: Object, default: null },
  loading: { type: Boolean, default: false },
  error: { type: String, default: '' },
  helpers: { type: Object, required: true },
})
const emit = defineEmits(['close', 'copy-diagnostic', 'copy-payload'])
const payloadKeys = ['client_request', 'upstream_request', 'upstream_response', 'client_response']
</script>

<template>
  <Drawer :open="!!log" title="调用诊断" @close="emit('close')">
    <div v-if="log" class="space-y-5">
      <section aria-labelledby="diagnostic-summary">
        <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
          <h3 id="diagnostic-summary" class="text-base font-semibold">请求摘要</h3>
          <div class="flex flex-wrap gap-2"><span class="chip" :class="helpers.typeChip(log.type)">{{ helpers.typeName(log.type) }}</span><span class="chip" :class="helpers.statusChip(log.status)">HTTP {{ log.status || '—' }}</span></div>
        </div>
        <dl class="grid gap-3 rounded-lg border border-line bg-ghost/40 p-3 sm:grid-cols-2">
          <div><dt class="field-label">时间</dt><dd class="font-mono text-xs">{{ helpers.fmt(log.created_at) }}</dd></div>
          <div><dt class="field-label">日志 ID</dt><dd class="break-all font-mono text-xs">{{ log.id || '—' }}</dd></div>
          <div><dt class="field-label">请求 ID</dt><dd class="break-all font-mono text-xs">{{ log.request_id || '—' }}</dd></div>
          <div><dt class="field-label">上游请求 ID</dt><dd class="break-all font-mono text-xs">{{ log.upstream_request_id || '—' }}</dd></div>
          <div><dt class="field-label">渠道</dt><dd class="break-all">{{ log.channel_name || (log.channel_id ? `#${log.channel_id}` : '—') }}</dd></div>
          <div><dt class="field-label">协议</dt><dd class="break-all font-mono text-xs">{{ log.endpoint_type || '—' }} → {{ log.api_type || '—' }}</dd></div>
          <div><dt class="field-label">客户端模型</dt><dd class="break-all font-mono text-xs font-medium">{{ log.src_model || '—' }}</dd></div>
          <div v-if="helpers.isModelMapped(log)"><dt class="field-label">实际请求模型</dt><dd class="break-all font-mono text-xs text-soft">{{ log.mapped_model }}</dd></div>
          <div><dt class="field-label">令牌 / 分组</dt><dd class="break-all font-mono text-xs">{{ log.token_name || '—' }} · {{ log.group || '—' }}</dd></div>
          <div><dt class="field-label">Tokens / 费用</dt><dd class="font-mono text-xs">{{ log.prompt_tokens || 0 }} / {{ log.completion_tokens || 0 }} · {{ helpers.cost(log.quota) }}<span v-if="log.usage_estimated" class="ml-1 text-test">估算</span></dd></div>
          <div><dt class="field-label">缓存写入 / 读取 / 推理</dt><dd class="font-mono text-xs">{{ log.cache_creation_input_tokens || 0 }} / {{ log.cache_read_input_tokens || 0 }} / {{ log.reasoning_tokens || 0 }}</dd></div>
          <div><dt class="field-label">耗时</dt><dd class="font-mono text-xs">{{ log.use_time_ms || 0 }} ms · 首字 {{ log.first_byte_ms || 0 }} ms</dd></div>
        </dl>
      </section>

      <section aria-labelledby="diagnostic-error"><h3 id="diagnostic-error" class="mb-3 text-base font-semibold">错误</h3><pre v-if="log.error" class="max-h-64 overflow-auto whitespace-pre-wrap break-all rounded-lg border border-trip/30 bg-trip-wash p-3 text-xs text-trip">{{ log.error }}</pre><div v-else class="rounded-lg border border-dashed border-line p-4 text-sm text-soft">该日志没有错误信息。</div></section>

      <section aria-labelledby="diagnostic-route">
        <div class="mb-3 flex items-center justify-between gap-2"><h3 id="diagnostic-route" class="text-base font-semibold">故障转移步骤</h3><span class="chip">{{ log._failover_chain.length }} 次</span></div>
        <ol v-if="log._failover_chain.length" class="route-timeline space-y-3">
          <li v-for="(attempt, index) in log._failover_chain" :key="`${log.id}-${index}`" class="rounded-lg border border-line bg-white p-3">
            <div class="flex flex-wrap items-center justify-between gap-2"><span class="font-medium">步骤 {{ index + 1 }} · {{ attempt.channel_name || (attempt.channel_id ? `#${attempt.channel_id}` : '未知渠道') }}</span><span class="chip" :class="helpers.decisionChip(attempt.decision)">{{ helpers.decisionName(attempt.decision) }}</span></div>
            <div class="mt-2 flex flex-wrap gap-2"><span class="chip" :class="helpers.statusChip(attempt.status)">HTTP {{ attempt.status || '—' }}</span><span class="chip chip-blue">{{ attempt.api_type || '—' }}</span><span class="chip" :class="attempt.retryable ? 'chip-test' : ''">{{ attempt.retryable ? '可重试' : '不可重试' }}</span></div>
            <p class="mt-3 break-all text-xs" :class="attempt.error ? 'text-trip' : 'text-soft'">{{ attempt.error_category ? `${attempt.error_category} · ` : '' }}{{ attempt.error || '无错误信息' }}</p>
          </li>
        </ol>
        <div v-else class="rounded-lg border border-dashed border-line p-4 text-sm text-soft">该日志没有故障转移步骤。</div>
      </section>

      <section aria-labelledby="diagnostic-payload">
        <div class="mb-3 flex flex-wrap items-center justify-between gap-2"><div><h3 id="diagnostic-payload" class="text-base font-semibold">完整调用内容</h3><p class="mt-1 text-xs text-soft">客户端 → APIRelay → 上游 → 客户端</p></div><span class="chip" :class="log.has_full_record ? 'chip-blue' : ''">{{ log.has_full_record ? `gzip ${helpers.formatBytes(log.payload_original_size)} → ${helpers.formatBytes(log.payload_compressed_size)}` : '仅摘要' }}</span></div>
        <div v-if="loading" class="rounded-xl border border-line bg-ghost/40 p-5 text-center text-sm text-soft">正在解压完整调用内容…</div>
        <div v-else-if="error" class="rounded-xl border border-trip/25 bg-trip-wash p-4 text-sm text-trip">{{ error }}</div>
        <div v-else-if="payload" class="route-timeline space-y-5"><article v-for="key in payloadKeys" :key="key"><div class="mb-2 flex items-center justify-between gap-2"><h4 class="font-cond text-sm font-semibold text-ink">{{ helpers.payloadTitle(key) }}</h4><button v-if="payload[key]" class="btn btn-sm" type="button" @click="emit('copy-payload', payload[key])">复制</button></div><pre v-if="payload[key]" class="log-code">{{ helpers.prettyPayload(payload[key]) }}</pre><div v-else class="rounded-xl border border-dashed border-line px-4 py-3 text-xs text-soft">此阶段未配置记录或没有可记录内容。</div></article></div>
        <div v-else class="rounded-xl border border-dashed border-line p-4 text-sm text-soft">该日志仅保留路由、计费、耗时和错误摘要。</div>
      </section>
    </div>
    <template #footer><div class="flex justify-end"><button v-if="log" class="btn btn-primary" type="button" @click="emit('copy-diagnostic', log)">复制诊断包</button></div></template>
  </Drawer>
</template>

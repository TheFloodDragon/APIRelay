<script setup>
import Drawer from './Drawer.vue'
import InlineNotice from './InlineNotice.vue'
import ConsoleIcon from './ConsoleIcon.vue'

defineProps({
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
  <Drawer :open="!!log" title="调用诊断" width="max-w-3xl" @close="emit('close')">
    <div v-if="log" class="space-y-7">
      <section aria-labelledby="diagnostic-overview">
        <div class="mb-3 flex flex-wrap items-start justify-between gap-3">
          <div>
            <p class="font-mono text-[10px] uppercase tracking-[.12em] text-blue-grid">Overview</p>
            <h3 id="diagnostic-overview" class="mt-1 text-base font-semibold text-ink">概览</h3>
          </div>
          <div class="flex items-center gap-2 font-mono text-xs font-semibold text-ink">
            <span class="h-2.5 w-2.5 rounded-full" :class="helpers.statusTone(log)" />
            HTTP {{ log.status || '—' }} · {{ helpers.typeName(log.type) }}
          </div>
        </div>

        <dl class="grid border-y border-line sm:grid-cols-2">
          <div class="border-b border-line px-0 py-3 sm:border-r sm:px-3"><dt class="field-label">时间</dt><dd class="font-mono text-xs text-ink">{{ helpers.fmt(log.created_at) }}</dd></div>
          <div class="border-b border-line px-0 py-3 sm:px-3"><dt class="field-label">请求路径</dt><dd class="break-all font-mono text-xs text-ink">{{ helpers.requestPath(log) }}</dd></div>
          <div class="border-b border-line px-0 py-3 sm:border-r sm:px-3"><dt class="field-label">请求 ID / 日志 ID</dt><dd class="break-all font-mono text-xs text-ink">{{ log.request_id || '—' }} · {{ log.id || '—' }}</dd></div>
          <div class="border-b border-line px-0 py-3 sm:px-3"><dt class="field-label">上游请求 ID</dt><dd class="break-all font-mono text-xs text-ink">{{ log.upstream_request_id || '—' }}</dd></div>
          <div class="border-b border-line px-0 py-3 sm:border-r sm:px-3"><dt class="field-label">渠道</dt><dd class="break-all text-xs text-ink">{{ log.channel_name || (log.channel_id ? `#${log.channel_id}` : '—') }}</dd></div>
          <div class="border-b border-line px-0 py-3 sm:px-3"><dt class="field-label">协议转换</dt><dd class="break-all font-mono text-xs text-ink">{{ log.endpoint_type || '—' }} → {{ log.api_type || '—' }}</dd></div>
          <div class="px-0 py-3 sm:border-r sm:px-3"><dt class="field-label">模型</dt><dd class="break-all font-mono text-xs text-ink">{{ log.src_model || '—' }}<template v-if="helpers.isModelMapped(log)"> → {{ log.mapped_model }}</template></dd></div>
          <div class="px-0 py-3 sm:px-3"><dt class="field-label">令牌 / 分组 / 模式</dt><dd class="break-all font-mono text-xs text-ink">{{ log.token_name || '—' }} · {{ log.group || '—' }} · {{ log.is_stream ? 'stream' : 'sync' }}</dd></div>
        </dl>

        <InlineNotice v-if="log.error" tone="danger" title="请求错误" class="mt-3">
          <pre class="max-h-52 overflow-auto whitespace-pre-wrap break-all font-mono text-[11px] leading-5">{{ log.error }}</pre>
        </InlineNotice>
        <InlineNotice v-else tone="success" title="未记录错误" class="mt-3">请求摘要中没有错误信息。</InlineNotice>
      </section>

      <section aria-labelledby="diagnostic-failover">
        <div class="mb-3 flex items-center justify-between gap-3">
          <div>
            <p class="font-mono text-[10px] uppercase tracking-[.12em] text-blue-grid">Routing</p>
            <h3 id="diagnostic-failover" class="mt-1 text-base font-semibold text-ink">故障转移链</h3>
          </div>
          <span class="font-mono text-xs text-soft">{{ log._failover_chain.length }} ATTEMPTS</span>
        </div>
        <ol v-if="log._failover_chain.length" class="route-timeline space-y-4">
          <li v-for="(attempt, index) in log._failover_chain" :key="`${log.id}-${index}`" class="border-b border-line pb-4 last:border-b-0 last:pb-0">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <div class="min-w-0">
                <div class="truncate text-sm font-medium text-ink">{{ index + 1 }}. {{ attempt.channel_name || (attempt.channel_id ? `#${attempt.channel_id}` : '未知渠道') }}</div>
                <div class="mt-1 font-mono text-[10px] text-faint">{{ attempt.api_type || '未知协议' }} · {{ helpers.fmt(attempt.at_ms) }}</div>
              </div>
              <div class="flex items-center gap-3 font-mono text-[11px]">
                <span class="text-soft">HTTP {{ attempt.status || '—' }}</span>
                <span :class="attempt.retryable ? 'text-test' : 'text-faint'">{{ attempt.retryable ? '可重试' : '不可重试' }}</span>
                <span class="font-semibold" :class="helpers.decisionChip(attempt.decision).includes('trip') ? 'text-trip' : 'text-ink'">{{ helpers.decisionName(attempt.decision) }}</span>
              </div>
            </div>
            <p v-if="attempt.error" class="mt-2 break-all font-mono text-[11px] leading-5 text-trip">{{ attempt.error_category ? `${attempt.error_category} · ` : '' }}{{ attempt.error }}</p>
          </li>
        </ol>
        <InlineNotice v-else tone="info">该日志没有故障转移步骤。</InlineNotice>
      </section>

      <section aria-labelledby="diagnostic-usage">
        <div class="mb-3">
          <p class="font-mono text-[10px] uppercase tracking-[.12em] text-blue-grid">Metering</p>
          <h3 id="diagnostic-usage" class="mt-1 text-base font-semibold text-ink">用量计费</h3>
        </div>
        <dl class="grid overflow-hidden border-y border-line sm:grid-cols-2 lg:grid-cols-4">
          <div class="border-b border-line py-3 sm:border-r sm:px-3 lg:border-b-0"><dt class="field-label">输入 / 输出 Tokens</dt><dd class="font-mono text-sm font-semibold text-ink">{{ log.prompt_tokens || 0 }} / {{ log.completion_tokens || 0 }}</dd><p v-if="log.usage_estimated" class="mt-1 text-[10px] text-test">用量为估算值</p></div>
          <div class="border-b border-line py-3 sm:px-3 lg:border-b-0 lg:border-r"><dt class="field-label">缓存写 / 读 / 推理</dt><dd class="font-mono text-sm font-semibold text-ink">{{ log.cache_creation_input_tokens || 0 }} / {{ log.cache_read_input_tokens || 0 }} / {{ log.reasoning_tokens || 0 }}</dd></div>
          <div class="border-b border-line py-3 sm:border-b-0 sm:border-r sm:px-3"><dt class="field-label">总延迟 / 首字</dt><dd class="font-mono text-sm font-semibold text-ink">{{ log.use_time_ms || 0 }} / {{ log.first_byte_ms || 0 }} ms</dd></div>
          <div class="py-3 sm:px-3"><dt class="field-label">费用</dt><dd class="font-mono text-sm font-semibold text-ink">{{ helpers.cost(log.quota) }}</dd></div>
        </dl>
      </section>

      <section aria-labelledby="diagnostic-payload">
        <div class="mb-3 flex flex-wrap items-end justify-between gap-3">
          <div>
            <p class="font-mono text-[10px] uppercase tracking-[.12em] text-blue-grid">Payload</p>
            <h3 id="diagnostic-payload" class="mt-1 text-base font-semibold text-ink">完整载荷</h3>
            <p class="mt-1 text-xs text-soft">客户端 → APIRelay → 上游 → 客户端</p>
          </div>
          <span class="font-mono text-[10px] text-soft">{{ log.has_full_record ? `gzip ${helpers.formatBytes(log.payload_original_size)} → ${helpers.formatBytes(log.payload_compressed_size)}` : '仅摘要' }}</span>
        </div>
        <InlineNotice v-if="loading" tone="info">正在解压完整调用内容…</InlineNotice>
        <InlineNotice v-else-if="error" tone="danger" title="载荷读取失败">{{ error }}</InlineNotice>
        <div v-else-if="payload" class="space-y-5">
          <article v-for="key in payloadKeys" :key="key" class="border-t border-line pt-4 first:border-t-0 first:pt-0">
            <div class="mb-2 flex items-center justify-between gap-2">
              <h4 class="text-sm font-semibold text-ink">{{ helpers.payloadTitle(key) }}</h4>
              <button v-if="payload[key]" class="btn btn-ghost btn-sm" type="button" @click="emit('copy-payload', payload[key])">
                <ConsoleIcon name="command" class="h-4 w-4" />复制载荷
              </button>
            </div>
            <pre v-if="payload[key]" class="log-code">{{ helpers.prettyPayload(payload[key]) }}</pre>
            <div v-else class="border border-dashed border-line px-4 py-3 text-xs text-soft">此阶段未配置记录或没有可记录内容。</div>
          </article>
        </div>
        <InlineNotice v-else tone="warning">该日志仅保留路由、计费、耗时和错误摘要。</InlineNotice>
      </section>
    </div>

    <template #footer>
      <div class="flex w-full items-center justify-between gap-3">
        <span v-if="log" class="hidden font-mono text-[10px] text-faint sm:inline">LOG {{ log.id }} · {{ log.request_id || 'NO REQUEST ID' }}</span>
        <button v-if="log" class="btn btn-primary ml-auto" type="button" @click="emit('copy-diagnostic', log)">
          <ConsoleIcon name="command" class="h-4 w-4" />复制诊断包
        </button>
      </div>
    </template>
  </Drawer>
</template>

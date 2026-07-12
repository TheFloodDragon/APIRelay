<script setup>
import ConsoleIcon from './ConsoleIcon.vue'
import DataToolbar from './DataToolbar.vue'

defineProps({
  summary: { type: Object, default: () => ({ total: 0 }) },
  segments: { type: Array, default: () => [] },
  query: { type: String, default: '' },
  status: { type: String, default: 'all' },
  selectedCount: { type: Number, default: 0 },
  bulkDeleting: { type: Boolean, default: false },
  reordering: { type: Boolean, default: false },
  visibleCount: { type: Number, default: 0 },
})

const emit = defineEmits(['update:query', 'update:status', 'bulk-delete'])
</script>

<template>
  <header class="channel-queue-header">
    <div class="flex min-w-0 items-start justify-between gap-3">
      <div class="min-w-0">
        <div class="dim-title">渠道队列</div>
        <p class="mt-1 text-[11px] text-soft">显示 {{ visibleCount }} / {{ summary.total }}，队列顺序即故障转移优先级。</p>
      </div>
      <span v-if="reordering" class="chip chip-test shrink-0" role="status">同步中</span>
      <span v-else class="chip shrink-0">{{ summary.total }} 项</span>
    </div>

    <DataToolbar label="渠道搜索与批量操作" class="mt-3">
      <label class="relative block min-w-0 flex-1">
        <ConsoleIcon name="search" class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-faint" />
        <span class="sr-only">搜索渠道</span>
        <input
          :value="query"
          class="input input-mono pl-9"
          type="search"
          placeholder="搜索名称、地址或模型"
          @input="emit('update:query', $event.target.value)"
        />
      </label>
      <template #actions>
        <button
          v-if="selectedCount"
          type="button"
          class="btn btn-danger btn-sm w-full"
          :disabled="bulkDeleting"
          @click="emit('bulk-delete')"
        >
          <ConsoleIcon name="trash" class="h-4 w-4" />
          {{ bulkDeleting ? '删除中' : `删除已选 ${selectedCount} 项` }}
        </button>
      </template>
    </DataToolbar>

    <div class="queue-status-grid mt-3" aria-label="渠道状态筛选">
      <button
        type="button"
        class="queue-status-button"
        :class="status === 'all' ? 'route-bus-active' : ''"
        :aria-pressed="status === 'all'"
        @click="emit('update:status', 'all')"
      >
        <span>全部</span><b>{{ summary.total }}</b>
      </button>
      <button
        v-for="segment in segments"
        :key="segment.key"
        type="button"
        class="route-bus-segment"
        :class="[`route-bus-${segment.tone}`, status === segment.key ? 'route-bus-active' : '']"
        :aria-pressed="status === segment.key"
        @click="emit('update:status', status === segment.key ? 'all' : segment.key)"
      >
        <span>{{ segment.label }}</span><b>{{ segment.count }}</b>
      </button>
    </div>
  </header>
</template>

<style scoped>
.channel-queue-header {
  border-bottom: 1px solid rgb(var(--color-border));
  background: rgb(var(--color-surface-1));
  padding: 14px;
}
.channel-queue-header :deep(.data-toolbar) {
  display: flex;
  padding: 0;
  border: 0;
  background: transparent;
}
.channel-queue-header :deep(.data-toolbar-primary),
.channel-queue-header :deep(.data-toolbar-actions) { width: 100%; }
.queue-status-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 6px; }
.queue-status-button,
.route-bus-segment {
  --status-color: rgb(var(--color-text-muted));
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 5px;
  border: 1px solid rgb(var(--color-border));
  border-radius: 6px;
  background: rgb(var(--color-surface-1));
  padding: 7px 8px;
  color: rgb(var(--color-text-secondary));
  font-size: 10px;
  transition: border-color 150ms ease, background-color 150ms ease, color 150ms ease;
}
.queue-status-button b,
.route-bus-segment b { font-family: 'Spline Sans Mono', monospace; color: rgb(var(--color-text)); }
.route-bus-segment:hover,
.queue-status-button:hover { border-color: var(--status-color); }
.route-bus-active { border-color: var(--status-color); background: color-mix(in srgb, var(--status-color) 16%, rgb(var(--color-surface-2))); color: rgb(var(--color-text)); }
.route-bus-run { --status-color: #50705a; }
.route-bus-test { --status-color: #9a6a2f; }
.route-bus-trip { --status-color: #a4382f; }
.route-bus-off { --status-color: #938a7c; }
@media (max-width: 390px) {
  .channel-queue-header { padding: 12px; }
  .queue-status-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
</style>

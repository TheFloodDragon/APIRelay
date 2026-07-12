<script setup>
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
  <div class="border-b border-line bg-white px-4 py-4 sm:px-5">
    <div class="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
      <div class="min-w-0 flex-1">
        <div class="flex flex-wrap items-baseline gap-x-3 gap-y-1">
          <span class="dim-title">路由运行台</span>
          <span class="font-mono text-[10px] uppercase tracking-[0.14em] text-faint">{{ summary.total }} feeders online</span>
        </div>
        <div class="mt-1 text-[12px] text-soft">点击母线区段快速筛选；列表顺序即故障转移优先级。</div>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <button v-if="selectedCount" type="button" class="btn btn-danger btn-sm" :disabled="bulkDeleting" @click="emit('bulk-delete')">{{ bulkDeleting ? '删除中' : `删除已选 ${selectedCount} 项` }}</button>
        <span v-if="reordering" class="chip chip-test" role="status">正在同步优先级</span>
        <span v-else class="chip">显示 {{ visibleCount }} / {{ summary.total }}</span>
      </div>
    </div>

    <div class="route-bus mt-4" aria-label="路由状态母线">
      <button
        v-for="segment in segments"
        :key="segment.key"
        type="button"
        class="route-bus-segment"
        :class="[`route-bus-${segment.tone}`, status === segment.key ? 'route-bus-active' : '']"
        :style="{ '--segment-grow': Math.max(segment.count, 1) }"
        :aria-pressed="status === segment.key"
        @click="emit('update:status', status === segment.key ? 'all' : segment.key)"
      >
        <span class="route-bus-line" aria-hidden="true"></span>
        <span class="route-bus-copy"><b>{{ segment.count }}</b><span>{{ segment.label }} · {{ segment.percent }}%</span></span>
      </button>
    </div>

    <div class="mt-4 grid gap-3 lg:grid-cols-[minmax(280px,1fr)_auto] lg:items-center">
      <label class="relative block">
        <span class="pointer-events-none absolute inset-y-0 left-3 flex items-center text-faint" aria-hidden="true">⌕</span>
        <span class="sr-only">搜索渠道</span>
        <input :value="query" class="input input-mono pl-9" type="search" placeholder="搜索渠道、分组、地址或模型" @input="emit('update:query', $event.target.value)" />
      </label>
      <div class="flex flex-wrap gap-1.5" aria-label="渠道状态筛选">
        <button type="button" class="chip" :class="status === 'all' ? 'chip-blue' : ''" :aria-pressed="status === 'all'" @click="emit('update:status', 'all')">全部 {{ summary.total }}</button>
        <button v-for="segment in segments" :key="`filter-${segment.key}`" type="button" class="chip" :class="status === segment.key ? `chip-${segment.tone === 'off' ? 'test' : segment.tone}` : ''" :aria-pressed="status === segment.key" @click="emit('update:status', segment.key)">{{ segment.label }} {{ segment.count }}</button>
      </div>
    </div>
  </div>
</template>

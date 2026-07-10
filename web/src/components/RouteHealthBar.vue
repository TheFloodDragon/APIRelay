<script setup>
import StatusBadge from './StatusBadge.vue'

defineProps({ rows: { type: Array, default: () => [] } })
</script>

<template>
  <div v-if="rows.length">
    <div class="hidden xl:block">
      <div class="grid grid-cols-[64px_minmax(150px,1.1fr)_120px_105px_70px_minmax(210px,1.5fr)] gap-3 border-b border-line bg-ghost/70 px-4 py-2 text-xs font-medium text-soft">
        <span>优先级</span><span>渠道</span><span>协议</span><span>状态</span><span class="text-right">模型数</span><span>最近延迟 / 错误</span>
      </div>
      <div v-for="row in rows" :key="row.id" class="grid grid-cols-[64px_minmax(150px,1.1fr)_120px_105px_70px_minmax(210px,1.5fr)] items-center gap-3 border-b border-line px-4 py-3 text-sm last:border-b-0 hover:bg-canvas">
        <span class="font-mono text-xs text-soft">{{ String(row.priority).padStart(2, '0') }}</span>
        <div class="min-w-0"><div class="truncate font-medium text-ink" :title="row.name">{{ row.name }}</div><div class="mt-0.5 truncate text-xs text-soft">{{ row.group }}</div></div>
        <span class="truncate font-mono text-xs text-soft" :title="row.protocol">{{ row.protocol }}</span>
        <StatusBadge :status="row.status" :label="row.statusLabel" />
        <span class="text-right font-mono text-[13px] text-ink">{{ row.modelCount }}</span>
        <div class="min-w-0"><div class="truncate text-xs text-ink" :title="row.recentPrimary">{{ row.recentPrimary }}</div><div v-if="row.recentSecondary" class="mt-0.5 truncate text-xs text-soft" :title="row.recentSecondary">{{ row.recentSecondary }}</div></div>
      </div>
    </div>

    <div class="divide-y divide-line xl:hidden">
      <article v-for="row in rows" :key="row.id" class="p-4">
        <div class="flex items-start gap-3">
          <span class="mt-0.5 rounded-md bg-ghost px-2 py-1 font-mono text-xs text-soft">{{ String(row.priority).padStart(2, '0') }}</span>
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center justify-between gap-2"><h3 class="truncate font-medium text-ink">{{ row.name }}</h3><StatusBadge :status="row.status" :label="row.statusLabel" /></div>
            <div class="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-soft"><span>{{ row.protocol }}</span><span>{{ row.group }}</span><span>{{ row.modelCount }} 个模型</span></div>
            <p class="mt-2 break-words text-xs text-ink">{{ row.recentPrimary }}</p>
            <p v-if="row.recentSecondary" class="mt-1 break-words text-xs text-soft">{{ row.recentSecondary }}</p>
          </div>
        </div>
      </article>
    </div>
  </div>
  <div v-else class="px-4 py-10 text-center text-sm text-soft">尚未配置渠道</div>
</template>

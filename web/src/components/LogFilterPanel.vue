<script setup>
import DataToolbar from './DataToolbar.vue'
import ConsoleIcon from './ConsoleIcon.vue'

defineProps({
  filters: { type: Object, required: true },
  logTypes: { type: Array, default: () => [] },
  timeRanges: { type: Array, default: () => [] },
  activeCount: { type: Number, default: 0 },
  moreCount: { type: Number, default: 0 },
  expanded: { type: Boolean, default: false },
})
const emit = defineEmits(['apply', 'clear', 'quick', 'update:expanded'])
</script>

<template>
  <form class="log-filter-panel min-w-0 space-y-2" @submit.prevent="emit('apply')">
    <DataToolbar label="日志过滤工具栏" sticky>
      <label class="w-[132px] shrink-0">
        <span class="field-label">时间范围</span>
        <select v-model="filters.range" class="input" @change="emit('apply')">
          <option v-for="item in timeRanges" :key="item.value" :value="item.value">{{ item.label }}</option>
        </select>
      </label>
      <label class="w-[112px] shrink-0">
        <span class="field-label">类型</span>
        <select v-model="filters.type" class="input" @change="emit('apply')">
          <option v-for="item in logTypes" :key="item.value" :value="item.value">{{ item.label }}</option>
        </select>
      </label>
      <label class="w-[104px] shrink-0">
        <span class="field-label">状态</span>
        <input v-model="filters.status" class="input input-mono" inputmode="numeric" placeholder="200 / 503" />
      </label>
      <label class="w-[112px] shrink-0">
        <span class="field-label">渠道</span>
        <input v-model="filters.channel_id" class="input input-mono" inputmode="numeric" placeholder="渠道 ID" />
      </label>
      <label class="min-w-[150px] flex-1">
        <span class="field-label">模型</span>
        <input v-model="filters.model" class="input input-mono" placeholder="gpt-4o" />
      </label>
      <label class="min-w-[180px] flex-[1.25]">
        <span class="field-label">请求 ID</span>
        <input v-model="filters.request_id" class="input input-mono" placeholder="req..." />
      </label>

      <template #actions>
        <span class="hidden font-mono text-[10px] text-soft xl:inline" aria-live="polite">{{ activeCount }} ACTIVE</span>
        <button
          class="btn btn-sm"
          type="button"
          :aria-expanded="expanded"
          aria-controls="log-advanced-filters"
          @click="emit('update:expanded', !expanded)"
        >
          <ConsoleIcon name="filter" class="h-4 w-4" />
          高级<span v-if="moreCount"> {{ moreCount }}</span>
          <ConsoleIcon name="chevronDown" class="h-3.5 w-3.5 transition-transform" :class="{ 'rotate-180': expanded }" />
        </button>
        <button class="btn btn-sm" type="button" :disabled="activeCount === 0" @click="emit('clear')">清除</button>
        <button class="btn btn-primary btn-sm min-w-16" type="submit">
          <ConsoleIcon name="search" class="h-4 w-4" />查询
        </button>
      </template>
    </DataToolbar>

    <div v-show="expanded" id="log-advanced-filters" class="border border-line bg-surface px-3 py-3">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <div>
          <div class="text-xs font-semibold text-ink">高级筛选</div>
          <div class="mt-0.5 text-[11px] text-soft">细化身份、响应模式与状态范围。</div>
        </div>
        <div class="flex items-center gap-1.5" aria-label="快捷异常筛选">
          <span class="mr-1 text-[10px] text-faint">快捷</span>
          <button class="btn btn-ghost btn-sm" type="button" @click="emit('quick', '2')">异常</button>
          <button class="btn btn-ghost btn-sm" type="button" @click="emit('quick', '2', '429')">429</button>
          <button class="btn btn-ghost btn-sm" type="button" @click="emit('quick', '2', '504')">504</button>
        </div>
      </div>
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
        <label><span class="field-label">令牌</span><input v-model="filters.token_name" class="input input-mono" placeholder="token name" /></label>
        <label><span class="field-label">上游请求 ID</span><input v-model="filters.upstream_request_id" class="input input-mono" placeholder="upstream..." /></label>
        <label><span class="field-label">响应模式</span><select v-model="filters.is_stream" class="input"><option value="">全部</option><option value="true">流式</option><option value="false">非流式</option></select></label>
        <label><span class="field-label">完整记录</span><select v-model="filters.has_full_record" class="input"><option value="">全部</option><option value="true">有完整内容</option><option value="false">仅摘要</option></select></label>
        <label><span class="field-label">最低状态码</span><input v-model="filters.status_min" class="input input-mono" inputmode="numeric" placeholder="400" /></label>
        <label><span class="field-label">最高状态码</span><input v-model="filters.status_max" class="input input-mono" inputmode="numeric" placeholder="599" /></label>
      </div>
    </div>
  </form>
</template>

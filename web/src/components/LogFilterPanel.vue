<script setup>
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
  <section class="sheet log-filter-panel min-w-0">
    <div class="sheet-head">
      <div>
        <div class="dim-title">筛选日志</div>
        <div class="mt-1 text-xs text-soft" aria-live="polite">{{ activeCount }} 个筛选条件已启用</div>
      </div>
      <div class="flex flex-wrap items-center gap-2" aria-label="快捷筛选">
        <button class="btn btn-sm" type="button" @click="emit('quick', '2')">异常</button>
        <button class="btn btn-sm" type="button" @click="emit('quick', '2', '429')">429</button>
        <button class="btn btn-sm" type="button" @click="emit('quick', '2', '504')">504</button>
        <button class="btn btn-sm" type="button" :disabled="activeCount === 0" @click="emit('clear')">清除</button>
      </div>
    </div>

    <form class="space-y-3 p-4" @submit.prevent="emit('apply')">
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
        <label><span class="field-label">类型</span><select v-model="filters.type" class="input" @change="emit('apply')"><option v-for="item in logTypes" :key="item.value" :value="item.value">{{ item.label }}</option></select></label>
        <label><span class="field-label">时间范围</span><select v-model="filters.range" class="input" @change="emit('apply')"><option v-for="item in timeRanges" :key="item.value" :value="item.value">{{ item.label }}</option></select></label>
        <label><span class="field-label">模型</span><input v-model="filters.model" class="input input-mono" placeholder="gpt-4o" /></label>
        <label><span class="field-label">令牌</span><input v-model="filters.token_name" class="input input-mono" placeholder="token name" /></label>
        <label><span class="field-label">状态码</span><input v-model="filters.status" class="input input-mono" inputmode="numeric" placeholder="503" /></label>
      </div>

      <button class="btn btn-sm" type="button" :aria-expanded="expanded" aria-controls="log-more-filters" @click="emit('update:expanded', !expanded)">
        更多筛选<span v-if="moreCount">（{{ moreCount }}）</span>
      </button>

      <div v-show="expanded" id="log-more-filters" class="grid gap-3 rounded-xl border border-line bg-ghost/40 p-3 sm:grid-cols-2 xl:grid-cols-4">
        <label><span class="field-label">请求 ID</span><input v-model="filters.request_id" class="input input-mono" placeholder="req..." /></label>
        <label><span class="field-label">上游请求 ID</span><input v-model="filters.upstream_request_id" class="input input-mono" placeholder="upstream..." /></label>
        <label><span class="field-label">渠道 ID</span><input v-model="filters.channel_id" class="input input-mono" inputmode="numeric" placeholder="42" /></label>
        <label><span class="field-label">响应模式</span><select v-model="filters.is_stream" class="input"><option value="">全部</option><option value="true">流式</option><option value="false">非流式</option></select></label>
        <label><span class="field-label">完整记录</span><select v-model="filters.has_full_record" class="input"><option value="">全部</option><option value="true">有完整内容</option><option value="false">仅摘要</option></select></label>
        <label><span class="field-label">最低状态码</span><input v-model="filters.status_min" class="input input-mono" inputmode="numeric" placeholder="400" /></label>
        <label><span class="field-label">最高状态码</span><input v-model="filters.status_max" class="input input-mono" inputmode="numeric" placeholder="599" /></label>
      </div>

      <div class="flex justify-end"><button class="btn btn-primary min-w-28" type="submit">查询</button></div>
    </form>
  </section>
</template>

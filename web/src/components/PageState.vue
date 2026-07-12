<script setup>
defineProps({
  loading: { type: Boolean, default: false },
  error: { type: String, default: '' },
  empty: { type: Boolean, default: false },
  emptyText: { type: String, default: '暂无数据' },
  emptyHint: { type: String, default: '' },
})

defineEmits(['retry'])
</script>

<template>
  <div v-if="loading" class="page-state page-state-loading" role="status" aria-live="polite">
    <span class="sr-only">正在加载，请稍候</span>
    <div class="state-skeleton" aria-hidden="true">
      <div class="state-skeleton-head"><span></span><span></span></div>
      <div class="state-skeleton-row" v-for="index in 5" :key="index"><span></span><span></span><span></span><span></span></div>
    </div>
  </div>

  <div v-else-if="error" class="page-state page-state-error" role="alert">
    <div class="page-state-mark">!</div>
    <div class="min-w-0">
      <h2>内容加载失败</h2>
      <p>{{ error }}</p>
    </div>
    <button class="btn btn-sm" type="button" @click="$emit('retry')">重新加载</button>
  </div>

  <div v-else-if="empty" class="page-state page-state-empty">
    <div class="page-state-mark">—</div>
    <div class="min-w-0">
      <h2>{{ emptyText }}</h2>
      <p v-if="emptyHint">{{ emptyHint }}</p>
      <div v-if="$slots.empty" class="mt-4"><slot name="empty" /></div>
    </div>
  </div>

  <slot v-else />
</template>

<style scoped>
.state-skeleton span {
  background: linear-gradient(100deg, rgb(var(--color-surface-2)) 20%, rgb(var(--color-surface-3)) 42%, rgb(var(--color-surface-2)) 64%);
  background-size: 220% 100%;
  animation: skeleton-flow 1.35s ease-in-out infinite;
}
@keyframes skeleton-flow { to { background-position-x: -220%; } }
@media (prefers-reduced-motion: reduce) { .state-skeleton span { animation: none; } }
</style>

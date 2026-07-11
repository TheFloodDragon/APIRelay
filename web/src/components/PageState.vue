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
  <div v-if="loading" class="min-h-52 rounded-lg border border-line bg-white p-5" role="status" aria-live="polite">
    <span class="sr-only">正在加载，请稍候</span>
    <div class="state-skeleton mx-auto max-w-4xl" aria-hidden="true">
      <div class="flex items-center gap-3">
        <span class="h-9 w-9 rounded-lg"></span>
        <div class="flex-1 space-y-2"><span class="block h-3 w-32 rounded"></span><span class="block h-2.5 w-52 max-w-full rounded"></span></div>
      </div>
      <div class="mt-5 grid gap-3 sm:grid-cols-3"><span class="h-16 rounded-lg"></span><span class="h-16 rounded-lg"></span><span class="h-16 rounded-lg"></span></div>
      <div class="mt-4 space-y-2"><span class="block h-10 rounded-lg"></span><span class="block h-10 rounded-lg"></span></div>
    </div>
  </div>

  <div v-else-if="error" class="flex min-h-52 items-center justify-center rounded-lg border border-trip/20 bg-trip-wash px-6 py-10 text-center" role="alert">
    <div class="max-w-lg">
      <div class="mx-auto flex h-10 w-10 items-center justify-center rounded-full bg-white text-lg font-semibold text-trip" aria-hidden="true">!</div>
      <h2 class="mt-3 text-base font-semibold text-ink">内容加载失败</h2>
      <p class="mt-1 break-words text-sm leading-6 text-soft">{{ error }}</p>
      <button class="btn mt-4" @click="$emit('retry')">重新加载</button>
    </div>
  </div>

  <div v-else-if="empty" class="flex min-h-52 items-center justify-center rounded-lg border border-dashed border-line bg-white px-6 py-10 text-center">
    <div class="max-w-lg">
      <div class="mx-auto h-10 w-10 rounded-full bg-ghost" aria-hidden="true"></div>
      <h2 class="mt-3 text-base font-semibold text-ink">{{ emptyText }}</h2>
      <p v-if="emptyHint" class="mt-1 text-sm leading-6 text-soft">{{ emptyHint }}</p>
      <div v-if="$slots.empty" class="mt-4"><slot name="empty" /></div>
    </div>
  </div>

  <slot v-else />
</template>

<style scoped>
.state-skeleton span { background: linear-gradient(100deg, #eef2f7 20%, #f8fafd 42%, #eef2f7 64%); background-size: 220% 100%; animation: skeleton-flow 1.35s ease-in-out infinite; }
@keyframes skeleton-flow { to { background-position-x: -220%; } }
@media (prefers-reduced-motion: reduce) { .state-skeleton span { animation: none; } }
</style>

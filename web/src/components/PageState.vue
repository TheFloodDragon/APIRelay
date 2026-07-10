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
  <div v-if="loading" class="flex min-h-52 items-center justify-center rounded-lg border border-line bg-white" role="status" aria-live="polite">
    <div class="text-center">
      <span class="mx-auto block h-7 w-7 animate-spin rounded-full border-2 border-line border-t-blue" aria-hidden="true"></span>
      <p class="mt-3 text-sm text-soft">正在加载，请稍候</p>
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

<script setup>
defineProps({
  label: { type: String, default: '数据工具栏' },
  sticky: { type: Boolean, default: false },
})
</script>

<template>
  <section class="data-toolbar" :class="{ 'data-toolbar-sticky': sticky }" :aria-label="label">
    <div class="data-toolbar-primary"><slot /></div>
    <div v-if="$slots.actions" class="data-toolbar-actions"><slot name="actions" /></div>
  </section>
</template>

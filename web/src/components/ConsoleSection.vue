<script setup>
const props = defineProps({
  title: { type: String, required: true },
  description: { type: String, default: '' },
  eyebrow: { type: String, default: '' },
  flush: { type: Boolean, default: false },
  as: { type: String, default: 'section' },
})
</script>

<template>
  <component :is="props.as" class="console-section">
    <header class="console-section-header">
      <div class="min-w-0">
        <p v-if="eyebrow" class="console-section-eyebrow">{{ eyebrow }}</p>
        <h2 class="console-section-title">{{ title }}</h2>
        <p v-if="description" class="console-section-description">{{ description }}</p>
      </div>
      <div v-if="$slots.actions" class="console-section-actions"><slot name="actions" /></div>
    </header>
    <div class="console-section-body" :class="{ 'console-section-body-flush': flush }"><slot /></div>
  </component>
</template>

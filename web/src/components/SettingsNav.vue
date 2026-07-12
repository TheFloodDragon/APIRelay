<script setup>
import ConsoleIcon from './ConsoleIcon.vue'

const props = defineProps({
  tabs: { type: Array, default: () => [] },
  activeTab: { type: String, default: '' },
})
const emit = defineEmits(['select'])

function onKeydown(event, index) {
  if (!['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  let next = index
  if (event.key === 'ArrowRight' || event.key === 'ArrowDown') next = (index + 1) % props.tabs.length
  if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') next = (index - 1 + props.tabs.length) % props.tabs.length
  if (event.key === 'Home') next = 0
  if (event.key === 'End') next = props.tabs.length - 1
  const tab = props.tabs[next]
  if (!tab) return
  emit('select', tab.id)
  requestAnimationFrame(() => document.getElementById(`settings-tab-${tab.id}`)?.focus())
}
</script>

<template>
  <nav class="settings-nav" aria-label="设置分类">
    <div class="settings-tablist" role="tablist" aria-label="设置分类" aria-orientation="vertical">
      <button
        v-for="(tab, index) in tabs"
        :id="`settings-tab-${tab.id}`"
        :key="tab.id"
        class="settings-tab"
        :class="{ 'settings-tab-active': activeTab === tab.id }"
        type="button"
        role="tab"
        :aria-selected="activeTab === tab.id"
        :aria-controls="`settings-panel-${tab.id}`"
        :tabindex="activeTab === tab.id ? 0 : -1"
        @click="emit('select', tab.id)"
        @keydown="onKeydown($event, index)"
      >
        <ConsoleIcon :name="tab.icon || 'settings'" class="settings-tab-icon" />
        <span class="min-w-0"><strong>{{ tab.label }}</strong><small v-if="tab.description">{{ tab.description }}</small></span>
      </button>
    </div>
  </nav>
</template>

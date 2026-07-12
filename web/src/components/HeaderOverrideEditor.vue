<script setup>
import { computed, watch } from 'vue'
import ConsoleIcon from './ConsoleIcon.vue'
import InlineNotice from './InlineNotice.vue'

const props = defineProps({
  modelValue: { type: String, default: '' },
  disabled: { type: Boolean, default: false },
})

const emit = defineEmits(['update:modelValue', 'validation'])

const dangerousHeaders = [
  'Authorization',
  'X-Api-Key',
  'Anthropic-Version',
  'Content-Length',
  'Host',
  'Connection',
  'Transfer-Encoding',
  'Content-Type',
]
const dangerousNames = new Set(dangerousHeaders.map((name) => name.toLowerCase()))

const validation = computed(() => {
  const source = props.modelValue || ''
  if (!source.trim()) return { valid: true, error: '', allowedCount: 0, ignored: [] }

  let parsed
  try {
    parsed = JSON.parse(source)
  } catch (error) {
    return { valid: false, error: `JSON 格式有误：${error.message}`, allowedCount: 0, ignored: [] }
  }

  if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
    return { valid: false, error: '请求头必须填写为 JSON 对象。', allowedCount: 0, ignored: [] }
  }

  for (const [name, value] of Object.entries(parsed)) {
    if (typeof value !== 'string') {
      return { valid: false, error: `请求头“${name}”的值必须是字符串。`, allowedCount: 0, ignored: [] }
    }
  }

  const names = Object.keys(parsed)
  const ignored = names
    .map((name) => name.trim())
    .filter((name) => !name || dangerousNames.has(name.toLowerCase()))
  return { valid: true, error: '', allowedCount: names.length - ignored.length, ignored }
})

watch(validation, (value) => emit('validation', value), { immediate: true })
</script>

<template>
  <div class="override-editor">
    <div class="flex min-w-0 items-start justify-between gap-3">
      <div class="min-w-0">
        <label class="flex items-center gap-2 text-sm font-semibold text-ink" for="header-override">
          <ConsoleIcon name="key" class="h-4 w-4 text-blue-grid" />
          自定义请求头
        </label>
        <p class="mt-1 text-[11px] leading-4 text-soft">可留空；对象中的每个值都必须是字符串。</p>
      </div>
      <span class="chip shrink-0" :class="validation.valid ? 'chip-run' : 'chip-trip'">
        {{ validation.valid ? `${validation.allowedCount} 个生效` : '需要修正' }}
      </span>
    </div>

    <textarea
      id="header-override"
      :value="modelValue"
      rows="9"
      class="input input-mono mt-3 resize-y text-[12px]"
      :class="validation.valid ? '' : 'border-trip'"
      :disabled="disabled"
      placeholder='{"X-Trace-Source":"api-relay"}'
      aria-describedby="header-override-help header-override-error"
      @input="emit('update:modelValue', $event.target.value)"
    ></textarea>
    <p id="header-override-help" class="mt-2 text-[11px] leading-4 text-soft">探测、单测和正式转发均会使用允许的请求头。</p>
    <p v-if="validation.error" id="header-override-error" class="mt-2 text-xs text-trip" role="alert">{{ validation.error }}</p>

    <InlineNotice class="mt-3" :tone="validation.ignored.length ? 'warning' : 'info'" title="受保护请求头">
      <span class="break-words font-mono text-[10px]">{{ dangerousHeaders.join(' · ') }}</span>
      <p v-if="validation.ignored.length" class="mt-1 text-trip">当前将忽略：{{ validation.ignored.map((name) => name || '空名称').join('、') }}</p>
    </InlineNotice>
  </div>
</template>

<style scoped>
.override-editor { min-width: 0; }
</style>

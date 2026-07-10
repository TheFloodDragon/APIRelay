<script setup>
import { computed, watch } from 'vue'

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
  const allowedCount = names.length - ignored.length
  return { valid: true, error: '', allowedCount, ignored }
})

watch(validation, (value) => emit('validation', value), { immediate: true })
</script>

<template>
  <div class="space-y-2">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <label class="field-label" for="header-override">自定义请求头</label>
      <span class="chip" :class="validation.valid ? 'chip-run' : 'chip-trip'">
        {{ validation.valid ? `允许 ${validation.allowedCount} 个` : '需要修正' }}
      </span>
    </div>
    <textarea
      id="header-override"
      :value="modelValue"
      rows="7"
      class="input input-mono text-[12px]"
      :class="validation.valid ? '' : 'border-trip'"
      :disabled="disabled"
      placeholder='{"X-Trace-Source":"api-relay"}'
      aria-describedby="header-override-help header-override-error"
      @input="emit('update:modelValue', $event.target.value)"
    ></textarea>
    <p id="header-override-help" class="text-[12px] text-soft">
      可留空。填写时必须是 JSON 对象，且每个值都必须是字符串。
    </p>
    <p v-if="validation.error" id="header-override-error" class="text-[12px] text-trip" role="alert">
      {{ validation.error }}
    </p>
    <div class="border border-line bg-panel px-3 py-2 text-[12px] text-soft">
      <div class="font-medium text-ink">以下危险请求头始终会被忽略</div>
      <div class="mt-1 break-words font-mono text-[11px]">{{ dangerousHeaders.join(' · ') }}</div>
      <div v-if="validation.ignored.length" class="mt-1 text-trip">
        当前输入中将忽略：{{ validation.ignored.map((name) => name || '空名称').join('、') }}
      </div>
    </div>
  </div>
</template>

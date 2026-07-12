<script setup>
import { computed, watch } from 'vue'
import ConsoleIcon from './ConsoleIcon.vue'
import InlineNotice from './InlineNotice.vue'

const props = defineProps({
  modelValue: { type: String, default: '' },
  disabled: { type: Boolean, default: false },
})

const emit = defineEmits(['update:modelValue', 'validation'])

// 顶层受保护字段：与后端 model.SafeBodyOverride 保持一致，覆盖会破坏流式契约。
const protectedTopLevel = ['stream']
const protectedNames = new Set(protectedTopLevel)

const validation = computed(() => {
  const source = props.modelValue || ''
  if (!source.trim()) return { valid: true, error: '', keyCount: 0, ignored: [] }

  let parsed
  try {
    parsed = JSON.parse(source)
  } catch (error) {
    return { valid: false, error: `JSON 格式有误：${error.message}`, keyCount: 0, ignored: [] }
  }

  if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
    return { valid: false, error: '请求体复写必须填写为 JSON 对象。', keyCount: 0, ignored: [] }
  }

  const names = Object.keys(parsed)
  const ignored = names.filter((name) => protectedNames.has(name))
  return { valid: true, error: '', keyCount: names.length - ignored.length, ignored }
})

watch(validation, (value) => emit('validation', value), { immediate: true })
</script>

<template>
  <div class="override-editor">
    <div class="flex min-w-0 items-start justify-between gap-3">
      <div class="min-w-0">
        <label class="flex items-center gap-2 text-sm font-semibold text-ink" for="body-override">
          <ConsoleIcon name="command" class="h-4 w-4 text-blue-grid" />
          请求体复写
        </label>
        <p class="mt-1 text-[11px] leading-4 text-soft">协议转换后深合并；数组、标量和 null 整体覆盖。</p>
      </div>
      <span class="chip shrink-0" :class="validation.valid ? 'chip-run' : 'chip-trip'">
        {{ validation.valid ? `${validation.keyCount} 个字段` : '需要修正' }}
      </span>
    </div>

    <textarea
      id="body-override"
      :value="modelValue"
      rows="9"
      class="input input-mono mt-3 resize-y text-[12px]"
      :class="validation.valid ? '' : 'border-trip'"
      :disabled="disabled"
      placeholder='{"reasoning":{"effort":"high"}}'
      aria-describedby="body-override-help body-override-error"
      @input="emit('update:modelValue', $event.target.value)"
    ></textarea>
    <p id="body-override-help" class="mt-2 text-[11px] leading-4 text-soft">对象递归合并，留空则保持协议转换后的请求体不变。</p>
    <p v-if="validation.error" id="body-override-error" class="mt-2 text-xs text-trip" role="alert">{{ validation.error }}</p>

    <InlineNotice class="mt-3" :tone="validation.ignored.length ? 'warning' : 'info'" title="受保护顶层字段">
      <span class="font-mono text-[10px]">{{ protectedTopLevel.join(' · ') }}</span>
      <p v-if="validation.ignored.length" class="mt-1 text-trip">当前将忽略：{{ validation.ignored.join('、') }}</p>
    </InlineNotice>
  </div>
</template>

<style scoped>
.override-editor { min-width: 0; }
</style>

<script setup>
import { computed, watch } from 'vue'

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
  <div class="space-y-2">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <label class="field-label" for="body-override">请求体复写</label>
      <span class="chip" :class="validation.valid ? 'chip-run' : 'chip-trip'">
        {{ validation.valid ? `合并 ${validation.keyCount} 个字段` : '需要修正' }}
      </span>
    </div>
    <textarea
      id="body-override"
      :value="modelValue"
      rows="7"
      class="input input-mono text-[12px]"
      :class="validation.valid ? '' : 'border-trip'"
      :disabled="disabled"
      placeholder='{"reasoning":{"effort":"high"}}'
      aria-describedby="body-override-help body-override-error"
      @input="emit('update:modelValue', $event.target.value)"
    ></textarea>
    <p id="body-override-help" class="text-[12px] text-soft">
      可留空。在协议转换后、发往上游前深合并进请求体：对象递归合并，数组与标量整体替换，null 按普通值覆盖。
    </p>
    <p v-if="validation.error" id="body-override-error" class="text-[12px] text-trip" role="alert">
      {{ validation.error }}
    </p>
    <div class="border border-line bg-panel px-3 py-2 text-[12px] text-soft">
      <div class="font-medium text-ink">以下顶层字段始终会被忽略</div>
      <div class="mt-1 break-words font-mono text-[11px]">{{ protectedTopLevel.join(' · ') }}</div>
      <div v-if="validation.ignored.length" class="mt-1 text-trip">
        当前输入中将忽略：{{ validation.ignored.join('、') }}
      </div>
    </div>
  </div>
</template>

<template>
  <article class="channel-card" :class="{ 'is-disabled': !channel.enabled }">
    <div class="card-topline">
      <button class="drag-handle" type="button" title="拖动调整优先级">
        <el-icon><Rank /></el-icon>
      </button>
      <div class="channel-title">
        <h3>{{ channel.name }}</h3>
        <p>{{ channel.base_url || '未配置 Base URL' }}</p>
      </div>
      <el-switch :model-value="channel.enabled" @change="onToggle" />
    </div>

    <div class="badge-row">
      <el-tag :type="healthMeta.type" effect="light" round>{{ healthMeta.label }}</el-tag>
      <el-tag type="info" effect="plain" round>{{ channel.type || 'openai_compatible' }}</el-tag>
      <el-tag :type="channel.enabled ? 'success' : 'info'" effect="plain" round>
        {{ channel.enabled ? '已启用' : '已停用' }}
      </el-tag>
    </div>

    <div class="channel-metrics">
      <div>
        <span>优先级</span>
        <strong>{{ channel.priority }}</strong>
      </div>
      <div>
        <span>权重</span>
        <strong>{{ channel.weight }}</strong>
      </div>
      <div>
        <span>超时</span>
        <strong>{{ timeoutSeconds }}s</strong>
      </div>
      <div>
        <span>重试</span>
        <strong>{{ channel.max_retries }}</strong>
      </div>
    </div>

    <div class="model-preview">
      <div class="section-label">模型</div>
      <div v-if="visibleModels.length" class="model-tags">
        <el-tag v-for="model in visibleModels" :key="model" effect="plain">{{ model }}</el-tag>
        <el-tag v-if="hiddenModelCount > 0" effect="plain">+{{ hiddenModelCount }}</el-tag>
      </div>
      <p v-else class="empty-hint">暂无模型，点击“获取模型”同步。</p>
    </div>

    <div class="channel-foot">
      <span>最后检查：{{ lastCheckText }}</span>
      <div class="channel-actions">
        <el-button size="small" :icon="Connection" @click="$emit('test', channel)">测试</el-button>
        <el-button size="small" :icon="Refresh" @click="$emit('fetch-models', channel)">获取模型</el-button>
        <el-button size="small" :icon="EditPen" @click="$emit('edit', channel)">编辑</el-button>
        <el-button size="small" type="danger" :icon="Delete" @click="$emit('delete', channel)">删除</el-button>
      </div>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Connection, Delete, EditPen, Rank, Refresh } from '@element-plus/icons-vue'
import type { Channel } from '@/api/channels'

const props = defineProps<{
  channel: Channel
}>()

const emit = defineEmits<{
  toggle: [channel: Channel, enabled: boolean]
  test: [channel: Channel]
  edit: [channel: Channel]
  delete: [channel: Channel]
  'fetch-models': [channel: Channel]
}>()

type TagType = 'success' | 'danger' | 'info'

const visibleModels = computed(() => (props.channel.models || []).slice(0, 5))
const hiddenModelCount = computed(() => Math.max((props.channel.models?.length || 0) - visibleModels.value.length, 0))
const timeoutSeconds = computed(() => Math.round((props.channel.timeout || 0) / 1000))
const healthMeta = computed<{ label: string; type: TagType }>(() => {
  switch ((props.channel.health_status || 'unknown').toLowerCase()) {
    case 'healthy':
      return { label: '健康', type: 'success' }
    case 'unhealthy':
      return { label: '异常', type: 'danger' }
    default:
      return { label: '未知', type: 'info' }
  }
})
const lastCheckText = computed(() => formatDate(props.channel.last_check))

function onToggle(value: boolean | string | number) {
  emit('toggle', props.channel, Boolean(value))
}

function formatDate(value?: string | null) {
  if (!value) return '从未检查'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  }).format(date)
}
</script>

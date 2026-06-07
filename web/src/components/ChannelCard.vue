<template>
  <div class="channel-card">
    <span class="drag-handle">⋮⋮</span>
    <div class="channel-info">
      <h3>{{ channel.name }}</h3>
      <p class="channel-meta">
        类型：{{ channel.type }} ｜ 优先级：{{ channel.priority }} ｜ 权重：{{ channel.weight }} ｜ 健康：{{ channel.health_status || 'unknown' }}
      </p>
      <p class="channel-models">
        {{ channel.models?.length ? channel.models.join(', ') : '暂无模型，点击获取模型' }}
      </p>
    </div>
    <div class="channel-actions">
      <el-switch :model-value="channel.enabled" @change="onToggle" />
      <el-button size="small" @click="$emit('test', channel)">测试</el-button>
      <el-button size="small" @click="$emit('fetch-models', channel)">获取模型</el-button>
      <el-button size="small" @click="$emit('edit', channel)">编辑</el-button>
      <el-button size="small" type="danger" @click="$emit('delete', channel)">删除</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
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

function onToggle(value: boolean | string | number) {
  emit('toggle', props.channel, Boolean(value))
}
</script>

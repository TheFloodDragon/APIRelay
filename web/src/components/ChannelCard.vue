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
      <el-tag :type="healthMeta.type" effect="light" round>
        <span class="health-indicator" :class="healthMeta.pulseClass"></span>
        {{ healthMeta.label }}
      </el-tag>
      <el-tag type="info" effect="plain" round>{{ channel.type || 'openai_compatible' }}</el-tag>
      <el-tag :type="channel.enabled ? 'success' : 'info'" effect="plain" round>
        {{ channel.enabled ? '已启用' : '已停用' }}
      </el-tag>
    </div>

    <div class="channel-metrics">
      <div class="metric-item">
        <span>优先级</span>
        <strong>{{ channel.priority }}</strong>
      </div>
      <div class="metric-item">
        <span>权重</span>
        <strong>{{ channel.weight }}</strong>
      </div>
      <div class="metric-item">
        <span>超时</span>
        <strong>{{ timeoutSeconds }}s</strong>
      </div>
      <div class="metric-item">
        <span>重试</span>
        <strong>{{ channel.max_retries }}</strong>
      </div>
    </div>

    <div class="model-preview">
      <div class="model-preview-head">
        <div class="section-label">上游模型 ({{ totalModels }})</div>
        <el-button v-if="totalModels" text size="small" class="view-models-btn" @click.stop="openModelDialog">
          查看全部
        </el-button>
      </div>
      <div v-if="visibleModels.length" class="model-tags">
        <el-tag v-for="model in visibleModels" :key="model" effect="plain" size="small">
          {{ model }}
        </el-tag>
        <el-tag
          v-if="hiddenModelCount > 0"
          effect="plain"
          size="small"
          type="info"
          class="more-model-tag"
          @click.stop="openModelDialog"
        >
          +{{ hiddenModelCount }}，查看全部
        </el-tag>
      </div>
      <p v-else class="empty-hint">暂无模型,点击"获取模型"同步。</p>
    </div>

    <div class="channel-foot">
      <span class="last-check">
        <el-icon style="margin-right: 4px"><Clock /></el-icon>
        {{ lastCheckText }}
      </span>
      <div class="channel-actions">
        <el-button size="small" :icon="Connection" @click="$emit('test', channel)">测试</el-button>
        <el-button size="small" :icon="Refresh" @click="$emit('fetch-models', channel)">
          获取模型
        </el-button>
        <el-button size="small" :icon="EditPen" @click="$emit('edit', channel)">编辑</el-button>
        <el-button size="small" type="danger" :icon="Delete" @click="$emit('delete', channel)">
          删除
        </el-button>
      </div>
    </div>
  </article>

  <el-dialog
    v-model="modelDialogVisible"
    :title="`${channel.name} 的上游模型`"
    width="620px"
    class="model-list-dialog"
  >
    <div class="model-dialog-summary">
      <span>共 {{ totalModels }} 个模型</span>
      <small>这些名称是该渠道上游实际支持的模型，可在模型列表页调整对外调用名称。</small>
    </div>
    <div class="full-model-list">
      <el-tag v-for="model in allModels" :key="model" effect="plain" size="large">
        {{ model }}
      </el-tag>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { Connection, Delete, EditPen, Rank, Refresh, Clock } from '@element-plus/icons-vue'
import type { Channel } from '@/api/channels'

const props = defineProps<{ channel: Channel }>()
const modelDialogVisible = ref(false)
const emit = defineEmits<{
  toggle: [channel: Channel, enabled: boolean]
  test: [channel: Channel]
  edit: [channel: Channel]
  delete: [channel: Channel]
  'fetch-models': [channel: Channel]
}>()

const healthMeta = computed(() => {
  const status = props.channel.health_status
  if (status === 'healthy')
    return { label: '健康', type: 'success' as const, pulseClass: 'pulse-success' }
  if (status === 'unhealthy')
    return { label: '异常', type: 'danger' as const, pulseClass: 'pulse-danger' }
  return { label: '未知', type: 'warning' as const, pulseClass: 'pulse-warning' }
})

const timeoutSeconds = computed(() => Math.round((props.channel.timeout || 60000) / 1000))
const allModels = computed(() => props.channel.models || [])
const totalModels = computed(() => allModels.value.length)
const visibleModels = computed(() => allModels.value.slice(0, 8))
const hiddenModelCount = computed(() => Math.max(0, totalModels.value - visibleModels.value.length))

const lastCheckText = computed(() => {
  if (!props.channel.last_check) return '从未检查'
  const date = new Date(props.channel.last_check)
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  const minutes = Math.floor(diff / 60000)
  if (minutes < 1) return '刚刚'
  if (minutes < 60) return `${minutes} 分钟前`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} 小时前`
  const days = Math.floor(hours / 24)
  return `${days} 天前`
})

function openModelDialog() {
  if (totalModels.value > 0) {
    modelDialogVisible.value = true
  }
}

function onToggle(value: boolean | string | number) {
  emit('toggle', props.channel, Boolean(value))
}
</script>

<style scoped>
.model-preview-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 8px;
}

.model-preview-head .section-label {
  margin-bottom: 0;
}

.view-models-btn {
  height: auto;
  padding: 0 2px;
  font-size: 12px;
}

.model-tags :deep(.el-tag__content) {
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.more-model-tag {
  cursor: pointer;
  border-style: dashed;
}

.model-dialog-summary {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
  color: var(--muted);
}

.model-dialog-summary span {
  color: var(--text);
  font-weight: 700;
}

.model-dialog-summary small {
  line-height: 1.6;
  text-align: right;
}

.full-model-list {
  display: flex;
  max-height: 52vh;
  overflow: auto;
  gap: 10px;
  flex-wrap: wrap;
  padding: 2px 4px 4px 0;
}

.full-model-list :deep(.el-tag__content) {
  max-width: 260px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.health-indicator {
  display: inline-block;
  width: 8px;
  height: 8px;
  margin-right: 6px;
  border-radius: 999px;
  background: currentColor;
}

.pulse-success {
  animation: pulse-success 2s ease-in-out infinite;
}

.pulse-danger {
  animation: pulse-danger 2s ease-in-out infinite;
}

.pulse-warning {
  animation: pulse-warning 2s ease-in-out infinite;
}

@keyframes pulse-success {
  0%,
  100% {
    opacity: 1;
    box-shadow: 0 0 0 0 rgba(18, 183, 106, 0.7);
  }
  50% {
    opacity: 0.8;
    box-shadow: 0 0 0 4px rgba(18, 183, 106, 0);
  }
}

@keyframes pulse-danger {
  0%,
  100% {
    opacity: 1;
    box-shadow: 0 0 0 0 rgba(240, 68, 56, 0.7);
  }
  50% {
    opacity: 0.8;
    box-shadow: 0 0 0 4px rgba(240, 68, 56, 0);
  }
}

@keyframes pulse-warning {
  0%,
  100% {
    opacity: 1;
    box-shadow: 0 0 0 0 rgba(247, 144, 9, 0.7);
  }
  50% {
    opacity: 0.8;
    box-shadow: 0 0 0 4px rgba(247, 144, 9, 0);
  }
}

.metric-item {
  transition: var(--transition-fast);
}

.metric-item:hover {
  transform: translateY(-2px);
  background: #ffffff;
}

.last-check {
  display: flex;
  align-items: center;
}
</style>

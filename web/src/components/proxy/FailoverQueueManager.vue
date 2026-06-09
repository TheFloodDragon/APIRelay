<template>
  <el-card class="panel-card" shadow="never">
    <template #header>
      <div class="panel-header">
        <div>
          <span>全局故障转移队列</span>
          <small>拖拽调整所有入口共享的渠道尝试顺序。</small>
        </div>
        <div class="queue-actions">
          <el-button :loading="saving" @click="resetOrder">按优先级重置</el-button>
          <el-button type="primary" :loading="saving" @click="saveQueue">保存队列</el-button>
        </div>
      </div>
    </template>

    <div v-loading="loading" class="queue-wrap">
      <draggable
        v-if="localQueue.length"
        :list="localQueue"
        item-key="id"
        handle=".queue-drag-handle"
        ghost-class="drag-ghost"
        chosen-class="drag-chosen"
        drag-class="drag-active"
        :animation="180"
        :disabled="loading || saving"
        :force-fallback="true"
      >
        <template #item="{ element, index }">
          <div class="queue-item">
            <button class="queue-drag-handle" type="button" title="拖动排序">
              <el-icon><Rank /></el-icon>
            </button>
            <div class="queue-index">#{{ index + 1 }}</div>
            <div class="queue-main">
              <strong>{{ element.name }}</strong>
              <span>{{ element.type }} · {{ element.base_url || '未配置 Base URL' }}</span>
            </div>
            <div class="queue-meta">
              <el-tag :type="element.enabled ? 'success' : 'info'" effect="plain">
                {{ element.enabled ? '启用' : '停用' }}
              </el-tag>
              <el-tag effect="plain">优先级 {{ element.priority }}</el-tag>
              <el-tag effect="plain">权重 {{ element.weight }}</el-tag>
            </div>
            <el-button text type="danger" @click="removeChannel(element.id)">移除</el-button>
          </div>
        </template>
      </draggable>

      <el-empty v-else description="暂无故障转移队列，请从下方选择渠道加入" :image-size="88" />

      <div class="queue-add-area">
        <el-select v-model="selectedChannelID" filterable clearable placeholder="选择要加入队列的渠道" class="queue-select">
          <el-option
            v-for="channel in availableChannels"
            :key="channel.id"
            :label="`${channel.name} (${channel.type})`"
            :value="channel.id"
          />
        </el-select>
        <el-button type="primary" plain :disabled="!selectedChannelID" @click="addSelectedChannel">加入队列</el-button>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import draggable from 'vuedraggable'
import { Rank } from '@element-plus/icons-vue'
import type { Channel } from '@/api/channels'
import type { FailoverQueueItem } from '@/api/proxy'

const props = defineProps<{
  channels: Channel[]
  queue: FailoverQueueItem[]
  loading?: boolean
  saving?: boolean
}>()

const emit = defineEmits<{
  save: [channelIDs: number[]]
}>()

type QueueChannel = Pick<Channel, 'id' | 'name' | 'type' | 'base_url' | 'priority' | 'weight' | 'enabled'>

const localQueue = ref<QueueChannel[]>([])
const selectedChannelID = ref<number | null>(null)

const channelsByID = computed(() => new Map(props.channels.map((channel) => [channel.id, channel])))
const queuedIDs = computed(() => new Set(localQueue.value.map((channel) => channel.id)))
const availableChannels = computed(() =>
  props.channels
    .filter((channel) => !queuedIDs.value.has(channel.id))
    .sort((a, b) => b.priority - a.priority || a.id - b.id)
)

watch(
  () => [props.queue, props.channels] as const,
  () => syncLocalQueue(),
  { immediate: true, deep: true }
)

function syncLocalQueue() {
  const next: QueueChannel[] = []
  const seen = new Set<number>()

  for (const item of props.queue || []) {
    const channel = item.channel || channelsByID.value.get(item.channel_id)
    if (!channel || seen.has(channel.id)) continue
    next.push(toQueueChannel(channel))
    seen.add(channel.id)
  }

  for (const channel of props.channels) {
    if (!seen.has(channel.id)) {
      next.push(toQueueChannel(channel))
      seen.add(channel.id)
    }
  }

  localQueue.value = next
}

function toQueueChannel(channel: Channel): QueueChannel {
  return {
    id: channel.id,
    name: channel.name,
    type: channel.type,
    base_url: channel.base_url,
    priority: Number(channel.priority || 0),
    weight: Number(channel.weight || 1),
    enabled: Boolean(channel.enabled)
  }
}

function addSelectedChannel() {
  if (!selectedChannelID.value) return
  const channel = channelsByID.value.get(selectedChannelID.value)
  if (!channel || queuedIDs.value.has(channel.id)) return
  localQueue.value.push(toQueueChannel(channel))
  selectedChannelID.value = null
}

function removeChannel(channelID: number) {
  localQueue.value = localQueue.value.filter((channel) => channel.id !== channelID)
}

function resetOrder() {
  localQueue.value = props.channels
    .slice()
    .sort((a, b) => b.priority - a.priority || a.id - b.id)
    .map(toQueueChannel)
}

function saveQueue() {
  emit('save', localQueue.value.map((channel) => channel.id))
}
</script>

<style scoped>
.panel-header > div:first-child {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.panel-header small {
  color: var(--muted);
  font-size: 12px;
  font-weight: 400;
}

.queue-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.queue-wrap {
  min-height: 160px;
}

.queue-item {
  display: grid;
  grid-template-columns: auto auto minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 14px;
  margin-bottom: 10px;
  padding: 14px;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: #ffffff;
  box-shadow: var(--shadow-subtle);
  transition: var(--transition-fast);
}

.queue-item:hover {
  transform: translateY(-1px);
  border-color: rgba(37, 99, 235, 0.22);
}

.queue-drag-handle {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  color: var(--muted);
  cursor: grab;
  border: 0;
  border-radius: 10px;
  background: #f8fafc;
}

.queue-drag-handle:active {
  cursor: grabbing;
}

.queue-index {
  color: var(--primary);
  font-size: 13px;
  font-weight: 800;
}

.queue-main {
  min-width: 0;
}

.queue-main strong,
.queue-main span {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.queue-main span {
  margin-top: 4px;
  color: var(--muted);
  font-size: 12px;
}

.queue-meta {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.queue-add-area {
  display: flex;
  gap: 10px;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--border-light);
}

.queue-select {
  flex: 1;
}

@media (max-width: 980px) {
  .queue-item {
    grid-template-columns: auto auto minmax(0, 1fr);
  }

  .queue-meta,
  .queue-item .el-button {
    grid-column: 3;
    justify-content: flex-start;
  }
}

@media (max-width: 720px) {
  .queue-add-area,
  .queue-actions {
    flex-direction: column;
  }
}
</style>

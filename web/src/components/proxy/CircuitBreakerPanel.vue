<template>
  <el-card class="panel-card" shadow="never">
    <template #header>
      <div class="panel-header">
        <div>
          <span>渠道熔断状态</span>
          <small>按渠道 ID 管理，不包含 App 或协议维度。</small>
        </div>
        <el-button :loading="loading" @click="$emit('refresh')">刷新</el-button>
      </div>
    </template>

    <el-table v-loading="loading" :data="circuits" class="admin-table" empty-text="暂无熔断状态">
      <el-table-column label="渠道" min-width="190" show-overflow-tooltip>
        <template #default="{ row }">
          <div class="channel-cell">
            <strong>{{ row.channel?.name || `渠道 #${row.channel_id}` }}</strong>
            <small>{{ row.channel?.type || '-' }}</small>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="熔断状态" width="130" align="center">
        <template #default="{ row }">
          <el-tag :type="stateMeta(row.circuit.state).type" effect="light" round>
            {{ stateMeta(row.circuit.state).label }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="健康状态" width="130" align="center">
        <template #default="{ row }">
          <el-tag :type="row.health?.is_healthy ? 'success' : 'danger'" effect="plain" round>
            {{ row.health?.is_healthy ? '健康' : '异常' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="失败/恢复" width="130" align="center">
        <template #default="{ row }">
          <span>{{ row.circuit.consecutive_failures || row.health?.consecutive_failures || 0 }} / {{ row.circuit.consecutive_successes || 0 }}</span>
        </template>
      </el-table-column>
      <el-table-column label="打开至" min-width="180">
        <template #default="{ row }">
          {{ row.circuit.opened_until ? formatDate(row.circuit.opened_until) : '-' }}
        </template>
      </el-table-column>
      <el-table-column label="最近错误" min-width="220" show-overflow-tooltip>
        <template #default="{ row }">{{ row.health?.last_error || '-' }}</template>
      </el-table-column>
      <el-table-column label="操作" width="110" fixed="right" align="center">
        <template #default="{ row }">
          <el-button size="small" type="primary" text :loading="resettingID === row.channel_id" @click="$emit('reset', row.channel_id)">
            重置
          </el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>

<script setup lang="ts">
import type { CircuitState, CircuitStatus } from '@/api/proxy'

defineProps<{
  circuits: CircuitStatus[]
  loading?: boolean
  resettingID?: number | null
}>()

defineEmits<{
  refresh: []
  reset: [channelID: number]
}>()

type TagType = 'success' | 'warning' | 'danger' | 'info'

function stateMeta(state: CircuitState): { label: string; type: TagType } {
  if (state === 'closed') return { label: '关闭', type: 'success' }
  if (state === 'open') return { label: '打开', type: 'danger' }
  if (state === 'half_open') return { label: '半开', type: 'warning' }
  return { label: state || '未知', type: 'info' }
}

function formatDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
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

.channel-cell strong,
.channel-cell small {
  display: block;
}

.channel-cell small {
  margin-top: 4px;
  color: var(--muted);
  font-size: 12px;
}
</style>

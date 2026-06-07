<template>
  <section class="page-hero">
    <div>
      <p class="eyebrow">Model Catalog</p>
      <h1>模型列表</h1>
      <p>查看已同步到管理台的模型、所属渠道以及可用状态。</p>
    </div>
    <div class="page-actions">
      <el-button :icon="Refresh" :loading="loading" @click="loadModels">刷新</el-button>
    </div>
  </section>

  <div class="metric-grid compact">
    <div class="metric-card">
      <span class="metric-label">模型总数</span>
      <strong>{{ models.length }}</strong>
      <small>全部同步模型</small>
    </div>
    <div class="metric-card">
      <span class="metric-label">可用模型</span>
      <strong>{{ enabledCount }}</strong>
      <small>enabled = true</small>
    </div>
    <div class="metric-card">
      <span class="metric-label">覆盖渠道</span>
      <strong>{{ channelCount }}</strong>
      <small>拥有模型记录的渠道</small>
    </div>
  </div>

  <el-card class="table-card" shadow="never">
    <el-table v-loading="loading" :data="models" class="admin-table" empty-text="暂无模型，请先在渠道页获取模型">
      <el-table-column prop="name" label="模型名" min-width="220" show-overflow-tooltip />
      <el-table-column label="渠道" min-width="170" show-overflow-tooltip>
        <template #default="{ row }">{{ row.channel?.name || row.channel_id || '-' }}</template>
      </el-table-column>
      <el-table-column label="渠道类型" min-width="130">
        <template #default="{ row }">{{ row.channel?.type || '-' }}</template>
      </el-table-column>
      <el-table-column label="别名" min-width="140">
        <template #default="{ row }">{{ row.alias || '-' }}</template>
      </el-table-column>
      <el-table-column label="重定向" min-width="160">
        <template #default="{ row }">{{ row.redirect_to || '-' }}</template>
      </el-table-column>
      <el-table-column label="状态" width="110">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'" effect="light" round>
            {{ row.enabled ? '启用' : '停用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="创建时间" width="180">
        <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="110" fixed="right">
        <template #default="{ row }">
          <el-button type="danger" text :icon="Delete" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Refresh } from '@element-plus/icons-vue'
import { deleteModel, getModels, type ModelRecord } from '@/api/models'

const loading = ref(false)
const models = ref<ModelRecord[]>([])

const enabledCount = computed(() => models.value.filter((item) => item.enabled).length)
const channelCount = computed(() => new Set(models.value.map((item) => item.channel_id).filter(Boolean)).size)

onMounted(loadModels)

async function loadModels() {
  loading.value = true
  try {
    const res = await getModels()
    models.value = res.data.data || []
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || '加载模型列表失败')
  } finally {
    loading.value = false
  }
}

async function handleDelete(model: ModelRecord) {
  try {
    await ElMessageBox.confirm(`确定删除模型「${model.name}」吗？`, '删除确认', { type: 'warning' })
  } catch {
    return
  }

  try {
    await deleteModel(model.id)
    ElMessage.success('模型已删除')
    await loadModels()
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || '删除模型失败')
  }
}

function formatDate(value?: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  }).format(date)
}
</script>

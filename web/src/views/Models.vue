<template>
  <section class="page-hero">
    <div>
      <p class="eyebrow">Model Catalog</p>
      <h1>模型列表</h1>
      <p>管理已同步模型的显示名称和可用状态,控制对外调用时的模型路由。</p>
    </div>
    <div class="page-actions">
      <el-input
        v-model="searchKeyword"
        placeholder="搜索模型名称或渠道..."
        :prefix-icon="Search"
        clearable
        style="width: 280px"
      />
      <el-select v-model="filterChannel" placeholder="筛选渠道" clearable style="width: 180px">
        <el-option label="全部渠道" value="" />
        <el-option
          v-for="channel in uniqueChannels"
          :key="channel.id"
          :label="channel.name"
          :value="channel.id"
        />
      </el-select>
      <el-switch
        v-model="showDisabled"
        active-text="显示全部"
        inactive-text="仅启用"
        @change="loadModels"
      />
      <el-button :icon="Refresh" :loading="loading" @click="loadModels">刷新</el-button>
    </div>
  </section>

  <div class="metric-grid compact">
    <div class="metric-card">
      <span class="metric-label">模型总数</span>
      <strong>{{ totalCount }}</strong>
      <small>全部同步模型</small>
    </div>
    <div class="metric-card accent-green">
      <span class="metric-label">可用模型</span>
      <strong>{{ enabledCount }}</strong>
      <small>已启用并可调用</small>
    </div>
    <div class="metric-card accent-red">
      <span class="metric-label">隐藏模型</span>
      <strong>{{ disabledCount }}</strong>
      <small>已禁用不可调用</small>
    </div>
  </div>

  <el-alert type="info" :closable="false" show-icon style="margin-bottom: 20px">
    <template #title>
      <span style="font-size: 14px">
        "调用名称/显示名"是对外调用时使用的模型名；"上游真实模型名"是渠道实际支持的模型。
        关闭"启用"开关后,模型将隐藏且不可被路由调用。双击显示名可快速编辑。
      </span>
    </template>
  </el-alert>

  <el-card class="table-card" shadow="never">
    <el-table
      v-loading="loading"
      :data="paginatedModels"
      class="admin-table enhanced-table"
      empty-text="暂无模型,请先在渠道页获取模型"
      @row-dblclick="openEditDialogFromRow"
    >
      <el-table-column type="selection" width="55" />
      <el-table-column label="调用名称/显示名" min-width="200" sortable>
        <template #default="{ row }">
          <div class="model-name-cell">
            <div>
              <span class="display-name" :class="{ 'is-disabled': !row.enabled }">
                {{ row.display_name || row.name }}
              </span>
              <el-tag v-if="!row.enabled" type="info" size="small" effect="plain" style="margin-left: 8px">
                隐藏
              </el-tag>
            </div>
            <el-button type="primary" text size="small" :icon="Edit" @click="openEditDialog(row)">
              编辑
            </el-button>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="name" label="上游真实模型名" min-width="200" show-overflow-tooltip sortable />
      <el-table-column label="所属渠道" min-width="150" show-overflow-tooltip sortable>
        <template #default="{ row }">
          <span class="channel-badge">{{ row.channel?.name || row.channel_id || '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="渠道类型" min-width="130" sortable>
        <template #default="{ row }">
          <el-tag effect="plain" size="small">{{ row.channel?.type || '-' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="启用状态" width="100" align="center">
        <template #default="{ row }">
          <el-switch
            v-model="row.enabled"
            size="small"
            inline-prompt
            active-text="启"
            inactive-text="隐"
            @change="toggleModel(row)"
          />
        </template>
      </el-table-column>
      <el-table-column label="创建时间" width="180" sortable>
        <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="110" fixed="right" align="center">
        <template #default="{ row }">
          <el-button type="danger" text :icon="Delete" size="small" @click="handleDelete(row)">
            删除
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="table-footer">
      <span>共 {{ filteredModels.length }} 条记录</span>
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :total="filteredModels.length"
        :page-sizes="[20, 50, 100]"
        layout="sizes, prev, pager, next"
        small
      />
    </div>
  </el-card>

  <el-dialog v-model="editDialogVisible" title="编辑模型" width="500px" class="form-dialog">
    <el-form :model="editForm" label-position="top">
      <el-form-item label="上游真实模型名（只读）">
        <el-input v-model="editForm.name" disabled />
      </el-form-item>
      <el-form-item label="调用名称/显示名">
        <el-input v-model="editForm.display_name" placeholder="如 gpt-4o-mini" />
        <small class="form-tip">留空则使用上游真实模型名</small>
      </el-form-item>
      <el-form-item label="启用状态">
        <el-switch v-model="editForm.enabled" active-text="启用" inactive-text="隐藏" />
        <small class="form-tip">关闭后模型将隐藏且不可调用</small>
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="editDialogVisible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="saveModel">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Edit, Refresh, Search } from '@element-plus/icons-vue'
import { deleteModel, getModels, updateModel, type ModelRecord } from '@/api/models'

const loading = ref(false)
const saving = ref(false)
const showDisabled = ref(false)
const searchKeyword = ref('')
const filterChannel = ref<number | ''>('')
const currentPage = ref(1)
const pageSize = ref(20)
const models = ref<ModelRecord[]>([])
const editDialogVisible = ref(false)
const editingModel = ref<ModelRecord | null>(null)

const editForm = reactive({
  name: '',
  display_name: '',
  enabled: true
})

const totalCount = computed(() => models.value.length)
const enabledCount = computed(() => models.value.filter((item) => item.enabled).length)
const disabledCount = computed(() => models.value.filter((item) => !item.enabled).length)

const uniqueChannels = computed(() => {
  const channelMap = new Map()
  models.value.forEach((model) => {
    if (model.channel && !channelMap.has(model.channel.id)) {
      channelMap.set(model.channel.id, model.channel)
    }
  })
  return Array.from(channelMap.values())
})

const filteredModels = computed(() => {
  let result = models.value

  // 筛选启用状态
  if (!showDisabled.value) {
    result = result.filter((item) => item.enabled)
  }

  // 筛选渠道
  if (filterChannel.value !== '') {
    result = result.filter((item) => item.channel_id === filterChannel.value)
  }

  // 搜索关键词
  if (searchKeyword.value) {
    const keyword = searchKeyword.value.toLowerCase()
    result = result.filter(
      (item) =>
        item.name.toLowerCase().includes(keyword) ||
        item.display_name?.toLowerCase().includes(keyword) ||
        item.channel?.name?.toLowerCase().includes(keyword)
    )
  }

  return result
})

const paginatedModels = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  const end = start + pageSize.value
  return filteredModels.value.slice(start, end)
})

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

function openEditDialog(model: ModelRecord) {
  editingModel.value = model
  editForm.name = model.name
  editForm.display_name = model.display_name || model.name
  editForm.enabled = model.enabled
  editDialogVisible.value = true
}

function openEditDialogFromRow(row: ModelRecord) {
  openEditDialog(row)
}

async function saveModel() {
  if (!editingModel.value) return

  saving.value = true
  try {
    await updateModel(editingModel.value.id, {
      display_name: editForm.display_name,
      enabled: editForm.enabled
    })
    ElMessage.success('模型已更新')
    editDialogVisible.value = false
    await loadModels()
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || '更新失败')
  } finally {
    saving.value = false
  }
}

async function toggleModel(model: ModelRecord) {
  try {
    await updateModel(model.id, { enabled: model.enabled })
    ElMessage.success(model.enabled ? '模型已启用' : '模型已隐藏')
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || '更新状态失败')
    // 失败时恢复状态
    model.enabled = !model.enabled
  }
}

async function handleDelete(model: ModelRecord) {
  try {
    await ElMessageBox.confirm(
      `确定删除模型「${model.display_name || model.name}」吗?此操作不可恢复,建议改用"隐藏"功能。`,
      '删除确认',
      { type: 'warning' }
    )
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

<style scoped>
.model-name-cell {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.display-name {
  font-weight: 500;
  color: var(--text);
  transition: var(--transition-fast);
}

.display-name.is-disabled {
  color: var(--muted);
  text-decoration: line-through;
}

.channel-badge {
  padding: 4px 10px;
  border-radius: 8px;
  font-size: 13px;
  background: var(--primary-light);
  color: var(--primary);
}

.enhanced-table {
  --el-table-row-hover-bg-color: var(--primary-light);
}

.enhanced-table :deep(.el-table__row) {
  cursor: pointer;
  transition: var(--transition-fast);
}

.enhanced-table :deep(.el-table__row:hover) {
  transform: scale(1.005);
}

.form-tip {
  display: block;
  margin-top: 4px;
  font-size: 12px;
  color: var(--muted);
}
</style>

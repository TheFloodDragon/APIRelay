<template>
  <section class="page-hero">
    <div>
      <p class="eyebrow">Model Catalog</p>
      <h1>模型列表</h1>
      <p>管理已同步模型的显示名称和可用状态,控制对外调用时的模型路由。</p>
    </div>
    <div class="page-actions toolbar-panel">
      <el-input
        v-model="searchKeyword"
        placeholder="搜索模型名称或渠道..."
        :prefix-icon="Search"
        clearable
        style="width: 280px"
        @input="resetPage"
        @clear="resetPage"
      />
      <el-select v-model="filterChannel" placeholder="筛选渠道" clearable style="width: 180px" @change="resetPage">
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
        @change="handleFilterChange"
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
        关闭"参与路由"后,模型将隐藏且不可被真实请求路由调用；"允许测试"只影响管理台连通性测试。双击显示名可快速编辑。
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
            <el-button type="primary" text size="small" :icon="Edit" @click.stop="openEditDialog(row)">
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
      <el-table-column label="参与路由" width="100" align="center">
        <template #default="{ row }">
          <el-switch
            v-model="row.enabled"
            size="small"
            inline-prompt
            active-text="启"
            inactive-text="隐"
            @click.stop
            @change="toggleModel(row)"
          />
        </template>
      </el-table-column>
      <el-table-column label="允许测试" width="100" align="center">
        <template #default="{ row }">
          <el-switch
            v-model="row.test_enabled"
            size="small"
            inline-prompt
            active-text="测"
            inactive-text="禁"
            @click.stop
            @change="toggleModelTest(row)"
          />
        </template>
      </el-table-column>
      <el-table-column label="创建时间" width="180" sortable>
        <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="170" fixed="right" align="center">
        <template #default="{ row }">
          <el-button type="primary" text :icon="Connection" size="small" @click.stop="openTestDialog(row)">
            测试
          </el-button>
          <el-button type="danger" text :icon="Delete" size="small" @click.stop="handleDelete(row)">
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
      <div class="readonly-field">
        <span>上游真实模型名（只读）</span>
        <strong>{{ editForm.name || '-' }}</strong>
      </div>
      <el-form-item label="调用名称/显示名">
        <el-input v-model="editForm.display_name" placeholder="如 gpt-4o-mini" />
        <small class="form-tip">留空则使用上游真实模型名</small>
      </el-form-item>
      <el-form-item label="参与路由">
        <el-switch v-model="editForm.enabled" active-text="参与路由" inactive-text="隐藏" />
        <small class="form-tip">关闭后模型将隐藏且不可被真实请求路由调用</small>
      </el-form-item>
      <el-form-item label="允许测试">
        <el-switch v-model="editForm.test_enabled" active-text="允许测试" inactive-text="禁止测试" />
        <small class="form-tip">该开关只影响管理台模型测试，不影响真实请求路由</small>
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="editDialogVisible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="saveModel">保存</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="testDialogVisible" title="模型连通性测试" width="680px" class="form-dialog">
    <div v-if="testingModel" class="test-summary">
      <div>
        <span>调用名称</span>
        <strong>{{ testingModel.display_name || testingModel.name }}</strong>
      </div>
      <div>
        <span>上游模型</span>
        <strong>{{ testingModel.name }}</strong>
      </div>
      <div>
        <span>当前渠道</span>
        <strong>{{ testingModel.channel?.name || '-' }}</strong>
      </div>
    </div>

    <el-form :model="testForm" label-position="top">
      <el-form-item label="测试渠道">
        <el-select v-model="testForm.channel_id" placeholder="选择测试渠道" style="width: 100%" :loading="loadingTestChannels">
          <el-option
            v-for="item in testChannels"
            :key="item.channel.id"
            :label="`${item.channel.name} · ${item.model_name}`"
            :value="item.channel.id"
          >
            <span>{{ item.channel.name }} · {{ item.model_name }}</span>
            <span style="float: right; color: var(--muted); font-size: 12px">
              {{ item.route_enabled ? '可路由' : '不可路由' }} / {{ item.channel.enabled ? '渠道启用' : '渠道关闭' }}
            </span>
          </el-option>
        </el-select>
      </el-form-item>
      <el-form-item label="Prompt">
        <el-input v-model="testForm.prompt" type="textarea" :rows="3" />
      </el-form-item>
      <el-collapse>
        <el-collapse-item title="高级选项" name="advanced">
          <div class="advanced-grid">
            <el-form-item label="超时 ms">
              <el-input-number v-model="testForm.timeout_ms" :min="1000" :max="600000" :step="1000" style="width: 100%" />
            </el-form-item>
            <el-form-item label="最大输出 tokens">
              <el-input-number v-model="testForm.max_output_tokens" :min="1" :max="8192" style="width: 100%" />
            </el-form-item>
            <el-form-item label="temperature">
              <el-input-number v-model="testForm.temperature" :min="0" :max="2" :step="0.1" :precision="1" style="width: 100%" />
            </el-form-item>
          </div>
        </el-collapse-item>
      </el-collapse>
    </el-form>

    <el-result
      v-if="testResult"
      :icon="testResult.ok ? 'success' : 'error'"
      :title="testResult.ok ? '测试成功' : '测试失败'"
      :sub-title="`${testResult.channel_name || '-'} · ${testResult.latency_ms || 0}ms · HTTP ${testResult.status_code || '-'}`"
    >
      <template #extra>
        <el-input
          :model-value="testResult.ok ? testResult.content : testResult.error || testResult.content"
          type="textarea"
          :rows="4"
          readonly
        />
      </template>
    </el-result>

    <template #footer>
      <el-button @click="testDialogVisible = false">关闭</el-button>
      <el-button type="primary" :loading="testing" :disabled="!testForm.channel_id" @click="runModelTest">开始测试</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Connection, Delete, Edit, Refresh, Search } from '@element-plus/icons-vue'
import { useRoute } from 'vue-router'
import {
  deleteModel,
  getModelTestChannels,
  getModels,
  testModel,
  updateModel,
  type ModelRecord,
  type ModelTestChannel,
  type ModelTestResult
} from '@/api/models'
import { getSettings } from '@/api/settings'

const route = useRoute()
const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const loadingTestChannels = ref(false)
const showDisabled = ref(false)
const searchKeyword = ref('')
const filterChannel = ref<number | ''>('')
const currentPage = ref(1)
const pageSize = ref(20)
const models = ref<ModelRecord[]>([])
const editDialogVisible = ref(false)
const testDialogVisible = ref(false)
const editingModel = ref<ModelRecord | null>(null)
const testingModel = ref<ModelRecord | null>(null)
const testChannels = ref<ModelTestChannel[]>([])
const testResult = ref<ModelTestResult | null>(null)
let loadToken = 0

const editForm = reactive({
  name: '',
  display_name: '',
  enabled: true,
  test_enabled: true
})

const testForm = reactive({
  channel_id: undefined as number | undefined,
  prompt: 'Say OK in one short sentence.',
  timeout_ms: 30000,
  max_output_tokens: 32,
  temperature: 0
})

const totalCount = computed(() => models.value.length)
const enabledCount = computed(() => models.value.filter((item) => item.enabled).length)
const disabledCount = computed(() => models.value.filter((item) => !item.enabled).length)

const uniqueChannels = computed(() => {
  const channelMap = new Map<number, NonNullable<ModelRecord['channel']>>()
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
  if (searchKeyword.value.trim()) {
    const keyword = searchKeyword.value.trim().toLowerCase()
    result = result.filter((item) =>
      [item.name, item.display_name, item.channel?.name, item.channel?.type]
        .some((field) => String(field || '').toLowerCase().includes(keyword))
    )
  }

  return result
})

const paginatedModels = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  const end = start + pageSize.value
  return filteredModels.value.slice(start, end)
})

watch(
  () => route.fullPath,
  () => loadModels(),
  { immediate: true }
)

watch([filteredModels, pageSize], () => {
  const maxPage = Math.max(1, Math.ceil(filteredModels.value.length / pageSize.value))
  if (currentPage.value > maxPage) {
    currentPage.value = maxPage
  }
})

async function loadModels() {
  const currentToken = ++loadToken
  loading.value = true
  try {
    const res = await getModels()
    if (currentToken === loadToken) {
      models.value = normalizeModels(res.data.data)
    }
  } catch (error: any) {
    if (currentToken === loadToken) {
      ElMessage.error(error?.response?.data?.error || '加载模型列表失败')
    }
  } finally {
    if (currentToken === loadToken) {
      loading.value = false
    }
  }
}

function normalizeModels(value?: ModelRecord[] | null): ModelRecord[] {
  return (value || []).map((model) => ({
    ...model,
    name: model.name || '',
    display_name: model.display_name || model.name || '',
    enabled: model.enabled ?? true,
    test_enabled: model.test_enabled ?? true
  }))
}

function resetPage() {
  currentPage.value = 1
}

function handleFilterChange() {
  resetPage()
  loadModels()
}

function openEditDialog(model: ModelRecord) {
  editingModel.value = model
  editForm.name = model.name
  editForm.display_name = model.display_name || model.name
  editForm.enabled = model.enabled
  editForm.test_enabled = model.test_enabled
  editDialogVisible.value = true
}

function openEditDialogFromRow(row: ModelRecord) {
  openEditDialog(row)
}

async function saveModel() {
  if (!editingModel.value) return

  const displayName = editForm.display_name.trim() || editForm.name
  saving.value = true
  try {
    await updateModel(editingModel.value.id, {
      display_name: displayName,
      enabled: editForm.enabled,
      test_enabled: editForm.test_enabled
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
  const nextEnabled = model.enabled
  try {
    await updateModel(model.id, { enabled: nextEnabled })
    ElMessage.success(nextEnabled ? '模型已启用' : '模型已隐藏')
    if (!showDisabled.value && !nextEnabled) {
      await loadModels()
    }
  } catch (error: any) {
    model.enabled = !nextEnabled
    ElMessage.error(error?.response?.data?.error || '更新状态失败')
  }
}

async function toggleModelTest(model: ModelRecord) {
  const nextEnabled = model.test_enabled
  try {
    await updateModel(model.id, { test_enabled: nextEnabled })
    ElMessage.success(nextEnabled ? '模型已允许测试' : '模型已禁止测试')
  } catch (error: any) {
    model.test_enabled = !nextEnabled
    ElMessage.error(error?.response?.data?.error || '更新测试状态失败')
  }
}

async function openTestDialog(model: ModelRecord) {
  testingModel.value = model
  testDialogVisible.value = true
  testResult.value = null
  testChannels.value = []
  testForm.channel_id = undefined
  loadingTestChannels.value = true
  try {
    const [settingsRes, channelsRes] = await Promise.all([getSettings(), getModelTestChannels(model.id)])
    const settings = settingsRes.data.data.model_test
    testForm.prompt = settings.default_prompt
    testForm.timeout_ms = settings.timeout_ms
    testForm.max_output_tokens = settings.max_output_tokens
    testForm.temperature = settings.temperature
    testChannels.value = channelsRes.data.data || []
    testForm.channel_id = testChannels.value[0]?.channel?.id
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || '加载测试配置失败')
  } finally {
    loadingTestChannels.value = false
  }
}

async function runModelTest() {
  if (!testingModel.value) return
  testing.value = true
  testResult.value = null
  try {
    const res = await testModel(testingModel.value.id, {
      channel_id: testForm.channel_id,
      prompt: testForm.prompt,
      timeout_ms: testForm.timeout_ms,
      max_output_tokens: testForm.max_output_tokens,
      temperature: testForm.temperature
    })
    testResult.value = res.data.data
    if (testResult.value.ok) {
      ElMessage.success('模型测试成功')
    } else {
      ElMessage.warning('模型测试失败，请查看详情')
    }
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || '模型测试请求失败')
  } finally {
    testing.value = false
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

.readonly-field {
  margin-bottom: 18px;
  padding: 14px 16px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: linear-gradient(135deg, #f8fafc, #ffffff);
}

.readonly-field span,
.readonly-field strong {
  display: block;
}

.readonly-field span {
  margin-bottom: 6px;
  color: var(--muted);
  font-size: 12px;
  font-weight: 700;
}

.readonly-field strong {
  color: var(--text);
  word-break: break-all;
}

.form-tip {
  display: block;
  margin-top: 4px;
  font-size: 12px;
  color: var(--muted);
}

.test-summary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 18px;
}

.test-summary > div {
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: #f8fafc;
}

.test-summary span,
.test-summary strong {
  display: block;
}

.test-summary span {
  margin-bottom: 4px;
  color: var(--muted);
  font-size: 12px;
  font-weight: 700;
}

.test-summary strong {
  color: var(--text);
  word-break: break-all;
}

.advanced-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

@media (max-width: 900px) {
  .test-summary,
  .advanced-grid {
    grid-template-columns: 1fr;
  }
}
</style>

<template>
  <section class="page-hero">
    <div>
      <p class="eyebrow">Model Catalog</p>
      <h1>模型列表</h1>
      <p>参与路由决定真实请求是否能命中模型；允许测试仅控制管理台连通性测试。两者互不影响，方便先验证再开放。</p>
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

  <div class="metric-grid compact model-metrics">
    <div class="metric-card">
      <span class="metric-label">模型总数</span>
      <strong>{{ totalCount }}</strong>
      <small>全部同步模型</small>
    </div>
    <div class="metric-card accent-green">
      <span class="metric-label">参与路由</span>
      <strong>{{ routeEnabledCount }}</strong>
      <small>真实请求可调用</small>
    </div>
    <div class="metric-card accent-purple">
      <span class="metric-label">允许测试</span>
      <strong>{{ testEnabledCount }}</strong>
      <small>管理台可测试</small>
    </div>
    <div class="metric-card accent-red">
      <span class="metric-label">已隐藏</span>
      <strong>{{ disabledCount }}</strong>
      <small>不会参与路由</small>
    </div>
  </div>

  <el-alert class="guide-card" type="info" :closable="false" show-icon>
    <template #title>
      <span>
        “调用名称/显示名”是对外调用时使用的名称，下方小字为渠道真实模型名。关闭“参与路由”会从真实请求中隐藏；关闭“允许测试”只禁止管理台测试。双击行可快速编辑。
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
      <el-table-column type="selection" width="48" />
      <el-table-column label="调用名称 / 显示名" min-width="260" sortable>
        <template #default="{ row }">
          <div class="model-name-cell">
            <div class="model-title-wrap">
              <div class="model-title-line">
                <span class="display-name" :class="{ 'is-disabled': !row.enabled }">
                  {{ row.display_name || row.name }}
                </span>
                <el-tag v-if="!row.enabled" type="info" size="small" effect="plain">隐藏</el-tag>
              </div>
              <small class="real-model-name">真实模型：{{ row.name || '-' }}</small>
            </div>
            <el-button type="primary" text size="small" :icon="Edit" @click.stop="openEditDialog(row)">
              编辑
            </el-button>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="所属渠道" min-width="180" show-overflow-tooltip sortable>
        <template #default="{ row }">
          <div class="channel-cell">
            <span class="channel-badge">{{ row.channel?.name || row.channel_id || '-' }}</span>
            <small>{{ row.channel?.type || '未知协议' }}</small>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="状态" min-width="220" align="center">
        <template #default="{ row }">
          <div class="status-switch-group">
            <label class="compact-switch-card" :class="{ active: row.enabled }" @click.stop>
              <span>参与路由</span>
              <el-switch
                v-model="row.enabled"
                size="small"
                inline-prompt
                active-text="开"
                inactive-text="关"
                @change="toggleModel(row)"
              />
            </label>
            <label class="compact-switch-card" :class="{ active: row.test_enabled }" @click.stop>
              <span>允许测试</span>
              <el-switch
                v-model="row.test_enabled"
                size="small"
                inline-prompt
                active-text="开"
                inactive-text="关"
                @change="toggleModelTest(row)"
              />
            </label>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="创建时间" width="170" sortable>
        <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="190" fixed="right" align="center">
        <template #default="{ row }">
          <div class="row-action-group">
            <el-button type="primary" plain :icon="Connection" size="small" @click.stop="openTestDialog(row)">
              测试
            </el-button>
            <el-button type="danger" plain :icon="Delete" size="small" @click.stop="handleDelete(row)">
              删除
            </el-button>
          </div>
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

  <el-dialog v-model="testDialogVisible" title="模型连通性测试" width="760px" class="form-dialog test-dialog">
    <div v-if="testingModel" class="test-summary">
      <div>
        <span>当前模型</span>
        <strong>{{ testingModel.display_name || testingModel.name }}</strong>
      </div>
      <div>
        <span>默认渠道</span>
        <strong>{{ testingModel.channel?.name || '-' }}</strong>
      </div>
      <div>
        <span>协议类型</span>
        <strong>{{ testingModel.channel?.type || '-' }}</strong>
      </div>
    </div>

    <el-form :model="testForm" label-position="top">
      <el-form-item label="测试渠道">
        <el-select
          v-model="testForm.channel_id"
          placeholder="选择测试渠道"
          style="width: 100%"
          :loading="loadingTestChannels"
        >
          <el-option
            v-for="item in testChannels"
            :key="item.channel.id"
            :label="`${item.channel.name} · ${item.model_name}`"
            :value="item.channel.id"
          >
            <div class="test-channel-option">
              <div>
                <strong>{{ item.channel.name }}</strong>
                <small>{{ item.model_name }}</small>
              </div>
              <div class="option-tags">
                <el-tag :type="item.route_enabled ? 'success' : 'info'" size="small" effect="plain">
                  {{ item.route_enabled ? '可路由' : '不路由' }}
                </el-tag>
                <el-tag :type="item.channel.enabled ? 'success' : 'danger'" size="small" effect="plain">
                  {{ item.channel.enabled ? '渠道启用' : '渠道关闭' }}
                </el-tag>
              </div>
            </div>
          </el-option>
        </el-select>
      </el-form-item>
      <el-form-item label="Prompt">
        <el-input v-model="testForm.prompt" type="textarea" :rows="3" />
      </el-form-item>
      <el-collapse class="advanced-collapse">
        <el-collapse-item title="高级参数" name="advanced">
          <p class="param-note">兼容渠道可能拒绝 temperature 或 max tokens 等参数；系统会自动使用最小请求重试，帮助区分参数兼容问题与连通性问题。</p>
          <div class="advanced-grid">
            <el-form-item label="超时 ms">
              <el-input-number v-model="testForm.timeout_ms" :min="1000" :max="600000" :step="1000" style="width: 100%" />
              <small class="form-tip">等待上游响应的最长时间。</small>
            </el-form-item>
            <el-form-item label="最大输出 tokens">
              <el-input-number v-model="testForm.max_output_tokens" :min="1" :max="8192" style="width: 100%" />
              <small class="form-tip">限制模型最多返回的文本长度。</small>
            </el-form-item>
            <el-form-item label="temperature">
              <el-input-number v-model="testForm.temperature" :min="0" :max="2" :step="0.1" :precision="1" style="width: 100%" />
              <small class="form-tip">控制输出随机性，0 更稳定，数值越高越发散。</small>
            </el-form-item>
          </div>
        </el-collapse-item>
      </el-collapse>
    </el-form>

    <div v-if="testResult" class="test-result-card" :class="testResult.ok ? 'is-success' : 'is-error'">
      <div class="result-header">
        <div>
          <span class="result-kicker">{{ testResult.ok ? 'Success' : 'Failed' }}</span>
          <h3>{{ testResult.ok ? '测试成功' : '测试失败' }}</h3>
        </div>
        <el-tag :type="testResult.ok ? 'success' : 'danger'" effect="dark">
          HTTP {{ testResult.status_code || '-' }}
        </el-tag>
      </div>
      <div class="result-meta">
        <span>{{ testResult.channel_name || '-' }}</span>
        <span>{{ testResult.latency_ms || 0 }}ms</span>
        <span>{{ testResult.resolved_model || testResult.model || '-' }}</span>
      </div>
      <div v-if="!testResult.ok" class="retry-hint">
        上游返回错误。若错误与参数不兼容有关，请关注后端最小请求重试结果；也可调低高级参数后再次测试。
      </div>
      <el-input
        :model-value="testResult.ok ? testResult.content : testResult.error || testResult.content"
        type="textarea"
        :rows="5"
        readonly
      />
    </div>

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
const routeEnabledCount = computed(() => models.value.filter((item) => item.enabled).length)
const testEnabledCount = computed(() => models.value.filter((item) => item.test_enabled).length)
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
.model-metrics {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.model-name-cell {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.model-title-wrap {
  min-width: 0;
}

.model-title-line {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.display-name {
  overflow: hidden;
  color: var(--text);
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: var(--transition-fast);
}

.display-name.is-disabled {
  color: var(--muted);
  text-decoration: line-through;
}

.real-model-name {
  display: block;
  margin-top: 5px;
  overflow: hidden;
  color: var(--muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.channel-cell {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
}

.channel-cell small {
  color: var(--muted);
  font-size: 12px;
}

.channel-badge {
  max-width: 100%;
  overflow: hidden;
  padding: 4px 10px;
  border-radius: 8px;
  background: var(--primary-light);
  color: var(--primary);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status-switch-group {
  display: grid;
  grid-template-columns: repeat(2, minmax(92px, 1fr));
  gap: 8px;
}

.compact-switch-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: #f8fafc;
  transition: var(--transition-fast);
}

.compact-switch-card.active {
  border-color: rgba(18, 183, 106, 0.28);
  background: rgba(220, 252, 231, 0.45);
}

.compact-switch-card span {
  color: var(--muted);
  font-size: 12px;
  font-weight: 800;
  white-space: nowrap;
}

.row-action-group {
  display: inline-flex;
  gap: 8px;
  padding: 5px;
  border: 1px solid var(--border-light);
  border-radius: 14px;
  background: #f8fafc;
}

.row-action-group :deep(.el-button + .el-button) {
  margin-left: 0;
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
  color: var(--muted);
  font-size: 12px;
  line-height: 1.5;
}

.test-summary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 18px;
  padding: 12px;
  border: 1px solid rgba(37, 99, 235, 0.12);
  border-radius: var(--radius-lg);
  background: linear-gradient(135deg, rgba(239, 246, 255, 0.9), rgba(255, 255, 255, 0.86));
}

.test-summary > div {
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: rgba(255, 255, 255, 0.72);
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
  overflow: hidden;
  color: var(--text);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.test-channel-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  width: 100%;
  min-height: 42px;
}

.test-channel-option strong,
.test-channel-option small {
  display: block;
}

.test-channel-option strong {
  line-height: 1.3;
}

.test-channel-option small {
  margin-top: 3px;
  color: var(--muted);
  font-size: 12px;
}

.option-tags {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 6px;
}

.advanced-collapse {
  margin-bottom: 18px;
}

.param-note {
  margin: 0 0 14px;
  padding: 12px 14px;
  border: 1px solid rgba(14, 165, 233, 0.18);
  border-radius: var(--radius-md);
  background: var(--info-light);
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.advanced-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.test-result-card {
  margin-top: 18px;
  padding: 18px;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: linear-gradient(135deg, #ffffff, #f8fafc);
  box-shadow: var(--shadow-subtle);
}

.test-result-card.is-success {
  border-color: rgba(18, 183, 106, 0.26);
  background: linear-gradient(135deg, rgba(220, 252, 231, 0.72), #ffffff 55%);
}

.test-result-card.is-error {
  border-color: rgba(240, 68, 56, 0.24);
  background: linear-gradient(135deg, rgba(254, 228, 226, 0.72), #ffffff 55%);
}

.result-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.result-kicker {
  color: var(--muted);
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.result-header h3 {
  margin: 4px 0 0;
  font-size: 18px;
}

.result-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}

.result-meta span {
  padding: 6px 10px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.78);
  color: var(--muted);
  font-size: 12px;
  font-weight: 700;
}

.retry-hint {
  margin-bottom: 12px;
  padding: 10px 12px;
  border-radius: var(--radius-md);
  background: var(--danger-light);
  color: #b42318;
  font-size: 13px;
  line-height: 1.6;
}

@media (max-width: 1180px) {
  .model-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 900px) {
  .test-summary,
  .advanced-grid,
  .status-switch-group {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 620px) {
  .model-metrics {
    grid-template-columns: 1fr;
  }

  .row-action-group,
  .test-channel-option,
  .result-header {
    align-items: stretch;
    flex-direction: column;
  }

  .option-tags {
    justify-content: flex-start;
  }
}
</style>

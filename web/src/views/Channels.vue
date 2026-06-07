<template>
  <section class="page-hero">
    <div>
      <p class="eyebrow">Channels</p>
      <h1>渠道管理</h1>
      <p>配置多供应商 API 渠道,按优先级和权重完成模型路由与故障切换。</p>
    </div>
    <div class="page-actions">
      <el-button :icon="Refresh" :loading="loading" @click="loadChannels">刷新</el-button>
      <el-button type="primary" :icon="Plus" @click="openCreateDialog">添加渠道</el-button>
    </div>
  </section>

  <div class="metric-grid compact">
    <div class="metric-card">
      <span class="metric-label">渠道总数</span>
      <strong>{{ channels.length }}</strong>
      <small>{{ enabledCount }} 个已启用</small>
    </div>
    <div class="metric-card">
      <span class="metric-label">健康渠道</span>
      <strong>{{ healthyCount }}</strong>
      <small>{{ unhealthyCount }} 个异常</small>
    </div>
    <div class="metric-card">
      <span class="metric-label">模型覆盖</span>
      <strong>{{ modelCount }}</strong>
      <small>去重后的模型数</small>
    </div>
  </div>

  <el-alert
    class="guide-card"
    title="拖动卡片左上角手柄可调整渠道优先级；优先级越高越先尝试,同优先级下可结合权重做调度。"
    type="info"
    show-icon
    :closable="false"
  />

  <div v-loading="loading" class="channel-grid-wrap">
    <draggable
      v-if="channels.length"
      v-model="channels"
      class="channel-grid"
      item-key="id"
      handle=".drag-handle"
      ghost-class="drag-ghost"
      :animation="180"
      @end="onDragEnd"
    >
      <template #item="{ element }">
        <ChannelCard
          :channel="element"
          @toggle="toggleChannel"
          @test="handleTest"
          @edit="openEditDialog"
          @delete="handleDelete"
          @fetch-models="handleFetchModels"
        />
      </template>
    </draggable>

    <el-empty v-else-if="!loading" class="empty-card" description="暂无渠道,请添加一个 API 渠道">
      <el-button type="primary" :icon="Plus" @click="openCreateDialog">添加渠道</el-button>
    </el-empty>
  </div>

  <el-dialog v-model="dialogVisible" :title="editingChannel ? '编辑渠道' : '添加渠道'" width="800px" class="form-dialog">
    <el-form :model="form" label-position="top">
      <!-- 协议选择区 -->
      <div class="form-section protocol-section">
        <h3>协议选择</h3>
        <div class="protocol-cards">
          <div
            v-for="protocol in protocolOptions"
            :key="protocol.type"
            class="protocol-card"
            :class="{ active: form.type === protocol.type }"
            @click="selectProtocol(protocol)"
          >
            <div class="protocol-header">
              <strong>{{ protocol.name }}</strong>
              <span v-if="form.type === protocol.type" class="active-badge">已选</span>
            </div>
            <ul class="protocol-features">
              <li v-for="(feature, idx) in protocol.features" :key="idx">{{ feature }}</li>
            </ul>
          </div>
        </div>
      </div>

      <div class="form-section">
        <h3>基础信息</h3>
        <div class="form-grid">
          <el-form-item label="渠道名称" class="span-2">
            <el-input v-model="form.name" placeholder="如 OpenAI Primary" />
          </el-form-item>
        </div>
      </div>

      <div class="form-section">
        <h3>认证与地址</h3>
        <div class="form-grid">
          <el-form-item label="API Key" class="span-2">
            <el-input v-model="form.api_key" show-password placeholder="sk-..." />
          </el-form-item>
          <el-form-item label="Base URL（留空使用默认）" class="span-2">
            <el-input v-model="form.base_url" :placeholder="baseURLPlaceholder" />
            <small class="form-tip">{{ baseURLHint }}</small>
          </el-form-item>
        </div>
      </div>

      <div class="form-section">
        <h3>调度与可靠性</h3>
        <div class="form-grid four-columns">
          <el-form-item label="优先级">
            <el-input-number v-model="form.priority" :min="0" controls-position="right" />
          </el-form-item>
          <el-form-item label="权重">
            <el-input-number v-model="form.weight" :min="1" controls-position="right" />
          </el-form-item>
          <el-form-item label="超时(ms)">
            <el-input-number v-model="form.timeout" :min="1000" :step="1000" controls-position="right" />
          </el-form-item>
          <el-form-item label="重试次数">
            <el-input-number v-model="form.max_retries" :min="0" controls-position="right" />
          </el-form-item>
        </div>
      </div>

      <div class="form-section">
        <h3>上游真实模型列表</h3>
        <el-alert type="info" :closable="false" show-icon style="margin-bottom: 12px">
          <template #title>
            <span style="font-size: 14px">
              这里填写上游渠道实际支持的模型名称（如 gpt-4o、claude-3-5-sonnet-20241022）；
              对外调用时的显示名称可在"模型列表"页编辑。
            </span>
          </template>
        </el-alert>
        <el-form-item label="模型列表">
          <el-input
            v-model="modelsText"
            type="textarea"
            :rows="5"
            placeholder="每行一个模型,如 gpt-4o；也支持用逗号分隔"
          />
        </el-form-item>
      </div>

      <div class="form-section">
        <h3>启用状态</h3>
        <el-form-item label="启用渠道">
          <el-switch v-model="form.enabled" active-text="启用" inactive-text="停用" />
        </el-form-item>
      </div>
    </el-form>

    <template #footer>
      <el-button @click="dialogVisible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="saveChannel">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import draggable from 'vuedraggable'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh } from '@element-plus/icons-vue'
import ChannelCard from '@/components/ChannelCard.vue'
import {
  createChannel,
  deleteChannel,
  fetchChannelModels,
  getChannels,
  reorderChannels,
  testChannel,
  updateChannel,
  type Channel
} from '@/api/channels'

interface ProtocolOption {
  type: string
  name: string
  features: string[]
  defaultURL: string
  hint: string
}

const loading = ref(false)
const saving = ref(false)
const channels = ref<Channel[]>([])
const dialogVisible = ref(false)
const editingChannel = ref<Channel | null>(null)
const modelsText = ref('')

const protocolOptions: ProtocolOption[] = [
  {
    type: 'openai_compatible',
    name: 'OpenAI 兼容',
    features: [
      '路径: /v1/chat/completions、/v1/responses',
      '认证: Authorization: Bearer',
      '适用: NewAPI/OneAPI/OpenRouter/DeepSeek 等'
    ],
    defaultURL: 'https://api.openai.com/v1',
    hint: 'OpenAI 兼容渠道通常填写服务商的 /v1 地址'
  },
  {
    type: 'anthropic',
    name: 'Anthropic 官方',
    features: [
      '路径: /v1/messages',
      '认证: x-api-key + anthropic-version',
      '结构: system/max_tokens 独立字段'
    ],
    defaultURL: 'https://api.anthropic.com/v1',
    hint: 'Anthropic 官方协议,需要 x-api-key 和 anthropic-version 头'
  },
  {
    type: 'gemini',
    name: 'Gemini 官方',
    features: [
      '路径: /v1beta/models/{model}:generateContent',
      '认证: x-goog-api-key',
      '结构: contents/parts,模型在 URL 中'
    ],
    defaultURL: 'https://generativelanguage.googleapis.com/v1beta',
    hint: 'Gemini 官方协议,模型名会拼接到 URL 路径中'
  }
]

const form = reactive<Partial<Channel>>(defaultForm())

const enabledCount = computed(() => channels.value.filter((item) => item.enabled).length)
const healthyCount = computed(() => channels.value.filter((item) => item.health_status === 'healthy').length)
const unhealthyCount = computed(() => channels.value.filter((item) => item.health_status === 'unhealthy').length)
const modelCount = computed(() => new Set(channels.value.flatMap((item) => item.models || [])).size)

const baseURLPlaceholder = computed(() => {
  const protocol = protocolOptions.find((p) => p.type === form.type)
  return protocol?.defaultURL || 'https://api.example.com/v1'
})

const baseURLHint = computed(() => {
  const protocol = protocolOptions.find((p) => p.type === form.type)
  return protocol?.hint || '根据协议选择填写对应的 Base URL'
})

onMounted(loadChannels)

function defaultForm(): Partial<Channel> {
  return {
    name: '',
    type: 'openai_compatible',
    api_key: '',
    base_url: '',
    models: [],
    priority: channels.value.length + 1,
    weight: 1,
    enabled: true,
    timeout: 60000,
    max_retries: 3
  }
}

function selectProtocol(protocol: ProtocolOption) {
  form.type = protocol.type
  // 如果 Base URL 为空,填入建议地址
  if (!form.base_url) {
    form.base_url = protocol.defaultURL
  }
}

async function loadChannels() {
  loading.value = true
  try {
    const res = await getChannels()
    channels.value = res.data.data || []
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || '加载渠道失败')
  } finally {
    loading.value = false
  }
}

function resetForm() {
  Object.assign(form, defaultForm())
  modelsText.value = ''
}

function openCreateDialog() {
  editingChannel.value = null
  resetForm()
  dialogVisible.value = true
}

function openEditDialog(channel: Channel) {
  editingChannel.value = channel
  Object.assign(form, { ...channel })
  modelsText.value = channel.models?.join('\n') || ''
  dialogVisible.value = true
}

async function saveChannel() {
  const payload: Partial<Channel> = {
    ...form,
    models: modelsText.value
      .split(/[\n,]+/)
      .map((item) => item.trim())
      .filter(Boolean)
  }

  saving.value = true
  try {
    if (editingChannel.value) {
      await updateChannel(editingChannel.value.id, payload)
      ElMessage.success('渠道已更新')
    } else {
      await createChannel(payload)
      ElMessage.success('渠道已创建')
    }
    dialogVisible.value = false
    await loadChannels()
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || '保存失败')
  } finally {
    saving.value = false
  }
}

async function toggleChannel(channel: Channel, enabled: boolean) {
  try {
    await updateChannel(channel.id, { ...channel, enabled })
    channel.enabled = enabled
    ElMessage.success(enabled ? '渠道已启用' : '渠道已禁用')
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || '更新状态失败')
  }
}

async function handleTest(channel: Channel) {
  try {
    const res = await testChannel(channel.id)
    if (res.data.success) {
      ElMessage.success(res.data.message)
    } else {
      ElMessage.warning(res.data.message)
    }
    await loadChannels()
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || '测试失败')
  }
}

async function handleFetchModels(channel: Channel) {
  try {
    const res = await fetchChannelModels(channel.id)
    ElMessage.success(`已获取 ${res.data.models.length} 个模型`)
    await loadChannels()
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || '获取模型失败')
  }
}

async function handleDelete(channel: Channel) {
  try {
    await ElMessageBox.confirm(`确定删除渠道「${channel.name}」吗?`, '删除确认', { type: 'warning' })
  } catch {
    return
  }

  try {
    await deleteChannel(channel.id)
    ElMessage.success('渠道已删除')
    await loadChannels()
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || '删除失败')
  }
}

async function onDragEnd() {
  const orders = channels.value.map((channel, index) => ({
    id: channel.id,
    priority: channels.value.length - index
  }))

  try {
    await reorderChannels(orders)
    ElMessage.success('优先级已更新')
    await loadChannels()
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || '更新优先级失败')
  }
}
</script>

<style scoped>
.protocol-section {
  padding-bottom: 16px;
}

.protocol-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 12px;
}

.protocol-card {
  border: 2px solid var(--el-border-color);
  border-radius: 8px;
  padding: 14px;
  cursor: pointer;
  transition: all 0.2s;
}

.protocol-card:hover {
  border-color: var(--el-color-primary-light-5);
  background: var(--el-fill-color-light);
}

.protocol-card.active {
  border-color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
}

.protocol-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.protocol-header strong {
  font-size: 15px;
  color: var(--el-text-color-primary);
}

.active-badge {
  font-size: 12px;
  color: var(--el-color-primary);
  font-weight: 600;
}

.protocol-features {
  list-style: none;
  padding: 0;
  margin: 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--el-text-color-regular);
}

.protocol-features li {
  margin-bottom: 4px;
}

.form-tip {
  display: block;
  margin-top: 4px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
</style>

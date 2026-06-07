<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">渠道管理</h1>
      <div>
        <el-input
          v-model="adminKey"
          placeholder="管理密钥"
          show-password
          style="width: 260px; margin-right: 12px"
          @change="saveAdminKey"
        />
        <el-button type="primary" @click="openCreateDialog">添加渠道</el-button>
      </div>
    </div>

    <el-alert
      title="提示：拖动左侧图标可调整优先级顺序。当前页面是轻量原型，后续可扩展更多 NewAPI/CCSwitch 风格能力。"
      type="info"
      show-icon
      :closable="false"
      style="margin-bottom: 16px"
    />

    <draggable
      v-model="channels"
      class="channel-list"
      item-key="id"
      handle=".drag-handle"
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

    <el-empty v-if="channels.length === 0 && !loading" description="暂无渠道，请添加一个 API 渠道" />

    <el-dialog v-model="dialogVisible" :title="editingChannel ? '编辑渠道' : '添加渠道'" width="620px">
      <el-form :model="form" label-width="100px">
        <el-form-item label="渠道名称">
          <el-input v-model="form.name" placeholder="如 OpenAI Primary" />
        </el-form-item>
        <el-form-item label="渠道类型">
          <el-select v-model="form.type" style="width: 100%">
            <el-option label="OpenAI" value="openai" />
            <el-option label="OpenAI兼容" value="openai_compatible" />
            <el-option label="Anthropic Claude" value="anthropic" />
            <el-option label="Google Gemini" value="gemini" />
            <el-option label="DeepSeek" value="deepseek" />
            <el-option label="Codex" value="codex" />
          </el-select>
        </el-form-item>
        <el-form-item label="API Key">
          <el-input v-model="form.api_key" show-password placeholder="sk-..." />
        </el-form-item>
        <el-form-item label="Base URL">
          <el-input v-model="form.base_url" placeholder="https://api.openai.com/v1" />
        </el-form-item>
        <el-form-item label="模型列表">
          <el-input
            v-model="modelsText"
            type="textarea"
            :rows="3"
            placeholder="每行一个模型，如 gpt-4o"
          />
        </el-form-item>
        <el-form-item label="优先级">
          <el-input-number v-model="form.priority" :min="0" />
        </el-form-item>
        <el-form-item label="权重">
          <el-input-number v-model="form.weight" :min="1" />
        </el-form-item>
        <el-form-item label="超时(ms)">
          <el-input-number v-model="form.timeout" :min="1000" :step="1000" />
        </el-form-item>
        <el-form-item label="重试次数">
          <el-input-number v-model="form.max_retries" :min="0" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveChannel">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import draggable from 'vuedraggable'
import { ElMessage, ElMessageBox } from 'element-plus'
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

const loading = ref(false)
const channels = ref<Channel[]>([])
const dialogVisible = ref(false)
const editingChannel = ref<Channel | null>(null)
const modelsText = ref('')
const adminKey = ref(localStorage.getItem('apirelay_admin_key') || 'change-me-in-production')

const form = reactive<Partial<Channel>>({
  name: '',
  type: 'openai',
  api_key: '',
  base_url: '',
  models: [],
  priority: 10,
  weight: 1,
  enabled: true,
  timeout: 60000,
  max_retries: 3
})

onMounted(loadChannels)

function saveAdminKey() {
  localStorage.setItem('apirelay_admin_key', adminKey.value)
  ElMessage.success('管理密钥已保存')
  loadChannels()
}

async function loadChannels() {
  loading.value = true
  try {
    const res = await getChannels()
    channels.value = res.data.data
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || '加载渠道失败')
  } finally {
    loading.value = false
  }
}

function resetForm() {
  Object.assign(form, {
    name: '',
    type: 'openai',
    api_key: '',
    base_url: '',
    models: [],
    priority: channels.value.length + 1,
    weight: 1,
    enabled: true,
    timeout: 60000,
    max_retries: 3
  })
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
  const payload = {
    ...form,
    models: modelsText.value
      .split('\n')
      .map((item) => item.trim())
      .filter(Boolean)
  }

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
  await ElMessageBox.confirm(`确定删除渠道「${channel.name}」吗？`, '删除确认', {
    type: 'warning'
  })

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

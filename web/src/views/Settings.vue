<template>
  <section class="page-hero">
    <div>
      <p class="eyebrow">Global Settings</p>
      <h1>全局设置</h1>
      <p>集中管理管理台行为开关。当前设置仅影响管理台模型测试，不影响真实用户请求路由。</p>
    </div>
    <div class="page-actions">
      <el-button :icon="Refresh" :loading="loading" @click="loadSettings">刷新</el-button>
    </div>
  </section>

  <el-alert class="guide-card" type="info" show-icon :closable="false">
    <template #title>
      <span>模型路由开关与测试开关彼此独立：路由由模型列表中的“参与路由”控制，测试由“允许测试”和本页默认参数控制。</span>
    </template>
  </el-alert>

  <el-card class="panel-card" shadow="never" v-loading="loading">
    <template #header>
      <div class="panel-header">
        <span>模型测试设置</span>
        <el-tag effect="plain">仅管理台测试</el-tag>
      </div>
    </template>

    <el-form :model="form" label-position="top" class="settings-form">
      <el-form-item label="默认测试 Prompt">
        <el-input v-model="form.model_test.default_prompt" type="textarea" :rows="3" placeholder="Say OK in one short sentence." />
      </el-form-item>
      <div class="settings-grid">
        <el-form-item label="测试超时 ms">
          <el-input-number v-model="form.model_test.timeout_ms" :min="1000" :max="600000" :step="1000" style="width: 100%" />
        </el-form-item>
        <el-form-item label="默认最大输出 tokens">
          <el-input-number v-model="form.model_test.max_output_tokens" :min="1" :max="8192" style="width: 100%" />
        </el-form-item>
        <el-form-item label="默认 temperature">
          <el-input-number v-model="form.model_test.temperature" :min="0" :max="2" :step="0.1" :precision="1" style="width: 100%" />
        </el-form-item>
      </div>
      <el-form-item label="测试已隐藏模型">
        <el-switch v-model="form.model_test.include_disabled_models" active-text="允许" inactive-text="不允许" />
        <small class="form-tip">该选项作为管理偏好保存；模型测试资格仍以每条模型记录的“允许测试”为准。</small>
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button type="primary" :loading="saving" @click="saveSettings">保存设置</el-button>
    </template>
  </el-card>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { useRoute } from 'vue-router'
import { getSettings, updateSettings, type Settings } from '@/api/settings'

const route = useRoute()
const loading = ref(false)
const saving = ref(false)

const form = reactive<Settings>({
  model_test: {
    default_prompt: 'Say OK in one short sentence.',
    timeout_ms: 30000,
    max_output_tokens: 32,
    temperature: 0,
    include_disabled_models: true
  }
})

watch(() => route.fullPath, () => loadSettings(), { immediate: true })

async function loadSettings() {
  loading.value = true
  try {
    const res = await getSettings()
    Object.assign(form.model_test, res.data.data.model_test)
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || '加载设置失败')
  } finally {
    loading.value = false
  }
}

async function saveSettings() {
  saving.value = true
  try {
    const res = await updateSettings({ model_test: { ...form.model_test } })
    Object.assign(form.model_test, res.data.data.model_test)
    ElMessage.success('设置已保存')
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || '保存设置失败')
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.settings-form {
  max-width: 860px;
}

.settings-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}

.form-tip {
  display: block;
  margin-top: 6px;
  color: var(--muted);
  font-size: 12px;
}

@media (max-width: 900px) {
  .settings-grid {
    grid-template-columns: 1fr;
  }
}
</style>

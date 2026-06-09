<template>
  <section class="page-hero">
    <div>
      <p class="eyebrow">Global Settings</p>
      <h1>全局设置</h1>
      <p>集中管理管理台模型测试的默认行为。这里的配置不会改变真实用户请求路由，只会影响测试面板的初始参数与范围偏好。</p>
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

  <div v-loading="loading" class="settings-layout">
    <el-form :model="form" label-position="top" class="settings-form">
      <el-card class="panel-card settings-section-card" shadow="never">
        <template #header>
          <div class="panel-header">
            <span>默认 Prompt</span>
            <el-tag effect="plain">测试输入</el-tag>
          </div>
        </template>
        <el-form-item label="默认测试 Prompt">
          <el-input
            v-model="form.model_test.default_prompt"
            type="textarea"
            :rows="4"
            placeholder="Say OK in one short sentence."
          />
          <small class="form-tip">打开模型测试弹窗时会自动填入，可用于统一连通性测试问题。</small>
        </el-form-item>
      </el-card>

      <el-card class="panel-card settings-section-card" shadow="never">
        <template #header>
          <div class="panel-header">
            <span>测试参数</span>
            <el-tag type="success" effect="plain">默认值</el-tag>
          </div>
        </template>
        <div class="settings-grid">
          <el-form-item label="测试超时 ms">
            <el-input-number v-model="form.model_test.timeout_ms" :min="1000" :max="600000" :step="1000" style="width: 100%" />
            <small class="form-tip">等待上游响应的最长时间，网络较慢或模型较大时可适当调高。</small>
          </el-form-item>
          <el-form-item label="默认最大输出 tokens">
            <el-input-number v-model="form.model_test.max_output_tokens" :min="1" :max="8192" style="width: 100%" />
            <small class="form-tip">限制测试响应的最大输出长度，数值越小越适合快速连通性验证。</small>
          </el-form-item>
          <el-form-item label="默认 temperature">
            <el-input-number v-model="form.model_test.temperature" :min="0" :max="2" :step="0.1" :precision="1" style="width: 100%" />
            <small class="form-tip">控制输出随机性：0 更稳定，数值越高越随机；部分兼容渠道可能不支持。</small>
          </el-form-item>
        </div>
      </el-card>

      <el-card class="panel-card settings-section-card" shadow="never">
        <template #header>
          <div class="panel-header">
            <span>测试范围</span>
            <el-tag type="warning" effect="plain">管理偏好</el-tag>
          </div>
        </template>
        <div class="range-card">
          <div>
            <strong>测试已隐藏模型</strong>
            <p>开启后，管理台可在测试场景中考虑已隐藏模型；真实请求仍不会路由到已隐藏模型。</p>
          </div>
          <el-switch v-model="form.model_test.include_disabled_models" active-text="允许" inactive-text="不允许" />
        </div>
        <small class="form-tip">最终测试资格仍以每条模型记录的“允许测试”为准。</small>
      </el-card>
    </el-form>

    <div class="save-bar">
      <div>
        <strong>保存全局设置</strong>
        <span>{{ saving ? '正在写入配置...' : '修改后点击保存，新的默认值会用于后续测试弹窗。' }}</span>
      </div>
      <el-button type="primary" :loading="saving" @click="saveSettings">保存设置</el-button>
    </div>
  </div>
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
.settings-layout {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.settings-form {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 18px;
}

.settings-section-card :deep(.el-card__body) {
  background: linear-gradient(145deg, rgba(255, 255, 255, 0.98), rgba(248, 251, 255, 0.92));
}

.settings-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}

.range-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding: 18px;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: linear-gradient(135deg, #ffffff, #f8fafc);
}

.range-card strong,
.range-card p {
  display: block;
  margin: 0;
}

.range-card p {
  margin-top: 6px;
  color: var(--muted);
  line-height: 1.6;
}

.form-tip {
  display: block;
  margin-top: 6px;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.5;
}

.save-bar {
  position: sticky;
  bottom: 18px;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding: 16px 18px;
  border: 1px solid rgba(223, 231, 243, 0.92);
  border-radius: var(--radius-lg);
  background: rgba(255, 255, 255, 0.9);
  box-shadow: var(--shadow-soft);
  backdrop-filter: blur(14px);
}

.save-bar strong,
.save-bar span {
  display: block;
}

.save-bar span {
  margin-top: 4px;
  color: var(--muted);
  font-size: 13px;
}

@media (max-width: 900px) {
  .settings-grid {
    grid-template-columns: 1fr;
  }

  .range-card,
  .save-bar {
    align-items: flex-start;
    flex-direction: column;
  }

  .save-bar .el-button {
    align-self: flex-end;
  }
}

@media (max-width: 620px) {
  .save-bar .el-button {
    align-self: stretch;
  }
}
</style>

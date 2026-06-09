<template>
  <el-card class="panel-card proxy-config-card" shadow="never">
    <template #header>
      <div class="panel-header">
        <div>
          <span>全局代理配置</span>
          <small>所有协议入口共享同一套全局配置。</small>
        </div>
        <el-button type="primary" :loading="saving" @click="saveConfig">保存配置</el-button>
      </div>
    </template>

    <el-form :model="form" label-position="top" class="proxy-config-form">
      <div class="toggle-row">
        <div class="toggle-item">
          <div>
            <strong>代理开关</strong>
            <span>关闭后兼容 API 不再选择任何上游渠道。</span>
          </div>
          <el-switch v-model="form.enabled" active-text="启用" inactive-text="关闭" />
        </div>
        <div class="toggle-item">
          <div>
            <strong>自动故障转移</strong>
            <span>启用后按全局队列顺序尝试可用渠道。</span>
          </div>
          <el-switch v-model="form.auto_failover_enabled" active-text="启用" inactive-text="关闭" />
        </div>
      </div>

      <div class="form-grid three-columns">
        <el-form-item label="最大重试次数">
          <el-input-number v-model="form.max_retries" :min="0" :max="20" controls-position="right" />
        </el-form-item>
        <el-form-item label="非流式超时(ms)">
          <el-input-number v-model="form.non_streaming_timeout_ms" :min="1000" :step="1000" controls-position="right" />
        </el-form-item>
        <el-form-item label="流式首包超时(ms)">
          <el-input-number v-model="form.streaming_first_byte_timeout" :min="100" :step="500" controls-position="right" />
        </el-form-item>
        <el-form-item label="流式静默超时(ms)">
          <el-input-number v-model="form.streaming_idle_timeout_ms" :min="1000" :step="1000" controls-position="right" />
        </el-form-item>
        <el-form-item label="熔断失败阈值">
          <el-input-number v-model="form.circuit_failure_threshold" :min="1" :max="100" controls-position="right" />
        </el-form-item>
        <el-form-item label="熔断恢复阈值">
          <el-input-number v-model="form.circuit_success_threshold" :min="1" :max="100" controls-position="right" />
        </el-form-item>
        <el-form-item label="熔断打开时间(s)">
          <el-input-number v-model="form.circuit_open_seconds" :min="1" :max="86400" controls-position="right" />
        </el-form-item>
      </div>
    </el-form>
  </el-card>
</template>

<script setup lang="ts">
import { reactive, watch } from 'vue'
import type { ProxyConfig } from '@/api/proxy'

const props = defineProps<{
  config: ProxyConfig | null
  saving?: boolean
}>()

const emit = defineEmits<{
  save: [config: Partial<ProxyConfig>]
}>()

const form = reactive<ProxyConfig>(defaultConfig())

watch(
  () => props.config,
  (config) => {
    Object.assign(form, defaultConfig(), config || {})
  },
  { immediate: true, deep: true }
)

function defaultConfig(): ProxyConfig {
  return {
    enabled: true,
    auto_failover_enabled: true,
    max_retries: 2,
    non_streaming_timeout_ms: 60000,
    streaming_first_byte_timeout: 5000,
    streaming_idle_timeout_ms: 60000,
    circuit_failure_threshold: 3,
    circuit_success_threshold: 1,
    circuit_open_seconds: 30
  }
}

function saveConfig() {
  emit('save', { ...form })
}
</script>

<style scoped>
.proxy-config-card {
  margin-bottom: 20px;
}

.panel-header > div {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.panel-header small {
  color: var(--muted);
  font-size: 12px;
  font-weight: 400;
}

.proxy-config-form {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.toggle-row {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.toggle-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 18px;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: linear-gradient(135deg, #ffffff, #f8fafc);
}

.toggle-item strong,
.toggle-item span {
  display: block;
}

.toggle-item strong {
  margin-bottom: 6px;
  color: var(--text);
}

.toggle-item span {
  color: var(--muted);
  font-size: 13px;
  line-height: 1.55;
}

.form-grid.three-columns {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.proxy-config-form :deep(.el-input-number) {
  width: 100%;
}

@media (max-width: 960px) {
  .toggle-row,
  .form-grid.three-columns {
    grid-template-columns: 1fr;
  }
}
</style>

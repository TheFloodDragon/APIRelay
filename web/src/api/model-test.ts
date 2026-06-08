import request from '@/utils/request'
import type { ApiResponse } from '@/api/channels'

export interface ModelTestConfig {
  timeout_secs: number
  max_retries: number
  degraded_threshold_ms: number
  test_prompt: string
  max_tokens: number
  stream: boolean
  default_models: Record<string, string>
}

export function getModelTestConfig() {
  return request.get<ApiResponse<ModelTestConfig>>('/model-test/config')
}

export function saveModelTestConfig(data: ModelTestConfig) {
  return request.put<ApiResponse<ModelTestConfig>>('/model-test/config', data)
}

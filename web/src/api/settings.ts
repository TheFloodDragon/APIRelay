import type { ApiResponse } from '@/api/channels'
import request from '@/utils/request'

export interface ModelTestSettings {
  default_prompt: string
  timeout_ms: number
  max_output_tokens: number
  temperature: number
  include_disabled_models: boolean
}

export interface Settings {
  model_test: ModelTestSettings
}

export function getSettings() {
  return request.get<ApiResponse<Settings>>('/settings')
}

export function updateSettings(payload: Settings) {
  return request.put<ApiResponse<Settings>>('/settings', payload)
}

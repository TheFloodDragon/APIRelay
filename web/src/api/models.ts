import type { ApiResponse, Channel } from '@/api/channels'
import request from '@/utils/request'

export interface ModelRecord {
  id: number
  name: string
  display_name: string
  channel_id: number
  channel?: Channel | null
  alias?: string
  redirect_to?: string
  enabled: boolean
  test_enabled: boolean
  created_at: string
}

export interface ModelTestChannel {
  model_id: number
  model_name: string
  display_name: string
  test_enabled: boolean
  route_enabled: boolean
  channel: Channel
}

export interface ModelTestPayload {
  channel_id?: number
  prompt?: string
  timeout_ms?: number
  max_output_tokens?: number
  temperature?: number
}

export interface ModelTestResult {
  ok: boolean
  model_id: number
  model: string
  resolved_model: string
  channel_id: number
  channel_name: string
  channel_type: string
  latency_ms: number
  status_code: number
  content: string
  error: string
}

export function getModels() {
  return request.get<ApiResponse<ModelRecord[]>>('/models')
}

export function getAvailableModels() {
  return request.get<ApiResponse<ModelRecord[]>>('/models/available')
}

export function updateModel(id: number, payload: { display_name?: string; enabled?: boolean; test_enabled?: boolean }) {
  return request.put<{ success: boolean; message: string }>(`/models/${id}`, payload)
}

export function getModelTestChannels(id: number) {
  return request.get<ApiResponse<ModelTestChannel[]>>(`/models/${id}/test-channels`)
}

export function testModel(id: number, payload: ModelTestPayload) {
  return request.post<ApiResponse<ModelTestResult>>(`/models/${id}/test`, payload)
}

export function deleteModel(id: number) {
  return request.delete<{ success: boolean; message: string }>(`/models/${id}`)
}

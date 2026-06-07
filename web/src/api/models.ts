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
  created_at: string
}

export function getModels() {
  return request.get<ApiResponse<ModelRecord[]>>('/models')
}

export function getAvailableModels() {
  return request.get<ApiResponse<ModelRecord[]>>('/models/available')
}

export function updateModel(id: number, payload: { display_name?: string; enabled?: boolean }) {
  return request.put<{ success: boolean; message: string }>(`/models/${id}`, payload)
}

export function deleteModel(id: number) {
  return request.delete<{ success: boolean; message: string }>(`/models/${id}`)
}

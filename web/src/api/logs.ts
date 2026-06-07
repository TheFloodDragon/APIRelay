import type { Channel } from '@/api/channels'
import request from '@/utils/request'

export interface RequestLog {
  id: number
  request_id?: string
  channel_id?: number | null
  channel?: Channel | null
  channel_type?: string
  api_type?: string
  relay_mode?: string
  relay_format?: string
  resolved_model?: string
  model: string
  method: string
  path: string
  status_code: number
  request_tokens: number
  response_tokens: number
  latency: number
  error?: string
  ip: string
  api_key_id?: number | null
  created_at: string
}

export interface LogsQuery {
  limit?: number
  offset?: number
}

export interface LogsResponse {
  success: boolean
  data: RequestLog[]
  total: number
}

export function getLogs(params: LogsQuery = {}) {
  return request.get<LogsResponse>('/logs', { params })
}

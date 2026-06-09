import request from '@/utils/request'
import type { Channel, ApiResponse } from '@/api/channels'

export interface ProxyConfig {
  id?: number
  enabled: boolean
  auto_failover_enabled: boolean
  max_retries: number
  non_streaming_timeout_ms: number
  streaming_first_byte_timeout: number
  streaming_idle_timeout_ms: number
  circuit_failure_threshold: number
  circuit_success_threshold: number
  circuit_open_seconds: number
  created_at?: string
  updated_at?: string
}

export interface FailoverQueueItem {
  id?: number
  channel_id: number
  channel?: Channel
  position: number
  created_at?: string
}

export type CircuitState = 'closed' | 'open' | 'half_open' | string

export interface CircuitSnapshot {
  channel_id: number
  state: CircuitState
  consecutive_failures: number
  consecutive_successes: number
  opened_until?: string | null
  half_open_permit_in_use: boolean
  failure_threshold: number
  success_threshold: number
  open_duration_seconds: number
}

export interface ProviderHealth {
  channel_id: number
  is_healthy: boolean
  consecutive_failures: number
  last_success_at?: string | null
  last_failure_at?: string | null
  last_error?: string
  updated_at?: string
}

export interface CircuitStatus {
  channel_id: number
  channel?: Omit<Channel, 'api_key'>
  health?: ProviderHealth
  circuit: CircuitSnapshot
}

export interface ProxyStatus {
  config: ProxyConfig
  failover_queue: FailoverQueueItem[]
  circuits: CircuitStatus[]
}

export interface SaveFailoverQueuePayload {
  channel_ids: number[]
}

export function getProxyStatus() {
  return request.get<ApiResponse<ProxyStatus>>('/proxy/status')
}

export function getProxyConfig() {
  return request.get<ApiResponse<ProxyConfig>>('/proxy/config')
}

export function updateProxyConfig(data: Partial<ProxyConfig>) {
  return request.put<ApiResponse<ProxyConfig>>('/proxy/config', data)
}

export function getFailoverQueue() {
  return request.get<ApiResponse<FailoverQueueItem[]>>('/proxy/failover-queue')
}

export function updateFailoverQueue(channelIDs: number[]) {
  return request.put<ApiResponse<FailoverQueueItem[]>>('/proxy/failover-queue', { channel_ids: channelIDs } satisfies SaveFailoverQueuePayload)
}

export function getCircuits() {
  return request.get<ApiResponse<CircuitStatus[]>>('/proxy/circuits')
}

export function resetCircuit(channelID: number) {
  return request.post<ApiResponse<null>>(`/proxy/circuits/${channelID}/reset`)
}

import request from '@/utils/request'

export interface Channel {
  id: number
  name: string
  type: string
  api_key: string
  base_url: string
  models: string[]
  priority: number
  weight: number
  enabled: boolean
  timeout: number
  max_retries: number
  health_status: string
}

export function getChannels() {
  return request.get<{ success: boolean; data: Channel[] }>('/channels')
}

export function createChannel(data: Partial<Channel>) {
  return request.post('/channels', data)
}

export function updateChannel(id: number, data: Partial<Channel>) {
  return request.put(`/channels/${id}`, data)
}

export function deleteChannel(id: number) {
  return request.delete(`/channels/${id}`)
}

export function reorderChannels(orders: Array<{ id: number; priority: number }>) {
  return request.put('/channels/reorder', { orders })
}

export function testChannel(id: number) {
  return request.post(`/channels/${id}/test`)
}

export function fetchChannelModels(id: number) {
  return request.post(`/channels/${id}/models`)
}

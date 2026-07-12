export async function resetBreakerAndConfirm(api, channelId, refreshChannels, now = Date.now()) {
  await api.post(`/channels/${channelId}/health/reset`)
  const [health, channels] = await Promise.all([
    api.get(`/channels/${channelId}/health`),
    refreshChannels(),
  ])
  const channel = channels?.find((item) => item.id === channelId)
  if (!channel || health?.circuit_state !== 'closed' || (channel.cooldown_until && channel.cooldown_until > now)) {
    throw new Error('后端未确认熔断与冷却状态已清除')
  }
  return { health, channel }
}

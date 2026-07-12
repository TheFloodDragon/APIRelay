import { describe, expect, it, vi } from 'vitest'
import { resetBreakerAndConfirm } from '../breakerReset'

describe('resetBreakerAndConfirm', () => {
  it('waits for backend health and refreshed channel confirmation', async () => {
    const api = {
      post: vi.fn().mockResolvedValue({}),
      get: vi.fn().mockResolvedValue({ circuit_state: 'closed' }),
    }
    const refresh = vi.fn().mockResolvedValue([{ id: 7, cooldown_until: 0 }])

    const result = await resetBreakerAndConfirm(api, 7, refresh, 1000)

    expect(api.post).toHaveBeenCalledWith('/channels/7/health/reset')
    expect(api.get).toHaveBeenCalledWith('/channels/7/health')
    expect(refresh).toHaveBeenCalledOnce()
    expect(result.channel.cooldown_until).toBe(0)
  })

  it.each([
    [{ circuit_state: 'open' }, [{ id: 7, cooldown_until: 0 }]],
    [{ circuit_state: 'closed' }, [{ id: 7, cooldown_until: 2000 }]],
  ])('rejects when backend still reports open or cooldown', async (health, channels) => {
    const api = {
      post: vi.fn().mockResolvedValue({}),
      get: vi.fn().mockResolvedValue(health),
    }

    await expect(resetBreakerAndConfirm(api, 7, async () => channels, 1000))
      .rejects.toThrow('后端未确认熔断与冷却状态已清除')
  })
})

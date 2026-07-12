import { describe, expect, it, vi } from 'vitest'

vi.mock('../api', () => ({ fmtTime: (value) => String(value || '—') }))

import { healthClass, healthPercent, healthText } from '../health'

describe('health helpers', () => {
  it('formats empty and healthy samples', () => {
    expect(healthText(null)).toBe('未调用')
    const sample = { total: 10, success: 9, availability: 90 }
    expect(healthPercent(sample)).toBe(90)
    expect(healthText(sample)).toBe('90% · 9/10')
  })

  it('maps thresholds to semantic states', () => {
    const config = { healthy_threshold: 95, warning_threshold: 70 }
    expect(healthClass({ total: 10, availability: 98 }, config)).toBe('chip-run')
    expect(healthClass({ total: 10, availability: 80 }, config)).toBe('chip-test')
    expect(healthClass({ total: 10, availability: 50 }, config)).toBe('chip-trip')
  })
})

import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import SettingsNav from '../components/SettingsNav.vue'

describe('SettingsNav', () => {
  const tabs = [{ id: 'logging', label: '日志' }, { id: 'network', label: '网络' }, { id: 'health', label: '健康度' }]

  it('exposes an accessible tablist and selects by click', async () => {
    const wrapper = mount(SettingsNav, { props: { tabs, activeTab: 'logging' } })
    expect(wrapper.get('[role="tablist"]').exists()).toBe(true)
    expect(wrapper.get('#settings-tab-logging').attributes('aria-selected')).toBe('true')
    await wrapper.get('#settings-tab-network').trigger('click')
    expect(wrapper.emitted('select')?.[0]).toEqual(['network'])
  })

  it('supports arrow key navigation', async () => {
    const wrapper = mount(SettingsNav, { props: { tabs, activeTab: 'logging' } })
    await wrapper.get('#settings-tab-logging').trigger('keydown', { key: 'ArrowRight' })
    expect(wrapper.emitted('select')?.[0]).toEqual(['network'])
  })
})

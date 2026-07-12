import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import ChannelConsoleHeader from '../components/ChannelConsoleHeader.vue'

describe('ChannelConsoleHeader', () => {
  const props = {
    summary: { total: 3 },
    segments: [{ key: 'run', label: '在线', count: 2, percent: 67, tone: 'run' }],
    query: '', status: 'all', selectedCount: 2, visibleCount: 3,
  }

  it('emits search, filter and bulk actions', async () => {
    const wrapper = mount(ChannelConsoleHeader, { props })
    await wrapper.get('input[type="search"]').setValue('openai')
    expect(wrapper.emitted('update:query')?.[0]).toEqual(['openai'])
    await wrapper.get('.route-bus-segment').trigger('click')
    expect(wrapper.emitted('update:status')?.[0]).toEqual(['run'])
    await wrapper.get('.btn-danger').trigger('click')
    expect(wrapper.emitted('bulk-delete')).toHaveLength(1)
  })
})

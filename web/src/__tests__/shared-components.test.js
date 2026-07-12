import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import PageHeader from '../components/PageHeader.vue'
import ServiceStatus from '../components/ServiceStatus.vue'

describe('shared presentation components', () => {
  it('renders page hierarchy and actions', () => {
    const wrapper = mount(PageHeader, {
      props: { eyebrow: '诊断', title: '请求日志', description: '查看调用路径。' },
      slots: { actions: '<button>刷新</button>' },
    })
    expect(wrapper.get('h1').text()).toBe('请求日志')
    expect(wrapper.text()).toContain('查看调用路径。')
    expect(wrapper.get('button').text()).toBe('刷新')
  })

  it('announces service state without relying on color', async () => {
    const wrapper = mount(ServiceStatus, { props: { online: true } })
    expect(wrapper.get('[role="status"]').text()).toContain('服务在线')
    await wrapper.setProps({ online: false })
    expect(wrapper.text()).toContain('服务不可用')
  })
})

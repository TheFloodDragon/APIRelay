import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import AppSidebar from '../components/AppSidebar.vue'
import Drawer from '../components/Drawer.vue'
import Modal from '../components/Modal.vue'
import PageState from '../components/PageState.vue'

const RouterLinkStub = {
  props: ['to'],
  template: '<a :href="to"><slot /></a>',
}

afterEach(() => {
  document.body.innerHTML = ''
  document.body.style.overflow = ''
})

describe('console shell components', () => {
  it('marks the active route and emits logout', async () => {
    const wrapper = mount(AppSidebar, {
      props: { routeName: 'logs', username: 'admin', online: true },
      global: { stubs: { RouterLink: RouterLinkStub } },
    })

    const active = wrapper.get('[aria-current="page"]')
    expect(active.classes()).toContain('sidebar-link-active')
    expect(active.text()).toContain('日志')

    await wrapper.get('.sidebar-logout').trigger('click')
    expect(wrapper.emitted('logout')).toHaveLength(1)
  })

  it('renders the mobile navigation variant with visible labels', () => {
    const wrapper = mount(AppSidebar, {
      props: { mobile: true, routeName: 'dashboard' },
      global: { stubs: { RouterLink: RouterLinkStub } },
    })
    expect(wrapper.classes()).toContain('console-sidebar-mobile')
    expect(wrapper.text()).toContain('渠道')
  })

  it('traps drawer context and closes with Escape', async () => {
    const wrapper = mount(Drawer, {
      props: { open: true, title: '详情' },
      slots: { default: '<button data-autofocus>首个操作</button>' },
      attachTo: document.body,
    })
    await new Promise((resolve) => requestAnimationFrame(resolve))
    expect(document.body.style.overflow).toBe('hidden')
    expect(document.activeElement?.textContent).toContain('首个操作')

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
  })

  it('keeps persistent modals open on Escape', async () => {
    const wrapper = mount(Modal, {
      props: { open: true, title: '一次性密钥', persistent: true },
      slots: { default: '<button>复制</button>' },
      attachTo: document.body,
    })
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(wrapper.emitted('close')).toBeUndefined()
    wrapper.unmount()
  })

  it('renders loading, error and empty page states', async () => {
    const wrapper = mount(PageState, { props: { loading: true } })
    expect(wrapper.get('[role="status"]').exists()).toBe(true)

    await wrapper.setProps({ loading: false, error: 'network down' })
    expect(wrapper.get('[role="alert"]').text()).toContain('network down')
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('retry')).toHaveLength(1)

    await wrapper.setProps({ error: '', empty: true, emptyText: '没有记录' })
    expect(wrapper.text()).toContain('没有记录')
  })
})

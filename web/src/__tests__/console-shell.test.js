import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import AppSidebar from '../components/AppSidebar.vue'
import Drawer from '../components/Drawer.vue'
import Modal from '../components/Modal.vue'
import PageState from '../components/PageState.vue'
import { overlayStackDepth } from '../composables/useOverlayLayer'

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

  // 嵌套弹层：Esc 必须关闭最内层。此前 Modal/Drawer 各自在捕获阶段监听 document，
  // 按注册顺序触发，先打开的抽屉会抢先吃掉 Esc 并 stopPropagation，
  // 结果「抽屉里弹确认框」时 Esc 关掉的是抽屉。
  it('closes only the topmost overlay on Escape', async () => {
    const drawer = mount(Drawer, {
      props: { open: true, title: '渠道配置' },
      slots: { default: '<button>抽屉按钮</button>' },
      attachTo: document.body,
    })
    // 抽屉先打开，确认框后打开 —— 确认框应位于栈顶。
    const modal = mount(Modal, {
      props: { open: true, title: '确认删除' },
      slots: { default: '<button>确认</button>' },
      attachTo: document.body,
    })
    await new Promise((resolve) => requestAnimationFrame(resolve))

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(modal.emitted('close')).toHaveLength(1)
    expect(drawer.emitted('close')).toBeUndefined()

    // 内层关闭后，Esc 才应落到抽屉上。
    await modal.setProps({ open: false })
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(drawer.emitted('close')).toHaveLength(1)

    modal.unmount()
    drawer.unmount()
  })

  // 多层弹层共享 body 滚动锁：内层关闭时不能提前解锁，否则外层背景可滚动。
  it('keeps the body scroll locked until every overlay closes', async () => {
    const drawer = mount(Drawer, {
      props: { open: true, title: '外层' },
      attachTo: document.body,
    })
    const modal = mount(Modal, {
      props: { open: true, title: '内层' },
      attachTo: document.body,
    })
    await new Promise((resolve) => requestAnimationFrame(resolve))
    expect(document.body.style.overflow).toBe('hidden')

    await modal.setProps({ open: false })
    expect(document.body.style.overflow).toBe('hidden')

    await drawer.setProps({ open: false })
    expect(document.body.style.overflow).not.toBe('hidden')

    modal.unmount()
    drawer.unmount()
  })

  // 卸载必须把自己从栈里摘掉，否则残留令牌会让后续弹层永远不是栈顶。
  it('removes itself from the overlay stack on unmount', async () => {
    expect(overlayStackDepth()).toBe(0)
    const wrapper = mount(Drawer, {
      props: { open: true, title: '临时' },
      attachTo: document.body,
    })
    await new Promise((resolve) => requestAnimationFrame(resolve))
    expect(overlayStackDepth()).toBe(1)

    wrapper.unmount()
    expect(overlayStackDepth()).toBe(0)

    // 栈已清空，新弹层应能正常响应 Esc。
    const next = mount(Modal, {
      props: { open: true, title: '之后' },
      attachTo: document.body,
    })
    await new Promise((resolve) => requestAnimationFrame(resolve))
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(next.emitted('close')).toHaveLength(1)
    next.unmount()
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

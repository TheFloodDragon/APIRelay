import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import Channels from '../views/Channels.vue'

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  patch: vi.fn(),
  del: vi.fn(),
  confirmAction: vi.fn(),
  replace: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { get: mocks.get, post: mocks.post, put: mocks.put, patch: mocks.patch, delete: mocks.del },
}))

vi.mock('../composables/useConfirm', () => ({ confirmAction: mocks.confirmAction }))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ replace: mocks.replace }),
}))

const channelFixtures = [
  {
    id: 7,
    name: '主力 OpenAI',
    type: 1,
    base_url: 'https://api.openai.com/v1',
    group: 'default',
    status: 1,
    weight: 3,
    model_configs: JSON.stringify([{ name: 'gpt-4o', enabled: true }, { name: 'o4-mini', enabled: true }]),
    model_health: {},
    // 后端只下发掩码，不下发明文凭据。
    has_key: true,
    key_masked: 'sk-1…cdef',
  },
  {
    id: 12,
    name: '备用 Anthropic',
    type: 2,
    base_url: 'https://api.anthropic.com',
    group: 'backup',
    status: 2,
    weight: 1,
    model_configs: JSON.stringify([{ name: 'claude-sonnet-4', enabled: true }]),
    model_health: {},
  },
]

const PassThrough = { template: '<div><slot name="actions" /><slot /></div>' }
const DrawerStub = {
  props: ['open', 'title', 'persistent'],
  template: '<div v-if="open" class="drawer-stub"><h2 class="drawer-stub-title">{{ title }}</h2><slot /><div class="drawer-stub-footer"><slot name="footer" /></div></div>',
}
const QueueHeaderStub = {
  props: ['selectedCount', 'visibleCount'],
  template: '<div class="queue-head-stub">{{ selectedCount }} of {{ visibleCount }}</div>',
}

function mountChannels() {
  return mount(Channels, {
    global: {
      config: { globalProperties: { $toast: { add: vi.fn() } } },
      stubs: {
        BodyOverrideEditor: true,
        ChannelConsoleHeader: QueueHeaderStub,
        ConsoleIcon: true,
        ConsoleSection: PassThrough,
        DataToolbar: PassThrough,
        Drawer: DrawerStub,
        HeaderOverrideEditor: true,
        InlineNotice: PassThrough,
        Modal: { props: ['open'], template: '<div v-if="open"><slot /></div>' },
        PageHeader: PassThrough,
        PageState: PassThrough,
      },
    },
  })
}

function rows(wrapper) {
  return wrapper.findAll('.channel-table tbody tr')
}

async function saveEditor(wrapper) {
  const save = wrapper.findAll('button').find((button) => button.text().includes('保存更改'))
  if (!save) throw new Error('save button not found')
  await save.trigger('click')
  await flushPromises()
}

describe('Channels workspace', () => {
  beforeEach(() => {
    mocks.confirmAction.mockResolvedValue(true)
    mocks.get.mockImplementation((url) => {
      if (url === '/channels') return Promise.resolve(channelFixtures.map((channel) => ({ ...channel })))
      if (url === '/channel-types') return Promise.resolve([{ value: 1, name: 'OpenAI', default_base_url: 'https://api.openai.com' }, { value: 2, name: 'Anthropic', default_base_url: 'https://api.anthropic.com' }])
      if (url === '/protocols') return Promise.resolve([{ value: 'openai', name: 'OpenAI' }, { value: 'anthropic', name: 'Anthropic' }])
      if (url === '/settings/test-prompt') return Promise.resolve({ prompt: 'ping' })
      if (url === '/settings/model-health') return Promise.resolve({})
      return Promise.resolve(null)
    })
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('lists every channel full width without opening the editor', async () => {
    const wrapper = mountChannels()
    await flushPromises()

    const listed = rows(wrapper)
    expect(listed).toHaveLength(2)
    expect(listed[0].text()).toContain('主力 OpenAI')
    expect(listed[0].text()).toContain('api.openai.com/v1')
    expect(listed[0].text()).toContain('×3')
    expect(listed[1].classes()).toContain('channel-row-off')
    expect(listed[1].get('.channel-switch').attributes('aria-pressed')).toBe('false')
    expect(wrapper.find('.drawer-stub').exists()).toBe(false)
  })

  it('opens and closes the shared editor drawer for a row', async () => {
    const wrapper = mountChannels()
    await flushPromises()

    await rows(wrapper)[1].trigger('click')
    await flushPromises()

    const drawer = wrapper.get('.drawer-stub')
    expect(drawer.get('.drawer-stub-title').text()).toBe('渠道配置 · 备用 Anthropic')
    expect(wrapper.get('#channel-name').element.value).toBe('备用 Anthropic')
    expect(rows(wrapper)[1].classes()).toContain('channel-row-selected')

    const cancel = drawer.findAll('button').find((button) => button.text() === '取消')
    await cancel.trigger('click')
    await flushPromises()
    expect(wrapper.find('.drawer-stub').exists()).toBe(false)
  })

  it('creates a new channel from the header action', async () => {
    const wrapper = mountChannels()
    await flushPromises()

    const create = wrapper.findAll('button').find((button) => button.text().includes('新建渠道'))
    await create.trigger('click')
    await flushPromises()

    expect(wrapper.get('.drawer-stub-title').text()).toBe('新建渠道')
    expect(wrapper.get('#channel-name').element.value).toBe('')
    expect(wrapper.get('#channel-url').element.value).toBe('https://api.openai.com')
  })

  it('selects and clears every visible channel from the table header', async () => {
    const wrapper = mountChannels()
    await flushPromises()

    const selectAll = wrapper.get('.channel-table thead input[type="checkbox"]')
    await selectAll.setValue(true)
    expect(wrapper.get('.queue-head-stub').text()).toBe('2 of 2')
    expect(selectAll.element.checked).toBe(true)

    await selectAll.setValue(false)
    expect(wrapper.get('.queue-head-stub').text()).toBe('0 of 2')
  })

  it('toggles channel status inline without opening the editor', async () => {
    mocks.patch.mockResolvedValue({})
    const wrapper = mountChannels()
    await flushPromises()

    await rows(wrapper)[0].get('.channel-switch').trigger('click')
    await flushPromises()

    expect(mocks.patch).toHaveBeenCalledWith('/channels/7/status', { enabled: false })
    expect(wrapper.find('.drawer-stub').exists()).toBe(false)
  })

  it('keeps the key field empty and shows the mask when editing a saved channel', async () => {
    const wrapper = mountChannels()
    await flushPromises()

    await rows(wrapper)[0].trigger('click')
    await flushPromises()

    // 编辑已保存渠道时输入框必须为空（明文不回显），并提示已保存的掩码。
    expect(wrapper.get('#channel-key').element.value).toBe('')
    expect(wrapper.get('#channel-key').attributes('placeholder')).toBe('留空表示不修改')
    expect(wrapper.get('#channel-key-hint').text()).toContain('sk-1…cdef')
    // 已有凭据时不应把 key 标成必填。
    expect(wrapper.find('label[for="channel-key"]').text()).not.toContain('*')
  })

  it('omits the key field when saving without entering a new credential', async () => {
    mocks.put.mockResolvedValue({ id: 7 })
    const wrapper = mountChannels()
    await flushPromises()

    await rows(wrapper)[0].trigger('click')
    await flushPromises()

    // 改一个无关字段让保存按钮可用，key 保持留空。
    await wrapper.get('#channel-name').setValue('主力 OpenAI 改名')
    await saveEditor(wrapper)

    const [, payload] = mocks.put.mock.calls[0]
    // 留空提交不能发送 key（否则会把上游凭据覆盖成空串），也不回传只读展示字段。
    expect(payload).not.toHaveProperty('key')
    expect(payload).not.toHaveProperty('key_masked')
    expect(payload).not.toHaveProperty('has_key')
    expect(payload.name).toBe('主力 OpenAI 改名')
  })

  // 冷却到点后「已熔断」标记必须自动消失。此前用 Date.now() 判断，
  // 而它不是响应式源，chip 会一直挂着直到用户手动刷新。
  it('clears the tripped chip once the cooldown expires', async () => {
    vi.useFakeTimers()
    try {
      const cooldownUntil = Date.now() + 3000
      mocks.get.mockImplementation((url) => {
        if (url === '/channels') {
          return Promise.resolve([{ ...channelFixtures[0], cooldown_until: cooldownUntil }])
        }
        if (url === '/channel-types') return Promise.resolve([{ value: 1, name: 'OpenAI', default_base_url: 'https://api.openai.com' }])
        if (url === '/protocols') return Promise.resolve([])
        if (url === '/settings/test-prompt') return Promise.resolve({ prompt: 'ping' })
        if (url === '/settings/model-health') return Promise.resolve({})
        return Promise.resolve(null)
      })

      const wrapper = mountChannels()
      await vi.advanceTimersByTimeAsync(0)
      await flushPromises()
      expect(rows(wrapper)[0].text()).toContain('已熔断')

      // 推进到冷却结束之后，时钟 tick 应让状态自动恢复。
      await vi.advanceTimersByTimeAsync(6000)
      await flushPromises()
      expect(rows(wrapper)[0].text()).not.toContain('已熔断')
      expect(rows(wrapper)[0].text()).toContain('运行中')
      wrapper.unmount()
    } finally {
      vi.useRealTimers()
    }
  })

  // 并发 load：后发起的请求先返回时，旧响应不能覆盖新列表。
  // load 会被刷新按钮、save、removeChannel、resetBreaker 等多路调用，
  // 这里用「刷新 + 保存」构造交错完成的两次请求。
  it('discards stale channel list responses', async () => {
    const pending = []
    let callCount = 0
    mocks.get.mockImplementation((url) => {
      if (url === '/channels') {
        callCount += 1
        if (callCount === 1) {
          return Promise.resolve(channelFixtures.map((channel) => ({ ...channel })))
        }
        // 后续两次请求都挂起，由测试控制完成顺序。
        return new Promise((resolve) => { pending.push(resolve) })
      }
      if (url === '/channel-types') return Promise.resolve([{ value: 1, name: 'OpenAI', default_base_url: 'https://api.openai.com' }])
      if (url === '/protocols') return Promise.resolve([])
      if (url === '/settings/test-prompt') return Promise.resolve({ prompt: 'ping' })
      if (url === '/settings/model-health') return Promise.resolve({})
      return Promise.resolve(null)
    })

    const wrapper = mountChannels()
    await flushPromises()
    expect(rows(wrapper)).toHaveLength(2)

    // 模拟两条业务路径先后调用 load（save / removeChannel / resetBreaker / 刷新
    // 都会调它，且走不同的 loading 标志，彼此不互斥）。两次请求都挂起。
    wrapper.vm.load()
    await flushPromises()
    wrapper.vm.load()
    await flushPromises()
    expect(pending).toHaveLength(2)

    // 让「最新」的那次先返回，再让过期的那次返回。
    pending[1]([{ ...channelFixtures[0], name: '最新数据' }])
    await flushPromises()
    expect(rows(wrapper)[0].text()).toContain('最新数据')

    pending[0]([{ ...channelFixtures[0], name: '过期数据' }])
    await flushPromises()
    expect(rows(wrapper)[0].text()).toContain('最新数据')
    expect(wrapper.text()).not.toContain('过期数据')
    wrapper.unmount()
  })

  it('sends the key field when a new credential is entered', async () => {
    mocks.put.mockResolvedValue({ id: 7 })
    const wrapper = mountChannels()
    await flushPromises()

    await rows(wrapper)[0].trigger('click')
    await flushPromises()

    await wrapper.get('#channel-key').setValue('sk-rotated')
    await saveEditor(wrapper)

    const [, payload] = mocks.put.mock.calls[0]
    expect(payload.key).toBe('sk-rotated')
  })
})

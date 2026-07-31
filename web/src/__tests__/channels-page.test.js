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
  copyText: vi.fn().mockResolvedValue(true),
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
})

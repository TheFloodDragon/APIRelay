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

const modelConfigs = [
  { name: 'gpt-4o', enabled: true, protocol: '', upstream: '', input: 2.5, output: 10 },
  { name: 'gpt-4o-mini', enabled: true, protocol: '', upstream: '', input: 0.15, output: 0.6 },
  { name: 'o4-mini', enabled: false, protocol: 'responses', upstream: '', input: 1, output: 4 },
]

const channelFixture = {
  id: 7,
  name: '主力 OpenAI',
  type: 1,
  base_url: 'https://api.openai.com/v1',
  group: 'default',
  status: 1,
  weight: 3,
  model_configs: JSON.stringify(modelConfigs),
  model_health: {},
  has_key: true,
  key_masked: 'sk-1…cdef',
}

const PassThrough = { template: '<div><slot name="actions" /><slot /></div>' }
const DrawerStub = {
  props: ['open', 'title', 'persistent'],
  template: '<div v-if="open" class="drawer-stub"><slot /><div class="drawer-stub-footer"><slot name="footer" /></div></div>',
}

function mountChannels() {
  return mount(Channels, {
    global: {
      config: { globalProperties: { $toast: { add: vi.fn() } } },
      stubs: {
        BodyOverrideEditor: true,
        ChannelConsoleHeader: true,
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

// 打开编辑器并切到「模型与价格」页签。
async function openModelsTab(wrapper) {
  await wrapper.get('.channel-table tbody tr').trigger('click')
  await flushPromises()
  const modelsTab = wrapper.findAll('[role="tab"]').find((tab) => tab.text().includes('模型与价格'))
  await modelsTab.trigger('click')
  await flushPromises()
}

function modelRows(wrapper) {
  return wrapper.findAll('table[aria-label="模型配置表"] tbody tr')
}

function rowCheckbox(wrapper, index) {
  return modelRows(wrapper)[index].get('input[type="checkbox"]')
}

function headerCheckbox(wrapper) {
  return wrapper.get('table[aria-label="模型配置表"] thead input[type="checkbox"]')
}

function bulkButton(wrapper, label) {
  const found = wrapper.findAll('.model-bulk-bar button').find((button) => button.text().includes(label))
  if (!found) throw new Error(`bulk button ${label} not found`)
  return found
}

describe('channel model bulk operations', () => {
  beforeEach(() => {
    mocks.confirmAction.mockResolvedValue(true)
    mocks.get.mockImplementation((url) => {
      if (url === '/channels') return Promise.resolve([{ ...channelFixture }])
      if (url === '/channel-types') return Promise.resolve([{ value: 1, name: 'OpenAI', default_base_url: 'https://api.openai.com' }])
      if (url === '/protocols') {
        return Promise.resolve([
          { value: 'openai', name: 'OpenAI' },
          { value: 'anthropic', name: 'Anthropic' },
          { value: 'responses', name: 'OpenAI-Responses' },
        ])
      }
      if (url === '/settings/test-prompt') return Promise.resolve({ prompt: 'ping' })
      if (url === '/settings/model-health') return Promise.resolve({})
      return Promise.resolve(null)
    })
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('shows a checkbox per model row and no bulk bar until something is selected', async () => {
    const wrapper = mountChannels()
    await flushPromises()
    await openModelsTab(wrapper)

    expect(modelRows(wrapper)).toHaveLength(3)
    expect(wrapper.find('.model-bulk-bar').exists()).toBe(false)

    await rowCheckbox(wrapper, 0).setValue(true)
    expect(wrapper.get('.model-bulk-bar').text()).toContain('已选 1 / 3')
  })

  it('selects and clears every model from the table header', async () => {
    const wrapper = mountChannels()
    await flushPromises()
    await openModelsTab(wrapper)

    await headerCheckbox(wrapper).setValue(true)
    expect(wrapper.get('.model-bulk-bar').text()).toContain('已选 3 / 3')
    expect(headerCheckbox(wrapper).element.checked).toBe(true)

    await headerCheckbox(wrapper).setValue(false)
    expect(wrapper.find('.model-bulk-bar').exists()).toBe(false)
  })

  // 部分选中时表头应显示 indeterminate，而不是简单的未选中。
  it('marks the header checkbox indeterminate on a partial selection', async () => {
    const wrapper = mountChannels()
    await flushPromises()
    await openModelsTab(wrapper)

    await rowCheckbox(wrapper, 0).setValue(true)
    expect(headerCheckbox(wrapper).element.indeterminate).toBe(true)
    expect(headerCheckbox(wrapper).element.checked).toBe(false)

    await rowCheckbox(wrapper, 1).setValue(true)
    await rowCheckbox(wrapper, 2).setValue(true)
    expect(headerCheckbox(wrapper).element.indeterminate).toBe(false)
    expect(headerCheckbox(wrapper).element.checked).toBe(true)
  })

  it('enables and disables the selected models locally', async () => {
    const wrapper = mountChannels()
    await flushPromises()
    await openModelsTab(wrapper)

    // o4-mini 初始为停用状态。
    expect(wrapper.vm.models[2].enabled).toBe(false)

    await headerCheckbox(wrapper).setValue(true)
    await bulkButton(wrapper, '启用所选').trigger('click')
    await flushPromises()
    expect(wrapper.vm.models.every((model) => model.enabled)).toBe(true)
    // 纯本地改动，不应触达后端。
    expect(mocks.put).not.toHaveBeenCalled()
    expect(mocks.patch).not.toHaveBeenCalled()

    await bulkButton(wrapper, '停用所选').trigger('click')
    await flushPromises()
    expect(wrapper.vm.models.every((model) => !model.enabled)).toBe(true)
  })

  it('removes the selected models after confirmation and clears the selection', async () => {
    const wrapper = mountChannels()
    await flushPromises()
    await openModelsTab(wrapper)

    await rowCheckbox(wrapper, 0).setValue(true)
    await rowCheckbox(wrapper, 2).setValue(true)
    await bulkButton(wrapper, '移除所选').trigger('click')
    await flushPromises()

    expect(mocks.confirmAction).toHaveBeenCalled()
    const names = wrapper.vm.models.map((model) => model.name)
    expect(names).toEqual(['gpt-4o-mini'])
    // 选择集必须一并清空，否则「已选 N」会指向不存在的行。
    expect(wrapper.find('.model-bulk-bar').exists()).toBe(false)
    // 移除是本地编辑，不该立刻调用删除接口。
    expect(mocks.del).not.toHaveBeenCalled()
  })

  it('keeps the models untouched when removal is cancelled', async () => {
    mocks.confirmAction.mockResolvedValue(false)
    const wrapper = mountChannels()
    await flushPromises()
    await openModelsTab(wrapper)

    await headerCheckbox(wrapper).setValue(true)
    await bulkButton(wrapper, '移除所选').trigger('click')
    await flushPromises()

    expect(wrapper.vm.models).toHaveLength(3)
    expect(wrapper.get('.model-bulk-bar').text()).toContain('已选 3 / 3')
  })

  it('applies a protocol to every selected model', async () => {
    const wrapper = mountChannels()
    await flushPromises()
    await openModelsTab(wrapper)

    await rowCheckbox(wrapper, 0).setValue(true)
    await rowCheckbox(wrapper, 1).setValue(true)
    wrapper.vm.bulkProtocol = 'anthropic'
    await flushPromises()
    await wrapper.get('[data-bulk-apply="protocol"]').trigger('click')
    await flushPromises()

    expect(wrapper.vm.models[0].protocol).toBe('anthropic')
    expect(wrapper.vm.models[1].protocol).toBe('anthropic')
    // 未选中的行不受影响。
    expect(wrapper.vm.models[2].protocol).toBe('responses')
  })

  it('applies prices to every selected model', async () => {
    const wrapper = mountChannels()
    await flushPromises()
    await openModelsTab(wrapper)

    await headerCheckbox(wrapper).setValue(true)
    wrapper.vm.bulkPriceInput = '3'
    wrapper.vm.bulkPriceOutput = '12'
    await flushPromises()
    await wrapper.get('[data-bulk-apply="price"]').trigger('click')
    await flushPromises()

    expect(wrapper.vm.models.every((model) => model.input === 3 && model.output === 12)).toBe(true)
  })

  // 只填一个价格字段时，另一个必须保持原值而不是被清零。
  it('leaves the other price untouched when only one field is filled', async () => {
    const wrapper = mountChannels()
    await flushPromises()
    await openModelsTab(wrapper)

    await rowCheckbox(wrapper, 0).setValue(true)
    wrapper.vm.bulkPriceInput = '5'
    await flushPromises()
    await wrapper.get('[data-bulk-apply="price"]').trigger('click')
    await flushPromises()

    expect(wrapper.vm.models[0].input).toBe(5)
    expect(wrapper.vm.models[0].output).toBe(10) // 原值
  })

  it('rejects a negative bulk price without mutating anything', async () => {
    const wrapper = mountChannels()
    await flushPromises()
    await openModelsTab(wrapper)

    await headerCheckbox(wrapper).setValue(true)
    wrapper.vm.bulkPriceInput = '-1'
    await flushPromises()
    await wrapper.get('[data-bulk-apply="price"]').trigger('click')
    await flushPromises()

    expect(wrapper.vm.models[0].input).toBe(2.5)
  })

  // 「测试所选」只提交勾选的模型，且不要求它们处于启用状态 ——
  // 用户可能正想先验证再决定是否启用。
  it('tests only the selected models, including disabled ones', async () => {
    mocks.post.mockResolvedValue({ results: [], summary: { total: 1, success: 1, failed: 0 } })
    const wrapper = mountChannels()
    await flushPromises()
    await openModelsTab(wrapper)

    // 只勾选停用的 o4-mini。
    await rowCheckbox(wrapper, 2).setValue(true)
    await bulkButton(wrapper, '测试所选').trigger('click')
    await flushPromises()

    const call = mocks.post.mock.calls.find(([url]) => url === '/channels/test-batch')
    expect(call).toBeTruthy()
    expect(call[1].models).toEqual(['o4-mini'])
  })

  it('clears the selection from the bulk bar', async () => {
    const wrapper = mountChannels()
    await flushPromises()
    await openModelsTab(wrapper)

    await headerCheckbox(wrapper).setValue(true)
    await bulkButton(wrapper, '清空选择').trigger('click')
    await flushPromises()

    expect(wrapper.find('.model-bulk-bar').exists()).toBe(false)
    expect(wrapper.vm.models).toHaveLength(3)
  })

  // 关掉编辑器再打开时不能带着上一次的勾选。
  it('resets the selection when the editor is reopened', async () => {
    const wrapper = mountChannels()
    await flushPromises()
    await openModelsTab(wrapper)

    await headerCheckbox(wrapper).setValue(true)
    expect(wrapper.get('.model-bulk-bar').exists()).toBe(true)

    const cancel = wrapper.findAll('button').find((button) => button.text() === '取消')
    await cancel.trigger('click')
    await flushPromises()

    await openModelsTab(wrapper)
    expect(wrapper.find('.model-bulk-bar').exists()).toBe(false)
  })
})

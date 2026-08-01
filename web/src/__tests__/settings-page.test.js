import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import Settings from '../views/Settings.vue'

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  put: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { get: mocks.get, put: mocks.put },
}))

const PassThrough = { template: '<div><slot name="actions" /><slot /></div>' }

function mountSettings() {
  return mount(Settings, {
    global: {
      config: { globalProperties: { $toast: { add: vi.fn() } } },
      stubs: {
        ConsoleIcon: true,
        ConsoleSection: PassThrough,
        InlineNotice: PassThrough,
        PageHeader: PassThrough,
        PageState: PassThrough,
        SettingsNav: true,
      },
    },
  })
}

describe('Settings page', () => {
  beforeEach(() => {
    mocks.put.mockResolvedValue({})
    mocks.get.mockImplementation((url) => {
      switch (url) {
        case '/settings/protocol-rules':
          return Promise.resolve([
            { pattern: '^claude', protocol: 'anthropic' },
            { pattern: '^gpt', protocol: 'openai' },
          ])
        case '/protocols':
          return Promise.resolve([
            { value: 'openai', name: 'OpenAI' },
            { value: 'anthropic', name: 'Anthropic' },
          ])
        case '/settings/model-prices':
          return Promise.resolve([{ model: 'gpt-4o', input: 2.5, output: 10 }])
        case '/settings/circuit-breaker':
          return Promise.resolve({})
        case '/settings/logging':
          return Promise.resolve({})
        case '/settings/network':
          return Promise.resolve({ mode: 'system' })
        case '/settings/test-prompt':
          return Promise.resolve({ prompt: 'ping' })
        case '/settings/model-health':
          return Promise.resolve({})
        default:
          return Promise.resolve(null)
      }
    })
  })

  afterEach(() => {
    vi.clearAllMocks()
    vi.useRealTimers()
  })

  // 防抖 timer 必须在卸载时清掉。否则离开设置页后回调仍会执行：
  // 向已卸载组件的 ref 写值并发起 PUT，表现为「改完立刻切页发生看不见的保存」。
  // 先确认防抖机制确实存在（否则下一条卸载测试会因为「本来就不会保存」而假通过）。
  it('debounces the auto-save after an edit', async () => {
    vi.useFakeTimers()
    const wrapper = mountSettings()
    await vi.advanceTimersByTimeAsync(0)
    await flushPromises()
    mocks.put.mockClear()

    // 直接改状态触发 watch（对应的输入框只在激活 tab 内渲染）。
    wrapper.vm.testPrompt = 'changed prompt'
    await flushPromises()

    // 防抖窗口内不应发请求。
    await vi.advanceTimersByTimeAsync(200)
    expect(mocks.put).not.toHaveBeenCalled()

    // 超过防抖延迟后应发出保存请求。
    await vi.advanceTimersByTimeAsync(600)
    await flushPromises()
    expect(mocks.put).toHaveBeenCalled()
    wrapper.unmount()
  })

  // 防抖 timer 必须在卸载时清掉。否则离开设置页后回调仍会执行：
  // 向已卸载组件的 ref 写值并发起 PUT，表现为「改完立刻切页发生看不见的保存」。
  it('clears pending debounce timers on unmount', async () => {
    vi.useFakeTimers()
    const wrapper = mountSettings()
    await vi.advanceTimersByTimeAsync(0)
    await flushPromises()

    // 改一个字段触发防抖保存，但在延迟到达前就卸载组件。
    wrapper.vm.testPrompt = 'changed prompt'
    await flushPromises()
    mocks.put.mockClear()
    wrapper.unmount()

    // 推进到远超防抖延迟：已清理的 timer 不应再触发任何 PUT。
    await vi.advanceTimersByTimeAsync(5000)
    await flushPromises()
    expect(mocks.put).not.toHaveBeenCalled()
  })

  // 规则行用稳定 uid 作 key：上移/下移会交换位置，
  // 用 index 作 key 时 Vue 会复用错误的 DOM，把输入状态留在别的规则上。
  it('assigns stable row ids to protocol rules and prices', async () => {
    const wrapper = mountSettings()
    await flushPromises()

    const rules = wrapper.vm.rules
    const prices = wrapper.vm.prices
    expect(rules).toHaveLength(2)
    expect(prices).toHaveLength(1)

    // 每行都有 uid，且互不重复。
    const ruleUids = rules.map((rule) => rule._uid)
    expect(ruleUids.every(Boolean)).toBe(true)
    expect(new Set(ruleUids).size).toBe(ruleUids.length)
    expect(prices[0]._uid).toBeTruthy()
    // 规则与价格共用序列，uid 不能互相撞。
    expect(ruleUids).not.toContain(prices[0]._uid)
  })

  // 交换顺序后 uid 必须随行一起移动（这正是稳定 key 的意义）。
  it('keeps row ids attached to their rows when reordering', async () => {
    const wrapper = mountSettings()
    await flushPromises()

    const [firstUid, secondUid] = wrapper.vm.rules.map((rule) => rule._uid)
    const firstPattern = wrapper.vm.rules[0].pattern

    wrapper.vm.moveRule(0, 1)
    await flushPromises()

    const reordered = wrapper.vm.rules
    expect(reordered[0]._uid).toBe(secondUid)
    expect(reordered[1]._uid).toBe(firstUid)
    // uid 与内容保持绑定关系。
    expect(reordered[1].pattern).toBe(firstPattern)
  })

  // _uid 是纯前端字段，绝不能出现在提交给后端的 payload 里。
  it('strips row ids from submitted payloads', async () => {
    const wrapper = mountSettings()
    await flushPromises()

    const rulesPayload = wrapper.vm.rulesPayload()
    expect(rulesPayload).toHaveLength(2)
    rulesPayload.forEach((rule) => {
      expect(rule).not.toHaveProperty('_uid')
      expect(Object.keys(rule).sort()).toEqual(['pattern', 'protocol'])
    })

    const pricesPayload = wrapper.vm.pricesPayload()
    expect(pricesPayload).toHaveLength(1)
    expect(pricesPayload[0]).not.toHaveProperty('_uid')
    expect(Object.keys(pricesPayload[0]).sort()).toEqual(['input', 'model', 'output'])
  })

  // 新增行同样要带 uid，否则新行的 key 会是 undefined 并与其它 undefined 撞。
  it('assigns ids to newly added rows', async () => {
    const wrapper = mountSettings()
    await flushPromises()

    wrapper.vm.addRule()
    wrapper.vm.addPrice()
    await flushPromises()

    const newRule = wrapper.vm.rules[wrapper.vm.rules.length - 1]
    const newPrice = wrapper.vm.prices[wrapper.vm.prices.length - 1]
    expect(newRule._uid).toBeTruthy()
    expect(newPrice._uid).toBeTruthy()

    const allUids = [
      ...wrapper.vm.rules.map((rule) => rule._uid),
      ...wrapper.vm.prices.map((price) => price._uid),
    ]
    expect(new Set(allUids).size).toBe(allUids.length)
  })
})

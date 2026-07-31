import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import Logs from '../views/Logs.vue'

const mocks = vi.hoisted(() => ({
  apiGet: vi.fn(),
  downloadBlob: vi.fn(),
  confirmAction: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { get: mocks.apiGet },
  copyText: vi.fn(),
  cost: (value) => String(value || '—'),
  downloadBlob: mocks.downloadBlob,
  fmtTime: (value) => String(value || '—'),
  takeLatest: (fn) => fn,
}))

vi.mock('../composables/useConfirm', () => ({
  confirmAction: mocks.confirmAction,
}))

function mountLogs(toastAdd) {
  return mount(Logs, {
    global: {
      config: { globalProperties: { $toast: { add: toastAdd } } },
      stubs: {
        ConsoleIcon: true,
        LogDetailDrawer: true,
        LogFilterPanel: true,
        PageState: { template: '<div><slot /></div>' },
      },
    },
  })
}

function exportButton(wrapper, label) {
  return wrapper.findAll('button').find((button) => button.text().includes(label))
}

const expectedFilters = {
  start_time: new Date(2026, 6, 30, 12, 34, 56).getTime(),
  end_time: new Date(2026, 6, 31, 12, 34, 56).getTime(),
}

describe('Logs CSV export', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 6, 31, 12, 34, 56))
    mocks.apiGet.mockResolvedValue({ items: [], total: 0 })
    mocks.confirmAction.mockResolvedValue(true)
  })

  afterEach(() => {
    vi.clearAllMocks()
    vi.useRealTimers()
  })

  it('exports only current filters and exposes the exporting state', async () => {
    let finishDownload
    mocks.downloadBlob.mockReturnValue(new Promise((resolve) => { finishDownload = resolve }))
    const toastAdd = vi.fn()
    const wrapper = mountLogs(toastAdd)
    await flushPromises()

    const button = exportButton(wrapper, '导出摘要')
    await button.trigger('click')

    expect(button.attributes('disabled')).toBeDefined()
    expect(exportButton(wrapper, '导出完整报文').attributes('disabled')).toBeDefined()
    expect(button.text()).toContain('导出中…')
    expect(mocks.confirmAction).not.toHaveBeenCalled()
    expect(mocks.downloadBlob).toHaveBeenCalledOnce()

    const [url, filename, config] = mocks.downloadBlob.mock.calls[0]
    expect(url).toBe('/logs/export')
    expect(filename).toBe('apirelay-logs-20260731-123456.csv')
    expect(config.params).toEqual(expectedFilters)
    expect(config.params).not.toHaveProperty('page')
    expect(config.params).not.toHaveProperty('page_size')
    expect(config.params).not.toHaveProperty('include_payload')

    finishDownload()
    await flushPromises()
    expect(button.attributes('disabled')).toBeUndefined()
    expect(button.text()).toContain('导出摘要')
    expect(toastAdd).toHaveBeenCalledWith('日志 CSV 已导出', 'success')
  })

  it('confirms before exporting full payloads and requests them explicitly', async () => {
    mocks.downloadBlob.mockResolvedValue(undefined)
    const toastAdd = vi.fn()
    const wrapper = mountLogs(toastAdd)
    await flushPromises()

    await exportButton(wrapper, '导出完整报文').trigger('click')
    await flushPromises()

    expect(mocks.confirmAction).toHaveBeenCalledOnce()
    const [url, filename, config] = mocks.downloadBlob.mock.calls[0]
    expect(url).toBe('/logs/export')
    expect(filename).toBe('apirelay-logs-full-20260731-123456.csv')
    expect(config.params).toEqual({ ...expectedFilters, include_payload: true })
    expect(toastAdd).toHaveBeenCalledWith('完整日志 CSV 已导出', 'success')
  })

  it('skips the full export when the confirmation is declined', async () => {
    mocks.confirmAction.mockResolvedValue(false)
    const toastAdd = vi.fn()
    const wrapper = mountLogs(toastAdd)
    await flushPromises()

    await exportButton(wrapper, '导出完整报文').trigger('click')
    await flushPromises()

    expect(mocks.downloadBlob).not.toHaveBeenCalled()
    expect(toastAdd).not.toHaveBeenCalled()
    expect(exportButton(wrapper, '导出完整报文').attributes('disabled')).toBeUndefined()
  })

  it('restores the button and shows an error toast when export fails', async () => {
    mocks.downloadBlob.mockRejectedValue(new Error('导出范围过大'))
    const toastAdd = vi.fn()
    const wrapper = mountLogs(toastAdd)
    await flushPromises()

    const button = exportButton(wrapper, '导出摘要')
    await button.trigger('click')
    await flushPromises()

    expect(button.attributes('disabled')).toBeUndefined()
    expect(button.text()).toContain('导出摘要')
    expect(toastAdd).toHaveBeenCalledWith('导出范围过大', 'error')
  })
})

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import api, { downloadBlob } from '../api'

describe('downloadBlob', () => {
  const createObjectURL = vi.fn(() => 'blob:apirelay-test')
  const revokeObjectURL = vi.fn()

  beforeEach(() => {
    vi.stubGlobal('URL', { createObjectURL, revokeObjectURL })
    document.body.innerHTML = ''
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('downloads through the authenticated api instance and revokes the object URL', async () => {
    const blob = new Blob(['id,status\n1,200'], { type: 'text/csv' })
    vi.spyOn(api, 'request').mockResolvedValue(blob)
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})

    await downloadBlob('/logs/export', 'apirelay-logs.csv', { params: { model: 'gpt-4o' } })

    expect(api.request).toHaveBeenCalledWith({
      method: 'get',
      params: { model: 'gpt-4o' },
      url: '/logs/export',
      responseType: 'blob',
    })
    expect(createObjectURL).toHaveBeenCalledWith(blob)
    expect(click).toHaveBeenCalledOnce()
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:apirelay-test')
    expect(document.querySelector('a[download]')).toBeNull()
  })

  it('extracts a JSON message from Blob error responses', async () => {
    const rejected = api.interceptors.response.handlers[0].rejected
    const error = {
      response: {
        status: 400,
        data: new Blob([JSON.stringify({ message: '导出范围过大' })], { type: 'application/json' }),
      },
      message: 'Request failed',
    }

    await expect(rejected(error)).rejects.toThrow('导出范围过大')
  })
})

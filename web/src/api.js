import axios from 'axios'
import router from './router'

const TOKEN_KEY = 'apirelay_session'

const api = axios.create({ baseURL: '/api' })

// 请求拦截：注入会话令牌
api.interceptors.request.use((config) => {
  const token = localStorage.getItem(TOKEN_KEY)
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 响应拦截：401 跳登录；统一解包 {success,data,message}，直接返回 data 字段
api.interceptors.response.use(
  (resp) => {
    const body = resp.data
    if (body && body.success === false) {
      return Promise.reject(new Error(body.message || '请求失败'))
    }
    return body && typeof body === 'object' && 'data' in body ? body.data : body
  },
  (err) => {
    if (err.response && err.response.status === 401) {
      localStorage.removeItem(TOKEN_KEY)
      if (router.currentRoute.value.name !== 'login') {
        router.push({ name: 'login' })
      }
    }
    const msg = err.response?.data?.message || err.message || '网络错误'
    return Promise.reject(new Error(msg))
  }
)

/**
 * 退出登录：先请求后端吊销会话，再清除本地令牌。
 * 吊销失败也照常登出（令牌随 24h 过期兜底）。
 */
export async function logout() {
  try {
    await api.post('/auth/logout')
  } catch {
    /* 已尽力吊销 */
  }
  localStorage.removeItem(TOKEN_KEY)
  router.push({ name: 'login' })
}

/**
 * 竞态守卫：包装一个异步加载函数，只有「最后一次发起」的结果会被采纳。
 * 用于列表页快速翻页/连续筛选时丢弃过期响应。
 *
 * const load = takeLatest(async (params) => { ... return data })
 * const data = await load(params)  // 若期间又发起了新调用，resolve(undefined)
 */
export function takeLatest(fn) {
  let seq = 0
  return async (...args) => {
    const my = ++seq
    const result = await fn(...args)
    if (my !== seq) return undefined // 已被更新的调用取代
    return result
  }
}

/**
 * 复制文本到剪贴板，带 execCommand 降级（HTTP 内网环境 clipboard API 不可用）。
 * 返回是否成功。
 */
export async function copyText(text) {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text)
      return true
    }
  } catch {
    /* 走降级 */
  }
  try {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.position = 'fixed'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.select()
    const okFlag = document.execCommand('copy')
    document.body.removeChild(ta)
    return okFlag
  } catch {
    return false
  }
}

/** 微美元 → 美元字符串（固定 4 位小数） */
export function usd(microUSD) {
  const v = (Number(microUSD) || 0) / 1_000_000
  return '$' + v.toFixed(4)
}

/** 毫秒时间戳 → "MM-DD HH:mm:ss" */
export function fmtTime(ms) {
  if (!ms) return '—'
  const d = new Date(ms)
  const p = (n) => String(n).padStart(2, '0')
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

export { TOKEN_KEY }
export default api

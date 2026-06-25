import axios from 'axios'
import router from './router'

const api = axios.create({ baseURL: '/api' })

// 请求拦截：注入会话令牌
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('session_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 响应拦截：401 跳登录；统一解包 {success,data,message}
api.interceptors.response.use(
  (resp) => {
    const body = resp.data
    if (body && body.success === false) {
      return Promise.reject(new Error(body.message || '请求失败'))
    }
    return body && 'data' in body ? body.data : body
  },
  (err) => {
    if (err.response && err.response.status === 401) {
      localStorage.removeItem('session_token')
      if (router.currentRoute.value.name !== 'login') {
        router.push({ name: 'login' })
      }
    }
    const msg = err.response?.data?.message || err.message || '网络错误'
    return Promise.reject(new Error(msg))
  }
)

export default api

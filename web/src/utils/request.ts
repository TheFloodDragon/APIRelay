import axios from 'axios'

const request = axios.create({
  baseURL: '/api',
  timeout: 30000
})

request.interceptors.request.use((config) => {
  const adminKey = localStorage.getItem('apirelay_admin_key') || 'change-me-in-production'
  config.headers.Authorization = `Bearer ${adminKey}`
  return config
})

export default request

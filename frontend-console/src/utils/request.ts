import axios from 'axios'
import { useUserStore } from '@/stores/user'

const request = axios.create({
  baseURL: '/console',
  timeout: 10000
})

// 请求拦截器
request.interceptors.request.use(
  (config) => {
    const userStore = useUserStore()
    // 普通用户接口使用 token
    if (userStore.token) {
      config.headers.Authorization = `Bearer ${userStore.token}`
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// 响应拦截器
request.interceptors.response.use(
  (response) => {
    const { code, message, data } = response.data
    if (code === 0) {
      return data
    } else {
      // 业务错误：由各调用方按操作弹一次错误提示（避免与拦截器重复弹窗）
      return Promise.reject(new Error(message || '请求失败'))
    }
  },
  (error) => {
    // HTTP 错误（网络错误、服务器错误等）
    if (error.response?.status === 401) {
      const userStore = useUserStore()
      userStore.logout()
      window.location.href = '/login'
    }
    // 错误提示由各调用方统一弹出
    return Promise.reject(error)
  }
)

// 响应拦截器已解包 data 返回业务数据（而非 AxiosResponse），
// 因此对 request 实例做类型收窄，避免 vue-tsc 误判返回类型。
const typedRequest = request as unknown as {
  get<T = any>(url: string, config?: any): Promise<T>
  post<T = any>(url: string, data?: any, config?: any): Promise<T>
  put<T = any>(url: string, data?: any, config?: any): Promise<T>
  delete<T = any>(url: string, config?: any): Promise<T>
}

export default typedRequest

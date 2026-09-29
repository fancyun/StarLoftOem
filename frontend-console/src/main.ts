import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus, { ElMessage } from 'element-plus'
import 'element-plus/dist/index.css'
import '@/assets/main.css'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import App from './App.vue'
import router from './router'

// 全局报错弹窗去重：请求拦截器已统一弹出业务错误，页面 catch 又会再弹一次，
// 导致一次操作出现 2~3 个重复报错。此处对相同错误消息做 1.5s 去重，仅保留一个。
const _elMessageError = ElMessage.error.bind(ElMessage)
const recentErrorMessages = new Map<string, number>()
ElMessage.error = ((message: any, options?: any) => {
  const key = String(message)
  const now = Date.now()
  const last = recentErrorMessages.get(key)
  if (last && now - last < 1500) {
    return // 短时间内重复的相同消息直接忽略
  }
  recentErrorMessages.set(key, now)
  // 防止 Map 无限增长
  if (recentErrorMessages.size > 50) {
    recentErrorMessages.clear()
  }
  return _elMessageError(message, options)
}) as typeof ElMessage.error

const app = createApp(App)

// 注册所有图标
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component)
}

app.use(createPinia())
app.use(router)
app.use(ElementPlus)

app.mount('#app')

import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import '@/assets/main.css'
import { applyTitle, loadBrand } from '@/utils/brand'

const app = createApp(App)
app.use(router)
app.mount('#app')

// 品牌由后端公开配置下发：加载完成后刷新标签标题（取不到时用默认品牌名）
loadBrand().then(() => applyTitle())
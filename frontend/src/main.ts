import { createApp } from 'vue'
import App from './App.vue'
import { usePrefs } from './composables/usePrefs'

// Element Plus：亮色 + 暗色主题变量
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/dark/css-vars.css'
// 本应用主题桥接（必须在 EP 样式之后，才能覆盖 --el-* 深色变量）
import './theme.css'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'

const app = createApp(App)

// 全局注册所有 Element Plus 图标为 <el-icon-xxx> 形式
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component)
}

// 初始化界面偏好（语言 + 主题：dark/light/auto），从 localStorage 恢复并应用到 <html>。
usePrefs()

app.mount('#app')

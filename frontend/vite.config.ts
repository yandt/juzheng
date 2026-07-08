import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import wails from "@wailsio/runtime/plugins/vite";
import AutoImport from "unplugin-auto-import/vite";
import Components from "unplugin-vue-components/vite";
import { ElementPlusResolver } from "unplugin-vue-components/resolvers";
import { fileURLToPath, URL } from "node:url";
import type { Plugin } from "vite";

// 调试模式下让 vite 采取整页 reload：源码改动时强制刷新整个页面，
// 而非 HMR 局部热替换。HMR 在 Wails 的 WKWebView 里可能残留旧状态/注入，
// 整页 reload 更可靠（app 已完成首屏加载，此 reload 不与启动期依赖优化竞态冲突）。
function fullReloadOnChange(): Plugin {
  return {
    name: "wails-full-reload",
    apply: "serve", // 仅 dev（wails3 dev）生效，生产 build 不受影响
    handleHotUpdate({ server }) {
      server.ws.send({ type: "full-reload" });
      return []; // 吞掉默认 HMR，改为整页 reload
    },
  };
}

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  // 预打包 Element Plus 及其按需组件样式：否则 vite 在首屏运行时才发现这些
  // 依赖并触发一次整页 reload。普通浏览器能扛住这次 reload，但 Wails 的
  // WKWebView 连接后被 reload 打断（binding 请求 "request has been stopped"），
  // 页面渲染出来却丢失交互（按钮点击无响应）。提前 include 让优化在 dev server
  // 启动阶段完成，首屏加载后不再 reload。
  //
  // 显式列全所有用到的组件样式（含 collapse-item/tooltip 等只在嵌套渲染时才被
  // 发现的组件）——通配符 include 会漏掉这些延迟发现的依赖，仍触发 reload。
  optimizeDeps: {
    include: [
      "element-plus/es",
      "element-plus/es/components/*/style/css",
      "element-plus/es/components/alert/style/css",
      "element-plus/es/components/aside/style/css",
      "element-plus/es/components/base/style/css",
      "element-plus/es/components/button/style/css",
      "element-plus/es/components/card/style/css",
      "element-plus/es/components/checkbox/style/css",
      "element-plus/es/components/collapse/style/css",
      "element-plus/es/components/collapse-item/style/css",
      "element-plus/es/components/container/style/css",
      "element-plus/es/components/dialog/style/css",
      "element-plus/es/components/divider/style/css",
      "element-plus/es/components/empty/style/css",
      "element-plus/es/components/form/style/css",
      "element-plus/es/components/form-item/style/css",
      "element-plus/es/components/icon/style/css",
      "element-plus/es/components/input/style/css",
      "element-plus/es/components/input-number/style/css",
      "element-plus/es/components/main/style/css",
      "element-plus/es/components/menu/style/css",
      "element-plus/es/components/menu-item/style/css",
      "element-plus/es/components/option/style/css",
      "element-plus/es/components/progress/style/css",
      "element-plus/es/components/radio/style/css",
      "element-plus/es/components/radio-group/style/css",
      "element-plus/es/components/select/style/css",
      "element-plus/es/components/switch/style/css",
      "element-plus/es/components/tabs/style/css",
      "element-plus/es/components/tab-pane/style/css",
      "element-plus/es/components/tag/style/css",
      "element-plus/es/components/tooltip/style/css",
    ],
  },
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
      "@types": fileURLToPath(new URL("./src/types", import.meta.url)),
      "@bindings": fileURLToPath(new URL("./bindings", import.meta.url)),
    },
  },
  plugins: [
    vue(),
    wails("./bindings"),
    fullReloadOnChange(),
    // Element Plus 按需自动导入（组件 + ElMessage/ElMessageBox 等 API）
    AutoImport({
      resolvers: [ElementPlusResolver()],
      // 生成的声明文件，供 TS 识别自动导入的符号
      dts: "src/auto-imports.d.ts",
    }),
    Components({
      resolvers: [ElementPlusResolver()],
      dts: "src/components.d.ts",
    }),
  ],
});

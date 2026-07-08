// composables/service/useApp.ts — 应用元信息（收口 appservice binding）。
//
// AppSidebar 直接 import appservice binding 调用 GetAppInfo，收口到本 composable。
// 单例：appInfo 模块级 ref，首次 use 时加载。

import { ref } from 'vue'
import * as app from '@bindings/github.com/zhanghui/juzheng/appservice'

interface AppInfo {
  appVersion: string
  os: string
  arch: string
  isDev: boolean
}

const appInfo = ref<AppInfo>({ appVersion: '', os: '', arch: '', isDev: false })
let loaded = false

async function loadAppInfo() {
  if (loaded) return
  loaded = true
  try {
    const info = await app.GetAppInfo() as any
    appInfo.value = {
      appVersion: info.appVersion,
      os: info.os,
      arch: info.arch,
      isDev: !!info.isDev,
    }
  } catch { /* 静默 */ }
}

export function useApp() {
  return { appInfo, loadAppInfo }
}

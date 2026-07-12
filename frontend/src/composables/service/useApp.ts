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

// 本机局域网 IPv4 地址（供「对外代理服务」卡片显示,提示其他设备指向哪个 IP）。
const lanAddresses = ref<string[]>([])

async function loadLanAddresses() {
  try {
    lanAddresses.value = (await app.GetLANAddresses()) ?? []
  } catch { lanAddresses.value = [] }
}

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
  return { appInfo, loadAppInfo, lanAddresses, loadLanAddresses }
}

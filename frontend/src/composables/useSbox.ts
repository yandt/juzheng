// useSbox: sing-box 内核与 helper 守护进程的状态与操作（单例 composable）。
// 持有 helperStatus/sboxRunning/配置（结构化 + JSON 双形态），供节点/规则/设置/源码页复用。

import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Events } from '@wailsio/runtime'
import { t } from '../i18n'
import * as singbox from '../../bindings/github.com/zhanghui/juzheng/singboxservice'
import * as sysproxy from '../../bindings/github.com/zhanghui/juzheng/sysproxyservice'
import * as subscription from '../../bindings/github.com/zhanghui/juzheng/subscriptionservice'
import { parseConfig, serializeConfig, type SingBoxConfig, blankConfig } from '../configModel'
import { useSubscriptions } from './useSubscriptions'

// 单例状态
const helperStatus = ref<any>({ installed: false, running: false, loaded: false, pid: 0, installCmd: '' })
const sboxRunning = ref(false)
const sboxBusy = ref(false)
const helperBusy = ref(false)

// 系统代理状态
const sysProxyEnabled = ref(false)
const sysProxyBusy = ref(false)

// 配置：结构化（图形编辑用）+ JSON 字符串（源码编辑用）
const config = ref<SingBoxConfig>(blankConfig())
const configJson = ref('')          // 源码模式显示的 JSON
const configDirty = ref(false)      // 是否有未保存改动（自动保存前的短暂窗口）
const configError = ref('')         // 解析/保存错误
const saving = ref(false)           // 自动保存进行中（供状态指示）

// 即改即存：编辑后防抖自动保存，无需手动点保存按钮。
const AUTO_SAVE_DELAY = 600
let autoSaveTimer: ReturnType<typeof setTimeout> | null = null
// 本次待保存是否需要热重载运行中的内核（路由/抓包域名/DNS 等结构性改动需要；
// 节点切换、代理模式走 clash_api 已实时生效，无需重载）。保存成功后触发一次。
let pendingReload = false
function scheduleAutoSave(fromJson = false) {
  if (autoSaveTimer) clearTimeout(autoSaveTimer)
  autoSaveTimer = setTimeout(async () => {
    autoSaveTimer = null
    saving.value = true
    try { await saveConfig(fromJson, { silent: true }) } finally { saving.value = false }
  }, AUTO_SAVE_DELAY)
}

let cancelStart: (() => void) | null = null
let cancelStop: (() => void) | null = null
// 模块级 init flag：防止 v-show 常驻页面重复订阅事件（替代原 onMounted 方案）。
let initialized = false

// 系统代理状态（模块级单例，原在 useSbox 函数体内导致每页独立副本，现移此处保证一致）。
const sysProxyState = ref<any>({
  enabled: false, server: '127.0.0.1', port: 9788,
  enableHttp: true, enableHttps: true, enableSocks: true,
  bypass: ['127.0.0.1', '192.168.0.0/16', '10.0.0.0/8', '172.16.0.0/12', 'localhost', '*.local'],
})

async function refreshHelperStatus() {
  try { helperStatus.value = await singbox.GetHelperStatus() } catch { /* 不可达 */ }
}

async function installHelper() {
  helperBusy.value = true
  try {
    await singbox.InstallHelper()
    await refreshHelperStatus()
    // helper 安装后 launchd 启动进程需要几秒，轮询等待 Running=true
    for (let i = 0; i < 5 && !helperStatus.value.running; i++) {
      await new Promise(r => setTimeout(r, 1000))
      await refreshHelperStatus()
    }
    if (helperStatus.value.running) ElMessage.success(t('msg.tunInstalledRunning'))
    else if (helperStatus.value.installed) ElMessage.warning(t('msg.tunInstalledNotReady'))
    else ElMessage.warning(t('msg.installIncomplete'))
  } catch (e: any) {
    ElMessage.error(t('msg.helperInstallFailed', { error: e?.message || e }))
  } finally { helperBusy.value = false }
}

async function uninstallHelper() {
  helperBusy.value = true
  try {
    await singbox.UninstallHelper()
    await refreshHelperStatus()
    sboxRunning.value = false
    ElMessage.success(t('msg.tunUninstalled'))
  } catch (e: any) {
    ElMessage.error(t('msg.uninstallFailed', { error: e?.message || e }))
  } finally { helperBusy.value = false }
}

async function startSbox() {
  if (!helperStatus.value.running) {
    ElMessage.warning(t('msg.tunNotRunning'))
    return
  }
  sboxBusy.value = true
  try {
    await singbox.Start()
    sboxRunning.value = true
    ElMessage.success(t('msg.sboxStarted'))
  } catch (e: any) {
    ElMessage.error(t('msg.startFailed', { error: e?.message || e }))
  } finally { sboxBusy.value = false }
}

async function stopSbox() {
  sboxBusy.value = true
  try {
    await singbox.Stop()
    sboxRunning.value = false
  } catch (e: any) {
    ElMessage.error(t('msg.stopFailed', { error: e?.message || e }))
  } finally { sboxBusy.value = false }
}

// 从后端加载配置 → 解析成结构化 + 同步 JSON
async function loadConfig() {
  configError.value = ''
  try {
    const json = await singbox.GetConfig() as unknown as string
    configJson.value = json
    const parsed = parseConfig(json)
    if (parsed) {
      config.value = parsed
      configDirty.value = false
    } else {
      configError.value = t('msg.configParseFailed')
    }
  } catch (e: any) {
    configError.value = t('msg.loadFailed', { error: e?.message || e })
  }
}

// 保存配置。persistJson=true 时用 configJson 直接保存（源码模式），
// 否则把结构化 config 序列化后保存（图形模式）。
// silent=true 时不弹「配置已保存」toast（自动保存用，避免频繁打扰；错误仍提示）。
async function saveConfig(fromJson = false, opts: { silent?: boolean } = {}) {
  configError.value = ''
  let toSave = ''
  if (fromJson) {
    toSave = configJson.value
    try { JSON.parse(toSave) } catch (e: any) {
      configError.value = t('msg.jsonSyntaxError', { error: e.message }); return
    }
  } else {
    toSave = serializeConfig(config.value)
    configJson.value = toSave // 同步源码视图
  }
  try {
    await singbox.SetConfig(toSave)
    // 同时回写活动订阅文件，避免切换订阅再切回时改动丢失
    const { activeSubscription } = useSubscriptions()
    if (activeSubscription.value) {
      try { await subscription.SetSubscription(activeSubscription.value, toSave) } catch { /* 回写失败不阻断 */ }
    }
    configDirty.value = false
    if (!opts.silent) ElMessage.success(t('msg.configSaved'))
    // 结构性改动（route/DNS/抓包域名等）保存后热重载运行中的内核，使其即时生效。
    if (pendingReload && sboxRunning.value) {
      pendingReload = false
      try {
        await singbox.ReloadConfig()
        ElMessage.success(t('msg.appliedLive'))
      } catch (e: any) {
        ElMessage.warning(t('msg.reloadFailed', { error: e?.message || e }))
      }
    } else {
      pendingReload = false
    }
  } catch (e: any) {
    configError.value = t('msg.saveFailed', { error: e?.message || e })
  }
}

// 图形化编辑后：标记 dirty + 同步 JSON 预览 + 触发防抖自动保存（即改即存）。
// reload=true（默认）：改动需内核重载生效（route/DNS/抓包域名等）。
// reload=false：节点切换、代理模式——已走 clash_api 实时生效，无需重载。
function markDirty(reload = true) {
  configDirty.value = true
  configJson.value = serializeConfig(config.value)
  if (reload) pendingReload = true
  scheduleAutoSave(false)
}

// 源码编辑后：尝试解析回结构化。解析成功才自动保存（避免把语法错误的 JSON 写盘）。
function syncFromJson() {
  configDirty.value = true
  const parsed = parseConfig(configJson.value)
  if (parsed) {
    config.value = parsed
    configError.value = ''
    pendingReload = true   // 源码整体改动，视为需重载
    scheduleAutoSave(true)
  } else {
    configError.value = t('msg.jsonParseFailed')
  }
}

export function useSbox() {
  // 事件订阅 + 初始加载只在首次 import 时执行一次（模块级 init flag），
  // 避免每个使用 useSbox 的组件 setup 时重复订阅事件（v-show 常驻 6 页都会 setup）。
  // 状态本身是模块级单例 ref，跨组件共享。
  if (!initialized) {
    initialized = true
    cancelStart = Events.On('singbox:started', () => { sboxRunning.value = true })
    cancelStop = Events.On('singbox:stopped', () => { sboxRunning.value = false })
    // 异步初始化（不阻塞渲染）
    ;(async () => {
      try {
        await refreshHelperStatus()
        // helper 已安装但 ping 不通时，重试几次（launchd 刚拉起进程可能还没就绪）
        for (let i = 0; i < 3 && helperStatus.value.installed && !helperStatus.value.running; i++) {
          await new Promise(r => setTimeout(r, 1000))
          await refreshHelperStatus()
        }
        sboxRunning.value = await singbox.IsRunning().catch(() => false) as unknown as boolean
        await loadConfig()
      } catch { /* 静默 */ }
    })()
  }

  // 刷新系统代理状态（含设置项）。
  // 关键：只有当 macOS 系统代理确实指向【本 app 的端口】时，才算"本 app 已启用"。
  // 否则别的应用（如 Clash 用别的端口）开了系统代理，不应被误判为本 app 启用。
  async function refreshSysProxy() {
    try {
      const st = await sysproxy.GetSystemProxy() as any
      if (st) {
        const ourPort = config.value.settings?.mixedBackPort || st.port
        const localServer = st.server === '127.0.0.1' || st.server === 'localhost'
        const isOurs = !!st.enabled && localServer && Number(st.port) === Number(ourPort)
        // 端口用本 app 端口（供开启时正确设置），enabled 反映"是否本 app 的代理"。
        sysProxyState.value = { ...st, port: ourPort, enabled: isOurs }
        sysProxyEnabled.value = isOurs
      }
    } catch { /* 静默 */ }
  }
  // 切换系统代理开关。端口统一取内核代理端口（config.settings.mixedBackPort），
  // 保证系统代理目标 = 内核实际监听端口。
  async function toggleSysProxy(enabled: boolean) {
    sysProxyBusy.value = true
    try {
      const port = config.value.settings?.mixedBackPort || sysProxyState.value.port
      const state = { ...sysProxyState.value, port, server: '127.0.0.1', enabled }
      await sysproxy.SetSystemProxy(state)
      sysProxyState.value = state
      sysProxyEnabled.value = enabled
      ElMessage.success(enabled ? t('msg.sysProxyOn') : t('msg.sysProxyOff'))
    } catch (e: any) {
      ElMessage.error(t('msg.sysProxySetFailed', { error: e?.message || e }))
    } finally {
      sysProxyBusy.value = false
    }
  }
  // 应用系统代理设置（弹窗保存时调用）。端口统一取内核代理端口。
  async function applySysProxySettings() {
    sysProxyBusy.value = true
    try {
      const port = config.value.settings?.mixedBackPort || sysProxyState.value.port
      const state = { ...sysProxyState.value, port, server: '127.0.0.1' }
      await sysproxy.SetSystemProxy(state)
      sysProxyState.value = state
      sysProxyEnabled.value = !!state.enabled
      ElMessage.success(t('msg.sysProxyApplied'))
    } catch (e: any) {
      ElMessage.error(t('msg.sysProxySetFailed', { error: e?.message || e }))
    } finally {
      sysProxyBusy.value = false
    }
  }

  return {
    helperStatus, sboxRunning, sboxBusy, helperBusy,
    sysProxyEnabled, sysProxyBusy, sysProxyState,
    config, configJson, configDirty, configError, saving,
    refreshHelperStatus, installHelper, uninstallHelper,
    startSbox, stopSbox, loadConfig, saveConfig, markDirty, syncFromJson,
    refreshSysProxy, toggleSysProxy, applySysProxySettings,
  }
}

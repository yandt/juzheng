// useSubscriptions: 多订阅管理（单例 composable）。
// 持有订阅列表 + 活动订阅，供 HomePage/SubscriptionsPage 复用。

import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Dialogs } from '@wailsio/runtime'
import { t } from '../i18n'
import * as subscription from '../../bindings/github.com/zhanghui/juzheng/subscriptionservice'
import type { SubscriptionMeta } from '../../bindings/github.com/zhanghui/juzheng/models'
import type { Info as SubscriptionInfo } from '../../bindings/github.com/zhanghui/juzheng/internal/subscriptions/models'

// ===== 用量/到期格式化工具 =====

// 字节转可读 (GB/MB)
export function formatBytes(b: number | undefined): string {
  if (!b || b <= 0) return '0'
  const gb = b / 1024 / 1024 / 1024
  if (gb >= 1) return gb.toFixed(2) + ' GB'
  const mb = b / 1024 / 1024
  return mb.toFixed(0) + ' MB'
}

// 用量百分比（已用/总）
export function usagePercent(info: SubscriptionInfo | null | undefined): number {
  if (!info || !info.total) return 0
  return Math.min(100, Math.round(((info.upload + info.download) / info.total) * 100))
}

// 到期时间格式化
export function formatExpire(ts: number | undefined): string {
  if (!ts) return ''
  const d = new Date(ts * 1000)
  if (isNaN(d.getTime())) return ''
  return d.toLocaleDateString('zh-CN')
}

// 到期剩余天数（负数=已过期）
export function daysUntilExpire(ts: number | undefined): number | null {
  if (!ts) return null
  const diff = ts * 1000 - Date.now()
  return Math.ceil(diff / 86400000)
}

// 单例状态
const subscriptions = ref<SubscriptionMeta[]>([])
const activeSubscription = ref<string>('')

async function refresh() {
  try {
    subscriptions.value = (await subscription.ListSubscriptions() as unknown as SubscriptionMeta[]) ?? []
    activeSubscription.value = await subscription.GetActiveSubscription() as unknown as string
  } catch { /* 静默 */ }
}

// 切换活动订阅（拷到运行副本 + 提示重启）
async function setActive(name: string) {
  try {
    await subscription.SetActiveSubscription(name)
    activeSubscription.value = name
    await refresh()
    ElMessage.success(t('msg.switchedTo', { name }))
  } catch (e: any) {
    ElMessage.error(t('msg.switchFailed', { error: e?.message || e }))
  }
}

// 新建订阅（弹窗输名，复制当前活动订阅）
async function createSubscription(source: string = 'active') {
  try {
    const { value } = await ElMessageBox.prompt(t('msg.enterSubName'), t('msg.newSubTitle'), {
      confirmButtonText: t('msg.create'),
      cancelButtonText: t('msg.cancel'),
      inputPattern: /^[^/\\:*?"<>|]+$/,
      inputErrorMessage: t('msg.subNameInvalid'),
    })
    if (!value) return
    await subscription.CreateSubscriptionFromTemplate(value, source)
    await refresh()
    ElMessage.success(t('msg.subCreated'))
  } catch (e: any) {
    if (e !== 'cancel' && e?.message !== 'cancel') {
      ElMessage.error(t('msg.createFailed', { error: e?.message || e }))
    }
  }
}

// 删除订阅
async function removeSubscription(name: string) {
  try {
    await ElMessageBox.confirm(t('msg.confirmDeleteSub', { name }), t('msg.deleteSubTitle'), { type: 'warning' })
    await subscription.DeleteSubscription(name)
    await refresh()
    ElMessage.success(t('msg.deleted'))
  } catch (e: any) {
    if (e !== 'cancel' && e?.message !== 'cancel') {
      ElMessage.error(t('msg.deleteFailed', { error: e?.message || e }))
    }
  }
}

// 重命名订阅
async function renameSubscription(old: string) {
  try {
    const { value } = await ElMessageBox.prompt(t('msg.enterSubName'), t('msg.renameSubTitle'), {
      confirmButtonText: t('msg.confirm'),
      cancelButtonText: t('msg.cancel'),
      inputValue: old,
      inputPattern: /^[^/\\:*?"<>|]+$/,
      inputErrorMessage: t('msg.subNameInvalidShort'),
    })
    if (!value || value === old) return
    await subscription.RenameSubscription(old, value)
    await refresh()
    ElMessage.success(t('msg.renamed'))
  } catch (e: any) {
    if (e !== 'cancel' && e?.message !== 'cancel') {
      ElMessage.error(t('msg.renameFailed', { error: e?.message || e }))
    }
  }
}

// 刷新单个订阅的用量信息（重新请求 URL）。返回是否成功，供 view 控制 loading。
async function refreshInfo(name: string): Promise<boolean> {
  try {
    await subscription.RefreshSubscriptionInfo(name)
    await refresh()
    ElMessage.success(t('msg.usageRefreshed'))
    return true
  } catch (e: any) {
    ElMessage.error(t('msg.refreshFailed', { error: e?.message || e }))
    return false
  }
}

// 本地文件导入：弹原生文件选择器 → 导入 → 刷新。返回是否完成导入。
async function importFromFile(name: string): Promise<boolean> {
  const selected = await Dialogs.OpenFile({
    title: t('msg.selectSubFile'),
    canChooseFiles: true,
    allowsMultipleSelection: false,
    filters: [{ displayName: t('msg.configFile'), pattern: '*.json;*.yaml;*.yml;*.txt' }],
  } as any)
  const path = Array.isArray(selected) ? selected[0] : selected
  if (!path) return false
  try {
    await subscription.ImportSubscriptionFromFile(name, path as string)
    await refresh()
    ElMessage.success(t('msg.importSuccess'))
    return true
  } catch (e: any) {
    ElMessage.error(t('msg.importFailed', { error: e?.message || e }))
    return false
  }
}

// 订阅 URL 导入。
async function importFromUrl(name: string, url: string): Promise<boolean> {
  try {
    await subscription.ImportSubscriptionFromURL(name, url)
    await refresh()
    ElMessage.success(t('msg.importSuccess'))
    return true
  } catch (e: any) {
    ElMessage.error(t('msg.importFailed', { error: e?.message || e }))
    return false
  }
}

// 粘贴导入：支持 sing-box JSON / Clash YAML / base64 订阅 / 分享链接，
// 由后端 Set 统一识别并转换（无需前端预校验格式）。
async function importFromJson(name: string, content: string): Promise<boolean> {
  try {
    await subscription.SetSubscription(name, content)
    await refresh()
    ElMessage.success(t('msg.importSuccess'))
    return true
  } catch (e: any) {
    ElMessage.error(t('msg.importFailed', { error: e?.message || e }))
    return false
  }
}

// 导出订阅：取内容 → 选保存路径 → 复制到剪贴板兜底（Wails SaveFile 不直接写内容）。
async function exportSubscription(name: string): Promise<void> {
  try {
    const content = await subscription.ExportSubscription(name) as unknown as string
    const path = await Dialogs.SaveFile({
      title: t('msg.exportSubTitle'),
      defaultFilename: `${name}.json`,
      filters: [{ displayName: 'sing-box JSON', pattern: '*.json' }],
    } as any)
    if (path) {
      await navigator.clipboard.writeText(content)
      ElMessage.success(t('msg.copiedToClipboard'))
    }
  } catch (e: any) {
    ElMessage.error(t('msg.exportFailed', { error: e?.message || e }))
  }
}

export function useSubscriptions() {
  return {
    subscriptions, activeSubscription,
    refreshSubscriptions: refresh, setActive, createSubscription, removeSubscription, renameSubscription,
    refreshInfo, importFromFile, importFromUrl, importFromJson, exportSubscription,
  }
}

// useClashApi: 通过 sing-box 的 Clash 兼容 API 查询/切换代理组节点。
// sing-box 配置 experimental.clash_api.external_controller=127.0.0.1:9090 后，
// 主 app 用户态可直接 HTTP 访问本地端口（helper 是 root 但 API 绑 127.0.0.1）。

import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { t } from '../i18n'
import type { ProxyGroup } from '../types/clash'

// ProxyGroup 类型已集中到 types/clash.ts

const CLASH_API = 'http://127.0.0.1:9090'

// 节点延迟（tag → 毫秒，-1=超时/失败）
const proxyGroups = ref<ProxyGroup[]>([])
const delays = ref<Record<string, number>>({})
// 当前 Clash 代理模式（Rule/Global/Direct），运行中从 API 同步
const clashMode = ref('Rule')
let pollTimer: ReturnType<typeof setInterval> | null = null

// 拉取当前代理模式（GET /configs → mode）
async function fetchMode() {
  try {
    const res = await fetch(`${CLASH_API}/configs`)
    if (!res.ok) return
    const data = await res.json()
    if (typeof data.mode === 'string' && data.mode) clashMode.value = data.mode
  } catch { /* API 不可达，保持当前值 */ }
}

// 实时切换代理模式（PATCH /configs {mode}）。仅内核运行时可用。
async function setMode(mode: string) {
  try {
    const res = await fetch(`${CLASH_API}/configs`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ mode }),
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    clashMode.value = mode
  } catch (e: any) {
    ElMessage.error(t('msg.setModeFailed', { error: e?.message || e }))
  }
}

// 拉取代理组列表
async function fetchGroups() {
  try {
    const res = await fetch(`${CLASH_API}/proxies`)
    if (!res.ok) return
    const data = await res.json()
    const proxies = data.proxies || {}
    const groups: ProxyGroup[] = []
    for (const [name, p] of Object.entries<any>(proxies)) {
      // Selector / URLTest 类型作为代理组（有 all 字段）
      if (Array.isArray(p.all) && p.all.length > 0) {
        groups.push({
          name,
          type: p.type || '',
          now: p.now || '',
          all: p.all,
        })
      }
    }
    proxyGroups.value = groups
  } catch {
    // API 不可达（sing-box 未运行或未配 clash_api），清空
    proxyGroups.value = []
  }
}

// 切换某代理组的当前节点
async function selectNode(group: string, node: string) {
  try {
    const res = await fetch(`${CLASH_API}/proxies/${encodeURIComponent(group)}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: node }),
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    ElMessage.success(`${group} → ${node}`)
    await fetchGroups()
  } catch (e: any) {
    ElMessage.error(t('msg.selectNodeFailed', { error: e?.message || e }))
  }
}

// 测速单个节点
async function testDelay(node: string) {
  try {
    const url = encodeURIComponent('https://www.gstatic.com/generate_204')
    const res = await fetch(`${CLASH_API}/proxies/${encodeURIComponent(node)}/delay?url=${url}&timeout=5000`)
    const data = await res.json()
    const d = typeof data.delay === 'number' ? data.delay : -1
    delays.value = { ...delays.value, [node]: d }
  } catch {
    delays.value = { ...delays.value, [node]: -1 }
  }
}

// 启动/停止轮询（仅 sing-box 运行时）
function startPolling() {
  if (pollTimer) return
  fetchGroups()
  fetchMode()
  pollTimer = setInterval(() => { fetchGroups(); fetchMode() }, 5000)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  proxyGroups.value = []
  delays.value = {}
}

export function useClashApi() {
  return { proxyGroups, delays, clashMode, fetchGroups, fetchMode, selectNode, setMode, testDelay, startPolling, stopPolling }
}

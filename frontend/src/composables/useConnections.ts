// useConnections：通过 sing-box Clash API /connections 查看当前经代理的连接，
// 计算每条连接的上传/下载速度，并按主机聚合统计。单例。
import { ref } from 'vue'

const CLASH_API = 'http://127.0.0.1:9090'

export interface ConnMeta {
  network: string; type: string
  sourceIP: string; destinationIP: string
  sourcePort: string; destinationPort: string
  host: string; processPath: string
}
export interface ConnView {
  id: string
  meta: ConnMeta
  host: string
  src: string        // 源地址 ip:port
  dest: string       // 目标地址 ip:port
  proc: string       // 进程名（从 processPath 提取）
  inbound: string    // 入口：sing-box metadata.type = "入口类型/入口tag"，如 "tun/tun-in"、"mixed/mixed-back"
  chains: string[]   // 代理链路
  rule: string       // 适配规则
  start: string      // 连接开始时间
  download: number; upload: number       // 累计字节
  downSpeed: number; upSpeed: number      // 字节/秒
}
export interface HostGroup {
  host: string
  download: number; upload: number
  downSpeed: number; upSpeed: number
  count: number
  processes: string[]
  types: string[]
  conns: ConnView[]
}

const connections = ref<ConnView[]>([])
const hostGroups = ref<HostGroup[]>([])
const totalDown = ref(0)
const totalUp = ref(0)
const connected = ref(false)   // API 是否可达（内核运行 + clash_api）

let prev: Record<string, { up: number; down: number }> = {}
let lastTs = 0
let timer: ReturnType<typeof setInterval> | null = null

// 从 processPath 提取进程名："/Applications/Foo.app/Contents/MacOS/Foo (user)" → "Foo"
function procName(p: string): string {
  if (!p) return ''
  const base = p.split(' (')[0]
  return base.split('/').pop() || base
}

function aggregate(list: ConnView[]): HostGroup[] {
  const map = new Map<string, HostGroup>()
  for (const c of list) {
    let g = map.get(c.host)
    if (!g) {
      g = { host: c.host, download: 0, upload: 0, downSpeed: 0, upSpeed: 0, count: 0, processes: [], types: [], conns: [] }
      map.set(c.host, g)
    }
    g.download += c.download; g.upload += c.upload
    g.downSpeed += c.downSpeed; g.upSpeed += c.upSpeed
    g.count++
    g.conns.push(c)
    if (c.proc && !g.processes.includes(c.proc)) g.processes.push(c.proc)
    if (c.meta.network && !g.types.includes(c.meta.network)) g.types.push(c.meta.network)
  }
  // 下载量降序
  return [...map.values()].sort((a, b) => b.download - a.download)
}

async function poll() {
  try {
    const res = await fetch(`${CLASH_API}/connections`)
    if (!res.ok) { connected.value = false; return }
    const data = await res.json()
    connected.value = true
    const now = Date.now()
    const dt = lastTs ? (now - lastTs) / 1000 : 1
    lastTs = now
    totalDown.value = data.downloadTotal || 0
    totalUp.value = data.uploadTotal || 0
    const list: ConnView[] = []
    const newPrev: Record<string, { up: number; down: number }> = {}
    for (const c of (data.connections || [])) {
      const m: ConnMeta = c.metadata || {}
      const p = prev[c.id]
      const downSpeed = p && dt > 0 ? Math.max(0, (c.download - p.down) / dt) : 0
      const upSpeed = p && dt > 0 ? Math.max(0, (c.upload - p.up) / dt) : 0
      newPrev[c.id] = { up: c.upload, down: c.download }
      list.push({
        id: c.id,
        meta: m,
        host: m.host || m.destinationIP || '-',
        src: `${m.sourceIP || '-'}:${m.sourcePort || ''}`,
        dest: `${m.destinationIP || '-'}:${m.destinationPort || ''}`,
        proc: procName(m.processPath || ''),
        inbound: m.type || '',
        chains: c.chains || [],
        rule: c.rule || '',
        start: c.start || '',
        download: c.download || 0, upload: c.upload || 0,
        downSpeed, upSpeed,
      })
    }
    prev = newPrev
    connections.value = list
    hostGroups.value = aggregate(list)
  } catch {
    connected.value = false
    connections.value = []
    hostGroups.value = []
  }
}

function startPolling() {
  if (timer) return
  poll()
  timer = setInterval(poll, 1500)
}
function stopPolling() {
  if (timer) { clearInterval(timer); timer = null }
  prev = {}; lastTs = 0
}

// 关闭全部连接（Clash API DELETE /connections）。
async function closeAll() {
  try { await fetch(`${CLASH_API}/connections`, { method: 'DELETE' }); await poll() } catch { /* 忽略 */ }
}

// ===== 格式化辅助 =====
export function fmtBytes(n: number): string {
  if (!n) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0, v = n
  while (v >= 1024 && i < u.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(v >= 100 || i === 0 ? 0 : 1)} ${u[i]}`
}
export function fmtSpeed(n: number): string {
  return n > 0 ? fmtBytes(n) + '/s' : '—'
}
export function fmtDuration(start: string): string {
  if (!start) return '—'
  const t = new Date(start).getTime()
  if (isNaN(t)) return '—'
  let s = Math.max(0, Math.floor((Date.now() - t) / 1000))
  if (s < 60) return `${s}s`
  const m = Math.floor(s / 60); s %= 60
  if (m < 60) return `${m}m${s}s`
  const h = Math.floor(m / 60)
  return `${h}h${m % 60}m`
}

// ===== 入口(inbound)解析：区分 TUN(虚拟网卡) / MITM回注 / 系统代理 =====
// raw = sing-box metadata.type，形如 "tun/tun-in"、"mixed/mixed-back"、"http/xxx"。
// mixed-back 是 go-mitmproxy 解密后回注 sing-box 的入口，单独标为 mitm。
export function inboundKind(raw: string): 'tun' | 'mitm' | 'proxy' | '' {
  if (!raw) return ''
  if (raw.split('/')[0].toLowerCase() === 'tun') return 'tun'
  if (inboundTag(raw) === 'mixed-back') return 'mitm'
  return 'proxy'
}
// 取入口 tag（"/" 后半段），如 "tun/tun-in" → "tun-in"；无 tag 时返回类型本身。
export function inboundTag(raw: string): string {
  if (!raw) return ''
  const i = raw.indexOf('/')
  return i >= 0 ? raw.slice(i + 1) : raw
}

export function useConnections() {
  return { connections, hostGroups, totalDown, totalUp, connected, startPolling, stopPolling, closeAll }
}

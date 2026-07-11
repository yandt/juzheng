// useTrafficStats：首页流量统计的数据采集与聚合（单例）。
//
// 复用 useConnections 的连接快照（含每连接上下行速率 + chains 代理链路）与全局累计流量，
// 再单独拉 Clash /memory（流式端点，只读首帧）得到内核内存。每 ~2s 采一次样，维护 60 分钟
// 滚动缓冲；按选择的时间跨度（1/5/10/30/60 分钟）切片，按选择的代理组（''=全部）归并，
// 输出时间序列（供手绘 SVG 图表）与 6 项当前指标。
import { ref, computed } from 'vue'
import { useConnections } from './useConnections'
import { useClashApi } from './useClashApi'

const CLASH_API = 'http://127.0.0.1:9090'
const SAMPLE_MS = 2000
const RETAIN_MS = 60 * 60 * 1000 // 60 分钟滚动缓冲
const MAX_POINTS = 90            // 图表最多渲染点数（超出按桶平均降采样）

export type SpanMin = 1 | 5 | 10 | 30 | 60
export const SPAN_OPTIONS: SpanMin[] = [1, 5, 10, 30, 60]

interface GroupSample { up: number; down: number; conns: number }
interface Sample {
  t: number
  mem: number
  totUp: number   // 全局累计上传字节（Clash uploadTotal）
  totDown: number // 全局累计下载字节
  globalConns: number
  groups: Record<string, GroupSample> // 按代理组归并的瞬时速率 + 连接数
}

export interface SeriesPoint { t: number; up: number; down: number }

const samples = ref<Sample[]>([])
const spanMinutes = ref<SpanMin>(10)
const selectedGroup = ref<string>('') // '' = 全部
const memSupported = ref(true)

let timer: ReturnType<typeof setInterval> | null = null

const { connections, totalUp, totalDown, startPolling: startConns } = useConnections()
const { proxyGroups } = useClashApi()

// Clash /memory 与 /traffic 是「持续推送」的流式端点（每秒一帧 JSON），普通 fetch().json()
// 会一直挂起。这里读到第一帧完整 JSON 即中止连接。
async function fetchStreamOnce(url: string): Promise<any | null> {
  const ctrl = new AbortController()
  try {
    const res = await fetch(url, { signal: ctrl.signal })
    if (!res.ok || !res.body) { ctrl.abort(); return null }
    const reader = res.body.getReader()
    const dec = new TextDecoder()
    let buf = ''
    for (let i = 0; i < 5; i++) {
      const { value, done } = await reader.read()
      if (done) break
      buf += dec.decode(value, { stream: true })
      const nl = buf.indexOf('\n')
      if (nl >= 0) { ctrl.abort(); try { return JSON.parse(buf.slice(0, nl).trim()) } catch { return null } }
      try { const o = JSON.parse(buf.trim()); ctrl.abort(); return o } catch { /* 继续读 */ }
    }
    ctrl.abort()
    return null
  } catch { return null }
}

async function sample() {
  // 按 chains 里出现的代理组名归并每连接速率 + 连接数（嵌套组会同时计入各级）。
  const names = new Set(proxyGroups.value.map(g => g.name))
  const groups: Record<string, GroupSample> = {}
  for (const c of connections.value) {
    for (const ch of c.chains) {
      if (!names.has(ch)) continue
      const g = groups[ch] || (groups[ch] = { up: 0, down: 0, conns: 0 })
      g.up += c.upSpeed
      g.down += c.downSpeed
      g.conns++
    }
  }

  let mem = 0
  if (memSupported.value) {
    const m = await fetchStreamOnce(`${CLASH_API}/memory`)
    if (m && typeof m.inuse === 'number') mem = m.inuse
    else if (m === null) memSupported.value = false // 端点不可用则不再重试
  }

  const s: Sample = {
    t: Date.now(),
    mem,
    totUp: totalUp.value,
    totDown: totalDown.value,
    globalConns: connections.value.length,
    groups,
  }
  const arr = samples.value.slice()
  arr.push(s)
  const cutoff = s.t - RETAIN_MS
  while (arr.length && arr[0].t < cutoff) arr.shift()
  samples.value = arr
}

// 取当前时间跨度内的样本
function windowSamples(): Sample[] {
  const from = Date.now() - spanMinutes.value * 60 * 1000
  return samples.value.filter(s => s.t >= from)
}

// 按桶平均降采样到 max 个点
function downsample(pts: SeriesPoint[], max: number): SeriesPoint[] {
  if (pts.length <= max) return pts
  const bucket = Math.ceil(pts.length / max)
  const out: SeriesPoint[] = []
  for (let i = 0; i < pts.length; i += bucket) {
    const slice = pts.slice(i, i + bucket)
    const up = slice.reduce((a, b) => a + b.up, 0) / slice.length
    const down = slice.reduce((a, b) => a + b.down, 0) / slice.length
    out.push({ t: slice[slice.length - 1].t, up, down })
  }
  return out
}

// 单个样本对指定组的瞬时上下行速率（''=全局，用累计量差分；否则用组内速率之和）
function rateAt(win: Sample[], i: number, group: string): { up: number; down: number } {
  const s = win[i]
  if (group === '') {
    const prev = win[i - 1]
    if (!prev) return { up: 0, down: 0 }
    const dt = (s.t - prev.t) / 1000
    if (dt <= 0) return { up: 0, down: 0 }
    return {
      up: Math.max(0, (s.totUp - prev.totUp) / dt),
      down: Math.max(0, (s.totDown - prev.totDown) / dt),
    }
  }
  const g = s.groups[group]
  return g ? { up: g.up, down: g.down } : { up: 0, down: 0 }
}

const series = computed<SeriesPoint[]>(() => {
  const win = windowSamples()
  const g = selectedGroup.value
  const pts: SeriesPoint[] = []
  for (let i = 0; i < win.length; i++) {
    const r = rateAt(win, i, g)
    pts.push({ t: win[i].t, up: r.up, down: r.down })
  }
  return downsample(pts, MAX_POINTS)
})

export interface Metrics {
  upSpeed: number; downSpeed: number
  conns: number
  upVol: number; downVol: number
  mem: number
}

const metrics = computed<Metrics>(() => {
  const win = windowSamples()
  const g = selectedGroup.value
  const last = win[win.length - 1]
  const empty: Metrics = { upSpeed: 0, downSpeed: 0, conns: 0, upVol: 0, downVol: 0, mem: 0 }
  if (!last) return empty
  const r = rateAt(win, win.length - 1, g)
  let conns = 0, upVol = 0, downVol = 0
  if (g === '') {
    conns = last.globalConns
    const first = win[0]
    upVol = Math.max(0, last.totUp - first.totUp)
    downVol = Math.max(0, last.totDown - first.totDown)
  } else {
    conns = last.groups[g]?.conns ?? 0
    // 组内无累计量，按速率对时间积分得到窗口内数据量
    for (let i = 1; i < win.length; i++) {
      const gp = win[i].groups[g]
      if (!gp) continue
      const dt = (win[i].t - win[i - 1].t) / 1000
      upVol += gp.up * dt
      downVol += gp.down * dt
    }
  }
  return { upSpeed: r.up, downSpeed: r.down, conns, upVol, downVol, mem: last.mem }
})

const groupOptions = computed<{ label: string; value: string }[]>(() => [
  { label: '', value: '' }, // label 由组件用 i18n 覆盖为「全部」
  ...proxyGroups.value.map(g => ({ label: g.name, value: g.name })),
])

function start() {
  startConns()
  if (timer) return
  sample()
  timer = setInterval(sample, SAMPLE_MS)
}

function stop() {
  if (timer) { clearInterval(timer); timer = null }
}

export function useTrafficStats() {
  return {
    spanMinutes, selectedGroup, groupOptions,
    series, metrics, memSupported,
    hasData: computed(() => samples.value.length > 0),
    start, stop,
  }
}

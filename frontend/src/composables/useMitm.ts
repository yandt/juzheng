// useMitm: MITM 流量监控的状态与操作（单例 composable）。
// 持有 flows/selected/证书/helper 联动，供 TrafficMonitor 等页面复用。

import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { Events } from '@wailsio/runtime'
import { t } from '../i18n'
import * as mitm from '../../bindings/github.com/zhanghui/juzheng/mitmproxyservice'
import { FlowUpdate, ServiceOptions } from '../../bindings/github.com/zhanghui/juzheng/models'
import type { CapturedFlow } from '../../bindings/github.com/zhanghui/juzheng/internal/mitmcore/models'
import { useSbox } from './useSbox'
import { forgetFlow, forgetAllFlows } from './useMonitor'

// 抓包流量内存上限：超出后淘汰最旧（Map 头部），防止长时间抓包无限增长吃内存。
// 与监控命中的 200 上限对称；抓包流量是主数据故放宽到 1000。可按需调整。
const MAX_FLOWS = 1000

// 单例状态（模块级，跨组件共享）
const flows = ref<Map<string, CapturedFlow>>(new Map())
const flowList = computed(() => Array.from(flows.value.values()).reverse())
const selectedId = ref<string | null>(null)
const selected = computed(() => flows.value.get(selectedId.value ?? '') ?? null)

const running = ref(false)
const addr = ref('')
const upstream = ref('')
const caCertPath = ref('')
// MITM 配置项（首页 MITM 配置弹窗用）
const upstreamInput = ref('')
const mitmAddrInput = ref(':9080')        // 监听地址
const mitmSslInsecure = ref(true)         // 跳过上游 TLS 校验

const certStatus = ref<any>({ exists: false, trusted: false, path: '', installCmd: '', fingerprint: '' })
const installing = ref(false)

const filterHost = ref('')
const filteredList = computed(() => {
  if (!filterHost.value.trim()) return flowList.value
  const kw = filterHost.value.trim().toLowerCase()
  return flowList.value.filter(f => f.host.toLowerCase().includes(kw))
})

let cancelFlow: (() => void) | null = null
let cancelStart: (() => void) | null = null
let cancelStop: (() => void) | null = null
// 模块级 init flag：防止 v-show 常驻页面重复订阅事件。
let mitmInitialized = false

// ===== SSE 重连归并 =====
// 长连接 SSE 经代理链路常被空闲超时掐断，浏览器 EventSource 会自动重连，
// 每次重连是一个新的 HTTP 请求(新 flow id) —— 表现为"同一条 SSE 流在列表里裂成多行"。
// 这里在接收层把「同 method+URL 且时间相邻」的重连流归并成一行(取首个为主 flow)，
// 后续事件按时序拼进主 flow；用户只看到整合好的单条数据。
const SSE_MERGE_WINDOW = 60_000                   // 主流最近活动 60s 内的同 URL 新流视为重连
const sseParentByKey = new Map<string, string>()  // method+url -> 主 flow id
const sseAlias = new Map<string, string>()        // 重连子 flow id -> 主 flow id
const sseLastTs = new Map<string, number>()       // 主 flow id -> 最近事件到达时刻(ms)，用于重连窗口判断

function sseKey(f: CapturedFlow): string {
  return (f.method || '') + ' ' + (f.url || (f.host || '') + (f.path || ''))
}
// flow 的"最近活动时间"：优先结束时间，否则起始时间(毫秒)。拿不到返回 0。
function flowTs(f: CapturedFlow | undefined): number {
  if (!f) return 0
  const s = (f.endTime as any) || f.startTime
  if (!s) return 0
  const t = new Date(s as any).getTime()
  return isNaN(t) ? 0 : t
}
// 事件按时间排序键：SSE 事件无独立时间戳，保持追加顺序即可，这里仅用于主流已有事件之后拼接。
function appendEvents(parent: CapturedFlow, evs: CapturedFlow['sseEvents']) {
  if (!evs || evs.length === 0) return
  parent.sseEvents = [...(parent.sseEvents ?? []), ...evs]
}
// 归并映射清理：主 flow 被淘汰/清空时，连带清掉指向它的别名与索引，防止 Map 无限增长。
function dropSseMaps(parentId: string) {
  for (const [k, p] of sseParentByKey) if (p === parentId) sseParentByKey.delete(k)
  for (const [c, p] of sseAlias) if (p === parentId) sseAlias.delete(c)
  sseLastTs.delete(parentId)
}
// 主流最近活动时刻：优先实时记录的事件到达时间，退回 flow 时间戳。
function lastActiveTs(parentId: string, parent: CapturedFlow | undefined): number {
  return Math.max(sseLastTs.get(parentId) ?? 0, flowTs(parent))
}
// 解析事件目标 flow id：若是重连子流则重定向到主 flow。
function resolveTarget(id: string): string {
  return sseAlias.get(id) ?? id
}

function onFlowUpdate(evt: any) {
  const u: FlowUpdate = evt.data as FlowUpdate
  if (!u || !u.id) return
  switch (u.type) {
    case 'request':
      if (u.flow) {
        // 若同 method+URL 已有活跃的 SSE 主流(60s 内)，判定为重连：不建新行，归并到主流。
        const key = sseKey(u.flow)
        const parentId = sseParentByKey.get(key)
        const parent = parentId ? flows.value.get(parentId) : undefined
        if (parent && parentId && parentId !== u.id &&
            Date.now() - lastActiveTs(parentId, parent) < SSE_MERGE_WINDOW) {
          sseAlias.set(u.id, parentId)
          sseLastTs.set(parentId, Date.now())
          appendEvents(parent, u.flow.sseEvents)  // 一般为空，稳妥起见
          break
        }
        flows.value.set(u.id, u.flow)
        // 环形淘汰：超上限时从最旧(Map 头部)删起；被删的若正被选中则清空选择。
        while (flows.value.size > MAX_FLOWS) {
          const oldest = flows.value.keys().next().value
          if (oldest === undefined) break
          flows.value.delete(oldest)
          forgetFlow(oldest)  // 对齐清理该流量的命中标记，避免 hitByFlow 无限增长
          dropSseMaps(oldest)
          if (selectedId.value === oldest) selectedId.value = null
        }
      }
      break
    case 'response': {
      const ex = flows.value.get(resolveTarget(u.id))
      if (ex && u.flow) {
        ex.statusCode = u.flow.statusCode
        // 重连子流的响应头/体不覆盖主流已有的(以首个流为准)；仅补状态与结束时间。
        if (!sseAlias.has(u.id)) {
          ex.respHeaders = u.flow.respHeaders
          ex.respBody = u.flow.respBody
        }
        ex.endTime = u.flow.endTime
        ex.durationMs = u.flow.durationMs
      }
      break
    }
    case 'sse-start': {
      const ex = flows.value.get(resolveTarget(u.id))
      if (ex) {
        ex.isSSE = true
        // 登记/刷新该 URL 的 SSE 主流索引，供后续重连归并。
        const tid = resolveTarget(u.id)
        sseParentByKey.set(sseKey(ex), tid)
        sseLastTs.set(tid, Date.now())
      }
      break
    }
    case 'sse-message': {
      const tid = resolveTarget(u.id)
      const ex = flows.value.get(tid)
      if (ex && u.sseEvent) {
        ex.sseEvents = [...(ex.sseEvents ?? []), u.sseEvent]
        sseLastTs.set(tid, Date.now())  // 刷新活动时刻，长连接期间的重连也能正确归并
      }
      break
    }
    case 'sse-end': {
      const ex = flows.value.get(resolveTarget(u.id))
      if (ex && u.flow) { ex.endTime = u.flow.endTime; ex.durationMs = u.flow.durationMs }
      break
    }
  }
  flows.value = new Map(flows.value)
}

async function refreshCert() {
  try { certStatus.value = await mitm.GetCertStatus() } catch { /* 静默 */ }
}

async function toggleProxy() {
  try {
    if (running.value) {
      await mitm.Stop()
    } else {
      // 上游：留空则自动链到 sing-box 内核的 mixed-back 入站 —— 这样"解密后再经代理出网"
      // 自动成立，无需手动填。用户填了则用用户的（高级覆盖）。
      let upstream = upstreamInput.value.trim()
      if (!upstream) {
        const { config } = useSbox()
        const port = config.value.settings?.mixedBackPort || 9788
        upstream = `http://127.0.0.1:${port}`
      } else if (!/^\w+:\/\//.test(upstream)) {
        upstream = 'http://' + upstream // 无 scheme 时补 http://
      }
      const opts: ServiceOptions = {
        addr: mitmAddrInput.value.trim() || ':9080',
        upstream,
        sslInsecure: mitmSslInsecure.value,
      } as ServiceOptions
      await mitm.SetOptions(opts)
      addr.value = await mitm.Start() as unknown as string
      await refreshCert()
    }
  } catch (e) {
    ElMessage.error(t('msg.operationFailed', { error: e as any }))
  }
}

async function installCert() {
  installing.value = true
  try {
    await mitm.InstallCert()
    await refreshCert()
    if (certStatus.value.trusted) ElMessage.success(t('msg.caCertTrusted'))
    else ElMessage.warning(t('msg.installIncompleteCancel'))
  } catch (e) {
    ElMessage.error(t('msg.certInstallFailed', { error: e as any }))
  } finally {
    installing.value = false
  }
}

function clearFlows() {
  flows.value = new Map()
  selectedId.value = null
  sseParentByKey.clear(); sseAlias.clear(); sseLastTs.clear()  // 清 SSE 归并索引
  forgetAllFlows()  // 对齐清理 per-flow 命中标记（保留告警历史 hits）
}

// 对一批 flow 做 SSE 重连归并（backfill 用）：同 method+URL 且与主流时间相邻(60s)的 SSE 流
// 合并进主流，事件按后端有序(旧→新)拼接。返回归并后的 Map（保持插入序）。
function consolidateSSE(src: Map<string, CapturedFlow>): Map<string, CapturedFlow> {
  const out = new Map<string, CapturedFlow>()
  const parentByKey = new Map<string, string>()   // key -> 主 flow id(out 中)
  for (const [id, f] of src) {
    if (f.isSSE) {
      const key = sseKey(f)
      const pid = parentByKey.get(key)
      const parent = pid ? out.get(pid) : undefined
      // 子流「起始」与主流「最近活动(结束时间)」间隔在窗口内 → 视为重连，归并。
      // 用子流 startTime(而非 flowTs 的 endTime 优先)，避免长子流把自身时长算进间隔。
      const childStart = (() => { const t = new Date((f.startTime as any)).getTime(); return isNaN(t) ? 0 : t })()
      if (parent && pid && childStart - flowTs(parent) < SSE_MERGE_WINDOW) {
        appendEvents(parent, f.sseEvents)
        if (f.statusCode) parent.statusCode = f.statusCode
        if (f.endTime) { parent.endTime = f.endTime; parent.durationMs = f.durationMs }
        sseAlias.set(id, pid)  // 记录别名，实时事件也重定向到主流
        continue
      }
      out.set(id, f)
      parentByKey.set(key, id)
      sseParentByKey.set(key, id)
    } else {
      out.set(id, f)
    }
  }
  return out
}

// 补拉后端已抓流量：弥补"流量发生在订阅之前 / 实时事件投递失败"导致的漏显。
// 打开流量监控页时调用；不覆盖已有条目（实时事件的数据可能更新）。
async function backfillFlows() {
  try {
    const list = (await mitm.GetFlows()) as unknown as CapturedFlow[]
    if (!list || list.length === 0) return
    const m = new Map<string, CapturedFlow>()
    for (const f of list) m.set(f.id, f)            // 历史流量（旧→新，后端有序）
    for (const [id, f] of flows.value) m.set(id, f) // 叠加期间已到达的实时流量（保留其更新数据）
    // 重新计算 SSE 归并索引，再对整批做重连归并 —— 后端返回的是拆散的独立流。
    sseParentByKey.clear(); sseAlias.clear(); sseLastTs.clear()
    flows.value = consolidateSSE(m)
  } catch { /* 静默：拿不到就维持实时事件模式 */ }
}

export function useMitm() {
  // 事件订阅 + 初始加载只在首次 import 时执行一次（模块级 init flag），
  // 避免 v-show 常驻页面重复订阅 flow:update/proxy:* 事件。
  if (!mitmInitialized) {
    mitmInitialized = true
    cancelFlow = Events.On('flow:update', onFlowUpdate)
    cancelStart = Events.On('proxy:started', (e: any) => { running.value = true; addr.value = e?.data?.addr ?? '' })
    cancelStop = Events.On('proxy:stopped', () => { running.value = false })
    ;(async () => {
      try {
        const opts = await mitm.GetOptions()
        upstreamInput.value = opts.upstream ?? ''
        mitmAddrInput.value = opts.addr ?? ':9080'
        mitmSslInsecure.value = opts.sslInsecure ?? true
        caCertPath.value = await mitm.GetCACertPath()
        running.value = await mitm.IsRunning()
        await refreshCert()
        await backfillFlows()  // 首次挂载即补拉一次，弥补订阅前已抓的流量
      } catch { /* 初始化失败静默 */ }
    })()
  }
  return {
    flows, flowList, filteredList, filterHost,
    selectedId, selected,
    running, addr, upstream, caCertPath,
    upstreamInput, mitmAddrInput, mitmSslInsecure,
    certStatus, installing,
    toggleProxy, installCert, refreshCert, clearFlows, backfillFlows,
    selectFlow: (id: string) => { selectedId.value = id },
  }
}

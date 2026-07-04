<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { Events } from '@wailsio/runtime'
import * as mitm from '../bindings/github.com/zhanghui/juzheng/mitmproxyservice'
import { CapturedFlow, FlowUpdate, ServiceOptions } from '../bindings/github.com/zhanghui/juzheng/models'
import { analyzeRequest, type Segment, type Risk } from './analyzer'

// ====== 状态 ======
const flows = ref<Map<string, CapturedFlow>>(new Map())
const flowList = computed(() => Array.from(flows.value.values()).reverse()) // 最新的在前
const selectedId = ref<string | null>(null)
const selected = computed(() => flows.value.get(selectedId.value ?? '') ?? null)

// 分析引擎：对选中请求做片段拆解
const segments = computed<Segment[]>(() => {
  if (!selected.value) return []
  return analyzeRequest(selected.value.reqBody)
})
// 风险统计
const riskSummary = computed(() => {
  const counts = { danger: 0, warn: 0, info: 0, safe: 0 }
  for (const s of segments.value) counts[s.risk]++
  return counts
})

const running = ref(false)
const addr = ref('')
const upstream = ref('')
const caCertPath = ref('')
const upstreamInput = ref('')
const statusMsg = ref('')

// 证书状态
const certStatus = ref<any>({ exists: false, trusted: false, path: '', installCmd: '', fingerprint: '' })
const installing = ref(false)

// 过滤
const filterHost = ref('')
const filteredList = computed(() => {
  if (!filterHost.value.trim()) return flowList.value
  const kw = filterHost.value.trim().toLowerCase()
  return flowList.value.filter(f => f.host.toLowerCase().includes(kw))
})

// ====== 事件订阅 ======
let cancelFlow: (() => void) | null = null
let cancelStart: (() => void) | null = null
let cancelStop: (() => void) | null = null

function onFlowUpdate(evt: any) {
  const u: FlowUpdate = evt.data as FlowUpdate
  if (!u || !u.id) return
  switch (u.type) {
    case 'request': {
      if (u.flow) flows.value.set(u.id, u.flow)
      break
    }
    case 'response': {
      const existing = flows.value.get(u.id)
      if (existing && u.flow) {
        // 合并响应字段
        existing.statusCode = u.flow.statusCode
        existing.respHeaders = u.flow.respHeaders
        existing.respBody = u.flow.respBody
        existing.endTime = u.flow.endTime
        existing.durationMs = u.flow.durationMs
      }
      break
    }
    case 'sse-start': {
      const existing = flows.value.get(u.id)
      if (existing) existing.isSSE = true
      break
    }
    case 'sse-message': {
      const existing = flows.value.get(u.id)
      if (existing && u.sseEvent) {
        existing.sseEvents = [...(existing.sseEvents ?? []), u.sseEvent]
      }
      break
    }
    case 'sse-end': {
      // 无额外处理，endTime 已在后端更新
      break
    }
  }
  // 触发响应式（Map mutation 需要手动触发）
  flows.value = new Map(flows.value)
}

function onProxyStarted(evt: any) {
  running.value = true
  const d = evt.data
  addr.value = d?.addr ?? ''
  statusMsg.value = `代理运行中 @ ${addr.value}` + (d?.upstream ? ` → upstream ${d.upstream}` : '')
}
function onProxyStopped() {
  running.value = false
  statusMsg.value = '代理已停止'
}

onMounted(async () => {
  cancelFlow = Events.On('flow:update', onFlowUpdate)
  cancelStart = Events.On('proxy:started', onProxyStarted)
  cancelStop = Events.On('proxy:stopped', onProxyStopped)
  try {
    const opts = await mitm.GetOptions()
    upstreamInput.value = opts.upstream ?? ''
    caCertPath.value = await mitm.GetCACertPath()
    running.value = await mitm.IsRunning()
    await refreshCert()
  } catch (e) {
    statusMsg.value = '初始化失败: ' + e
  }
})

onUnmounted(() => {
  cancelFlow?.()
  cancelStart?.()
  cancelStop?.()
})

// ====== 操作 ======
async function toggleProxy() {
  try {
    if (running.value) {
      await mitm.Stop()
    } else {
      // 先同步配置（仅 upstream 可改）
      const opts = new ServiceOptions()
      opts.upstream = upstreamInput.value.trim()
      await mitm.SetOptions(opts)
      const a = await mitm.Start()
      addr.value = a
      await refreshCert() // 启动后证书才生成，刷新状态
    }
  } catch (e) {
    statusMsg.value = '操作失败: ' + e
  }
}

// 刷新证书状态
async function refreshCert() {
  try {
    certStatus.value = await mitm.GetCertStatus()
  } catch {
    /* 证书尚未生成时静默 */
  }
}

// 一键安装证书（osascript 提权）
async function installCert() {
  installing.value = true
  try {
    const out = await mitm.InstallCert()
    await refreshCert()
    if (certStatus.value.trusted) {
      statusMsg.value = '✓ 证书已信任'
    } else {
      statusMsg.value = '安装未完成: ' + (out || '（用户取消？）')
    }
  } catch (e) {
    statusMsg.value = '安装失败: ' + e
  } finally {
    installing.value = false
  }
}

function selectFlow(id: string) {
  selectedId.value = id
}

function clearFlows() {
  flows.value = new Map()
  selectedId.value = null
}

// 复制到剪贴板（供验证撇号码点等用）
async function copyText(t: string) {
  try {
    await navigator.clipboard.writeText(t)
    statusMsg.value = '已复制到剪贴板'
  } catch {
    statusMsg.value = '复制失败'
  }
}

// 时间格式化
function fmtTime(s: string | undefined): string {
  if (!s) return ''
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  return d.toLocaleTimeString('zh-CN', { hour12: false })
}

// body 尝试美化（JSON 则格式化）
function prettyBody(s: string): string {
  if (!s) return ''
  try {
    return JSON.stringify(JSON.parse(s), null, 2)
  } catch {
    return s
  }
}

// 撇号码点检测（审计关键点）
function apostropheInfo(text: string): string {
  if (!text) return ''
  // 找 Today?s date 中的撇号
  const m = text.match(/Today(.?)s date/i)
  if (m && m[1]) {
    const ch = m[1]
    const cp = ch.codePointAt(0)
    const map: Record<number, string> = {
      0x27: "U+0027 普通 ASCII 撇号（官方 API）",
      0x2019: "U+2019 右单引号 → 中转站标记",
      0x02BC: "U+02BC 修饰字母撇号 → 国产模型标记",
      0x02B9: "U+02B9 希腊语调符 → 两者皆是标记",
    }
    return `撇号码点: U+${(cp ?? 0).toString(16).toUpperCase().padStart(4, '0')} ${map[cp ?? 0] ?? '（未分类）'}`
  }
  return ''
}

// 状态码分级样式
function codeClass(c: number | undefined): string {
  if (!c) return ''
  if (c < 300) return 'ok'
  if (c < 400) return 'redir'
  if (c < 500) return 'err'
  return 'err5'
}

// 分析面板的风险汇总
function riskClass(s: { danger: number; warn: number; info: number; safe: number }): string {
  if (s.danger > 0) return 'danger'
  if (s.warn > 0) return 'warn'
  if (s.info > 0) return 'info'
  return 'safe'
}
function riskText(s: { danger: number; warn: number; info: number; safe: number }): string {
  if (s.danger > 0) return `🔴 高风险 ×${s.danger}`
  if (s.warn > 0) return `🟡 注意 ×${s.warn}`
  if (s.info > 0) return '🟢 正常'
  return '⚪ 无标记'
}
</script>

<template>
  <div class="app">
    <!-- 顶部工具栏 -->
    <header class="toolbar">
      <div class="brand">
        <span class="logo">⏻</span>
        <span class="title">Juzheng</span>
        <span class="subtitle">CC 流量监控</span>
      </div>
      <div class="status" :class="{ on: running }">{{ statusMsg || '就绪' }}</div>
      <div class="actions">
        <label class="field">
          <span>上游代理</span>
          <input v-model="upstreamInput" :disabled="running" placeholder="127.0.0.1:7890 (可空)" />
        </label>
        <button class="btn primary" @click="toggleProxy">{{ running ? '停止' : '启动' }}监控</button>
        <button class="btn" @click="clearFlows">清空</button>
      </div>
    </header>

    <!-- 代理运行时的使用提示 -->
    <div v-if="running" class="hint">
      将 Claude Code 走代理: <code>HTTPS_PROXY=http://127.0.0.1{{ addr }} claude</code>
      <button class="mini" @click="copyText(`HTTPS_PROXY=http://127.0.0.1${addr} claude`)">复制</button>
      ｜ 首次需信任 CA: <code>{{ caCertPath }}</code>
    </div>

    <!-- 证书引导：未信任时显著提示 -->
    <div v-if="certStatus.exists && !certStatus.trusted" class="cert-warn">
      <span>⚠ CA 证书尚未信任，Claude Code 会因证书校验失败而无法连接</span>
      <button class="btn primary small" :disabled="installing" @click="installCert">
        {{ installing ? '安装中…（请输入系统密码）' : '一键信任证书' }}
      </button>
      <button class="mini" @click="copyText(certStatus.installCmd)">复制手动命令</button>
      <button class="mini" @click="refreshCert">刷新</button>
    </div>
    <div v-else-if="certStatus.exists && certStatus.trusted" class="cert-ok">
      ✓ CA 证书已信任（指纹 {{ (certStatus.fingerprint || '').slice(0, 16) }}…）
    </div>

    <main class="main">
      <!-- 左：请求列表 -->
      <section class="list-pane">
        <div class="list-head">
          <input v-model="filterHost" placeholder="按域名过滤…" class="filter" />
          <span class="count">{{ filteredList.length }} / {{ flowList.length }}</span>
        </div>
        <div class="list">
          <div
            v-for="f in filteredList"
            :key="f.id"
            class="row"
            :class="{ sel: f.id === selectedId, sse: f.isSSE }"
            @click="selectFlow(f.id)"
          >
            <span class="m" :class="f.method">{{ f.method }}</span>
            <span class="host">{{ f.host }}</span>
            <span class="path">{{ f.path }}</span>
            <span class="code" :class="codeClass(f.statusCode)">{{ f.statusCode || '…' }}</span>
            <span v-if="f.isSSE" class="badge">SSE {{ f.sseEvents?.length ?? 0 }}</span>
          </div>
          <div v-if="flowList.length === 0" class="empty">尚无流量。启动监控后，用 Claude Code 走代理发消息。</div>
        </div>
      </section>

      <!-- 右：详情 -->
      <section class="detail-pane">
        <div v-if="!selected" class="empty">← 选择左侧请求查看明文</div>
        <template v-else>
          <div class="detail-head">
            <span :class="['m', selected.method]">{{ selected.method }}</span>
            <span class="url">{{ selected.url }}</span>
            <span :class="['code', codeClass(selected.statusCode)]">{{ selected.statusCode }}</span>
            <span class="dur" v-if="selected.durationMs">{{ selected.durationMs }}ms</span>
          </div>

          <!-- 片段分析面板 -->
          <div v-if="segments.length" class="analysis">
            <div class="analysis-head">
              <span class="analysis-title">片段分析</span>
              <span class="risk-badge" :class="riskClass(riskSummary)">{{ riskText(riskSummary) }}</span>
            </div>
            <div class="segments">
              <div
                v-for="(seg, i) in segments"
                :key="i"
                class="segment"
                :class="seg.risk"
                :title="seg.riskNote || seg.detail || ''"
              >
                <span class="seg-label">{{ seg.label }}</span>
                <span class="seg-value">{{ seg.value }}</span>
                <span v-if="seg.riskNote" class="seg-risk">⚠ {{ seg.riskNote }}</span>
              </div>
            </div>
          </div>

          <div class="tabs">
            <div class="tab-section">
              <h4>请求 Body <button class="mini" @click="copyText(selected.reqBody)">复制</button></h4>
              <pre class="body">{{ prettyBody(selected.reqBody) || '(空)' }}</pre>
            </div>
            <div v-if="selected.isSSE" class="tab-section">
              <h4>SSE 事件流 ({{ selected.sseEvents?.length ?? 0 }}) </h4>
              <div class="sse">
                <div v-for="(ev, i) in selected.sseEvents" :key="i" class="sse-ev">
                  <span class="ev-type">{{ ev.event || 'message' }}</span>
                  <pre class="ev-data">{{ ev.data }}</pre>
                </div>
              </div>
            </div>
            <div v-else class="tab-section">
              <h4>响应 Body</h4>
              <pre class="body">{{ prettyBody(selected.respBody) || '(空)' }}</pre>
            </div>
          </div>
        </template>
      </section>
    </main>
  </div>
</template>

<style scoped>
.app { display: flex; flex-direction: column; height: 100vh; color: #e6e6e6; font-family: -apple-system, "SF Pro Text", sans-serif; }
.toolbar { display: flex; align-items: center; gap: 16px; padding: 10px 16px; background: rgba(20,22,28,.9); border-bottom: 1px solid #2a2d36; }
.brand { display: flex; align-items: baseline; gap: 8px; }
.logo { font-size: 20px; color: #4ade80; }
.title { font-weight: 700; font-size: 16px; }
.subtitle { font-size: 12px; color: #888; }
.status { flex: 1; font-size: 12px; color: #999; }
.status.on { color: #4ade80; }
.actions { display: flex; align-items: center; gap: 10px; }
.field { display: flex; align-items: center; gap: 6px; font-size: 12px; color: #888; }
.field input { width: 170px; padding: 4px 8px; background: #1a1c22; border: 1px solid #333; border-radius: 4px; color: #ddd; font-size: 12px; }
.btn { padding: 6px 14px; border: 1px solid #3a3d46; background: #262932; color: #ddd; border-radius: 5px; cursor: pointer; font-size: 13px; }
.btn:hover { background: #2f333d; }
.btn.primary { background: #2563eb; border-color: #2563eb; color: #fff; }
.btn.primary:hover { background: #1d4ed8; }
.hint { padding: 6px 16px; background: #1a2030; border-bottom: 1px solid #2a2d36; font-size: 12px; color: #8ab4f8; display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.hint code { background: #11141a; padding: 2px 6px; border-radius: 3px; font-size: 11px; }
.mini { padding: 2px 8px; font-size: 11px; background: #2a2d36; border: 1px solid #3a3d46; color: #aaa; border-radius: 3px; cursor: pointer; }
.main { flex: 1; display: flex; overflow: hidden; }
.list-pane { width: 42%; display: flex; flex-direction: column; border-right: 1px solid #2a2d36; }
.list-head { display: flex; align-items: center; gap: 8px; padding: 8px 12px; border-bottom: 1px solid #222; }
.filter { flex: 1; padding: 4px 8px; background: #1a1c22; border: 1px solid #333; border-radius: 4px; color: #ddd; font-size: 12px; }
.count { font-size: 11px; color: #666; }
.list { flex: 1; overflow-y: auto; }
.row { display: flex; align-items: center; gap: 8px; padding: 6px 12px; border-bottom: 1px solid #1d1f25; cursor: pointer; font-size: 12px; }
.row:hover { background: #1d2027; }
.row.sel { background: #1e2a3a; }
.row.sse { border-left: 2px solid #f59e0b; }
.m { font-size: 10px; font-weight: 700; padding: 1px 5px; border-radius: 3px; background: #333; color: #ccc; min-width: 36px; text-align: center; }
.m.GET { background: #1e3a5f; color: #60a5fa; }
.m.POST { background: #1e4d3a; color: #4ade80; }
.m.PUT { background: #4d3a1e; color: #fbbf24; }
.m.DELETE { background: #4d1e1e; color: #f87171; }
.host { color: #8ab4f8; }
.path { color: #aaa; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.code { font-size: 11px; font-weight: 600; }
.code.ok { color: #4ade80; }
.code.redir { color: #fbbf24; }
.code.err { color: #f87171; }
.code.err5 { color: #f87171; }
.badge { font-size: 10px; padding: 1px 6px; background: #4d3a1e; color: #fbbf24; border-radius: 8px; }
.empty { padding: 40px; text-align: center; color: #555; font-size: 13px; }
.detail-pane { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.detail-head { display: flex; align-items: center; gap: 8px; padding: 10px 14px; border-bottom: 1px solid #222; font-size: 13px; }
.url { flex: 1; color: #ddd; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.dur { font-size: 11px; color: #888; }
.audit { margin: 8px 14px; padding: 8px 12px; background: #3d1e1e; border: 1px solid #5c2424; border-radius: 5px; font-size: 12px; color: #fca5a5; }
.tabs { flex: 1; overflow-y: auto; padding: 0 14px 14px; }
.tab-section h4 { margin: 12px 0 6px; font-size: 12px; color: #888; font-weight: 600; display: flex; align-items: center; gap: 8px; }
.body { background: #0f1116; border: 1px solid #222; border-radius: 5px; padding: 10px; font-size: 11px; line-height: 1.5; overflow-x: auto; white-space: pre-wrap; word-break: break-word; max-height: 40vh; overflow-y: auto; color: #cdd6e0; font-family: "SF Mono", Menlo, monospace; }
.sse { display: flex; flex-direction: column; gap: 4px; max-height: 40vh; overflow-y: auto; }
.sse-ev { background: #0f1116; border: 1px solid #222; border-radius: 4px; padding: 6px 8px; }
.ev-type { font-size: 10px; color: #fbbf24; font-weight: 600; }
.ev-data { font-size: 11px; color: #cdd6e0; white-space: pre-wrap; word-break: break-word; margin: 4px 0 0; font-family: "SF Mono", Menlo, monospace; }
.cert-warn { padding: 8px 16px; background: #3d2e0e; border-bottom: 1px solid #5c4818; font-size: 12px; color: #fbbf24; display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.cert-warn span { flex: 1; }
.cert-ok { padding: 5px 16px; background: #0e3d1e; border-bottom: 1px solid #185c24; font-size: 11px; color: #4ade80; font-family: "SF Mono", Menlo, monospace; }
.btn.small { padding: 4px 10px; font-size: 12px; }
.analysis { margin: 8px 14px; border: 1px solid #2a2d36; border-radius: 6px; overflow: hidden; }
.analysis-head { display: flex; align-items: center; justify-content: space-between; padding: 6px 10px; background: #16181e; border-bottom: 1px solid #2a2d36; }
.analysis-title { font-size: 12px; font-weight: 600; color: #aaa; }
.risk-badge { font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 10px; }
.risk-badge.danger { background: #4d1e1e; color: #fca5a5; }
.risk-badge.warn { background: #4d3a1e; color: #fbbf24; }
.risk-badge.info { background: #1e3a4d; color: #60a5fa; }
.risk-badge.safe { background: #1e4d3a; color: #4ade80; }
.segments { display: flex; flex-direction: column; }
.segment { display: flex; align-items: center; gap: 10px; padding: 6px 10px; border-bottom: 1px solid #1d1f25; font-size: 12px; }
.segment:last-child { border-bottom: none; }
.segment.danger { background: rgba(77, 30, 30, .3); }
.segment.warn { background: rgba(77, 58, 30, .2); }
.seg-label { font-size: 10px; font-weight: 600; padding: 1px 6px; border-radius: 3px; background: #2a2d36; color: #ccc; min-width: 60px; text-align: center; }
.segment.danger .seg-label { background: #4d1e1e; color: #fca5a5; }
.segment.warn .seg-label { background: #4d3a1e; color: #fbbf24; }
.seg-value { color: #cdd6e0; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: "SF Mono", Menlo, monospace; }
.seg-risk { font-size: 11px; color: #fca5a5; max-width: 45%; }
.segment.warn .seg-risk { color: #fbbf24; }
</style>

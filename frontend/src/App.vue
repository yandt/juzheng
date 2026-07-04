<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { Events } from '@wailsio/runtime'
import * as mitm from '../bindings/github.com/zhanghui/juzheng/mitmproxyservice'
import * as singbox from '../bindings/github.com/zhanghui/juzheng/singboxservice'
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

// sing-box 代理内核状态
const showProxyPanel = ref(false)              // 代理面板是否展开
const helperStatus = ref<any>({ installed: false, running: false, loaded: false, pid: 0, installCmd: '' })
const sboxRunning = ref(false)                  // sing-box 内核是否在运行
const sboxBusy = ref(false)                     // 启停操作进行中
const helperBusy = ref(false)                   // 安装/卸载进行中
const sboxConfig = ref('')                      // sing-box JSON 配置内容
const sboxConfigDirty = ref(false)              // 配置是否有未保存修改
const sboxConfigError = ref('')                 // 配置保存/解析错误

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
let cancelSboxStart: (() => void) | null = null
let cancelSboxStop: (() => void) | null = null

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
  cancelSboxStart = Events.On('singbox:started', onSboxStarted)
  cancelSboxStop = Events.On('singbox:stopped', onSboxStopped)
  try {
    const opts = await mitm.GetOptions()
    upstreamInput.value = opts.upstream ?? ''
    caCertPath.value = await mitm.GetCACertPath()
    running.value = await mitm.IsRunning()
    await refreshCert()
    // sing-box 初始状态
    await refreshHelperStatus()
    sboxRunning.value = await singbox.IsRunning().catch(() => false)
  } catch (e) {
    statusMsg.value = '初始化失败: ' + e
  }
})

onUnmounted(() => {
  cancelFlow?.()
  cancelStart?.()
  cancelStop?.()
  cancelSboxStart?.()
  cancelSboxStop?.()
})

// ====== 操作 ======
async function toggleProxy() {
  try {
    if (running.value) {
      await mitm.Stop()
    } else {
      // 先同步配置（仅 upstream 可改）
      const opts = { upstream: upstreamInput.value.trim() } as ServiceOptions
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

// ===== sing-box 代理内核 =====

function onSboxStarted() { sboxRunning.value = true; statusMsg.value = 'sing-box 内核已启动' }
function onSboxStopped() { sboxRunning.value = false; statusMsg.value = 'sing-box 内核已停止' }

// 刷新 helper 守护进程状态
async function refreshHelperStatus() {
  try {
    helperStatus.value = await singbox.GetHelperStatus()
  } catch {
    /* helper 不可达，保持默认 false */
  }
}

// 安装 helper（osascript 提权 + launchctl bootstrap）
async function installHelper() {
  helperBusy.value = true
  try {
    await singbox.InstallHelper()
    await refreshHelperStatus()
    statusMsg.value = helperStatus.value.installed ? '✓ helper 已安装并加载' : '安装未完成'
  } catch (e: any) {
    statusMsg.value = 'helper 安装失败: ' + (e?.message || e)
  } finally {
    helperBusy.value = false
  }
}

// 卸载 helper
async function uninstallHelper() {
  helperBusy.value = true
  try {
    await singbox.UninstallHelper()
    await refreshHelperStatus()
    sboxRunning.value = false
    statusMsg.value = 'helper 已卸载'
  } catch (e: any) {
    statusMsg.value = '卸载失败: ' + (e?.message || e)
  } finally {
    helperBusy.value = false
  }
}

// 启动 sing-box 内核（需 helper 已安装且运行）
async function startSbox() {
  if (!helperStatus.value.running) {
    statusMsg.value = '✗ helper 未运行，请先安装 helper'
    return
  }
  sboxBusy.value = true
  try {
    await singbox.Start()
    sboxRunning.value = true
    // 联动：启动 sing-box 后，把 MITM 的 upstream 指向 mixed-back(1081)
    if (running.value) {
      const o = { upstream: '127.0.0.1:1081' } as ServiceOptions
      await mitm.SetOptions(o).catch(() => {})
    }
  } catch (e: any) {
    statusMsg.value = 'sing-box 启动失败: ' + (e?.message || e)
  } finally {
    sboxBusy.value = false
  }
}

// 停止 sing-box 内核
async function stopSbox() {
  sboxBusy.value = true
  try {
    await singbox.Stop()
    sboxRunning.value = false
  } catch (e: any) {
    statusMsg.value = '停止失败: ' + (e?.message || e)
  } finally {
    sboxBusy.value = false
  }
}

// 加载 sing-box 配置到编辑器
async function loadSboxConfig() {
  try {
    sboxConfig.value = await singbox.GetConfig()
    sboxConfigDirty.value = false
    sboxConfigError.value = ''
  } catch (e: any) {
    sboxConfigError.value = '加载失败: ' + (e?.message || e)
  }
}

// 保存配置（仅内核停止时可改）
async function saveSboxConfig() {
  sboxConfigError.value = ''
  // 校验 JSON
  try {
    JSON.parse(sboxConfig.value)
  } catch (e: any) {
    sboxConfigError.value = 'JSON 语法错误: ' + e.message
    return
  }
  try {
    await singbox.SetConfig(sboxConfig.value)
    sboxConfigDirty.value = false
    statusMsg.value = '✓ 配置已保存'
  } catch (e: any) {
    sboxConfigError.value = '保存失败: ' + (e?.message || e)
  }
}

// 配置编辑器内容变更
function onConfigInput() {
  sboxConfigDirty.value = true
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

    <!-- 代理内核面板（可折叠） -->
    <div class="proxy-panel">
      <button class="proxy-toggle" @click="showProxyPanel = !showProxyPanel">
        <span class="toggle-icon">{{ showProxyPanel ? '▼' : '▶' }}</span>
        代理内核 (sing-box)
        <span class="sbox-state" :class="{ on: sboxRunning, installed: helperStatus.installed }">
          {{ sboxRunning ? '内核运行中' : helperStatus.installed ? 'helper 已装' : '未安装' }}
        </span>
      </button>
      <div v-if="showProxyPanel" class="proxy-body">
        <!-- helper 状态行 -->
        <div class="proxy-row">
          <div class="proxy-label">helper 守护进程</div>
          <div class="proxy-status">
            <span class="dot" :class="{ on: helperStatus.running }"></span>
            {{ helperStatus.running ? `运行中 (pid ${helperStatus.pid})` : '未运行' }}
            <span v-if="helperStatus.installed" class="tag green">已部署</span>
            <span v-else class="tag red">未部署</span>
            <span v-if="helperStatus.loaded" class="tag blue">launchd 已加载</span>
          </div>
          <div class="proxy-actions">
            <button class="mini" :disabled="helperBusy" @click="refreshHelperStatus">刷新</button>
            <button v-if="!helperStatus.installed" class="btn primary small" :disabled="helperBusy" @click="installHelper">
              {{ helperBusy ? '安装中…（输密码）' : '安装 helper' }}
            </button>
            <button v-else class="btn small" :disabled="helperBusy" @click="uninstallHelper">卸载</button>
          </div>
        </div>
        <!-- sing-box 内核启停 -->
        <div class="proxy-row">
          <div class="proxy-label">sing-box 内核</div>
          <div class="proxy-status">
            <span class="dot" :class="{ on: sboxRunning }"></span>
            {{ sboxRunning ? 'TUN 接管中' : '已停止' }}
          </div>
          <div class="proxy-actions">
            <button v-if="!sboxRunning" class="btn primary small" :disabled="sboxBusy || !helperStatus.running" @click="startSbox">启动</button>
            <button v-else class="btn small" :disabled="sboxBusy" @click="stopSbox">停止</button>
          </div>
        </div>
        <!-- 配置编辑器 -->
        <div class="proxy-row col">
          <div class="config-head">
            <span class="proxy-label">sing-box 配置 (JSON)</span>
            <div class="proxy-actions">
              <button class="mini" @click="loadSboxConfig">重新加载</button>
              <button class="btn small" :disabled="sboxRunning || !sboxConfigDirty" @click="saveSboxConfig">
                {{ sboxConfigDirty ? '保存*' : '已保存' }}
              </button>
            </div>
          </div>
          <textarea
            class="config-editor"
            v-model="sboxConfig"
            @input="onConfigInput"
            :disabled="sboxRunning"
            spellcheck="false"
            placeholder="点击「重新加载」读取配置…"
          ></textarea>
          <div v-if="sboxConfigError" class="config-err">⚠ {{ sboxConfigError }}</div>
          <div v-if="sboxRunning" class="config-hint">内核运行时配置只读，停止后可编辑</div>
        </div>
      </div>
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

/* 代理内核面板 */
.proxy-panel { background: #16181e; border-bottom: 1px solid #2a2d36; }
.proxy-toggle { width: 100%; display: flex; align-items: center; gap: 8px; padding: 7px 16px; background: transparent; border: none; color: #cdd6e0; font-size: 12px; cursor: pointer; text-align: left; }
.proxy-toggle:hover { background: #1d2027; }
.toggle-icon { color: #666; width: 12px; }
.sbox-state { margin-left: auto; font-size: 10px; padding: 2px 8px; border-radius: 10px; background: #3a2a2a; color: #fca5a5; }
.sbox-state.on { background: #1e4d3a; color: #4ade80; }
.sbox-state.installed { background: #2a2d3a; color: #93c5fd; }
.proxy-body { padding: 4px 16px 10px; display: flex; flex-direction: column; gap: 6px; }
.proxy-row { display: flex; align-items: center; gap: 12px; padding: 4px 0; font-size: 12px; }
.proxy-row.col { flex-direction: column; align-items: stretch; gap: 4px; }
.proxy-label { color: #888; min-width: 110px; font-size: 11px; }
.proxy-status { flex: 1; display: flex; align-items: center; gap: 6px; color: #cdd6e0; }
.dot { width: 7px; height: 7px; border-radius: 50%; background: #555; }
.dot.on { background: #4ade80; box-shadow: 0 0 6px rgba(74,222,128,.5); }
.tag { font-size: 9px; padding: 1px 6px; border-radius: 8px; }
.tag.green { background: #1e4d3a; color: #4ade80; }
.tag.red { background: #4d1e1e; color: #fca5a5; }
.tag.blue { background: #1e3a4d; color: #60a5fa; }
.proxy-actions { display: flex; gap: 6px; }
.config-head { display: flex; align-items: center; justify-content: space-between; margin-top: 4px; }
.config-editor { width: 100%; min-height: 180px; max-height: 280px; background: #0f1116; border: 1px solid #2a2d36; border-radius: 5px; padding: 8px 10px; color: #cdd6e0; font-family: "SF Mono", Menlo, monospace; font-size: 11px; line-height: 1.5; resize: vertical; outline: none; }
.config-editor:focus { border-color: #2563eb; }
.config-editor:disabled { opacity: .6; cursor: not-allowed; }
.config-err { font-size: 11px; color: #fca5a5; margin-top: 4px; }
.config-hint { font-size: 11px; color: #fbbf24; margin-top: 4px; }
</style>

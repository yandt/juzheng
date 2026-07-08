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

function onFlowUpdate(evt: any) {
  const u: FlowUpdate = evt.data as FlowUpdate
  if (!u || !u.id) return
  switch (u.type) {
    case 'request':
      if (u.flow) flows.value.set(u.id, u.flow)
      break
    case 'response': {
      const ex = flows.value.get(u.id)
      if (ex && u.flow) {
        ex.statusCode = u.flow.statusCode
        ex.respHeaders = u.flow.respHeaders
        ex.respBody = u.flow.respBody
        ex.endTime = u.flow.endTime
        ex.durationMs = u.flow.durationMs
      }
      break
    }
    case 'sse-start': {
      const ex = flows.value.get(u.id); if (ex) ex.isSSE = true
      break
    }
    case 'sse-message': {
      const ex = flows.value.get(u.id)
      if (ex && u.sseEvent) ex.sseEvents = [...(ex.sseEvents ?? []), u.sseEvent]
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
      } catch { /* 初始化失败静默 */ }
    })()
  }
  return {
    flows, flowList, filteredList, filterHost,
    selectedId, selected,
    running, addr, upstream, caCertPath,
    upstreamInput, mitmAddrInput, mitmSslInsecure,
    certStatus, installing,
    toggleProxy, installCert, refreshCert, clearFlows,
    selectFlow: (id: string) => { selectedId.value = id },
  }
}

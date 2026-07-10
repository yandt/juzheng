// useMonitor：内容层监控规则的读写 + 命中(monitor:hit)订阅，单例。
// 规则保存走 SetMonitorRules（后端持久化 + 热更新引擎，即时生效）。
import { ref } from 'vue'
import { Events } from '@wailsio/runtime'
import * as mitm from '../../bindings/github.com/zhanghui/juzheng/mitmproxyservice'
import type { MonitorRule, MonitorHit } from '../types/monitor'

const rules = ref<MonitorRule[]>([])
// 命中记录（最新在前，最多留 200 条），供报警面板展示。
const hits = ref<MonitorHit[]>([])
// flowId → 该 flow 最近一次命中（供流量列表打标）。
const hitByFlow = ref<Record<string, MonitorHit>>({})
const MAX_HITS = 200

let loaded = false
let saveTimer: ReturnType<typeof setTimeout> | null = null
let subscribed = false

async function loadRules() {
  try {
    rules.value = (await mitm.GetMonitorRules()) as unknown as MonitorRule[]
  } catch { rules.value = [] }
}

// 防抖保存（即改即存）：写盘 + 热更新引擎。
function scheduleSave() {
  if (saveTimer) clearTimeout(saveTimer)
  saveTimer = setTimeout(async () => {
    saveTimer = null
    try { await mitm.SetMonitorRules(rules.value as any) } catch { /* 保存失败静默，下次改动再试 */ }
  }, 400)
}

function subscribeHits() {
  if (subscribed) return
  subscribed = true
  Events.On('monitor:hit', (e: any) => {
    const h = e?.data as MonitorHit
    if (!h) return
    hits.value = [h, ...hits.value].slice(0, MAX_HITS)
    hitByFlow.value = { ...hitByFlow.value, [h.flowId]: h }
  })
}

function clearHits() {
  hits.value = []
  hitByFlow.value = {}
}

// 忘记某个 flow 的命中标记。供 useMitm 淘汰旧流量时对齐清理，避免 hitByFlow 随 flow 无限增长。
// ref<对象> 的 .value 是深响应式代理，直接 delete 即可触发更新（O(1)，不整对象 spread）。
export function forgetFlow(id: string) {
  if (id in hitByFlow.value) delete hitByFlow.value[id]
}

// 清空所有 per-flow 命中标记（供 useMitm 清空流量列表时对齐）。
// 只清标记，保留告警历史 hits —— hits 是独立的命中日志，不随流量列表清空而丢。
export function forgetAllFlows() {
  hitByFlow.value = {}
}

export function useMonitor() {
  if (!loaded) { loaded = true; loadRules() }
  subscribeHits()
  return { rules, hits, hitByFlow, loadRules, scheduleSave, clearHits }
}

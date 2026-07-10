// 内容层监控规则 —— 前端模型（与 Go internal/mitmcore JSON 对齐）。

export type MonitorField = 'host' | 'url' | 'path' | 'reqBody' | 'respBody'
export type MonitorOp = 'contains' | 'equals' | 'suffix' | 'regex'
export type MonitorPhase = 'request' | 'response' | 'both'
export type MonitorAction = 'forward' | 'alert' | 'modify' | 'block'
export type MonitorLevel = 'info' | 'warn' | 'danger'
export type ModifyTarget = 'body' | 'header' | 'status' | 'url'
export type BodyOp = 'replace' | 'regexReplace' | 'setFull'

export interface MonitorCondition {
  field: MonitorField
  op: MonitorOp
  value: string
}

export interface MonitorBlock {
  status: number   // 0=默认403
  body: string     // 空=默认提示
}

export interface MonitorModify {
  target: ModifyTarget
  bodyOp: BodyOp
  find: string
  replace: string
  headerName: string
  headerValue: string   // 空=删除该头
  status: number        // 仅 response
  url: string           // 仅 request
}

export interface MonitorRule {
  id: string
  name: string
  enabled: boolean
  phase: MonitorPhase
  logic: 'and' | 'or'
  conditions: MonitorCondition[]
  action: MonitorAction
  block?: MonitorBlock
  modify?: MonitorModify
  alertLevel: MonitorLevel
}

// 命中事件（后端 monitor:hit 推送）。
export interface MonitorHit {
  flowId: string
  ruleId: string
  ruleName: string
  phase: string
  action: string
  field: string
  level: string
  host: string
  url: string
  time: number
}

// 新建一条默认规则。
export function newMonitorRule(): MonitorRule {
  return {
    id: 'r' + Date.now().toString(36),
    name: '',
    enabled: true,
    phase: 'response',
    logic: 'and',
    conditions: [{ field: 'host', op: 'contains', value: '' }],
    action: 'alert',
    alertLevel: 'warn',
    block: { status: 0, body: '' },
    modify: { target: 'body', bodyOp: 'replace', find: '', replace: '', headerName: '', headerValue: '', status: 0, url: '' },
  }
}

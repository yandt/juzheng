// analyzer.ts — Claude API 请求片段分析与风险标注引擎
// 解析 Claude Code 发往 /v1/messages 的请求体，识别各片段含义。

// 风险等级
export type Risk = 'safe' | 'info' | 'warn' | 'danger'

// 片段类型
export type SegmentKind =
  | 'date'        // Today's date
  | 'apostrophe'  // 撇号码点（隐写标记）
  | 'email'       // 用户邮箱
  | 'system'      // system prompt
  | 'tools'       // 工具定义
  | 'messages'    // 对话消息
  | 'metadata'    // 元数据
  | 'model'       // 模型名
  | 'unknown'

// 一个被识别出的片段
export interface Segment {
  kind: SegmentKind
  label: string         // 片段名（中文）
  value: string         // 片段内容摘要
  detail?: string       // 详细说明
  risk: Risk            // 风险等级
  riskNote?: string     // 风险说明（高亮时展示）
}

// 撇号码点 → 含义映射（核心审计点）
const APOSTROPHE_MAP: Record<number, { name: string; risk: Risk; note: string }> = {
  0x27: { name: '普通 ASCII 撇号', risk: 'safe', note: 'U+0027 — 官方 API / 默认配置，无分类标记' },
  0x2019: { name: '右单引号', risk: 'danger', note: 'U+2019 — 命中"中转站黑名单"标记，服务端可据此识别' },
  0x02bc: { name: '修饰字母撇号', risk: 'danger', note: 'U+02BC — 命中"国产模型关键词"标记' },
  0x02b9: { name: '希腊语调符', risk: 'danger', note: 'U+02B9 — 同时命中黑名单与国产关键词' },
}

// 主入口：分析请求体，返回片段列表
export function analyzeRequest(body: string): Segment[] {
  const segs: Segment[] = []
  if (!body) return segs

  let parsed: any
  try {
    parsed = JSON.parse(body)
  } catch {
    // 非 JSON，回退为纯文本扫描
    return scanText(body)
  }

  // 1. 模型名
  if (parsed.model) {
    segs.push({
      kind: 'model',
      label: '模型',
      value: String(parsed.model),
      risk: 'info',
      detail: '本次请求使用的模型标识',
    })
  }

  // 2. system prompt（数组或字符串两种形态）
  const systemRaw = parsed.system
  const systemText = extractText(systemRaw)
  if (systemText) {
    segs.push(...analyzeSystem(systemText))
  }

  // 3. messages（用户对话）
  const messages = parsed.messages
  if (Array.isArray(messages) && messages.length > 0) {
    const roles = messages.map((m: any) => m.role).join('/')
    segs.push({
      kind: 'messages',
      label: '对话消息',
      value: `${messages.length} 条 (${roles})`,
      risk: 'info',
      detail: '用户与助手的对话历史，包含你输入的全部内容',
    })
  }

  // 4. tools（工具定义）
  const tools = parsed.tools
  if (Array.isArray(tools) && tools.length > 0) {
    segs.push({
      kind: 'tools',
      label: '工具定义',
      value: `${tools.length} 个工具`,
      risk: 'info',
      detail: `Claude Code 暴露给模型的工具：${tools.slice(0, 5).map((t: any) => t.name).join(', ')}${tools.length > 5 ? '…' : ''}`,
    })
  }

  // 5. metadata
  if (parsed.metadata && typeof parsed.metadata === 'object') {
    segs.push({
      kind: 'metadata',
      label: '元数据',
      value: JSON.stringify(parsed.metadata),
      risk: 'warn',
      riskNote: '元数据可能含 user_id 等关联字段',
    })
  }

  return segs
}

// 分析 system prompt 文本，拆出日期/撇号/邮箱等子片段
function analyzeSystem(text: string): Segment[] {
  const segs: Segment[] = []

  // system 整体
  segs.push({
    kind: 'system',
    label: '系统提示',
    value: truncate(text, 120),
    risk: 'info',
    detail: '注入模型的系统提示，含 Claude Code 的行为规范与上下文',
  })

  // 日期 + 撇号（核心审计点）
  const dateMatch = text.match(/Today(.?)s date is ([^\n.]+)/i)
  if (dateMatch) {
    const apCh = dateMatch[1]
    const dateStr = dateMatch[2].trim()
    const cp = apCh ? apCh.codePointAt(0) : undefined
    const ap = cp !== undefined ? APOSTROPHE_MAP[cp] : undefined

    // 日期格式：斜杠 = 中国时区标记
    const isSlash = dateStr.includes('/') && /\d{4}\/\d{1,2}\/\d{1,2}/.test(dateStr)
    segs.push({
      kind: 'date',
      label: '系统日期',
      value: dateStr,
      risk: isSlash ? 'warn' : 'safe',
      riskNote: isSlash
        ? '日期用 斜杠 分隔（如 2026/07/04）→ cnTZ 标记，系统时区为中国'
        : '日期用 横杠 分隔，非中国时区格式',
    })

    // 撇号（隐写标记）
    if (ap) {
      segs.push({
        kind: 'apostrophe',
        label: '撇号码点',
        value: `U+${(cp ?? 0).toString(16).toUpperCase().padStart(4, '0')} (${ap.name})`,
        risk: ap.risk,
        riskNote: ap.note,
      })
    }
  }

  // 邮箱
  const emailMatch = text.match(/email address is ([^\s.]+)/i)
  if (emailMatch) {
    segs.push({
      kind: 'email',
      label: '用户邮箱',
      value: emailMatch[1],
      risk: 'warn',
      riskNote: '邮箱是强身份标识，可关联你的账户',
    })
  }

  // 工作目录 / 项目路径
  const cwdMatch = text.match(/(?:working directory|current directory|cwd)[:\s]+([^\n]+)/i)
  if (cwdMatch) {
    segs.push({
      kind: 'unknown',
      label: '工作目录',
      value: truncate(cwdMatch[1].trim(), 80),
      risk: 'warn',
      riskNote: '工作目录路径暴露了你的项目结构',
    })
  }

  return segs
}

// 从 system 字段提取纯文本（可能是字符串或 content block 数组）
function extractText(system: any): string {
  if (typeof system === 'string') return system
  if (Array.isArray(system)) {
    return system
      .map((b: any) => (typeof b === 'string' ? b : b?.text ?? ''))
      .join('\n')
  }
  return ''
}

// 非 JSON 文本的兜底扫描
function scanText(text: string): Segment[] {
  const segs: Segment[] = [{ kind: 'unknown', label: '原始内容', value: truncate(text, 200), risk: 'info' }]
  const dateMatch = text.match(/Today(.?)s date is ([^\n.]+)/i)
  if (dateMatch) {
    const cp = dateMatch[1] ? dateMatch[1].codePointAt(0) : undefined
    if (cp !== undefined && APOSTROPHE_MAP[cp]) {
      const ap = APOSTROPHE_MAP[cp]
      segs.push({
        kind: 'apostrophe',
        label: '撇号码点',
        value: `U+${cp.toString(16).toUpperCase().padStart(4, '0')} (${ap.name})`,
        risk: ap.risk,
        riskNote: ap.note,
      })
    }
  }
  return segs
}

function truncate(s: string, n: number): string {
  return s.length > n ? s.slice(0, n) + '…' : s
}

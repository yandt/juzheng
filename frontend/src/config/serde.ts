// config/serde.ts — sing-box 配置的解析（JSON → 结构化）与序列化（结构化 → JSON）。
// 把 sing-box JSON 拆成结构化对象（供图形编辑），并保留未图形化字段（透传，避免丢失）。

import type {
  SingBoxConfig, SingBoxNode, SelectorGroup, SingBoxRule, OrderedRule,
  GroupRule, SingBoxSettings, DnsServer, RuleMatchType, DnsMatchType,
} from './types'
import { DNS_REJECT, PROXY_OUT, SYSTEM_NODE_TAGS, MITM_NODE_TAG } from './types'

// 已图形化的字段集合（解析时不放进 raw，序列化时从结构化取）。
const GRAPHICAL_KEYS = new Set(['type', 'tag', 'server', 'server_port'])

export function parseConfig(jsonStr: string): SingBoxConfig | null {
  let obj: any
  try { obj = JSON.parse(jsonStr) } catch { return null }

  // outbounds → nodes（用户可见节点）+ systemNodes（系统隐身节点）+ groups（代理组）
  const nodes: SingBoxNode[] = []
  const systemNodes: SingBoxNode[] = []
  const groups: SelectorGroup[] = []
  if (Array.isArray(obj.outbounds)) {
    for (const ob of obj.outbounds) {
      const t = ob.type ?? 'direct'
      if (t === 'selector' || t === 'urltest') {
        // 代理组
        const g: SelectorGroup = {
          type: t,
          tag: ob.tag ?? '',
          outbounds: Array.isArray(ob.outbounds) ? ob.outbounds : [],
        }
        if (t === 'selector' && ob.default !== undefined) g.default = ob.default
        if (t === 'urltest') {
          if (ob.url !== undefined) g.url = ob.url
          if (ob.interval !== undefined) g.interval = ob.interval
        }
        // 其余字段（tolerance/idle_timeout/interrupt_exist_connections 等）透传
        const raw: Record<string, any> = {}
        for (const k of Object.keys(ob)) {
          if (!['type', 'tag', 'outbounds', 'default', 'url', 'interval'].includes(k)) raw[k] = ob[k]
        }
        if (Object.keys(raw).length) g.raw = raw
        groups.push(g)
      } else {
        // 普通节点：系统保留 tag → systemNodes（隐身），其余 → nodes（用户可见）
        const node: SingBoxNode = { type: t, tag: ob.tag ?? '' }
        if (ob.server !== undefined) node.server = ob.server
        if (ob.server_port !== undefined) node.server_port = ob.server_port
        const raw: Record<string, any> = {}
        for (const k of Object.keys(ob)) {
          if (!GRAPHICAL_KEYS.has(k)) raw[k] = ob[k]
        }
        if (Object.keys(raw).length) node.raw = raw
        if (SYSTEM_NODE_TAGS.has(node.tag)) {
          systemNodes.push(node)
        } else {
          nodes.push(node)
        }
      }
    }
  }

  // route.rules → 按 outbound 分发：
  // outbound 匹配某代理组 tag 的规则 → 拆成 GroupRule 挂到 g.rules
  // outbound 匹配某用户节点 tag 的规则 → 拆成 GroupRule 挂到 node.rules
  // 其余规则（指向系统节点出口如 dns-out/direct/to-mitmproxy）→ 留在 cfg.rules（系统规则，raw 保真）
  const rules: SingBoxRule[] = []
  const orderedRules: OrderedRule[] = []
  const mitmRules: GroupRule[] = []
  const routeRules = obj?.route?.rules
  // 组 tag → 组对象的索引
  const groupByTag = new Map<string, SelectorGroup>()
  for (const g of groups) groupByTag.set(g.tag, g)
  // 用户节点 tag → 节点对象的索引（系统节点不参与，它的规则走系统规则路径保真）
  const nodeByTag = new Map<string, SingBoxNode>()
  for (const n of nodes) nodeByTag.set(n.tag, n)

  // 所有匹配类型字段（用于判断规则是否含可分发的 matchType）
  const ALL_MATCH_TYPES = ['domain_suffix', 'domain_keyword', 'domain', 'domain_regex', 'ip_cidr', 'geosite', 'geoip', 'protocol'] as RuleMatchType[]

  if (Array.isArray(routeRules)) {
    for (const r of routeRules) {
      const outbound = r.outbound ?? r.action ?? ''
      const targetGroup = groupByTag.get(outbound)
      const targetNode = nodeByTag.get(outbound)
      // 检查规则是否含至少一个 matchType 字段。
      // 纯 inbound/action 限定规则（如回环打破 inbound:["mixed-back"]）无 matchType，
      // 即使 outbound 匹配组/节点，也走系统规则路径保真，避免规则内容丢失。
      const hasMatchType = ALL_MATCH_TYPES.some(mt => r[mt] !== undefined)
      if ((targetGroup || targetNode) && hasMatchType) {
        // 用户规则：展开成有序扁平条目，保留 route.rules 的原始顺序（= 匹配优先级）。
        for (const mt of ALL_MATCH_TYPES) {
          const v = r[mt]
          if (v !== undefined) {
            const arr = Array.isArray(v) ? v.map(String) : [String(v)]
            for (const val of arr) orderedRules.push({ value: val, matchType: mt, outbound })
          }
        }
      } else {
        // 系统规则（指向系统节点出口，或纯 inbound 限定规则）：保留在 cfg.rules（raw 保真，含 inbound 等字段）
        const domainBits = [r.domain_suffix, r.domain, r.domain_keyword, r.inbound, r.protocol, r.geosite]
          .filter(Boolean).flat().join(', ')
        rules.push({
          raw: r,
          desc: domainBits || JSON.stringify(r).slice(0, 60),
          outbound,
          isMitmRule: Array.isArray(r.domain_suffix) && r.domain_suffix && r.domain_suffix.length > 0,
        })
        // MITM 域名规则提取：outbound=to-mitmproxy 的规则拆成 GroupRule 供首页编辑
        if (outbound === MITM_NODE_TAG) {
          for (const mt of ALL_MATCH_TYPES) {
            const v = r[mt]
            if (v !== undefined) {
              const arr = Array.isArray(v) ? v.map(String) : [String(v)]
              if (arr.length) mitmRules.push({ matchType: mt, values: arr })
            }
          }
        }
      }
    }
  }

  // settings：扫 inbounds 找 tun / mixed-back
  const settings: SingBoxSettings = {
    tunEnabled: false, tunAddress: '', tunMtu: 1500, tunStack: 'system',
    tunInterfaceName: '', tunAutoRoute: true, tunStrictRoute: true,
    mixedBackPort: 9788, logLevel: 'info', dnsServers: [], dnsStrategy: '',
    dnsRules: [], dnsFinal: '', dnsRawRules: [], clashMode: 'Rule',
  }
  if (Array.isArray(obj.inbounds)) {
    for (const ib of obj.inbounds) {
      if (ib.type === 'tun') {
        settings.tunEnabled = true
        settings.tunAddress = Array.isArray(ib.address) ? ib.address[0] ?? '' : ib.address ?? ''
        settings.tunMtu = ib.mtu ?? 1500
        settings.tunStack = ib.stack ?? 'system'
        settings.tunInterfaceName = ib.interface_name ?? ''
        settings.tunAutoRoute = ib.auto_route ?? true
        settings.tunStrictRoute = ib.strict_route ?? true
      } else if (ib.tag === 'mixed-back' && ib.type === 'mixed') {
        settings.mixedBackPort = ib.listen_port ?? 9788
      }
    }
  }
  if (obj.log?.level) settings.logLevel = obj.log.level
  if (Array.isArray(obj.dns?.servers)) {
    // 补全 tag（dns.rules / dns.final 按 tag 引用；无 tag 的按索引生成稳定 tag）。
    settings.dnsServers = obj.dns.servers
      .map((s: any, i: number) => typeof s === 'string'
        ? { address: s, detour: '', tag: `dns-${i}` }
        : { address: s.address ?? '', detour: s.detour ?? '', tag: s.tag || `dns-${i}` })
      .filter((s: DnsServer) => s.address)
  }
  if (typeof obj.dns?.strategy === 'string') settings.dnsStrategy = obj.dns.strategy
  // dns.rules → 结构化 dnsRules（单匹配器 + 目标 server/reject）+ dnsFinal（兜底）+ dnsRawRules（高级规则原样保真）。
  const DNS_MATCH_TYPES = ['domain_suffix', 'domain', 'domain_keyword', 'domain_regex', 'rule_set'] as DnsMatchType[]
  if (Array.isArray(obj.dns?.rules)) {
    for (const r of obj.dns.rules) {
      // 目标：action:"reject" → 拒绝解析；否则视为 route（server 字段，旧式无 action 也按此）。
      const server = r.action === 'reject' ? DNS_REJECT : (r.server ?? '')
      const presentTypes = DNS_MATCH_TYPES.filter(mt => r[mt] !== undefined)
      // 除匹配器/server/action 外的其它键（outbound 属于兜底语义，单独判定）。
      const otherKeys = Object.keys(r).filter(k => k !== 'server' && k !== 'action' && k !== 'outbound' && !DNS_MATCH_TYPES.includes(k as DnsMatchType))
      const isCatchAll = (r.outbound === 'any' || presentTypes.length === 0) && otherKeys.length === 0
      if (isCatchAll && server && server !== DNS_REJECT) {
        settings.dnsFinal = server   // outbound:any / 无匹配器 + server → 兜底
        continue
      }
      // 简单规则：恰好一个支持的匹配器、无其它未知键、有目标 → 拆成有序扁平条目。
      if (presentTypes.length === 1 && otherKeys.length === 0 && !('outbound' in r) && server) {
        const mt = presentTypes[0]
        const v = r[mt]
        const arr = Array.isArray(v) ? v.map(String) : [String(v)]
        for (const val of arr) settings.dnsRules.push({ value: val, matchType: mt, server })
        continue
      }
      settings.dnsRawRules.push(r)   // 其余：高级规则原样保留
    }
  }
  if (typeof obj.dns?.final === 'string' && !settings.dnsFinal) settings.dnsFinal = obj.dns.final
  // 代理模式：从 experimental.clash_api.default_mode 读取（缺省 Rule）
  settings.clashMode = obj.experimental?.clash_api?.default_mode || 'Rule'

  // 顶层 raw：保留所有字段（序列化时覆盖图形化的部分）
  return { nodes, systemNodes, groups, orderedRules, rules, mitmRules, settings, raw: obj }
}

export function serializeConfig(cfg: SingBoxConfig): string {
  // 以 raw 为基底，用图形化结果覆盖 outbounds/inbounds 相关部分。
  const obj: Record<string, any> = cfg.raw ? JSON.parse(JSON.stringify(cfg.raw)) : {}

  // outbounds：systemNodes（系统隐身节点）+ nodes（用户节点）+ groups（代理组）。
  // 系统节点放最前（sing-box 要求 tag 先定义，且 to-mitmproxy/direct 等被规则引用）。
  const systemNodeObs = (cfg.systemNodes ?? []).map(n => {
    const ob: Record<string, any> = { type: n.type, tag: n.tag }
    if (n.server !== undefined) ob.server = n.server
    if (n.server_port !== undefined) ob.server_port = n.server_port
    if (n.raw) Object.assign(ob, n.raw)
    return ob
  })
  const nodeObs = cfg.nodes.map(n => {
    const ob: Record<string, any> = { type: n.type, tag: n.tag }
    if (n.server !== undefined) ob.server = n.server
    if (n.server_port !== undefined) ob.server_port = n.server_port
    if (n.raw) Object.assign(ob, n.raw)
    return ob
  })
  const groupObs = (cfg.groups ?? []).map(g => {
    const ob: Record<string, any> = { type: g.type, tag: g.tag, outbounds: g.outbounds }
    if (g.type === 'selector' && g.default !== undefined) ob.default = g.default
    if (g.type === 'urltest') {
      if (g.url !== undefined) ob.url = g.url
      if (g.interval !== undefined) ob.interval = g.interval
    }
    if (g.raw) Object.assign(ob, g.raw)
    return ob
  })
  obj.outbounds = [...systemNodeObs, ...nodeObs, ...groupObs]

  // inbounds：修改 mixed-back 端口、TUN 设置（基于 raw 里的 inbound 结构改字段）
  if (Array.isArray(obj.inbounds)) {
    obj.inbounds = obj.inbounds.map((ib: any) => {
      if (ib.type === 'tun') {
        return {
          ...ib,
          address: cfg.settings.tunAddress ? [cfg.settings.tunAddress] : ib.address,
          mtu: cfg.settings.tunMtu,
          stack: cfg.settings.tunStack,
          interface_name: cfg.settings.tunInterfaceName || undefined,
          auto_route: cfg.settings.tunAutoRoute,
          strict_route: cfg.settings.tunStrictRoute,
        }
      }
      if (ib.tag === 'mixed-back' && ib.type === 'mixed') {
        return { ...ib, listen_port: cfg.settings.mixedBackPort }
      }
      return ib
    })
  }

  // log level
  obj.log = { ...(obj.log || {}), level: cfg.settings.logLevel }

  // 代理模式：写回 experimental.clash_api.default_mode（启动默认模式）。
  {
    const exp: Record<string, any> = { ...(obj.experimental || {}) }
    const capi: Record<string, any> = { ...(exp.clash_api || {}) }
    capi.default_mode = cfg.settings.clashMode || 'Rule'
    exp.clash_api = capi
    obj.experimental = exp
  }

  // dns：从结构化 dnsServers/strategy 写回，保留 dns.rules 等其它字段（来自 raw）。
  // 每个 server 保留/补全 tag（sing-box dns.rules 可能按 tag 引用），address + detour 来自 UI。
  {
    const dns: Record<string, any> = { ...(obj.dns || {}) }
    dns.servers = (cfg.settings.dnsServers ?? [])
      .filter(d => (d.address ?? '').trim())
      .map((d, i) => {
        const srv: Record<string, any> = { tag: d.tag || `dns-${i}`, address: d.address.trim() }
        if (d.detour) srv.detour = d.detour
        return srv
      })
    if (cfg.settings.dnsStrategy) dns.strategy = cfg.settings.dnsStrategy
    else delete dns.strategy

    // dns.rules：结构化分流规则（按序，合并连续同 server+matchType）+ 追加高级原样规则。
    const dnsNewRules: Record<string, any>[] = []
    let dcur: { server: string; matchType: DnsMatchType; values: string[] } | null = null
    const flushD = () => {
      if (!dcur || dcur.values.length === 0) { dcur = null; return }
      const rule: Record<string, any> = {}
      rule[dcur.matchType] = dcur.values
      if (dcur.server === DNS_REJECT) rule.action = 'reject'
      else rule.server = dcur.server
      dnsNewRules.push(rule)
      dcur = null
    }
    for (const r of (cfg.settings.dnsRules ?? [])) {
      const val = (r.value ?? '').trim()
      if (!val || !r.server) continue
      if (dcur && dcur.server === r.server && dcur.matchType === r.matchType) {
        if (!dcur.values.includes(val)) dcur.values.push(val)
      } else {
        flushD()
        dcur = { server: r.server, matchType: r.matchType, values: [val] }
      }
    }
    flushD()
    for (const rr of (cfg.settings.dnsRawRules ?? [])) dnsNewRules.push(rr)
    if (dnsNewRules.length > 0) dns.rules = dnsNewRules
    else delete dns.rules

    // dns.final：兜底服务器 tag（空则删除，sing-box 默认用第一台）。
    if (cfg.settings.dnsFinal) dns.final = cfg.settings.dnsFinal
    else delete dns.final

    obj.dns = dns
  }

  // route.rules 顺序约定（MITM/系统优先）：系统规则在前 → 用户规则在后。
  // 系统规则含 MITM 解密（to-mitmproxy，多为 inbound:tun-in 限定）、DNS 劫持等，
  // 命中即停，保证解密/系统规则不被用户规则盖过；用户规则处理其余流量（含 mixed-back 回注）。
  const newRules: Record<string, any>[] = []

  // 1. 系统规则（cfg.rules，指向系统出口 + 纯 inbound 限定规则）原样回写，排在最前。
  //    MITM 域名规则（outbound=to-mitmproxy）特殊处理：用 cfg.mitmRules 更新 domain_suffix 等字段，
  //    保留原规则的 inbound 限定（如 inbound:["tun-in"]），避免字段丢失。
  for (const sr of (cfg.rules ?? [])) {
    // clash_mode 规则由下方按 settings.clashMode 统一生成，跳过旧的避免重复。
    if (sr.raw && sr.raw.clash_mode !== undefined) continue
    // mixed-back 回注兜底规则由下方统一注入到【用户规则之后】，这里跳过旧的，避免它排在最前把回注流量短路。
    if (sr.raw && Array.isArray(sr.raw.inbound) && sr.raw.inbound.includes('mixed-back')) continue
    const cloned = JSON.parse(JSON.stringify(sr.raw))
    if (sr.outbound === MITM_NODE_TAG && cfg.mitmRules) {
      for (const mt of ['domain_suffix', 'domain_keyword', 'domain', 'domain_regex', 'ip_cidr', 'geosite', 'geoip', 'protocol']) {
        delete cloned[mt]
      }
      const byType = new Map<string, string[]>()
      for (const gr of cfg.mitmRules) {
        const exist = byType.get(gr.matchType) ?? []
        for (const v of gr.values) if (!exist.includes(v)) exist.push(v)
        byType.set(gr.matchType, exist)
      }
      for (const [mt, vals] of byType) {
        if (vals.length > 0) cloned[mt] = vals
      }
    }
    newRules.push(cloned)
  }

  // 2. 用户规则：严格按 orderedRules 顺序输出（顺序 = 优先级），排在系统规则之后。
  //    平铺 —— 每条 orderedRule 独立输出一条 route 规则（不按出口/类型合并打包）。
  //    这样序列化是纯 1:1 确定性映射：规则页看到什么、配置就是什么，无合并变形、稳定可预测。
  for (const r of (cfg.orderedRules ?? [])) {
    const val = (r.value ?? '').trim()
    if (!val) continue
    const rule: Record<string, any> = { outbound: r.outbound }
    rule[r.matchType] = [val]
    newRules.push(rule)
  }

  // 系统骨架保证（MITM 链路必需，缺则注入）——无论导入什么格式、怎么编辑都成立：
  // 1) sniff 必须在最前：嗅探 TUN 加密流量的 SNI/Host，否则按域名匹配的规则（含抓包域名→to-mitmproxy）永远命不中。
  if (!newRules.some(r => r.action === 'sniff')) {
    newRules.unshift({ action: 'sniff' })
  }

  // 主代理出口解析：骨架里的 Global / mixed-back 回环出口必须指向【配置里真实存在的出口】，
  //   否则 sing-box 启动校验 "outbound 不存在" 会直接拒绝启动 → 端口全不监听 → 完全不通。
  //   模板里叫 proxy-node，但真实订阅的主组多为 selector 代理组（如 "🔰 节点选择"，也是 route.final）。
  //   优先级：存在 proxy-node → route.final → 第一个代理组(selector/urltest) → 第一个真实节点 → 兜底 proxy-node。
  const outList: any[] = Array.isArray(obj.outbounds) ? obj.outbounds : []
  const outTags = new Set(outList.map((o: any) => o?.tag).filter(Boolean))
  const resolveMainProxyOut = (): string => {
    if (outTags.has(PROXY_OUT)) return PROXY_OUT
    const finalTag = obj.route?.final
    if (typeof finalTag === 'string' && outTags.has(finalTag)) return finalTag
    const grp = outList.find((o: any) => o?.type === 'selector' || o?.type === 'urltest')
    if (grp?.tag) return grp.tag
    const real = outList.find((o: any) => o?.tag && !['direct', 'block', 'dns'].includes(o?.type) && o.tag !== MITM_NODE_TAG)
    if (real?.tag) return real.tag
    return PROXY_OUT
  }
  const mainProxyOut = resolveMainProxyOut()

  // 代理模式：注入 clash_mode 分流规则（放在 sniff 之后、其余规则之前，优先级最高）。
  //   Direct 模式 → 全部直连；Global 模式 → 全部走主代理出口；Rule 模式两条都不匹配，走下方常规规则。
  const clashModeRules = [
    { clash_mode: 'Direct', outbound: 'direct' },
    { clash_mode: 'Global', outbound: mainProxyOut },
  ]
  const insertAt = (newRules[0] && newRules[0].action === 'sniff') ? 1 : 0
  newRules.splice(insertAt, 0, ...clashModeRules)

  // 2) mixed-back 回注兜底规则：go-mitmproxy 解密后回注 sing-box 的流量必须能出网，否则链路断。
  //    关键：放在【所有用户域名规则之后】当兜底，而不是最前面。
  //    这样解密回注的流量会先按上方用户域名规则【严格分流】（如 ipdata → AI代理-Anthropic），
  //    都不匹配时才走此兜底到主代理出口。防回环靠抓包规则的 inbound:tun-in 限定（mixed-back 流量不会再被抓去解密）。
  //    先移除可能残留的 mixed-back 规则，再统一追加到末尾，保证位置正确。
  for (let i = newRules.length - 1; i >= 0; i--) {
    const r = newRules[i]
    if (Array.isArray(r.inbound) && r.inbound.includes('mixed-back') &&
        !r.domain && !r.domain_suffix && !r.domain_keyword && !r.domain_regex) {
      newRules.splice(i, 1)
    }
  }
  newRules.push({ inbound: ['mixed-back'], outbound: mainProxyOut })

  // 写回 route.rules，保留 route 顶层其他字段（final/auto_detect_interface 等）
  obj.route = { ...(obj.route || {}) }
  // 开启进程查找：让 Clash API /connections 能上报每条连接的进程名（连接页用）。
  obj.route.find_process = true
  if (newRules.length > 0) {
    obj.route.rules = newRules
  } else {
    delete obj.route.rules
  }

  // 输出给 sing-box 的 JSON 必须兼容其严格解析器：
  // 移除所有 _comment 字段（sing-box 不认未知字段会报错）。
  // 内部 config 对象仍保留 _comment（供用户阅读），只影响序列化输出的字符串。
  stripComments(obj)

  return JSON.stringify(obj, null, 2)
}

// stripComments 递归移除对象内所有 _comment 字段（sing-box 严格 JSON 不允许未知字段）。
function stripComments(o: any): void {
  if (Array.isArray(o)) {
    for (const item of o) stripComments(item)
  } else if (o && typeof o === 'object') {
    delete o._comment
    for (const v of Object.values(o)) stripComments(v)
  }
}

// 空配置（初始化用）。
export function blankConfig(): SingBoxConfig {
  return { nodes: [], systemNodes: [], groups: [], orderedRules: [], rules: [], mitmRules: [], settings: { tunEnabled: false, tunAddress: '', tunMtu: 1500, tunStack: 'system', tunInterfaceName: '', tunAutoRoute: true, tunStrictRoute: true, mixedBackPort: 9788, logLevel: 'info', dnsServers: [], dnsStrategy: '', dnsRules: [], dnsFinal: '', dnsRawRules: [], clashMode: 'Rule' }, raw: {} }
}

// config/types.ts — sing-box 配置的结构化数据模型（类型定义 + 相关常量）。
// 纯类型/常量，无逻辑；解析/序列化见 serde.ts，协议表单字段见 fields.ts。

export type ProtocolType =
  | 'direct' | 'block' | 'dns' | 'http'
  | 'vmess' | 'vless' | 'trojan' | 'shadowsocks'
  | 'hysteria2' | 'wireguard'

// 一个节点（outbound，非组类型）。raw 保留图形化未覆盖的字段。
export interface SingBoxNode {
  type: string
  tag: string
  server?: string
  server_port?: number
  rules?: GroupRule[]         // 该节点的路由规则（outbound 自动绑节点 tag）
  // 其余协议特有字段放在 raw 里透传
  raw?: Record<string, any>
}

// 一个代理组（selector/urltest 类型 outbound）。
// selector：手动选择；urltest：自动测速选最快。
// 一个节点可被多个组引用（多对多，由 outbounds tag 列表实现）。
export interface SelectorGroup {
  type: 'selector' | 'urltest'
  tag: string
  outbounds: string[]         // 候选节点 tag 列表
  default?: string            // selector 默认选中（仅 selector）
  url?: string                // 测速 URL（仅 urltest）
  interval?: string           // 测速间隔（仅 urltest）
  rules?: GroupRule[]         // 该组的路由规则（outbound 自动绑组 tag）
  raw?: Record<string, any>   // 其余字段透传（tolerance/interrupt_exist_connections 等）
}

// 代理组的路由规则（轻量编辑模型）。
// 一条 GroupRule = 一种匹配类型 + 域名/IP 列表（UI 用 textarea 每行一个）。
// 序列化时 outbound 自动设为所属组的 tag。
export type RuleMatchType =
  | 'domain_suffix' | 'domain_keyword' | 'domain' | 'domain_regex'
  | 'ip_cidr' | 'geosite' | 'geoip' | 'protocol'

export interface GroupRule {
  matchType: RuleMatchType
  values: string[]            // 域名/IP/协议值列表（每行一个）
}

// 一条有序路由规则（RuleList 的编辑单元）。orderedRules 的顺序 = sing-box 匹配优先级
// （route.rules 自上而下、命中即停）。每条 = 一个值 + 匹配类型 + 出口。
export interface OrderedRule {
  value: string
  matchType: RuleMatchType
  outbound: string
}

// 一条 route rule（只读概览为主）。
export interface SingBoxRule {
  raw: Record<string, any>   // 整条规则原样保留
  // 概览用：抽取的展示字段
  desc: string               // 人类可读描述
  outbound: string
  isMitmRule: boolean        // 是否 MITM 关键规则
}

// 一个 DNS 服务器：地址 + 出口（detour：走直连还是走代理解析）+ 可选 tag。
export interface DnsServer {
  address: string            // 如 223.5.5.5 / https://1.1.1.1/dns-query / tls://8.8.8.8
  detour: string             // 解析该 DNS 时的出口 tag：direct(直连) / proxy-node(走代理) / 组名
  tag?: string               // sing-box 内部标识（dns.rules 按 tag 引用；加载时自动补全）
}

// DNS 分流规则的匹配类型（对齐 sing-box dns.rules 支持的域名类匹配器）。
export type DnsMatchType =
  | 'domain_suffix' | 'domain' | 'domain_keyword' | 'domain_regex' | 'rule_set'

// reject 目标的特殊 server 值（表示 action:"reject"，拒绝解析，常用于广告拦截）。
export const DNS_REJECT = '__reject__'

// Clash 代理模式：Rule(规则)/Global(全局代理)/Direct(直连)。
// 对应 experimental.clash_api.default_mode，以及 route.rules 的 clash_mode 匹配。
export const CLASH_MODES = ['Rule', 'Global', 'Direct'] as const
export type ClashMode = typeof CLASH_MODES[number]
// 主代理出口 tag（Global 模式全量走此出口；订阅导入后为 selector 代理组）。
export const PROXY_OUT = 'proxy-node'

// 一条 DNS 分流规则：匹配某类域名 → 指定用哪台 DNS 服务器解析（server=DNS 服务器 tag，
// 或 DNS_REJECT 表示拒绝解析）。dnsRules 顺序 = 优先级（dns.rules 自上而下、命中即停）。
export interface DnsRule {
  value: string
  matchType: DnsMatchType
  server: string
}

// 设置（TUN/DNS/端口等常用项）。
export interface SingBoxSettings {
  tunEnabled: boolean
  tunAddress: string
  tunMtu: number
  tunStack: string
  tunInterfaceName: string   // 虚拟网卡名（如 utun100）
  tunAutoRoute: boolean      // 自动配置路由
  tunStrictRoute: boolean    // 严格路由
  mixedBackPort: number      // mixed-back inbound 端口
  logLevel: string
  dnsServers: DnsServer[]    // DNS 服务器列表（地址 + detour）
  dnsStrategy: string        // 解析策略：''(默认)/prefer_ipv4/prefer_ipv6/ipv4_only/ipv6_only
  dnsRules: DnsRule[]        // DNS 分流规则（有序，顺序=优先级）：域名 → 指定 DNS 服务器
  dnsFinal: string           // 兜底 DNS 服务器 tag（无规则命中时用；空=用第一台）
  dnsRawRules: any[]         // 无法图形化的高级 DNS 规则（原样保真，序列化时追加在结构化规则之后）
  clashMode: string          // 代理模式：Rule/Global/Direct（写入 clash_api.default_mode + 注入 clash_mode 规则）
}

// 完整配置的结构化形态。
export interface SingBoxConfig {
  nodes: SingBoxNode[]        // 用户可见节点（vmess/vless/trojan/...，不含系统节点）
  systemNodes: SingBoxNode[]  // 系统隐身节点（to-mitmproxy/direct/dns-out/...，用户不可见）
  groups: SelectorGroup[]     // 代理组（selector/urltest）
  orderedRules: OrderedRule[] // 用户路由规则（有序，顺序=优先级；RuleList 编辑，序列化按序输出）
  rules: SingBoxRule[]        // 系统规则（含 MITM/inbound 限定，raw 保真）
  mitmRules: GroupRule[]      // MITM 解密域名规则（从 to-mitmproxy 系统规则提取，首页编辑）
  settings: SingBoxSettings
  raw: Record<string, any>   // 顶层未图形化字段（log/dns/experimental 等原样）
}

// 系统保留节点 tag（基础设施，对用户隐身，不可见不可删）。
// 这些节点由配置模板/后台自动维护，用户只看到真实代理节点。
// 注：sing-box 1.13 移除了 block/dns 特殊 outbound（改为 rule action），此处只保留实际存在的。
export const SYSTEM_NODE_TAGS = new Set(['to-mitmproxy', 'direct'])

// MITM 隐身节点 tag（规则自动注入的目标）。
export const MITM_NODE_TAG = 'to-mitmproxy'

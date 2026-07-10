// configOps.ts —— 配置结构化修改的【集中入口】，分层编写。
//
// 目的：所有会影响【引用一致性】的配置修改（重命名 / 删除 节点或代理组）都必须走这里。
// 杜绝"改了名字或删了东西，但 groups / rules / dns / route.final 里的引用没跟着变"
// —— 那会造成 sing-box 启动时校验 "outbound 不存在" 直接拒绝启动、代理完全不通。
//
// 分层：
//   L1  rewriteOutboundRefs —— 单一事实源：枚举并改写 config 里【所有出口(outbound tag)引用出现的位置】。
//                              改名、删除都基于它，保证覆盖全、不散落、不遗漏。
//   L2  renameOutbound / removeOutbound —— 基于 L1 的语义化操作。
//   L3  （调用方）useCrudDialog 的 onRename / beforeRemove 钩子直接调 L2，UI 层不再各自清引用。

import { DNS_REJECT, type SingBoxConfig } from './configModel'

// ───────────────────────────────────────────────────────────────────────────
// L1：出口引用改写（单一事实源）
// rewrite 回调对每个"出口 tag 引用"返回：
//   - 新字符串 → 替换该引用
//   - 相同字符串 → 不变
//   - null → 删除该引用（按所在位置采用合适的删除语义）
//
// 「出口 tag 在配置里出现的全部位置」清单（增改这里 = 全局生效）：
//   1) groups[].outbounds[]        代理组成员（节点/子组 tag）
//   2) groups[].default            selector 默认选中（= 某个成员）
//   3) orderedRules[].outbound     用户有序路由规则的出口
//   4) rules[].raw.outbound        系统规则的出口（raw 是序列化真正用的）
//   5) settings.dnsServers[].detour DNS 解析走哪个出口
//   6) raw.route.final             兜底出口（序列化时从 raw 带出保留）
// 注意：settings.dnsFinal 是 DNS 服务器 tag，不是出口 tag，故不在此处理。
export function rewriteOutboundRefs(cfg: SingBoxConfig, rewrite: (tag: string) => string | null): void {
  // 1) + 2) 代理组成员列表与默认选中
  for (const g of cfg.groups ?? []) {
    const next: string[] = []
    for (const m of g.outbounds ?? []) {
      const r = rewrite(m)
      if (r) next.push(r)           // null=删除该成员；空串视为无效同样丢弃
    }
    g.outbounds = next
    if (g.default) {
      const r = rewrite(g.default)
      g.default = r ?? (next[0] ?? '')  // 默认项被删 → 退回第一个成员
    }
  }

  // 3) 用户有序规则：出口被删 → 整条规则失效，丢弃（避免留下悬空引用）
  if (cfg.orderedRules) {
    cfg.orderedRules = cfg.orderedRules.filter(r => {
      const nr = rewrite(r.outbound)
      if (nr === null) return false
      r.outbound = nr
      return true
    })
  }

  // 4) 系统规则：以 raw.outbound 为准；无 outbound 的动作规则(sniff/hijack-dns/纯 inbound)跳过
  if (cfg.rules) {
    cfg.rules = cfg.rules.filter(sr => {
      const ob = sr.raw?.outbound
      if (typeof ob !== 'string') return true
      const nr = rewrite(ob)
      if (nr === null) return false
      sr.raw.outbound = nr
      sr.outbound = nr
      return true
    })
  }

  // 5) DNS 服务器 detour
  for (const d of cfg.settings?.dnsServers ?? []) {
    if (d.detour) {
      const r = rewrite(d.detour)
      d.detour = r ?? ''            // 被删 → 空(用默认路由)
    }
  }

  // 6) route.final（从 cfg.raw 带出，serialize 保留）
  const route = cfg.raw?.route
  if (route && typeof route.final === 'string') {
    const r = rewrite(route.final)
    if (r === null) delete route.final   // 兜底出口被删 → 交给 serialize 的 resolveMainProxyOut 兜底
    else route.final = r
  }
}

// ───────────────────────────────────────────────────────────────────────────
// L2：语义化操作
// 重命名出口：把所有对 oldTag 的引用改成 newTag（不动实体本身的 tag，由调用方设置）。
export function renameOutbound(cfg: SingBoxConfig, oldTag: string, newTag: string): void {
  if (!oldTag || !newTag || oldTag === newTag) return
  rewriteOutboundRefs(cfg, t => (t === oldTag ? newTag : t))
}

// 删除出口：清除所有对 tag 的引用（实体本身由调用方从 nodes/groups 列表移除）。
export function removeOutbound(cfg: SingBoxConfig, tag: string): void {
  if (!tag) return
  rewriteOutboundRefs(cfg, t => (t === tag ? null : t))
}

// ───────────────────────────────────────────────────────────────────────────
// DNS 服务器 tag 引用（与出口同构：L1 单一事实源 → L2 语义化操作）
//
// L1：DNS 服务器 tag 在配置里出现的全部位置清单：
//   1) settings.dnsRules[].server     分流规则用哪台 DNS 解析（DNS_REJECT 是特殊动作值，跳过）
//   2) settings.dnsFinal              兜底 DNS 服务器
//   3) settings.dnsRawRules[].server  高级 DNS 规则（尽力处理顶层 server 字段）
// 说明：serialize 时 dns 段整体由 settings 重建，故引用只在 settings 内，不涉及 raw。
export function rewriteDnsServerRefs(cfg: SingBoxConfig, rewrite: (tag: string) => string | null): void {
  const st = cfg.settings
  if (!st) return

  // 1) 分流规则
  if (st.dnsRules) {
    st.dnsRules = st.dnsRules.filter(r => {
      if (r.server === DNS_REJECT) return true    // 拒绝解析：动作值，非服务器 tag
      const nr = rewrite(r.server)
      if (nr === null) return false               // 服务器被删 → 规则失效丢弃
      r.server = nr
      return true
    })
  }

  // 2) 兜底服务器
  if (st.dnsFinal) {
    const nr = rewrite(st.dnsFinal)
    st.dnsFinal = nr ?? ''                         // 被删 → 空(sing-box 用第一台)
  }

  // 3) 高级 DNS 规则：只处理带顶层 server 字段的；引用被删 → 丢弃该条（避免悬空），无 server 字段的原样保留。
  if (st.dnsRawRules) {
    st.dnsRawRules = st.dnsRawRules.filter(raw => {
      if (!raw || typeof raw.server !== 'string') return true
      const nr = rewrite(raw.server)
      if (nr === null) return false
      raw.server = nr
      return true
    })
  }
}

// L2：重命名 DNS 服务器（把所有对 oldTag 的引用改为 newTag；实体 tag 由调用方设置）。
export function renameDnsServer(cfg: SingBoxConfig, oldTag: string, newTag: string): void {
  if (!oldTag || !newTag || oldTag === newTag) return
  rewriteDnsServerRefs(cfg, t => (t === oldTag ? newTag : t))
}

// L2：删除 DNS 服务器（清除所有对 tag 的引用；实体本身由调用方从 dnsServers 列表移除）。
export function removeDnsServer(cfg: SingBoxConfig, tag: string): void {
  if (!tag) return
  rewriteDnsServerRefs(cfg, t => (t === tag ? null : t))
}

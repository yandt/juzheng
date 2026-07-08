// src/types/clash.ts — Clash 兼容 API 的前端类型（非后端生成，HTTP 解析）。
//
// useClashApi 通过 127.0.0.1:9090 HTTP 查询代理组，返回结构由前端解析。

/** Clash API 返回的代理组信息。 */
export interface ProxyGroup {
  name: string
  type: string
  now: string
  all: string[]
}

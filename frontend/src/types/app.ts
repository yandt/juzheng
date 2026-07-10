// src/types/app.ts — 前端独有应用类型（非后端 binding 生成）。
//
// 集中管理散落在 App.vue/composables 的类型定义，便于跨文件复用。
// 后端生成的类型仍在 bindings/.../models.ts，本文件只放前端独有。

/** 侧栏页面 key 联合类型（App.vue 的 activePage 用）。 */
export type Page = 'home' | 'subscriptions' | 'traffic' | 'connections' | 'groups' | 'rules' | 'settings' | 'source'

/** 侧栏菜单项定义。 */
export interface MenuItem {
  key: Page
  label: string
  icon: string
}

/** 首页卡片定义（预设卡片的标题/图标，CARD_DEFS 数组用）。 */
export interface CardDef {
  key: string
  title: string
  icon: string
}

/**
 * 节点/协议类型 → 分类标签（全局统一）。
 * 用于节点类型 tag 的颜色映射，所有显示节点类型的地方共用。
 * 分类：基础设施(direct/block/dns/http) / 代理协议(*)。
 */
export const NODE_TYPE_CATEGORY: Record<string, 'infra' | 'proxy'> = {
  direct: 'infra',
  block: 'infra',
  dns: 'infra',
  http: 'infra',
  vmess: 'proxy',
  vless: 'proxy',
  trojan: 'proxy',
  shadowsocks: 'proxy',
  hysteria2: 'proxy',
  wireguard: 'proxy',
}

/** 节点类型 → 分类（未知类型默认归 proxy）。 */
export function nodeTypeCategory(type: string): 'infra' | 'proxy' {
  return NODE_TYPE_CATEGORY[type] ?? 'proxy'
}

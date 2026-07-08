// useMeta: 首页卡片配置（显隐 + 排序），单例 composable。
// 卡片配置存在 ~/.juzheng/meta.json。

import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { t } from '../i18n'
import * as subscription from '../../bindings/github.com/zhanghui/juzheng/subscriptionservice'
import type { AppMeta } from '../../bindings/github.com/zhanghui/juzheng/models'
import type { CardConfig } from '../../bindings/github.com/zhanghui/juzheng/internal/singboxcfg/models'
import type { CardDef } from '../types/app'

// 单例状态
const meta = ref<AppMeta>({ activeScheme: '', cards: [] })

// 预设卡片定义（key → 标题 i18n key / 图标）。title 存 i18n key，由界面层 t() 翻译，
// 以保证切换语言时标题响应式更新（模块级不能直接调用 t()）。CardDef 类型见 types/app。
export const CARD_DEFS: CardDef[] = [
  { key: 'current-scheme', title: 'home.cardCurrentScheme', icon: 'Files' },
  { key: 'proxy-mode', title: 'home.cardProxyMode', icon: 'Operation' },
  { key: 'proxy-groups', title: 'home.cardProxyGroups', icon: 'Connection' },
  { key: 'quick-toggles', title: 'home.cardNetwork', icon: 'Switch' },
  { key: 'traffic', title: 'home.cardTraffic', icon: 'DataLine' },
]

// 可见且排序后的卡片列表
const visibleCards = computed<CardConfig[]>(() =>
  (meta.value.cards ?? [])
    .filter(c => c.visible)
    .sort((a, b) => a.order - b.order)
)

async function loadMeta() {
  try {
    meta.value = await subscription.GetMeta() as unknown as AppMeta
    if (!meta.value.cards) meta.value.cards = []
    // 补齐新卡片：CARD_DEFS 中存在但 meta 未记录的（如新增功能卡片）追加进来，默认显示。
    let added = false
    let maxOrder = meta.value.cards.reduce((m, c) => Math.max(m, c.order), -1)
    for (const def of CARD_DEFS) {
      if (!meta.value.cards.find(c => c.key === def.key)) {
        meta.value.cards.push({ key: def.key, visible: true, order: ++maxOrder })
        added = true
      }
    }
    if (added) await saveMeta()
  } catch { /* 静默 */ }
}

async function saveMeta() {
  try {
    await subscription.SetMeta(meta.value)
  } catch (e: any) {
    ElMessage.error(t('msg.cardConfigSaveFailed', { error: e?.message || e }))
  }
}

// 切换某卡片显隐
async function toggleCard(key: string) {
  const c = (meta.value.cards ?? []).find(x => x.key === key)
  if (c) {
    c.visible = !c.visible
    await saveMeta()
  }
}

// 移动卡片顺序（drag delta）
async function moveCard(key: string, delta: number) {
  const sorted = [...(meta.value.cards ?? [])].sort((a, b) => a.order - b.order)
  const idx = sorted.findIndex(c => c.key === key)
  const newIdx = idx + delta
  if (newIdx < 0 || newIdx >= sorted.length) return
  ;[sorted[idx], sorted[newIdx]] = [sorted[newIdx], sorted[idx]]
  sorted.forEach((c, i) => c.order = i)
  await saveMeta()
}

export function useMeta() {
  return { meta, visibleCards, loadMeta, saveMeta, toggleCard, moveCard }
}

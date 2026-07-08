// 轻量 i18n（无第三方依赖）：响应式 locale + 字典 + t()。
// t() 在模板里调用时读取 locale.value（响应式），切换语言即自动重渲染。
// 字典按页拆分到 locales/<page>.ts，这里汇总合并；新增页面时在此追加一行 import + 展开。
import { ref } from 'vue'
import { zh as commonZh, en as commonEn } from './locales/common'
import { zh as homeZh, en as homeEn } from './locales/home'
import { zh as subsZh, en as subsEn } from './locales/subscriptions'
import { zh as trafficZh, en as trafficEn } from './locales/traffic'
import { zh as groupsZh, en as groupsEn } from './locales/groups'
import { zh as rulesZh, en as rulesEn } from './locales/rules'
import { zh as sourceZh, en as sourceEn } from './locales/source'
import { zh as componentsZh, en as componentsEn } from './locales/components'
import { zh as messagesZh, en as messagesEn } from './locales/messages'

export type Lang = 'zh' | 'en'

export const locale = ref<Lang>('zh')

type Dict = Record<string, string>

const zh: Dict = {
  ...commonZh, ...homeZh, ...subsZh, ...trafficZh, ...groupsZh,
  ...rulesZh, ...sourceZh, ...componentsZh, ...messagesZh,
}
const en: Dict = {
  ...commonEn, ...homeEn, ...subsEn, ...trafficEn, ...groupsEn,
  ...rulesEn, ...sourceEn, ...componentsEn, ...messagesEn,
}

const messages: Record<Lang, Dict> = { zh, en }

// 翻译函数：查不到 key 时回退到中文，再回退到 key 本身。
// 支持 {name} 形式的占位符插值：t('key', { name: 'x' })。
export function t(key: string, params?: Record<string, string | number>): string {
  let s = messages[locale.value]?.[key] ?? messages.zh[key] ?? key
  if (params) for (const k in params) s = s.replace(new RegExp(`\\{${k}\\}`, 'g'), String(params[k]))
  return s
}

// usePrefs: 界面偏好（语言 / 主题），持久化到 localStorage，全局单例。
// 主题：dark / light / auto（auto 跟随系统 prefers-color-scheme）。
import { ref, watch } from 'vue'
import { locale, type Lang } from '../i18n'

export type ThemeMode = 'dark' | 'light' | 'auto'

const LS_THEME = 'juzheng.theme'
const LS_LANG = 'juzheng.lang'

const theme = ref<ThemeMode>((localStorage.getItem(LS_THEME) as ThemeMode) || 'dark')
const language = ref<Lang>((localStorage.getItem(LS_LANG) as Lang) || 'zh')
// 当前实际生效的是否深色（供需要按主题切换的组件用，如 Codemirror 代码主题）。auto 时反映系统。
const isDark = ref(true)

const mql = typeof window !== 'undefined' && window.matchMedia
  ? window.matchMedia('(prefers-color-scheme: dark)')
  : null

// 应用主题：dark → html.dark；light → html.light；auto → 跟随系统。
function applyTheme() {
  const dark = theme.value === 'dark' || (theme.value === 'auto' && !!mql?.matches)
  isDark.value = dark
  const root = document.documentElement
  root.classList.toggle('dark', dark)
  root.classList.toggle('light', !dark)
}

let initialized = false
function initOnce() {
  if (initialized) return
  initialized = true
  locale.value = language.value
  applyTheme()
  // 系统主题变化时，若为 auto 则跟随。
  mql?.addEventListener?.('change', () => { if (theme.value === 'auto') applyTheme() })
  watch(theme, () => { localStorage.setItem(LS_THEME, theme.value); applyTheme() })
  watch(language, () => { localStorage.setItem(LS_LANG, language.value); locale.value = language.value })
}

export function usePrefs() {
  initOnce()
  return { theme, language, isDark }
}

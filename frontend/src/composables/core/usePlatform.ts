// composables/core/usePlatform.ts — 平台判断（单例，收口 System.IsMac 散落调用）。
//
// App.vue / AppSidebar 等多处用 System.IsMac()，集中到本 composable 统一管理。
// 一处定义、跨组件共享，避免散落的 System import。

import { computed } from 'vue'
import { System } from '@wailsio/runtime'

/** macOS 判断（三按钮在左上）。 */
const isMac = computed(() => System.IsMac())
/** Windows/Linux（三按钮在右上）。 */
const isWinLike = computed(() => !System.IsMac())

/**
 * 平台判断单例。
 * 返回响应式平台标识，供布局避让/条件渲染复用。
 */
export function usePlatform() {
  return { isMac, isWinLike }
}

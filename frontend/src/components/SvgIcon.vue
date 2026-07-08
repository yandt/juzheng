<script setup lang="ts">
// SvgIcon：自定义 SVG 图标统一管理组件。
//
// 用法：<SvgIcon name="drag" /> —— 按 name 索引内置 SVG。
// 新增自定义 SVG：在下面 ICONS 对象里加一项（name → path 数据）。
// 今后所有用户提供的 SVG 均通过本组件管理，便于复用 + 统一尺寸/颜色。

defineProps<{
  /** 图标名（对应 ICONS 的 key） */
  name: string
  /** 尺寸（px），默认 16 */
  size?: number | string
  /** 颜色，默认 currentColor（继承父元素） */
  color?: string
}>()

// 内置自定义 SVG 图标库（viewBox 统一 0 0 1024 1024，只存 path d 数据）。
// 新增图标在这里加一项即可，全项目复用。
const ICONS: Record<string, string> = {
  // 拖拽手柄（六点）
  drag: 'M469.333333 256a85.333333 85.333333 0 1 1-85.333333-85.333333 85.333333 85.333333 0 0 1 85.333333 85.333333z m-85.333333 170.666667a85.333333 85.333333 0 1 0 85.333333 85.333333 85.333333 85.333333 0 0 0-85.333333-85.333333z m0 256a85.333333 85.333333 0 1 0 85.333333 85.333333 85.333333 85.333333 0 0 0-85.333333-85.333333z m256-341.333334a85.333333 85.333333 0 1 0-85.333333-85.333333 85.333333 85.333333 0 0 0 85.333333 85.333333z m0 85.333334a85.333333 85.333333 0 1 0 85.333333 85.333333 85.333333 85.333333 0 0 0-85.333333-85.333333z m0 256a85.333333 85.333333 0 1 0 85.333333 85.333333 85.333333 85.333333 0 0 0-85.333333-85.333333z',
  // 实心三角（播放/开始）—— 放大字形填满画框，与描边图标视觉大小一致
  play: 'M272 168 L856 512 L272 856 Z',
  // 圆角实心方块（停止）—— 放大字形填满画框
  stop: 'M336 272 H688 A64 64 0 0 1 752 336 V688 A64 64 0 0 1 688 752 H336 A64 64 0 0 1 272 688 V336 A64 64 0 0 1 336 272 Z',
}

defineOptions({ inheritAttrs: false })
</script>

<template>
  <svg
    class="svg-icon"
    :width="size ?? 16"
    :height="size ?? 16"
    viewBox="0 0 1024 1024"
    xmlns="http://www.w3.org/2000/svg"
    :style="{ color: color ?? 'currentColor' }"
    v-bind="$attrs"
  >
    <path :d="ICONS[name] ?? ''" fill="currentColor" />
  </svg>
</template>

<style scoped>
.svg-icon { display: inline-block; flex-shrink: 0; }
</style>

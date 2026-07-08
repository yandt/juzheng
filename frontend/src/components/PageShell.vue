<script setup lang="ts">
/**
 * 页面骨架：统一所有功能页的布局结构。
 *
 * ┌─ page-header（始终显示，sticky 固定顶部）──────────┐
 * │ [h3 title] [title-extra slot]   [actions slot]    │
 * └────────────────────────────────────────────────────┘
 * ┌─ page-body（可滚动）──────────────────────────────┐
 * │  默认 slot（页面主体）                             │
 * └────────────────────────────────────────────────────┘
 *
 * - title：页面标题文字
 * - title-extra slot：标题右侧紧邻的自定义展示区（tag/状态/输入框）
 * - actions slot：右侧操作区（仅放 IconButton，正方形图标 + tooltip）
 */
defineProps<{
  title: string
}>()
</script>

<template>
  <div class="page-shell">
    <!-- 始终显示的顶部标题栏 -->
    <header class="page-header">
      <!-- 左侧：标题 + 紧邻的自定义展示 -->
      <div class="title-area">
        <h3 class="page-title">{{ title }}</h3>
        <div class="title-extra"><slot name="title-extra" /></div>
      </div>
      <!-- 右侧：操作区（仅放 IconButton） -->
      <div class="actions-area"><slot name="actions" /></div>
    </header>
    <!-- 主体内容（可滚动） -->
    <main class="page-body">
      <slot />
    </main>
  </div>
</template>

<style scoped>
.page-shell {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}
/* 始终显示的标题栏：sticky 固定，滚动时停留顶部。
   统一 padding 16px —— 三按钮在窗口左/右上角（sidebar 上方或 Windows 窗口右上），
   不与 page-header 区域重叠，无需任何避让变量。 */
.page-header {
  display: flex;
  align-items: center;
  gap: 12px;
  flex: 0 0 auto;
  height: var(--titlebar-h, 56px);
  padding: 0 16px;
  background: var(--jz-surface-2);
  border-bottom: 1px solid var(--jz-border);
  /* 关键：sticky 让 header 在 page-body 滚动时始终可见 */
  position: sticky;
  top: 0;
  z-index: 10;
}
.title-area {
  display: flex;
  align-items: center;
  gap: 10px;
  /* margin-right:auto 强制 title-area 贴左，actions-area 推到最右 */
  margin-right: auto;
  min-width: 0; /* 允许收缩，避免内容撑爆 */
}
.page-title {
  margin: 0;
  color: var(--jz-text);
  font-size: 16px;
  font-weight: 600;
  flex: 0 0 auto;
}
.title-extra {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.actions-area {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 0 0 auto;
}
/* 主体：占满剩余空间，内部滚动 */
.page-body {
  flex: 1;
  padding: 12px 16px 16px;
  overflow-y: auto;
  min-height: 0; /* flex 子项允许收缩，让 overflow 生效 */
}
</style>

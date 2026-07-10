<script setup lang="ts">
import { computed } from 'vue'
import { t } from '../i18n'
import { usePlatform } from '../composables/core/usePlatform'
import { useApp } from '../composables/service/useApp'

defineProps<{
  pages: { key: string; label: string; icon: string }[]
  activePage: string
}>()
const emit = defineEmits<{ (e: 'select', p: string): void }>()

// 平台判断（收口到 core/usePlatform）
const { isMac } = usePlatform()

// 应用元信息（收口到 service/useApp，binding 调用不再泄漏到组件）
const { appInfo, loadAppInfo } = useApp()
loadAppInfo()
</script>

<template>
  <el-aside class="sidebar" width="180px">
    <!-- 顶部品牌行：与窗口三按钮同高，水平布局。
         macOS：左侧留出三按钮空间（--titlebar-btn-left），Logo 紧跟其后；
         Windows/Linux：无三按钮占位，Logo 直接左对齐。 -->
    <div class="brand" :class="{ 'brand-mac': isMac, 'brand-win': !isMac }">
      <img src="/logo-silhouette.png" alt="Juzheng" class="brand-logo" />
      <span class="brand-name">Juzheng</span>
    </div>

    <!-- 导航 -->
    <el-menu :default-active="activePage" class="nav" @select="(k: string) => emit('select', k)">
      <el-menu-item v-for="p in pages" :key="p.key" :index="p.key">
        <el-icon><component :is="p.icon" /></el-icon>
        <span>{{ p.label }}</span>
      </el-menu-item>
    </el-menu>

    <!-- 版本号（居中，固定底部）；dev 模式显示醒目调试标记 -->
    <div class="version">
      <div v-if="appInfo.isDev" class="dev-badge">{{ t('comp.devBadge') }}</div>
      <div class="ver-text">Juzheng v{{ appInfo.appVersion || '...' }}</div>
    </div>
  </el-aside>
</template>

<style scoped>
.sidebar {
  background: var(--jz-surface-2);
  border-right: 1px solid var(--jz-border);
  display: flex;
  flex-direction: column;
  /* 关键：交叉轴不拉伸子项，防止 brand/nav 被纵向撑开 */
  align-items: stretch;
  height: 100vh;
  overflow: hidden;
}
/* 品牌行：在三按钮区下方，高度与菜单项一致（44px），视觉上作为菜单首项。
   macOS：上方留出三按钮区高度（--titlebar-h），整行可拖拽。
   Windows/Linux：无三按钮占位，Logo 行紧贴顶部。 */
.brand {
  display: flex; align-items: center; gap: 8px;
  height: 44px;
  flex: 0 0 44px;
  padding: 0 18px;
  color: var(--jz-text);
  overflow: hidden;
}
/* macOS：上方留出三按钮区高度（~28px），让 Logo 在三按钮下方 */
.brand-mac {
  margin-top: 28px;
  -webkit-app-region: drag;
}
/* Windows/Linux：Logo 行紧贴顶部，高度与右侧 page-header 一致（--titlebar-h=56px），
   使 Logo 行与页面标题栏在同一水平基线上、其下的首个菜单项与 page-body 顶部对齐。 */
.brand-win {
  margin-top: 0;
  height: var(--titlebar-h, 56px);
  flex: 0 0 var(--titlebar-h, 56px);
}
/* brand 内交互元素（logo 点击等）取消拖拽，避免误操作 */
.brand img, .brand span { -webkit-app-region: no-drag; }
.brand-logo { width: 22px; height: 22px; flex: 0 0 22px; }
.brand-name { font-weight: 700; font-size: 15px; }

.nav { background: transparent; border-right: none; padding: 0 !important; margin: 0 !important; flex: 0 0 auto; }
/* 关键：消除 el-menu 所有默认内外边距，菜单项紧贴 brand */
.nav :deep(.el-menu) { border-right: none; background: transparent; padding: 0 !important; margin: 0 !important; }
.nav :deep(.el-menu-item) { color: var(--el-text-color-regular); height: 44px; line-height: 44px; margin: 0 !important; padding: 0 18px !important; }
.nav :deep(.el-menu-item:hover) { background: var(--el-fill-color); color: var(--jz-text); }
.nav :deep(.el-menu-item.is-active) { background: var(--el-color-primary-light-9); color: var(--el-color-primary); }

/* 版本号：居中，推到侧栏底部；dev 模式显示醒目标记 */
.version {
  margin-top: auto;
  padding: 8px 0 12px;
  text-align: center;
}
.dev-badge {
  display: inline-block;
  padding: 2px 10px;
  margin-bottom: 6px;
  font-size: 10px;
  font-weight: 700;
  color: #fbbf24;
  background: rgba(251, 191, 36, 0.12);
  border: 1px solid rgba(251, 191, 36, 0.4);
  border-radius: 10px;
  letter-spacing: 1px;
}
.ver-text {
  font-size: 11px;
  color: var(--jz-text-dim);
  font-family: monospace;
}
</style>

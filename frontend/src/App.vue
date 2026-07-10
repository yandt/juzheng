<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { System, Window } from '@wailsio/runtime'
import AppSidebar from './components/AppSidebar.vue'
import HomePage from './views/HomePage.vue'
import SubscriptionsPage from './views/SubscriptionsPage.vue'
import TrafficMonitor from './views/TrafficMonitor.vue'
import ConnectionsPage from './views/ConnectionsPage.vue'
import GroupList from './views/GroupList.vue'
import RuleList from './views/RuleList.vue'
import SettingsForm from './views/SettingsForm.vue'
import JsonSource from './views/JsonSource.vue'
import type { Page, MenuItem } from './types/app'
import { t } from './i18n'

const activePage = ref<Page>('home')

// 平台判断：拖拽区位置和窗口按钮避让因平台而异
const isMac = ref(System.IsMac())

// 导航项用 computed + t()，切换语言时标签自动更新。
const pages = computed<MenuItem[]>(() => [
  { key: 'home', label: t('nav.home'), icon: 'HomeFilled' },
  { key: 'subscriptions', label: t('nav.subscriptions'), icon: 'Collection' },
  { key: 'traffic', label: t('nav.traffic'), icon: 'DataLine' },
  { key: 'connections', label: t('nav.connections'), icon: 'Connection' },
  { key: 'groups', label: t('nav.groups'), icon: 'Share' },
  { key: 'rules', label: t('nav.rules'), icon: 'Filter' },
  { key: 'settings', label: t('nav.settings'), icon: 'Setting' },
  { key: 'source', label: t('nav.source'), icon: 'Document' },
])

// 根据平台注入 CSS 变量：控制窗口三按钮避让区宽度。
// macOS：三按钮在左上 → 左侧 padding 加大
// Windows/Linux：三按钮在右上 → 右侧 padding 加大
// 这些变量被 PageShell 的 header padding 引用，确保标题/操作区不被三按钮遮挡。
// 同步设置（不依赖 onMounted），避免 runtime 初始化时序问题导致平台误判。
function applyPlatformVars() {
  const root = document.documentElement
  const mac = System.IsMac()
  // 多重判断：Wails IsMac + userAgent 兜底（防 runtime 未就绪）
  const ua = (typeof navigator !== 'undefined' && navigator.userAgent) || ''
  const isMacFinal = mac || /Mac|iPhone|iPad/.test(ua)
  if (isMacFinal) {
    root.style.setProperty('--titlebar-btn-left', '80px')
    root.style.setProperty('--titlebar-btn-right', '0px')
  } else {
    root.style.setProperty('--titlebar-btn-left', '0px')
    root.style.setProperty('--titlebar-btn-right', '142px')
  }
  root.style.setProperty('--titlebar-h', '56px')
}
// 立即执行（setup 阶段，确保首屏渲染就用对平台变量）
applyPlatformVars()
onMounted(applyPlatformVars)

// Windows 无边框窗口的自绘控制按钮：最小化 → 任务栏；关闭 → 隐藏到托盘（代理/内核继续运行，
// 从托盘「显示窗口」唤起，「退出 Juzheng」才真正退出）。最大化不适用（窗口固定尺寸）。
function winMinimise() { Window.Minimise() }
function winClose() { Window.Hide() }
</script>

<template>
  <el-container class="app-layout">
    <AppSidebar
      :pages="pages"
      :active-page="activePage"
      @select="(p: string) => activePage = p as Page"
    />
    <el-main class="main-area">
      <!-- 沉浸式标题栏拖拽区：macOS 在左侧（避开左上三按钮），Windows 在右侧（避开右上三按钮） -->
      <div class="titlebar-drag" :class="{ 'drag-left': isMac, 'drag-right': !isMac }"></div>

      <!-- Windows 无边框窗口的自绘控制按钮（右上角，替代被隐藏的系统标题栏三按钮）。
           最小化 / 最大化(固定尺寸，禁用) / 关闭。macOS 用系统红黄绿按钮，不显示这组。 -->
      <div v-if="!isMac" class="win-controls">
        <button class="win-btn" title="最小化" @click="winMinimise">
          <svg width="10" height="10" viewBox="0 0 10 10"><rect x="0" y="4.5" width="10" height="1" fill="currentColor"/></svg>
        </button>
        <button class="win-btn win-disabled" title="窗口尺寸固定，无法最大化" disabled>
          <svg width="10" height="10" viewBox="0 0 10 10"><rect x="0.5" y="0.5" width="9" height="9" fill="none" stroke="currentColor"/></svg>
        </button>
        <button class="win-btn win-close" title="关闭（隐藏到托盘，从托盘退出）" @click="winClose">
          <svg width="10" height="10" viewBox="0 0 10 10"><path d="M0.5 0.5 L9.5 9.5 M9.5 0.5 L0.5 9.5" stroke="currentColor" stroke-width="1.1"/></svg>
        </button>
      </div>
      <HomePage v-show="activePage === 'home'" />
      <SubscriptionsPage v-show="activePage === 'subscriptions'" />
      <TrafficMonitor v-show="activePage === 'traffic'" :active="activePage === 'traffic'" />
      <ConnectionsPage v-if="activePage === 'connections'" />
      <GroupList v-show="activePage === 'groups'" />
      <RuleList v-if="activePage === 'rules'" />
      <SettingsForm v-show="activePage === 'settings'" />
      <JsonSource v-show="activePage === 'source'" />
    </el-main>
  </el-container>
</template>

<style>
/* 全局：撑满窗口，清除默认间距 */
html, body, #app { height: 100%; margin: 0; padding: 0; }
body { overflow: hidden; }
* { box-sizing: border-box; }

.app-layout { height: 100vh; width: 100vw; }

/* main-area 撑满剩余空间，无内边距，内部各 view 自管 padding */
.main-area {
  padding: 0 !important;
  background: var(--jz-bg);
  overflow: hidden;
  flex: 1;
  display: flex;
  flex-direction: column;
  position: relative;
}

/* 沉浸式标题栏拖拽区：避开窗口三按钮（macOS 左上 / Windows 右上），
   双击可最大化。仅覆盖按钮所在角，不遮挡 view 顶部工具栏交互。 */
.titlebar-drag {
  position: absolute;
  top: 0;
  height: var(--titlebar-h, 40px);
  -webkit-app-region: drag;
  z-index: 20;
}
/* macOS：三按钮在左上，拖拽区覆盖左侧 */
.drag-left {
  left: 0;
  width: 76px;
}
/* Windows/Linux：三按钮在右上，拖拽区覆盖右侧 */
.drag-right {
  right: 0;
  width: 140px;
}
/* 所有交互元素默认 no-drag，确保按钮可点 */
.main-area button, .main-area input, .main-area select, .main-area .el-select, .main-area a {
  -webkit-app-region: no-drag;
}

/* Windows 无边框窗口的自绘控制按钮：右上角，浮在拖拽区之上（z-index 高于 titlebar-drag）。 */
.win-controls {
  position: absolute;
  top: 0;
  right: 0;
  display: flex;
  height: var(--titlebar-h, 40px);
  z-index: 30;
  -webkit-app-region: no-drag;
}
.win-btn {
  width: 46px;
  height: 100%;
  border: 0;
  background: transparent;
  color: var(--jz-text-dim, #b8b8b8);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: background 0.12s, color 0.12s;
}
.win-btn:hover { background: rgba(255, 255, 255, 0.08); color: var(--jz-text, #fff); }
.win-btn.win-close:hover { background: #e81123; color: #fff; }
.win-btn.win-disabled { color: rgba(255, 255, 255, 0.18); cursor: default; }
.win-btn.win-disabled:hover { background: transparent; color: rgba(255, 255, 255, 0.18); }

/* ===== 容器溢出控制（核心原则：任何容器不能超过页面，溢出在内部处理） ===== */

/* el-dialog：限制最大高度，body 区滚动。
   原默认无高度限制，内容多了超出视口。 */
.el-overlay-dialog .el-dialog {
  display: flex;
  flex-direction: column;
  margin-top: 8vh !important;          /* 顶部留间距 */
  max-height: 84vh;                    /* 不超过视口高度 */
  margin-bottom: 8vh;
}
.el-overlay-dialog .el-dialog__header {
  flex: 0 0 auto;                      /* 标题不缩 */
}
.el-overlay-dialog .el-dialog__body {
  flex: 1;                             /* body 占满中间 */
  overflow-y: auto;                    /* 内容溢出时滚动 */
  min-height: 0;                       /* flex 子项允许收缩 */
}
.el-overlay-dialog .el-dialog__footer {
  flex: 0 0 auto;                      /* 底部按钮不缩 */
}

/* el-tabs：内容区限制高度，超出滚动。
   tab 嵌在 PageShell page-body 里时，默认会撑开导致页面溢出。 */
.el-tabs__content {
  overflow: visible;                   /* tab 自身不滚动，交给外层 PageShell page-body */
}

/* ===== 节点类型标签（全局统一样式，所有显示节点类型处共用） ===== */
/* 分类：infra 基础设施（direct/block/dns/http）/ proxy 代理协议（vmess/vless/...） */
.node-type {
  display: inline-flex;
  align-items: center;
  padding: 0 8px;
  height: 20px;
  border-radius: 4px;
  font-size: 11px;
  font-family: monospace;
  font-weight: 500;
  line-height: 1;
  white-space: nowrap;
  flex-shrink: 0;
}
.node-type.infra {
  color: var(--jz-text-dim);
  background: var(--jz-border);
  border: 1px solid var(--jz-border-hover);
}
.node-type.proxy {
  color: #4ade80;
  background: rgba(74, 222, 128, 0.1);
  border: 1px solid rgba(74, 222, 128, 0.3);
}
</style>

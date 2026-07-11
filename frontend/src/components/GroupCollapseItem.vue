<script setup lang="ts">
// GroupCollapseItem：代理组折叠面板（标题 + 操作工具栏 + 成员节点网格）。
// 单个代理组的完整展开/折叠视图，含测速/编辑/规则操作 + 节点切换。
import { Timer, Edit } from '@element-plus/icons-vue'
import { t } from '../i18n'
import { nodeTypeCategory } from '../types/app'
import type { SelectorGroup } from '../configModel'

const props = defineProps<{
  group: SelectorGroup
  /** 是否正在测速（全局，loading 状态） */
  testingNode: string
  /** sing-box 是否运行中（测速需要） */
  sboxRunning: boolean
  /** 过滤文本（v-model，每组件独立） */
  filterText: string
  /** 节点延迟 map */
  delays: Record<string, number>
  /** 正在测速的节点 map（tag → true），测速中在延迟位置显示等待 */
  testing: Record<string, boolean>
  /** 节点类型查询函数（tag → type 字符串） */
  memberNodeType: (tag: string) => string
}>()

const emit = defineEmits<{
  (e: 'update:filterText', v: string): void
  (e: 'edit-group'): void
  (e: 'test-all', members: string[]): void
  (e: 'test-node', tag: string): void
  (e: 'node-change', tag: string): void
}>()

// 过滤成员节点
function filteredMembers(): string[] {
  const kw = (props.filterText || '').trim().toLowerCase()
  if (!kw) return props.group.outbounds
  return props.group.outbounds.filter(t => t.toLowerCase().includes(kw))
}

// 当前选中节点
function currentNode(): string {
  return props.group.default || props.group.outbounds[0] || ''
}

// 类型圆点颜色
const groupTypeDot: Record<string, string> = { selector: 'var(--el-color-primary)', urltest: '#4ade80' }
// 类型标签（函数形式，保证切换语言时响应式更新）
function groupTypeLabel(type: string): string {
  const m: Record<string, string> = { selector: t('comp.groupSelector'), urltest: t('comp.groupUrltest') }
  return m[type] || type
}

// 延迟颜色
function delayClass(d: number | undefined): string {
  if (d === undefined) return ''
  if (d <= 0) return 'bad'
  if (d < 500) return 'good'
  if (d < 1500) return 'ok'
  return 'slow'
}
</script>

<template>
  <el-collapse-item :name="String(group.tag)">
    <!-- 折叠标题：类型圆点 + 组名 + 当前节点 + 节点数 + 规则数 -->
    <template #title>
      <div class="collapse-title">
        <el-tooltip :content="groupTypeLabel(group.type)" placement="top">
          <span class="group-type-dot" :style="{ background: groupTypeDot[group.type] || 'var(--jz-text-dim)' }"></span>
        </el-tooltip>
        <span class="collapse-name">{{ group.tag }}</span>
        <el-tag v-if="group.type === 'selector'" size="small" type="success" effect="plain">
          {{ currentNode() || '-' }}
        </el-tag>
        <span class="collapse-count">{{ t('comp.nodesCount', { n: group.outbounds.length }) }}</span>
      </div>
    </template>
    <!-- 展开内容：操作栏 + 成员节点网格 -->
    <div class="collapse-body">
      <div class="group-toolbar">
        <div class="toolbar-left">
          <el-tooltip :content="sboxRunning ? t('comp.testAll') : t('comp.needStartKernel')" placement="top">
            <el-button text size="small" :loading="testingNode !== ''" :disabled="!sboxRunning" @click="emit('test-all', filteredMembers())">
              <el-icon><Timer /></el-icon>
            </el-button>
          </el-tooltip>
          <el-tooltip :content="t('comp.editGroup')" placement="top">
            <el-button text size="small" @click="emit('edit-group')"><el-icon><Edit /></el-icon></el-button>
          </el-tooltip>
        </div>
        <FilterInput
          :model-value="filterText"
          @update:model-value="$emit('update:filterText', $event)"
          :placeholder="t('comp.filterNodesPlaceholder')"
          class="filter-input"
        />
      </div>
      <div class="member-grid">
        <div
          v-for="memberTag in filteredMembers()"
          :key="memberTag"
          class="member-card"
          :class="{ active: memberTag === currentNode(), selectable: group.type === 'selector' }"
          @click="group.type === 'selector' ? emit('node-change', memberTag) : null"
        >
          <div class="member-top">
            <el-icon v-if="memberTag === currentNode()" class="member-active-icon"><CircleCheckFilled /></el-icon>
            <span class="member-tag">{{ memberTag }}</span>
          </div>
          <div class="member-bottom">
            <span class="node-type" :class="nodeTypeCategory(memberNodeType(memberTag))">{{ memberNodeType(memberTag) }}</span>
            <span v-if="testing[memberTag]" class="member-delay testing">
              <el-icon class="is-loading"><Loading /></el-icon>{{ t('comp.testing') }}
            </span>
            <span v-else-if="delays[memberTag] !== undefined" class="member-delay" :class="delayClass(delays[memberTag])">
              {{ delays[memberTag] > 0 ? `${delays[memberTag]}ms` : t('comp.timeout') }}
            </span>
          </div>
        </div>
        <el-empty v-if="filteredMembers().length === 0" :description="t('comp.noMatchNodes')" :image-size="40" />
      </div>
    </div>
  </el-collapse-item>
</template>

<style scoped>
/* el-collapse-item：清除默认过大 padding，收紧边框 */
:deep(.el-collapse-item__header) { padding: 0 16px; height: 48px; line-height: 48px; background: transparent; }
:deep(.el-collapse-item__content) { padding: 0 16px 12px; }

.collapse-title { display: flex; align-items: center; gap: 8px; flex: 1; }
.group-type-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; display: inline-block; box-shadow: 0 0 0 2px rgba(255,255,255,.06); }
.collapse-name { font-weight: 600; color: var(--jz-text); font-size: 13px; flex: 1; }
.collapse-count { font-size: 11px; color: var(--jz-text-dim); }
.collapse-rules { margin-left: 4px; }
.collapse-body { padding: 4px 0; }
/* 操作工具栏 */
.group-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 10px; padding-bottom: 8px; border-bottom: 1px solid var(--jz-border); }
.toolbar-left { display: flex; align-items: center; gap: 2px; }
.toolbar-left .el-button { width: 24px; height: 24px; padding: 0; border: none !important; background: transparent !important; }
.toolbar-left .el-button:hover { background: var(--jz-border) !important; }
.toolbar-left .el-button .el-icon { font-size: 14px; }
.group-toolbar .filter-input { width: 200px; flex-shrink: 0; }
/* 成员节点网格 */
.member-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 8px; }
.member-card { display: flex; flex-direction: column; gap: 6px; padding: 10px; border: 1px solid var(--jz-border); border-radius: 6px; background: var(--jz-surface); transition: all .15s; cursor: default; }
.member-card.selectable { cursor: pointer; }
.member-card.selectable:hover { border-color: var(--el-color-primary); background: var(--el-fill-color); }
.member-card.active { border-color: #4ade80; background: rgba(74,222,128,.08); }
.member-card.active.selectable:hover { border-color: #4ade80; }
.member-top { display: flex; align-items: center; gap: 4px; }
.member-active-icon { color: #4ade80; font-size: 14px; flex-shrink: 0; }
.member-tag { font-size: 12px; color: var(--el-text-color-regular); font-family: monospace; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.member-bottom { display: flex; align-items: center; justify-content: space-between; gap: 4px; }
.member-delay { font-size: 11px; font-family: monospace; }
.member-delay.testing { display: inline-flex; align-items: center; gap: 3px; color: var(--jz-text-dim); }
.member-delay.good { color: #4ade80; }
.member-delay.ok { color: #fbbf24; }
.member-delay.slow { color: #fb923c; }
.member-delay.bad { color: #f87171; }
</style>

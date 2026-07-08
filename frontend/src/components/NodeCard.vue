<script setup lang="ts">
// NodeCard：节点卡片（类型徽标 + 服务器信息 + 归属组数 + 操作按钮 + 拖拽）。
// 拖拽通过 emit 上抛给父组件处理（列表级状态，不下沉到卡片）。
import { Edit, Delete } from '@element-plus/icons-vue'
import { t } from '../i18n'
import { nodeTypeCategory } from '../types/app'
import type { SingBoxNode } from '../configModel'

const props = defineProps<{
  node: SingBoxNode
  /** 拖拽中（当前卡片是被拖拽的） */
  dragging?: boolean
  /** drop 目标高亮 */
  dragOver?: boolean
  /** 归属的代理组数 */
  groupCount: number
  /** 是否已归属代理组（禁止删除） */
  inUse: boolean
}>()

const emit = defineEmits<{
  (e: 'edit'): void
  (e: 'remove'): void
  (e: 'dragstart'): void
  (e: 'dragover', ev: DragEvent): void
  (e: 'drop'): void
  (e: 'dragend'): void
}>()
</script>

<template>
  <el-card
    class="node-card"
    :class="{ dragging, 'drag-over': dragOver }"
    shadow="hover"
    draggable="true"
    @dragstart="emit('dragstart')"
    @dragover="emit('dragover', $event)"
    @drop="emit('drop')"
    @dragend="emit('dragend')"
  >
    <div class="card-head">
      <SvgIcon name="drag" :size="16" class="drag-handle" :title="t('comp.dragSort')" />
      <span class="card-title">{{ node.tag }}</span>
      <span class="node-type" :class="nodeTypeCategory(node.type)">{{ node.type }}</span>
    </div>
    <div class="card-sub" :class="{ 'empty-sub': !node.server }">
      <template v-if="node.server">{{ node.server }}<span v-if="node.server_port">:{{ node.server_port }}</span></template>
      <template v-else>—</template>
    </div>
    <div class="node-meta">
      <span v-if="groupCount > 0">{{ t('comp.belongsGroups', { n: groupCount }) }}</span>
      <span v-else class="placeholder">{{ t('comp.notInGroup') }}</span>
    </div>
    <div class="card-actions node-actions">
      <el-tooltip :content="t('comp.editNode')" placement="top">
        <el-button text size="small" @click="emit('edit')"><el-icon><Edit /></el-icon></el-button>
      </el-tooltip>
      <el-tooltip :content="inUse ? t('comp.inUseCannotDelete') : t('comp.deleteNode')" placement="top">
        <el-button text size="small" type="danger" :disabled="inUse" @click="emit('remove')"><el-icon><Delete /></el-icon></el-button>
      </el-tooltip>
    </div>
  </el-card>
</template>

<style scoped>
/* 节点卡片：深色背景 + 圆角 + hover 阴影 */
.node-card { display: flex; flex-direction: column; background: var(--jz-surface) !important; border: 1px solid var(--jz-border) !important; border-radius: 8px !important; transition: border-color .2s, box-shadow .2s; }
.node-card:hover { border-color: var(--jz-border-hover) !important; box-shadow: 0 2px 8px rgba(0,0,0,.15); }
.node-card :deep(.el-card__body) { flex: 1; display: flex; flex-direction: column; padding: 10px !important; gap: 6px; }
.node-card.dragging { opacity: 0.4; }
.node-card.drag-over { border-color: var(--el-color-primary) !important; box-shadow: 0 0 0 2px rgba(96,165,250,0.4) !important; }
.card-head { display: flex; align-items: center; gap: 6px; }
.card-title { font-weight: 600; color: var(--jz-text); font-family: monospace; font-size: 13px; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.card-sub { color: var(--jz-text-dim); font-size: 11px; min-height: 14px; font-family: monospace; }
.card-sub.empty-sub { color: var(--jz-text-dim); }
.node-meta { font-size: 10px; color: var(--jz-text-dim); flex: 1; }
.node-meta .placeholder { color: var(--jz-text-dim); }
.node-actions { display: flex; align-items: center; gap: 2px; justify-content: flex-end; border-top: 1px solid var(--jz-border); padding-top: 6px; margin-top: 2px; }
.drag-handle { cursor: grab; color: var(--jz-text-dim); font-size: 14px; flex-shrink: 0; transition: color .2s; }
.drag-handle:hover { color: var(--jz-text-dim); }
.drag-handle:active { cursor: grabbing; }
.node-actions .el-button { width: 24px; height: 24px; padding: 0; border: none !important; background: transparent !important; }
.node-actions .el-button:hover { background: var(--jz-border) !important; }
.node-actions .el-button .el-icon { font-size: 14px; }
.rules-badge { margin-left: 2px; }
</style>

<script setup lang="ts">
/**
 * 标准正方形图标按钮：强制"图标 + tooltip"模式，用于 PageShell 的 actions 区。
 *
 * - icon：Element Plus 图标组件（如 Refresh、Check）
 * - title：tooltip 提示文字（必填，无障碍 + 说明按钮用途）
 * - type：按钮类型（primary/success/danger 等）
 * - disabled / loading：禁用 / 加载态
 *
 * 用法：<IconButton :icon="Refresh" title="刷新" @click="onRefresh" />
 */
import { computed, type Component } from 'vue'

const props = withDefaults(defineProps<{
  icon?: Component        // Element Plus 图标组件（与 svg 二选一）
  svg?: string            // SvgIcon 自定义字形名（与 icon 二选一，如 play/stop）
  color?: string          // 图标颜色覆盖（配合 svg 用，如 var(--el-color-success)）
  title: string
  type?: '' | 'primary' | 'success' | 'warning' | 'danger' | 'info'
  disabled?: boolean
  loading?: boolean
}>(), {
  type: '',
  disabled: false,
  loading: false,
})

const emit = defineEmits<{ (e: 'click', ev: MouseEvent): void }>()

// el-button 的 type 在空字符串时不传，避免控制台警告
const btnType = computed(() => props.type || undefined)
</script>

<template>
  <el-tooltip :content="title" placement="bottom" :show-after="300">
    <!-- 用 span 包一层，让 tooltip 在 disabled 按钮上也能触发 -->
    <span class="icon-btn-wrap">
      <el-button
        :type="btnType"
        :disabled="disabled"
        :loading="loading"
        class="icon-btn"
        @click="(ev: MouseEvent) => emit('click', ev)"
      >
        <SvgIcon v-if="svg" :name="svg" :size="18" :color="color" class="icon-btn-icon" />
        <el-icon v-else class="icon-btn-icon"><component :is="icon" /></el-icon>
      </el-button>
    </span>
  </el-tooltip>
</template>

<style scoped>
.icon-btn-wrap {
  display: inline-flex; /* disabled 时仍可触发 tooltip */
}
/* 正方形按钮：36×36，固定尺寸，图标居中。
   默认无边框透明背景（融入 header），仅 hover 时显示边框 + 背景反馈。 */
.icon-btn {
  width: 36px;
  height: 36px;
  padding: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid transparent;
  background: transparent;
}
/* 背景透明时，primary/danger 类型按钮的图标改用主题色（默认白色文字配透明底会看不见，浅色模式尤甚） */
.icon-btn.el-button--primary {
  --el-button-text-color: var(--el-color-primary);
  --el-button-hover-text-color: var(--el-color-primary);
}
.icon-btn.el-button--danger {
  --el-button-text-color: var(--el-color-danger);
  --el-button-hover-text-color: var(--el-color-danger);
}
/* hover：显示边框 + 轻微背景，提供明确交互反馈 */
.icon-btn:hover:not(.is-disabled) {
  border-color: var(--jz-border-hover);
  background: var(--el-fill-color);
}
/* 非主操作按钮 hover 文字色统一 */
.icon-btn:hover:not(.is-disabled):not(.el-button--primary):not(.el-button--danger) {
  color: var(--jz-text);
}
/* 图标尺寸：足够大，清晰可辨 */
.icon-btn-icon {
  font-size: 18px;
}
</style>

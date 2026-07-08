<script setup lang="ts">
// FilterInput：带搜索图标的过滤输入框（v-model 双向绑定）。
// 统一 TrafficMonitor / GroupList 的过滤输入样式，消除重复。
import { Search } from '@element-plus/icons-vue'
import { t } from '../i18n'

defineProps<{
  modelValue: string
  placeholder?: string
}>()
defineEmits<{ (e: 'update:modelValue', v: string): void }>()
</script>

<template>
  <el-input
    :model-value="modelValue"
    @update:model-value="$emit('update:modelValue', $event)"
    size="small"
    clearable
    :placeholder="placeholder ?? t('comp.filterPlaceholder')"
  >
    <template #prefix><el-icon><Search /></el-icon></template>
  </el-input>
</template>

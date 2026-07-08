<script setup lang="ts">
// SaveButton：标准保存按钮（IconButton 封装），统一 dirty 状态 + 保存语义。
// 4 个页面（GroupList/SettingsForm/JsonSource/HomePage 弹窗）共用，消除重复。
import { Check } from '@element-plus/icons-vue'
import { t } from '../i18n'

withDefaults(defineProps<{
  /** 是否有未保存改动（控制 disabled + tooltip 文案） */
  dirty?: boolean
  /** 额外 disabled 原因（如 sboxRunning 时禁用保存） */
  disabled?: boolean
}>(), {
  dirty: false,
  disabled: false,
})
defineEmits<{ (e: 'save'): void }>()
</script>

<template>
  <IconButton
    :icon="Check"
    type="primary"
    :title="dirty ? t('comp.saveConfig') : t('comp.saved')"
    :disabled="disabled || !dirty"
    @click="$emit('save')"
  />
</template>

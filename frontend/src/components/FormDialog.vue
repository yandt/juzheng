<script setup lang="ts">
// FormDialog：通用编辑弹窗骨架（el-dialog + footer 取消/保存）。
// 统一所有编辑弹窗（节点/代理组/规则/订阅导入等）的结构。
//
// 用法：
//   <FormDialog v-model="visible" title="编辑节点" width="520px" :save-text="..." @save="onSave">
//     <表单内容放这里（slot）/>
//   </FormDialog>
import { computed } from 'vue'
import { t } from '../i18n'

const props = withDefaults(defineProps<{
  /** 双向绑定可见性（v-model） */
  modelValue: boolean
  /** 弹窗标题 */
  title: string
  /** 弹窗宽度，默认 520px */
  width?: string
  /** 保存按钮文字，默认"保存" */
  saveText?: string
  /** 保存按钮禁用 */
  saveDisabled?: boolean
}>(), {
  width: '520px',
  saveDisabled: false,
})

const emit = defineEmits<{
  (e: 'update:modelValue', v: boolean): void
  (e: 'save'): void
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})

function onCancel() {
  visible.value = false
}
</script>

<template>
  <el-dialog v-model="visible" :title="title" :width="width">
    <slot />
    <template #footer>
      <el-button @click="onCancel">{{ t('comp.cancel') }}</el-button>
      <el-button type="primary" :disabled="saveDisabled" @click="$emit('save')">{{ saveText ?? t('comp.save') }}</el-button>
    </template>
  </el-dialog>
</template>

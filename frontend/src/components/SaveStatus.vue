<script setup lang="ts">
// SaveStatus：即改即存的状态指示（替代手动保存按钮）。
// 编辑后自动保存，这里只展示状态：保存中 / 待保存 / 已保存。
import { Loading, Select } from '@element-plus/icons-vue'
import { t } from '../i18n'

defineProps<{
  saving?: boolean   // 自动保存进行中
  dirty?: boolean    // 有改动、等待防抖保存
}>()
</script>

<template>
  <span class="save-status">
    <template v-if="saving">
      <el-icon class="spin"><Loading /></el-icon>&nbsp;{{ t('comp.saving') }}
    </template>
    <template v-else-if="dirty">{{ t('comp.pendingSave') }}</template>
    <template v-else>
      <el-icon class="ok"><Select /></el-icon>&nbsp;{{ t('comp.saved') }}
    </template>
  </span>
</template>

<style scoped>
.save-status { display: inline-flex; align-items: center; font-size: 12px; color: var(--jz-text-dim); }
.save-status .ok { color: #4ade80; }
.spin { animation: save-spin 0.8s linear infinite; }
@keyframes save-spin { to { transform: rotate(360deg); } }
</style>

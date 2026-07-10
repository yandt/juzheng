<script setup lang="ts">
// 明文抓包「抓包域名」配置弹窗（config.mitmRules）。
// 自包含（状态取自单例 useSbox），首页网络设置与流量监控页共用同一份。
// 即改即存：编辑后 markDirty 触发防抖保存 + 内核热重载（运行中即时生效）。
import { computed } from 'vue'
import { useSbox } from '../composables/useSbox'
import { t } from '../i18n'

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void }>()
const visible = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit('update:modelValue', v),
})

const { config, markDirty } = useSbox()
</script>

<template>
  <el-dialog v-model="visible" :title="t('home.mitmDialogTitle')" width="560px">
    <el-alert type="info" :closable="false" show-icon style="margin-bottom: 12px">{{ t('home.mitmAlert') }}</el-alert>
    <RuleEditor v-model="config.mitmRules" tag="to-mitmproxy" @update:model-value="markDirty()" />
    <div class="cap-hint">{{ t('home.mitmMatchHint') }}</div>
    <template #footer>
      <el-button type="primary" @click="visible = false">{{ t('home.done') }}</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.cap-hint { margin-top: 8px; color: var(--jz-text-dim); font-size: 11px; }
</style>

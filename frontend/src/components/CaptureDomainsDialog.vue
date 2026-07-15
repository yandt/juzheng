<script setup lang="ts">
// 明文抓包「抓包域名」配置弹窗（config.mitmRules）。
// 自包含（状态取自单例 useSbox），首页网络设置与流量监控页共用同一份。
//
// 提交时机：关窗时统一提交一次，而非每次输入。
// RuleEditor 为维护 textarea 文本镜像会「逐字符」emit update:modelValue；若直接接 markDirty，
// 每敲一个字符都会全量序列化配置 + 写盘（singbox.json + 订阅文件）+ 热重载内核 —— 只要打字
// 停顿超过自动保存防抖（600ms，输域名时很常见），就会一个字符重载一次内核，造成网络抖动与提示刷屏。
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

// 本次开窗期间是否编辑过：没改动就关窗时不触发保存/重载。
// v-model 已把编辑直接写进 config.mitmRules，这里只负责决定「何时提交」。
let touched = false
function onEdit() { touched = true }
function onClosed() {
  if (!touched) return
  touched = false
  markDirty()   // 一次性提交：序列化 + 防抖写盘 + 内核热重载
}
</script>

<template>
  <el-dialog v-model="visible" :title="t('home.mitmDialogTitle')" width="560px" @closed="onClosed">
    <el-alert type="info" :closable="false" show-icon style="margin-bottom: 12px">{{ t('home.mitmAlert') }}</el-alert>
    <RuleEditor v-model="config.mitmRules" tag="to-mitmproxy" @update:model-value="onEdit" />
    <div class="cap-hint">{{ t('home.mitmMatchHint') }}</div>
    <template #footer>
      <el-button type="primary" @click="visible = false">{{ t('home.done') }}</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.cap-hint { margin-top: 8px; color: var(--jz-text-dim); font-size: 11px; }
</style>

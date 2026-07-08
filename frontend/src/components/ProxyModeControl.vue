<script setup lang="ts">
// 代理模式控件：Rule(规则)/Global(全局代理)/Direct(直连)。
// 自包含（状态取自单例 composable），首页卡片与设置页共用同一份。
import { computed } from 'vue'
import { useSbox } from '../composables/useSbox'
import { useClashApi } from '../composables/useClashApi'
import { t } from '../i18n'

const { sboxRunning, config, markDirty } = useSbox()
const { clashMode, setMode } = useClashApi()

// 以配置 settings.clashMode 为准（持久化=启动默认模式）；运行中额外走 Clash API 实时切换。
const proxyMode = computed<string>({
  get: () => (sboxRunning.value ? clashMode.value : (config.value.settings?.clashMode || 'Rule')),
  set: (mode: string) => {
    if (config.value.settings) config.value.settings.clashMode = mode
    markDirty(false)   // 代理模式走 clash_api 实时切换，无需内核重载
    if (sboxRunning.value) setMode(mode)   // 实时切换（PATCH /configs）
  },
})
const modeDesc = computed(() => t('home.modeDesc_' + proxyMode.value))
</script>

<template>
  <div class="proxy-mode-control">
    <el-radio-group v-model="proxyMode" size="small" class="mode-group">
      <el-radio-button value="Rule">{{ t('home.modeRule') }}</el-radio-button>
      <el-radio-button value="Global">{{ t('home.modeGlobal') }}</el-radio-button>
      <el-radio-button value="Direct">{{ t('home.modeDirect') }}</el-radio-button>
    </el-radio-group>
    <div class="mode-desc">{{ modeDesc }}</div>
    <div v-if="!sboxRunning" class="mode-hint">{{ t('home.modeOfflineHint') }}</div>
  </div>
</template>

<style scoped>
.mode-group { display: flex; width: 100%; }
.mode-group :deep(.el-radio-button) { flex: 1; }
.mode-group :deep(.el-radio-button__inner) { width: 100%; }
.mode-desc { margin-top: 10px; font-size: 12px; color: var(--jz-text-dim); line-height: 1.5; }
.mode-hint { margin-top: 4px; font-size: 11px; color: var(--jz-text-dim); }
</style>

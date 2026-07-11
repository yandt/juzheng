<script setup lang="ts">
// 首页「流量统计」卡片主体：时间跨度切换 + 代理组过滤 + 双序列图 + 6 项指标。
// 采样生命周期由 HomePage 统一管理（跟随内核运行状态），本组件只消费数据。
import { computed } from 'vue'
import { useTrafficStats, SPAN_OPTIONS } from '../composables/useTrafficStats'
import { useSbox } from '../composables/useSbox'
import { fmtBytes, fmtSpeed } from '../composables/useConnections'
import { t } from '../i18n'
import TrafficChart from './TrafficChart.vue'

const { sboxRunning } = useSbox()
const { spanMinutes, selectedGroup, groupOptions, series, metrics, hasData, memSupported } = useTrafficStats()

// 组下拉：把「全部」空值项本地化
const groupSelectOptions = computed(() =>
  groupOptions.value.map(o => ({ value: o.value, label: o.value === '' ? t('stats.allGroups') : o.label })))
</script>

<template>
  <div v-if="!sboxRunning" class="stats-empty">
    <el-empty :description="t('stats.needCore')" :image-size="40" />
  </div>
  <div v-else class="stats">
    <div class="stats-toolbar">
      <el-radio-group v-model="spanMinutes" size="small">
        <el-radio-button v-for="s in SPAN_OPTIONS" :key="s" :value="s">{{ s }}{{ t('stats.min') }}</el-radio-button>
      </el-radio-group>
      <el-select v-model="selectedGroup" size="small" class="group-sel" :placeholder="t('stats.group')">
        <el-option v-for="o in groupSelectOptions" :key="o.value" :label="o.label" :value="o.value" />
      </el-select>
    </div>

    <div class="legend">
      <span class="lg"><i class="dot down"></i>{{ t('stats.download') }}</span>
      <span class="lg"><i class="dot up"></i>{{ t('stats.upload') }}</span>
    </div>

    <div class="chart-wrap">
      <TrafficChart :points="series" :height="120" />
      <div v-if="!hasData" class="collecting">{{ t('stats.collecting') }}</div>
    </div>

    <div class="metrics">
      <div class="metric"><span class="mlabel">{{ t('stats.upSpeed') }}</span><b class="up">{{ fmtSpeed(metrics.upSpeed) }}</b></div>
      <div class="metric"><span class="mlabel">{{ t('stats.downSpeed') }}</span><b class="down">{{ fmtSpeed(metrics.downSpeed) }}</b></div>
      <div class="metric"><span class="mlabel">{{ t('stats.activeConns') }}</span><b>{{ metrics.conns }}</b></div>
      <div class="metric"><span class="mlabel">{{ t('stats.upVol') }}</span><b>{{ fmtBytes(metrics.upVol) }}</b></div>
      <div class="metric"><span class="mlabel">{{ t('stats.downVol') }}</span><b>{{ fmtBytes(metrics.downVol) }}</b></div>
      <div class="metric"><span class="mlabel">{{ t('stats.memory') }}</span><b>{{ memSupported ? fmtBytes(metrics.mem) : '—' }}</b></div>
    </div>
  </div>
</template>

<style scoped>
.stats { display: flex; flex-direction: column; gap: 10px; }
.stats-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 8px; flex-wrap: wrap; }
.group-sel { width: 150px; }
.legend { display: flex; gap: 14px; }
.lg { display: flex; align-items: center; gap: 5px; font-size: 11px; color: var(--jz-text-dim); }
.dot { width: 8px; height: 8px; border-radius: 2px; display: inline-block; }
.dot.down { background: #60a5fa; }
.dot.up { background: #4ade80; }
.chart-wrap { position: relative; }
.collecting {
  position: absolute; inset: 0; display: flex; align-items: center; justify-content: center;
  font-size: 12px; color: var(--jz-text-dim);
}
.metrics {
  display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px 12px;
  border-top: 1px solid var(--jz-border, rgba(255,255,255,.06)); padding-top: 10px;
}
.metric { display: flex; flex-direction: column; gap: 2px; }
.mlabel { font-size: 11px; color: var(--jz-text-dim); }
.metric b { font-family: monospace; font-size: 14px; color: var(--el-text-color-regular); }
.metric b.up { color: #4ade80; }
.metric b.down { color: #60a5fa; }
</style>

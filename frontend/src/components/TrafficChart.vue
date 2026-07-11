<script setup lang="ts">
// 手绘 SVG 双序列面积图（上传/下载），零依赖。宽度随容器自适应（ResizeObserver），
// 高度固定；viewBox 宽度等于实际像素宽，避免非等比缩放导致描边变形。
import { ref, computed, onMounted, onUnmounted } from 'vue'
import type { SeriesPoint } from '../composables/useTrafficStats'
import { fmtSpeed, fmtBytes } from '../composables/useConnections'

const props = withDefaults(defineProps<{ points: SeriesPoint[]; height?: number }>(), { height: 170 })

const wrap = ref<HTMLElement | null>(null)
const width = ref(600)
let ro: ResizeObserver | null = null
onMounted(() => {
  if (!wrap.value) return
  width.value = wrap.value.clientWidth || 600
  ro = new ResizeObserver(entries => { for (const e of entries) width.value = Math.max(1, e.contentRect.width) })
  ro.observe(wrap.value)
})
onUnmounted(() => ro?.disconnect())

const padTop = 8
const padBottom = 4
const plotH = computed(() => Math.max(1, props.height - padTop - padBottom))
const maxVal = computed(() => Math.max(1, ...props.points.flatMap(p => [p.up, p.down])))

function px(i: number): number {
  const n = props.points.length
  return n <= 1 ? width.value / 2 : (i / (n - 1)) * width.value
}
function py(v: number): number {
  return padTop + (1 - v / maxVal.value) * plotH.value
}

function areaPath(key: 'up' | 'down'): string {
  const n = props.points.length
  if (n === 0) return ''
  const base = props.height - padBottom
  let d = `M ${px(0).toFixed(1)} ${base}`
  props.points.forEach((p, i) => { d += ` L ${px(i).toFixed(1)} ${py(p[key]).toFixed(1)}` })
  d += ` L ${px(n - 1).toFixed(1)} ${base} Z`
  return d
}
function linePath(key: 'up' | 'down'): string {
  const n = props.points.length
  if (n === 0) return ''
  return props.points.map((p, i) => `${i === 0 ? 'M' : 'L'} ${px(i).toFixed(1)} ${py(p[key]).toFixed(1)}`).join(' ')
}

const gridFractions = [0.25, 0.5, 0.75]
const peakLabel = computed(() => fmtSpeed(maxVal.value))

// ===== 鼠标悬停：定位最近时点，显示流量标签 =====
const hoverIdx = ref<number | null>(null)
function onMove(e: MouseEvent) {
  const el = wrap.value
  const n = props.points.length
  if (!el || n === 0) { hoverIdx.value = null; return }
  const rect = el.getBoundingClientRect()
  const x = e.clientX - rect.left
  const i = Math.round((x / rect.width) * (n - 1))
  hoverIdx.value = Math.max(0, Math.min(n - 1, i))
}
function onLeave() { hoverIdx.value = null }

const hoverPoint = computed(() => hoverIdx.value === null ? null : props.points[hoverIdx.value] ?? null)
const hoverX = computed(() => hoverIdx.value === null ? 0 : px(hoverIdx.value))
// 标签靠边时收进可视区（宽约 120px，左右各留 60）
const tipLeft = computed(() => Math.max(62, Math.min(width.value - 62, hoverX.value)))
function fmtTime(t: number): string {
  return new Date(t).toLocaleTimeString('zh-CN', { hour12: false })
}
</script>

<template>
  <div ref="wrap" class="chart" @mousemove="onMove" @mouseleave="onLeave">
    <svg :width="width" :height="height" :viewBox="`0 0 ${width} ${height}`" preserveAspectRatio="none">
      <line
        v-for="f in gridFractions" :key="f"
        x1="0" :x2="width" :y1="padTop + f * plotH" :y2="padTop + f * plotH"
        class="grid"
      />
      <path :d="areaPath('down')" class="area area-down" />
      <path :d="areaPath('up')" class="area area-up" />
      <path :d="linePath('down')" class="line line-down" />
      <path :d="linePath('up')" class="line line-up" />
      <!-- 悬停指示：竖线 + 两序列高亮点 -->
      <template v-if="hoverPoint">
        <line :x1="hoverX" :x2="hoverX" :y1="padTop" :y2="height - padBottom" class="cursor" />
        <circle :cx="hoverX" :cy="py(hoverPoint.down)" r="3" class="dot-down" />
        <circle :cx="hoverX" :cy="py(hoverPoint.up)" r="3" class="dot-up" />
      </template>
    </svg>
    <span class="peak">{{ peakLabel }}</span>
    <!-- 时点流量标签 -->
    <div v-if="hoverPoint" class="tip" :style="{ left: tipLeft + 'px' }">
      <div class="tip-time">{{ fmtTime(hoverPoint.t) }}</div>
      <div class="tip-row"><i class="dot down"></i><span class="tip-val">{{ fmtBytes(hoverPoint.down) }}/s</span></div>
      <div class="tip-row"><i class="dot up"></i><span class="tip-val">{{ fmtBytes(hoverPoint.up) }}/s</span></div>
    </div>
    <div v-if="points.length === 0" class="empty-hint"></div>
  </div>
</template>

<style scoped>
.chart { position: relative; width: 100%; line-height: 0; }
svg { display: block; width: 100%; }
.grid { stroke: var(--jz-border, rgba(255,255,255,.08)); stroke-width: 1; stroke-dasharray: 3 3; }
.area { stroke: none; }
.area-down { fill: rgba(96,165,250,.22); }
.area-up { fill: rgba(74,222,128,.20); }
.line { fill: none; stroke-width: 1.5; vector-effect: non-scaling-stroke; }
.line-down { stroke: #60a5fa; }
.line-up { stroke: #4ade80; }
.peak {
  position: absolute; top: 2px; right: 6px;
  font-size: 10px; font-family: monospace; color: var(--jz-text-dim);
  line-height: 1;
}
/* 悬停指示线与高亮点 */
.cursor { stroke: var(--jz-text-dim, #888); stroke-width: 1; stroke-dasharray: 2 2; vector-effect: non-scaling-stroke; }
.dot-down { fill: #60a5fa; stroke: var(--el-bg-color-overlay, #1b1d22); stroke-width: 1; }
.dot-up { fill: #4ade80; stroke: var(--el-bg-color-overlay, #1b1d22); stroke-width: 1; }
/* 时点流量标签 */
.tip {
  position: absolute; top: 4px; transform: translateX(-50%);
  pointer-events: none;
  background: var(--el-bg-color-overlay, rgba(20,22,26,.95)); border: 1px solid var(--jz-border, rgba(255,255,255,.12));
  border-radius: 6px; padding: 4px 8px; line-height: 1.4;
  box-shadow: 0 2px 8px rgba(0,0,0,.3);
  white-space: nowrap; z-index: 2;
}
.tip-time { font-size: 10px; color: var(--jz-text-dim); font-family: monospace; margin-bottom: 2px; }
.tip-row { display: flex; align-items: center; gap: 5px; }
.tip .dot { width: 7px; height: 7px; border-radius: 2px; display: inline-block; }
.tip .dot.down { background: #60a5fa; }
.tip .dot.up { background: #4ade80; }
.tip-val { font-size: 11px; font-family: monospace; color: var(--el-text-color-regular); }
</style>

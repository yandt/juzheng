<script setup lang="ts">
import { onMounted, onUnmounted, computed, watch, ref } from 'vue'
import { Setting } from '@element-plus/icons-vue'
import { useSubscriptions, formatBytes, usagePercent, formatExpire, daysUntilExpire } from '../composables/useSubscriptions'
import { useMeta, CARD_DEFS } from '../composables/useMeta'
import { useClashApi } from '../composables/useClashApi'
import { useTrafficStats } from '../composables/useTrafficStats'
import { useSbox } from '../composables/useSbox'
import { useMitm } from '../composables/useMitm'
import { t } from '../i18n'

const { subscriptions, activeSubscription, refreshSubscriptions } = useSubscriptions()
const { meta, visibleCards, loadMeta, saveMeta } = useMeta()
// 所有卡片按 order 排序（含隐藏），供管理菜单显隐 + 拖拽排序
const allCardsSorted = computed(() => [...(meta.value.cards ?? [])].sort((a, b) => a.order - b.order))
// 管理菜单内拖拽排序（HTML5 DnD）：拖动后重写各卡片 order 并持久化
const cardDragFrom = ref<number | null>(null)
function onCardDragStart(i: number) { cardDragFrom.value = i }
function onCardDrop(to: number) {
  const from = cardDragFrom.value
  cardDragFrom.value = null
  if (from === null || from === to) return
  const arr = [...allCardsSorted.value]
  const [moved] = arr.splice(from, 1)
  arr.splice(to, 0, moved)
  arr.forEach((c, idx) => { c.order = idx })   // c 为 meta.cards 中对象的引用，直接改 order
  saveMeta()
}
const { proxyGroups, delays, testing, selectNode, testDelay, startPolling, stopPolling } = useClashApi()

// 节点下拉 label：测速中显示等待文案，否则显示延迟（ms/超时）。
function nodeLabel(node: string): string {
  if (testing.value[node]) return `${node} (${t('home.testing')}…)`
  const d = delays.value[node]
  if (d === undefined) return node
  return `${node} (${d > 0 ? `${d}ms` : t('home.timeout')})`
}
const { start: startStats, stop: stopStats } = useTrafficStats()
// 网络设置/代理模式的开关与配置已抽到 NetworkControl / ProxyModeControl 组件，首页只保留链路总览、代理组、流量所需的状态。
const { sboxRunning, helperStatus, config, markDirty } = useSbox()
const { running: mitmRunning } = useMitm()

// 代理组卡片：先选代理组，再选该组默认节点（两个联动 select）。
// 数据源统一：内核运行中用 Clash API 实时数据（now/延迟）；未运行时回退到静态配置
// （config.groups），这样不必打开虚拟网卡也能查看/设置默认节点。
const selectedGroupName = ref('')
const groupView = computed(() => {
  if (sboxRunning.value && proxyGroups.value.length) {
    return proxyGroups.value.map(g => ({ name: g.name, now: g.now, all: g.all }))
  }
  return (config.value.groups ?? []).map(g => ({
    name: g.tag,
    now: g.default || g.outbounds[0] || '',
    all: g.outbounds,
  }))
})
const selectedGroup = computed(() => groupView.value.find(g => g.name === selectedGroupName.value) ?? null)
// 组列表变化时，若当前所选组已失效则默认选第一个
watch(groupView, (gs) => {
  if (!gs.find(g => g.name === selectedGroupName.value)) selectedGroupName.value = gs[0]?.name ?? ''
}, { immediate: true, deep: true })

// 切换默认节点：运行中走 Clash API 实时切换；未运行时写回配置的 default（启动后生效）。
function onGroupNodeChange(groupName: string, node: string) {
  if (sboxRunning.value) {
    selectNode(groupName, node)
  } else {
    const g = (config.value.groups ?? []).find(g => g.tag === groupName)
    if (g) { g.default = node; markDirty(false) }   // 未运行仅存盘，无需重载
  }
}

// 链路状态：TUN 服务 → 虚拟网卡 → MITM 三步，全 on 才算链路打通
const chainSteps = computed(() => [
  { name: t('home.chainTun'), on: helperStatus.value.running },
  { name: t('home.chainNic'), on: sboxRunning.value },
  { name: 'MITM', on: mitmRunning.value },
])
const chainOk = computed(() => chainSteps.value.every(s => s.on))
const chainBrokenMsg = computed(() => {
  const broken = chainSteps.value.filter(s => !s.on).map(s => s.name)
  return broken.join(t('home.listSep')) + t('home.notRunning')
})

// 卡片标题/图标映射
const cardTitle = (key: string) => { const k = CARD_DEFS.find(c => c.key === key)?.title; return k ? t(k) : key }
const cardIcon = (key: string) => CARD_DEFS.find(c => c.key === key)?.icon ?? 'Menu'

// 当前活动订阅对象
const activeSubObj = computed(() => subscriptions.value.find(s => s.name === activeSubscription.value))

onMounted(async () => {
  await Promise.all([refreshSubscriptions(), loadMeta()])
  if (sboxRunning.value) { startPolling(); startStats() }
})
onUnmounted(() => { stopPolling(); stopStats() })

// sing-box 运行状态变化时启停轮询
watch(sboxRunning, (running) => {
  if (running) { startPolling(); startStats() }
  else { stopPolling(); stopStats() }
})
</script>

<template>
  <PageShell :title="t('home.title')">
    <template #actions>
      <!-- reference 用不含 tooltip 的普通按钮：避免与 IconButton 内置 el-tooltip 形成 popper 嵌套导致点击无效 -->
      <el-popover placement="bottom-end" :width="280" trigger="click" popper-class="manage-popover">
        <template #reference>
          <el-button text class="manage-trigger" :title="t('home.manageCards')">
            <el-icon class="manage-trigger-icon"><Setting /></el-icon>
          </el-button>
        </template>
        <div class="manage-menu">
          <div class="manage-menu-title">{{ t('home.manageHomeCards') }}</div>
          <div
            v-for="(c, i) in allCardsSorted"
            :key="c.key"
            class="manage-menu-item"
            :class="{ dragging: cardDragFrom === i }"
            draggable="true"
            @dragstart="onCardDragStart(i)"
            @dragover.prevent
            @drop="onCardDrop(i)"
          >
            <SvgIcon name="drag" :size="14" class="drag-handle" :title="t('home.dragReorder')" />
            <el-checkbox v-model="c.visible" @change="saveMeta()" />
            <span class="manage-menu-name">{{ cardTitle(c.key) }}</span>
          </div>
        </div>
      </el-popover>
    </template>

    <!-- 卡片区：按 visibleCards 渲染 -->
    <div class="card-grid">
      <el-card
        v-for="card in visibleCards"
        :key="card.key"
        class="home-card"
        :class="{ span2: card.key === 'traffic' }"
        shadow="hover"
      >
        <template #header>
          <div class="card-header">
            <span class="card-title">
              <el-icon><component :is="cardIcon(card.key)" /></el-icon>
              {{ cardTitle(card.key) }}
            </span>
            <!-- 网络设置卡片：链路状态挪到标题栏，节省卡片内空间 -->
            <span
              v-if="card.key === 'quick-toggles'"
              class="chain-status header-chain"
              :class="chainOk ? 'ok' : 'broken'"
              :title="chainOk ? t('home.chainReady') : t('home.chainBrokenPrefix') + chainBrokenMsg"
            >
              <span class="chain-dot" v-for="(s, i) in chainSteps" :key="i" :class="s.on ? 'on' : 'off'"></span>
              <span class="chain-text">{{ chainOk ? t('home.chainReady') : t('home.chainBrokenPrefix') + chainBrokenMsg }}</span>
            </span>
          </div>
        </template>

        <!-- 当前订阅卡片（只展示当前启用订阅，不在此切换） -->
        <div v-if="card.key === 'current-scheme'" class="card-body">
          <el-empty v-if="!activeSubObj" :description="t('home.noActiveSub')" :image-size="40" />
          <template v-else>
            <div class="sub-name-row">
              <el-icon><Files /></el-icon>
              <span class="sub-name-text">{{ activeSubObj.name }}</span>
              <el-tag type="success" size="small" effect="dark">{{ t('home.enabled') }}</el-tag>
            </div>
            <div class="sub-info">
              <div class="info-row"><span>{{ t('home.nodeCount') }}</span><b>{{ activeSubObj.nodeCount }}</b></div>
              <div class="info-row"><span>{{ t('home.updated') }}</span><b>{{ new Date(activeSubObj.updatedAt * 1000).toLocaleString('zh-CN') }}</b></div>
            </div>
            <!-- 用量/到期（仅 URL 导入的订阅有） -->
            <template v-if="activeSubObj.info">
              <el-progress
                :percentage="usagePercent(activeSubObj.info)"
                :color="usagePercent(activeSubObj.info) > 90 ? '#f56c6c' : '#409eff'"
                :stroke-width="14"
                :text-inside="true"
                :format="() => `${formatBytes(activeSubObj?.info?.upload ?? 0)} / ${formatBytes(activeSubObj?.info?.total ?? 0)}`"
              />
              <div class="info-row">
                <span>{{ t('home.expire') }}</span>
                <b :class="{ expired: (daysUntilExpire(activeSubObj.info.expire) ?? 999) < 0 }">
                  {{ formatExpire(activeSubObj.info.expire) || t('home.none') }}
                  <template v-if="daysUntilExpire(activeSubObj.info.expire) !== null">
                    ({{ (daysUntilExpire(activeSubObj.info.expire) ?? 0) >= 0 ? t('home.daysLeft', { n: daysUntilExpire(activeSubObj.info.expire) ?? 0 }) : t('home.daysExpired', { n: -(daysUntilExpire(activeSubObj.info.expire) ?? 0) }) }})
                  </template>
                </b>
              </div>
            </template>
          </template>
        </div>

        <!-- 代理模式卡片：规则 / 代理 / 直连（复用 ProxyModeControl 组件） -->
        <div v-else-if="card.key === 'proxy-mode'" class="card-body">
          <ProxyModeControl />
        </div>

        <!-- 代理组节点卡片 -->
        <div v-else-if="card.key === 'proxy-groups'" class="card-body">
          <el-empty v-if="groupView.length === 0" :description="t('home.noProxyGroups')" :image-size="40" />
          <div v-else class="group-picker">
            <div class="picker-row">
              <span class="picker-label">{{ t('home.selectGroup') }}</span>
              <el-select v-model="selectedGroupName" size="small" class="picker-select">
                <el-option v-for="g in groupView" :key="g.name" :label="g.name" :value="g.name" />
              </el-select>
            </div>
            <div v-if="selectedGroup" class="picker-row">
              <span class="picker-label">{{ t('home.selectNode') }}</span>
              <el-select
                :model-value="selectedGroup.now"
                size="small"
                class="picker-select"
                @change="(v: string) => onGroupNodeChange(selectedGroup!.name, v)"
              >
                <el-option
                  v-for="node in selectedGroup.all"
                  :key="node"
                  :label="nodeLabel(node)"
                  :value="node"
                />
              </el-select>
              <el-button text size="small" :disabled="!sboxRunning" :loading="!!testing[selectedGroup.now]" @click="testDelay(selectedGroup.now)"><el-icon v-if="!testing[selectedGroup.now]"><Timer /></el-icon></el-button>
            </div>
            <div v-if="!sboxRunning" class="picker-hint">{{ t('home.groupsOfflineHint') }}</div>
          </div>
        </div>

        <!-- 网络设置卡片（复用 NetworkControl 组件）。链路状态在卡片标题栏（header-chain）。 -->
        <div v-else-if="card.key === 'quick-toggles'" class="card-body">
          <NetworkControl />
        </div>

        <!-- 流量统计卡片（跨 2 列，图表 + 指标） -->
        <div v-else-if="card.key === 'traffic'" class="card-body">
          <HomeTrafficStats />
        </div>
      </el-card>

      <el-empty v-if="visibleCards.length === 0" :description="t('home.noVisibleCards')" />
    </div>

  </PageShell>
</template>

<style scoped>
.card-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 14px; }
/* 流量统计卡片跨 2 列（单列布局时自动收窄为 1 列，不溢出） */
.home-card.span2 { grid-column: span 2; }
.home-card :deep(.el-card__header) { padding: 10px 14px; }
.home-card :deep(.el-card__body) { padding: 14px; }
.card-header { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.card-title { display: flex; align-items: center; gap: 6px; flex: 1; flex-shrink: 0; white-space: nowrap; font-weight: 600; color: var(--jz-text); font-size: 14px; }
.move-btn { padding: 4px; }
.card-body { display: flex; flex-direction: column; gap: 10px; }
.sub-name-row { display: flex; align-items: center; gap: 6px; color: var(--jz-text); }
.sub-name-text { font-weight: 600; flex: 1; font-size: 14px; }
.sub-info { display: flex; flex-direction: column; gap: 4px; }
.info-row { display: flex; justify-content: space-between; font-size: 12px; color: var(--jz-text-dim); }
.info-row b { color: var(--el-text-color-regular); font-family: monospace; }
.info-row b.expired { color: #f56c6c; }
.sub-actions { display: flex; gap: 4px; justify-content: flex-end; }
.group-picker { display: flex; flex-direction: column; gap: 10px; }
.picker-row { display: flex; align-items: center; gap: 8px; }
.picker-label { font-size: 12px; color: #8ab4f8; min-width: 64px; flex-shrink: 0; }
.picker-select { flex: 1; }
.picker-hint { font-size: 11px; color: var(--jz-text-dim); }
/* 链路总状态条 */
.chain-status {
  display: flex; align-items: center; gap: 6px;
  padding: 8px 10px; border-radius: 6px;
  font-size: 12px;
}
.chain-status.ok { background: rgba(74,222,128,.1); color: #4ade80; }
.chain-status.broken { background: rgba(248,113,113,.1); color: #fca5a5; }
.chain-dot {
  width: 8px; height: 8px; flex-shrink: 0; border-radius: 50%;
  background: #555;
}
.chain-dot.on { background: #4ade80; box-shadow: 0 0 4px rgba(74,222,128,.6); }
.chain-status.broken .chain-dot.on { background: #4ade80; }
.chain-status.broken .chain-dot.off { background: #f87171; }
.chain-text { margin-left: 4px; }
/* 标题栏内的紧凑链路状态：更小内边距，信息完整显示（不截断，必要时换行） */
.header-chain { padding: 2px 8px; font-size: 11px; font-weight: normal; }
.header-chain .chain-dot { width: 6px; height: 6px; }
.header-chain .chain-text { white-space: normal; }
.ctrl-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
/* 卡片管理触发按钮：正方形图标钮，风格同 IconButton（但无嵌套 tooltip） */
.manage-trigger { width: 36px; height: 36px; padding: 0; border: 1px solid transparent; }
.manage-trigger:hover { border-color: var(--jz-border-hover); background: var(--el-fill-color); }
.manage-trigger-icon { font-size: 18px; }
/* 卡片管理菜单（el-popover 面板）：显隐 + 拖拽排序 */
.manage-menu { display: flex; flex-direction: column; gap: 2px; }
.manage-menu-title { font-size: 12px; color: var(--jz-text-dim); padding: 2px 4px 6px; }
.manage-menu-item { display: flex; align-items: center; gap: 8px; padding: 6px 4px; border-radius: 6px; }
.manage-menu-item:hover { background: var(--el-fill-color-light); }
.manage-menu-item.dragging { opacity: .5; }
.manage-menu-name { flex: 1; font-size: 13px; color: var(--jz-text); }
.drag-handle { color: var(--jz-text-dim); cursor: grab; }
.drag-handle:active { cursor: grabbing; }
</style>

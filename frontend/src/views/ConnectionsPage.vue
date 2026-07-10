<script setup lang="ts">
// 连接页：每条连接平铺为一行（不展开）。支持过滤 + 自定义显示列与顺序。
import { onMounted, onUnmounted, ref, computed } from 'vue'
import { Delete, Operation } from '@element-plus/icons-vue'
import { useConnections, fmtBytes, fmtSpeed, fmtDuration, inboundKind, inboundTag } from '../composables/useConnections'
import { t } from '../i18n'

const { connections, totalDown, totalUp, connected, startPolling, stopPolling, closeAll } = useConnections()

onMounted(() => startPolling())
onUnmounted(() => stopPolling())

// 过滤：按主机/进程关键字（不区分大小写）
const filter = ref('')
const filteredConns = computed(() => {
  const q = filter.value.trim().toLowerCase()
  const list = q
    ? connections.value.filter(c => c.host.toLowerCase().includes(q) || c.proc.toLowerCase().includes(q))
    : connections.value.slice()
  // 下载量降序，和聚合视图口径一致
  return list.sort((a, b) => b.download - a.download)
})

// ===== 列显隐 + 顺序（持久化）=====
const COL_KEYS = ['host', 'proc', 'inbound', 'type', 'chains', 'rule', 'src', 'dest', 'duration', 'down', 'up'] as const
type ColKey = typeof COL_KEYS[number]
// 默认隐藏的列（避免一行过宽，用户可在列配置里打开）
const DEFAULT_HIDDEN = new Set<ColKey>(['src', 'dest'])
const LS_COLS = 'juzheng.conn.columns.flat'
interface ColCfg { key: ColKey; visible: boolean }

function loadCols(): ColCfg[] {
  let saved: ColCfg[] = []
  try { const s = JSON.parse(localStorage.getItem(LS_COLS) || 'null'); if (Array.isArray(s)) saved = s } catch { /* ignore */ }
  const known = new Set(COL_KEYS as readonly string[])
  const out = saved.filter(c => known.has(c.key))
  for (const k of COL_KEYS) if (!out.find(c => c.key === k)) out.push({ key: k, visible: !DEFAULT_HIDDEN.has(k) })
  return out
}
const columns = ref<ColCfg[]>(loadCols())
const visibleColumns = computed(() => columns.value.filter(c => c.visible))
function saveCols() { localStorage.setItem(LS_COLS, JSON.stringify(columns.value)) }
const colLabel = (k: string) => t('conn.' + k)

// 入口(inbound)展示：TUN(虚拟网卡) / MITM回注 / 系统代理；tooltip 显示真实入口 tag
type TagType = 'info' | 'primary' | 'success' | 'warning' | 'danger'
const IN_LABEL: Record<string, string> = { tun: 'conn.viaTun', mitm: 'conn.viaMitm', proxy: 'conn.viaProxy' }
const IN_TAGTYPE: Record<string, TagType> = { tun: 'primary', mitm: 'success', proxy: 'warning' }
const inLabel = (raw: string) => { const k = inboundKind(raw); return k ? t(IN_LABEL[k]) : '—' }
const inTagType = (raw: string): TagType => IN_TAGTYPE[inboundKind(raw)] || 'info'

// 列配置拖拽排序
const dragFrom = ref<number | null>(null)
function onColDrop(to: number) {
  const from = dragFrom.value
  dragFrom.value = null
  if (from === null || from === to) return
  const arr = columns.value
  const [m] = arr.splice(from, 1)
  arr.splice(to, 0, m)
  saveCols()
}
</script>

<template>
  <PageShell :title="t('conn.title')">
    <template #title-extra>
      <el-tag v-if="connected" type="success" size="small" effect="plain">↓ {{ fmtBytes(totalDown) }} · ↑ {{ fmtBytes(totalUp) }}</el-tag>
      <el-tag v-else type="info" size="small">{{ t('conn.disconnected') }}</el-tag>
    </template>
    <template #actions>
      <!-- 列配置：popover 用普通按钮做 reference（避免与 IconButton 内置 tooltip 嵌套） -->
      <el-popover placement="bottom-end" :width="220" trigger="click">
        <template #reference>
          <el-button text class="col-btn" :title="t('conn.columns')"><el-icon><Operation /></el-icon></el-button>
        </template>
        <div class="col-menu">
          <div class="col-menu-title">{{ t('conn.colConfig') }}</div>
          <div
            v-for="(c, i) in columns"
            :key="c.key"
            class="col-item"
            :class="{ dragging: dragFrom === i }"
            draggable="true"
            @dragstart="dragFrom = i"
            @dragover.prevent
            @drop="onColDrop(i)"
          >
            <SvgIcon name="drag" :size="14" class="drag-h" />
            <el-checkbox v-model="c.visible" @change="saveCols" />
            <span class="col-name">{{ colLabel(c.key) }}</span>
          </div>
        </div>
      </el-popover>
      <IconButton :icon="Delete" :title="t('conn.closeAll')" @click="closeAll" />
    </template>

    <el-empty v-if="!connected" :description="t('conn.needCore')" :image-size="60" />
    <template v-else>
      <div class="conn-toolbar">
        <FilterInput v-model="filter" :placeholder="t('conn.filterPh')" class="conn-filter" />
        <span class="count">{{ filteredConns.length }} / {{ connections.length }}</span>
      </div>

      <el-empty v-if="filteredConns.length === 0" :description="t('conn.empty')" :image-size="60" />
      <el-table v-else :data="filteredConns" row-key="id" size="small" class="conn-table">
        <!-- 动态列：按配置的可见性与顺序渲染，每条连接一行平铺 -->
        <template v-for="col in visibleColumns" :key="col.key">
          <el-table-column v-if="col.key === 'host'" :label="t('conn.host')" min-width="190" show-overflow-tooltip>
            <template #default="{ row }"><span class="host">{{ row.host }}</span></template>
          </el-table-column>
          <el-table-column v-else-if="col.key === 'proc'" :label="t('conn.proc')" width="120" show-overflow-tooltip>
            <template #default="{ row }">{{ row.proc || '—' }}</template>
          </el-table-column>
          <el-table-column v-else-if="col.key === 'inbound'" :label="t('conn.inbound')" width="110">
            <template #default="{ row }">
              <el-tag size="small" :type="inTagType(row.inbound)" effect="plain" :title="inboundTag(row.inbound) || row.inbound">{{ inLabel(row.inbound) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column v-else-if="col.key === 'type'" :label="t('conn.type')" width="72">
            <template #default="{ row }"><el-tag size="small" effect="plain">{{ row.meta.network }}</el-tag></template>
          </el-table-column>
          <el-table-column v-else-if="col.key === 'chains'" :label="t('conn.chains')" min-width="150" show-overflow-tooltip>
            <template #default="{ row }"><span class="chains">{{ row.chains.slice().reverse().join(' → ') || '—' }}</span></template>
          </el-table-column>
          <el-table-column v-else-if="col.key === 'rule'" :label="t('conn.rule')" min-width="140" show-overflow-tooltip>
            <template #default="{ row }"><span class="rule">{{ row.rule || '—' }}</span></template>
          </el-table-column>
          <el-table-column v-else-if="col.key === 'src'" :label="t('conn.src')" min-width="140" show-overflow-tooltip>
            <template #default="{ row }"><span class="mono">{{ row.src }}</span></template>
          </el-table-column>
          <el-table-column v-else-if="col.key === 'dest'" :label="t('conn.dest')" min-width="160" show-overflow-tooltip>
            <template #default="{ row }"><span class="mono">{{ row.dest }}</span></template>
          </el-table-column>
          <el-table-column v-else-if="col.key === 'duration'" :label="t('conn.duration')" width="82">
            <template #default="{ row }">{{ fmtDuration(row.start) }}</template>
          </el-table-column>
          <el-table-column v-else-if="col.key === 'down'" :label="t('conn.down')" prop="download" width="104" align="right" sortable>
            <template #default="{ row }"><b class="dl">{{ fmtBytes(row.download) }}</b><span class="sp">{{ fmtSpeed(row.downSpeed) }}</span></template>
          </el-table-column>
          <el-table-column v-else-if="col.key === 'up'" :label="t('conn.up')" prop="upload" width="104" align="right" sortable>
            <template #default="{ row }"><b class="ul">{{ fmtBytes(row.upload) }}</b><span class="sp">{{ fmtSpeed(row.upSpeed) }}</span></template>
          </el-table-column>
        </template>
      </el-table>
    </template>
  </PageShell>
</template>

<style scoped>
:deep(.page-body) { display: flex; flex-direction: column; overflow: hidden; padding: 8px 16px 16px; }
.conn-toolbar { display: flex; align-items: center; gap: 10px; margin-bottom: 8px; flex: 0 0 auto; }
.conn-filter { width: 260px; }
.count { font-size: 12px; color: var(--jz-text-dim); }
.conn-table { flex: 1; overflow: auto; }
.host { color: #8ab4f8; font-weight: 600; }
.dl { color: #4ade80; }
.ul { color: #fbbf24; }
.sp { display: block; font-size: 10px; color: var(--jz-text-dim); }
.mono { font-family: monospace; font-size: 12px; }
.chains { font-size: 12px; color: var(--el-color-primary); }
.rule { font-size: 12px; color: var(--jz-text-dim); }
/* 列配置菜单 */
.col-btn { width: 36px; height: 36px; padding: 0; border: 1px solid transparent; }
.col-btn:hover { border-color: var(--jz-border-hover); background: var(--el-fill-color); }
.col-menu { display: flex; flex-direction: column; gap: 2px; }
.col-menu-title { font-size: 12px; color: var(--jz-text-dim); padding: 2px 4px 6px; }
.col-item { display: flex; align-items: center; gap: 8px; padding: 5px 4px; border-radius: 6px; }
.col-item:hover { background: var(--el-fill-color-light); }
.col-item.dragging { opacity: .5; }
.col-name { flex: 1; font-size: 13px; color: var(--jz-text); }
.drag-h { color: var(--jz-text-dim); cursor: grab; }
</style>

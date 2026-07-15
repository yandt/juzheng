<script setup lang="ts">
// 路由规则页 —— 直接编辑 config.orderedRules（有序扁平列表，顺序 = sing-box 匹配优先级）。
// 支持：拖拽重排 + 置顶/上移/下移；过滤查询；虚拟滚动（上万条不卡）；点击进入编辑。
// 只读行纯 HTML/轻组件；仅当前编辑行用 el-input/el-select。过滤时禁用重排（位置歧义）。
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { Plus, Delete, Edit, Top, ArrowUp, ArrowDown } from '@element-plus/icons-vue'
import { useSbox } from '../composables/useSbox'
import { t } from '../i18n'
import type { RuleMatchType, OrderedRule } from '../configModel'

const { config, configDirty, saving, markDirty } = useSbox()

const matchTypes: { value: RuleMatchType; labelKey: string }[] = [
  { value: 'domain_suffix', labelKey: 'rules.matchType.domainSuffix' },
  { value: 'domain_keyword', labelKey: 'rules.matchType.domainKeyword' },
  { value: 'domain', labelKey: 'rules.matchType.domain' },
  { value: 'domain_regex', labelKey: 'rules.matchType.domainRegex' },
  { value: 'ip_cidr', labelKey: 'rules.matchType.ipCidr' },
  { value: 'protocol', labelKey: 'rules.matchType.protocol' },
  // 按发起进程分流（依赖 route.find_process,序列化时已强制开启）。
  { value: 'process_name', labelKey: 'rules.matchType.processName' },
  { value: 'process_path', labelKey: 'rules.matchType.processPath' },
  { value: 'process_path_regex', labelKey: 'rules.matchType.processPathRegex' },
]
const matchTypeLabel = (mt: RuleMatchType) => {
  const found = matchTypes.find(m => m.value === mt)
  return found ? t(found.labelKey) : mt
}

const outboundOptions = computed(() => [
  ...(config.value.groups ?? []).map(g => ({ label: t('rules.outboundGroup', { tag: g.tag }), value: g.tag })),
  ...(config.value.nodes ?? []).map(n => ({ label: t('rules.outboundNode', { tag: n.tag }), value: n.tag })),
  ...(config.value.systemNodes ?? []).map(n => ({ label: t('rules.outboundSystem', { tag: n.tag }), value: n.tag })),
])

// 有序规则列表就是 config.orderedRules（唯一真源）。确保数组存在，供直接增删改/重排。
function rules(): OrderedRule[] {
  if (!config.value.orderedRules) config.value.orderedRules = []
  return config.value.orderedRules
}
const all = computed<OrderedRule[]>(() => config.value.orderedRules ?? [])

// 过滤（按 值/出口/类型），保留原始下标 origIdx（编辑/删除/重排都作用于原数组）。
const query = ref('')
const reorderable = computed(() => query.value.trim() === '') // 过滤时不可重排
const filtered = computed<{ origIdx: number; rule: OrderedRule }[]>(() => {
  const arr = all.value
  const q = query.value.trim().toLowerCase()
  if (!q) return arr.map((rule, i) => ({ origIdx: i, rule }))
  const out: { origIdx: number; rule: OrderedRule }[] = []
  for (let i = 0; i < arr.length; i++) {
    const r = arr[i]
    if (r.value.toLowerCase().includes(q) || r.outbound.toLowerCase().includes(q) || matchTypeLabel(r.matchType).toLowerCase().includes(q))
      out.push({ origIdx: i, rule: r })
  }
  return out
})
watch(query, () => { if (scrollEl.value) scrollEl.value.scrollTop = 0; scrollTop.value = 0 })
// config 整体替换（切换订阅/重载）时重置编辑态。
watch(() => config.value, () => { editingIdx.value = null })

// ===== 虚拟滚动 =====
const ROW_H = 36
const BUFFER = 8
const scrollEl = ref<HTMLElement | null>(null)
const scrollTop = ref(0)
const viewportH = ref(600)
let ro: ResizeObserver | null = null
function onScroll() { if (scrollEl.value) scrollTop.value = scrollEl.value.scrollTop }
function measure() { if (scrollEl.value) viewportH.value = scrollEl.value.clientHeight }
onMounted(() => {
  nextTick(measure)
  if (scrollEl.value && typeof ResizeObserver !== 'undefined') { ro = new ResizeObserver(measure); ro.observe(scrollEl.value) }
})
onBeforeUnmount(() => ro?.disconnect())
const totalH = computed(() => filtered.value.length * ROW_H)
const startIdx = computed(() => Math.max(0, Math.floor(scrollTop.value / ROW_H) - BUFFER))
const endIdx = computed(() => Math.min(filtered.value.length, Math.ceil((scrollTop.value + viewportH.value) / ROW_H) + BUFFER))
const visible = computed(() => {
  const out: { pos: number; origIdx: number; rule: OrderedRule }[] = []
  const f = filtered.value
  for (let i = startIdx.value; i < endIdx.value; i++) { const it = f[i]; if (it) out.push({ pos: i, origIdx: it.origIdx, rule: it.rule }) }
  return out
})

// ===== 编辑 =====
const editingIdx = ref<number | null>(null)
function startEdit(idx: number) { editingIdx.value = idx }
function stopEdit() { editingIdx.value = null; markDirty() }
function addRule() {
  query.value = ''
  rules().push({ value: '', matchType: 'domain_suffix', outbound: outboundOptions.value[0]?.value ?? 'direct' })
  editingIdx.value = rules().length - 1
  nextTick(() => { if (scrollEl.value) scrollEl.value.scrollTop = totalH.value })
}
function removeRule(idx: number) {
  rules().splice(idx, 1)
  if (editingIdx.value === idx) editingIdx.value = null
  markDirty()
}

// ===== 重排（顺序 = 优先级） =====
function move(from: number, to: number) {
  const arr = rules()
  if (to < 0 || to >= arr.length || from === to) return
  const [item] = arr.splice(from, 1)
  arr.splice(to, 0, item)
  editingIdx.value = null
  markDirty()
}
const moveTop = (i: number) => move(i, 0)
const moveUp = (i: number) => move(i, i - 1)
const moveDown = (i: number) => move(i, i + 1)

// 拖拽重排（HTML5 DnD）。仅未过滤时可用。
const dragFrom = ref<number | null>(null)
function onDragStart(i: number) { if (reorderable.value) dragFrom.value = i }
function onDrop(to: number) {
  if (!reorderable.value || dragFrom.value === null) return
  move(dragFrom.value, to)
  dragFrom.value = null
}
</script>

<template>
  <PageShell :title="t('rules.title')">
    <template #actions>
      <SaveStatus :saving="saving" :dirty="configDirty" />
    </template>

    <div class="rules-page">
      <el-alert type="info" :closable="false" show-icon class="info-alert"
        :title="t('rules.hint')" />

      <div class="rules-toolbar">
        <el-button size="small" type="primary" plain @click="addRule">
          <el-icon><Plus /></el-icon>&nbsp;{{ t('rules.add') }}
        </el-button>
        <FilterInput v-model="query" :placeholder="t('rules.filterPlaceholder')" class="rules-filter" />
        <span class="rules-count">
          <template v-if="query.trim()">{{ t('rules.countFiltered', { matched: filtered.length, total: all.length }) }}</template>
          <template v-else>{{ t('rules.countAll', { total: all.length }) }}</template>
        </span>
      </div>

      <div v-if="all.length === 0" class="rules-empty">{{ t('rules.empty') }}</div>
      <div v-else-if="filtered.length === 0" class="rules-empty">{{ t('rules.noMatch', { query }) }}</div>

      <div v-else ref="scrollEl" class="rule-scroll" @scroll="onScroll">
        <div class="rule-spacer" :style="{ height: totalH + 'px' }">
          <div
            v-for="v in visible"
            :key="v.origIdx"
            class="rule-abs"
            :style="{ top: v.pos * ROW_H + 'px', height: ROW_H + 'px' }"
            :draggable="reorderable && editingIdx !== v.origIdx"
            @dragstart="onDragStart(v.origIdx)"
            @dragover.prevent
            @drop="onDrop(v.origIdx)"
          >
            <!-- 编辑态：仅当前行用重控件 -->
            <div v-if="editingIdx === v.origIdx" class="rule-row editing">
              <el-input v-model="v.rule.value" size="small" :placeholder="t('rules.valuePlaceholder')" class="rule-value" @keyup.enter="stopEdit" />
              <el-select v-model="v.rule.matchType" size="small" class="rule-type">
                <el-option v-for="mt in matchTypes" :key="mt.value" :label="t(mt.labelKey)" :value="mt.value" />
              </el-select>
              <el-select v-model="v.rule.outbound" size="small" filterable class="rule-outbound" :placeholder="t('rules.outboundPlaceholder')">
                <el-option v-for="o in outboundOptions" :key="o.value" :label="o.label" :value="o.value" />
              </el-select>
              <el-button size="small" type="primary" @click="stopEdit">{{ t('rules.done') }}</el-button>
              <el-button text size="small" type="danger" @click="removeRule(v.origIdx)"><el-icon><Delete /></el-icon></el-button>
            </div>
            <!-- 只读态：纯文字 + 重排/编辑/删除 -->
            <div v-else class="rule-row readonly">
              <SvgIcon v-if="reorderable" name="drag" :size="14" class="drag-handle" :title="t('rules.dragReorder')" />
              <span class="prio">{{ v.origIdx + 1 }}</span>
              <span class="rv" @click="startEdit(v.origIdx)">{{ v.rule.value || t('rules.emptyValue') }}</span>
              <span class="rt">{{ matchTypeLabel(v.rule.matchType) }}</span>
              <span class="ro">→ {{ v.rule.outbound }}</span>
              <span class="row-actions">
                <template v-if="reorderable">
                  <el-icon class="ic" :title="t('rules.moveTop')" @click="moveTop(v.origIdx)"><Top /></el-icon>
                  <el-icon class="ic" :title="t('rules.moveUp')" @click="moveUp(v.origIdx)"><ArrowUp /></el-icon>
                  <el-icon class="ic" :title="t('rules.moveDown')" @click="moveDown(v.origIdx)"><ArrowDown /></el-icon>
                </template>
                <el-icon class="ic ic-edit" :title="t('rules.edit')" @click="startEdit(v.origIdx)"><Edit /></el-icon>
                <el-icon class="ic ic-del" :title="t('rules.delete')" @click="removeRule(v.origIdx)"><Delete /></el-icon>
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </PageShell>
</template>

<style scoped>
.rules-page { display: flex; flex-direction: column; height: 100%; min-height: 0; }
.info-alert { margin: 0 0 8px; flex: 0 0 auto; }
.rules-toolbar { display: flex; align-items: center; gap: 10px; margin-bottom: 8px; flex: 0 0 auto; }
.rules-filter { width: 260px; }
.rules-count { font-size: 11px; color: var(--jz-text-dim); margin-left: auto; flex: 0 0 auto; }
.rules-empty { font-size: 12px; color: var(--jz-text-dim); padding: 20px; text-align: center; background: var(--jz-surface); border: 1px dashed var(--jz-border); border-radius: 4px; }

.rule-scroll { flex: 1; min-height: 0; overflow-y: auto; position: relative; }
.rule-spacer { position: relative; width: 100%; }
.rule-abs { position: absolute; left: 0; right: 0; padding-bottom: 4px; }

.rule-row { display: flex; align-items: center; gap: 6px; padding: 4px 8px; height: 32px; box-sizing: border-box; background: var(--jz-surface); border: 1px solid var(--jz-border); border-radius: 4px; }
.rule-row:hover { border-color: var(--jz-border-hover); }
.rule-row.readonly .drag-handle { color: var(--jz-text-dim); cursor: grab; flex-shrink: 0; }
.rule-row.readonly .prio { width: 34px; flex-shrink: 0; font-size: 11px; color: var(--jz-text-dim); text-align: right; font-family: monospace; }
.rule-row.readonly .rv { flex: 1; font-family: monospace; font-size: 12px; color: var(--jz-text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; cursor: pointer; }
.rule-row.readonly .rt { width: 90px; flex-shrink: 0; font-size: 11px; color: var(--jz-text-dim); }
.rule-row.readonly .ro { width: 170px; flex-shrink: 0; font-size: 12px; color: var(--el-color-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.row-actions { display: flex; align-items: center; gap: 8px; flex-shrink: 0; opacity: 0; transition: opacity .1s; }
.rule-row.readonly:hover .row-actions { opacity: 1; }
.row-actions .el-icon { font-size: 14px; cursor: pointer; color: var(--jz-text-dim); }
.row-actions .ic-edit { color: var(--jz-text-dim); }
.row-actions .ic-del { color: #f56c6c; }

.rule-value { flex: 1; }
.rule-value :deep(.el-input__inner) { font-family: monospace; font-size: 12px; }
.rule-type { width: 120px; flex-shrink: 0; }
.rule-outbound { width: 170px; flex-shrink: 0; }
</style>

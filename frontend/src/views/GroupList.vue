<script setup lang="ts">
// 代理与节点页面 —— 用 GroupCollapseItem / NodeCard / NodeForm / FormDialog / useCrudDialog 组件化。
import { ref, computed } from 'vue'
import { Plus } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useSbox } from '../composables/useSbox'
import { useSubscriptions } from '../composables/useSubscriptions'
import { useClashApi } from '../composables/useClashApi'
import { useCrudDialog } from '../composables/ui/useCrudDialog'
import { renameOutbound, removeOutbound } from '../configOps'
import { t } from '../i18n'
import type { SingBoxNode, SelectorGroup } from '../configModel'

const { config, sboxRunning, configDirty, saving, markDirty } = useSbox()
const { activeSubscription } = useSubscriptions()
const { delays, testing, selectNode, testDelay } = useClashApi()

const activeTab = ref('groups')
const activeCollapse = ref<string>('')
const testingNode = ref('')
// 节点过滤（每个代理组折叠面板独立过滤文本）
const filterText = ref<Record<number, string>>({})

// 成员节点类型查询
function memberNodeType(tag: string): string {
  const n = config.value.nodes.find(n => n.tag === tag)
  if (n) return n.type
  if (tag === 'direct') return t('groups.nodeDirect')
  if (tag === 'block') return t('groups.nodeBlock')
  if (tag === 'dns-out' || tag === 'dns') return 'DNS'
  const g = config.value.groups?.find(g => g.tag === tag)
  return g ? g.type : 'unknown'
}

// 节点切换/测速。切换默认节点走 clash_api 实时生效（selectNode），仅存盘无需内核重载。
async function onGroupNodeChange(g: SelectorGroup, nodeTag: string) {
  g.default = nodeTag; markDirty(false)
  if (sboxRunning.value) await selectNode(g.tag, nodeTag)
}
async function onTestAll(members: string[]) {
  if (!sboxRunning.value) { ElMessage.warning(t('groups.errTestNeedsRunning')); return }
  for (const m of members) { testingNode.value = m; await testDelay(m) }
  testingNode.value = ''
}

// 节点 CRUD（useCrudDialog）
const nodeTags = computed(() => config.value.nodes.map(n => n.tag))
const nodeCrud = useCrudDialog<SingBoxNode>({
  list: () => config.value.nodes,
  makeDefault: () => ({ type: 'vmess', tag: `node-${config.value.nodes.length + 1}`, server: '', server_port: 443, rules: [] }),
  validate: (d) => { if (!d.tag) return t('groups.errNodeTag') },
  // 改名/删除都走集中的 configOps：级联更新/清除 groups、规则、DNS detour、route.final 里的全部引用。
  onRename: (oldTag, newTag) => renameOutbound(config.value, oldTag, newTag),
  beforeRemove: (item) => removeOutbound(config.value, item.tag),
  messages: { added: t('groups.nodeAdded'), updated: t('groups.nodeUpdated'), confirmRemove: (n) => t('groups.confirmRemoveNode', { tag: n.tag }) },
})

// 代理组 CRUD
const groupCrud = useCrudDialog<SelectorGroup>({
  list: () => config.value.groups ?? (config.value.groups = []),
  makeDefault: () => ({ type: 'selector', tag: `组${(config.value.groups?.length ?? 0) + 1}`, outbounds: [], rules: [] }),
  validate: (d) => {
    if (!d.tag) return t('groups.errGroupTag')
    if (d.outbounds.length === 0) return t('groups.errOutbounds')
  },
  // 代理组改名/删除同样走集中的 configOps（组会被其它组、规则、route.final 引用）。
  onRename: (oldTag, newTag) => renameOutbound(config.value, oldTag, newTag),
  beforeRemove: (g) => removeOutbound(config.value, g.tag),
  messages: { added: t('groups.groupAdded'), updated: t('groups.groupUpdated'), confirmRemove: (g) => t('groups.confirmRemoveGroup', { tag: g.tag }) },
})
const defaultOptions = computed(() => groupCrud.draft.value.outbounds)

// 节点拖拽排序
const dragNodeIndex = ref<number | null>(null)
const dragOverIndex = ref<number | null>(null)
function onDragStart(i: number) { dragNodeIndex.value = i }
function onDragOver(e: DragEvent, i: number) { e.preventDefault(); dragOverIndex.value = i }
function onDrop(i: number) {
  if (dragNodeIndex.value === null || dragNodeIndex.value === i) { dragNodeIndex.value = null; dragOverIndex.value = null; return }
  const arr = config.value.nodes
  const [moved] = arr.splice(dragNodeIndex.value, 1)
  arr.splice(i, 0, moved)
  markDirty(); dragNodeIndex.value = null; dragOverIndex.value = null
}
function onDragEnd() { dragNodeIndex.value = null; dragOverIndex.value = null }

// 辅助
function isNodeInUse(tag: string): boolean {
  return (config.value.groups ?? []).some(g => g.outbounds.includes(tag))
}
function groupsOfNodeCount(tag: string): number {
  return (config.value.groups ?? []).filter(g => g.outbounds.includes(tag)).length
}

// 即改即存：所有编辑（切换默认节点、拖拽排序、节点/组增删改）都通过 markDirty
// 触发防抖自动保存，无需手动保存按钮。
</script>

<template>
  <PageShell :title="t('groups.title')">
    <template #title-extra>
      <el-tag v-if="activeSubscription" type="primary" size="small" effect="plain">{{ t('groups.subscription', { name: activeSubscription }) }}</el-tag>
    </template>
    <template #actions>
      <SaveStatus :saving="saving" :dirty="configDirty" />
    </template>
    <el-alert v-if="sboxRunning" type="info" :closable="false" show-icon class="info-alert"
      :title="t('groups.runningAlert')" />

    <el-tabs v-model="activeTab" class="page-tabs">
      <!-- 代理组 Tab -->
      <el-tab-pane :label="t('groups.tabGroups', { n: config.groups?.length ?? 0 })" name="groups">
        <div class="section-head">
          <h4>{{ t('groups.sectionGroups') }}</h4>
          <el-button type="primary" size="small" @click="groupCrud.openAdd()">
            <el-icon><Plus /></el-icon>&nbsp;{{ t('groups.newGroup') }}
          </el-button>
        </div>
        <el-collapse v-if="(config.groups?.length ?? 0) > 0" v-model="activeCollapse" accordion>
          <GroupCollapseItem
            v-for="(g, i) in (config.groups ?? [])"
            :key="i"
            :group="g"
            :testing-node="testingNode"
            :sbox-running="sboxRunning"
            :filter-text="filterText[i] ?? ''"
            :delays="delays"
            :testing="testing"
            :member-node-type="memberNodeType"
            @update:filter-text="filterText[i] = $event"
            @edit-group="groupCrud.openEdit(i)"
            @test-all="onTestAll"
            @node-change="(tag: string) => onGroupNodeChange(g, tag)"
          />
        </el-collapse>
        <el-empty v-else :description="t('groups.emptyGroups')" :image-size="50" />
      </el-tab-pane>

      <!-- 节点 Tab -->
      <el-tab-pane :label="t('groups.tabNodes', { n: config.nodes.length })" name="nodes">
        <div class="section-head">
          <h4>{{ t('groups.sectionNodes') }}</h4>
          <el-button type="primary" size="small" @click="nodeCrud.openAdd()">
            <el-icon><Plus /></el-icon>&nbsp;{{ t('groups.newNode') }}
          </el-button>
        </div>
        <div class="card-wall node-wall">
          <NodeCard
            v-for="(n, i) in config.nodes"
            :key="n.tag"
            :node="n"
            :dragging="dragNodeIndex === i"
            :drag-over="dragOverIndex === i && dragNodeIndex !== i"
            :group-count="groupsOfNodeCount(n.tag)"
            :in-use="isNodeInUse(n.tag)"
            @edit="nodeCrud.openEdit(i)"
            @remove="nodeCrud.remove(i)"
            @dragstart="onDragStart(i)"
            @dragover="onDragOver($event, i)"
            @drop="onDrop(i)"
            @dragend="onDragEnd"
          />
          <el-empty v-if="config.nodes.length === 0" :description="t('groups.emptyNodes')" :image-size="50" />
        </div>
      </el-tab-pane>
    </el-tabs>

    <!-- 节点编辑弹窗 -->
    <FormDialog v-model="nodeCrud.dialog.value" :title="nodeCrud.index.value === -1 ? t('groups.dialogAddNode') : t('groups.dialogEditNode')" width="520px" @save="() => { nodeCrud.save(); markDirty() }">
      <NodeForm v-model="nodeCrud.draft.value" />
    </FormDialog>

    <!-- 代理组编辑弹窗 -->
    <FormDialog v-model="groupCrud.dialog.value" :title="groupCrud.index.value === -1 ? t('groups.dialogAddGroup') : t('groups.dialogEditGroup')" width="640px" @save="() => { groupCrud.save(); markDirty() }">
      <el-form label-width="100px">
        <el-form-item :label="t('groups.fieldType')">
          <el-radio-group v-model="groupCrud.draft.value.type">
            <el-radio value="selector">{{ t('groups.typeSelector') }}</el-radio>
            <el-radio value="urltest">{{ t('groups.typeUrltest') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="t('groups.fieldTag')" required>
          <el-input v-model="groupCrud.draft.value.tag" :placeholder="t('groups.tagPlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('groups.fieldOutbounds')" required>
          <el-select v-model="groupCrud.draft.value.outbounds" multiple filterable :placeholder="t('groups.outboundsPlaceholder')" style="width: 100%">
            <el-option v-for="t in nodeTags" :key="t" :label="t" :value="t" />
          </el-select>
          <div class="field-hint">{{ t('groups.outboundsHint') }}</div>
        </el-form-item>
        <el-form-item v-if="groupCrud.draft.value.type === 'selector'" :label="t('groups.fieldDefault')">
          <el-select v-model="groupCrud.draft.value.default" clearable :placeholder="t('groups.defaultPlaceholder')">
            <el-option v-for="t in defaultOptions" :key="t" :label="t" :value="t" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('groups.fieldUrl')">
          <el-input v-model="groupCrud.draft.value.url" placeholder="https://www.gstatic.com/generate_204" />
        </el-form-item>
        <el-form-item v-if="groupCrud.draft.value.type === 'urltest'" :label="t('groups.fieldInterval')">
          <el-input v-model="groupCrud.draft.value.interval" placeholder="3m" />
        </el-form-item>
      </el-form>
    </FormDialog>

    <!-- 规则管理已移到侧栏「规则」页 -->
  </PageShell>
</template>

<style scoped>
.info-alert { margin: 8px 0; }
.section-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; }
.section-head h4 { margin: 0; color: var(--el-text-color-regular); font-size: 14px; }
.field-hint { color: var(--jz-text-dim); font-size: 11px; margin-top: 4px; }
:deep(.page-body) { display: flex; flex-direction: column; overflow: hidden; padding: 8px 16px 16px; }
.page-tabs { margin-top: 0; flex: 1; display: flex; flex-direction: column; min-height: 0; }
.page-tabs :deep(.el-tabs__content) { padding-top: 4px; flex: 1; overflow-y: auto; min-height: 0; }
.page-tabs :deep(.el-tabs__header) { margin-bottom: 12px; flex: 0 0 auto; }
/* el-collapse 容器样式（在父组件里，:deep 穿透到子组件的 el-collapse-item） */
.el-collapse { border-top: 1px solid var(--jz-border); border-bottom: 1px solid var(--jz-border); }
.el-collapse :deep(.el-collapse-item) { border-bottom: 1px solid var(--jz-border); }
.el-collapse :deep(.el-collapse-item:last-child) { border-bottom: none; }
.card-wall { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 10px; }
/* 节点卡片墙：让 NodeCard 组件均匀排列 */
.node-wall { align-items: start; }
</style>

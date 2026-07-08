<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Download, Plus, Refresh } from '@element-plus/icons-vue'
import { useSubscriptions, formatBytes, usagePercent, formatExpire, daysUntilExpire } from '../composables/useSubscriptions'
import { useSbox } from '../composables/useSbox'
import { t } from '../i18n'

// 所有订阅数据操作（导入/导出/刷新/启停）都走 composable，view 只管本地 UI 状态。
const {
  subscriptions, refreshSubscriptions, setActive, createSubscription, removeSubscription, renameSubscription,
  refreshInfo: refreshInfoAction, importFromFile: importFromFileAction,
  importFromUrl: importFromUrlAction, importFromJson: importFromJsonAction, exportSubscription,
} = useSubscriptions()
const { loadConfig } = useSbox()

onMounted(() => refreshSubscriptions())

// 刷新单个订阅用量（本地只维护"哪一项在刷新"的 UI 状态）
const refreshingName = ref('')
async function refreshInfo(name: string) {
  refreshingName.value = name
  try {
    await refreshInfoAction(name)
  } finally {
    refreshingName.value = ''
  }
}

// 导入对话框（纯 UI 本地状态）
const importDialog = ref(false)
const importTab = ref('file')
const importName = ref('')
const importUrl = ref('')
const importJson = ref('')
const importing = ref(false)

function openImport() {
  importName.value = `${t('subs.defaultNamePrefix')}-${new Date().toLocaleDateString('zh-CN').replace(/\//g, '')}`
  importUrl.value = ''
  importJson.value = ''
  importTab.value = 'file'
  importDialog.value = true
}

async function importFromFile() {
  if (!importName.value) { ElMessage.warning(t('subs.enterName')); return }
  importing.value = true
  try {
    if (await importFromFileAction(importName.value)) importDialog.value = false
  } finally {
    importing.value = false
  }
}

async function importFromUrl() {
  if (!importName.value) { ElMessage.warning(t('subs.enterName')); return }
  if (!importUrl.value) { ElMessage.warning(t('subs.enterUrl')); return }
  importing.value = true
  try {
    if (await importFromUrlAction(importName.value, importUrl.value)) importDialog.value = false
  } finally {
    importing.value = false
  }
}

async function importFromJson() {
  if (!importName.value) { ElMessage.warning(t('subs.enterName')); return }
  if (!importJson.value.trim()) { ElMessage.warning(t('subs.pasteContent')); return }
  importing.value = true
  try {
    if (await importFromJsonAction(importName.value, importJson.value)) importDialog.value = false
  } finally {
    importing.value = false
  }
}

// 启用订阅（每次只能启用一个，作为内核运行配置）
async function onSetActive(name: string) {
  await setActive(name)
  await loadConfig() // 重载运行副本到前端
}

// 导出订阅（委托 composable）
function exportSub(name: string) {
  return exportSubscription(name)
}
</script>

<template>
  <PageShell :title="t('subs.title')">
    <template #actions>
      <IconButton :icon="Download" type="primary" :title="t('subs.import')" @click="openImport" />
      <IconButton :icon="Plus" :title="t('subs.createEmpty')" @click="createSubscription('')" />
      <IconButton :icon="Refresh" :title="t('subs.refreshList')" @click="refreshSubscriptions" />
    </template>

    <!-- 订阅列表 -->
    <div class="card-wall">
      <el-card v-for="s in subscriptions" :key="s.name" class="sub-card" :class="{ active: s.active }" shadow="hover">
        <div class="card-head">
          <el-icon class="sub-icon"><Files /></el-icon>
          <span class="sub-name">{{ s.name }}</span>
          <el-tag v-if="s.active" type="success" size="small" effect="dark">{{ t('subs.enabled') }}</el-tag>
        </div>
        <div class="card-meta">
          <div><el-icon><Connection /></el-icon> {{ t('subs.nodeCount', { n: s.nodeCount }) }}</div>
          <div><el-icon><Clock /></el-icon> {{ new Date(s.updatedAt * 1000).toLocaleString('zh-CN') }}</div>
        </div>
        <!-- 用量/到期（仅 URL 导入且机场返回了 Subscription-Userinfo 才有） -->
        <div v-if="s.info" class="usage-block">
          <el-progress
            :percentage="usagePercent(s.info)"
            :color="usagePercent(s.info) > 90 ? '#f56c6c' : '#409eff'"
            :stroke-width="12"
            :text-inside="true"
            :format="() => `${formatBytes(s.info!.upload + s.info!.download)} / ${formatBytes(s.info!.total)}`"
          />
          <div class="expire-row">
            <el-icon><Calendar /></el-icon>
            <span :class="{ expired: (daysUntilExpire(s.info.expire) ?? 999) < 0 }">
              {{ formatExpire(s.info.expire) || t('subs.noExpire') }}
              <template v-if="daysUntilExpire(s.info.expire) !== null">
                · {{ (daysUntilExpire(s.info.expire) ?? 0) >= 0 ? t('subs.daysLeft', { d: daysUntilExpire(s.info.expire) ?? 0 }) : t('subs.expiredDays', { d: -(daysUntilExpire(s.info.expire) ?? 0) }) }}
              </template>
            </span>
          </div>
        </div>
        <div v-else-if="s.sourceUrl" class="no-usage">{{ t('subs.noUsage') }}</div>
        <div class="card-actions">
          <el-button v-if="!s.active" size="small" type="primary" @click="onSetActive(s.name)">{{ t('subs.enable') }}</el-button>
          <el-tag v-else type="success" size="small" effect="plain" class="enabled-mark">{{ t('subs.runningWithThis') }}</el-tag>
          <el-button
            v-if="s.sourceUrl"
            text size="small"
            :loading="refreshingName === s.name"
            @click="refreshInfo(s.name)"
          ><el-icon><Refresh /></el-icon>&nbsp;{{ t('subs.refreshUsage') }}</el-button>
          <el-button text size="small" @click="renameSubscription(s.name)">{{ t('subs.rename') }}</el-button>
          <el-button text size="small" @click="exportSub(s.name)">{{ t('subs.export') }}</el-button>
          <el-button v-if="!s.active" text size="small" type="danger" @click="removeSubscription(s.name)">{{ t('subs.delete') }}</el-button>
        </div>
      </el-card>
      <el-empty v-if="subscriptions.length === 0" :description="t('subs.empty')" />
    </div>

    <!-- 导入对话框 -->
    <el-dialog v-model="importDialog" :title="t('subs.import')" width="540px">
      <el-form label-width="90px">
        <el-form-item :label="t('subs.nameLabel')">
          <el-input v-model="importName" :placeholder="t('subs.namePlaceholder')" />
        </el-form-item>
        <el-tabs v-model="importTab">
          <el-tab-pane :label="t('subs.tabFile')" name="file">
            <div class="import-hint">{{ t('subs.fileHint') }}</div>
            <el-button type="primary" :loading="importing" @click="importFromFile">{{ t('subs.selectFile') }}</el-button>
          </el-tab-pane>
          <el-tab-pane :label="t('subs.tabUrl')" name="url">
            <el-input v-model="importUrl" placeholder="https://example.com/sub" />
            <div class="import-hint">{{ t('subs.urlHint') }}</div>
            <el-button type="primary" :loading="importing" @click="importFromUrl" style="margin-top: 8px">{{ t('subs.downloadImport') }}</el-button>
          </el-tab-pane>
          <el-tab-pane :label="t('subs.tabPaste')" name="json">
            <el-input v-model="importJson" type="textarea" :rows="8" :placeholder="t('subs.pastePlaceholder')" />
            <el-button type="primary" :loading="importing" @click="importFromJson" style="margin-top: 8px">{{ t('subs.importBtn') }}</el-button>
          </el-tab-pane>
        </el-tabs>
      </el-form>
    </el-dialog>
  </PageShell>
</template>

<style scoped>
.card-wall { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 12px; }
.sub-card { border-left: 3px solid transparent; }
.sub-card.active { border-left-color: #4ade80; }
.sub-card :deep(.el-card__body) { padding: 14px; }
.card-head { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.sub-icon { color: #8ab4f8; }
.sub-name { font-weight: 600; color: var(--jz-text); flex: 1; }
.card-meta { display: flex; flex-direction: column; gap: 4px; font-size: 12px; color: var(--jz-text-dim); margin-bottom: 8px; }
.card-meta div { display: flex; align-items: center; gap: 4px; }
.usage-block { margin-bottom: 8px; }
.expire-row { display: flex; align-items: center; gap: 4px; font-size: 11px; color: var(--jz-text-dim); margin-top: 6px; }
.expire-row .expired { color: #f56c6c; }
.no-usage { font-size: 11px; color: var(--jz-text-dim); margin-bottom: 8px; }
.card-actions { display: flex; gap: 4px; flex-wrap: wrap; border-top: 1px solid var(--jz-border); padding-top: 8px; margin-top: 8px; align-items: center; }
.enabled-mark { flex: 1; }
.import-hint { color: var(--jz-text-dim); font-size: 12px; margin: 8px 0; }
</style>

<script setup lang="ts">
import { ElMessage } from 'element-plus'
import { Delete } from '@element-plus/icons-vue'
import { useMitm } from '../composables/useMitm'
import { useSbox } from '../composables/useSbox'
import { t } from '../i18n'

const {
  flows, flowList, filteredList, filterHost,
  selectedId, selected,
  running, addr, caCertPath,
  certStatus, installing,
  toggleProxy, installCert, refreshCert, clearFlows, selectFlow,
} = useMitm()

// sing-box 状态：决定流量路径提示
const { sboxRunning } = useSbox()

function codeClass(c: number | undefined): string {
  if (!c) return ''
  if (c < 300) return 'ok'
  if (c < 400) return 'redir'
  if (c < 500) return 'err'
  return 'err5'
}

function prettyBody(s: string): string {
  if (!s) return ''
  try { return JSON.stringify(JSON.parse(s), null, 2) } catch { return s }
}

async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success(t('traffic.copied'))
  } catch { ElMessage.error(t('traffic.copyFailed')) }
}

function fmtTime(s: string | undefined): string {
  if (!s) return ''
  const d = new Date(s)
  return isNaN(d.getTime()) ? s : d.toLocaleTimeString('zh-CN', { hour12: false })
}
</script>

<template>
  <PageShell :title="t('traffic.title')">
    <template #title-extra>
      <el-tag v-if="running" type="success" size="small" effect="dark">{{ t('traffic.monitoringAt', { addr }) }}</el-tag>
      <el-tag v-else type="info" size="small">{{ t('traffic.stopped') }}</el-tag>
      <el-tag v-if="certStatus.exists && certStatus.trusted" type="success" size="small" effect="dark">
        {{ t('traffic.caTrusted', { fp: (certStatus.fingerprint || '').slice(0, 16) }) }}
      </el-tag>
    </template>
    <template #actions>
      <IconButton
        :svg="running ? 'stop' : 'play'"
        :color="running ? 'var(--el-color-danger)' : 'var(--el-color-success)'"
        :title="running ? t('traffic.stopMonitor') : t('traffic.startMonitor')"
        @click="toggleProxy"
      />
      <IconButton :icon="Delete" :title="t('traffic.clearList')" @click="clearFlows" />
    </template>

    <!-- 证书引导 -->
    <el-alert
      v-if="certStatus.exists && !certStatus.trusted"
      type="warning" :closable="false" show-icon
      :title="t('traffic.certUntrusted')"
      class="cert-alert"
    >
      <template #default>
        <el-button type="primary" size="small" :loading="installing" @click="installCert">{{ t('traffic.installTrust') }}</el-button>
        <el-button size="small" @click="copyText(certStatus.installCmd)">{{ t('traffic.copyCmd') }}</el-button>
        <el-button size="small" @click="refreshCert">{{ t('traffic.refresh') }}</el-button>
      </template>
    </el-alert>

    <!-- 流量路径提示：虚拟网卡模式运行时无需设置；否则需 HTTPS_PROXY -->
    <el-alert v-if="running && sboxRunning" type="success" :closable="false" show-icon class="hint-alert">
      {{ t('traffic.tunHint') }}
    </el-alert>
    <el-alert v-else-if="running" type="info" :closable="false" show-icon class="hint-alert">
      <template #title>
        {{ t('traffic.mitmStarted') }}
        <code>HTTPS_PROXY=http://127.0.0.1{{ addr }} your-app</code>
        <el-button text size="small" @click="copyText(`HTTPS_PROXY=http://127.0.0.1${addr} your-app`)">{{ t('traffic.copy') }}</el-button>
        {{ t('traffic.tunTip') }}
      </template>
    </el-alert>

    <!-- 主体：列表 + 详情 -->
    <div class="main">
      <section class="list-pane">
        <FilterInput v-model="filterHost" :placeholder="t('traffic.filterPlaceholder')" class="filter" />
        <div class="count">{{ filteredList.length }} / {{ flowList.length }}</div>
        <div class="list">
          <div
            v-for="f in filteredList" :key="f.id"
            class="row" :class="{ sel: f.id === selectedId, sse: f.isSSE }"
            @click="selectFlow(f.id)"
          >
            <span class="m" :class="f.method">{{ f.method }}</span>
            <span class="host">{{ f.host }}</span>
            <span class="path">{{ f.path }}</span>
            <span class="code" :class="codeClass(f.statusCode)">{{ f.statusCode || '…' }}</span>
            <el-tag v-if="f.isSSE" size="small" type="warning" effect="plain">SSE {{ f.sseEvents?.length ?? 0 }}</el-tag>
          </div>
          <el-empty v-if="flowList.length === 0" :description="t('traffic.emptyFlows')" :image-size="60" />
        </div>
      </section>

      <section class="detail-pane">
        <el-empty v-if="!selected" :description="t('traffic.selectRequest')" :image-size="80" />
        <template v-else>
          <div class="detail-head">
            <el-tag :class="selected.method" effect="dark">{{ selected.method }}</el-tag>
            <span class="url">{{ selected.url }}</span>
            <el-tag :type="selected.statusCode < 400 ? 'success' : 'danger'" effect="dark">{{ selected.statusCode }}</el-tag>
            <span v-if="selected.durationMs" class="dur">{{ selected.durationMs }}ms</span>
          </div>

          <div class="tabs">
            <div class="tab-section">
              <div class="tab-head">
                <h4>{{ t('traffic.reqBody') }}</h4>
                <el-button text size="small" @click="copyText(selected.reqBody)">{{ t('traffic.copy') }}</el-button>
              </div>
              <pre class="body">{{ prettyBody(selected.reqBody) || t('traffic.empty') }}</pre>
            </div>
            <div v-if="selected.isSSE" class="tab-section">
              <h4>{{ t('traffic.sseStream', { n: selected.sseEvents?.length ?? 0 }) }}</h4>
              <div class="sse">
                <div v-for="(ev, i) in selected.sseEvents" :key="selected.id + '_' + i" class="sse-ev">
                  <el-tag size="small" type="warning">{{ ev.event || 'message' }}</el-tag>
                  <pre class="ev-data">{{ ev.data }}</pre>
                </div>
              </div>
            </div>
            <div v-else class="tab-section">
              <h4>{{ t('traffic.respBody') }}</h4>
              <pre class="body">{{ prettyBody(selected.respBody) || t('traffic.empty') }}</pre>
            </div>
          </div>
        </template>
      </section>
    </div>
  </PageShell>
</template>

<style scoped>
/* TrafficMonitor 主体需 flex 撑满（流量列表区），覆盖 PageShell page-body 的滚动 */
:deep(.page-body) { display: flex; flex-direction: column; overflow: hidden; padding: 0; }
.cert-alert { margin: 0; border-radius: 0; }
.hint-alert { margin: 0; border-radius: 0; border-left: 0; border-right: 0; }
.hint-alert code { background: var(--jz-surface-2); padding: 2px 6px; border-radius: 3px; font-size: 11px; }

.main { flex: 1; display: flex; overflow: hidden; }
.list-pane { width: 40%; display: flex; flex-direction: column; border-right: 1px solid var(--jz-border); padding: 8px; gap: 6px; }
.filter { }
.count { font-size: 11px; color: var(--jz-text-dim); padding: 0 4px; }
.list { flex: 1; overflow-y: auto; }
.row { display: flex; align-items: center; gap: 8px; padding: 6px 8px; border-radius: 5px; cursor: pointer; font-size: 12px; }
.row:hover { background: var(--el-fill-color); }
.row.sel { background: var(--el-color-primary-light-9); }
.row.sse { border-left: 2px solid #f59e0b; }
.m { font-size: 10px; font-weight: 700; padding: 1px 5px; border-radius: 3px; background: #333; color: #ccc; min-width: 36px; text-align: center; }
.m.GET { background: #1e3a5f; color: var(--el-color-primary); }
.m.POST { background: #1e4d3a; color: #4ade80; }
.m.PUT { background: #4d3a1e; color: #fbbf24; }
.m.DELETE { background: #4d1e1e; color: #f87171; }
.host { color: #8ab4f8; }
.path { color: var(--jz-text-dim); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.code { font-size: 11px; font-weight: 600; min-width: 28px; }
.code.ok { color: #4ade80; } .code.redir { color: #fbbf24; } .code.err, .code.err5 { color: #f87171; }

.detail-pane { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.detail-head { display: flex; align-items: center; gap: 8px; padding: 10px 14px; border-bottom: 1px solid var(--jz-border); font-size: 13px; }
.url { flex: 1; color: #ddd; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.dur { font-size: 11px; color: var(--jz-text-dim); }
.tabs { flex: 1; overflow-y: auto; padding: 0 14px 14px; }
.tab-section h4 { margin: 12px 0 6px; font-size: 12px; color: var(--jz-text-dim); font-weight: 600; display: flex; align-items: center; gap: 8px; }
.tab-head { display: flex; align-items: center; justify-content: space-between; }
.tab-head h4 { margin: 12px 0 6px; }
.body { background: #0f1116; border: 1px solid var(--jz-border); border-radius: 5px; padding: 10px; font-size: 11px; line-height: 1.5; overflow-x: auto; white-space: pre-wrap; word-break: break-word; max-height: 36vh; overflow-y: auto; color: var(--el-text-color-regular); font-family: "SF Mono", Menlo, monospace; }
.sse { display: flex; flex-direction: column; gap: 4px; max-height: 36vh; overflow-y: auto; }
.sse-ev { background: #0f1116; border: 1px solid var(--jz-border); border-radius: 4px; padding: 6px 8px; }
.ev-data { font-size: 11px; color: var(--el-text-color-regular); white-space: pre-wrap; word-break: break-word; margin: 4px 0 0; font-family: "SF Mono", Menlo, monospace; }
</style>

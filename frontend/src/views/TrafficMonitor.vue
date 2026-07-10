<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { Delete, Setting, Aim, Refresh } from '@element-plus/icons-vue'
import { useMitm } from '../composables/useMitm'
import { useSbox } from '../composables/useSbox'
import { useMonitor } from '../composables/useMonitor'
import { t } from '../i18n'

// active：本页是否可见（父层 v-show 控制）。本页常驻挂载，onMounted 只触发一次，
// 故用 active 变可见时补拉，保证"每次打开都拉到后端最新已抓流量"。
const props = defineProps<{ active?: boolean }>()

const {
  flows, flowList, filteredList, filterHost,
  selectedId, selected,
  running, addr, caCertPath,
  certStatus, installing,
  toggleProxy, installCert, refreshCert, clearFlows, backfillFlows, selectFlow,
} = useMitm()

// 打开页面即补拉：首挂载 + 每次由隐藏变可见。弥补订阅前/漏收的实时事件。
onMounted(() => { if (props.active !== false) backfillFlows() })
watch(() => props.active, (v) => { if (v) backfillFlows() })

// 手动刷新：重新从后端补拉已抓流量。
async function refreshFlows() {
  await backfillFlows()
  ElMessage.success(t('traffic.refreshed'))
}

// sing-box 状态：决定流量路径提示
const { sboxRunning } = useSbox()

// 抓包域名配置弹窗（与首页明文抓包同款，复用 CaptureDomainsDialog）
const showCapture = ref(false)

// 监控规则弹窗 + 命中标记
const showMonitor = ref(false)
const { hitByFlow } = useMonitor()

function prettyBody(s: string): string {
  if (!s) return ''
  try { return JSON.stringify(JSON.parse(s), null, 2) } catch { return s }
}

// headers 对象 → "Key: Value" 每行一条（key 升序，稳定展示）
function fmtHeaders(h: Record<string, string | undefined> | null | undefined): string {
  if (!h) return ''
  return Object.keys(h).sort().map(k => `${k}: ${h[k] ?? ''}`).join('\n')
}

// 详情区当前 tab：请求 / 响应
const detailTab = ref<'req' | 'resp'>('req')

// SSE 显示模式：合并（拼成整段流式文本）/ 分列（每个事件一块）。默认合并。
const sseMerged = ref(true)

// 从单个 SSE 事件的 data 中智能提取"文本增量"：
// 兼容 OpenAI(choices[].delta.content)、Anthropic(delta.text / content_block.text)、
// 以及裸 content/text 字段；无法解析或非文本增量则返回空串（不计入合并正文）。
function sseDelta(data: string): string {
  const raw = (data ?? '').trim()
  if (!raw || raw === '[DONE]') return ''
  let obj: any
  try { obj = JSON.parse(raw) } catch { return raw }  // 非 JSON：整段视为文本
  // OpenAI chat/completions 流
  const ch = obj?.choices?.[0]
  if (ch) {
    if (typeof ch.delta?.content === 'string') return ch.delta.content
    if (typeof ch.delta?.text === 'string') return ch.delta.text
    if (typeof ch.text === 'string') return ch.text
    return ''
  }
  // Anthropic messages 流
  if (typeof obj?.delta?.text === 'string') return obj.delta.text
  if (typeof obj?.content_block?.text === 'string') return obj.content_block.text
  // 裸字段兜底
  if (typeof obj?.content === 'string') return obj.content
  if (typeof obj?.text === 'string') return obj.text
  return ''
}

// 合并后的完整流式文本：拼接所有事件提取到的增量。
const mergedSSEText = computed(() => {
  const evs = selected.value?.sseEvents
  if (!evs) return ''
  return evs.map(ev => sseDelta(ev.data)).join('')
})

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
      <IconButton :icon="Refresh" :title="t('traffic.refresh')" @click="refreshFlows" />
      <IconButton :icon="Delete" :title="t('traffic.clearList')" @click="clearFlows" />
      <IconButton :icon="Setting" :title="t('home.mitmConfig')" @click="showCapture = true" />
      <IconButton :icon="Aim" :title="t('monitor.manage')" @click="showMonitor = true" />
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
            <span v-if="f.isSSE" class="sse-badge">SSE</span>
            <span
              v-if="hitByFlow[f.id]"
              class="hit-badge" :class="hitByFlow[f.id].level"
              :title="hitByFlow[f.id].ruleName"
            >{{ t('monitor.action_' + hitByFlow[f.id].action) }}</span>
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

          <el-tabs v-model="detailTab" class="detail-tabs">
            <!-- 请求 tab：headers + body -->
            <el-tab-pane :label="t('traffic.reqTab')" name="req">
              <div class="tab-section">
                <div class="tab-head">
                  <h4>{{ t('traffic.reqHeaders') }}</h4>
                  <el-button text size="small" @click="copyText(fmtHeaders(selected.reqHeaders))">{{ t('traffic.copy') }}</el-button>
                </div>
                <pre class="body headers">{{ fmtHeaders(selected.reqHeaders) || t('traffic.empty') }}</pre>
              </div>
              <div class="tab-section">
                <div class="tab-head">
                  <h4>{{ t('traffic.reqBody') }}</h4>
                  <el-button text size="small" @click="copyText(selected.reqBody)">{{ t('traffic.copy') }}</el-button>
                </div>
                <pre class="body">{{ prettyBody(selected.reqBody) || t('traffic.empty') }}</pre>
              </div>
            </el-tab-pane>

            <!-- 响应 tab：headers + body（SSE 时显示事件流） -->
            <el-tab-pane :label="t('traffic.respTab')" name="resp">
              <div class="tab-section">
                <div class="tab-head">
                  <h4>{{ t('traffic.respHeaders') }}</h4>
                  <el-button text size="small" @click="copyText(fmtHeaders(selected.respHeaders))">{{ t('traffic.copy') }}</el-button>
                </div>
                <pre class="body headers">{{ fmtHeaders(selected.respHeaders) || t('traffic.empty') }}</pre>
              </div>
              <div v-if="selected.isSSE" class="tab-section">
                <div class="tab-head">
                  <h4>{{ t('traffic.sseStream', { n: selected.sseEvents?.length ?? 0 }) }}</h4>
                  <div class="sse-actions">
                    <el-radio-group v-model="sseMerged" size="small">
                      <el-radio-button :value="true">{{ t('traffic.sseMerged') }}</el-radio-button>
                      <el-radio-button :value="false">{{ t('traffic.sseList') }}</el-radio-button>
                    </el-radio-group>
                    <el-button v-if="sseMerged" text size="small" @click="copyText(mergedSSEText)">{{ t('traffic.copy') }}</el-button>
                  </div>
                </div>
                <!-- 合并：拼成整段流式文本 -->
                <pre v-if="sseMerged" class="body">{{ mergedSSEText || t('traffic.empty') }}</pre>
                <!-- 分列：每个事件一块 -->
                <div v-else class="sse">
                  <div v-for="(ev, i) in selected.sseEvents" :key="selected.id + '_' + i" class="sse-ev">
                    <el-tag size="small" type="warning">{{ ev.event || 'message' }}</el-tag>
                    <pre class="ev-data">{{ ev.data }}</pre>
                  </div>
                </div>
              </div>
              <div v-else class="tab-section">
                <div class="tab-head">
                  <h4>{{ t('traffic.respBody') }}</h4>
                  <el-button text size="small" @click="copyText(selected.respBody)">{{ t('traffic.copy') }}</el-button>
                </div>
                <pre class="body">{{ prettyBody(selected.respBody) || t('traffic.empty') }}</pre>
              </div>
            </el-tab-pane>
          </el-tabs>
        </template>
      </section>
    </div>

    <!-- 抓包域名配置（点右上齿轮打开，与首页明文抓包同款；即改即存 + 运行中热重载生效） -->
    <CaptureDomainsDialog v-model="showCapture" />
    <!-- 监控规则（内容层：匹配→转发/报警/修改/拦截；即改即存 + 运行中即时生效） -->
    <MonitorRulesDialog v-model="showMonitor" />
  </PageShell>
</template>

<style scoped>
/* TrafficMonitor 主体需 flex 撑满（流量列表区），覆盖 PageShell page-body 的滚动 */
:deep(.page-body) { display: flex; flex-direction: column; overflow: hidden; padding: 0; }
.cert-alert { margin: 0; border-radius: 0; }
.hint-alert { margin: 0; border-radius: 0; border-left: 0; border-right: 0; }
.hint-alert code { background: var(--jz-surface-2); padding: 2px 6px; border-radius: 3px; font-size: 11px; }

.main { flex: 1; display: flex; overflow: hidden; }
.list-pane { width: 40%; flex-shrink: 0; display: flex; flex-direction: column; border-right: 1px solid var(--jz-border); padding: 8px; gap: 6px; }
.filter { }
.count { font-size: 11px; color: var(--jz-text-dim); padding: 0 4px; }
.list { flex: 1; overflow-y: auto; }
/* 行：显式横向、绝不换行、左对齐 —— 防止历史/缓存里的 flex-wrap 让列竖排居中 */
.row { display: flex; flex-direction: row; flex-wrap: nowrap; justify-content: flex-start; align-items: center; gap: 8px; padding: 6px 8px; border-radius: 5px; cursor: pointer; font-size: 12px; text-align: left; min-width: 0; }
.row:hover { background: var(--el-fill-color); }
.row.sel { background: var(--el-color-primary-light-9); }
.row.sse { border-left: 2px solid #f59e0b; }
/* 标签/状态码/method 不收缩；host 可收缩、path 吃掉剩余空间，均单行省略 */
.m { flex: 0 0 auto; font-size: 10px; font-weight: 700; padding: 1px 5px; border-radius: 3px; background: #333; color: #ccc; min-width: 36px; text-align: center; }
.m.GET { background: #1e3a5f; color: var(--el-color-primary); }
.m.POST { background: #1e4d3a; color: #4ade80; }
.m.PUT { background: #4d3a1e; color: #fbbf24; }
.m.DELETE { background: #4d1e1e; color: #f87171; }
.host { flex: 0 1 auto; min-width: 0; max-width: 45%; color: #8ab4f8; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.path { color: var(--jz-text-dim); flex: 1 1 0; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
/* SSE/命中徽标：纯 span，绝不收缩、绝不换行，替代 el-tag 避免组件级样式意外撑行 */
.sse-badge, .hit-badge { flex: 0 0 auto; font-size: 10px; line-height: 16px; height: 16px; padding: 0 6px; border-radius: 8px; white-space: nowrap; }
.sse-badge { color: #fbbf24; background: rgba(245, 158, 11, 0.12); border: 1px solid rgba(245, 158, 11, 0.35); }
.hit-badge { color: #fff; }
.hit-badge.danger { background: #d9363e; }
.hit-badge.warn { background: #e6a23c; }
.hit-badge.info { background: #6b7280; }

.detail-pane { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.detail-head { display: flex; align-items: center; gap: 8px; padding: 10px 14px; border-bottom: 1px solid var(--jz-border); font-size: 13px; }
.url { flex: 1; color: #ddd; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.dur { font-size: 11px; color: var(--jz-text-dim); }
.tabs { flex: 1; overflow-y: auto; padding: 0 14px 14px; }
/* 请求/响应分 tab：tabs 撑满详情区、内容各自滚动 */
.detail-tabs { flex: 1; display: flex; flex-direction: column; overflow: hidden; padding: 0 14px 8px; }
.detail-tabs :deep(.el-tabs__content) { flex: 1; overflow-y: auto; }
.detail-tabs :deep(.el-tab-pane) { padding-bottom: 8px; }
/* headers 块：比 body 矮一些、字色偏暗，避免压掉 body 空间 */
.body.headers { max-height: 24vh; color: var(--jz-text-dim); }
.tab-section h4 { margin: 12px 0 6px; font-size: 12px; color: var(--jz-text-dim); font-weight: 600; display: flex; align-items: center; gap: 8px; }
.tab-head { display: flex; align-items: center; justify-content: space-between; }
.tab-head h4 { margin: 12px 0 6px; }
.body { background: #0f1116; border: 1px solid var(--jz-border); border-radius: 5px; padding: 10px; font-size: 11px; line-height: 1.5; overflow-x: auto; white-space: pre-wrap; word-break: break-word; max-height: 36vh; overflow-y: auto; color: var(--el-text-color-regular); font-family: "SF Mono", Menlo, monospace; }
.sse-actions { display: flex; align-items: center; gap: 8px; }
.sse { display: flex; flex-direction: column; gap: 4px; max-height: 36vh; overflow-y: auto; }
.sse-ev { background: #0f1116; border: 1px solid var(--jz-border); border-radius: 4px; padding: 6px 8px; }
.ev-data { font-size: 11px; color: var(--el-text-color-regular); white-space: pre-wrap; word-break: break-word; margin: 4px 0 0; font-family: "SF Mono", Menlo, monospace; }
</style>

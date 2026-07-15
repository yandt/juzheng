<script setup lang="ts">
import { computed, ref } from 'vue'
import { Plus, Delete, Top, Bottom } from '@element-plus/icons-vue'
import { useSbox } from '../composables/useSbox'
import { usePrefs } from '../composables/usePrefs'
import { t } from '../i18n'
import { DNS_REJECT, type DnsMatchType } from '../configModel'
import { removeDnsServer } from '../configOps'

const { config, sboxRunning, configDirty, saving, markDirty } = useSbox()
// 界面偏好（语言/主题）——纯前端偏好，localStorage 持久化，即选即生效，不进 sing-box 配置。
const { theme, language } = usePrefs()

// 用 computed 保持对 config.value.settings 的响应式引用（config 整体替换后仍拿最新）。
const s = computed(() => config.value.settings)

// detour 可选出口：默认(跟随路由) / 直连 / 走代理 / 各代理组。
const detourOptions = computed(() => [
  { label: t('detour.default'), value: '' },
  { label: t('detour.direct'), value: 'direct' },
  { label: t('detour.proxy'), value: 'proxy-node' },
  ...(config.value.groups ?? []).map(g => ({ label: t('detour.group', { tag: g.tag }), value: g.tag })),
])

// DNS 分流规则匹配类型
const dnsMatchTypes: DnsMatchType[] = ['domain_suffix', 'domain', 'domain_keyword', 'domain_regex', 'rule_set']
const dnsMatchLabel = (mt: DnsMatchType) => t('settings.mt_' + mt)

// 规则目标可选：各 DNS 服务器（按 tag 引用，展示地址）+ 拒绝解析(reject)
const dnsTargetOptions = computed(() => [
  ...s.value.dnsServers.filter(d => d.tag).map(d => ({ label: d.address || d.tag || '', value: d.tag || '' })),
  { label: t('settings.dnsReject'), value: DNS_REJECT },
])
// 兜底可选：仅 DNS 服务器（reject 作兜底无意义）
const dnsFinalOptions = computed(() => s.value.dnsServers.filter(d => d.tag).map(d => ({ label: d.address || d.tag || '', value: d.tag || '' })))

// 生成不冲突的 DNS 服务器 tag（dns.rules/final 按 tag 引用，须稳定唯一）
function genDnsTag(): string {
  const used = new Set(s.value.dnsServers.map(d => d.tag).filter(Boolean))
  let n = 0
  while (used.has(`dns-${n}`)) n++
  return `dns-${n}`
}

// 设置分类 Tab
const activeSettingsTab = ref('general')

// 即改即存
function onInput() { markDirty() }
function addDns() { s.value.dnsServers.push({ address: '', detour: 'direct', tag: genDnsTag() }); markDirty() }
function removeDns(i: number) {
  // 集中清引用：删 DNS 服务器前，先级联清除 dnsRules/dnsFinal/dnsRawRules 里对它的引用（configOps）。
  const tag = s.value.dnsServers[i]?.tag
  if (tag) removeDnsServer(config.value, tag)
  s.value.dnsServers.splice(i, 1); markDirty()
}

// DNS 分流规则增删与排序（顺序=优先级）
function addDnsRule() {
  s.value.dnsRules.push({ value: '', matchType: 'domain_suffix', server: s.value.dnsServers[0]?.tag || '' })
  markDirty()
}
function removeDnsRule(i: number) { s.value.dnsRules.splice(i, 1); markDirty() }
function moveDnsRule(i: number, dir: number) {
  const j = i + dir
  if (j < 0 || j >= s.value.dnsRules.length) return
  const arr = s.value.dnsRules
  const tmp = arr[i]; arr[i] = arr[j]; arr[j] = tmp
  markDirty()
}
</script>

<template>
  <PageShell :title="t('settings.title')">
    <template #title-extra>
      <el-tag v-if="sboxRunning" type="info" size="small">{{ t('settings.kernelRunningHint') }}</el-tag>
    </template>
    <template #actions>
      <SaveStatus :saving="saving" :dirty="configDirty" />
    </template>

    <el-tabs v-model="activeSettingsTab" class="page-tabs">
    <!-- 通用：界面（语言/主题）+ 日志 -->
    <el-tab-pane :label="t('settings.tabGeneral')" name="general">
    <el-form label-width="120px" label-position="right" class="settings-form">
      <!-- 界面：语言 + 主题（纯前端偏好，即选即生效） -->
      <el-divider content-position="left">{{ t('settings.ui') }}</el-divider>
      <el-form-item :label="t('settings.language')">
        <el-select v-model="language" class="set-ctrl">
          <el-option label="简体中文" value="zh" />
          <el-option label="English" value="en" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('settings.theme')">
        <el-radio-group v-model="theme" class="seg-full">
          <el-radio-button value="dark">{{ t('theme.dark') }}</el-radio-button>
          <el-radio-button value="light">{{ t('theme.light') }}</el-radio-button>
          <el-radio-button value="auto">{{ t('theme.auto') }}</el-radio-button>
        </el-radio-group>
      </el-form-item>

      <!-- TUN 接口设置已移至首页（TUN 服务卡片），此处不再重复 -->

      <el-divider content-position="left">{{ t('settings.log') }}</el-divider>
      <el-form-item :label="t('settings.logLevel')">
        <el-select v-model="s.logLevel" @change="onInput" class="set-ctrl">
          <el-option v-for="l in ['trace', 'debug', 'info', 'warn', 'error']" :key="l" :label="l" :value="l" />
        </el-select>
      </el-form-item>
    </el-form>
    </el-tab-pane>

    <!-- 网络与代理：复用首页组件，但用展开式布局（分区平铺、配置内联） -->
    <el-tab-pane :label="t('settings.tabNetwork')" name="network">
    <div class="net-tab">
      <el-divider content-position="left">{{ t('home.cardProxyMode') }}</el-divider>
      <ProxyModeControl />
      <NetworkControl expanded />

      <!-- 局域网共享：对外提供混合(HTTP+SOCKS)代理入站,供同网段其他设备使用 -->
      <el-divider content-position="left">{{ t('settings.lanShare') }}</el-divider>
      <el-form label-width="120px" label-position="right" class="settings-form">
        <el-form-item :label="t('settings.lanEnable')">
          <el-switch v-model="s.lanEnabled" @change="onInput" />
          <span class="hint">{{ t('settings.lanEnableHint') }}</span>
        </el-form-item>
        <template v-if="s.lanEnabled">
          <el-form-item :label="t('settings.lanPort')">
            <el-input-number v-model="s.lanPort" :min="1" :max="65535" :controls="false" class="lan-port" @change="onInput" />
            <span class="hint" v-html="t('settings.lanPortHint', { port: s.lanPort })"></span>
          </el-form-item>
          <el-form-item :label="t('settings.lanUser')">
            <el-input v-model="s.lanUsername" size="default" class="set-ctrl" :placeholder="t('settings.lanUserPh')" @change="onInput" />
          </el-form-item>
          <el-form-item :label="t('settings.lanPass')">
            <el-input v-model="s.lanPassword" size="default" type="password" show-password class="set-ctrl" :placeholder="t('settings.lanPassPh')" @change="onInput" />
          </el-form-item>
          <el-form-item label="">
            <el-alert type="warning" :closable="false" show-icon :title="t('settings.lanWarn')" />
          </el-form-item>
        </template>
      </el-form>
    </div>
    </el-tab-pane>

    <!-- DNS：解析策略 / 服务器 / 分流规则 / 兜底 -->
    <el-tab-pane :label="t('settings.dns')" name="dns">
    <el-form label-width="120px" label-position="right" class="settings-form">
      <el-alert type="info" :closable="false" show-icon class="dns-explain" :title="t('settings.dnsExplain')" />

      <el-form-item :label="t('settings.dnsStrategy')">
        <el-select v-model="s.dnsStrategy" @change="onInput" class="set-ctrl">
          <el-option :label="t('strategy.default')" value="" />
          <el-option :label="t('strategy.preferV4')" value="prefer_ipv4" />
          <el-option :label="t('strategy.preferV6')" value="prefer_ipv6" />
          <el-option :label="t('strategy.v4Only')" value="ipv4_only" />
          <el-option :label="t('strategy.v6Only')" value="ipv6_only" />
        </el-select>
        <span class="hint">{{ t('settings.dnsStrategyHint') }}</span>
      </el-form-item>

      <el-form-item :label="t('settings.dnsServers')">
        <div class="dns-editor">
          <div v-for="(d, i) in s.dnsServers" :key="i" class="dns-row">
            <el-select v-model="d.detour" size="small" class="dns-detour" @change="onInput">
              <el-option v-for="o in detourOptions" :key="o.value" :label="o.label" :value="o.value" />
            </el-select>
            <el-input v-model="d.address" size="small" class="dns-addr"
              placeholder="223.5.5.5 / https://1.1.1.1/dns-query / tls://8.8.8.8" @change="onInput" />
            <el-button text type="danger" size="small" @click="removeDns(i)"><el-icon><Delete /></el-icon></el-button>
          </div>
          <el-button size="small" plain type="primary" @click="addDns"><el-icon><Plus /></el-icon>&nbsp;{{ t('settings.dnsAdd') }}</el-button>
          <span v-if="s.dnsServers.length === 0" class="hint">{{ t('settings.dnsEmpty') }}</span>
        </div>
      </el-form-item>

      <!-- DNS 分流规则：某类域名 → 指定用哪台 DNS 解析。顺序=优先级，命中即停。 -->
      <el-form-item :label="t('settings.dnsRules')">
        <div class="dns-editor">
          <div v-for="(r, i) in s.dnsRules" :key="i" class="dns-rule-row">
            <el-select v-model="r.matchType" size="small" class="dns-mt" @change="onInput">
              <el-option v-for="m in dnsMatchTypes" :key="m" :label="dnsMatchLabel(m)" :value="m" />
            </el-select>
            <el-input v-model="r.value" size="small" class="dns-rule-val" :placeholder="t('settings.dnsRuleValuePh')" @change="onInput" />
            <span class="dns-arrow">→</span>
            <el-select v-model="r.server" size="small" class="dns-rule-server" :placeholder="t('settings.dnsUseServer')" @change="onInput">
              <el-option v-for="o in dnsTargetOptions" :key="o.value" :label="o.label" :value="o.value" />
            </el-select>
            <el-button text size="small" :disabled="i === 0" @click="moveDnsRule(i, -1)"><el-icon><Top /></el-icon></el-button>
            <el-button text size="small" :disabled="i === s.dnsRules.length - 1" @click="moveDnsRule(i, 1)"><el-icon><Bottom /></el-icon></el-button>
            <el-button text type="danger" size="small" @click="removeDnsRule(i)"><el-icon><Delete /></el-icon></el-button>
          </div>
          <el-button size="small" plain type="primary" @click="addDnsRule"><el-icon><Plus /></el-icon>&nbsp;{{ t('settings.dnsAddRule') }}</el-button>
          <span v-if="s.dnsRules.length === 0" class="hint">{{ t('settings.dnsRulesEmpty') }}</span>
          <span v-if="s.dnsRawRules.length > 0" class="hint">{{ t('settings.dnsAdvancedRules', { n: s.dnsRawRules.length }) }}</span>
        </div>
      </el-form-item>

      <!-- 兜底 DNS：无任何分流规则命中时使用（空=用列表第一台）。 -->
      <el-form-item :label="t('settings.dnsFinal')">
        <el-select v-model="s.dnsFinal" clearable :placeholder="t('settings.dnsFinalPh')" class="set-ctrl" @change="onInput">
          <el-option v-for="o in dnsFinalOptions" :key="o.value" :label="o.label" :value="o.value" />
        </el-select>
        <span class="hint">{{ t('settings.dnsFinalHint') }}</span>
      </el-form-item>

      <el-form-item label="">
        <div class="dns-tip">
          <div v-html="t('settings.dnsFormats')"></div>
          <div v-html="t('settings.dnsPairing')"></div>
        </div>
      </el-form-item>
    </el-form>
    </el-tab-pane>
    </el-tabs>
  </PageShell>
</template>

<style scoped>
:deep(.page-body) { display: flex; flex-direction: column; overflow: hidden; padding: 8px 16px 16px; }
/* 整个 tabs（含 tab 头 + 内容）统一限宽并水平居中 */
.page-tabs { margin-top: 0; flex: 1; display: flex; flex-direction: column; min-height: 0; align-self: center; width: 100%; max-width: 680px; }
.page-tabs :deep(.el-tabs__content) { padding-top: 4px; flex: 1; overflow-y: auto; min-height: 0; }
.page-tabs :deep(.el-tabs__header) { margin-bottom: 12px; flex: 0 0 auto; }
/* 内容填满已居中限宽的 tabs 列（限宽 + 居中在 .page-tabs 上统一处理） */
.settings-form, .net-tab { width: 100%; }
/* 统一：所有设置控件填满「控件列」（标签右侧至容器右缘），右缘对齐，各 tab 一致 */
.set-ctrl { width: 100%; }
.lan-port { width: 140px; }
.lan-port :deep(.el-input__inner) { text-align: left; font-family: monospace; }
/* 分段控件（主题）铺满控件列，与代理模式等一致 */
.seg-full { display: flex; width: 100%; }
.seg-full :deep(.el-radio-button) { flex: 1; }
.seg-full :deep(.el-radio-button__inner) { width: 100%; }
/* 提示语：控件已铺满整行，提示统一落到控件下方一行 */
.hint { display: inline-block; margin-top: 6px; color: var(--jz-text-dim); font-size: 12px; }
.dns-explain { margin-bottom: 12px; }
.dns-editor { display: flex; flex-direction: column; gap: 8px; width: 100%; }
.dns-row { display: flex; align-items: center; gap: 8px; }
.dns-addr { flex: 1; }
.dns-addr :deep(.el-input__inner) { font-family: monospace; }
.dns-detour { width: 160px; flex-shrink: 0; }
.dns-rule-row { display: flex; align-items: center; gap: 6px; }
.dns-mt { width: 120px; flex-shrink: 0; }
.dns-rule-val { flex: 1; min-width: 150px; }
.dns-rule-val :deep(.el-input__inner) { font-family: monospace; }
.dns-arrow { color: var(--jz-text-dim); flex-shrink: 0; }
.dns-rule-server { width: 150px; flex-shrink: 0; }
.dns-tip { font-size: 12px; color: var(--jz-text-dim); line-height: 1.7; }
.dns-tip code { background: var(--jz-border); padding: 1px 5px; border-radius: 3px; font-family: monospace; color: var(--jz-text-dim); }
</style>

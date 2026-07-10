<script setup lang="ts">
// 网络设置控件：虚拟网卡(TUN) / 系统代理 / MITM 三组开关 + 配置。
// 自包含（状态取自单例 composable），首页卡片与设置页共用同一份逻辑与弹窗。
// - 默认（compact）：首页用，tab 分组，省空间。
// - expanded：设置页用，三块分区平铺展开。
// 两种布局的「配置」都点齿轮弹出同一批弹窗（复用），不内联表单。
import { ref, onMounted } from 'vue'
import { Setting } from '@element-plus/icons-vue'
import { useSbox } from '../composables/useSbox'
import { useMitm } from '../composables/useMitm'
import { t } from '../i18n'

withDefaults(defineProps<{ expanded?: boolean }>(), { expanded: false })

const {
  sboxRunning, sboxBusy, helperStatus, helperBusy, startSbox, stopSbox,
  config, markDirty, installHelper, uninstallHelper,
  sysProxyEnabled, sysProxyBusy, sysProxyState, toggleSysProxy, applySysProxySettings, refreshSysProxy,
} = useSbox()
const { running: mitmRunning, toggleProxy } = useMitm()

const netTab = ref('tun')
const tunStacks = ['system', 'gvisor', 'mixed']
const showTunSettings = ref(false)
const showSysProxySettings = ref(false)
const bypassText = ref('')       // bypass 编辑（textarea，每行一个）
const showMitmSettings = ref(false)

// 初始化系统代理实际状态（首页与设置页都会挂载本组件，各自刷新一次；单例状态共享）
onMounted(() => refreshSysProxy())

function openSysProxySettings() {
  bypassText.value = (sysProxyState.value.bypass ?? []).join('\n')
  showSysProxySettings.value = true
}
async function saveSysProxySettings() {
  sysProxyState.value.bypass = bypassText.value.split('\n').map(s => s.trim()).filter(s => s)
  if (sysProxyState.value.enabled) {
    sysProxyState.value.enabled = true
    await applySysProxySettings()
  }
  showSysProxySettings.value = false
}
async function toggleMitm() { await toggleProxy() }
// TUN 服务开关：已安装则卸载，未安装则安装
async function toggleHelper() {
  if (helperStatus.value.installed) await uninstallHelper()
  else await installHelper()
}
</script>

<template>
  <div class="network-control">
    <!-- ===== 展开式（设置页）：分区平铺（el-divider 标题，与其它 tab 统一），配置仍点齿轮开弹窗 ===== -->
    <template v-if="expanded">
      <el-divider content-position="left">{{ t('home.tabTun') }}</el-divider>
      <div class="toggle-row">
        <div class="toggle-label">
          <span>{{ t('home.nicMode') }}</span>
          <span class="toggle-sub">{{ sboxRunning ? t('home.takingOver') : t('home.stopped') }}</span>
        </div>
        <el-switch :model-value="sboxRunning" :loading="sboxBusy" :disabled="!helperStatus.running || helperBusy"
          @change="(v: string | number | boolean) => v ? startSbox() : stopSbox()" />
      </div>
      <div class="toggle-row sub">
        <div class="toggle-label">
          <span>{{ t('home.tunService') }}</span>
          <span class="toggle-sub">
            {{ helperBusy ? t('home.processing') : (helperStatus.installed ? (helperStatus.running ? t('home.runningPid', { pid: helperStatus.pid }) : t('home.installedNotRunning')) : t('home.notInstalled')) }}
          </span>
        </div>
        <div class="toggle-right">
          <el-icon v-if="helperStatus.installed" class="tun-set-icon" @click="showTunSettings = true" :title="t('home.tunSettings')"><Setting /></el-icon>
          <el-switch :model-value="helperStatus.installed" :loading="helperBusy" :disabled="helperBusy" @change="() => toggleHelper()" />
        </div>
      </div>
      <div v-if="!helperStatus.running" class="dep-hint">{{ t('home.tunDepHint') }}</div>

      <el-divider content-position="left">{{ t('home.sysProxy') }}</el-divider>
      <div class="toggle-row">
        <div class="toggle-label">
          <span>{{ t('home.sysProxy') }}</span>
          <span class="toggle-sub">{{ sysProxyBusy ? t('home.processing') : (sysProxyEnabled ? t('home.sysProxyOn') : t('home.sysProxyOff')) }}</span>
        </div>
        <div class="toggle-right">
          <el-icon class="tun-set-icon" @click="openSysProxySettings" :title="t('home.sysProxySettings')"><Setting /></el-icon>
          <el-switch :model-value="sysProxyEnabled" :loading="sysProxyBusy" :disabled="sysProxyBusy"
            @change="(v: string | number | boolean) => toggleSysProxy(!!v)" />
        </div>
      </div>

      <el-divider content-position="left">{{ t('home.mitmDecrypt') }}</el-divider>
      <div class="toggle-row">
        <div class="toggle-label">
          <span>{{ t('home.mitmDecrypt') }}</span>
          <span class="toggle-sub">{{ mitmRunning ? t('home.running') : t('home.stopped') }}</span>
        </div>
        <div class="toggle-right">
          <el-icon class="tun-set-icon" @click="showMitmSettings = true" :title="t('home.mitmConfig')"><Setting /></el-icon>
          <el-switch :model-value="mitmRunning" @change="() => toggleMitm()" />
        </div>
      </div>
    </template>

    <!-- ===== 紧凑式（首页卡片）：tab 分组 ===== -->
    <template v-else>
      <el-tabs v-model="netTab" class="net-tabs">
        <el-tab-pane :label="t('home.tabTun')" name="tun">
          <div class="toggle-row">
            <div class="toggle-label">
              <span>{{ t('home.nicMode') }}</span>
              <span class="toggle-sub">{{ sboxRunning ? t('home.takingOver') : t('home.stopped') }}</span>
            </div>
            <el-switch :model-value="sboxRunning" :loading="sboxBusy" :disabled="!helperStatus.running || helperBusy"
              @change="(v: string | number | boolean) => v ? startSbox() : stopSbox()" />
          </div>
          <div class="toggle-row sub">
            <div class="toggle-label">
              <span>{{ t('home.tunService') }}</span>
              <span class="toggle-sub">
                {{ helperBusy ? t('home.processing') : (helperStatus.installed ? (helperStatus.running ? t('home.runningPid', { pid: helperStatus.pid }) : t('home.installedNotRunning')) : t('home.notInstalled')) }}
              </span>
            </div>
            <div class="toggle-right">
              <el-icon v-if="helperStatus.installed" class="tun-set-icon" @click="showTunSettings = true" :title="t('home.tunSettings')"><Setting /></el-icon>
              <el-switch :model-value="helperStatus.installed" :loading="helperBusy" :disabled="helperBusy" @change="() => toggleHelper()" />
            </div>
          </div>
          <div v-if="!helperStatus.running" class="dep-hint">{{ t('home.tunDepHint') }}</div>
        </el-tab-pane>

        <el-tab-pane :label="t('home.tabSysProxy')" name="sysproxy">
          <div class="toggle-row">
            <div class="toggle-label">
              <span>{{ t('home.sysProxy') }}</span>
              <span class="toggle-sub">{{ sysProxyBusy ? t('home.processing') : (sysProxyEnabled ? t('home.sysProxyOn') : t('home.sysProxyOff')) }}</span>
            </div>
            <div class="toggle-right">
              <el-icon class="tun-set-icon" @click="openSysProxySettings" :title="t('home.sysProxySettings')"><Setting /></el-icon>
              <el-switch :model-value="sysProxyEnabled" :loading="sysProxyBusy" :disabled="sysProxyBusy"
                @change="(v: string | number | boolean) => toggleSysProxy(!!v)" />
            </div>
          </div>
        </el-tab-pane>

        <el-tab-pane :label="t('home.tabMitm')" name="mitm">
          <div class="toggle-row">
            <div class="toggle-label">
              <span>{{ t('home.mitmDecrypt') }}</span>
              <span class="toggle-sub">{{ mitmRunning ? t('home.running') : t('home.stopped') }}</span>
            </div>
            <div class="toggle-right">
              <el-icon class="tun-set-icon" @click="showMitmSettings = true" :title="t('home.mitmConfig')"><Setting /></el-icon>
              <el-switch :model-value="mitmRunning" @change="() => toggleMitm()" />
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>
    </template>

    <!-- ===== 共用弹窗（compact / expanded 都点齿轮打开同一批） ===== -->
    <!-- TUN 设置弹窗 -->
    <el-dialog v-model="showTunSettings" :title="t('home.tunSettingsTitle')" width="460px">
      <el-alert type="info" :closable="false" show-icon style="margin-bottom: 16px">{{ t('home.tunSettingsAlert') }}</el-alert>
      <el-form label-width="120px" label-position="right">
        <el-form-item :label="t('home.stackMode')">
          <el-select v-model="config.settings.tunStack" @change="markDirty()" style="width: 100%">
            <el-option v-for="s in tunStacks" :key="s" :label="s + (s === 'system' ? t('home.stackSystem') : s === 'gvisor' ? t('home.stackGvisor') : t('home.stackMixed'))" :value="s" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('home.tunInterfaceName')">
          <el-input v-model="config.settings.tunInterfaceName" :placeholder="t('home.tunInterfacePlaceholder')" @input="markDirty()" />
        </el-form-item>
        <el-form-item :label="t('home.tunAddress')">
          <el-input v-model="config.settings.tunAddress" placeholder="172.18.0.1/30" @input="markDirty()" />
        </el-form-item>
        <el-form-item label="MTU">
          <el-input-number v-model="config.settings.tunMtu" :min="576" :max="9000" @change="markDirty()" />
        </el-form-item>
        <el-form-item :label="t('home.tunAutoRoute')">
          <el-switch v-model="config.settings.tunAutoRoute" @change="markDirty()" />
          <span class="set-hint">{{ t('home.tunAutoRouteHint') }}</span>
        </el-form-item>
        <el-form-item :label="t('home.tunStrictRoute')">
          <el-switch v-model="config.settings.tunStrictRoute" @change="markDirty()" />
          <span class="set-hint">{{ t('home.tunStrictRouteHint') }}</span>
        </el-form-item>
      </el-form>
      <template #footer><el-button type="primary" @click="showTunSettings = false">{{ t('home.done') }}</el-button></template>
    </el-dialog>

    <!-- 系统代理设置弹窗 -->
    <el-dialog v-model="showSysProxySettings" :title="t('home.sysProxySettingsTitle')" width="480px">
      <el-alert type="info" :closable="false" show-icon style="margin-bottom: 16px">{{ t('home.sysProxyAlert') }}</el-alert>
      <el-form label-width="110px" label-position="right">
        <el-form-item :label="t('home.proxyPort')">
          <el-input-number v-model="config.settings.mixedBackPort" :min="1024" :max="65535" @change="markDirty()" />
          <span class="set-hint">{{ t('home.proxyPortHint') }}</span>
        </el-form-item>
        <el-divider class="ctrl-divider">{{ t('home.enableTypes') }}</el-divider>
        <el-form-item :label="t('home.httpProxy')"><el-switch v-model="sysProxyState.enableHttp" /></el-form-item>
        <el-form-item :label="t('home.httpsProxy')"><el-switch v-model="sysProxyState.enableHttps" /></el-form-item>
        <el-form-item :label="t('home.socksProxy')"><el-switch v-model="sysProxyState.enableSocks" /></el-form-item>
        <el-divider class="ctrl-divider">{{ t('home.bypassList') }}</el-divider>
        <el-form-item :label="t('home.bypassDomains')">
          <el-input v-model="bypassText" type="textarea" :rows="5" :placeholder="t('home.bypassPlaceholder')" />
          <div class="set-hint">{{ t('home.bypassHint') }}</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showSysProxySettings = false">{{ t('home.cancel') }}</el-button>
        <el-button type="primary" @click="saveSysProxySettings">{{ t('home.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- 抓包域名配置弹窗（复用组件，流量监控页同款） -->
    <CaptureDomainsDialog v-model="showMitmSettings" />
  </div>
</template>

<style scoped>
/* 紧凑（首页）tab */
.net-tabs { margin-top: 8px; }
.net-tabs :deep(.el-tabs__header) { margin: 0 0 8px; }
.net-tabs :deep(.el-tabs__nav-wrap::after) { height: 1px; }
.net-tabs :deep(.el-tabs__item) { height: 32px; font-size: 13px; }
/* 通用行 */
.toggle-row { display: flex; align-items: center; justify-content: space-between; font-size: 13px; color: var(--el-text-color-regular); gap: 12px; margin-bottom: 10px; }
.toggle-row.sub { padding-left: 18px; }
.toggle-row.sub .toggle-label > span:first-child { font-size: 12px; color: var(--jz-text-dim); }
.dep-hint { font-size: 11px; color: #fbbf24; padding-left: 18px; margin-top: -2px; margin-bottom: 8px; }
.tun-set-icon { cursor: pointer; color: var(--jz-text-dim); font-size: 14px; margin-right: 8px; transition: color .15s; }
.tun-set-icon:hover { color: var(--el-color-primary); }
.toggle-right { display: flex; align-items: center; }
.toggle-label { display: flex; flex-direction: column; gap: 2px; }
.toggle-label > span:first-child { font-size: 13px; color: var(--jz-text); }
.toggle-sub { font-size: 11px; color: var(--jz-text-dim); }
.set-hint { margin-left: 12px; color: var(--jz-text-dim); font-size: 11px; }
.ctrl-divider { margin: 6px 0; }
</style>

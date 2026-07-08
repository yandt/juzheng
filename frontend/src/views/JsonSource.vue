<script setup lang="ts">
import { ref, computed, watch } from 'vue'
// vue-codemirror 的 dts 不完整，用 // @ts-ignore 绕过类型检查
// @ts-ignore
import { Codemirror } from 'vue-codemirror'
import type { Extension } from '@codemirror/state'
import { json } from '@codemirror/lang-json'
import { oneDark } from '@codemirror/theme-one-dark'
import { useSbox } from '../composables/useSbox'
import { usePrefs } from '../composables/usePrefs'
import { t } from '../i18n'
import * as singbox from '../../bindings/github.com/zhanghui/juzheng/singboxservice'

const { configJson, configDirty, configError, saving, sboxRunning, syncFromJson } = useSbox()
const { isDark } = usePrefs()

// 代码主题跟随应用主题：深色用 oneDark，浅色用 Codemirror 默认亮色主题。
const extensions = computed<Extension[]>(() => isDark.value ? [json(), oneDark as Extension] : [json()])
const view = ref()

// 视图模式：edit=可编辑源码；final=最终供 sing-box 运行的配置（只读，经 Sanitize 清洗）。
const mode = ref<'edit' | 'final'>('edit')
const finalJson = ref('')
async function refreshFinal() {
  try {
    finalJson.value = await singbox.SanitizeConfig(configJson.value) as unknown as string
  } catch (e: any) {
    finalJson.value = '// ' + t('source.genFinalFailed') + (e?.message || e)
  }
}
// 进入 final 模式、或 final 模式下源码变化时，重算最终配置。
watch(mode, m => { if (m === 'final') refreshFinal() })
watch(configJson, () => { if (mode.value === 'final') refreshFinal() })

// 即改即存：编辑 JSON 即同步解析，解析成功自动保存（语法错误时不写盘）。
function onChange() {
  syncFromJson()
}
</script>

<template>
  <PageShell :title="t('source.title')">
    <template #title-extra>
      <el-radio-group v-model="mode" size="small">
        <el-radio-button value="edit">{{ t('source.modeEdit') }}</el-radio-button>
        <el-radio-button value="final">{{ t('source.modeFinal') }}</el-radio-button>
      </el-radio-group>
      <el-tag v-if="mode === 'edit'" :type="configDirty ? 'warning' : 'info'" size="small">
        {{ configDirty ? t('source.unsaved') : t('source.saved') }}
      </el-tag>
    </template>
    <template #actions>
      <SaveStatus v-if="mode === 'edit'" :saving="saving" :dirty="configDirty" />
    </template>

    <el-alert
      v-if="mode === 'edit' && configError"
      type="error" :closable="false" show-icon class="err-alert"
      :title="configError"
    />
    <el-alert
      v-else-if="mode === 'final'"
      type="success" :closable="false" show-icon class="err-alert"
      :title="t('source.finalHint')"
    />

    <!-- 编辑模式：可编辑源码。:key 含主题，切主题时重建编辑器以应用新代码主题。 -->
    <div v-show="mode === 'edit'" class="editor-wrap">
      <Codemirror
        :key="'edit-' + (isDark ? 'd' : 'l')"
        v-model="configJson"
        :placeholder="t('source.placeholder')"
        :extensions="extensions"
        :disabled="sboxRunning"
        :style="{ height: '100%', fontSize: '12px' }"
        @change="onChange"
      />
    </div>
    <!-- 最终配置模式：只读展示 sing-box 实际运行的配置 -->
    <div v-show="mode === 'final'" class="editor-wrap">
      <Codemirror
        :key="'final-' + (isDark ? 'd' : 'l')"
        v-model="finalJson"
        :extensions="extensions"
        :disabled="true"
        :style="{ height: '100%', fontSize: '12px' }"
      />
    </div>

    <div v-if="mode === 'edit' && sboxRunning" class="readonly-hint">{{ t('source.readonlyHint') }}</div>
  </PageShell>
</template>

<style scoped>
:deep(.page-body) { display: flex; flex-direction: column; overflow: hidden; padding: 8px 16px 16px; }
.err-alert { margin-bottom: 8px; flex: 0 0 auto; }
.editor-wrap { flex: 1; border: 1px solid var(--jz-border); border-radius: 6px; overflow: hidden; background: var(--jz-surface); min-height: 300px; }
.editor-wrap :deep(.cm-editor) { height: 100%; }
.readonly-hint { color: #fbbf24; font-size: 12px; margin-top: 8px; flex: 0 0 auto; }
</style>

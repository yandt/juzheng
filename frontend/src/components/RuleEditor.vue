<script setup lang="ts">
// RuleEditor：路由规则编辑器（v-model 双向绑定 rules 数组）。
// 每种匹配类型只允许出现一组，添加时自动选未被使用的类型，下拉里已选类型禁用。
import { ref, computed, watch } from 'vue'
import { Plus, Delete } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { t } from '../i18n'
import type { GroupRule, RuleMatchType } from '../configModel'

const props = defineProps<{
  modelValue?: GroupRule[]
  tag: string // 组名/节点名（用于展示 outbound 提示）
}>()
const emit = defineEmits<{ (e: 'update:modelValue', v: GroupRule[]): void }>()

// 匹配类型选项（computed 保证切换语言时 label 响应式更新）
const matchTypeOptions = computed<{ value: RuleMatchType; label: string }[]>(() => [
  { value: 'domain_suffix', label: t('comp.matchDomainSuffix') },
  { value: 'domain_keyword', label: t('comp.matchDomainKeyword') },
  { value: 'domain', label: t('comp.matchDomain') },
  { value: 'domain_regex', label: t('comp.matchDomainRegex') },
  { value: 'ip_cidr', label: t('comp.matchIpCidr') },
  // 注意：geosite/geoip 在 sing-box 1.12+ 已移除内置数据库，会被 sanitize 整条丢弃 → 规则静默失效（漏抓）。
  // 故不再作为可选匹配类型。按地区分流请改用 rule_set。
  { value: 'protocol', label: t('comp.matchProtocol') },
  // 按发起进程匹配（依赖 route.find_process,序列化时已强制开启）。
  { value: 'process_name', label: t('comp.matchProcessName') },
  { value: 'process_path', label: t('comp.matchProcessPath') },
  { value: 'process_path_regex', label: t('comp.matchProcessPathRegex') },
])

// textarea 文本镜像（每条规则一个，values.join('\n')）
const ruleTexts = ref<string[]>([])

// 标记：本组件自己发出的 modelValue 更新，不回灌 ruleTexts。
// 否则输入时 values 经 filter(Boolean) 去掉空行后 join('\n') 会把刚敲的换行/空行抹掉，导致「打不出回车」。
let selfUpdate = false

// 同步外部 modelValue → ruleTexts（仅弹窗打开/外部数据变化时；自身编辑产生的更新跳过，保留原始文本含换行/空行）
watch(() => props.modelValue, (rules) => {
  if (selfUpdate) { selfUpdate = false; return }
  ruleTexts.value = (rules ?? []).map(r => r.values.join('\n'))
}, { immediate: true })

// 已使用的匹配类型集合（用于禁用重复）
const usedTypes = computed(() => new Set((props.modelValue ?? []).map(r => r.matchType)))

// 是否还能添加规则（还有未使用的类型）
const canAddRule = computed(() => usedTypes.value.size < matchTypeOptions.value.length)

function addRule() {
  if (!canAddRule.value) {
    ElMessage.warning(t('comp.allTypesUsed'))
    return
  }
  // 自动选第一个未被使用的类型
  const nextType = matchTypeOptions.value.find(o => !usedTypes.value.has(o.value))?.value ?? 'domain_suffix'
  const newRules = [...(props.modelValue ?? []), { matchType: nextType, values: [] as string[] }]
  ruleTexts.value.push('')
  selfUpdate = true
  emit('update:modelValue', newRules)
}

function removeRule(idx: number) {
  const newRules = [...(props.modelValue ?? [])]
  newRules.splice(idx, 1)
  ruleTexts.value.splice(idx, 1)
  selfUpdate = true
  emit('update:modelValue', newRules)
}

function onTextInput(idx: number, text: string) {
  ruleTexts.value[idx] = text  // 保留原始文本（含换行/空行），textarea 正常换行
  if (props.modelValue && props.modelValue[idx]) {
    const newRules = [...props.modelValue]
    // 出参仍做 trim + 去空行（配置只要干净值），但不回灌 textarea
    newRules[idx] = { ...newRules[idx], values: text.split('\n').map(s => s.trim()).filter(Boolean) }
    selfUpdate = true
    emit('update:modelValue', newRules)
  }
}
</script>

<template>
  <div class="rules-section">
    <div class="rules-head">
      <el-button size="small" type="primary" plain :disabled="!canAddRule" @click="addRule">
        <el-icon><Plus /></el-icon>&nbsp;{{ t('comp.addRule') }}
      </el-button>
      <span class="rules-hint">{{ t('comp.rulesHint', { n: (modelValue ?? []).length }) }}</span>
    </div>
    <div v-if="(modelValue ?? []).length === 0" class="rules-empty">
      {{ t('comp.rulesEmpty') }}
    </div>
    <div v-for="(rule, idx) in (modelValue ?? [])" :key="idx" class="rule-card">
      <div class="rule-card-head">
        <el-select v-model="rule.matchType" size="small" style="width: 240px">
          <el-option
            v-for="o in matchTypeOptions"
            :key="o.value"
            :label="o.label"
            :value="o.value"
            :disabled="usedTypes.has(o.value) && rule.matchType !== o.value"
          />
        </el-select>
        <el-button text size="small" type="danger" @click="removeRule(idx)">
          <el-icon><Delete /></el-icon>
        </el-button>
      </div>
      <el-input
        type="textarea"
        :rows="5"
        :model-value="ruleTexts[idx] ?? ''"
        @update:model-value="(v: string) => onTextInput(idx, v)"
        :placeholder="rule.matchType === 'ip_cidr' ? t('comp.placeholderIpCidr') : t('comp.placeholderDomain')"
        class="rule-textarea"
      />
      <div class="rule-meta">{{ t('comp.ruleMeta', { n: (rule.values ?? []).length, tag }) }}</div>
    </div>
  </div>
</template>

<style scoped>
.rules-section { width: 100%; }
.rules-head { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; }
.rules-hint { font-size: 11px; color: var(--jz-text-dim); }
.rules-empty { font-size: 12px; color: var(--jz-text-dim); padding: 12px; text-align: center; background: var(--jz-surface); border: 1px dashed var(--jz-border); border-radius: 4px; }
.rule-card { border: 1px solid var(--jz-border); border-radius: 6px; padding: 10px; margin-bottom: 8px; background: var(--jz-surface); }
.rule-card-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
.rule-textarea :deep(.el-textarea__inner) { font-family: monospace; font-size: 12px; }
.rule-meta { font-size: 10px; color: var(--jz-text-dim); margin-top: 4px; text-align: right; }
</style>

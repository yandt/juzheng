<script setup lang="ts">
// 监控规则管理弹窗：左侧规则列表 + 右侧规则编辑。即改即存 + 热更新。
// 规则名必填（空名在列表标红、编辑区提示）。
import { computed, ref, watch } from 'vue'
import { Plus, Delete, Warning } from '@element-plus/icons-vue'
import { useMonitor } from '../composables/useMonitor'
import { newMonitorRule, type MonitorRule } from '../types/monitor'
import { t } from '../i18n'

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void }>()
const visible = computed({ get: () => props.modelValue, set: (v: boolean) => emit('update:modelValue', v) })

const { rules, scheduleSave } = useMonitor()

const fields = ['host', 'url', 'path', 'reqBody', 'respBody'] as const
const ops = ['contains', 'equals', 'suffix', 'regex'] as const
const phases = ['request', 'response', 'both'] as const
const actions = ['forward', 'alert', 'modify', 'block'] as const
const levels = ['info', 'warn', 'danger'] as const
const targets = ['body', 'header', 'status', 'url'] as const
const bodyOps = ['replace', 'regexReplace', 'setFull'] as const

const sel = ref(0)
const current = computed<MonitorRule | null>(() => rules.value[sel.value] ?? null)
// 打开时纠正选中项
watch(visible, (v) => { if (v && sel.value >= rules.value.length) sel.value = Math.max(0, rules.value.length - 1) })

const changed = () => scheduleSave()
const nameEmpty = (r: MonitorRule) => !r.name.trim()

function addRule() { rules.value.push(newMonitorRule()); sel.value = rules.value.length - 1; changed() }
function removeRule(i: number) {
  rules.value.splice(i, 1)
  if (sel.value >= rules.value.length) sel.value = rules.value.length - 1
  changed()
}
function addCond(r: MonitorRule) { r.conditions.push({ field: 'host', op: 'contains', value: '' }); changed() }
function removeCond(r: MonitorRule, i: number) { r.conditions.splice(i, 1); changed() }

function actionTagType(a: string): 'info' | 'warning' | 'danger' | 'success' {
  return a === 'block' ? 'danger' : a === 'modify' ? 'warning' : a === 'forward' ? 'success' : 'info'
}
</script>

<template>
  <el-dialog v-model="visible" :title="t('monitor.title')" width="860px" top="6vh">
    <el-alert type="warning" :closable="false" show-icon class="mon-alert">{{ t('monitor.alert') }}</el-alert>

    <div class="mon-body">
      <!-- 左：规则列表 -->
      <div class="mon-list">
        <el-button plain type="primary" size="small" class="add-rule" @click="addRule">
          <el-icon><Plus /></el-icon>&nbsp;{{ t('monitor.addRule') }}
        </el-button>
        <div class="list-scroll">
          <div
            v-for="(r, i) in rules"
            :key="r.id"
            class="list-item"
            :class="{ active: i === sel, off: !r.enabled }"
            @click="sel = i"
          >
            <el-switch v-model="r.enabled" size="small" @change="changed" @click.stop />
            <el-icon v-if="nameEmpty(r)" class="warn-ic" :title="t('monitor.nameRequired')"><Warning /></el-icon>
            <span class="li-name" :class="{ unnamed: nameEmpty(r) }">{{ r.name.trim() || t('monitor.unnamed') }}</span>
            <el-tag size="small" :type="actionTagType(r.action)" effect="plain" class="li-act">{{ t('monitor.action_' + r.action) }}</el-tag>
            <el-icon class="del-ic" @click.stop="removeRule(i)"><Delete /></el-icon>
          </div>
          <el-empty v-if="rules.length === 0" :description="t('monitor.empty')" :image-size="46" />
        </div>
      </div>

      <!-- 右：规则编辑 -->
      <div class="mon-editor">
        <template v-if="current">
          <!-- 名称（必填） -->
          <div class="ed-field">
            <span class="lbl req">{{ t('monitor.name') }}</span>
            <div class="ed-ctrl">
              <el-input v-model="current.name" size="small" :placeholder="t('monitor.namePh')" :class="{ 'is-invalid': nameEmpty(current) }" @input="changed" />
              <div v-if="nameEmpty(current)" class="err-tip">{{ t('monitor.nameRequired') }}</div>
            </div>
          </div>

          <div class="ed-field">
            <span class="lbl">{{ t('monitor.phase') }}</span>
            <el-select v-model="current.phase" size="small" style="width: 130px" @change="changed">
              <el-option v-for="p in phases" :key="p" :label="t('monitor.phase_' + p)" :value="p" />
            </el-select>
            <span class="lbl" style="margin-left:12px">{{ t('monitor.action') }}</span>
            <el-select v-model="current.action" size="small" style="width: 130px" @change="changed">
              <el-option v-for="a in actions" :key="a" :label="t('monitor.action_' + a)" :value="a" />
            </el-select>
          </div>

          <!-- 二级：匹配条件（分组缩进，明确归属） -->
          <div class="group">
            <div class="group-head">
              <span class="group-title">{{ t('monitor.conditions') }}</span>
              <el-radio-group v-model="current.logic" size="small" @change="changed">
                <el-radio-button value="and">{{ t('monitor.and') }}</el-radio-button>
                <el-radio-button value="or">{{ t('monitor.or') }}</el-radio-button>
              </el-radio-group>
            </div>
            <div class="group-body">
              <div v-for="(c, ci) in current.conditions" :key="ci" class="cond">
                <el-select v-model="c.field" size="small" style="width: 110px" @change="changed">
                  <el-option v-for="f in fields" :key="f" :label="t('monitor.field_' + f)" :value="f" />
                </el-select>
                <el-select v-model="c.op" size="small" style="width: 96px" @change="changed">
                  <el-option v-for="o in ops" :key="o" :label="t('monitor.op_' + o)" :value="o" />
                </el-select>
                <el-input v-model="c.value" size="small" class="cond-val" :placeholder="t('monitor.valuePh')" @input="changed" />
                <el-icon class="del-ic" v-if="current.conditions.length > 1" @click="removeCond(current, ci)"><Delete /></el-icon>
                <el-icon class="add-ic" @click="addCond(current)"><Plus /></el-icon>
              </div>
            </div>
          </div>

          <!-- 二级：动作参数（分组缩进；转发无参数） -->
          <div class="group">
            <div class="group-head">
              <span class="group-title">{{ t('monitor.actionConfig') }}</span>
              <el-tag size="small" :type="actionTagType(current.action)" effect="plain">{{ t('monitor.action_' + current.action) }}</el-tag>
            </div>
            <div class="group-body">
              <div v-if="current.action === 'forward'" class="note">{{ t('monitor.forwardNote') }}</div>

              <div v-else-if="current.action === 'alert'" class="cond">
                <span class="lbl">{{ t('monitor.level') }}</span>
                <el-select v-model="current.alertLevel" size="small" style="width: 120px" @change="changed">
                  <el-option v-for="l in levels" :key="l" :label="t('monitor.level_' + l)" :value="l" />
                </el-select>
              </div>

              <div v-else-if="current.action === 'block' && current.block" class="cond">
                <span class="lbl">{{ t('monitor.blockStatus') }}</span>
                <el-input-number v-model="current.block.status" size="small" :min="0" :max="599" controls-position="right" style="width: 110px" @change="changed" />
                <el-input v-model="current.block.body" size="small" :placeholder="t('monitor.blockBodyPh')" style="flex:1" @input="changed" />
              </div>

              <template v-else-if="current.action === 'modify' && current.modify">
                <div class="cond">
                  <span class="lbl">{{ t('monitor.modifyTarget') }}</span>
                  <el-select v-model="current.modify.target" size="small" style="width: 120px" @change="changed">
                    <el-option v-for="tg in targets" :key="tg" :label="t('monitor.target_' + tg)" :value="tg" />
                  </el-select>
                </div>
                <div v-if="current.modify.target === 'body'" class="cond">
                  <el-select v-model="current.modify.bodyOp" size="small" style="width: 120px" @change="changed">
                    <el-option v-for="b in bodyOps" :key="b" :label="t('monitor.bodyOp_' + b)" :value="b" />
                  </el-select>
                  <el-input v-if="current.modify.bodyOp !== 'setFull'" v-model="current.modify.find" size="small" :placeholder="t('monitor.findPh')" style="flex:1" @input="changed" />
                  <el-input v-model="current.modify.replace" size="small" :placeholder="t('monitor.replacePh')" style="flex:1" @input="changed" />
                </div>
                <div v-else-if="current.modify.target === 'header'" class="cond">
                  <el-input v-model="current.modify.headerName" size="small" placeholder="Header" style="width: 180px" @input="changed" />
                  <el-input v-model="current.modify.headerValue" size="small" :placeholder="t('monitor.headerValPh')" style="flex:1" @input="changed" />
                </div>
                <div v-else-if="current.modify.target === 'status'" class="cond">
                  <el-input-number v-model="current.modify.status" size="small" :min="0" :max="599" controls-position="right" @change="changed" />
                </div>
                <div v-else-if="current.modify.target === 'url'" class="cond">
                  <el-input v-model="current.modify.url" size="small" placeholder="https://…" style="flex:1" @input="changed" />
                </div>
              </template>
            </div>
          </div>
        </template>

        <el-empty v-else :description="t('monitor.selectRule')" :image-size="60" />
      </div>
    </div>

    <template #footer>
      <span class="foot-hint">{{ t('monitor.savedHint') }}</span>
      <el-button type="primary" @click="visible = false">{{ t('common.done') }}</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.mon-alert { margin-bottom: 12px; }
.mon-body { display: flex; gap: 12px; height: 56vh; }
/* 左列表 */
.mon-list { width: 240px; flex-shrink: 0; display: flex; flex-direction: column; gap: 8px; border-right: 1px solid var(--jz-border); padding-right: 12px; }
.add-rule { align-self: stretch; }
.list-scroll { flex: 1; overflow-y: auto; display: flex; flex-direction: column; gap: 4px; }
.list-item { display: flex; align-items: center; gap: 6px; padding: 6px 8px; border-radius: 6px; cursor: pointer; }
.list-item:hover { background: var(--el-fill-color-light); }
.list-item.active { background: var(--el-color-primary-light-9); }
.list-item.off { opacity: .55; }
.li-name { flex: 1; font-size: 13px; color: var(--jz-text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.li-name.unnamed { color: var(--jz-text-dim); font-style: italic; }
.li-act { flex-shrink: 0; }
.warn-ic { color: var(--el-color-danger); flex-shrink: 0; }
.del-ic { color: var(--jz-text-dim); cursor: pointer; flex-shrink: 0; }
.del-ic:hover { color: var(--el-color-danger); }
.add-ic { color: var(--el-color-primary); cursor: pointer; flex-shrink: 0; }
/* 右编辑 */
.mon-editor { flex: 1; overflow-y: auto; display: flex; flex-direction: column; gap: 10px; padding-right: 4px; }
.ed-field { display: flex; align-items: flex-start; gap: 8px; }
.ed-field.mt { margin-top: 4px; }
.ed-ctrl { flex: 1; }
.lbl { font-size: 12px; color: var(--jz-text-dim); flex-shrink: 0; line-height: 24px; }
.lbl.req::after { content: ' *'; color: var(--el-color-danger); }
.err-tip { color: var(--el-color-danger); font-size: 11px; margin-top: 2px; }
.is-invalid :deep(.el-input__wrapper) { box-shadow: 0 0 0 1px var(--el-color-danger) inset; }
/* 二级编辑分组：带标题 + 左边框 + 缩进，明确归属 */
.group { border: 1px solid var(--jz-border); border-left: 3px solid var(--el-color-primary-light-5); border-radius: 6px; overflow: hidden; }
.group-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 6px 10px; background: var(--el-fill-color-light); }
.group-title { font-size: 12px; font-weight: 600; color: var(--jz-text); }
.group-body { padding: 8px 10px; display: flex; flex-direction: column; gap: 6px; }
.note { font-size: 12px; color: var(--jz-text-dim); }
.cond { display: flex; align-items: center; gap: 6px; }
.cond-val { flex: 1; }
.cond-val :deep(.el-input__inner) { font-family: monospace; }
.foot-hint { float: left; font-size: 11px; color: var(--jz-text-dim); line-height: 32px; }
</style>

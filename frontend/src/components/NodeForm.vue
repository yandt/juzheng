<script setup lang="ts">
// NodeForm：节点编辑表单（协议选择 + 按协议动态渲染字段）。
// 配合 FormDialog 使用，v-model 双向绑定 SingBoxNode。
import { fieldsFor, type SingBoxNode, type FieldDef } from '../configModel'
import { t } from '../i18n'

const protocolOptions = ['direct', 'block', 'dns', 'http', 'vmess', 'vless', 'trojan', 'shadowsocks', 'hysteria2', 'wireguard']

const props = defineProps<{
  modelValue: SingBoxNode
}>()
const emit = defineEmits<{ (e: 'update:modelValue', v: SingBoxNode): void }>()

// 字段值读写：type/tag/server/server_port 走顶层属性，其余走 raw[k]。
function fieldValue(def: FieldDef): any {
  const k = def.key
  const n = props.modelValue
  if (k === 'type') return n.type
  if (k === 'tag') return n.tag
  if (k === 'server') return n.server
  if (k === 'server_port') return n.server_port
  return n.raw?.[k]
}

function setFieldValue(def: FieldDef, v: any) {
  const k = def.key
  // 用浅拷贝触发 v-model 更新
  const updated = { ...props.modelValue }
  if (k === 'type') { updated.type = v; emit('update:modelValue', updated); return }
  if (k === 'tag') { updated.tag = v; emit('update:modelValue', updated); return }
  if (k === 'server') { updated.server = v; emit('update:modelValue', updated); return }
  if (k === 'server_port') { updated.server_port = Number(v); emit('update:modelValue', updated); return }
  if (!updated.raw) updated.raw = {}
  updated.raw[k] = def.type === 'int' ? Number(v) : v
  emit('update:modelValue', updated)
}
</script>

<template>
  <el-form label-width="120px" label-position="right">
    <el-form-item :label="t('comp.protocolType')">
      <el-select :model-value="modelValue.type" @update:model-value="(v: string) => setFieldValue({ key: 'type', label: '', type: 'string' }, v)">
        <el-option v-for="p in protocolOptions" :key="p" :label="p" :value="p" />
      </el-select>
    </el-form-item>
    <el-form-item v-for="def in fieldsFor(modelValue.type)" :key="def.key" :label="def.label" :required="def.required">
      <el-input
        v-if="def.type === 'string' || def.type === 'password'"
        :type="def.type === 'password' ? 'password' : 'text'"
        :show-password="def.type === 'password'"
        :placeholder="def.placeholder"
        :model-value="fieldValue(def)"
        @update:model-value="(v: string) => setFieldValue(def, v)"
      />
      <el-input-number
        v-else-if="def.type === 'int'"
        :model-value="fieldValue(def)"
        @update:model-value="(v: number | undefined) => setFieldValue(def, v ?? 0)"
        controls-position="right"
      />
      <el-select
        v-else-if="def.type === 'select'"
        :model-value="fieldValue(def)"
        @update:model-value="(v: string) => setFieldValue(def, v)"
      >
        <el-option v-for="o in def.options" :key="o" :label="o" :value="o" />
      </el-select>
    </el-form-item>
  </el-form>
</template>

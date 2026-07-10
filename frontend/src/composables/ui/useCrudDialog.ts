// composables/ui/useCrudDialog.ts — 通用 CRUD 弹窗状态机。
//
// 节点和代理组的 openAdd/openEdit/save/remove 完全同构，本 composable 吸收差异。
// 返回弹窗状态（dialog/index/draft）+ 4 个操作函数，配合 FormDialog + 深拷贝实现。
//
// 用法：
//   const nodeCrud = useCrudDialog({
//     list: () => config.value.nodes,        // 列表 getter（响应式）
//     makeDefault: () => ({ type:'vmess', tag:`node-${n}`, ... }),
//     validate: (draft) => { if (!draft.tag) return 'tag 不能为空' },
//     beforeRemove: (item) => { 清理引用 },
//     messages: { added: '节点已添加', updated: '节点已更新', removed: '已删除', confirmRemove: '确定删除？' },
//   })

import { ref, type Ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { t } from '../../i18n'

interface CrudOptions<T> {
  /** 列表 getter（返回响应式数组引用） */
  list: () => T[]
  /** 新增时的默认对象工厂 */
  makeDefault: () => T
  /** 保存前校验，返回错误信息字符串（通过则不返回） */
  validate?: (draft: T) => string | void
  /** 编辑时若 tag 改变（重命名），在写回列表前调用，用于级联更新所有引用 */
  onRename?: (oldTag: string, newTag: string) => void
  /** 删除前副作用（如清理其他列表的引用），在 splice 之前调用 */
  beforeRemove?: (item: T, index: number) => void
  /** 文案 */
  messages?: {
    added?: string
    updated?: string
    removed?: string
    confirmRemove?: (item: T) => string
  }
}

export function useCrudDialog<T extends { tag?: string }>(opts: CrudOptions<T>) {
  const dialog = ref(false)
  const index = ref(-1)
  const draft = ref<T>(opts.makeDefault()) as Ref<T>

  const messages = opts.messages ?? {}

  function openAdd() {
    index.value = -1
    draft.value = opts.makeDefault()
    dialog.value = true
  }

  function openEdit(i: number) {
    index.value = i
    draft.value = JSON.parse(JSON.stringify(opts.list()[i]))
    dialog.value = true
  }

  function save() {
    // 校验
    if (opts.validate) {
      const err = opts.validate(draft.value)
      if (err) { ElMessage.warning(err); return }
    }
    const arr = opts.list()
    if (index.value === -1) {
      arr.push(draft.value)
      ElMessage.success(messages.added ?? t('msg.added'))
    } else {
      // 重命名：tag 变了则先级联更新所有引用（groups/rules/dns/final），再写回列表
      const oldTag = (arr[index.value] as { tag?: string })?.tag
      const newTag = (draft.value as { tag?: string })?.tag
      if (opts.onRename && oldTag && newTag && oldTag !== newTag) opts.onRename(oldTag, newTag)
      arr[index.value] = draft.value
      ElMessage.success(messages.updated ?? t('msg.updated'))
    }
    dialog.value = false
  }

  function remove(i: number) {
    const arr = opts.list()
    const item = arr[i]
    const confirmMsg = messages.confirmRemove
      ? messages.confirmRemove(item)
      : t('msg.confirmRemove', { tag: (item as any).tag ?? '' })
    ElMessageBox.confirm(confirmMsg, t('msg.deleteTitle'), { type: 'warning' }).then(() => {
      if (opts.beforeRemove) opts.beforeRemove(item, i)
      arr.splice(i, 1)
      ElMessage.success(messages.removed ?? t('msg.removed'))
    }).catch(() => {})
  }

  return { dialog, index, draft, openAdd, openEdit, save, remove }
}

export const zh = {
  'source.title': '源码 (JSON)',
  'source.modeEdit': '编辑源码',
  'source.modeFinal': '最终运行配置',
  'source.unsaved': '未保存',
  'source.saved': '已保存',
  'source.finalHint': '这是实际发给 sing-box 内核运行的配置（已移除注释、legacy 字段、特殊 outbound 转 action、geo 规则等）。只读。',
  'source.placeholder': '点击保存加载配置…',
  'source.readonlyHint': '内核运行时配置只读，停止后可编辑',
  'source.genFinalFailed': '生成最终配置失败: ',
}

export const en: typeof zh = {
  'source.title': 'Source (JSON)',
  'source.modeEdit': 'Edit source',
  'source.modeFinal': 'Final runtime config',
  'source.unsaved': 'Unsaved',
  'source.saved': 'Saved',
  'source.finalHint': 'This is the actual config sent to the sing-box core (comments, legacy fields, special outbound-to-action conversions, geo rules, etc. removed). Read-only.',
  'source.placeholder': 'Click Save to load config…',
  'source.readonlyHint': 'Config is read-only while the core is running; stop it to edit.',
  'source.genFinalFailed': 'Failed to generate final config: ',
}

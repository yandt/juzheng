// 组件级 i18n 字典（comp.* 命名空间）：src/components/ 下所有共享组件的用户可见文案。
export const zh = {
  // AppSidebar
  'comp.devBadge': '调试中',
  // FilterInput
  'comp.filterPlaceholder': '过滤...',
  // FormDialog
  'comp.save': '保存',
  'comp.cancel': '取消',
  // GroupCollapseItem
  'comp.groupSelector': '手动选择',
  'comp.groupUrltest': '自动测速',
  'comp.nodesCount': '{n} 节点',
  'comp.testAll': '全部测速',
  'comp.needStartKernel': '需先启动内核',
  'comp.editGroup': '编辑代理组',
  'comp.filterNodesPlaceholder': '过滤节点...',
  'comp.timeout': '超时',
  'comp.noMatchNodes': '无匹配节点',
  // NodeCard
  'comp.dragSort': '拖拽排序',
  'comp.belongsGroups': '归属 {n} 组',
  'comp.notInGroup': '未归属',
  'comp.editNode': '编辑节点',
  'comp.inUseCannotDelete': '已归属代理组，不可删除',
  'comp.deleteNode': '删除节点',
  // NodeForm
  'comp.protocolType': '协议 (type)',
  // RuleEditor
  'comp.matchDomainSuffix': '域名后缀 (domain_suffix)',
  'comp.matchDomainKeyword': '域名关键字 (domain_keyword)',
  'comp.matchDomain': '完整域名 (domain)',
  'comp.matchDomainRegex': '域名正则 (domain_regex)',
  'comp.matchIpCidr': 'IP/CIDR (ip_cidr)',
  'comp.matchGeosite': 'GeoSite (geosite)',
  'comp.matchGeoip': 'GeoIP (geoip)',
  'comp.matchProtocol': '协议 (protocol)',
  'comp.allTypesUsed': '所有匹配类型已用完，每种类型只允许一条规则',
  'comp.addRule': '添加规则',
  'comp.rulesHint': '共 {n} 条规则 · 每种类型仅一条',
  'comp.rulesEmpty': '暂无规则，匹配的流量将走默认路由',
  'comp.placeholderIpCidr': '每行一个 IP/CIDR，如 10.0.0.0/8',
  'comp.placeholderDomain': '每行一个域名，如 example.com',
  'comp.ruleMeta': '{n} 项 · outbound = {tag}',
  // SaveButton
  'comp.saveConfig': '保存配置',
  'comp.saved': '已保存',
  // SaveStatus
  'comp.saving': '保存中…',
  'comp.pendingSave': '待保存…',
}

export const en: typeof zh = {
  // AppSidebar
  'comp.devBadge': 'Debugging',
  // FilterInput
  'comp.filterPlaceholder': 'Filter...',
  // FormDialog
  'comp.save': 'Save',
  'comp.cancel': 'Cancel',
  // GroupCollapseItem
  'comp.groupSelector': 'Manual select',
  'comp.groupUrltest': 'Auto test',
  'comp.nodesCount': '{n} nodes',
  'comp.testAll': 'Test all',
  'comp.needStartKernel': 'Start the kernel first',
  'comp.editGroup': 'Edit group',
  'comp.filterNodesPlaceholder': 'Filter nodes...',
  'comp.timeout': 'Timeout',
  'comp.noMatchNodes': 'No matching nodes',
  // NodeCard
  'comp.dragSort': 'Drag to sort',
  'comp.belongsGroups': 'In {n} groups',
  'comp.notInGroup': 'Not in any group',
  'comp.editNode': 'Edit node',
  'comp.inUseCannotDelete': 'In a proxy group, cannot delete',
  'comp.deleteNode': 'Delete node',
  // NodeForm
  'comp.protocolType': 'Protocol (type)',
  // RuleEditor
  'comp.matchDomainSuffix': 'Domain suffix (domain_suffix)',
  'comp.matchDomainKeyword': 'Domain keyword (domain_keyword)',
  'comp.matchDomain': 'Full domain (domain)',
  'comp.matchDomainRegex': 'Domain regex (domain_regex)',
  'comp.matchIpCidr': 'IP/CIDR (ip_cidr)',
  'comp.matchGeosite': 'GeoSite (geosite)',
  'comp.matchGeoip': 'GeoIP (geoip)',
  'comp.matchProtocol': 'Protocol (protocol)',
  'comp.allTypesUsed': 'All match types are used; only one rule per type',
  'comp.addRule': 'Add rule',
  'comp.rulesHint': '{n} rules · one per type',
  'comp.rulesEmpty': 'No rules yet; matched traffic uses the default route',
  'comp.placeholderIpCidr': 'One IP/CIDR per line, e.g. 10.0.0.0/8',
  'comp.placeholderDomain': 'One domain per line, e.g. example.com',
  'comp.ruleMeta': '{n} items · outbound = {tag}',
  // SaveButton
  'comp.saveConfig': 'Save config',
  'comp.saved': 'Saved',
  // SaveStatus
  'comp.saving': 'Saving…',
  'comp.pendingSave': 'Pending…',
}

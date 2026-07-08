// composables 内的 toast / 确认框 / 状态与错误提示文案（msg. 命名空间）。
export const zh = {
  // 通用按钮
  'msg.confirm': '确定',
  'msg.cancel': '取消',
  'msg.create': '创建',

  // useSbox
  'msg.tunInstalledRunning': 'TUN 服务已安装并运行',
  'msg.tunInstalledNotReady': 'TUN 服务已安装，但进程未就绪，请稍候',
  'msg.installIncomplete': '安装未完成',
  'msg.helperInstallFailed': 'helper 安装失败: {error}',
  'msg.tunUninstalled': 'TUN 服务已卸载',
  'msg.uninstallFailed': '卸载失败: {error}',
  'msg.tunNotRunning': 'TUN 服务未运行，请先安装',
  'msg.sboxStarted': 'sing-box 内核已启动',
  'msg.startFailed': '启动失败: {error}',
  'msg.stopFailed': '停止失败: {error}',
  'msg.configParseFailed': '配置解析失败（图形模式不可用，请用源码模式编辑）',
  'msg.loadFailed': '加载失败: {error}',
  'msg.jsonSyntaxError': 'JSON 语法错误: {error}',
  'msg.configSaved': '配置已保存',
  'msg.saveFailed': '保存失败: {error}',
  'msg.appliedLive': '已实时生效',
  'msg.reloadFailed': '实时生效失败，请重启内核: {error}',
  'msg.jsonParseFailed': 'JSON 解析失败（修正后自动保存；图形模式暂不可用）',
  'msg.sysProxyOn': '系统代理已开启',
  'msg.sysProxyOff': '系统代理已关闭',
  'msg.sysProxySetFailed': '系统代理设置失败: {error}',
  'msg.sysProxyApplied': '系统代理设置已应用',

  // useSubscriptions
  'msg.switchedTo': '已切换到订阅「{name}」，需重启内核生效',
  'msg.switchFailed': '切换失败: {error}',
  'msg.enterSubName': '请输入新订阅名',
  'msg.newSubTitle': '新建订阅',
  'msg.subNameInvalid': '订阅名不能含特殊字符 / \\ : * ? " < > |',
  'msg.subCreated': '订阅已创建',
  'msg.createFailed': '创建失败: {error}',
  'msg.confirmDeleteSub': '确定删除订阅「{name}」？',
  'msg.deleteSubTitle': '删除订阅',
  'msg.deleted': '已删除',
  'msg.deleteFailed': '删除失败: {error}',
  'msg.renameSubTitle': '重命名订阅',
  'msg.subNameInvalidShort': '订阅名不能含特殊字符',
  'msg.renamed': '已重命名',
  'msg.renameFailed': '重命名失败: {error}',
  'msg.usageRefreshed': '用量已刷新',
  'msg.refreshFailed': '刷新失败: {error}',
  'msg.selectSubFile': '选择订阅/配置文件',
  'msg.configFile': '配置文件',
  'msg.importSuccess': '导入成功',
  'msg.importFailed': '导入失败: {error}',
  'msg.exportSubTitle': '导出订阅',
  'msg.copiedToClipboard': '内容已复制到剪贴板（请粘贴保存）',
  'msg.exportFailed': '导出失败: {error}',

  // useMitm
  'msg.operationFailed': '操作失败: {error}',
  'msg.caCertTrusted': 'CA 证书已信任',
  'msg.installIncompleteCancel': '安装未完成（用户取消？）',
  'msg.certInstallFailed': '证书安装失败: {error}',

  // useClashApi
  'msg.selectNodeFailed': '切换节点失败: {error}',
  'msg.setModeFailed': '切换代理模式失败: {error}',

  // useMeta
  'msg.cardConfigSaveFailed': '卡片配置保存失败: {error}',

  // ui/useCrudDialog
  'msg.added': '已添加',
  'msg.updated': '已更新',
  'msg.removed': '已删除',
  'msg.confirmRemove': '确定删除「{tag}」？',
  'msg.deleteTitle': '删除',
}

export const en: typeof zh = {
  // 通用按钮
  'msg.confirm': 'OK',
  'msg.cancel': 'Cancel',
  'msg.create': 'Create',

  // useSbox
  'msg.tunInstalledRunning': 'TUN service installed and running',
  'msg.tunInstalledNotReady': 'TUN service installed, but the process is not ready yet, please wait',
  'msg.installIncomplete': 'Installation incomplete',
  'msg.helperInstallFailed': 'Helper installation failed: {error}',
  'msg.tunUninstalled': 'TUN service uninstalled',
  'msg.uninstallFailed': 'Uninstall failed: {error}',
  'msg.tunNotRunning': 'TUN service is not running, please install it first',
  'msg.sboxStarted': 'sing-box core started',
  'msg.startFailed': 'Start failed: {error}',
  'msg.stopFailed': 'Stop failed: {error}',
  'msg.configParseFailed': 'Failed to parse config (graphical mode unavailable, please edit in source mode)',
  'msg.loadFailed': 'Load failed: {error}',
  'msg.jsonSyntaxError': 'JSON syntax error: {error}',
  'msg.configSaved': 'Config saved',
  'msg.saveFailed': 'Save failed: {error}',
  'msg.appliedLive': 'Applied live',
  'msg.reloadFailed': 'Live apply failed, please restart the kernel: {error}',
  'msg.jsonParseFailed': 'JSON parse failed (auto-saves after fixing; graphical mode temporarily unavailable)',
  'msg.sysProxyOn': 'System proxy enabled',
  'msg.sysProxyOff': 'System proxy disabled',
  'msg.sysProxySetFailed': 'Failed to set system proxy: {error}',
  'msg.sysProxyApplied': 'System proxy settings applied',

  // useSubscriptions
  'msg.switchedTo': 'Switched to subscription "{name}", restart the core to take effect',
  'msg.switchFailed': 'Switch failed: {error}',
  'msg.enterSubName': 'Enter a new subscription name',
  'msg.newSubTitle': 'New Subscription',
  'msg.subNameInvalid': 'Subscription name cannot contain special characters / \\ : * ? " < > |',
  'msg.subCreated': 'Subscription created',
  'msg.createFailed': 'Creation failed: {error}',
  'msg.confirmDeleteSub': 'Delete subscription "{name}"?',
  'msg.deleteSubTitle': 'Delete Subscription',
  'msg.deleted': 'Deleted',
  'msg.deleteFailed': 'Delete failed: {error}',
  'msg.renameSubTitle': 'Rename Subscription',
  'msg.subNameInvalidShort': 'Subscription name cannot contain special characters',
  'msg.renamed': 'Renamed',
  'msg.renameFailed': 'Rename failed: {error}',
  'msg.usageRefreshed': 'Usage refreshed',
  'msg.refreshFailed': 'Refresh failed: {error}',
  'msg.selectSubFile': 'Select subscription/config file',
  'msg.configFile': 'Config file',
  'msg.importSuccess': 'Import successful',
  'msg.importFailed': 'Import failed: {error}',
  'msg.exportSubTitle': 'Export Subscription',
  'msg.copiedToClipboard': 'Content copied to clipboard (please paste to save)',
  'msg.exportFailed': 'Export failed: {error}',

  // useMitm
  'msg.operationFailed': 'Operation failed: {error}',
  'msg.caCertTrusted': 'CA certificate trusted',
  'msg.installIncompleteCancel': 'Installation incomplete (cancelled by user?)',
  'msg.certInstallFailed': 'Certificate installation failed: {error}',

  // useClashApi
  'msg.selectNodeFailed': 'Failed to switch node: {error}',
  'msg.setModeFailed': 'Failed to switch proxy mode: {error}',

  // useMeta
  'msg.cardConfigSaveFailed': 'Failed to save card config: {error}',

  // ui/useCrudDialog
  'msg.added': 'Added',
  'msg.updated': 'Updated',
  'msg.removed': 'Removed',
  'msg.confirmRemove': 'Delete "{tag}"?',
  'msg.deleteTitle': 'Delete',
}

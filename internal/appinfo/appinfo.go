// Package appinfo 提供应用与系统元信息（L1 系统层）。
//
// 版本号规则（语义化版本，三段式）：
//   - 主版本（Major）：整个系统颠覆式改变才递增
//   - 次版本（Minor）：功能增加或重大改变时递增（小版本归零）
//   - 修订版（Patch）：小范围改动/bugfix 时递增
//
// 不依赖 Wails。替换实现（如改成从 ldflags 注入版本）只需改本包，不影响上层 service。
package appinfo

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// 版本号组件（主.次.修，三段式，每次编译后递增）。
// 约定：小范围改动/bugfix → Patch +1；功能增加/重大改变 → Minor +1 且 Patch 归零；
// 系统颠覆式改变 → Major +1 且 Minor/Patch 归零。
const (
	VersionMajor = "0" // 系统颠覆式改变（未到 1.0 正式版）
	VersionMinor = "2" // 功能增加/重大改变（系统分层重构 + 模块化）
	VersionPatch = "95" // 规则新增 process_path_regex(进程路径正则),这是按进程通配的唯一途径:sing-box v1.13.14 的 process_name 是 processMap[filepath.Base(path)] 哈希精确匹配(大小写敏感、无通配),且不存在 process_name_regex;process_path_regex 用 regexp.MatchString 对完整路径做「搜索」而非全匹配,故一条 UURemote 即可覆盖 UURemote/UURemoteServer/UURemoteUpdater(已用本机真实进程路径实测:3 个全命中、零误伤)。含 94:修复连接页入口标签误报:mixed-back 入站身兼两职(MITM 解密回注 + 系统代理入口,sysproxy 就指向它),原 inboundKind 把所有 mixed-back 一律标成「MITM回注」,导致普通系统代理流量(Chrome/UURemote 等)被误标为已抓包解密,且「系统代理」分支成死代码。二者在 Clash API 层同形(均 mixed/mixed-back、源 127.0.0.1),改用发起进程判别:回注由内嵌 go-mitmproxy 即本进程发起,故新增 AppInfo.ExecName(filepath.Base(os.Executable()))供前端比对;execName 未知时保守按系统代理处理,不误标。含 93:修复文本输入逐字符提交:抓包域名弹窗每敲一个字符就 markDirty(RuleEditor 为维护 textarea 文本镜像会逐字符 emit)→ 全量序列化 1.2MB 配置 + 写 singbox.json 与订阅文件 + 热重载内核,打字停顿超过 600ms 防抖即触发,一个字符重载一次内核(网络抖动 + toast 刷屏)。改为关窗时统一提交一次(touched 标记,无改动不提交);另 6 处文本输入(TUN 网卡名/地址、DNS 服务器地址/分流规则值、局域网共享用户名/密码)由 @input(每键)改为 @change(失焦或回车)。含 92:配置目录改回 ~/.juzheng(全平台统一,含 Windows %USERPROFILE%\.juzheng),并删除整套一次性迁移代码(migrateLegacyDir/copyDir/copyFile/LegacyJuzhengDir + migrate_test)。起因:v85 的"平台规范化"(UserConfigDir/Juzheng)收益甚微却引入迁移逻辑,其守卫把"新目录已存在"等同于"已迁移",被 v0.2.83 之前版本(直接调 UserConfigDir)遗留的同名残留目录骗过 → 迁移静默跳过 → 真实配置(~/.juzheng)读不到,应用退回默认模板。点目录是开发/网络工具惯例(~/.ssh/.aws/.docker/.kube)且全平台路径一致。含 91:规则支持按进程分流:RuleMatchType 加 process_name/process_path(依赖已开启的 route.find_process),规则页+组/MITM 规则编辑器下拉可选,中英 i18n 齐全。并修复既有 bug:指向「直连」的用户规则重新加载后会从规则页消失(parseConfig 只认代理组/用户节点为用户规则出口,direct 被误归为系统规则致不可见/不可编辑且优先级被提前)——现 direct 纳入用户规则出口(排除 to-mitmproxy),并补 inbound 限定规则保真守卫;MITM/回环/clash_mode 均不受影响,往返幂等。含 90:首页新增「对外代理服务」卡片:显示局域网共享服务状态(服务中/待启动、是否鉴权),列出其他设备可用地址(本机各 LAN IP:端口,点击复制),未开启时引导去设置。新增 appinfo.LANAddresses() 探测本机私有 IPv4 + AppService.GetLANAddresses binding。含 89:新增局域网共享:设置→网络与代理加「局域网共享」开关,开启后 serde 注入 mixed 入站(lan-in,绑 0.0.0.0,HTTP+SOCKS5 同端口,默认 7890,可选用户名/密码鉴权)+路由兜底走主代理出口;parse/serialize 双向支持;README/说明书同步。含 88:主菜单顺序调整为 首页-订阅-代理-规则-监控-连接-设置-源码;多字标题缩为两字(流量监控→监控、代理与节点→代理)。含 87:重构:拆分 configModel.ts(622 行)到 config/ 子目录 —— types.ts(结构化数据模型+常量)/fields.ts(协议字段元数据)/serde.ts(parse/serialize 双向转换);configModel.ts 改为 barrel re-export,保持既有 import 路径零改动,typecheck+build 均通过。含 86:Windows 内嵌 wintun.dll(amd64/arm64 官方签名,按 helper 架构 embed):helper 安装时写到 %ProgramData%\Juzheng,TUN 模式开箱即用,免手动下载;卸载清理;preflight 文案改为自动安装+按 runtime.GOARCH。含 85:配置目录改各平台规范:os.UserConfigDir()/Juzheng(mac ~/Library/Application Support、win %AppData%、linux ~/.config);EnsureDefaults 首次运行自动从旧 ~/.juzheng 一次性迁移(优先 rename,跨卷复制且保留旧目录);helper 不受影响(经 IPC)。含 84:关闭启动自动弹 WebKit Inspector(OpenInspectorOnStartup=false,保留 DevTools 手动开)。含 83:macOS 托盘回归单色 Template(系统自适应),改用形状区分状态:运行=完整戴帽剪影(有帽翅)/待机=去帽翅剪影(裁掉帽檐 y 段,systray-mac-*)。两态均 SetTemplateIcon,绕开 Wails Template 单色化彩色图的坑。含 81-82:托盘着色方案迭代(已废弃):Wails setIcon 复用残留 isTemplateIcon 标志致 SetIcon(active) 仍被模板单色化抹掉绿点。改两态均用非模板着色图(灰身 idle/灰身+绿点 active,systray-mac-*),明暗菜单栏均可见。含 80:调试模式退出跳过停内核:内核在独立 helper 进程,调试重编重启 app 不再反复关停 TUN 致网络波动,重启后经 helper status 无缝接管(singbox_service ServiceShutdown)。含 79:测速时延迟位置显示等待:useClashApi 加共享 testing 状态(测速中置位),首页节点下拉 label 显示「测试中…」+按钮 loading,代理组成员卡片延迟位显示旋转 loading+「测试中」。含 78:网络设置卡片链路就绪提示压缩为「链路就绪」(三圆点已表状态),标题加 nowrap 防被挤换行。含 77:流量统计默认改 10 分钟跨度,图表高度 170→120 更紧凑。含 76:流量统计图表加鼠标悬停:定位最近时点,画竖线指示+双序列高亮点,浮层标签显示该时点时间与上/下行速率(TrafficChart)。含 75:首页流量统计卡片:双序列 SVG 面积图(上/下行,零依赖 TrafficChart),时间跨度 1/5/10/30/60 分钟切换,按代理组过滤(chains 归并),卡片跨 2 列;图下 6 指标(上/下速度、活跃连接、上/下数据量、内核内存)。useTrafficStats 单例每 2s 采样(复用 useConnections 连接快照 + Clash /memory 流式读首帧),60 分钟滚动缓冲+桶平均降采样。含 74:Windows 启动预检:启用 TUN 但缺 wintun.dll 时给可操作提示(下载链接+放置路径),不擅自打包 DLL(preflight_windows/other)。含 73:helper UAC 自提权安装:helper.exe 兼作安装器(--install/--uninstall),app 经 PowerShell RunAs 提权拉起;传当前用户 SID 收紧命名管道 ACL(EnvAllowedSID)。含 72:窗口/图标/托盘修复:page-header 右侧为三按钮让位(设置键可点);Logo 区高度对齐 page-header;exe 嵌图标+manifest(rsrc syso);托盘图标改可见着色(灰/绿,tray_darwin/other)。含 71:自绘三按钮(最小化/最大化禁用/关闭)移入界面右上角;最小化走任务栏(hideOnMinimise 平台相关),关闭隐藏到托盘。含 70:Frameless 无边框;69:Windows 移植地基:隔离 main.go cgo(dock_darwin/dock_other);iproto/helper 命名管道传输(go-winio);helper 可作 Windows 服务(svc.Run);sysproxy/helperclient/mitmcore证书/paths 拆 _darwin/_windows;Windows 侧实现就位(注册表代理/SCM 服务/certutil)。四路交叉编译通过,macOS 零回归。含 68:SSE 合并/重连归并/流量监控刷新
)

// Info 是应用与系统元信息（供前端「关于」/版本号显示）。
type Info struct {
	AppVersion string `json:"appVersion"` // 完整版本号 主.次.修
	GoVersion  string `json:"goVersion"`  // Go 工具链版本
	OS         string `json:"os"`         // 操作系统 darwin/windows/linux
	Arch       string `json:"arch"`       // CPU 架构
	AppName    string `json:"appName"`    // 应用名
	ExecName   string `json:"execName"`   // 本进程可执行文件名（连接页据此识别 MITM 回注：回注由内嵌 go-mitmproxy 即本进程发起）
	IsDev      bool   `json:"isDev"`      // 是否调试模式（wails3 dev）
}

// FullVersion 返回完整版本号：主.次.修。
func FullVersion() string {
	return VersionMajor + "." + VersionMinor + "." + VersionPatch
}

// Get 返回完整的应用与系统信息。
func Get() Info {
	return Info{
		AppVersion: FullVersion(),
		GoVersion:  runtime.Version(),
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		AppName:    "Juzheng",
		ExecName:   execName(),
		IsDev:      isDevBuild(),
	}
}

// execName 返回本进程可执行文件名（不含路径）。取不到时返回空串（调用方按未知处理）。
func execName() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Base(exe)
}

// LANAddresses 返回本机所有可供局域网访问的 IPv4 地址（供「对外代理服务」卡片显示,
// 让用户知道其他设备该把代理指向哪个 IP）。跳过回环、down、未启用的网卡与链路本地地址。
// 优先返回私有网段（192.168/10/172.16-31),排序时私有地址在前。
func LANAddresses() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var priv, other []string
	for _, ifc := range ifaces {
		// 跳过未启用或回环网卡。
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			ip4 := ip.To4()
			if ip4 == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue // 只要 IPv4,排除回环 / 169.254 链路本地
			}
			s := ip4.String()
			if ip4.IsPrivate() {
				priv = append(priv, s)
			} else {
				other = append(other, s)
			}
		}
	}
	return append(priv, other...)
}

// isDevBuild 判断当前是否为开发构建（wails3 dev）。
func isDevBuild() bool {
	if os.Getenv("WAILS_DEV") != "" {
		return true
	}
	if exe, err := os.Executable(); err == nil {
		return strings.Contains(exe, ".dev.app") || strings.Contains(exe, ".dev")
	}
	return false
}

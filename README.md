<div align="right">

**简体中文** · [English](README.en.md)

</div>

# 居正 · Juzheng

> 基于 sing-box 的跨平台网络代理客户端,内置 HTTPS 流量监控。

**居正**（jū zhèng）是一款面向 macOS 与 Windows 的桌面代理客户端。它以 [sing-box](https://github.com/SagerNet/sing-box) 为代理内核,通过特权 helper 以 TUN 模式实现系统级透明代理;同时内置基于 [go-mitmproxy](https://github.com/lqqyt2423/go-mitmproxy) 的 MITM 流量监控,可解密分析 HTTPS 明文。图形界面提供订阅管理、代理组/节点、规则编辑、DNS 配置、实时流量与连接监控。

## ✨ 功能特性

- **TUN 透明代理** —— 系统级接管全部流量,无需逐个应用配置代理。
- **订阅管理** —— 导入/更新订阅,自动解析节点与代理组。
- **代理与节点** —— 代理组（selector / urltest 自动测速）、节点延迟测试、一键切换。
- **规则编辑** —— 图形化编辑分流规则（域名/IP/GeoSite/GeoIP 等),顺序即优先级。
- **代理模式** —— Rule（规则）/ Global（全局）/ Direct（直连）一键切换。
- **DNS 配置** —— 多 DNS 服务器、分流规则、解析策略、兜底服务器。
- **流量监控** —— MITM 解密 HTTPS,查看请求/响应明文;支持上游链式代理。
- **实时统计与连接** —— 首页流量图表（按代理组过滤)、连接列表（含进程名)。
- **系统托盘** —— 状态图标随内核运行状态变化,后台常驻。

## 📖 文档

- [功能说明书](docs/FEATURES.md) —— 逐页详解各功能。
- [使用说明书](docs/USER-GUIDE.md) —— 安装、配置、日常使用与常见问题。

## 🧱 技术栈

| 层 | 技术 |
| --- | --- |
| 代理内核 | [sing-box](https://github.com/SagerNet/sing-box)（TUN 透明代理,经特权 helper 以 root/SYSTEM 运行) |
| 流量监控 | Go + [go-mitmproxy](https://github.com/lqqyt2423/go-mitmproxy)（内嵌 MITM,原生 SSE) |
| 桌面壳 | [Wails v3](https://v3.wails.io/) alpha2.115（原生 SystemTray + 无边框窗口) |
| 前端 | Vue 3 + TypeScript + Vite + Element Plus |
| 后端 | Go 1.26 |

## 🏗️ 架构

- **主应用**（非特权)：Wails 桌面应用,负责 UI、订阅/配置管理、MITM 流量监控。
- **特权 helper**（`juzheng-helper`)：以 root（macOS)/ SYSTEM（Windows）运行的守护进程,承载 sing-box 内核（TUN 需高权限)。
  - macOS：由 launchd（`/Library/LaunchDaemons`）管理。
  - Windows：注册为系统服务（SCM),内嵌 `wintun.dll`（按架构 embed,安装时写入数据目录,TUN 开箱即用)。
  - 主应用通过 unix socket（macOS)/ 命名管道（Windows）与之通信（start/stop/status/ping)。
- **分层约定**：`internal/` 下为基础设施与业务模块（paths/events/iproto → mitmcore/singboxcfg/subscriptions/sysproxy/helperclient),根目录 `*_service.go` 为 Wails service 薄壳,下层不调上层。
- **配置目录**：遵循各平台规范 `os.UserConfigDir()/Juzheng`（macOS `~/Library/Application Support`、Windows `%AppData%`、Linux `~/.config`)。

## 💻 支持平台

| 平台 | 架构 | 说明 |
| --- | --- | --- |
| macOS | Apple Silicon / Intel | 11+（arm64)、10.15+（amd64),通用二进制 |
| Windows | x64 / ARM64 | Windows 10/11,内置 wintun.dll |

## 🛠️ 开发环境要求

- Go 1.26+
- Node.js 20+
- [Wails v3 CLI](https://v3.wails.io/)
- macOS：Xcode Command Line Tools
- Windows：WebView2 Runtime（一般已随系统预装)

## 🚀 快速开始

### 1. 安装依赖

```bash
# wails3 CLI(如未装)
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
export PATH="$PATH:$(go env GOPATH)/bin"

# 前端依赖
cd frontend && npm install && cd ..
```

### 2. 开发模式运行

```bash
task dev
# 或： wails3 dev -config ./build/config.yml
```

### 3. 构建生产包

```bash
task package        # 当前平台打包
```

## 📦 发布

一键构建并发布到 GitHub Releases（macOS dmg + Windows zip/NSIS 安装包,含 amd64/arm64)：

```bash
scripts/release.sh                 # 构建并发布正式版
scripts/release.sh --no-publish    # 仅本地构建,不发布
scripts/release.sh --prerelease    # 发布预览版
```

## 🔍 使用流量监控（可选）

1. 在**监控**页点击「启动监控」（默认监听 `:9080`)。
2. **首次使用需信任 CA 证书**（应用会提示路径)：
   ```bash
   # macOS
   sudo security add-trusted-cert -d -r trustRoot \
     -k /Library/Keychains/System.keychain \
     "$HOME/Library/Application Support/Juzheng/ca/mitmproxy-ca-cert.cer"
   ```
   Windows 会自动经 `certutil` 导入到受信任根存储。
3. 让目标程序走代理（如 `HTTPS_PROXY=http://127.0.0.1:9080 curl https://example.com`),即可在监控页看到请求明文。
4. 「上游代理」可填其他代理的入口（如 `127.0.0.1:7890`),流量将链式经其出网。

## 📁 项目结构

```
juzheng/
├── main.go                  # 应用入口(Wails app + 窗口 + 托盘 + 事件注册)
├── *_service.go             # Wails service 薄壳(singbox/subscription/sysproxy/proxy/app)
├── flow_types.go            # 流量数据结构(前后端共享契约)
├── helper/                  # 特权 helper(root/SYSTEM 守护进程,承载 sing-box 内核)
├── internal/                # 基础设施与业务模块(分层,下层不调上层)
├── configs/                 # sing-box 默认配置模板
├── frontend/
│   ├── src/views/           # 页面(首页/订阅/代理/规则/监控/连接/设置/源码)
│   ├── src/config/          # sing-box 配置双向转换(types/fields/serde)
│   └── bindings/            # 自动生成的 TS 绑定(wails3 generate bindings)
├── build/                   # 平台构建配置
├── scripts/release.sh       # 一键发布脚本
└── Taskfile.yml             # 构建/打包任务
```

## 🔒 隐私说明

本工具仅在本地运行,捕获的流量明文留在内存中,不外传。CA 证书含私钥,仅本机使用,请勿提交或外传。

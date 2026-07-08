# Juzheng

基于 sing-box 的 macOS 网络代理客户端（Wails 桌面应用）。支持 TUN 透明代理、订阅管理、节点分组与规则编辑，并内置流量监控面板（MITM 解密分析 HTTPS 明文）。

## 技术栈

- **代理内核**：[sing-box](https://github.com/SagerNet/sing-box)（TUN 透明代理，经特权 helper 以 root 运行）
- **流量监控**：Go + [go-mitmproxy](https://github.com/lqqyt2423/go-mitmproxy)（内嵌 MITM 代理，原生支持 SSE）
- **桌面壳**：[Wails v3](https://v3.wails.io/) alpha2.112（原生 SystemTray + macOS LSUIElement）
- **前端**：Vue 3 + TypeScript + Vite

## 架构

- **主应用**（非 root）：Wails 桌面应用，提供 UI、订阅/配置管理、MITM 流量监控。
- **特权 helper**（`juzheng-helper`）：以 root 运行的守护进程，承载 sing-box 内核（TUN 需 root），由 launchd（`/Library/LaunchDaemons`）管理，主应用通过 unix socket 与之通信（start/stop/status/ping）。
- **分层约定**：`internal/` 下为基础设施与业务模块（paths/events/iproto → mitmcore/singboxcfg/subscriptions/sysproxy/helperclient），根目录 `*_service.go` 为 Wails service 薄壳，下层不调上层。

## 开发环境要求

- Go 1.26+
- Node.js 20+
- macOS 11+（Apple Silicon）或 10.15+（Intel）
- Xcode Command Line Tools

## 快速开始

### 1. 安装依赖

```bash
# wails3 CLI（如未装）
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
export PATH="$PATH:$(go env GOPATH)/bin"

# 前端依赖
cd frontend && npm install && cd ..
```

### 2. 开发模式运行（热重载）

```bash
wails3 dev
```

### 3. 或直接构建运行

```bash
# 构建前端
cd frontend && npm run build && cd ..
# 构建后端（含嵌入前端）
go build -o bin/juzheng .
./bin/juzheng
```

## 使用流量监控（可选）

1. 在监控页点击「启动监控」按钮（默认监听 `:9080`）
2. **首次使用需信任 CA 证书**（应用会提示路径）：
   ```bash
   sudo security add-trusted-cert -d -r trustRoot \
     -k /Library/Keychains/System.keychain \
     ~/Library/Application\ Support/juzheng/ca/mitmproxy-ca-cert.cer
   ```
3. 让目标程序走代理（如 `HTTPS_PROXY=http://127.0.0.1:9080 curl https://example.com`），即可在监控页看到请求明文
4. 「上游代理」可填其他代理的入口（如 `127.0.0.1:7890`），流量将链式经其出网

## 项目结构

```
juzheng/
├── main.go                  # 应用入口（Wails app + 窗口 + 托盘 + 事件注册）
├── *_service.go             # Wails service 薄壳（singbox/subscription/sysproxy/proxy/app）
├── flow_types.go            # 流量数据结构（前后端共享契约）
├── helper/                  # 特权 helper（root 守护进程，承载 sing-box 内核）
├── internal/                # 基础设施与业务模块（分层，下层不调上层）
├── configs/                 # sing-box 默认配置模板
├── frontend/
│   ├── src/views/           # 页面（主页/订阅/分组/规则/流量监控/设置）
│   └── bindings/            # 自动生成的 TS 绑定（wails3 generate bindings）
├── build/                   # 平台构建配置
└── Taskfile.yml             # 构建/打包任务
```

## 隐私说明

本工具仅在本地运行，捕获的流量明文留在内存中，不外传。CA 证书含私钥，仅本机使用，勿提交或外传。

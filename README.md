# Juzheng — Claude Code 流量监控器

实时监控 Claude Code 发往服务器的 HTTPS 请求明文，并解析/高亮关键片段（撇号码点、日期格式、邮箱、tools、system prompt），用于验证隐私审计发现。

## 技术栈

- **后端**：Go + [go-mitmproxy](https://github.com/lqqyt2423/go-mitmproxy)（内嵌 MITM 代理，原生支持 SSE）
- **桌面壳**：[Wails v3](https://v3.wails.io/) alpha2.112（原生 SystemTray + macOS LSUIElement）
- **前端**：Vue 3 + TypeScript + Vite

## 开发环境要求

- Go 1.25+
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

## 使用流程

1. 启动 Juzheng 应用，点击「启动监控」按钮（默认监听 `:9080`）
2. **首次使用需信任 CA 证书**（应用会提示路径）：
   ```bash
   sudo security add-trusted-cert -d -r trustRoot \
     -k /Library/Keychains/System.keychain \
     ~/Library/Application\ Support/juzheng/ca/mitmproxy-ca-cert.cer
   ```
3. 让 Claude Code 走代理：
   ```bash
   HTTPS_PROXY=http://127.0.0.1:9080 claude
   ```
4. 在 Juzheng 窗口实时看到所有请求的明文（system prompt、messages、tools）
5. 点击请求查看详情，审计提示会标出撇号码点等风险信号

### 链式到 Clash（保留代理出口）

在「上游代理」填 `127.0.0.1:7890`（Clash 的 mixed-port），流量会经 Clash 节点出网。

## 已验证（POC + 编译）

- ✅ go-mitmproxy 能解密 HTTPS（GET/POST + JSON body 完整还原）
- ✅ SSE 三 hook（SSEStart/SSEMessage/SSEEnd）正常触发
- ✅ Wails v3 应用在 macOS 26.5.1 正常启动、加载前端、优雅关闭
- ✅ 后端编译通过，前端类型检查 + 构建通过
- ✅ 绑定生成：1 Service / 7 Methods / 3 Events

## 待手动验证（GUI 交互）

- [ ] 点击「启动监控」后，curl 走代理的请求出现在列表中
- [ ] 信任 CA 后，Claude Code（HTTPS_PROXY）的请求被解密
- [ ] 请求详情里的 `Today's date` 撇号码点确认（官方 API 应为 U+0027）

## 项目结构

```
juzheng/
├── main.go              # 应用入口（Wails app + 窗口 + 事件注册）
├── proxy_service.go     # MitmProxyService（go-mitmproxy 封装 + Addon hooks）
├── flow_types.go        # 流量数据结构（前后端共享契约）
├── poc/main.go          # POC：独立验证 go-mitmproxy（不依赖 Wails）
├── frontend/
│   ├── src/App.vue      # 监控界面（请求列表 + 明文详情 + 审计提示）
│   └── bindings/        # 自动生成的 TS 绑定（wails3 generate bindings）
├── build/               # 平台构建配置
└── Taskfile.yml         # 构建/打包任务
```

## 隐私说明

本工具仅在本地运行，捕获的流量明文留在内存中，不外传。CA 证书含私钥，仅本机使用，勿提交或外传。

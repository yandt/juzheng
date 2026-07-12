<div align="right">

[简体中文](README.md) · **English**

</div>

# Juzheng （居正）

> A cross-platform network proxy client powered by sing-box, with built-in HTTPS traffic inspection.

**Juzheng** (居正, jū zhèng) is a desktop proxy client for macOS and Windows. It uses [sing-box](https://github.com/SagerNet/sing-box) as its proxy core and delivers system-wide transparent proxying in TUN mode via a privileged helper. It also embeds an [go-mitmproxy](https://github.com/lqqyt2423/go-mitmproxy)-based MITM inspector that decrypts and analyzes HTTPS traffic. The GUI covers subscription management, proxy groups/nodes, rule editing, DNS configuration, and real-time traffic & connection monitoring.

## ✨ Features

- **Transparent TUN proxy** — system-level capture of all traffic; no per-app proxy setup.
- **Subscription management** — import/update subscriptions; nodes and groups parsed automatically.
- **Proxies & nodes** — proxy groups (selector / urltest auto-speedtest), node latency tests, one-tap switching.
- **Rule editing** — graphical routing rules (domain / IP / GeoSite / GeoIP …); order = priority.
- **Proxy modes** — one-tap toggle between Rule / Global / Direct.
- **DNS configuration** — multiple DNS servers, routing rules, resolution strategy, fallback server.
- **Traffic inspection** — MITM-decrypt HTTPS to view request/response plaintext; supports upstream chained proxy.
- **Live stats & connections** — homepage traffic chart (filterable by proxy group), connection list (with process names).
- **System tray** — status icon reflects the kernel's running state; stays resident in the background.

## 📖 Documentation

- [Feature Guide](docs/FEATURES.en.md) — a page-by-page walkthrough of every feature.
- [User Guide](docs/USER-GUIDE.en.md) — installation, setup, daily use, and FAQ.

## 🧱 Tech Stack

| Layer | Technology |
| --- | --- |
| Proxy core | [sing-box](https://github.com/SagerNet/sing-box) (transparent TUN proxy, run as root/SYSTEM via a privileged helper) |
| Traffic inspection | Go + [go-mitmproxy](https://github.com/lqqyt2423/go-mitmproxy) (embedded MITM, native SSE) |
| Desktop shell | [Wails v3](https://v3.wails.io/) alpha2.115 (native SystemTray + frameless window) |
| Frontend | Vue 3 + TypeScript + Vite + Element Plus |
| Backend | Go 1.26 |

## 🏗️ Architecture

- **Main app** (unprivileged): Wails desktop app for the UI, subscription/config management, and MITM inspection.
- **Privileged helper** (`juzheng-helper`): a daemon running as root (macOS) / SYSTEM (Windows) that hosts the sing-box kernel (TUN needs elevated privileges).
  - macOS: managed by launchd (`/Library/LaunchDaemons`).
  - Windows: registered as a system service (SCM); embeds `wintun.dll` (per-arch, written to the data dir at install time so TUN works out of the box).
  - The main app talks to it over a unix socket (macOS) / named pipe (Windows): start/stop/status/ping.
- **Layering**: `internal/` holds infrastructure and business modules (paths/events/iproto → mitmcore/singboxcfg/subscriptions/sysproxy/helperclient); root-level `*_service.go` files are thin Wails service shells; lower layers never call upper ones.
- **Config directory**: follows each platform's convention `os.UserConfigDir()/Juzheng` (macOS `~/Library/Application Support`, Windows `%AppData%`, Linux `~/.config`).

## 💻 Supported Platforms

| Platform | Arch | Notes |
| --- | --- | --- |
| macOS | Apple Silicon / Intel | 11+ (arm64), 10.15+ (amd64); universal binary |
| Windows | x64 / ARM64 | Windows 10/11; bundles wintun.dll |

## 🛠️ Requirements

- Go 1.26+
- Node.js 20+
- [Wails v3 CLI](https://v3.wails.io/)
- macOS: Xcode Command Line Tools
- Windows: WebView2 Runtime (usually preinstalled)

## 🚀 Getting Started

### 1. Install dependencies

```bash
# wails3 CLI (if not installed)
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
export PATH="$PATH:$(go env GOPATH)/bin"

# frontend deps
cd frontend && npm install && cd ..
```

### 2. Run in dev mode

```bash
task dev
# or: wails3 dev -config ./build/config.yml
```

### 3. Build a production package

```bash
task package        # package for the current platform
```

## 📦 Release

One-click build & publish to GitHub Releases (macOS dmg + Windows zip/NSIS installer, amd64/arm64):

```bash
scripts/release.sh                 # build and publish a stable release
scripts/release.sh --no-publish    # build locally only, don't publish
scripts/release.sh --prerelease    # publish a pre-release
```

## 🔍 Using Traffic Inspection (optional)

1. On the **Monitor** page click "Start" (listens on `:9080` by default).
2. **First-time use requires trusting the CA certificate** (the app shows the path):
   ```bash
   # macOS
   sudo security add-trusted-cert -d -r trustRoot \
     -k /Library/Keychains/System.keychain \
     "$HOME/Library/Application Support/Juzheng/ca/mitmproxy-ca-cert.cer"
   ```
   On Windows it is imported into the Trusted Root store automatically via `certutil`.
3. Point a program at the proxy (e.g. `HTTPS_PROXY=http://127.0.0.1:9080 curl https://example.com`) to see the plaintext on the Monitor page.
4. The "upstream proxy" field can point at another proxy's inbound (e.g. `127.0.0.1:7890`); traffic will egress through it in a chain.

## 📁 Project Structure

```
juzheng/
├── main.go                  # entry point (Wails app + window + tray + event registration)
├── *_service.go             # thin Wails service shells (singbox/subscription/sysproxy/proxy/app)
├── flow_types.go            # traffic data structures (shared front/back contract)
├── helper/                  # privileged helper (root/SYSTEM daemon hosting the sing-box kernel)
├── internal/                # infrastructure & business modules (layered; lower never calls upper)
├── configs/                 # default sing-box config template
├── frontend/
│   ├── src/views/           # pages (home/subscriptions/proxies/rules/monitor/connections/settings/source)
│   ├── src/config/          # sing-box config two-way conversion (types/fields/serde)
│   └── bindings/            # auto-generated TS bindings (wails3 generate bindings)
├── build/                   # per-platform build config
├── scripts/release.sh       # one-click release script
└── Taskfile.yml             # build/package tasks
```

## 🔒 Privacy

This tool runs entirely locally; captured plaintext stays in memory and is never sent anywhere. The CA certificate contains a private key and is for local use only — do not commit or share it.

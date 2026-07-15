<div align="right">

[简体中文](FEATURES.md) · **English** · [← README](../README.en.md)

</div>

# Juzheng · Feature Guide

This guide walks through Juzheng's features page by page. For step-by-step instructions, see the [User Guide](USER-GUIDE.en.md).

Main menu order: **Home · Subscriptions · Proxies · Rules · Monitor · Connections · Settings · Source**.

---

## Home

The overview and control center.

- **Kernel switch**: start/stop the sing-box proxy kernel (transparent TUN proxy) with one tap. Start/stop goes through the privileged helper — no repeated authorization.
- **Chain status**: three dots visualize the "kernel → helper → network" readiness.
- **Proxy mode toggle**: switch instantly between Rule / Global / Direct, mapped to `clash_api.default_mode` and `clash_mode` routing rules.
- **Current node**: shows and quickly switches the main proxy group's selected node; supports per-node latency tests (shows "Testing…" while running).
- **LAN proxy service card**: shows the LAN-sharing (public proxy) status — whether it's serving and whether auth is on — and lists the full addresses other devices can use (this machine's LAN IPs : port, click to copy). When off, it points you to Settings to enable it.
- **Traffic stats card**: a dual-series area chart (upload/download speed), with time span switchable across **1 / 5 / 10 / 30 / 60 minutes** (default 10), filterable by proxy group. Hover to see the time and up/down speed at a point. Six metrics below: upload speed, download speed, active connections, total upload, total download, kernel memory usage.

## Subscriptions

Manage airport/self-hosted subscriptions.

- **Import**: paste a subscription URL to import; nodes and proxy groups are fetched and parsed automatically.
- **Update**: refresh subscription content with one tap.
- **Enable/switch**: keep multiple subscriptions; switch the active one with one tap.
- **Rename / Export / Delete**: manage saved subscriptions.

## Proxies

Proxy group and node management.

- **Proxy groups**: shows selector (manual choice) and urltest (auto-speedtest, pick fastest) groups. Expand to view members.
- **Node selection**: switch nodes manually within a selector group.
- **Latency tests**: test a whole group or a single node; a spinner + "Testing" shows at the latency slot while running.
- **Node list**: view all real nodes (system hidden nodes like `to-mitmproxy`/`direct` are not shown).

## Rules

Graphical editing of routing rules (sing-box `route.rules`).

- **Ordered rules**: rules apply top-down — **order is priority**, first match wins.
- **Match types**: `domain_suffix`, `domain_keyword`, `domain`, `domain_regex`, `ip_cidr`, `protocol`, plus **per-process routing**: `process_name` (exact executable-name match, e.g. `git`, `Telegram`), `process_path` (exact full-path match), and `process_path_regex` (**full-path regex — the way to do wildcards**). Process matching relies on `route.find_process`, which the app enables automatically — no configuration needed.
- **Process wildcards**: sing-box has no `process_name_regex`; use `process_path_regex`, which runs a regex *search* (not a full match) over the full path — so a single `UURemote` covers `UURemote`/`UURemoteServer`/`UURemoteUpdater`. Anchor it (e.g. `/UURemote[^/]*$`) when you need to be stricter.
- **Outbound targeting**: each rule targets an outbound (proxy group / node / direct).
- **System-rule fidelity**: MITM decryption, DNS hijack, loopback-break and other system rules are preserved verbatim and never overridden by user rules.
- **Deterministic mapping**: what you see on the Rules page is exactly what the config becomes — a 1:1 serialization with no implicit merging.

## Monitor (Traffic Inspection)

HTTPS plaintext capture and analysis via go-mitmproxy.

- **Start/stop monitoring**: listens on `:9080` by default.
- **CA certificate**: first use requires trusting the built-in CA (one-tap install/copy-command provided); Windows imports it into the Trusted Root store via `certutil`.
- **Request list**: live view of proxied requests — URL, method, status code, request/response plaintext.
- **Upstream proxy**: point at another proxy's inbound (e.g. `127.0.0.1:7890`) to egress through it in a chain.
- **Content-layer rules**: filter which traffic gets decrypted by domain and other conditions.

## Connections

Real-time connection monitoring (Clash API `/connections`).

- **Connection list**: current active connections with target address, rule chains, and up/down traffic.
- **Process name**: with `route.find_process` enabled, each connection can show its originating process.
- **Live refresh**: continuously updated via SSE / polling.

## Settings

Common items — TUN, DNS, ports, logging.

- **TUN**: on/off, address, MTU, stack (system/gVisor), interface name, auto route, strict route.
- **DNS**: multiple DNS servers (address + detour), routing rules (domain → specific DNS), resolution strategy (prefer_ipv4/ipv6, etc.), fallback server.
- **Ports**: mixed-back re-injection port, etc.
- **LAN sharing**: when enabled, exposes a mixed (HTTP+SOCKS5, same port) proxy inbound (bound to `0.0.0.0`, default port 7890) for other devices on the same network; optional username/password auth. Other devices point their proxy at "this machine's LAN IP : port" to egress through this machine (main proxy by default; rule-matched traffic follows the rules). Restart the kernel to apply.
- **Log level**: trace/debug/info/warn/error.

## Source

- **Raw config editing**: view/edit the underlying sing-box JSON directly.
- **Two-way sync**: graphical editing and source mirror each other; a change in one syncs to the other (on parse failure it warns and lets you fix in source mode).
- **Passthrough of non-graphical fields**: advanced fields not covered by the GUI are preserved verbatim and never lost.

---

## System Integration

- **System tray**: resident tray icon that reflects the kernel's running state (on macOS a monochrome Template icon that adapts to light/dark menu bars, distinguishing running/idle by shape).
- **Background resident**: closing the window minimizes to tray; the kernel keeps running.
- **Cross-platform privileged helper**: runs the kernel via launchd on macOS and a system service (SCM) on Windows; Windows bundles `wintun.dll` so TUN works out of the box.

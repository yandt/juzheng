<div align="right">

[简体中文](USER-GUIDE.md) · **English** · [← README](../README.en.md)

</div>

# Juzheng · User Guide

This guide covers how to use Juzheng step by step. For feature details, see the [Feature Guide](FEATURES.en.md).

---

## 1. Installation

### macOS

1. Download the `.dmg` from [Releases](https://github.com/yandt/juzheng/releases).
2. Open the dmg and drag **Juzheng** into Applications.
3. If first launch reports "cannot verify developer", allow it under **System Settings → Privacy & Security**.

### Windows

1. Download the installer for your architecture (`x64` or `ARM64`) from [Releases](https://github.com/yandt/juzheng/releases): NSIS installer or portable zip.
2. Run the installer or unzip the archive.
3. If WebView2 Runtime is missing, the installer will guide you to install it.

---

## 2. First-time Setup

### 1. Import a subscription

1. Open the **Subscriptions** page and click "Import".
2. Paste the subscription URL from your provider and confirm.
3. Nodes and proxy groups are parsed automatically; click "Enable" to make it the active subscription.

> With no subscription, you can also paste a sing-box JSON config manually on the **Source** page.

### 2. Start the kernel

1. Go back to **Home** and tap the kernel switch to start.
2. First start requests admin/system authorization (to install the privileged helper) — once is enough.
3. Once the three chain dots are all ready, system traffic goes through the transparent TUN proxy.

### 3. Pick a node

- Switch the main group's node quickly under "Current node" on **Home**; or
- Expand a proxy group on the **Proxies** page to choose manually or run latency tests to pick the fastest.

---

## 3. Daily Use

### Switch proxy mode

On **Home**, toggle **Rule / Global / Direct**:

- **Rule**: route by rules (recommended — direct domestically, proxied abroad).
- **Global**: all traffic goes through the main proxy outbound.
- **Direct**: everything direct (temporarily bypass proxying while the kernel keeps running).

### Edit routing rules

1. Open the **Rules** page and enter edit mode.
2. Choose a match type (domain suffix / keyword / IP CIDR / GeoSite, etc.), enter the value, and target an outbound.
3. **Order is priority** — reorder as needed; first match wins.

### Configure DNS

Under **Settings → DNS**:

- Add multiple DNS servers, each with a detour (resolve directly or through the proxy);
- Add routing rules (a class of domains → a specific DNS server);
- Set the resolution strategy and fallback server.

---

## 4. Using Traffic Inspection (optional)

For decrypting and analyzing your own device's HTTPS plaintext during debugging.

### 1. Trust the CA certificate (first time only)

1. Open the **Monitor** page and click "Start" (default `:9080`).
2. Click "Install & trust certificate", or copy the command and run it manually:

   **macOS**
   ```bash
   sudo security add-trusted-cert -d -r trustRoot \
     -k /Library/Keychains/System.keychain \
     "$HOME/Library/Application Support/Juzheng/ca/mitmproxy-ca-cert.cer"
   ```

   **Windows**: the app imports it into the Trusted Root store via `certutil` automatically.

### 2. Route traffic through the monitor

Point a target program at the monitor proxy:

```bash
HTTPS_PROXY=http://127.0.0.1:9080 curl https://example.com
```

You'll see the request's URL, status code, and request/response plaintext on the **Monitor** page.

### 3. Chain an upstream proxy (optional)

Enter another proxy's inbound (e.g. `127.0.0.1:7890`) in the "upstream proxy" field; monitored traffic is decrypted first, then egresses through that upstream.

---

## 5. Tray & Background

- Closing the window doesn't quit the app — it **minimizes to the system tray** and the kernel keeps running.
- The tray icon reflects the kernel state (running / idle).
- From the tray menu you can reopen the window or quit the app.

---

## 6. FAQ

**Q: Does debugging/restarting the app drop the network?**
In dev mode (`WAILS_DEV=1`), the app skips stopping the kernel on exit. The kernel runs in a separate helper process and is picked back up seamlessly via helper status on restart — no repeated TUN rebuilds causing network churn.

**Q: Windows says wintun.dll is missing when enabling TUN?**
Recent builds embed `wintun.dll` and write it to the data directory during helper installation. If it's a leftover from an old version, reinstall the helper once.

**Q: Where is the config stored?**
Following each platform's convention `os.UserConfigDir()/Juzheng`: macOS `~/Library/Application Support/Juzheng`, Windows `%AppData%\Juzheng`, Linux `~/.config/Juzheng`. An old `~/.juzheng` is migrated automatically on first run.

**Q: Is captured plaintext sent anywhere?**
No. All plaintext stays in local memory only. The CA certificate contains a private key and is for local use only — do not share it.

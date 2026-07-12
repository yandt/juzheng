<div align="right">

**简体中文** · [English](USER-GUIDE.en.md) · [← README](../README.md)

</div>

# 居正 · 使用说明书

本说明书按操作步骤介绍如何使用居正。功能细节请见[功能说明书](FEATURES.md)。

---

## 一、安装

### macOS

1. 从 [Releases](https://github.com/yandt/juzheng/releases) 下载 `.dmg` 安装包。
2. 打开 dmg,将「居正」拖入「应用程序」。
3. 首次启动若提示「无法验证开发者」,在**系统设置 → 隐私与安全性**中允许打开。

### Windows

1. 从 [Releases](https://github.com/yandt/juzheng/releases) 下载对应架构的安装包(`x64` 或 `ARM64`):NSIS 安装程序或 zip 便携版。
2. 运行安装程序或解压 zip。
3. 若系统缺少 WebView2 Runtime,安装程序会自动引导安装。

---

## 二、首次配置

### 1. 导入订阅

1. 打开**订阅**页,点击「导入」。
2. 粘贴机场提供的订阅 URL,确认导入。
3. 导入后自动解析出节点与代理组;点「启用」将其设为当前生效订阅。

> 无订阅时,也可在**源码**页手动粘贴 sing-box JSON 配置。

### 2. 启动内核

1. 回到**首页**,点击内核开关启动。
2. 首次启动会请求管理员/系统授权(安装特权 helper),授权一次即可。
3. 链路三圆点全部就绪后,系统流量即经 TUN 透明代理。

### 3. 选择节点

- 在**首页**「当前节点」处快速切换主组节点;
- 或在**代理**页展开代理组,手动选择或做延迟测试挑选最快节点。

---

## 三、日常使用

### 切换代理模式

在**首页**切换 **Rule / Global / Direct**：

- **Rule**：按规则分流(推荐,国内直连、国外走代理)。
- **Global**：全部流量走主代理出口。
- **Direct**：全部直连(临时关闭代理效果,内核仍运行)。

### 编辑分流规则

1. 打开**规则**页,进入编辑。
2. 选择匹配类型(域名后缀 / 关键字 / IP 段 / GeoSite 等),填入值,指定出口。
3. 规则**顺序即优先级**,可拖动调整;命中即停。

### 配置 DNS

在**设置 → DNS**：

- 添加多个 DNS 服务器,并为每个指定 detour(走直连还是走代理解析);
- 添加分流规则(某类域名 → 指定 DNS 服务器);
- 设置解析策略与兜底服务器。

---

## 四、使用流量监控(可选)

用于解密分析自己设备的 HTTPS 明文,便于调试。

### 1. 信任 CA 证书(仅首次)

1. 打开**监控**页,点击「启动监控」(默认 `:9080`)。
2. 点击「安装并信任证书」,或复制命令手动执行:

   **macOS**
   ```bash
   sudo security add-trusted-cert -d -r trustRoot \
     -k /Library/Keychains/System.keychain \
     "$HOME/Library/Application Support/Juzheng/ca/mitmproxy-ca-cert.cer"
   ```

   **Windows**：应用会经 `certutil` 自动导入受信任根存储。

### 2. 让流量经过监控

将目标程序指向监控代理:

```bash
HTTPS_PROXY=http://127.0.0.1:9080 curl https://example.com
```

即可在**监控**页看到该请求的 URL、状态码与请求/响应明文。

### 3. 链式上游代理(可选)

在「上游代理」填入另一代理入口(如 `127.0.0.1:7890`),被监控的流量会先解密再经该上游出网。

---

## 五、系统托盘与后台

- 关闭窗口不退出应用,而是**最小化到系统托盘**,内核继续运行。
- 托盘图标反映内核状态(运行 / 待机)。
- 从托盘菜单可重新打开窗口或退出应用。

---

## 六、常见问题

**Q：调试/重启应用会断网吗?**
调试模式(`WAILS_DEV=1`)退出时会跳过停内核,内核在独立 helper 进程中持续运行,重启后经 helper status 无缝接管,不会反复重建 TUN 致网络波动。

**Q：Windows 启用 TUN 提示缺少 wintun.dll?**
新版已内嵌 `wintun.dll`,随 helper 安装自动写入数据目录。若为旧版遗留,重装一次 helper 即可。

**Q：配置存在哪里?**
遵循各平台规范 `os.UserConfigDir()/Juzheng`:macOS `~/Library/Application Support/Juzheng`、Windows `%AppData%\Juzheng`、Linux `~/.config/Juzheng`。旧版 `~/.juzheng` 首次运行会自动迁移。

**Q：抓包明文会外传吗?**
不会。所有流量明文仅留在本地内存中。CA 证书含私钥,仅本机使用,请勿外传。

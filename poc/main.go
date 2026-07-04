// poc: 验证 go-mitmproxy 能解密 HTTPS 流量 + SSE hook
// 不依赖 Wails，独立验证核心能力。
// 用法:
//
//	1. go run ./poc
//	2. 另开终端: curl -x http://127.0.0.1:9080 -k https://example.com
//	3. 观察本程序打印的明文请求
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
)

// captureAddon 只实现关心的 hook，其余靠 proxy.BaseAddon 提供默认空实现。
type captureAddon struct {
	proxy.BaseAddon
}

func (a *captureAddon) Request(f *proxy.Flow) {
	log.Printf("➡️  REQ  %s %s  body=%d bytes", f.Request.Method, f.Request.URL.String(), len(f.Request.Body))
	body := f.Request.Body
	if len(body) > 200 {
		body = body[:200]
	}
	if len(body) > 0 {
		log.Printf("     body预览: %q", string(body))
	}
}

func (a *captureAddon) Response(f *proxy.Flow) {
	log.Printf("⬅️  RESP %d  body=%d bytes", f.Response.StatusCode, len(f.Response.Body))
}

// SSE hook —— 这是 Claude 流式响应的关键
func (a *captureAddon) SSEStart(f *proxy.Flow) {
	log.Printf("🔴 SSE 流开始: %s", f.Request.URL.String())
}
func (a *captureAddon) SSEMessage(f *proxy.Flow) {
	events := f.SSE.Events
	if len(events) > 0 {
		ev := events[len(events)-1]
		log.Printf("🔴 SSE 事件 #%d: event=%s data=%q", len(events), ev.Event, string(ev.Data))
	}
}
func (a *captureAddon) SSEEnd(f *proxy.Flow) {
	log.Printf("🔴 SSE 流结束 (共 %d 个事件)", len(f.SSE.Events))
}

func main() {
	addr := flag.String("addr", ":9080", "代理监听地址")
	upstream := flag.String("upstream", "", "上游代理 (如 127.0.0.1:7890 链式到 Clash)")
	flag.Parse()

	// CA 证书存到固定目录，方便后续信任
	caRoot, err := filepath.Abs(".poc-ca")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(caRoot, 0o755); err != nil {
		log.Fatal(err)
	}

	opts := &proxy.Options{
		Addr:        *addr,
		SslInsecure: true,
		CaRootPath:  caRoot,
	}
	if *upstream != "" {
		opts.Upstream = *upstream
		log.Printf("上游代理: %s", *upstream)
	}

	p, err := proxy.NewProxy(opts)
	if err != nil {
		log.Fatalf("创建代理失败: %v", err)
	}
	p.AddAddon(&captureAddon{})

	go func() {
		time.Sleep(200 * time.Millisecond)
		cert := p.GetCertificate()
		sum := sha256.Sum256(cert.Raw)
		log.Printf("✅ 代理已启动: http://127.0.0.1%s", *addr)
		log.Printf("📜 CA 证书目录: %s", caRoot)
		log.Printf("   颁发者: %s", cert.Issuer.CommonName)
		log.Printf("   指纹(SHA256): %s", hex.EncodeToString(sum[:]))
		log.Printf("   信任命令: sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain %s/mitmproxy-ca-cert.cer", caRoot)
		log.Printf("   测试: curl -x http://127.0.0.1%s -k https://example.com", *addr)
	}()

	if err := p.Start(); err != nil {
		log.Fatalf("代理运行失败: %v", err)
	}
}

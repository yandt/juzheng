//go:build singbox && windows

// run_windows.go：Windows 下 helper 的运行驱动 —— 作为 SCM 服务运行（被 SCM 拉起时），
// 否则回退 console 模式（直接双击/命令行运行，Ctrl+C 退出）。

package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"

	"github.com/zhanghui/juzheng/internal/iproto"
	"golang.org/x/sys/windows/svc"
)

const serviceName = "juzheng-helper"

// handleAdminArgs 处理提权动作参数（由主 app 经 UAC 拉起时传入）：
//   --install [--allow-sid <SID>] → 部署并注册服务
//   --uninstall                   → 停止并删除服务
// 服务由 SCM 正常启动时会带 --allow-sid（非 install/uninstall），此时仅落地到环境变量、
// 供命名管道 SDDL 收紧，随后返回 false 继续正常服务运行。
// 返回 true 表示已处理动作、调用方应直接退出。
func handleAdminArgs() bool {
	args := os.Args[1:]
	action, allowSID := "", ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--install":
			action = "install"
		case "--uninstall":
			action = "uninstall"
		case "--allow-sid":
			if i+1 < len(args) {
				allowSID = args[i+1]
				i++
			}
		}
	}
	if allowSID != "" {
		os.Setenv(iproto.EnvAllowedSID, allowSID)
	}
	switch action {
	case "install":
		if err := runInstall(allowSID); err != nil {
			log.Fatalf("安装失败: %v", err)
		}
		return true
	case "uninstall":
		if err := runUninstall(); err != nil {
			log.Fatalf("卸载失败: %v", err)
		}
		return true
	}
	return false
}

// runServer 判定运行环境：SCM 启动 → svc.Run 走服务协议；否则 console 模式。
func runServer(s *helperState, ln net.Listener, sockPath string) {
	isSvc, err := svc.IsWindowsService()
	if err == nil && isSvc {
		if err := svc.Run(serviceName, &winService{s: s, ln: ln, sockPath: sockPath}); err != nil {
			log.Fatalf("服务运行失败: %v", err)
		}
		return
	}
	// console 模式：Ctrl+C 优雅退出
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	s.serveLoop(ctx, ln, sockPath)
}

// winService 实现 svc.Handler：把 serveLoop 挂到服务生命周期上，响应 Stop/Shutdown。
type winService struct {
	s       *helperState
	ln      net.Listener
	sockPath string
}

func (w *winService) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		w.s.serveLoop(ctx, w.ln, w.sockPath)
		close(done)
	}()

	changes <- svc.Status{State: svc.Running, Accepts: accepted}
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				cancel()
				<-done
				return false, 0
			}
		case <-done:
			return false, 0
		}
	}
}

// helper 与主 app 之间的 IPC 协议（JSON over unix socket）。
//
// 已迁移至 internal/iproto（L0），本文件保留 package main 别名，
// 供 helper/main.go 尚未迁移的代码使用。
package main

import "github.com/zhanghui/juzheng/internal/iproto"

type IPCRequest = iproto.IPCRequest
type IPCResponse = iproto.IPCResponse

const SocketPath = iproto.SocketPath

// config/fields.ts — 协议字段元数据（供 NodeEditor 动态渲染表单）。

export type FieldType = 'string' | 'int' | 'select' | 'password'

export interface FieldDef {
  key: string
  label: string
  type: FieldType
  required?: boolean
  options?: string[]         // select 的枚举
  placeholder?: string
}

// 通用字段（所有协议都有，除了 direct/block/dns 可能没 server）。
const COMMON_FIELDS: FieldDef[] = [
  { key: 'tag', label: '节点名称 (tag)', type: 'string', required: true, placeholder: '如 proxy-node' },
]

// 各协议特有字段。direct/block/dns 无特殊字段。
export const PROTOCOL_FIELDS: Record<string, FieldDef[]> = {
  direct: [],
  block: [],
  dns: [],
  http: [
    { key: 'server', label: '服务器地址', type: 'string', required: true },
    { key: 'server_port', label: '端口', type: 'int', required: true },
    { key: 'username', label: '用户名', type: 'string' },
    { key: 'password', label: '密码', type: 'password' },
  ],
  vmess: [
    { key: 'server', label: '服务器地址', type: 'string', required: true },
    { key: 'server_port', label: '端口', type: 'int', required: true },
    { key: 'uuid', label: 'UUID', type: 'string', required: true },
    { key: 'security', label: '加密方式', type: 'select', options: ['auto', 'none', 'zero', 'aes-128-gcm', 'chacha20-poly1305', 'aes-128-ctr'] },
    { key: 'alter_id', label: 'Alter ID', type: 'int' },
    { key: 'network', label: '传输网络', type: 'select', options: ['tcp', 'ws', 'grpc', 'http2'] },
  ],
  vless: [
    { key: 'server', label: '服务器地址', type: 'string', required: true },
    { key: 'server_port', label: '端口', type: 'int', required: true },
    { key: 'uuid', label: 'UUID', type: 'string', required: true },
    { key: 'flow', label: 'Flow', type: 'string', placeholder: 'xtls-rprx-vision' },
    { key: 'network', label: '传输网络', type: 'select', options: ['tcp', 'ws', 'grpc', 'http2'] },
  ],
  trojan: [
    { key: 'server', label: '服务器地址', type: 'string', required: true },
    { key: 'server_port', label: '端口', type: 'int', required: true },
    { key: 'password', label: '密码', type: 'password', required: true },
    { key: 'sni', label: 'SNI', type: 'string' },
  ],
  shadowsocks: [
    { key: 'server', label: '服务器地址', type: 'string', required: true },
    { key: 'server_port', label: '端口', type: 'int', required: true },
    { key: 'method', label: '加密方法', type: 'select', options: ['2022-blake3-aes-128-gcm', '2022-blake3-aes-256-gcm', '2022-blake3-chacha20-poly1305', 'aes-256-gcm', 'aes-128-gcm', 'chacha20-ietf-poly1305', 'none'] },
    { key: 'password', label: '密码', type: 'password', required: true },
  ],
  hysteria2: [
    { key: 'server', label: '服务器地址', type: 'string', required: true },
    { key: 'server_port', label: '端口', type: 'int', required: true },
    { key: 'password', label: '密码', type: 'password', required: true },
    { key: 'up_mbps', label: '上行 (Mbps)', type: 'int' },
    { key: 'down_mbps', label: '下行 (Mbps)', type: 'int' },
  ],
  wireguard: [
    { key: 'server', label: '服务器地址', type: 'string', required: true },
    { key: 'server_port', label: '端口', type: 'int', required: true },
    { key: 'local_private_key', label: '本地私钥', type: 'password', required: true },
    { key: 'peer_public_key', label: '对端公钥', type: 'string', required: true },
    { key: 'reserved', label: 'Reserved (逗号分隔)', type: 'string', placeholder: '1,2,3' },
  ],
}

// 取某协议的完整字段表（通用 + 协议特有）。
export function fieldsFor(type: string): FieldDef[] {
  return [...COMMON_FIELDS, ...(PROTOCOL_FIELDS[type] ?? [])]
}

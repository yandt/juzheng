// configModel.ts — sing-box 配置双向转换层（barrel 桶文件）。
// 实际实现已拆分到 config/ 子目录，此处统一 re-export，保持既有 import 路径不变：
//   - config/types.ts  结构化数据模型（类型定义 + 常量）
//   - config/fields.ts 协议字段元数据（NodeEditor 动态表单）
//   - config/serde.ts  解析/序列化（JSON ↔ 结构化）
export * from './config/types'
export * from './config/fields'
export * from './config/serde'

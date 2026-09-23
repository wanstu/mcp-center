# 实施计划

## Phase 1：Core 能力稳定

目标：让 `mcp-center-core` 成为 ADM 与 MCP Center 的共同依赖。

- 对齐 ADM MCP Definition 与 Core Definition
- 补齐 Import / Export 与冲突策略
- Runtime Session Manager：Connect / Refresh / Disconnect / tools/list cache
- 统一 Inbound Authorization：OAuth / Bearer / Header / Query -> Principal
- Audit Hook
- Core 单测覆盖 HTTP / stdio / auth / import-export

完成标准：Core 不依赖 Wails、Web 用户、ADM Environment/Writer。

## Phase 2：MCP Center Web MVP

- Bootstrap Admin
- Login / Logout / Change Password
- MCP CRUD
- Probe / tools/list / tools/call
- Access Credential CRUD
- OAuth Client CRUD
- 调用日志
- 系统设置
- SQLite 持久化

完成标准：可以只通过浏览器完成 MCP 的配置、授权、测试和审计。

## Phase 3：ADM 接入支持（仅 Core 侧）

**本阶段不修改 ADM 工程。**

MCP Center Core 侧只需要准备：

1. 稳定公共 API
2. Definition 映射说明
3. Import / Export 兼容说明
4. Runtime 接入示例
5. 版本兼容策略
6. 升级 / Breaking Change 说明

未来 ADM 是否接入、如何接入、何时删除 ADM 内部重复实现，全部由 ADM 方负责。

ADM 自己的 Workspace、Environment selection、DefaultIncludeInEnvironment 语义、Retention、Gateway owner/runtime observation、Writer Lease 和管理权限均不属于本项目。

## Phase 4：产品化

只在确有需求后考虑：

- 普通用户
- 更细 Scope / ACL
- MCP 分组或聚合
- Prompt / Resource
- SSO
- 动态 OAuth Client Registration
- 远程部署与升级

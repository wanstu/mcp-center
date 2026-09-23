# MCP Center 与 mcp-center-core 契约

## 1. 为什么必须拆 Core

ADM 当前 MCP 能力已经包含：

- MCP Definition
- Streamable HTTP / stdio
- Secret-backed Header
- 健康探测
- tools/list
- tools/call
- 批量导入冲突策略
- 配置导出

这些逻辑不应继续只存在 ADM。

目标：

```text
mcp-center-core
   ^              ^
   |              |
ADM            MCP Center
```

ADM 与 MCP Center 都是 Core 的使用方。

## 2. Core 不应该依赖

Core 禁止依赖：

- ADM Workspace
- ADM Environment
- ADM Writer Lease
- ADM Desktop
- Wails
- MCP Center Web 用户界面
- MCP Center SQLite 具体表
- MCP Center 管理员 Session

## 3. Core 应该提供

### Catalog

- Add / Update / Remove / Get / List
- Validate
- Import
- Export
- conflict policy

### Runtime

- Resolve secrets / env refs
- Connect
- Probe
- Refresh
- Disconnect
- ListTools
- CallTool
- Runtime status

### Authorization

- Auth Policy
- Credential verifier interface
- OAuth token verifier interface
- Header / Query extraction
- Principal
- Scope check

### Audit

- Hook / Sink interface
- 结构化调用事件
- 敏感字段剔除

## 4. 当前 Core 实现现状

当前 `mcp-center-core` 已经出现第一版实现：

- `types.go`
- `service.go`
- `runtime.go`
- `store.go`
- `auth/service.go`

其中 MCP Definition 和 Runtime 已明显沿用 ADM 的模型，这是正确方向。

需要调整的边界：

- `auth/service.go` 当前包含 Web 用户、密码和 Session；长期看这部分更偏 MCP Center 应用层。
- Core 可以保留“通用认证 primitives”，但不应让 ADM 因为引用 MCP Core 而被迫带上 Web 管理账号体系。
- OAuth / Header / Query 针对 MCP Endpoint 的统一 Principal / Policy 属于 Core。
- BootstrapAdmin / Login 页面流程属于 Center。

如果当前开发为了快速验证先把 account auth 放 Core，也应在 API 稳定前拆到 Center 或独立 `account` 子模块，并确保 ADM 可不引用。

## 5. ADM 接入边界

本项目**不修改 ADM 工程**。

MCP Center / MCP Center Core 只负责：

1. 保持 Core Definition 与 ADM 现有 MCP 模型具备清晰可映射关系。
2. 提供稳定的 Validate / Resolve / Probe / ListTools / CallTool / Import / Export API。
3. 提供独立的 Core 接入文档、示例和兼容说明。
4. 不引用 ADM Workspace / Environment / Writer / Gateway 等实现。

未来由 ADM 方自行决定接入时机和 Adapter 实现，并自行修改、测试和发布 ADM。

因此，本项目的完成标准不是“ADM 已迁移”，而是“Core 已具备稳定、可独立集成的公共 API”。

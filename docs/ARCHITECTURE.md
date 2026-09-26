# MCP Center 架构

## 1. 总览

```text
Browser
  |
  | Web Session
  v
MCP Center Web/API
  |-- Admin Auth / Bootstrap
  |-- Settings
  |-- Credential Management
  |-- OAuth Authorization Server
  |-- Audit Query
  |
  +---- mcp-center-core
          |-- MCP Catalog
          |-- Runtime / Session
          |-- Probe / tools/list
          |-- tools/call
          |-- Import / Export
          |-- Inbound Authorization Engine
          |-- Audit Event Hooks
          |
          +---- upstream MCPs
```

MCP Center 是“控制程序”，Core 是可复用库。

## 2. 推荐技术栈

- Go 1.26+
- Web：Go HTTP Server + 轻量前端，不强制引入大型 SPA 框架
- 桌面壳（如果需要）：Wails + `wails-desktop-kit`
- 首版本地持久化：SQLite 更适合账号、OAuth Client、Credential、审计日志；Core 的 MCP Catalog 可以先保持 Repository 抽象
- Password Hash：Argon2id
- Access Credential：随机 256-bit secret，保存 SHA-256/HMAC 型指纹，不保存明文
- OAuth：Authorization Code + PKCE，短期 Access Token；Refresh Token 是否启用可作为后续项

## 3. 模块边界

### center/internal/web

- 路由
- HTML / 静态资源
- Web Session Cookie
- CSRF
- 管理 API

### center/internal/account

- BootstrapAdmin
- Login
- Logout
- ChangePassword
- 后续 AddUser

### center/internal/config

- Listen / External URL
- 授权入口开关
- Header 名、Query 参数名
- 日志策略

### center/internal/oauth

负责 UI 与 OAuth Server 的业务配置；令牌验证最终转换为 Core 的统一 Principal。

### center/internal/audit

负责落库和查询。

Core 只产生结构化 Audit Event，不知道 SQLite 表结构，也不知道管理页面。

## 4. MCP 请求入口

建议暴露：

```text
/mcp/{server}
/mcp/{server}/...
```

未来如果需要聚合工具命名空间，再增加：

```text
/mcp
```

Phase 1 不要一开始就做复杂“组 / 路由 / 虚拟 MCP”。

## 5. 上游 MCP

Core 继续支持 ADM 已有两类：

- `streamable-http`
- `stdio`

HTTP 上游继续支持 Secret-backed Headers。

MCP Center 额外提供“上游密钥变量”存储：Definition 仍只保存 `${NAME}` 引用，实际值由 Center 的变量解析器注入 Core。Center-managed 变量优先于系统环境变量，并使用本地随机主密钥 + AES-GCM 加密保存；Core 不依赖该存储实现。

ADM 现有字段应作为迁移兼容基线：

- id
- name
- default_include_in_environment
- transport
- auth_mode
- endpoint
- header_refs
- executable
- args
- env_refs
- health_policy

MCP Center 可以增加产品层字段，例如 description / enabled / tags，但不要反向污染 ADM 必须依赖的 Core 最小 Definition。

## 6. 运行时

Core Runtime 需要从“每次调用创建连接”逐步演进为可复用 Session Manager：

- key：MCP ID
- 显式 Refresh / Disconnect
- tools/list 缓存
- 健康状态
- 超时
- 自动重连
- 配置变化使旧 Session 失效

ADM 当前的 owner-local observation / session 思路可以参考，但 Core 不需要 ADM Environment / Writer 等概念。

## 7. 审计

Core 定义：

```go
type AuditEvent struct {
    RequestID   string
    Source      string
    PrincipalID string
    MCPID       string
    ToolName    string
    StartedAt   time.Time
    Duration    time.Duration
    Status      string
    ErrorKind   string
    InputMeta   map[string]any
    OutputMeta  map[string]any
}
```

MCP Center 注入 Sink：

```go
type AuditSink interface {
    Record(context.Context, AuditEvent)
}
```

原则：

- Core 产生事件。
- Center 持久化。
- ADM 可以选择自己的 Sink。
- 不记录 Access Token / Password / Authorization Header / Query Secret。

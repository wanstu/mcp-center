# MCP Center 产品设计（Phase 0 / 原型）

## 1. 产品定位

MCP Center 是 `mcp-center-core` 的独立控制面板和服务宿主。

它解决四件事：

- 管理上游 MCP：添加、编辑、启停、探测、导入、导出。
- 管理 MCP 工具：查看工具清单、Schema、启用状态、在线测试。
- 管理访问授权：OAuth 2.x、Bearer Token、指定 Header、Query 参数。
- 记录调用审计：谁通过什么授权方式调用了哪个 MCP / Tool，结果、耗时和错误是什么。

不把它做成“大而全”的 MCP 平台。首版不做市场、租户、计费、复杂 RBAC、工作流编排、Prompt/Resource 管理。

## 2. 首版导航

### 工作台

展示：

- MCP 服务总数 / 在线数
- 已发现工具数
- 最近 24h 调用量
- 最近错误
- 快速入口：添加 MCP、导入配置、创建访问密钥

### MCP 服务

列表字段：

- 名称
- 描述
- Transport：Streamable HTTP / stdio
- 状态：在线 / 离线 / 未探测 / 禁用
- 工具数
- 最近探测
- 启用开关
- 操作：详情、测试、导出、编辑、删除

操作：

- 添加
- 导入 JSON
- 导出全部
- 刷新 / 探测全部
- 搜索
- 状态筛选

首版导入兼容优先：

- 通用 `mcpServers` JSON
- ADM 当前 MCP 模型
- MCP Center 自身格式

冲突策略：

- 报错
- 跳过
- 按名称更新

### 工具测试

左侧选择 MCP 与工具，右侧展示：

- 工具描述
- inputSchema
- JSON 参数编辑器
- 调用按钮
- 结果
- 耗时
- 错误分类

测试调用也进入调用日志，并标记 `source=console`。

### 访问授权

分为两块：

**访问密钥**

统一管理一个 Credential 实体，允许配置可用入口：

- Authorization Bearer
- 自定义 Header（例如 `X-MCP-Token`）
- Query 参数（例如 `mcp_token`）

同一密钥可以只允许一种入口，也可允许多种。密钥只在创建时显示一次，持久化仅保存哈希。

**OAuth**

提供标准 OAuth 授权入口，至少覆盖 MCP 客户端常见的 Authorization Code + PKCE。首版 Scope 统一使用 `mcp:*`；细粒度 Scope 后续再开放。

控制台展示：

- Issuer / Authorization Endpoint / Token Endpoint
- OAuth 开关
- Client 列表
- Redirect URI
- Scope
- Client 状态

首版 OAuth Client 由管理员显式创建，不做动态客户端注册。

控制台提供 OAuth 端到端自测：选择一个 MCP 后使用临时 Public Client 完成真实 Authorization Code + PKCE、Token Exchange、MCP initialize / tools/list、Refresh Token 轮换和 Revoke 验证。为避免误操作真实业务 MCP，产品自测不自动执行 tools/call；发布 smoke 使用专用测试 MCP 继续覆盖 tools/call。

### 调用日志

结构化记录：

- 时间
- Request ID
- 调用来源：OAuth / Bearer / Header / Query / Console
- Principal / Credential ID（不记录密钥）
- MCP
- Tool
- 状态
- 耗时
- 错误分类
- 请求参数摘要
- 结果摘要

默认不把完整敏感参数直接写入日志；提供字段脱敏和最大体积限制。

### 用户

首版只需要：

- 当前管理员信息
- 修改密码
- 退出登录

预留“创建普通账号”，但 Phase 0 不实现。

### 设置

首版只保留：

- Listen Address / Port
- 外部 Base URL
- OAuth 开关
- Bearer 开关
- Header 开关 + Header Name
- Query 开关 + Query Parameter Name
- 日志保留天数
- 请求 / 结果最大日志体积

## 3. 首次初始化与登录

当系统中没有用户：

1. Web 访问 `/setup`。
2. 显示“创建第一个管理员”。
3. 创建成功后立刻关闭 Bootstrap 注册。
4. 后续 `/setup` 返回已初始化，不允许继续注册。

当已有管理员：

- 未登录访问管理页跳到 `/login`。
- 登录成功建立 Web Session。
- Web Session 与 MCP API Access Credential 不共用 Token。

## 4. 原型说明

`prototype/index.html` 是纯静态原型，用于确认信息架构和交互，不代表最终 API。

原型使用与 Wails Desktop Kit 一致的 `--dk-*` token、侧边栏、panel、button、badge、switch 等视觉语言，后续落实现时可以直接换成 Kit 挂载资源。

## 5. 明确不做

首版不做：

- 多租户
- 复杂角色权限
- 计费
- MCP 市场
- Prompt / Resource 管理
- MCP 分组编排
- 智能路由
- 工具结果压缩
- 云同步
- 动态 OAuth Client Registration
- SSO / LDAP

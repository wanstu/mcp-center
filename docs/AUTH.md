# MCP Center 认证与授权设计

## 1. 两套认证必须分开

### 管理后台账号

用于登录 MCP Center 控制台：

- username + password
- Web Session Cookie
- 首个用户 Bootstrap 为 admin

### MCP 调用授权

用于外部 MCP Client 请求 MCP Endpoint：

- OAuth Bearer Token
- Static Bearer Token
- 自定义 Header
- Query 参数

两者不能复用同一个 Session Token。

## 2. Bootstrap 管理员

状态机：

```text
users == 0
  -> /setup 可用
  -> 创建 admin
  -> users > 0
  -> /setup 永久关闭公开注册
```

要求：

- 数据库层事务保证“第一个管理员只能创建一次”
- 用户名大小写唯一
- 密码最少 10 位（最终实现建议同时做常见弱密码检查）
- Password 使用 Argon2id
- Session Cookie: HttpOnly + SameSite=Lax；HTTPS 下 Secure
- 修改密码后可使旧 Session 全部失效

首版只实现 admin。

## 3. 统一 Access Credential

建议数据结构：

```text
AccessCredential
- id
- name
- secret_hash
- secret_prefix
- enabled
- allow_bearer
- allow_header
- allow_query
- scopes
- expires_at
- created_at
- last_used_at
```

创建后只显示一次原始 secret。

验证结果统一为：

```text
Principal
- id
- kind: oauth | access_credential | console
- display_name
- scopes
- auth_channel: oauth | bearer | header | query | console
```

后续 MCP Runtime 只看 Principal，不关心 secret 来自哪里。

## 4. Bearer

请求：

```http
Authorization: Bearer mcp_xxx
```

系统设置可开关。

Static Bearer 与 OAuth Bearer 的区分流程：

1. 读取 Bearer。
2. 先按 OAuth Token 格式 / Token Store 验证。
3. 未匹配时再按 Access Credential 验证。
4. 两者都失败返回 401。

不要通过明文前缀泄露令牌类型也可以；实现时可通过哈希索引提高查找效率。

## 5. 自定义 Header

默认建议：

```http
X-MCP-Token: mcp_xxx
```

管理员可以改 Header Name。

开关关闭时，即使请求带 Header 也不参与授权。

不要允许配置成：

- Authorization
- Cookie
- Host
- Content-Length
- 其他 hop-by-hop / 敏感保留 Header

避免和协议本身冲突。

## 6. Query 授权

示例：

```text
/mcp/my-server?mcp_token=mcp_xxx
```

必须默认关闭。

原因：

- URL 可能进入浏览器历史
- Reverse Proxy Access Log
- CDN / APM
- Referer
- 截图或错误报告

启用时：

- UI 显示明确警告
- Query secret 在进入普通 access log 前立即脱敏
- MCP Center 自己的 Audit Log 只记录 Credential ID
- 禁止把完整 URL 原样写到日志

## 7. OAuth

Phase 1 最小支持：

- Authorization Code
- PKCE S256
- Access Token
- Scope
- Client 管理
- Redirect URI 精确匹配

建议端点：

```text
/.well-known/oauth-authorization-server
/oauth/authorize
/oauth/token
/oauth/revoke
```

如果 MCP Client 生态要求 Protected Resource Metadata，再增加：

```text
/.well-known/oauth-protected-resource
```

OAuth Client 首版由管理员创建：

- client_id
- name
- redirect_uris
- scopes
- enabled

Public Client 默认不发 client_secret，依赖 PKCE。

## 8. 授权顺序

建议固定顺序，避免多入口歧义：

```text
Authorization Bearer
  -> OAuth
  -> Static Access Credential

Custom Header
  -> Static Access Credential

Query
  -> Static Access Credential
```

如果一个请求同时携带多个不同 Credential，直接拒绝为 `ambiguous_credentials`，而不是猜一个。

## 9. Scope

首版可以简单：

- `mcp:read`：initialize / tools/list / resources read 等只读协议操作
- `mcp:call`：tools/call
- `mcp:*`：全部

后续再增加按 MCP / Tool 的细粒度限制。不要首版就做复杂 ACL。

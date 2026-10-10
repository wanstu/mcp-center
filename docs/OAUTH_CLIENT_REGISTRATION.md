# OAuth 自动客户端注册与 ChatGPT MCP 连接

MCP Center 为 MCP 资源提供受保护资源元数据、OAuth 授权服务器发现、
Authorization Code + PKCE S256、Refresh、Revoke。原先只有管理员创建
OAuth Client 的方式，导致 ChatGPT 自定义 MCP 连接页面要求手动填写 Client ID。

从此改动起，MCP Center 新增 RFC 7591 **Dynamic Client Registration (DCR)**：

- `GET /.well-known/oauth-authorization-server` 公开
  `registration_endpoint: <公开地址>/oauth/register`；
- `POST /oauth/register` 允许 OAuth 公共客户端自动注册，返回
  `201`、`client_id`、`client_id_issued_at`、
  `redirect_uris`、`grant_types`、`response_types`、
  `token_endpoint_auth_method: none`；
- 只支持 public client，不发放 `client_secret`；
- 注册请求限流，每个直接来源地址每小时最多 40 次注册请求；
  单实例客户端数量限制 1500，单次 JSON 请求限制 16 KiB；
- 注册信息不提供任何访问权限。用户仍需要登录、明确授权、PKCE S256，
  令牌与具体 MCP Resource URL 绑定；
- 新增同意页上显示客户端真实回调地址的提示，避免仅凭客户端自报名称判断可信度；
- 原有管理后台创建、修改、删除 OAuth Client 的 API 不变。

## ChatGPT 连接流程

1. 部署启用此改动的 MCP Center 版本；
2. 向 ChatGPT 提供 `https://example.com/mcp/<name-or-id>`；
3. ChatGPT 查询 MCP 资源元数据和授权服务器元数据；
4. ChatGPT 自动请求 `POST /oauth/register`，取得 Client ID，
   通常无需在高级 OAuth 设置手动填写；
5. ChatGPT 第一次调用工具时，用户登录 MCP Center 并批准访问，
   然后完成 PKCE 和 Access Token 交换。

使用 HTTPS 反向代理时，**一定要确认发现接口返回的 issuer、authorization_endpoint、
token_endpoint、registration_endpoint 都是浏览器可访问的 HTTPS 外部地址**。
若反向代理在内部终止 TLS，必须配置 MCP Center 的 `--base-url`
为公开 HTTPS 基础地址；服务不会盲目信任可能被伪造的 `X-Forwarded-Host`。
例如 `--base-url https://mcp.example.com`。

使用 curl 查看（不需要登录）：

```bash
curl -fsS https://mcp.example.com/.well-known/oauth-authorization-server
curl -fsS https://mcp.example.com/.well-known/oauth-protected-resource/mcp/example
```

DCR 兼容性根据 OpenAI MCP 文档、RFC 7591 和本地集成测试实现；
具体 ChatGPT UI 最终需部署后在真实 ChatGPT 新连接流程验证。
本改动没有修改 MCP Center Core、Kit 或 ADM。

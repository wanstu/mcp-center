# MCP Center

MCP Center 是 `mcp-center-core` 的独立 Web 控制程序。

当前开发边界：

- 本仓库只负责 MCP Center 控制程序。
- MCP 协议与复用能力放在 `mcp-center-core`。
- 不修改 AI Dev Manager；未来由 ADM 方自行接入 Core。

## 当前 MVP

已实现：

- 首次访问创建管理员，随后关闭公开注册
- 管理员登录 / Session / 退出
- MCP Definition CRUD
- Streamable HTTP / stdio
- MCP Probe
- tools/list
- tools/call
- 上游 MCP 代理为 `/mcp/{id}`
- OAuth Authorization Code + PKCE S256
- OAuth Metadata / Protected Resource Metadata
- MCP Token 与具体 Resource 绑定
- Wails Desktop Kit Web UI 资源与主题
- 可运行 CLI 服务入口
- Static Access Credential：Bearer / Header / Query
- 授权通道开关与 Header / Query 参数名配置
- MCP 工具调用审计：Request ID / Principal / 授权通道 / MCP / Tool / 耗时 / 错误分类
- Audit JSONL 持久化与控制台查询/清空
- MCP 导入 / 导出：MCP Center 与通用 mcpServers 格式
- 导入冲突策略：报错 / 跳过 / 按名称更新，完整预检后原子写入
- Runtime Session Manager：复用上游 Session、tools/list 缓存、刷新/断开、运行时状态
- 配置更新 / 删除 / 导入后自动失效旧 Session
- 管理员账号设置与修改密码；改密后旧 Session 全部失效并自动换发当前 Session
- OAuth Client 管理：管理员显式创建 / 编辑 / 删除，Public Client + PKCE
- OAuth Client 修改 / 删除会撤销已有授权码与 Token；不开放动态客户端注册
- OAuth Token 主动撤销：`/oauth/revoke`，Access / Refresh Grant 一并失效
- 上游密钥变量：供 MCP Header / stdio Env 中的 `${NAME}` 引用
- 上游密钥使用本地随机主密钥 + AES-GCM 加密保存；列表和 API 不回显明文
- 上游密钥替换 / 删除会断开现有 MCP Session，下次连接使用最新值
- 工作台 24h 调用量与最近错误汇总
- MCP 服务状态筛选：全部 / 已连接 / 未连接 / 异常
- 审计日志保留策略：默认 30 天，可在设置页修改；保存后立即清理过期日志

开发中：

- 更完整设置页面

## 本地启动

```powershell
go test ./...
go run ./cmd/mcp-center --listen 127.0.0.1:8610 --base-url http://127.0.0.1:8610
```

浏览器访问：

```text
http://127.0.0.1:8610/
```

查看当前版本：

```powershell
go run ./cmd/mcp-center --version
```

正式构建可通过 `-ldflags` 注入版本号；未注入时显示 `dev + Git commit`。

RC / Release 构建：

```powershell
./scripts/build-release.ps1 -Version v0.1.0-rc.1
```

详见 `docs/RELEASE.md`。

默认数据目录使用系统用户配置目录下的 `mcp-center`。开发时可通过 `--data-dir` 指定其他目录。

## 项目结构

```text
cmd/mcp-center/       CLI / Web 服务启动入口
internal/server/      MCP Center HTTP API、OAuth、MCP Proxy
web/                  正式 Web UI
prototype/            早期高保真原型
docs/                 产品与技术设计
assets.go             Web + Desktop Kit 资源挂载
```

## 依赖

本地开发目前使用：

- `../mcp-center-core`
- `../wails-desktop-kit`

通过 `go.mod replace` 引用，便于两个项目同步开发。

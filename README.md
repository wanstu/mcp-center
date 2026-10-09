# MCP Center

> License: **BSD 3-Clause** (SPDX: `BSD-3-Clause`). Copyright (c) 2026 wanstu.
> See [LICENSE](LICENSE). This applies to MCP Center's own source, not to
> third-party packages in the vendor directory, which retain their own terms.

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
- 上游 MCP 代理同时支持 `/mcp/{name}` 与 `/mcp/{id}`
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
- OAuth 端到端自测：临时 Client + PKCE → 浏览器授权 → Token → MCP tools/list → Refresh → Revoke → 自动清理
- OAuth 服务默认可用，不再提供独立的服务开关；OAuth Discovery / Authorize / Token / Revoke / 自测和管理控制台统一遵守可选的 Host 域名/IP 白名单；OAuth Client、PKCE、Token 认证仍然独立执行
- OAuth Token 主动撤销：`/oauth/revoke`，Access / Refresh Grant 一并失效
- 上游密钥变量：供 MCP Header / stdio Env 中的 `${NAME}` 引用
- 上游密钥使用本地随机主密钥 + AES-GCM 加密保存；列表和 API 不回显明文
- 上游密钥替换 / 删除会断开现有 MCP Session，下次连接使用最新值
- 工作台 24h 调用量与最近错误汇总
- MCP 服务启用 / 禁用：保留配置但可临时下线；禁用后工具测试、Runtime、OAuth 与外部代理统一拒绝访问
- MCP 服务状态筛选：全部 / 已连接 / 未连接 / 异常 / 已禁用
- 审计日志保留策略：默认 30 天，可在设置页修改；保存后立即清理过期日志
- 外部 MCP 接入地址总览：同一 MCP 同时支持 `<base-url>/mcp/{name}` 与 `<base-url>/mcp/{id}`；Name 地址易读，ID 地址稳定，均支持 OAuth / Static Credential
- 工具测试参数助手：从 inputSchema 自动生成最小 JSON 示例，并提供 Form / JSON 双模式
- 工具测试状态按 MCP + Tool 保留参数、模式和最后一次运行结果，切换后可恢复

正式版本：`v0.1.1`；当前升级候选版：`v0.1.2-rc.3`。使用统一 Kit `v0.11.3`，并修复 `mcp-center -h`、`--help` 及 `help` 命令帮助。

## 开源许可证

MCP Center 和独立的 MCP Center Core 均使用 **BSD 3-Clause License**
（SPDX: `BSD-3-Clause`，Copyright (c) 2026 wanstu）。详见本仓库
[`LICENSE`](LICENSE) 和 Core 的 `LICENSE` 文件。

Linux `.deb` 包会安装版权/许可证声明至
`/usr/share/doc/mcp-center/copyright`，并通过 AppStream 声明
`project_license=BSD-3-Clause`。
其中 AppStream XML 自身采用 `CC0-1.0`，仅适用于元数据文件，
**不改变软件的 BSD-3-Clause 授权**。第三方依赖仍遵守其各自的许可证。


## 本地启动

```powershell
go test ./...
go run ./cmd/mcp-center
```

浏览器访问：

```text
http://127.0.0.1:8610/
```

默认监听 `0.0.0.0:8610`，无需配置域名或 IP；可通过 `http://<服务器IP>:8610/` 或解析到服务器的域名直接访问（需确保防火墙允许端口）。

在 **设置 → 访问地址白名单** 可以单独启用域名/IP 白名单，默认关闭。这里检查请求访问的 **Host 域名/IP**，不是客户端来源 IP；白名单每行一个域名或服务器 IP，不包含端口/协议，保存后立即生效。登录与 MCP 访问密钥认证仍会独立执行。

外部 MCP 链接以及 OAuth Metadata / Authorize / Token / Resource 默认使用当前请求的域名/IP，不需要单独设置 OAuth 开关或 `--base-url`。**出于 OAuth 令牌安全要求，非本机资源必须使用 HTTPS**；HTTP OAuth 仅支持 `localhost` / 回环 IP。若通过反向代理终止 HTTPS，可以按需用 `--base-url` 固定公开的 HTTPS 地址。白名单只限制请求 Host，不代表跳过 OAuth 登录、客户端授权和 Token 校验。

查看当前版本：

```powershell
go run ./cmd/mcp-center --version
```

正式构建可通过 `-ldflags` 注入版本号；未注入时显示 `dev + Git commit`。

RC / Release 构建：

```powershell
./scripts/build-release.ps1 -Version v0.1.2-rc.2
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

## 依赖与源码分发

- `mcp-center-core` 保持独立项目边界；本地联调通过 `replace ../mcp-center-core` 使用同级源码。
- `wails-desktop-kit` 的 Go Module 和 Linux `.deb` 打包工具统一使用正式 `v0.11.3`。打包直接运行该 Go Module 的 `cmd/desktopkit`，不依赖本地 Kit 工作区或双重版本设置；manifest 记录 Kit module version 和 go.sum 哈希。
- 正式 Release 会额外生成 `mcp-center-<version>-source.zip`，其中包含 Center、Core 与完整 vendor。
- source bundle 会在 `GOPROXY=off`、隐藏 Core sibling 的条件下执行 `go test -mod=vendor ./...` 与 `go build -mod=vendor`，确认程序可离线构建。Linux `.deb` 统一使用 Kit `v0.11.3` 的 Go Module 打包器。

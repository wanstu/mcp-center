# MCP Center Release

## RC 构建

发布构建要求：

- `mcp-center` 工作区必须 clean。
- 同级目录存在 `../mcp-center-core` 与 `../wails-desktop-kit`。
- 当前源码仍通过 `go.mod replace` 使用这两个本地仓库。
- 发布前必须先通过 `go test ./...`。

PowerShell：

```powershell
./scripts/build-release.ps1 -Version v0.1.0-rc.1
```

产物写入：

```text
dist/<version>/
```

默认构建：

- windows/amd64
- windows/arm64
- linux/amd64
- linux/arm64
- darwin/amd64
- darwin/arm64

每个构建会注入 Version、Git commit、UTC build time，并生成 `SHA256SUMS.txt` 与 `release-manifest.json`。

## 验证

Windows：

```powershell
./dist/v0.1.0-rc.1/mcp-center-v0.1.0-rc.1-windows-amd64.exe --version
```

启动：

```powershell
./dist/v0.1.0-rc.1/mcp-center-v0.1.0-rc.1-windows-amd64.exe --listen 127.0.0.1:8610 --base-url http://127.0.0.1:8610
```

检查：

```text
GET /api/version
GET /api/setup
GET /.well-known/oauth-authorization-server
```

## 当前源码发布限制

目前 `mcp-center` 与 `mcp-center-core` 都还没有 Git remote，且 Center 通过本地 `replace` 引用 Core 与 Desktop Kit。

因此当前 RC 可以稳定发布二进制，但源码 tag 还不能作为“clone 后独立构建”的正式发布方式。

正式 Release 前建议：

1. 为 `mcp-center-core` 配置远端并发布对应版本。
2. 为 `wails-desktop-kit` 使用已发布版本。
3. 移除 `mcp-center/go.mod` 中的本地 `replace`。
4. 再发布可复现的源码 Release。

# MCP Center Release

## 正式构建

发布构建要求：

- `mcp-center` 工作区 clean。
- 同级目录存在并保持 clean 的 `../mcp-center-core`。
- Desktop Kit 使用已发布 Go module `github.com/wanstu/wails-desktop-kit v0.10.0`，不要求本地 Kit 仓库。
- 发布前必须同时通过 `mcp-center` 与 `mcp-center-core` 的 `go test ./...`。

PowerShell：

```powershell
./scripts/build-release.ps1 -Version v0.1.0
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

每个构建会注入 Version、Git commit、UTC build time，并生成：

- 六个平台二进制
- `mcp-center-<version>-source.zip`
- `SHA256SUMS.txt`
- `release-manifest.json`

Manifest 记录 Center / Core 源码 commit，以及 Desktop Kit module 版本和 checksum。

## Source bundle

`scripts/build-source-bundle.ps1` 会：

1. 从 Center/Core clean HEAD 使用 `git archive` 导出源码；
2. 在 Center 中执行 `go mod vendor`；
3. 打包 Center、Core、vendor、`BUILD.md`、`SOURCE-MANIFEST.json`；
4. 解压到全新临时目录；
5. 隐藏 sibling Core 目录；
6. 设置 `GOPROXY=off` 与 `GOFLAGS=-mod=vendor`；
7. 执行 `go test ./...` 和 `go build ./cmd/mcp-center`。

因此正式 source.zip 不依赖本机 `D:\projects` 布局，也不依赖网络下载模块。

## Release smoke

Windows 发布机会使用最终构建出的 Windows 二进制运行 `scripts/smoke-release.ps1`：

- HTTP MCP：`tools/list + tools/call`
- OAuth：Authorization Code + PKCE、Bearer MCP 访问、Refresh、Revoke
- stdio MCP：`tools/list + tools/call`

OAuth smoke 使用独立 Go verifier 进程，避免 PowerShell 子脚本提前结束发布流程。

全部通过后，发布脚本才会继续生成 source bundle、checksum 和 manifest。

## 验证

Windows：

```powershell
./dist/v0.1.0/mcp-center-v0.1.0-windows-amd64.exe --version
```

启动：

```powershell
./dist/v0.1.0/mcp-center-v0.1.0-windows-amd64.exe --listen 127.0.0.1:8610 --base-url http://127.0.0.1:8610
```

检查：

```text
GET /api/version
GET /api/setup
GET /.well-known/oauth-authorization-server
```

源码包离线验证：

```powershell
Expand-Archive ./dist/v0.1.0/mcp-center-v0.1.0-source.zip ./source
cd ./source/mcp-center-v0.1.0-source/mcp-center
$env:GOPROXY="off"
go test -mod=vendor ./...
go build -mod=vendor ./cmd/mcp-center
```

## 当前发布边界

- Center 与 Core 目前都没有配置 Git remote，因此正式版本仍以本地 tag + 本地发布目录为准。
- Core 继续作为独立项目维护，不合并进 Center。
- 不修改 ADM；未来由 ADM 方自行决定如何接入 Core。

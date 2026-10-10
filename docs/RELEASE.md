# MCP Center Release

## 正式发布来源

正式 GitHub Release 由 GitHub Actions 构建和发布：

- CI：`.github/workflows/ci.yml`
  - `master` push / Pull Request 自动执行。
  - Windows 与 Linux Runner 都测试 MCP Center。
  - 明确 checkout 对应版本的 `mcp-center-core`，不依赖开发机上的 sibling 目录。
  - 同时执行 Core 测试、Center 测试、`go vet` 与 Web UI smoke。
- Release：`.github/workflows/release.yml`
  - 新的 `v*` tag 自动触发。
  - 已存在但尚未由 CI 发布的 tag，可通过 `workflow_dispatch` 指定 Center tag、版本和 Core tag 重建。
  - 正式产物只使用 GitHub Runner 从 tag 源码重新构建的结果，不上传开发机 `dist/` 目录中的二进制。
  - 已存在的 Release 不允许覆盖附件；同一 tag 的正式产物发布后保持不可变，需要修复时发布新版本号。

当前 Center 使用 MCP Center Core `v0.1.1`。本地 `go.mod` 仍保留 sibling `replace ../mcp-center-core` 以方便 Center/Core 联调；GitHub Actions 会显式 checkout `v0.1.1` 到 sibling 路径，因此 CI 构建不依赖某台开发机的目录内容。

## 本地发布验证

本地脚本继续保留，用于 RC 验收、发布前验证和 CI 使用：

```powershell
./scripts/build-release.ps1 -Version v0.1.3
```

本地运行要求：

- `mcp-center` 工作区 clean。
- 同级目录存在且 clean 的 `../mcp-center-core`，并应 checkout 到目标 Core 版本。
- Go Module 和 Debian 打包器使用正式 Kit `v0.11.3`。
- 发布前同时通过 Center/Core 的 `go test ./...`。

产物写入：

```text
dist/<version>/
```

默认生成：

- Windows amd64 / arm64
- Linux amd64 / arm64
- macOS amd64 / arm64
- Linux amd64 / arm64 systemd `.deb`
- `LICENSE`
- self-contained source ZIP
- `SHA256SUMS.txt`
- `release-manifest.json`

本地 `dist/` **不是正式 GitHub Release 的上传来源**。正式 Release 必须由 GitHub Actions 对 tag 重新构建。

## Release smoke

`scripts/build-release.ps1` 会在 Windows Runner 上对最终 Windows 二进制执行：

- CLI help
- 自动定时探测
- HTTP MCP `tools/list + tools/call`
- OAuth Authorization Code + PKCE
- Bearer MCP 访问
- Refresh / Revoke
- stdio MCP `tools/list + tools/call`

全部通过后才生成 source bundle、checksum 和 manifest。

## Source bundle

`scripts/build-source-bundle.ps1` 会：

1. 从 Center/Core clean HEAD 使用 `git archive` 导出源码；
2. 在 Center 中执行 `go mod vendor`；
3. 打包 Center、Core、vendor、`BUILD.md`、`SOURCE-MANIFEST.json`；
4. 解压到全新临时目录；
5. 隐藏 sibling Core；
6. 设置 `GOPROXY=off` 与 `GOFLAGS=-mod=vendor`；
7. 执行 `go test ./...` 和 `go build ./cmd/mcp-center`。

因此 source ZIP 可以脱离开发机目录布局并在无模块网络条件下验证。

## GitHub Actions 发布流程

未来正常发布：

```text
开发 / PR
→ CI
→ master
→ 创建并 push v* tag
→ GitHub Actions checkout Center tag
→ checkout 固定 Core tag
→ test / vet / UI smoke
→ build-release.ps1
→ release smoke
→ SHA256 校验
→ GitHub Release
```

对于在 CI/CD 引入之前已经存在的 tag，例如 `v0.1.3`：

1. 在 GitHub Actions 选择 `Release` workflow。
2. 使用 `Run workflow`。
3. `ref=v0.1.3`
4. `version=v0.1.3`
5. `core_ref=v0.1.1`

workflow 会验证版本 tag 必须指向 checkout 的源码 commit，然后重新构建。不会读取或上传开发机上的历史 `dist/v0.1.3`。

## BSD-3-Clause

MCP Center 与 MCP Center Core 均使用 BSD 3-Clause License。

Linux Debian 包由 Wails Desktop Kit `v0.11.3` 的 systemd 打包能力生成。安装后：

- 二进制：`/usr/bin/mcp-center`
- systemd unit：`/lib/systemd/system/mcp-center.service`
- 数据：`/var/lib/mcp-center`

升级不会覆盖持久化业务数据。

## 安装示例

```bash
sudo apt install ./mcp-center-v0.1.3-linux-amd64.deb
mcp-center status
mcp-center doctor
```

首次公网注册管理员之前，应先限制访问并配置 HTTPS。

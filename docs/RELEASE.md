# MCP Center Release

## 正式构建

发布构建要求：

- `mcp-center` 工作区 clean。
- 同级目录存在并保持 clean 的 `../mcp-center-core`。
- Go Module 和 Debian 打包器都使用正式 Kit `v0.11.3`，以 `go.mod` 锁定、在 `release-manifest.json` 记录模块版本和校验和。无需切换本地 Kit 仓库，也不会打包未提交的本地 Kit 修改。
- 发布前必须同时通过 `mcp-center` 与 `mcp-center-core` 的 `go test ./...`。

PowerShell：

```powershell
./scripts/build-release.ps1 -Version v0.1.2
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
- 两个架构的 Linux systemd `.deb`
- `LICENSE`（BSD-3-Clause）
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
- 对最终构建出的可执行文件验证 `-h`、`--help`、`help`、`serve --help`，确保帮助中包含服务管理和端口选项。

OAuth smoke 使用独立 Go verifier 进程，避免 PowerShell 子脚本提前结束发布流程。

全部通过后，发布脚本才会继续生成 source bundle、checksum 和 manifest。

## 验证

Windows：

```powershell
./dist/v0.1.2/mcp-center-v0.1.2-windows-amd64.exe --version
```

启动：

```powershell
./dist/v0.1.2/mcp-center-v0.1.2-windows-amd64.exe --listen 127.0.0.1:8610 --base-url http://127.0.0.1:8610
```

检查：

```text
GET /api/version
GET /api/setup
GET /.well-known/oauth-authorization-server
```

源码包离线验证：

```powershell
Expand-Archive ./dist/v0.1.2/mcp-center-v0.1.2-source.zip ./source
cd ./source/mcp-center-v0.1.2-source/mcp-center
$env:GOPROXY="off"
go test -mod=vendor ./...
go build -mod=vendor ./cmd/mcp-center
```

## 当前发布边界

- Center 与 Core 目前都没有配置 Git remote，因此正式版本仍以本地 tag + 本地发布目录为准。
- Core 继续作为独立项目维护，不合并进 Center。
- 不修改 ADM；未来由 ADM 方自行决定如何接入 Core。

## BSD-3-Clause 开源许可证

MCP Center 与独立的 MCP Center Core 均使用 **BSD 3-Clause License**，
版权声明见各仓库根目录 `LICENSE`。

正式版发布目录包含 `LICENSE`；源码 ZIP 分别包含 Center、Core 的
`LICENSE`。Linux Debian 安装包会将完整许可证安装至
`/usr/share/doc/mcp-center/copyright`，并在 AppStream 元数据中声明
`project_license=BSD-3-Clause`。AppStream XML 文件自身采用
`CC0-1.0`，仅为了允许软件中心收录元数据，不改变软件许可证。
第三方依赖仍适用各自的许可证。

软件中心对来源可信度的提示和项目许可证是两回事；
添加许可证不等于系统已验证该安装包的来源。

## Linux Debian 安装与开机自启

本次发布同时生成 AMD64 和 ARM64 的 .deb 安装包：

    mcp-center-<version>-linux-amd64.deb
    mcp-center-<version>-linux-arm64.deb

由 Wails Desktop Kit `v0.11.3` 通用 systemd 打包能力生成。Go Module 和 Linux 打包器使用同一个固定版本，manifest 记录模块版本及校验和；不依赖 GTK、WebKit 或其他桌面组件。

Debian 12 安装示例：

    sudo apt install ./mcp-center-v0.1.2-linux-amd64.deb
    mcp-center status
    mcp-center doctor

安装到 /usr/bin/mcp-center；systemd unit 在
/lib/systemd/system/mcp-center.service；数据在
/var/lib/mcp-center（0700），安装时创建独立低权限 mcp-center 用户。
首次安装自动 enable --now，异常退出会重启。默认监听 0.0.0.0:8610。

升级：sudo apt install ./mcp-center-新版本-linux-amd64.deb。
如果服务原本运行则自动重新启动；不影响持久化业务数据。

卸载：sudo apt remove mcp-center。停止并禁用服务，保留
/var/lib/mcp-center 下的账号、配置、数据和密钥；purge 不删除业务数据。
如果用户确实要清空，需要单独确认并手动清理。

管理系统服务不需要直接调用 systemctl：

    mcp-center -h
    mcp-center status
    mcp-center doctor
    sudo mcp-center start
    sudo mcp-center stop
    sudo mcp-center restart
    sudo mcp-center autostart on
    sudo mcp-center autostart off
    sudo mcp-center set-port 8620

端口写入 `/etc/default/mcp-center` 并在服务重新启动后生效，升级不覆盖配置。原有 `mcp-center serve --listen 0.0.0.0:8610` 仍支持手动前台启动，但不管理已安装服务。

首次公网注册管理员之前，请先限制访问端口并配置 HTTPS。
访问地址白名单检查请求的 Host（访问域名/IP），不检查访客来源 IP，
也不会代替登录、OAuth Client 或 Token 校验。

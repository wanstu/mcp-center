# MCP Center：从其他客户端导入 MCP

MCP Center 的 MCP 服务页已具备“导入”入口。本次参照 ADM 的导入体验，
为该入口增加**识别来源、只读预览、逐项勾选、安全变量转换和确认导入**。

## 支持的来源

- MCP Center 自身 `{"mcps":[...]}` JSON；
- 通用 `{"mcpServers":{...}}`（Claude Desktop / Codex 插件等常见形态）；
- OpenCode：`{"mcp":{"servers":{...}}}`，支持 local command 数组、remote URL；
- WorkBuddy：`mcpServers` + 类型及扩展字段；
- Codex/插件：直接 MCP 命名对象或 `mcpServers`；
- Claude Code：顶层 `mcpServers` 或 `projects.<路径>.mcpServers`；
- MCPHub：`servers` 或 `mcpServers`。

可自动识别来源格式，也可手选。支持 JSON/JSONC 注释与末尾逗号；
会拒绝 JSON 重复键和大小写不敏感的重复 MCP 名。

## 操作步骤

1. 打开“**MCP 服务 → 导入**”。
2. 粘贴 JSON/JSONC 或选择文件，格式保持“自动识别”。
3. Claude Code 文件若有多个 project，在“源项目路径”中填写对应项目的完整键。
4. 选择名称冲突策略（报错/跳过/按名称更新），点击“预览配置”。
5. 审查每个候选项的传输方式、是否已存在、错误、忽略字段警告、待配置密钥变量；
   取消勾选不需要的项。
6. 点击“确认导入所选”，回到 MCP 服务列表，进一步测试连接。

预览不调用 MCP，也不写数据库。最终导入会再次解析校验用户选择的配置，
使用 Core 的事务式批量导入；遇到冲突或选中项无效时不会部分写入。
未选中的非法项不会阻止有效项导入。

## 密钥保护

支持把 Headers / Env 中的**明文字段**安全转换为 `${MCP_CENTER_IMPORT_...}`
引用；原始密钥值不写入 MCP 定义，不在预览或导入结果中回显。
生成的引用带稳定短校验后缀避免名称碰撞。

**注意：转换不会自动保存明文密钥值。** 导入后在
“**设置 → 上游密钥变量**”中按预览显示的变量名手动录入，
由 Center 的密钥存储加密保管。若不希望转换，可先在原始配置中
改成 `${TOKEN}` 或 `{env:TOKEN}`。

本导入器不是自动扫描系统中的其他客户端配置，不会读取 ADM 的账号或环境配置，
也不会修改 ADM 或 Core。导出功能继续保持原有格式。

## 后端 API

- `POST /api/mcps/import/preview`：只读预览。
- `POST /api/mcps/import`：导入所选项。
- `GET /api/mcps/export?format=...`：原有导出。

两个 POST 都需要管理员已登录 Session，入参为：

```json
{
  "format": "auto",
  "content": "{\"mcpServers\":{\"example\":{\"url\":\"https://example.com/mcp\"}}}",
  "source_scope": "",
  "convert_literals": true,
  "conflict": "error",
  "selected_names": ["example"]
}
```

`selected_names` 只供确认导入；旧客户端不传该字段时仍导入全部候选项。
显式传入空数组会被拒绝，避免误导入。

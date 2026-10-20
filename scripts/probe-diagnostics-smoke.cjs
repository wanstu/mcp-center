const assert = require("node:assert/strict");
const fs = require("node:fs");
const vm = require("node:vm");
const path = require("node:path");
const src = fs.readFileSync(path.join(__dirname, "../web/app.js"), "utf8");
const start = src.indexOf("function mcpProbeDiagnostic(");
const end = src.indexOf("function renderMCPs()", start);
assert(start > 0 && end > start, "Diagnostic implementation not found");
const diagnose = vm.runInNewContext(src.slice(start, end) + "\nmcpProbeDiagnostic");
for (const [health, runtime, title] of [
  [{error_kind:"auth_failure"}, {last_error:"MCP response status code: 401"}, "HTTP 401：未授权"],
  [{error_kind:"auth_failure"}, {last_error:"401 forbidden, Authorization Bearer km_SUPERSECRET"}, "HTTP 401：未授权"],
  [{error_kind:"connection_failed"}, {last_error:"GET /mcp returned 404"}, "HTTP 404：地址不存在"],
  [{error_kind:"connection_failed"}, {last_error:"HTTP status code 400"}, "HTTP 400：请求无效"],
  [{error_kind:"connection_failed"}, {last_error:"response status: 502"}, "HTTP 502：上游服务错误"],
  [{error_kind:"connection_failed"}, {last_error:"x509: certificate signed by unknown authority"}, "TLS / 证书错误"],
  [{error_kind:"connection_failed"}, {last_error:"unexpected response content-type text/html"}, "响应格式错误"],
  [{error_kind:"timeout"}, {}, "探测超时"],
  [{error_kind:"connection_refused"}, {}, "连接被拒绝"]
]) {
  const result = diagnose(health, runtime);
  assert.equal(result.title, title);
  assert(!JSON.stringify(result).includes("km_SUPERSECRET"), "Access token leaked in diagnostic");
}
assert(src.includes("mcp-probe-reason"), "MCP status cell must display persistent reason");
assert(src.includes("原因及处理建议"), "Users must be able to expand troubleshooting guidance");
assert(src.includes("刷新失败：'+detail.title"), "Refresh must explain errors");
console.log("MCP Center probe diagnostics passed");

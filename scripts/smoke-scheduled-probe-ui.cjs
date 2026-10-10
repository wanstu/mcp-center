const fs=require("node:fs");
const path=require("node:path");
const assert=require("node:assert/strict");
const html=fs.readFileSync(path.join(__dirname,"../web/index.html"),"utf8");
const source=fs.readFileSync(path.join(__dirname,"../web/app.js"),"utf8");

for(const id of ["mcp-health-enabled","mcp-health-interval","mcp-health-timeout"]){
  assert.ok(html.includes('id="'+id+'"'),"missing scheduled probe control "+id);
}
assert.match(html,/启用定时探测/);
assert.match(html,/探测间隔（秒）/);
assert.match(html,/单次超时（秒）/);
assert.match(source,/health_check_enabled:healthEnabled/);
assert.match(source,/check_interval_seconds:healthInterval/);
assert.match(source,/probe_timeout_seconds:healthTimeout/);
assert.match(source,/scheduled_probes/);
assert.match(source,/startRuntimePolling/);
assert.match(source,/定时 '\+interval\+' 秒/);
console.log("PASS: scheduled probe controls, persistence, runtime status and polling");

// Ensure every release keeps legacy SSE as a real upstream transport.
const fs=require("node:fs");
const path=require("node:path");
const assert=require("node:assert/strict");
const html=fs.readFileSync(path.join(__dirname,"../web/index.html"),"utf8");
const js=fs.readFileSync(path.join(__dirname,"../web/app.js"),"utf8");
const dropdown=html.match(/<select id="mcp-transport">([\s\S]*?)<\/select>/)?.[1]??"";
for(const value of ["streamable-http","sse","stdio"]){
 assert(dropdown.includes('value="'+value+'"'),"transport option missing: "+value);
}
assert(js.includes("m?.transport||'streamable-http'"),"edit form must preserve saved transport");
assert(js.includes("value==='sse'"),"SSE field behavior missing");
assert(js.includes("m.transport==='sse'?'sse':'streamable-http'"),"MCP export must preserve SSE");
const sse=fs.readFileSync(path.join(__dirname,"../internal/server/mcp_sse_test.go"),"utf8");
assert(sse.includes("TestSSEUpstreamAccessibleViaCenter"),"SSE end-to-end regression test is missing");
assert(sse.includes("TestSSEImportFromGenericConfig"),"SSE import regression test is missing");
const mod=fs.readFileSync(path.join(__dirname,"../go.mod"),"utf8");
assert(/github\.com\/wanstu\/mcp-center-core v0\.1\.[2-9]/.test(mod),"Core dependency must include SSE");
console.log("SSE UI + integration + dependency regression guards passed");

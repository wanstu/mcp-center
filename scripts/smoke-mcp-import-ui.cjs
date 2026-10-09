// Browser-free import workflow acceptance test (preview -> select -> apply).
const fs=require("node:fs");
const path=require("node:path");
const vm=require("node:vm");
const assert=require("node:assert/strict");
const source=fs.readFileSync(path.join(__dirname,"../web/app.js"),"utf8");
const html=fs.readFileSync(path.join(__dirname,"../web/index.html"),"utf8");
for(const id of ["import-dialog","import-form","import-preview","import-format","import-conflict","import-source-scope","import-content","import-submit","import-convert-literals"]){
  assert.ok(html.includes('id="'+id+'"'),"missing import control "+id);
}
assert.match(html,/type="submit" class="button primary">预览配置/);
const from=source.indexOf("function importRequestInput(){");
const to=source.indexOf("async function exportMCPs(format){",from);
assert.ok(from>=0&&to>from,"import flow not defined");

class FakeNode {
 constructor(tag,id=""){
  this.tagName=tag;this.id=id;this.value="";this.textContent="";this.type="";this.disabled=false;
  this.checked=false;this.children=[];this.className="";this.hidden=false;
  const classes=new Set(["hidden"]);
  this.classList={add:x=>classes.add(x),remove:x=>classes.delete(x),contains:x=>classes.has(x)};
 }
 append(...nodes){this.children.push(...nodes)}
 replaceChildren(...nodes){this.children=nodes}
 showModal(){this.modal=true}
 close(){this.modal=false}
 querySelectorAll(selector){
   const all=[];
   const visit=node=>{
     if(node.className==="import-select"&&node.checked&&!node.disabled)all.push(node);
     for(const child of node.children||[])visit(child);
   };
   visit(this);
   return selector===".import-select:checked"?all:[];
 }
}
const elements=Object.fromEntries(["import-format","import-conflict","import-source-scope","import-content","import-convert-literals","import-preview","import-submit","import-error","import-dialog","import-file-name"].map(id=>[id,new FakeNode("input",id)]));
elements["import-format"].value="auto";
elements["import-conflict"].value="error";
elements["import-content"].value='{"mcpServers":{"one":{"command":"node"},"two":{"url":"https://example.test/mcp"}}}';
elements["import-convert-literals"].checked=true;
const requests=[];
const mockPreview={format:"mcpServers",candidates:[
{name:"one",transport:"stdio",auth_mode:"none",endpoint_configured:false,warnings:[],errors:[]},
{name:"two",transport:"streamable-http",auth_mode:"none",endpoint_configured:true,existing_mcp_id:"mcp_existing",warnings:[],errors:[]}
]};
let listed=0;
const api=async(url,params)=>{
 const body=JSON.parse(params.body);
 requests.push({url,body});
 if(url.endsWith("/preview"))return mockPreview;
 if(url==="/api/mcps/import")return {added:1,updated:0,skipped:0};
 throw Error("Unexpected endpoint: "+url);
};
const state={importPreview:null,importPreviewSignature:"",health:new Map(),tools:new Map(),runtime:new Map()};
const context={state,el:id=>elements[id],document:{createElement:t=>new FakeNode(t)},
 api,clearError:node=>node.classList.add("hidden"),
 showError:(node,err)=>{node.textContent=String(err.message||err);node.classList.remove("hidden")},
 toast:()=>{},loadMCPs:async()=>{listed++}};
vm.createContext(context);
vm.runInContext(source.slice(from,to),context);
(async()=>{
 const event={preventDefault(){}};
 await context.importMCPs(event);
 assert.equal(requests.length,1);
 assert.equal(requests[0].url,"/api/mcps/import/preview");
 assert.equal(elements["import-dialog"].modal,undefined);
 assert.ok(state.importPreview);
 assert.equal(elements["import-submit"].textContent,"确认导入所选");
 assert.equal(elements["import-preview"].querySelectorAll(".import-select:checked").length,2);
 const checkboxes=elements["import-preview"].querySelectorAll(".import-select:checked");
 checkboxes[1].checked=false;
 await context.importMCPs(event);
 assert.equal(requests.length,2);
 assert.equal(requests[1].url,"/api/mcps/import");
 assert.equal(requests[1].body.selected_names.length,1);
 assert.equal(requests[1].body.selected_names[0],"one");
 assert.equal(listed,1);
 assert.equal(state.importPreview,null);
 assert.equal(elements["import-submit"].textContent,"预览配置");
 elements["import-content"].value='{"mcpServers":{"changed":{"command":"node"}}}';
 await context.importMCPs(event);
 assert.equal(requests.length,3,"changing the payload must require re-preview");
 assert.equal(requests[2].url,"/api/mcps/import/preview");
 assert.equal(listed,1,"re-preview must not apply changes");
 console.log("PASS: import UI preview-before-apply, selected candidates only, re-preview on change, no premature import");
})().catch(err=>{console.error(err);process.exitCode=1});

// Lightweight auth-form behavior contract; no browser or third-party packages.
const fs=require("node:fs");
const path=require("node:path");
const vm=require("node:vm");
const assert=require("node:assert/strict");
const source=fs.readFileSync(path.join(__dirname,"../web/app.js"),"utf8");
const html=fs.readFileSync(path.join(__dirname,"../web/index.html"),"utf8");
function between(start,end){
 const a=source.indexOf(start),b=source.indexOf(end,a);
 assert.ok(a>=0&&b>a,"auth handler missing: "+start);
 return source.slice(a,b);
}
assert.match(html,/id="auth-confirm-field"/);
assert.match(html,/id="auth-confirm-password"[^>]+type="password"/);
assert.match(html,/data-password-toggle="auth-password"/);
assert.equal((html.match(/data-password-toggle="auth-confirm-password"/g)||[]).length,0);
assert.equal((html.match(/data-password-toggle="auth-password"/g)||[]).length,1);
const code=between("function setPasswordVisibility(","async function enterApp(")+"\n"+
           between("async function submitAuth(","function switchPage(");
let calls=[],entered=0,focused="";
function fakeElement(id){
 const tags=new Set(["hidden"]);
 return {id,value:"",type:"password",autocomplete:"",required:false,textContent:"",
   classList:{add:x=>tags.add(x),remove:x=>tags.delete(x),toggle:(x,v)=>v?tags.add(x):tags.delete(x),contains:x=>tags.has(x)},
   focus(){focused=id;},
   setAttribute(k,v){this[k]=v},
 };
}
const names=["app","auth-screen","setup-copy","login-copy","auth-submit","auth-username","auth-password","auth-confirm-password","auth-confirm-field","auth-error"];
const elements=Object.fromEntries(names.map(id=>[id,fakeElement(id)]));
const buttonIDs=["auth-password"];
const buttons=Object.fromEntries(buttonIDs.map(id=>[id,fakeElement("button-"+id)]));
const el=id=>elements[id];
const state={needsBootstrap:false,user:null};
const document={querySelector:query=>buttons[query.match(/data-password-toggle="([^"]+)"/)?.[1]]};
const api=async (url,opts)=>{calls.push({url,body:JSON.parse(opts.body)});return {user:{username:"test-admin"}}};
const clearError=node=>{node.textContent="";node.classList.add("hidden")};
const showError=(node,message)=>{node.textContent=message;node.classList.remove("hidden")};
const context={el,state,document,api,clearError,showError,enterApp:()=>{entered++}};
vm.createContext(context);
vm.runInContext(code,context);

(async()=>{
 context.showAuth(true);
 assert.equal(el("auth-confirm-field").classList.contains("hidden"),false);
 assert.equal(el("auth-confirm-password").required,true);
 assert.equal(el("auth-password").autocomplete,"new-password");
 context.setPasswordVisibility(true);
 assert.equal(el("auth-password").type,"text");
 assert.equal(el("auth-confirm-password").type,"text");
 assert.equal(buttons["auth-password"]["aria-pressed"],"true");
 assert.equal(buttons["auth-password"].textContent,"隐藏");
 context.setPasswordVisibility(false);
 assert.equal(el("auth-password").type,"password");
 assert.equal(el("auth-confirm-password").type,"password");
 assert.equal(buttons["auth-password"]["aria-pressed"],"false");
 el("auth-username").value="admin";
 el("auth-password").value="very-secure-password";
 el("auth-confirm-password").value="another-password";
 let cancelled=false;
 await context.submitAuth({preventDefault(){cancelled=true}});
 assert.ok(cancelled);
 assert.equal(calls.length,0,"mismatched password should not send request");
 assert.match(el("auth-error").textContent,/密码不一致/);
 assert.equal(focused,"auth-confirm-password");
 el("auth-confirm-password").value=el("auth-password").value;
 await context.submitAuth({preventDefault(){}});
 assert.equal(calls.length,1);
 assert.equal(calls[0].url,"/api/setup/register");
 assert.deepEqual(calls[0].body,{username:"admin",password:"very-secure-password"});
 assert.equal(entered,1);
 context.showAuth(false);
 assert.equal(el("auth-confirm-field").classList.contains("hidden"),true);
 assert.equal(el("auth-confirm-password").required,false);
 assert.equal(el("auth-password").autocomplete,"current-password");
 assert.equal(el("auth-password").type,"password");
 assert.equal(el("auth-confirm-password").type,"password");
 el("auth-password").value="very-secure-password";
 el("auth-confirm-password").value="mismatch-ignored-on-login";
 await context.submitAuth({preventDefault(){}});
 assert.equal(calls.length,2);
 assert.equal(calls[1].url,"/api/auth/login");
 console.log("PASS: setup repeat password, mismatch rejection, single synchronized visibility toggle and login mode");
})().catch(error=>{console.error(error);process.exitCode=1});

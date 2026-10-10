const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');
const assert=require('node:assert/strict');
const html=fs.readFileSync(path.join(__dirname,'../web/index.html'),'utf8');
const js=fs.readFileSync(path.join(__dirname,'../web/app.js'),'utf8');
for(const id of ['page-about','about-version','about-commit','about-platform','about-architecture','about-go-version','about-kit-version','about-base-url','about-listen','about-data-dir','about-base-source','about-copy-diagnostics','settings-public-base-url','save-public-base-url','use-current-https-url','settings-public-base-url-error']){
 assert.ok(html.includes('id="'+id+'"'),'missing UI element '+id);
}
assert.match(html,/data-page="about">关于/);
const begin=js.indexOf('function renderAbout(){'),end=js.indexOf('async function saveSystemSettings(){',begin);
assert.ok(begin>=0&&end>begin,'About/settings functions missing');
function elFactory(){
 const list={};
 return id=>list[id]??(list[id]={value:'',textContent:'',disabled:false,classList:{add(){},remove(){},toggle(){}},focus(){}});
}
const el=elFactory();
const calls=[];
const state={
 versionInfo:{version:'v0.1.3-rc.4',commit:'testcommit',modified:false,platform:'linux',architecture:'amd64',go_version:'go1.26.5',kit_version:'v0.11.3'},
 connectionInfo:{base_url:'http://cmcps.wanstu.cn',base_url_source:'automatic',listen_address:'0.0.0.0:8610',data_dir:'/var/lib/mcp-center'},
 systemSettings:{public_base_url:''}
};
const context={state,el,URL,location:{protocol:'https:',origin:'https://cmcps.wanstu.cn'},clearError:()=>{},showError:(node,message)=>{node.textContent=String(message)},toast:()=>{},
 api:async(url,arg)=>{calls.push({url,body:JSON.parse(arg.body)});return {settings:{public_base_url:JSON.parse(arg.body).public_base_url}};},
 loadConnectionInfo:async()=>{state.connectionInfo.base_url=state.systemSettings.public_base_url;state.connectionInfo.base_url_source='settings'},
 loadSystemSettings:async()=>{},
 copyTextValue:async()=>{},
};
vm.createContext(context);
vm.runInContext(js.slice(begin,end),context);
context.renderAbout();
assert.equal(el('about-version').textContent,'v0.1.3-rc.4');
assert.equal(el('about-commit').textContent,'testcommit');
assert.equal(el('about-platform').textContent,'linux');
assert.equal(el('about-architecture').textContent,'amd64');
assert.equal(el('about-go-version').textContent,'go1.26.5');
assert.equal(el('about-kit-version').textContent,'v0.11.3');
assert.equal(el('about-base-url').textContent,'http://cmcps.wanstu.cn');
(async()=>{
 el('settings-public-base-url').value='https://cmcps.wanstu.cn/mcp/example';
 await context.savePublicBaseURL();
 assert.equal(calls.length,0,'invalid path must not reach API');
 el('settings-public-base-url').value='https://cmcps.wanstu.cn';
 await context.savePublicBaseURL();
 assert.equal(calls.length,1);
 assert.equal(calls[0].url,'/api/settings');
 assert.equal(calls[0].body.public_base_url,'https://cmcps.wanstu.cn');
 context.renderAbout();
 assert.equal(el('about-base-url').textContent,'https://cmcps.wanstu.cn');
 assert.equal(el('about-base-source').textContent,'设置中的公开 HTTPS 基础地址');
 console.log('PASS About navigation markup, runtime fields, HTTPS settings validation and saved origin');
})().catch(e=>{console.error(e);process.exitCode=1});
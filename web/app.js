if(globalThis.desktopKitTheme)globalThis.desktopKitTheme.apply('system');
const state={user:null,needsBootstrap:false,mcps:[],health:new Map(),runtime:new Map(),tools:new Map(),selectedTool:null,selectedToolMCPID:'',toolParamMode:'form',toolHistory:new Map(),connectionInfo:null,editingMCP:null,accessSettings:null,systemSettings:null,credentials:[],oauthClients:[],oauthTestID:'',oauthTestStatus:'',oauthTestAuthorizeURL:'',upstreamSecrets:[],audits:[],auditSummary:{total:0,success:0,errors:0,recent_errors:[]},page:'dashboard',mock:new URLSearchParams(location.search).get('mock')==='1'};
const el=id=>document.getElementById(id);
const json=v=>JSON.stringify(v,null,2);
const pageNames={dashboard:'工作台',mcps:'MCP 服务',tools:'工具测试',access:'访问授权',logs:'调用日志',settings:'设置'};
let oauthTestPollTimer=null;

function toast(message){const node=el('toast');node.textContent=message;node.classList.add('show');clearTimeout(window.__toastTimer);window.__toastTimer=setTimeout(()=>node.classList.remove('show'),2200)}
function showError(node,error){node.textContent=String(error?.message||error);node.classList.remove('hidden')}
function clearError(node){node.textContent='';node.classList.add('hidden')}

const mockData={mcps:[
{id:'mcp_demo_adm',name:'pj-adm',transport:'streamable-http',auth_mode:'none',endpoint:'http://127.0.0.1:8001/mcp',default_include_in_environment:true},
{id:'mcp_demo_db',name:'wm-test-db',transport:'stdio',auth_mode:'none',executable:'mysql-mcp',args:['--readonly'],default_include_in_environment:false}
],tools:{mcp_demo_adm:[{name:'environment_list',description:'List lightweight Environment summaries',input_schema:{type:'object',properties:{}}},{name:'read',description:'Read a text file',input_schema:{type:'object',properties:{path:{type:'string'}},required:['path']}}],mcp_demo_db:[{name:'query',description:'Run a read-only query',input_schema:{type:'object',properties:{sql:{type:'string'}},required:['sql']}}]},runtime:{},accessSettings:{bearer_enabled:true,header_enabled:true,header_name:'X-MCP-Token',query_enabled:false,query_name:'mcp_token'},systemSettings:{audit_retention_days:30},credentials:[],oauthClients:[],upstreamSecrets:[],audits:[{request_id:'req_demo',source:'console',principal_id:'usr_demo',principal_name:'admin',auth_channel:'session',mcp_id:'mcp_demo_adm',tool_name:'environment_list',started_at:new Date().toISOString(),duration_ms:42,status:'success',input_meta:{key_count:0,keys:[]},output_meta:{content_count:1,is_error:false,has_structured_content:true}}]};

async function api(path,options={}){
  if(state.mock)return mockApi(path,options);
  const init={credentials:'same-origin',...options,headers:{...(options.body?{'Content-Type':'application/json'}:{}),...(options.headers||{})}};
  const response=await fetch(path,init);
  let payload=null;try{payload=await response.json()}catch{}
  if(!response.ok)throw new Error(payload?.error||payload?.error_description||('HTTP '+response.status));
  return payload;
}
async function mockApi(path,options){
  await new Promise(r=>setTimeout(r,80));
  const method=options.method||'GET';
  if(path==='/api/setup')return {needs_bootstrap:false};
  if(path==='/api/me')return {user:{id:'usr_demo',username:'admin',role:'admin'}};
  if(path==='/api/auth/login')return {user:{id:'usr_demo',username:'admin',role:'admin'}};
  if(path==='/api/auth/logout')return {ok:true};
  if(path==='/api/version'&&method==='GET')return {version:'dev',commit:'mock123456789',modified:false};
  if(path==='/api/connection-info'&&method==='GET')return {base_url:location.origin,mcp_endpoint_template:location.origin+'/mcp/{id}',mcp_id_endpoint_template:location.origin+'/mcp/{id}',mcp_name_endpoint_template:location.origin+'/mcp/{name}',authorization_server_metadata:location.origin+'/.well-known/oauth-authorization-server',is_loopback:/^(localhost|127\.0\.0\.1|\[::1\])(?::|$)/.test(location.host)};
  if(path==='/api/account/password'&&method==='POST')return {user:{id:'usr_demo',username:'admin',role:'admin'}};
  if(path==='/api/settings'&&method==='GET')return {settings:mockData.systemSettings};
  if(path==='/api/settings'&&method==='PUT'){mockData.systemSettings=JSON.parse(options.body);return {settings:mockData.systemSettings}}
  if(path==='/api/upstream-secrets'&&method==='GET')return {secrets:mockData.upstreamSecrets};
  const upstreamSecret=path.match(/^\/api\/upstream-secrets\/([^/]+)$/);
  if(upstreamSecret){
    const name=decodeURIComponent(upstreamSecret[1]);
    if(method==='PUT'){
      const now=new Date().toISOString(),existing=mockData.upstreamSecrets.find(x=>x.name===name);
      const item={name,created_at:existing?.created_at||now,updated_at:now};
      mockData.upstreamSecrets=mockData.upstreamSecrets.filter(x=>x.name!==name);mockData.upstreamSecrets.push(item);return {secret:item};
    }
    if(method==='DELETE'){mockData.upstreamSecrets=mockData.upstreamSecrets.filter(x=>x.name!==name);return {ok:true}}
  }
  if(path==='/api/oauth/clients'&&method==='GET')return {clients:mockData.oauthClients};
  if(path==='/api/oauth/clients'&&method==='POST'){const input=JSON.parse(options.body),item={client_id:'client_'+Date.now(),created_at:new Date().toISOString(),grant_types:input.grant_types||['authorization_code','refresh_token'],response_types:input.response_types||['code'],...input};mockData.oauthClients.push(item);return {client:item}}
  const oauthClient=path.match(/^\/api\/oauth\/clients\/([^/]+)$/);
  if(oauthClient){const id=oauthClient[1];if(method==='DELETE'){mockData.oauthClients=mockData.oauthClients.filter(x=>x.client_id!==id);return {ok:true}}if(method==='PUT'){const input=JSON.parse(options.body),i=mockData.oauthClients.findIndex(x=>x.client_id===id);if(i<0)throw new Error('OAuth Client 不存在');mockData.oauthClients[i]={...mockData.oauthClients[i],...input};return {client:mockData.oauthClients[i]}}}
  if(path==='/api/access/settings'&&method==='GET')return {settings:mockData.accessSettings};
  if(path==='/api/access/settings'&&method==='PUT'){mockData.accessSettings=JSON.parse(options.body);return {settings:mockData.accessSettings}}
  if(path==='/api/access/credentials'&&method==='GET')return {credentials:mockData.credentials};
  if(path==='/api/audit/summary'&&method==='GET'){
    const since=Date.now()-24*60*60*1000,events=mockData.audits.filter(x=>new Date(x.started_at).getTime()>=since),errors=events.filter(x=>x.status==='error');
    return {since:new Date(since).toISOString(),total:events.length,success:events.length-errors.length,errors:errors.length,recent_errors:errors.slice(-5).reverse()};
  }
  if(path.startsWith('/api/audit')&&method==='GET')return {events:mockData.audits};
  if(path==='/api/audit'&&method==='DELETE'){mockData.audits=[];return {ok:true}}
  if(path==='/api/access/credentials'&&method==='POST'){const input=JSON.parse(options.body),secret='mcp_mock_'+Date.now(),item={id:'cred_'+Date.now(),secret_prefix:secret.slice(0,12),created_at:new Date().toISOString(),updated_at:new Date().toISOString(),last_used_at:null,...input};mockData.credentials.push(item);return {credential:item,secret}}
  const cred=path.match(/^\/api\/access\/credentials\/([^/]+)$/);
  if(cred){const id=cred[1];if(method==='DELETE'){mockData.credentials=mockData.credentials.filter(x=>x.id!==id);return {ok:true}}if(method==='PUT'){const input=JSON.parse(options.body),i=mockData.credentials.findIndex(x=>x.id===id);mockData.credentials[i]={...mockData.credentials[i],...input,updated_at:new Date().toISOString()};return {credential:mockData.credentials[i]}}}
  if(path==='/api/mcps/import'&&method==='POST'){
    const request=JSON.parse(options.body),doc=JSON.parse(request.content),configs=[];
    if(doc.mcpServers){for(const [name,item] of Object.entries(doc.mcpServers)){configs.push(item.url?{name,transport:'streamable-http',auth_mode:item.headers?'headers':'none',endpoint:item.url,header_refs:item.headers||undefined,health_policy:{}}:{name,transport:'stdio',auth_mode:'none',executable:item.command,args:item.args||[],env_refs:item.env||undefined,health_policy:{}})}}
    else if(Array.isArray(doc.mcps))configs.push(...doc.mcps);
    else throw new Error('无法识别导入格式');
    let added=0,updated=0,skipped=0;
    for(const cfg of configs){const i=mockData.mcps.findIndex(x=>x.name.toLowerCase()===String(cfg.name||'').toLowerCase());if(i>=0){if(request.conflict==='error')throw new Error('MCP 已存在: '+cfg.name);if(request.conflict==='skip'){skipped++;continue}mockData.mcps[i]={...mockData.mcps[i],...cfg};updated++;continue}mockData.mcps.push({id:'mcp_'+Date.now()+'_'+added,...cfg});added++}
    return {added,updated,skipped,items:[]};
  }
  if(path==='/api/runtime'&&method==='GET')return {runtime:mockData.mcps.map(m=>mockData.runtime[m.id]||{mcp_id:m.id,state:'disconnected',tool_count:0})};
  const runtimePath=path.match(/^\/api\/runtime\/([^/]+)\/(refresh|disconnect)$/);
  if(runtimePath&&method==='POST'){const id=runtimePath[1],action=runtimePath[2];if(action==='disconnect'){const status={mcp_id:id,state:'disconnected',tool_count:0};mockData.runtime[id]=status;return {status}}const tools=mockData.tools[id]||[];const status={mcp_id:id,state:'connected',tool_count:tools.length,connected_at:new Date().toISOString(),last_used_at:new Date().toISOString()};mockData.runtime[id]=status;return {status,tools}}
  if(path==='/api/mcps'&&method==='GET')return {mcps:mockData.mcps};
  if(path==='/api/mcps'&&method==='POST'){const cfg=JSON.parse(options.body);const m={id:'mcp_'+Date.now(),...cfg};mockData.mcps.push(m);return {mcp:m}}
  const statePath=path.match(/^\/api\/mcps\/([^/]+)\/state$/);
  if(statePath&&method==='PUT'){
    const id=statePath[1],input=JSON.parse(options.body),i=mockData.mcps.findIndex(x=>x.id===id);
    mockData.mcps[i]={...mockData.mcps[i],disabled:Boolean(input.disabled)};
    delete mockData.runtime[id];
    return {mcp:mockData.mcps[i]};
  }
  const m=path.match(/^\/api\/mcps\/([^/]+)(?:\/(probe|tools|call))?$/);
  if(m){const id=m[1],action=m[2];
    if(method==='DELETE'){mockData.mcps=mockData.mcps.filter(x=>x.id!==id);return {ok:true}}
    if(method==='PUT'){const cfg=JSON.parse(options.body);const i=mockData.mcps.findIndex(x=>x.id===id);mockData.mcps[i]={...mockData.mcps[i],...cfg};return {mcp:mockData.mcps[i]}}
    if(action==='probe'){const tools=mockData.tools[id]||[];mockData.runtime[id]={mcp_id:id,state:'connected',tool_count:tools.length,connected_at:new Date().toISOString(),last_used_at:new Date().toISOString()};return {mcp_id:id,state:'healthy',tools,checked_at:new Date().toISOString()}}
    if(action==='tools'){const tools=mockData.tools[id]||[];mockData.runtime[id]={mcp_id:id,state:'connected',tool_count:tools.length,connected_at:new Date().toISOString(),last_used_at:new Date().toISOString()};return {tools}}
    if(action==='call'){const current=mockData.runtime[id]||{mcp_id:id,state:'connected',tool_count:(mockData.tools[id]||[]).length,connected_at:new Date().toISOString()};mockData.runtime[id]={...current,state:'connected',last_used_at:new Date().toISOString()};return {content:[{type:'text',text:'mock result'}],structuredContent:{ok:true,arguments:JSON.parse(options.body).arguments||{}}}};
  }
  if(path==='/.well-known/oauth-authorization-server')return {issuer:location.origin,authorization_endpoint:location.origin+'/oauth/authorize',token_endpoint:location.origin+'/oauth/token',code_challenge_methods_supported:['S256']};
  throw new Error('Mock endpoint not implemented: '+method+' '+path);
}

async function bootstrap(){
  try{
    const setup=await api('/api/setup');
    state.needsBootstrap=Boolean(setup.needs_bootstrap);
    if(state.needsBootstrap)return showAuth(true);
    try{
      const me=await api('/api/me');
      state.user=me.user;enterApp();
    }catch{showAuth(false)}
  }catch(error){
    if(state.mock){showAuth(false);return}
    document.body.innerHTML='<main style="padding:32px;font-family:Segoe UI,sans-serif"><h1>MCP Center 启动失败</h1><p>'+escapeHTML(error.message)+'</p></main>';
  }
}
function showAuth(setup){
  el('app').classList.add('hidden');el('auth-screen').classList.remove('hidden');
  el('setup-copy').classList.toggle('hidden',!setup);el('login-copy').classList.toggle('hidden',setup);
  el('auth-submit').textContent=setup?'创建管理员':'登录';
  state.needsBootstrap=setup;el('auth-username').focus();
}
async function enterApp(){
  el('auth-screen').classList.add('hidden');el('app').classList.remove('hidden');
  el('me-name').textContent=state.user?.username||'admin';
  renderAccountSettings();
  await Promise.allSettled([loadMCPs(),loadOAuth(),loadAccess(),loadConnectionInfo(),loadVersion(),loadSystemSettings(),loadUpstreamSecrets(),loadAuditSummary()]);
}
async function submitAuth(event){
  event.preventDefault();clearError(el('auth-error'));
  const username=el('auth-username').value.trim(),password=el('auth-password').value;
  try{
    const path=state.needsBootstrap?'/api/setup/register':'/api/auth/login';
    const result=await api(path,{method:'POST',body:JSON.stringify({username,password})});
    state.user=result.user;enterApp();
  }catch(error){showError(el('auth-error'),error)}
}

function switchPage(page){
  state.page=page;
  document.querySelectorAll('.page').forEach(n=>n.classList.remove('active'));
  document.querySelectorAll('.nav button[data-page]').forEach(n=>n.classList.toggle('active',n.dataset.page===page));
  el('page-'+page).classList.add('active');el('page-title').textContent=pageNames[page]||page;
  if(page==='tools')refreshToolMCPSelect();
  if(page==='access')Promise.all([loadAccess(),loadOAuth(),loadConnectionInfo()]).catch(error=>toast(error.message));
  if(page==='logs')loadAudit().catch(error=>toast(error.message));
  if(page==='dashboard')loadAuditSummary().catch(error=>toast(error.message));
  if(page==='settings'){renderAccountSettings();Promise.all([loadVersion(),loadSystemSettings(),loadUpstreamSecrets()]).catch(error=>toast(error.message))}
}
async function loadAuditSummary(){
  const result=await api('/api/audit/summary');
  state.auditSummary=result||{total:0,success:0,errors:0,recent_errors:[]};
  renderDashboard();
}
async function loadMCPs(){
  const [result,runtimeResult]=await Promise.all([api('/api/mcps'),api('/api/runtime')]);
  state.mcps=result.mcps||[];
  state.runtime=new Map((runtimeResult.runtime||[]).map(item=>[item.mcp_id,item]));
  renderMCPs();renderDashboard();refreshToolMCPSelect();renderConnectionInfo();
}
async function loadRuntimeStatuses(){
  const result=await api('/api/runtime');
  state.runtime=new Map((result.runtime||[]).map(item=>[item.mcp_id,item]));
  renderMCPs();renderDashboard();
}
function mcpStatusKey(m){
  if(m.disabled)return 'disabled';
  const health=state.health.get(m.id),runtime=state.runtime.get(m.id);
  if(runtime?.state==='connected'||health?.state==='healthy')return 'connected';
  if(runtime?.state==='error'||health?.state==='error')return 'error';
  return 'disconnected';
}
function renderMCPs(){
  const q=el('mcp-search').value.trim().toLowerCase();
  const statusFilter=el('mcp-status-filter')?.value||'all';
  const items=state.mcps.filter(m=>{
    const matchesSearch=!q||[m.name,m.endpoint,m.executable,m.transport].some(v=>String(v||'').toLowerCase().includes(q));
    const matchesStatus=statusFilter==='all'||mcpStatusKey(m)===statusFilter;
    return matchesSearch&&matchesStatus;
  });
  const body=el('mcp-rows');body.innerHTML='';
  for(const m of items){
    const health=state.health.get(m.id),runtime=state.runtime.get(m.id),statusKey=mcpStatusKey(m);
    let status='<span class="status">未连接</span>';
    if(statusKey==='connected')status='<span class="status ok">已连接</span>';
    else if(statusKey==='error')status='<span class="status bad">异常</span>';
    else if(statusKey==='disabled')status='<span class="status warn">已禁用</span>';
    const tools=m.disabled?'—':(state.tools.get(m.id)?.length??health?.tools?.length??runtime?.tool_count??'—');
    const where=m.transport==='stdio'?escapeHTML([m.executable,...(m.args||[])].join(' ')):escapeHTML(m.endpoint||'');
    const runtimeActions=m.disabled?'':('<button class="button small" data-op="probe">刷新</button><button class="button small" data-op="test">测试</button>'+(runtime?.state==='connected'?'<button class="button ghost small" data-op="disconnect">断开</button> ':''));
    const stateAction='<button class="button ghost small" data-op="state">'+(m.disabled?'启用':'禁用')+'</button>';
    const tr=document.createElement('tr');
    tr.innerHTML='<td><strong>'+escapeHTML(m.name)+'</strong><br><small class="muted">'+escapeHTML(m.id)+'</small></td><td>'+escapeHTML(m.transport)+'</td><td>'+status+'</td><td>'+tools+'</td><td><span class="muted">'+where+'</span></td><td><div class="row">'+runtimeActions+'<button class="button ghost small" data-op="endpoint">复制接入地址</button>'+stateAction+'<button class="button ghost small" data-op="edit">编辑</button><button class="button ghost small" data-op="delete">删除</button></div></td>';
    tr.querySelector('[data-op="probe"]')?.addEventListener('click',()=>probeMCP(m.id));
    tr.querySelector('[data-op="test"]')?.addEventListener('click',()=>{switchPage('tools');el('tool-mcp').value=m.id;loadTools(m.id)});
    tr.querySelector('[data-op="endpoint"]').onclick=()=>copyMCPExternalURL(m,'name');
    tr.querySelector('[data-op="disconnect"]')?.addEventListener('click',()=>disconnectMCP(m.id));
    tr.querySelector('[data-op="state"]').onclick=()=>toggleMCPDisabled(m);
    tr.querySelector('[data-op="edit"]').onclick=()=>openMCPDialog(m);
    tr.querySelector('[data-op="delete"]').onclick=()=>deleteMCP(m);
    body.appendChild(tr);
  }
  el('mcp-table-wrap').classList.toggle('hidden',items.length===0);
  const empty=el('mcp-empty');
  empty.classList.toggle('hidden',items.length!==0);
  if(items.length===0)empty.textContent=state.mcps.length?'没有符合筛选条件的 MCP。':'还没有 MCP，先添加一个。';
}
function renderDashboard(){
  el('metric-mcps').textContent=state.mcps.length;
  const online=state.mcps.filter(m=>!m.disabled&&(state.runtime.get(m.id)?.state==='connected'||state.health.get(m.id)?.state==='healthy')).length;
  el('metric-online').textContent=online;
  let tools=0;
  for(const m of state.mcps)if(!m.disabled)tools+=state.tools.get(m.id)?.length??state.health.get(m.id)?.tools?.length??state.runtime.get(m.id)?.tool_count??0;
  el('metric-tools').textContent=tools;
  el('metric-mcps-sub').textContent=state.mcps.length?'已加载配置':'尚未添加';

  const summary=state.auditSummary||{total:0,errors:0,recent_errors:[]};
  el('metric-calls-24h').textContent=summary.total||0;
  el('metric-errors-24h').textContent='错误 '+(summary.errors||0);

  const box=el('dashboard-list');box.innerHTML='';
  if(!state.mcps.length){
    box.className='empty';box.textContent='暂无 MCP 服务';
  }else{
    box.className='service-list';
    for(const m of state.mcps.slice(0,6)){
      const row=document.createElement('div');row.className='service-item';
      const h=state.health.get(m.id),runtime=state.runtime.get(m.id);
      let status=m.disabled?'<span class="status warn">已禁用</span>':'<span class="status">未连接</span>';
      if(!m.disabled&&(runtime?.state==='connected'||h?.state==='healthy'))status='<span class="status ok">已连接</span>';
      else if(!m.disabled&&(runtime?.state==='error'||h?.state==='error'))status='<span class="status bad">异常</span>';
      row.innerHTML='<div><strong>'+escapeHTML(m.name)+'</strong><small>'+escapeHTML(m.transport)+' · '+escapeHTML(m.endpoint||m.executable||'')+'</small></div>'+status;
      box.appendChild(row);
    }
  }

  const errorsBox=el('dashboard-errors');errorsBox.innerHTML='';
  const errors=summary.recent_errors||[];
  if(!errors.length){
    errorsBox.className='empty';errorsBox.textContent='近 24 小时无错误';
  }else{
    errorsBox.className='service-list';
    for(const item of errors){
      const row=document.createElement('div');row.className='service-item';
      const when=item.started_at?new Date(item.started_at).toLocaleString():'—';
      row.innerHTML='<div><strong>'+escapeHTML((item.mcp_id||'未知 MCP')+' / '+(item.tool_name||'未知工具'))+'</strong><small>'+escapeHTML(when)+' · '+escapeHTML(item.request_id||'')+'</small></div><span class="status bad">'+escapeHTML(item.error_kind||'error')+'</span>';
      errorsBox.appendChild(row);
    }
  }
}
async function probeMCP(id){
  try{toast('正在刷新…');const result=await api('/api/mcps/'+encodeURIComponent(id)+'/probe',{method:'POST'});state.health.set(id,result);if(result.tools)state.tools.set(id,result.tools);await loadRuntimeStatuses();toast(result.state==='healthy'?'刷新成功':'刷新完成：'+result.state)}
  catch(error){state.health.set(id,{state:'error',message:error.message});await loadRuntimeStatuses().catch(()=>{});renderMCPs();toast('刷新失败：'+error.message)}
}
async function probeAll(){for(const m of state.mcps)await probeMCP(m.id)}
function formatKVMap(input){
  return Object.entries(input||{}).map(([key,value])=>key+'='+value).join('\n');
}
function parseKVLines(value,label){
  const out={};
  for(const [index,raw] of String(value||'').split(/\r?\n/).entries()){
    const line=raw.trim();
    if(!line||line.startsWith('#'))continue;
    const split=line.indexOf('=');
    if(split<=0)throw new Error(label+' 第 '+(index+1)+' 行格式应为 KEY=VALUE');
    const key=line.slice(0,split).trim(),item=line.slice(split+1).trim();
    if(!key||!item)throw new Error(label+' 第 '+(index+1)+' 行 KEY 和 VALUE 都不能为空');
    if(Object.prototype.hasOwnProperty.call(out,key))throw new Error(label+' 存在重复 KEY：'+key);
    out[key]=item;
  }
  return out;
}
function hasEnvRef(value){
  return /\$(?:[A-Za-z_][A-Za-z0-9_]*|\{[^}]+\})/.test(String(value||''));
}
function requireEnvRefs(items,label){
  for(const [key,value] of Object.entries(items)){
    if(!hasEnvRef(value))throw new Error(label+' '+key+' 必须使用环境变量引用，例如 ${TOKEN}');
  }
}
function openMCPDialog(m=null){
  state.editingMCP=m;
  el('mcp-dialog-title').textContent=m?'编辑 MCP':'添加 MCP';
  el('mcp-id').value=m?.id||'';
  el('mcp-name').value=m?.name||'';
  el('mcp-transport').value=m?.transport||'streamable-http';
  el('mcp-endpoint').value=m?.endpoint||'';
  el('mcp-executable').value=m?.executable||'';
  el('mcp-args').value=(m?.args||[]).join('\n');
  el('mcp-env').value=formatKVMap(m?.env_refs);
  el('mcp-auth').value=m?.auth_mode||'none';
  el('mcp-headers').value=formatKVMap(m?.header_refs);
  el('mcp-enabled').checked=!Boolean(m?.disabled);
  clearError(el('mcp-form-error'));
  updateTransportFields();
  el('mcp-dialog').showModal();
}
function updateTransportFields(){
  const stdio=el('mcp-transport').value==='stdio';
  document.querySelectorAll('.stdio-only').forEach(n=>n.classList.toggle('hidden',!stdio));
  document.querySelectorAll('.http-only').forEach(n=>n.classList.toggle('hidden',stdio));
  updateAuthFields();
}
function updateAuthFields(){
  const show=el('mcp-transport').value==='streamable-http'&&el('mcp-auth').value==='headers';
  document.querySelectorAll('.header-auth-only').forEach(n=>n.classList.toggle('hidden',!show));
}
async function saveMCP(event){
  event.preventDefault();clearError(el('mcp-form-error'));
  const id=el('mcp-id').value.trim(),transport=el('mcp-transport').value;
  const payload={
    name:el('mcp-name').value.trim(),
    transport,
    auth_mode:transport==='stdio'?'none':el('mcp-auth').value,
    disabled:!el('mcp-enabled').checked,
    default_include_in_environment:Boolean(state.editingMCP?.default_include_in_environment),
    health_policy:state.editingMCP?.health_policy||{health_check_enabled:false,auto_reconnect:false}
  };
  try{
    if(transport==='stdio'){
      payload.executable=el('mcp-executable').value.trim();
      payload.args=el('mcp-args').value.split(/\r?\n/).map(x=>x.trim()).filter(Boolean);
      payload.env_refs=parseKVLines(el('mcp-env').value,'环境变量');
      requireEnvRefs(payload.env_refs,'环境变量');
    }else{
      payload.endpoint=el('mcp-endpoint').value.trim();
      if(payload.auth_mode==='headers'){
        payload.header_refs=parseKVLines(el('mcp-headers').value,'请求头');
        if(!Object.keys(payload.header_refs).length)throw new Error('请求头认证至少需要配置一项 Header');
        requireEnvRefs(payload.header_refs,'请求头');
      }
    }
    await api(id?'/api/mcps/'+encodeURIComponent(id):'/api/mcps',{method:id?'PUT':'POST',body:JSON.stringify(payload)});
    state.editingMCP=null;
    el('mcp-dialog').close();
    await loadMCPs();
    toast(id?'MCP 已更新':'MCP 已添加，可点击“刷新”验证连接');
  }catch(error){showError(el('mcp-form-error'),error)}
}
async function toggleMCPDisabled(m){
  const nextDisabled=!Boolean(m.disabled);
  try{
    await api('/api/mcps/'+encodeURIComponent(m.id)+'/state',{method:'PUT',body:JSON.stringify({disabled:nextDisabled})});
    state.health.delete(m.id);state.tools.delete(m.id);state.runtime.delete(m.id);
    await loadMCPs();
    toast(nextDisabled?'MCP 已禁用':'MCP 已启用');
  }catch(error){toast((nextDisabled?'禁用':'启用')+'失败：'+error.message)}
}
async function deleteMCP(m){if(!confirm('删除 MCP “'+m.name+'”？'))return;try{await api('/api/mcps/'+encodeURIComponent(m.id),{method:'DELETE'});state.health.delete(m.id);state.tools.delete(m.id);await loadMCPs();toast('已删除')}catch(error){toast(error.message)}}
async function disconnectMCP(id){
  try{
    await api('/api/runtime/'+encodeURIComponent(id)+'/disconnect',{method:'POST'});
    state.health.delete(id);state.tools.delete(id);await loadRuntimeStatuses();toast('Session 已断开');
  }catch(error){toast('断开失败：'+error.message)}
}

function openImportDialog(){
  clearError(el('import-error'));
  if(!el('import-content').value.trim())el('import-content').value='{'+'\n  "mcpServers": {'+'\n    "example": {'+'\n      "url": "https://example.com/mcp"'+'\n    }'+'\n  }'+'\n}';
  el('import-dialog').showModal();
}
async function loadImportFile(event){
  clearError(el('import-error'));
  const file=event.target.files?.[0];
  if(!file)return;
  if(file.size>2*1024*1024){
    showError(el('import-error'),'JSON 文件不能超过 2 MB');
    event.target.value='';
    return;
  }
  try{
    const content=await file.text();
    JSON.parse(content);
    el('import-content').value=content;
    el('import-format').value='auto';
    el('import-file-name').textContent=file.name+' · '+Math.max(1,Math.round(file.size/1024))+' KB';
  }catch(error){
    showError(el('import-error'),'无法读取 JSON 文件：'+error.message);
    event.target.value='';
  }
}
async function importMCPs(event){
  event.preventDefault();clearError(el('import-error'));
  const payload={format:el('import-format').value,conflict:el('import-conflict').value,content:el('import-content').value};
  try{
    const result=await api('/api/mcps/import',{method:'POST',body:JSON.stringify(payload)});
    state.health.clear();state.tools.clear();el('import-dialog').close();await loadMCPs();
    toast('导入完成：新增 '+result.added+'，更新 '+result.updated+'，跳过 '+result.skipped);
  }catch(error){showError(el('import-error'),error)}
}
async function exportMCPs(format){
  if(state.mock){
    let doc;
    if(format==='mcpServers'){
      const servers={};
      for(const m of mockData.mcps){servers[m.name]=m.transport==='stdio'?{command:m.executable,args:m.args||[],env:m.env_refs||undefined}:{url:m.endpoint,headers:m.header_refs||undefined}}
      doc={mcpServers:servers};
    }else{
      doc={version:1,mcps:mockData.mcps.map(({id,created_at,updated_at,...m})=>m)};
    }
    downloadJSON(JSON.stringify(doc,null,2),format==='mcpServers'?'mcp-servers-export.json':'mcp-center-export.json');return;
  }
  try{
    const response=await fetch('/api/mcps/export?format='+encodeURIComponent(format),{credentials:'same-origin'});
    if(!response.ok){let payload=null;try{payload=await response.json()}catch{}throw new Error(payload?.error||('HTTP '+response.status))}
    const blob=await response.blob();
    const disposition=response.headers.get('Content-Disposition')||'';
    const match=disposition.match(/filename="([^"]+)"/);
    downloadBlob(blob,match?.[1]||(format==='mcpServers'?'mcp-servers-export.json':'mcp-center-export.json'));
    toast('导出完成');
  }catch(error){toast('导出失败：'+error.message)}
}
function downloadJSON(content,filename){downloadBlob(new Blob([content],{type:'application/json;charset=utf-8'}),filename)}
function downloadBlob(blob,filename){const url=URL.createObjectURL(blob),a=document.createElement('a');a.href=url;a.download=filename;document.body.appendChild(a);a.click();a.remove();setTimeout(()=>URL.revokeObjectURL(url),1000)}


function refreshToolMCPSelect(){
  const select=el('tool-mcp'),current=select.value,enabled=state.mcps.filter(m=>!m.disabled);select.innerHTML='';
  for(const m of enabled){const o=document.createElement('option');o.value=m.id;o.textContent=m.name;select.appendChild(o)}
  if(current&&enabled.some(m=>m.id===current))select.value=current;
  if(select.value&&state.page==='tools')loadTools(select.value);else if(!select.value){el('tool-list').innerHTML='<div class="empty">暂无已启用 MCP</div>';selectTool(null)}
}
async function loadTools(id){
  try{const result=await api('/api/mcps/'+encodeURIComponent(id)+'/tools');state.tools.set(id,result.tools||[]);renderToolList(result.tools||[]);await loadRuntimeStatuses()}
  catch(error){el('tool-list').innerHTML='<div class="form-error">'+escapeHTML(error.message)+'</div>';await loadRuntimeStatuses().catch(()=>{})}
}
function renderToolList(tools){
  const box=el('tool-list'),mcpID=el('tool-mcp').value;
  const previousToolName=state.selectedToolMCPID===mcpID?state.selectedTool?.name:'';
  box.innerHTML='';
  if(!tools.length){box.innerHTML='<div class="empty">未发现工具</div>';selectTool(null);return}
  for(const tool of tools){
    const b=document.createElement('button');b.dataset.toolName=tool.name;
    b.innerHTML='<strong>'+escapeHTML(tool.name)+'</strong><small>'+escapeHTML(tool.description||'')+'</small>';
    b.onclick=()=>{box.querySelectorAll('button').forEach(x=>x.classList.remove('active'));b.classList.add('active');selectTool(tool)};
    box.appendChild(b);
  }
  const preferred=previousToolName?[...box.querySelectorAll('button')].find(b=>b.dataset.toolName===previousToolName):null;
  (preferred||box.querySelector('button'))?.click();
}
function normalizeToolSchema(tool){
  let schema=tool?.input_schema||{};
  if(typeof schema==='string'){try{schema=JSON.parse(schema)}catch{return {}}}
  return schema&&typeof schema==='object'&&!Array.isArray(schema)?schema:{};
}
function effectiveSchema(schema){
  if(!schema||typeof schema!=='object')return {};
  if(Array.isArray(schema.allOf)&&schema.allOf.length){
    const merged={...schema,properties:{...(schema.properties||{})},required:[...(schema.required||[])]};
    for(const part of schema.allOf){const p=effectiveSchema(part);Object.assign(merged.properties,p.properties||{});merged.required=[...new Set([...merged.required,...(p.required||[])])];if(!merged.type&&p.type)merged.type=p.type}
    return merged;
  }
  if(Array.isArray(schema.oneOf)&&schema.oneOf.length)return {...schema,...effectiveSchema(schema.oneOf[0])};
  if(Array.isArray(schema.anyOf)&&schema.anyOf.length){const first=schema.anyOf.find(x=>x?.type!=='null')||schema.anyOf[0];return {...schema,...effectiveSchema(first)}}
  return schema;
}
function schemaValueType(schema){
  schema=effectiveSchema(schema);
  let type=schema?.type;
  if(Array.isArray(type))type=type.find(x=>x!=='null')||type[0];
  if(type)return type;
  if(schema?.properties)return 'object';
  if(schema?.items)return 'array';
  if(schema?.default!==undefined)return Array.isArray(schema.default)?'array':typeof schema.default;
  return 'string';
}
function firstSchemaValue(schema){
  if(schema?.const!==undefined)return schema.const;
  if(schema?.default!==undefined)return schema.default;
  if(schema?.example!==undefined)return schema.example;
  if(Array.isArray(schema?.examples)&&schema.examples.length)return schema.examples[0];
  if(Array.isArray(schema?.enum)&&schema.enum.length)return schema.enum[0];
  return undefined;
}
function exampleStringForKey(key,schema){
  const format=String(schema?.format||'').toLowerCase(),name=String(key||'').toLowerCase();
  let value='example';
  if(name==='sql'||name.endsWith('_sql')||name.includes('statement'))value='SELECT 1 AS ok';
  else if(name.includes('database')||name==='db')value='database_name';
  else if(name.includes('table'))value='table_name';
  else if(name.includes('url')||name.includes('uri')||format==='uri'||format==='url')value='https://example.com';
  else if(name.includes('path'))value='/path/to/resource';
  else if(name.includes('query')||name.includes('search'))value='search text';
  else if(name.includes('message')||name.includes('prompt')||name.includes('content')||name==='text')value='example text';
  else if(name==='host'||name.endsWith('_host'))value='127.0.0.1';
  else if(name.endsWith('_id')||name==='id')value='example-id';
  else if(name.includes('name'))value='example-name';
  else if(format==='email')value='user@example.com';
  else if(format==='date')value='2026-09-27';
  else if(format==='date-time')value='2026-09-27T12:00:00Z';
  const min=Number(schema?.minLength||0);
  while(value.length<min)value+='x';
  return value;
}
function buildSchemaExample(rawSchema,key='',depth=0){
  const schema=effectiveSchema(rawSchema);
  const explicit=firstSchemaValue(schema);if(explicit!==undefined)return typeof structuredClone==='function'?structuredClone(explicit):JSON.parse(JSON.stringify(explicit));
  if(depth>5)return null;
  const type=schemaValueType(schema);
  if(type==='object'){
    const out={},required=new Set(schema.required||[]);
    for(const [name,child] of Object.entries(schema.properties||{})){
      if(required.has(name)||firstSchemaValue(effectiveSchema(child))!==undefined)out[name]=buildSchemaExample(child,name,depth+1);
    }
    return out;
  }
  if(type==='array'){const min=Math.max(1,Number(schema.minItems||0));const item=buildSchemaExample(schema.items||{},key,depth+1);return Array.from({length:min},()=>item)}
  if(type==='integer'){const n=Number.isFinite(Number(schema.minimum))?Math.ceil(Number(schema.minimum)):1;return n}
  if(type==='number'){const n=Number.isFinite(Number(schema.minimum))?Number(schema.minimum):1;return n}
  if(type==='boolean')return false;
  if(type==='null')return null;
  return exampleStringForKey(key,schema);
}
function toolSchemaSummary(schema){
  schema=effectiveSchema(schema);const props=Object.keys(schema.properties||{}),required=schema.required||[];
  if(!props.length)return schemaValueType(schema)==='object'?'无固定字段':'JSON '+schemaValueType(schema);
  return '必填 '+required.length+' / 字段 '+props.length;
}
function createToolControl(name,rawSchema,required,value){
  const schema=effectiveSchema(rawSchema),type=schemaValueType(schema),wrap=document.createElement('div');
  wrap.className='tool-param'+((type==='object'||type==='array'||(type==='string'&&(/sql|query|prompt|content|text|message/i.test(name)||String(schema.description||'').length>100)))?' wide':'');
  const head=document.createElement('div');head.className='tool-param-head';
  const title=document.createElement('div');title.className='tool-param-name';title.innerHTML='<code>'+escapeHTML(name)+'</code>'+(required?'<span class="tool-param-required">*</span>':'');
  const meta=document.createElement('span');meta.className='tool-param-meta';meta.textContent=type+(required?' · required':' · optional');head.append(title,meta);wrap.appendChild(head);
  const desc=document.createElement('div');desc.className='tool-param-desc';desc.textContent=schema.description||'';wrap.appendChild(desc);
  let control;
  if(Array.isArray(schema.enum)&&schema.enum.length){
    control=document.createElement('select');if(!required){const opt=document.createElement('option');opt.value='';opt.textContent='未设置';control.appendChild(opt)}
    for(const item of schema.enum){const opt=document.createElement('option');opt.value=JSON.stringify(item);opt.textContent=String(item);control.appendChild(opt)}
    if(value!==undefined)control.value=JSON.stringify(value);control.dataset.valueEncoding='json';
  }else if(type==='boolean'){
    control=document.createElement('select');if(!required){const blank=document.createElement('option');blank.value='';blank.textContent='未设置';control.appendChild(blank)}
    for(const item of [true,false]){const opt=document.createElement('option');opt.value=String(item);opt.textContent=String(item);control.appendChild(opt)}
    if(value!==undefined)control.value=String(Boolean(value));
  }else if(type==='object'||type==='array'){
    control=document.createElement('textarea');control.spellcheck=false;if(value!==undefined)control.value=JSON.stringify(value,null,2);
  }else if(type==='integer'||type==='number'){
    control=document.createElement('input');control.type='number';control.step=type==='integer'?'1':'any';if(schema.minimum!==undefined)control.min=String(schema.minimum);if(schema.maximum!==undefined)control.max=String(schema.maximum);if(value!==undefined)control.value=String(value);
  }else if(type==='string'&&(/sql|query|prompt|content|text|message/i.test(name)||String(schema.description||'').length>100)){
    control=document.createElement('textarea');control.spellcheck=false;control.value=value===undefined?'':String(value);
  }else{
    control=document.createElement('input');control.type='text';control.value=value===undefined?'':String(value);if(schema.minLength!==undefined)control.minLength=Number(schema.minLength);if(schema.maxLength!==undefined)control.maxLength=Number(schema.maxLength);
  }
  control.classList.add('tool-param-control');control.dataset.paramName=name;control.dataset.paramType=type;control.dataset.required=required?'1':'0';
  const sync=()=>{syncToolFormToJSON(false);saveCurrentToolState()};
  control.addEventListener('input',sync);control.addEventListener('change',sync);wrap.appendChild(control);return wrap;
}
function renderToolForm(schema,values={}){
  schema=effectiveSchema(schema);const box=el('tool-form');box.innerHTML='';const props=schema.properties||{},required=new Set(schema.required||[]);
  const entries=Object.entries(props);
  if(!entries.length){box.innerHTML='<div class="tool-form-empty">该 Schema 没有可解析的固定 key，可切换到 JSON 模式直接填写。</div>';return}
  for(const [name,child] of entries)box.appendChild(createToolControl(name,child,required.has(name),values?.[name]));
}
function collectToolForm(validateRequired=true){
  const out={};let firstError=null;
  document.querySelectorAll('#tool-form .tool-param-control').forEach(control=>{
    control.classList.remove('tool-param-invalid');const name=control.dataset.paramName,type=control.dataset.paramType,required=control.dataset.required==='1',raw=control.value;
    if(raw===''&&!required)return;
    if(raw===''&&required){control.classList.add('tool-param-invalid');firstError=firstError||('必填参数 '+name+' 不能为空');return}
    try{
      let value;
      if(control.dataset.valueEncoding==='json')value=JSON.parse(raw);
      else if(type==='boolean')value=raw==='true';
      else if(type==='integer'){value=Number(raw);if(!Number.isInteger(value))throw new Error('必须是整数')}
      else if(type==='number'){value=Number(raw);if(!Number.isFinite(value))throw new Error('必须是数字')}
      else if(type==='object'||type==='array'){value=JSON.parse(raw);if(type==='array'&&!Array.isArray(value))throw new Error('必须是 JSON 数组');if(type==='object'&&(Array.isArray(value)||value===null||typeof value!=='object'))throw new Error('必须是 JSON 对象')}
      else value=raw;
      out[name]=value;
    }catch(error){control.classList.add('tool-param-invalid');firstError=firstError||(name+'：'+error.message)}
  });
  if(validateRequired&&firstError)throw new Error(firstError);return out;
}
function syncToolFormToJSON(showError=true){
  try{const args=collectToolForm(showError);el('tool-args').value=JSON.stringify(args,null,2);if(showError)clearError(el('tool-args-error'));return args}
  catch(error){if(showError)showError(el('tool-args-error'),error);throw error}
}
function toolHistoryKey(mcpID=state.selectedToolMCPID,tool=state.selectedTool){
  return mcpID&&tool?.name?mcpID+'::'+tool.name:'';
}
function saveCurrentToolState(){
  const key=toolHistoryKey();if(!key)return;
  const previous=state.toolHistory.get(key)||{};
  state.toolHistory.set(key,{
    ...previous,
    mode:state.toolParamMode,
    argsJSON:el('tool-args').value||'{}',
    resultText:el('tool-result').textContent||'等待调用…'
  });
}
function applyToolParamModeUI(mode){
  state.toolParamMode=mode;
  el('tool-form').classList.toggle('hidden',mode!=='form');
  el('tool-json-wrap').classList.toggle('hidden',mode!=='json');
  el('tool-mode-form').classList.toggle('active',mode==='form');
  el('tool-mode-json').classList.toggle('active',mode==='json');
}
function setToolParamMode(mode){
  if(mode!=='form'&&mode!=='json')return;
  if(mode==='form'&&state.toolParamMode==='json'){
    try{
      const values=JSON.parse(el('tool-args').value||'{}');
      if(values===null||Array.isArray(values)||typeof values!=='object')throw new Error('调用参数必须是 JSON 对象');
      renderToolForm(normalizeToolSchema(state.selectedTool),values);
      clearError(el('tool-args-error'));
    }catch(error){
      showError(el('tool-args-error'),'JSON 参数格式错误：'+error.message);
      return;
    }
  }else if(mode==='json'&&state.toolParamMode==='form'){
    try{syncToolFormToJSON(false)}catch{}
  }
  applyToolParamModeUI(mode);
  saveCurrentToolState();
}
function applyToolExample(){
  if(!state.selectedTool)return;
  const schema=normalizeToolSchema(state.selectedTool),example=buildSchemaExample(schema)??{};
  el('tool-args').value=JSON.stringify(example,null,2);
  renderToolForm(schema,example);
  clearError(el('tool-args-error'));
  saveCurrentToolState();
}
function selectTool(tool){
  saveCurrentToolState();
  const mcpID=el('tool-mcp').value||'';
  state.selectedTool=tool;
  state.selectedToolMCPID=tool?mcpID:'';
  el('tool-name').textContent=tool?.name||'选择一个工具';
  el('tool-desc').textContent=tool?.description||'工具描述会显示在这里。';
  const schema=normalizeToolSchema(tool);
  el('tool-schema').textContent=json(schema);
  el('tool-required-summary').textContent=tool?toolSchemaSummary(schema):'未选择';
  el('tool-call').disabled=!tool;
  el('tool-use-example').disabled=!tool;
  clearError(el('tool-args-error'));
  if(!tool){
    el('tool-example').textContent='{}';
    el('tool-args').value='{}';
    el('tool-result').textContent='等待调用…';
    renderToolForm({},{});
    applyToolParamModeUI('form');
    return;
  }

  const example=buildSchemaExample(schema)??{};
  el('tool-example').textContent=JSON.stringify(example,null,2);
  const cached=state.toolHistory.get(toolHistoryKey(mcpID,tool));
  if(cached){
    el('tool-args').value=cached.argsJSON||JSON.stringify(example,null,2);
    let values=example;
    try{
      const parsed=JSON.parse(el('tool-args').value||'{}');
      if(parsed&&typeof parsed==='object'&&!Array.isArray(parsed))values=parsed;
    }catch{}
    renderToolForm(schema,values);
    el('tool-result').textContent=cached.resultText||'等待调用…';
    applyToolParamModeUI(cached.mode==='json'?'json':'form');
  }else{
    el('tool-args').value=JSON.stringify(example,null,2);
    renderToolForm(schema,example);
    el('tool-result').textContent='等待调用…';
    applyToolParamModeUI('form');
    saveCurrentToolState();
  }
}
async function callTool(){
  const mcpID=el('tool-mcp').value;if(!mcpID||!state.selectedTool)return;
  clearError(el('tool-args-error'));
  const tool=state.selectedTool,key=toolHistoryKey(mcpID,tool),mode=state.toolParamMode;
  let args;
  try{
    if(mode==='form')args=syncToolFormToJSON(true);
    else{
      args=JSON.parse(el('tool-args').value||'{}');
      if(args===null||Array.isArray(args)||typeof args!=='object')throw new Error('调用参数必须是 JSON 对象');
    }
  }catch(error){
    showError(el('tool-args-error'),error.message||String(error));
    return;
  }

  const argsJSON=JSON.stringify(args,null,2);
  state.toolHistory.set(key,{mode,argsJSON,resultText:'调用中…'});
  el('tool-result').textContent='调用中…';el('tool-call').disabled=true;
  let resultText;
  try{
    const result=await api('/api/mcps/'+encodeURIComponent(mcpID)+'/call',{method:'POST',body:JSON.stringify({name:tool.name,arguments:args})});
    resultText=json(result);
  }catch(error){
    resultText='ERROR: '+error.message;
  }finally{
    const latest=state.toolHistory.get(key)||{mode,argsJSON};
    state.toolHistory.set(key,{...latest,resultText});
    if(toolHistoryKey()===key){
      el('tool-result').textContent=resultText;
      el('tool-call').disabled=false;
    }else{
      el('tool-call').disabled=!state.selectedTool;
    }
    loadAuditSummary().catch(()=>{});
  }
}
async function loadOAuth(){
  try{
    const [meta,clientsResult]=await Promise.all([api('/.well-known/oauth-authorization-server'),api('/api/oauth/clients')]);
    const box=el('oauth-meta');box.innerHTML='';
    for(const key of ['issuer','authorization_endpoint','token_endpoint']){
      const line=document.createElement('div');
      line.innerHTML='<strong>'+escapeHTML(key)+'</strong><div class="muted">'+escapeHTML(meta[key]||'—')+'</div>';
      box.appendChild(line);
    }
    state.oauthClients=clientsResult.clients||[];
    renderOAuthClients();
  }catch(error){
    el('oauth-meta').innerHTML='<span class="form-error">'+escapeHTML(error.message)+'</span>';
    state.oauthClients=[];renderOAuthClients();
  }
}
function renderOAuthClients(){
  const body=el('oauth-client-rows');body.innerHTML='';
  for(const item of state.oauthClients){
    const redirects=(item.redirect_uris||[]).join('\n');
    const tr=document.createElement('tr');
    tr.innerHTML='<td><strong>'+escapeHTML(item.client_name||'未命名')+'</strong></td><td><span class="muted">'+escapeHTML(item.client_id||'')+'</span></td><td><span class="muted">'+escapeHTML(redirects||'—').replace(/\n/g,'<br>')+'</span></td><td>'+escapeHTML(item.created_at?new Date(item.created_at).toLocaleString():'—')+'</td><td><div class="row"><button class="button ghost small" data-edit>编辑</button><button class="button ghost small" data-delete>删除</button></div></td>';
    tr.querySelector('[data-edit]').onclick=()=>openOAuthClientDialog(item);
    tr.querySelector('[data-delete]').onclick=()=>deleteOAuthClient(item);
    body.appendChild(tr);
  }
  const empty=state.oauthClients.length===0;
  el('oauth-client-table-wrap').classList.toggle('hidden',empty);
  el('oauth-client-empty').classList.toggle('hidden',!empty);
}
function openOAuthClientDialog(item=null){
  el('oauth-client-dialog-title').textContent=item?'编辑 OAuth Client':'创建 OAuth Client';
  el('oauth-client-id').value=item?.client_id||'';
  el('oauth-client-name').value=item?.client_name||'';
  el('oauth-client-redirects').value=(item?.redirect_uris||[]).join('\n');
  clearError(el('oauth-client-error'));
  el('oauth-client-dialog').showModal();
}
async function saveOAuthClient(event){
  event.preventDefault();clearError(el('oauth-client-error'));
  const id=el('oauth-client-id').value.trim();
  const redirects=el('oauth-client-redirects').value.split(/\r?\n/).map(x=>x.trim()).filter(Boolean);
  const payload={client_name:el('oauth-client-name').value.trim(),redirect_uris:redirects,grant_types:['authorization_code','refresh_token'],response_types:['code']};
  try{
    await api(id?'/api/oauth/clients/'+encodeURIComponent(id):'/api/oauth/clients',{method:id?'PUT':'POST',body:JSON.stringify(payload)});
    el('oauth-client-dialog').close();await loadOAuth();toast(id?'OAuth Client 已更新，旧授权已撤销':'OAuth Client 已创建');
  }catch(error){showError(el('oauth-client-error'),error)}
}
async function deleteOAuthClient(item){
  if(!confirm('删除 OAuth Client “'+(item.client_name||item.client_id)+'”？已有授权码和 Token 会同时失效。'))return;
  try{await api('/api/oauth/clients/'+encodeURIComponent(item.client_id),{method:'DELETE'});await loadOAuth();toast('OAuth Client 已删除，旧授权已撤销')}catch(error){toast(error.message)}
}
function stopOAuthSelfTestPolling(){
  if(oauthTestPollTimer){clearInterval(oauthTestPollTimer);oauthTestPollTimer=null}
}
function oauthTestStatusLabel(status){
  if(status==='passed')return ['通过','ok'];
  if(status==='failed')return ['失败','bad'];
  if(status==='running')return ['测试中','warn'];
  if(status==='waiting_authorization')return ['等待授权','warn'];
  return ['等待开始',''];
}
function renderOAuthSelfTest(test){
  if(!test)return;
  state.oauthTestStatus=test.status||'';
  el('oauth-test-resource').textContent=test.resource||'—';
  const [label,klass]=oauthTestStatusLabel(test.status);
  const badge=el('oauth-test-status');badge.textContent=label;badge.className='status '+klass;
  el('oauth-test-message').textContent=test.status==='passed'
    ?('OAuth 端到端自测通过'+(test.tool_count!==undefined?'，发现 '+test.tool_count+' 个工具':''))
    :(test.status==='failed'?'测试未通过':'请在授权窗口中继续操作');
  const box=el('oauth-test-steps');box.innerHTML='';
  for(const step of (test.steps||[])){
    const row=document.createElement('div');row.className='oauth-test-step';
    const [stepLabel,stepClass]=step.status==='passed'?['通过','ok']:step.status==='failed'?['失败','bad']:(step.status==='running'||step.status==='waiting')?['进行中','warn']:['等待',''];
    row.innerHTML='<strong>'+escapeHTML(step.label||step.key||'')+'</strong><span class="status '+stepClass+'">'+stepLabel+'</span><small>'+escapeHTML(step.detail||'')+'</small>';
    box.appendChild(row);
  }
  clearError(el('oauth-test-error'));
  if(test.error)showError(el('oauth-test-error'),test.error);
  if(test.status==='passed'||test.status==='failed'){
    stopOAuthSelfTestPolling();
    el('oauth-test-open-authorize').classList.add('hidden');
  }
}
async function pollOAuthSelfTest(){
  if(!state.oauthTestID)return;
  try{
    const result=await api('/api/oauth/tests/'+encodeURIComponent(state.oauthTestID));
    renderOAuthSelfTest(result.test);
  }catch(error){
    stopOAuthSelfTestPolling();
    showError(el('oauth-test-error'),error);
  }
}
function startOAuthSelfTestPolling(){
  stopOAuthSelfTestPolling();
  pollOAuthSelfTest();
  oauthTestPollTimer=setInterval(pollOAuthSelfTest,900);
}
function openOAuthSelfTestDialog(){
  stopOAuthSelfTestPolling();
  state.oauthTestID='';state.oauthTestStatus='';state.oauthTestAuthorizeURL='';
  const select=el('oauth-test-mcp');select.innerHTML='';
  const enabled=state.mcps.filter(m=>!m.disabled);
  for(const m of enabled){
    const option=document.createElement('option');option.value=m.name;option.textContent=m.name+' · '+m.transport;select.appendChild(option);
  }
  const empty=enabled.length===0;
  el('oauth-test-start').disabled=empty;
  el('oauth-test-start-actions').classList.remove('hidden');
  el('oauth-test-progress').classList.add('hidden');
  el('oauth-test-open-authorize').classList.add('hidden');
  clearError(el('oauth-test-error'));
  el('oauth-test-dialog').showModal();
}
async function startOAuthSelfTest(){
  const mcpRef=el('oauth-test-mcp').value;
  if(!mcpRef){toast('请先添加一个 MCP');return}
  const popup=window.open('about:blank','mcp-center-oauth-self-test','width=720,height=760');
  if(popup){
    try{popup.document.write('<!doctype html><title>OAuth 自测</title><p style="font-family:sans-serif;padding:24px">正在准备 OAuth 授权…</p>')}catch{}
  }
  el('oauth-test-start').disabled=true;
  try{
    const result=await api('/api/oauth/tests',{method:'POST',body:JSON.stringify({mcp_ref:mcpRef})});
    state.oauthTestID=result.test.id;
    state.oauthTestStatus=result.test.status;
    state.oauthTestAuthorizeURL=result.authorize_url;
    el('oauth-test-start-actions').classList.add('hidden');
    el('oauth-test-progress').classList.remove('hidden');
    renderOAuthSelfTest(result.test);
    if(popup)popup.location=result.authorize_url;
    else{
      el('oauth-test-open-authorize').classList.remove('hidden');
      showError(el('oauth-test-error'),'浏览器拦截了授权窗口，请点击“打开授权页”。');
    }
    startOAuthSelfTestPolling();
  }catch(error){
    if(popup&&!popup.closed)popup.close();
    showError(el('oauth-test-error'),error);
    el('oauth-test-start').disabled=false;
  }
}
function reopenOAuthSelfTestAuthorize(){
  if(!state.oauthTestAuthorizeURL)return;
  const opened=window.open(state.oauthTestAuthorizeURL,'mcp-center-oauth-self-test','width=720,height=760');
  if(!opened){showError(el('oauth-test-error'),'浏览器仍然拦截弹窗，请允许此站点打开弹窗。')}
}
async function closeOAuthSelfTestDialog(){
  stopOAuthSelfTestPolling();
  if(state.oauthTestID&&state.oauthTestStatus!=='passed'&&state.oauthTestStatus!=='failed'){
    try{await api('/api/oauth/tests/'+encodeURIComponent(state.oauthTestID),{method:'DELETE'})}catch{}
  }
  state.oauthTestID='';state.oauthTestStatus='';state.oauthTestAuthorizeURL='';
  el('oauth-test-dialog').close();
}
async function copyTextValue(value,successMessage='已复制'){
  try{
    await navigator.clipboard.writeText(value);
  }catch{
    const input=document.createElement('textarea');
    input.value=value;input.style.position='fixed';input.style.opacity='0';
    document.body.appendChild(input);input.select();
    const ok=document.execCommand('copy');input.remove();
    if(!ok)throw new Error('copy failed');
  }
  toast(successMessage);
}
function mcpExternalURL(mcp,kind='name'){
  const base=state.connectionInfo?.base_url;
  if(!base||!mcp)return '';
  const ref=kind==='id'?mcp.id:mcp.name;
  return base.replace(/\/$/,'')+'/mcp/'+encodeURIComponent(ref);
}
async function copyMCPExternalURL(mcp,kind='name'){
  if(!state.connectionInfo)await loadConnectionInfo();
  const url=mcpExternalURL(mcp,kind);
  if(!url){toast('暂时无法获取接入地址');return}
  await copyTextValue(url,kind==='id'?'MCP ID 地址已复制':'MCP Name 地址已复制');
}
async function loadConnectionInfo(){
  state.connectionInfo=await api('/api/connection-info');
  renderConnectionInfo();
}
function renderConnectionInfo(){
  const info=state.connectionInfo;
  if(!info)return;
  el('connection-base-url').textContent=info.base_url||'—';
  const status=el('connection-base-status');
  status.textContent=info.is_loopback?'仅本机':'外部地址';
  status.className='status '+(info.is_loopback?'warn':'ok');
  el('connection-loopback-warning').classList.toggle('hidden',!info.is_loopback);
  const box=el('connection-endpoints');box.innerHTML='';
  for(const m of state.mcps){
    const nameURL=mcpExternalURL(m,'name'),idURL=mcpExternalURL(m,'id');
    const row=document.createElement('div');row.className='connection-endpoint';
    row.innerHTML='<div class="connection-endpoint-main"><div class="row"><strong>'+escapeHTML(m.name)+'</strong>'+(m.disabled?'<span class="status warn">已禁用</span>':'<span class="status ok">已启用</span>')+'</div><small>MCP ID: '+escapeHTML(m.id)+(m.disabled?' · 当前地址不可调用':'')+'</small><div class="connection-address"><span>Name 地址</span><code>'+escapeHTML(nameURL)+'</code><button class="button ghost small" type="button" data-copy-name>复制</button></div><div class="connection-address"><span>ID 地址</span><code>'+escapeHTML(idURL)+'</code><button class="button ghost small" type="button" data-copy-id>复制</button></div></div>';
    row.querySelector('[data-copy-name]').onclick=()=>copyMCPExternalURL(m,'name');
    row.querySelector('[data-copy-id]').onclick=()=>copyMCPExternalURL(m,'id');
    box.appendChild(row);
  }
  const empty=state.mcps.length===0;
  box.classList.toggle('hidden',empty);
  el('connection-endpoints-empty').classList.toggle('hidden',!empty);
}
async function loadVersion(){
  const info=await api('/api/version');
  const node=el('settings-version');
  if(!node)return;
  let value=info.version||'dev';
  if(info.commit)value+=' ('+info.commit+(info.modified?', modified':'')+')';
  node.textContent=value;
}
async function loadSystemSettings(){
  const result=await api('/api/settings');
  state.systemSettings=result.settings||{audit_retention_days:30};
  el('settings-audit-retention').value=state.systemSettings.audit_retention_days??30;
}
async function saveSystemSettings(){
  clearError(el('settings-system-error'));
  const days=Number(el('settings-audit-retention').value);
  if(!Number.isInteger(days)||days<1||days>3650){showError(el('settings-system-error'),'审计日志保留天数必须是 1 到 3650 的整数');return}
  try{
    const result=await api('/api/settings',{method:'PUT',body:JSON.stringify({audit_retention_days:days})});
    state.systemSettings=result.settings;
    await Promise.all([loadAuditSummary(),loadAudit().catch(()=>{})]);
    toast('日志策略已保存，过期日志已清理');
  }catch(error){showError(el('settings-system-error'),error)}
}
async function loadUpstreamSecrets(){
  const result=await api('/api/upstream-secrets');
  state.upstreamSecrets=result.secrets||[];
  renderUpstreamSecrets();
}
function renderUpstreamSecrets(){
  const body=el('upstream-secret-rows');if(!body)return;
  body.innerHTML='';
  for(const item of state.upstreamSecrets){
    const tr=document.createElement('tr');
    const updated=item.updated_at?new Date(item.updated_at).toLocaleString():'—';
    tr.innerHTML='<td><strong>'+escapeHTML(item.name)+'</strong></td><td class="muted">'+escapeHTML(updated)+'</td><td><div class="row"><button class="button ghost small" data-replace>替换值</button><button class="button ghost small" data-delete>删除</button></div></td>';
    tr.querySelector('[data-replace]').onclick=()=>openUpstreamSecretDialog(item);
    tr.querySelector('[data-delete]').onclick=()=>deleteUpstreamSecret(item);
    body.appendChild(tr);
  }
  const empty=state.upstreamSecrets.length===0;
  el('upstream-secret-table-wrap').classList.toggle('hidden',empty);
  el('upstream-secret-empty').classList.toggle('hidden',!empty);
}
function openUpstreamSecretDialog(item=null){
  clearError(el('upstream-secret-error'));
  el('upstream-secret-dialog-title').textContent=item?'替换上游密钥变量':'添加上游密钥变量';
  el('upstream-secret-name').value=item?.name||'';
  el('upstream-secret-name').readOnly=Boolean(item);
  el('upstream-secret-value').value='';
  el('upstream-secret-dialog').showModal();
  setTimeout(()=>el(item?'upstream-secret-value':'upstream-secret-name').focus(),0);
}
async function saveUpstreamSecret(event){
  event.preventDefault();clearError(el('upstream-secret-error'));
  const name=el('upstream-secret-name').value.trim(),value=el('upstream-secret-value').value;
  if(!/^[A-Za-z_][A-Za-z0-9_]*$/.test(name)){showError(el('upstream-secret-error'),'变量名只能使用字母、数字、下划线，且不能以数字开头');return}
  if(!value){showError(el('upstream-secret-error'),'变量值不能为空');return}
  try{
    await api('/api/upstream-secrets/'+encodeURIComponent(name),{method:'PUT',body:JSON.stringify({value})});
    el('upstream-secret-dialog').close();el('upstream-secret-value').value='';
    await loadUpstreamSecrets();await loadRuntimeStatuses().catch(()=>{});
    toast('变量已保存，相关 MCP Session 已断开');
  }catch(error){showError(el('upstream-secret-error'),error)}
}
async function deleteUpstreamSecret(item){
  if(!confirm('删除上游密钥变量 “'+item.name+'”？引用它的 MCP 将无法连接，直到重新配置变量。'))return;
  try{
    await api('/api/upstream-secrets/'+encodeURIComponent(item.name),{method:'DELETE'});
    await loadUpstreamSecrets();await loadRuntimeStatuses().catch(()=>{});
    toast('变量已删除，相关 MCP Session 已断开');
  }catch(error){toast('删除失败：'+error.message)}
}
function renderAccountSettings(){
  const input=el('settings-username');
  if(input)input.value=state.user?.username||'admin';
}
async function changePassword(){
  clearError(el('settings-password-error'));
  const current=el('settings-current-password').value;
  const next=el('settings-new-password').value;
  const confirmPassword=el('settings-confirm-password').value;
  if(next!==confirmPassword){showError(el('settings-password-error'),'两次输入的新密码不一致');return}
  if(next.length<10){showError(el('settings-password-error'),'新密码至少需要 10 位');return}
  try{
    const result=await api('/api/account/password',{method:'POST',body:JSON.stringify({current_password:current,new_password:next})});
    state.user=result.user;
    el('me-name').textContent=state.user?.username||'admin';
    renderAccountSettings();
    el('settings-current-password').value='';
    el('settings-new-password').value='';
    el('settings-confirm-password').value='';
    toast('密码已修改，其他登录会话已失效');
  }catch(error){showError(el('settings-password-error'),error)}
}
async function loadAccess(){
  const [settingsResult,credentialsResult]=await Promise.all([api('/api/access/settings'),api('/api/access/credentials')]);
  state.accessSettings=settingsResult.settings;state.credentials=credentialsResult.credentials||[];
  el('setting-bearer').checked=Boolean(state.accessSettings.bearer_enabled);
  el('setting-header').checked=Boolean(state.accessSettings.header_enabled);
  el('setting-header-name').value=state.accessSettings.header_name||'X-MCP-Token';
  el('setting-query').checked=Boolean(state.accessSettings.query_enabled);
  el('setting-query-name').value=state.accessSettings.query_name||'mcp_token';
  renderCredentials();
}
function renderCredentials(){
  const body=el('credential-rows');body.innerHTML='';
  for(const item of state.credentials){
    const channels=[item.allow_bearer?'Bearer':'',item.allow_header?'Header':'',item.allow_query?'Query':''].filter(Boolean).join(' / ')||'—';
    const scopes=(item.scopes||[]).join(' ')||'—';
    const tr=document.createElement('tr');
    tr.innerHTML='<td><strong>'+escapeHTML(item.name)+'</strong></td><td><span class="muted">'+escapeHTML(item.secret_prefix||'')+'…</span></td><td>'+escapeHTML(channels)+'</td><td>'+escapeHTML(scopes)+'</td><td>'+(item.enabled?'<span class="status ok">启用</span>':'<span class="status">禁用</span>')+'</td><td class="muted">'+escapeHTML(item.last_used_at||'从未')+'</td><td><div class="row"><button class="button ghost small" data-edit>编辑</button><button class="button ghost small" data-delete>删除</button></div></td>';
    tr.querySelector('[data-edit]').onclick=()=>openCredentialDialog(item);
    tr.querySelector('[data-delete]').onclick=()=>deleteCredential(item);
    body.appendChild(tr);
  }
  const empty=state.credentials.length===0;
  body.closest('.table-wrap').classList.toggle('hidden',empty);
  el('credential-empty').classList.toggle('hidden',!empty);
}
function openCredentialDialog(item=null){
  el('credential-dialog-title').textContent=item?'编辑访问密钥':'创建访问密钥';
  el('credential-id').value=item?.id||'';
  el('credential-name').value=item?.name||'';
  el('credential-enabled').checked=item?Boolean(item.enabled):true;
  el('credential-bearer').checked=item?Boolean(item.allow_bearer):true;
  el('credential-header').checked=item?Boolean(item.allow_header):true;
  el('credential-query').checked=item?Boolean(item.allow_query):false;
  clearError(el('credential-form-error'));el('credential-dialog').showModal();
}
async function saveCredential(event){
  event.preventDefault();clearError(el('credential-form-error'));
  const id=el('credential-id').value.trim();
  const payload={name:el('credential-name').value.trim(),enabled:el('credential-enabled').checked,allow_bearer:el('credential-bearer').checked,allow_header:el('credential-header').checked,allow_query:el('credential-query').checked,scopes:['mcp:*']};
  try{
    const result=await api(id?'/api/access/credentials/'+encodeURIComponent(id):'/api/access/credentials',{method:id?'PUT':'POST',body:JSON.stringify(payload)});
    el('credential-dialog').close();await loadAccess();
    if(!id&&result.secret){el('created-secret').textContent=result.secret;el('secret-dialog').showModal()}
    else toast('访问密钥已更新');
  }catch(error){showError(el('credential-form-error'),error)}
}
async function deleteCredential(item){
  if(!confirm('删除访问密钥 “'+item.name+'”？'))return;
  try{await api('/api/access/credentials/'+encodeURIComponent(item.id),{method:'DELETE'});await loadAccess();toast('访问密钥已删除')}catch(error){toast(error.message)}
}
async function saveAccessSettings(){
  const payload={bearer_enabled:el('setting-bearer').checked,header_enabled:el('setting-header').checked,header_name:el('setting-header-name').value.trim(),query_enabled:el('setting-query').checked,query_name:el('setting-query-name').value.trim()};
  try{const result=await api('/api/access/settings',{method:'PUT',body:JSON.stringify(payload)});state.accessSettings=result.settings;toast('授权设置已保存')}catch(error){toast(error.message)}
}
async function loadAudit(){
  const result=await api('/api/audit?limit=500');
  state.audits=result.events||[];
  renderAudit();
}
function renderAudit(){
  const q=(el('audit-search')?.value||'').trim().toLowerCase();
  const items=state.audits.filter(item=>{
    if(!q)return true;
    return [item.request_id,item.principal_name,item.principal_id,item.auth_channel,item.mcp_id,item.tool_name,item.status,item.error_kind].some(v=>String(v||'').toLowerCase().includes(q));
  });
  const body=el('audit-rows');body.innerHTML='';
  for(const item of items){
    const tr=document.createElement('tr');
    const when=item.started_at?new Date(item.started_at).toLocaleString():'—';
    const principal=item.principal_name||item.principal_id||'—';
    const auth=[item.auth_channel,item.source].filter(Boolean).join(' / ')||'—';
    const status=item.status==='success'?'<span class="status ok">成功</span>':'<span class="status bad">'+escapeHTML(item.error_kind||'失败')+'</span>';
    const keys=Array.isArray(item.input_meta?.keys)?item.input_meta.keys.join(', '):'';
    const summary=keys?('keys: '+keys):'无参数';
    tr.innerHTML='<td>'+escapeHTML(when)+'</td><td><span class="muted">'+escapeHTML(item.request_id||'—')+'</span></td><td>'+escapeHTML(principal)+'</td><td>'+escapeHTML(auth)+'</td><td><strong>'+escapeHTML(item.mcp_id||'—')+'</strong><br><span class="muted">'+escapeHTML(item.tool_name||'—')+'</span></td><td>'+escapeHTML(String(item.duration_ms??0))+' ms</td><td>'+status+'</td><td><span class="muted">'+escapeHTML(summary)+'</span></td>';
    body.appendChild(tr);
  }
  const empty=items.length===0;
  el('audit-table-wrap').classList.toggle('hidden',empty);
  el('audit-empty').classList.toggle('hidden',!empty);
}
async function clearAudit(){
  if(!confirm('清空全部调用日志？此操作不可恢复。'))return;
  try{await api('/api/audit',{method:'DELETE'});state.audits=[];state.auditSummary={total:0,success:0,errors:0,recent_errors:[]};renderAudit();renderDashboard();toast('调用日志已清空')}catch(error){toast(error.message)}
}
function escapeHTML(value){return String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))}

document.addEventListener('click',event=>{
  const page=event.target.closest('[data-page]')?.dataset.page;if(page)switchPage(page);
  const link=event.target.closest('[data-page-link]')?.dataset.pageLink;if(link)switchPage(link);
  if(event.target.closest('[data-action="open-add"]'))openMCPDialog();
  if(event.target.closest('[data-action="close-dialog"]'))el('mcp-dialog').close();
  if(event.target.closest('[data-action="close-credential-dialog"]'))el('credential-dialog').close();
  if(event.target.closest('[data-action="close-import-dialog"]'))el('import-dialog').close();
  if(event.target.closest('[data-action="close-oauth-client-dialog"]'))el('oauth-client-dialog').close();
  if(event.target.closest('[data-action="close-oauth-test-dialog"]'))closeOAuthSelfTestDialog();
  if(event.target.closest('[data-action="close-upstream-secret-dialog"]'))el('upstream-secret-dialog').close();
});
el('auth-form').addEventListener('submit',submitAuth);
el('logout').onclick=async()=>{try{await api('/api/auth/logout',{method:'POST'})}finally{state.user=null;showAuth(false)}};
el('refresh').onclick=async()=>{try{await Promise.all([loadMCPs(),loadAuditSummary()]);toast('已刷新')}catch(error){toast(error.message)}};
el('probe-all').onclick=probeAll;
el('open-import').onclick=openImportDialog;
document.querySelectorAll('[data-export-format]').forEach(button=>button.addEventListener('click',()=>{
  const format=button.dataset.exportFormat;
  el('export-menu').removeAttribute('open');
  exportMCPs(format);
}));
document.addEventListener('click',event=>{
  const menu=el('export-menu');
  if(menu?.hasAttribute('open')&&!menu.contains(event.target))menu.removeAttribute('open');
});
el('import-form').addEventListener('submit',importMCPs);
el('import-file').addEventListener('change',loadImportFile);
el('mcp-search').addEventListener('input',renderMCPs);
el('mcp-status-filter').addEventListener('change',renderMCPs);
el('mcp-transport').addEventListener('change',updateTransportFields);
el('mcp-auth').addEventListener('change',updateAuthFields);
el('mcp-form').addEventListener('submit',saveMCP);
el('tool-mcp').addEventListener('change',event=>{selectTool(null);loadTools(event.target.value)});
el('tool-args').addEventListener('input',saveCurrentToolState);
el('tool-mode-form').onclick=()=>setToolParamMode('form');
el('tool-mode-json').onclick=()=>setToolParamMode('json');
el('tool-use-example').onclick=applyToolExample;
el('tool-call').onclick=callTool;
el('copy-base-url').onclick=()=>state.connectionInfo?.base_url&&copyTextValue(state.connectionInfo.base_url,'Base URL 已复制');
el('add-credential').onclick=()=>openCredentialDialog();
el('oauth-self-test').onclick=openOAuthSelfTestDialog;
el('oauth-test-start').onclick=startOAuthSelfTest;
el('oauth-test-open-authorize').onclick=reopenOAuthSelfTestAuthorize;
el('add-oauth-client').onclick=()=>openOAuthClientDialog();
el('add-upstream-secret').onclick=()=>openUpstreamSecretDialog();
el('oauth-client-form').addEventListener('submit',saveOAuthClient);
el('upstream-secret-form').addEventListener('submit',saveUpstreamSecret);
el('save-access-settings').onclick=saveAccessSettings;
el('credential-form').addEventListener('submit',saveCredential);
el('close-secret').onclick=()=>el('secret-dialog').close();
el('copy-secret').onclick=async()=>{try{await navigator.clipboard.writeText(el('created-secret').textContent);toast('密钥已复制')}catch{toast('复制失败，请手动复制')}};
el('refresh-audit').onclick=()=>loadAudit().then(()=>toast('日志已刷新')).catch(error=>toast(error.message));
el('clear-audit').onclick=clearAudit;
el('audit-search').addEventListener('input',renderAudit);
el('change-password').onclick=changePassword;
el('save-system-settings').onclick=saveSystemSettings;
bootstrap();
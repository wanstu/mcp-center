if(globalThis.desktopKitTheme)globalThis.desktopKitTheme.apply('system');
const state={user:null,needsBootstrap:false,mcps:[],health:new Map(),tools:new Map(),selectedTool:null,accessSettings:null,credentials:[],audits:[],page:'dashboard',mock:new URLSearchParams(location.search).get('mock')==='1'};
const el=id=>document.getElementById(id);
const json=v=>JSON.stringify(v,null,2);
const pageNames={dashboard:'工作台',mcps:'MCP 服务',tools:'工具测试',access:'访问授权',logs:'调用日志',settings:'设置'};

function toast(message){const node=el('toast');node.textContent=message;node.classList.add('show');clearTimeout(window.__toastTimer);window.__toastTimer=setTimeout(()=>node.classList.remove('show'),2200)}
function showError(node,error){node.textContent=String(error?.message||error);node.classList.remove('hidden')}
function clearError(node){node.textContent='';node.classList.add('hidden')}

const mockData={mcps:[
{id:'mcp_demo_adm',name:'pj-adm',transport:'streamable-http',auth_mode:'none',endpoint:'http://127.0.0.1:8001/mcp',default_include_in_environment:true},
{id:'mcp_demo_db',name:'wm-test-db',transport:'stdio',auth_mode:'none',executable:'mysql-mcp',args:['--readonly'],default_include_in_environment:false}
],tools:{mcp_demo_adm:[{name:'environment_list',description:'List lightweight Environment summaries',input_schema:{type:'object',properties:{}}},{name:'read',description:'Read a text file',input_schema:{type:'object',properties:{path:{type:'string'}},required:['path']}}],mcp_demo_db:[{name:'query',description:'Run a read-only query',input_schema:{type:'object',properties:{sql:{type:'string'}},required:['sql']}}]},accessSettings:{bearer_enabled:true,header_enabled:true,header_name:'X-MCP-Token',query_enabled:false,query_name:'mcp_token'},credentials:[],audits:[{request_id:'req_demo',source:'console',principal_id:'usr_demo',principal_name:'admin',auth_channel:'session',mcp_id:'mcp_demo_adm',tool_name:'environment_list',started_at:new Date().toISOString(),duration_ms:42,status:'success',input_meta:{key_count:0,keys:[]},output_meta:{content_count:1,is_error:false,has_structured_content:true}}]};

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
  if(path==='/api/access/settings'&&method==='GET')return {settings:mockData.accessSettings};
  if(path==='/api/access/settings'&&method==='PUT'){mockData.accessSettings=JSON.parse(options.body);return {settings:mockData.accessSettings}}
  if(path==='/api/access/credentials'&&method==='GET')return {credentials:mockData.credentials};
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
  if(path==='/api/mcps'&&method==='GET')return {mcps:mockData.mcps};
  if(path==='/api/mcps'&&method==='POST'){const cfg=JSON.parse(options.body);const m={id:'mcp_'+Date.now(),...cfg};mockData.mcps.push(m);return {mcp:m}}
  const m=path.match(/^\/api\/mcps\/([^/]+)(?:\/(probe|tools|call))?$/);
  if(m){const id=m[1],action=m[2];
    if(method==='DELETE'){mockData.mcps=mockData.mcps.filter(x=>x.id!==id);return {ok:true}}
    if(method==='PUT'){const cfg=JSON.parse(options.body);const i=mockData.mcps.findIndex(x=>x.id===id);mockData.mcps[i]={...mockData.mcps[i],...cfg};return {mcp:mockData.mcps[i]}}
    if(action==='probe')return {mcp_id:id,state:'healthy',tools:mockData.tools[id]||[],checked_at:new Date().toISOString()};
    if(action==='tools')return {tools:mockData.tools[id]||[]};
    if(action==='call')return {content:[{type:'text',text:'mock result'}],structuredContent:{ok:true,arguments:JSON.parse(options.body).arguments||{}}};
  }
  if(path==='/.well-known/oauth-authorization-server')return {issuer:location.origin,authorization_endpoint:location.origin+'/oauth/authorize',token_endpoint:location.origin+'/oauth/token',registration_endpoint:location.origin+'/oauth/register',code_challenge_methods_supported:['S256']};
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
  await Promise.allSettled([loadMCPs(),loadOAuthMetadata(),loadAccess()]);
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
  if(page==='access')loadAccess().catch(error=>toast(error.message));
  if(page==='logs')loadAudit().catch(error=>toast(error.message));
}
async function loadMCPs(){
  const result=await api('/api/mcps');state.mcps=result.mcps||[];renderMCPs();renderDashboard();refreshToolMCPSelect();
}
function renderMCPs(){
  const q=el('mcp-search').value.trim().toLowerCase();
  const items=state.mcps.filter(m=>!q||[m.name,m.endpoint,m.executable,m.transport].some(v=>String(v||'').toLowerCase().includes(q)));
  const body=el('mcp-rows');body.innerHTML='';
  for(const m of items){
    const health=state.health.get(m.id),status=health?.state==='healthy'?'<span class="status ok">在线</span>':health?.state==='error'?'<span class="status bad">异常</span>':'<span class="status">未探测</span>';
    const tools=health?.tools?.length??state.tools.get(m.id)?.length??'—';
    const where=m.transport==='stdio'?escapeHTML([m.executable,...(m.args||[])].join(' ')):escapeHTML(m.endpoint||'');
    const tr=document.createElement('tr');
    tr.innerHTML='<td><strong>'+escapeHTML(m.name)+'</strong><br><small class="muted">'+escapeHTML(m.id)+'</small></td><td>'+escapeHTML(m.transport)+'</td><td>'+status+'</td><td>'+tools+'</td><td><span class="muted">'+where+'</span></td><td><div class="row"><button class="button small" data-op="probe">探测</button><button class="button small" data-op="test">测试</button><button class="button ghost small" data-op="edit">编辑</button><button class="button ghost small" data-op="delete">删除</button></div></td>';
    tr.querySelector('[data-op="probe"]').onclick=()=>probeMCP(m.id);
    tr.querySelector('[data-op="test"]').onclick=()=>{switchPage('tools');el('tool-mcp').value=m.id;loadTools(m.id)};
    tr.querySelector('[data-op="edit"]').onclick=()=>openMCPDialog(m);
    tr.querySelector('[data-op="delete"]').onclick=()=>deleteMCP(m);
    body.appendChild(tr);
  }
  el('mcp-table-wrap').classList.toggle('hidden',items.length===0);el('mcp-empty').classList.toggle('hidden',items.length!==0);
}
function renderDashboard(){
  el('metric-mcps').textContent=state.mcps.length;
  const online=[...state.health.values()].filter(x=>x.state==='healthy').length;
  el('metric-online').textContent=online;
  let tools=0;for(const list of state.tools.values())tools+=list.length;for(const h of state.health.values())if(h.tools)tools=Math.max(tools,h.tools.length);
  el('metric-tools').textContent=tools;el('metric-mcps-sub').textContent=state.mcps.length?'已加载配置':'尚未添加';
  const box=el('dashboard-list');box.innerHTML='';
  if(!state.mcps.length){box.className='empty';box.textContent='暂无 MCP 服务';return}
  box.className='service-list';
  for(const m of state.mcps.slice(0,6)){const row=document.createElement('div');row.className='service-item';const h=state.health.get(m.id);row.innerHTML='<div><strong>'+escapeHTML(m.name)+'</strong><small>'+escapeHTML(m.transport)+' · '+escapeHTML(m.endpoint||m.executable||'')+'</small></div>'+(h?.state==='healthy'?'<span class="status ok">在线</span>':'<span class="status">未探测</span>');box.appendChild(row)}
}
async function probeMCP(id){
  try{toast('正在探测…');const result=await api('/api/mcps/'+encodeURIComponent(id)+'/probe',{method:'POST'});state.health.set(id,result);if(result.tools)state.tools.set(id,result.tools);renderMCPs();renderDashboard();toast(result.state==='healthy'?'探测成功':'探测完成：'+result.state)}
  catch(error){state.health.set(id,{state:'error',message:error.message});renderMCPs();toast('探测失败：'+error.message)}
}
async function probeAll(){for(const m of state.mcps)await probeMCP(m.id)}
function openMCPDialog(m=null){
  el('mcp-dialog-title').textContent=m?'编辑 MCP':'添加 MCP';el('mcp-id').value=m?.id||'';el('mcp-name').value=m?.name||'';el('mcp-transport').value=m?.transport||'streamable-http';el('mcp-endpoint').value=m?.endpoint||'';el('mcp-executable').value=m?.executable||'';el('mcp-args').value=(m?.args||[]).join('\n');el('mcp-auth').value=m?.auth_mode||'none';el('mcp-default').checked=Boolean(m?.default_include_in_environment);clearError(el('mcp-form-error'));updateTransportFields();el('mcp-dialog').showModal();
}
function updateTransportFields(){const stdio=el('mcp-transport').value==='stdio';document.querySelectorAll('.stdio-only').forEach(n=>n.classList.toggle('hidden',!stdio));document.querySelectorAll('.http-only').forEach(n=>n.classList.toggle('hidden',stdio))}
async function saveMCP(event){
  event.preventDefault();clearError(el('mcp-form-error'));
  const id=el('mcp-id').value.trim(),transport=el('mcp-transport').value;
  const payload={name:el('mcp-name').value.trim(),transport,auth_mode:transport==='stdio'?'none':el('mcp-auth').value,default_include_in_environment:el('mcp-default').checked,health_policy:{health_check_enabled:false,auto_reconnect:false}};
  if(transport==='stdio'){payload.executable=el('mcp-executable').value.trim();payload.args=el('mcp-args').value.split(/\r?\n/).map(x=>x.trim()).filter(Boolean)}
  else payload.endpoint=el('mcp-endpoint').value.trim();
  try{await api(id?'/api/mcps/'+encodeURIComponent(id):'/api/mcps',{method:id?'PUT':'POST',body:JSON.stringify(payload)});el('mcp-dialog').close();await loadMCPs();toast(id?'MCP 已更新':'MCP 已添加')}
  catch(error){showError(el('mcp-form-error'),error)}
}
async function deleteMCP(m){if(!confirm('删除 MCP “'+m.name+'”？'))return;try{await api('/api/mcps/'+encodeURIComponent(m.id),{method:'DELETE'});state.health.delete(m.id);state.tools.delete(m.id);await loadMCPs();toast('已删除')}catch(error){toast(error.message)}}
function openImportDialog(){
  clearError(el('import-error'));
  if(!el('import-content').value.trim())el('import-content').value='{'+'\n  "mcpServers": {'+'\n    "example": {'+'\n      "url": "https://example.com/mcp"'+'\n    }'+'\n  }'+'\n}';
  el('import-dialog').showModal();
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
async function exportMCPs(){
  const format=el('export-format').value;
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
  const select=el('tool-mcp'),current=select.value;select.innerHTML='';
  for(const m of state.mcps){const o=document.createElement('option');o.value=m.id;o.textContent=m.name;select.appendChild(o)}
  if(current&&state.mcps.some(m=>m.id===current))select.value=current;
  if(select.value)loadTools(select.value);else{el('tool-list').innerHTML='<div class="empty">暂无 MCP</div>';selectTool(null)}
}
async function loadTools(id){
  try{const result=await api('/api/mcps/'+encodeURIComponent(id)+'/tools');state.tools.set(id,result.tools||[]);renderToolList(result.tools||[]);renderDashboard()}
  catch(error){el('tool-list').innerHTML='<div class="form-error">'+escapeHTML(error.message)+'</div>'}
}
function renderToolList(tools){
  const box=el('tool-list');box.innerHTML='';if(!tools.length){box.innerHTML='<div class="empty">未发现工具</div>';selectTool(null);return}
  for(const tool of tools){const b=document.createElement('button');b.innerHTML='<strong>'+escapeHTML(tool.name)+'</strong><small>'+escapeHTML(tool.description||'')+'</small>';b.onclick=()=>{box.querySelectorAll('button').forEach(x=>x.classList.remove('active'));b.classList.add('active');selectTool(tool)};box.appendChild(b)}
  box.querySelector('button')?.click();
}
function selectTool(tool){state.selectedTool=tool;el('tool-name').textContent=tool?.name||'选择一个工具';el('tool-desc').textContent=tool?.description||'工具描述会显示在这里。';el('tool-schema').textContent=json(tool?.input_schema||{});el('tool-call').disabled=!tool}
async function callTool(){
  const mcpID=el('tool-mcp').value;if(!mcpID||!state.selectedTool)return;
  let args;try{args=JSON.parse(el('tool-args').value||'{}')}catch(error){toast('参数不是合法 JSON');return}
  el('tool-result').textContent='调用中…';el('tool-call').disabled=true;
  try{const result=await api('/api/mcps/'+encodeURIComponent(mcpID)+'/call',{method:'POST',body:JSON.stringify({name:state.selectedTool.name,arguments:args})});el('tool-result').textContent=json(result)}
  catch(error){el('tool-result').textContent='ERROR: '+error.message}
  finally{el('tool-call').disabled=false}
}
async function loadOAuthMetadata(){
  try{const meta=await api('/.well-known/oauth-authorization-server');const box=el('oauth-meta');box.innerHTML='';for(const key of ['issuer','authorization_endpoint','token_endpoint','registration_endpoint']){const line=document.createElement('div');line.innerHTML='<strong>'+escapeHTML(key)+'</strong><div class="muted">'+escapeHTML(meta[key]||'—')+'</div>';box.appendChild(line)}}
  catch(error){el('oauth-meta').innerHTML='<span class="form-error">'+escapeHTML(error.message)+'</span>'}
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
  el('credential-scopes').value=(item?.scopes||['mcp:*']).join(' ');
  clearError(el('credential-form-error'));el('credential-dialog').showModal();
}
async function saveCredential(event){
  event.preventDefault();clearError(el('credential-form-error'));
  const id=el('credential-id').value.trim();
  const payload={name:el('credential-name').value.trim(),enabled:el('credential-enabled').checked,allow_bearer:el('credential-bearer').checked,allow_header:el('credential-header').checked,allow_query:el('credential-query').checked,scopes:el('credential-scopes').value.split(/\s+/).filter(Boolean)};
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
  try{await api('/api/audit',{method:'DELETE'});state.audits=[];renderAudit();toast('调用日志已清空')}catch(error){toast(error.message)}
}
function escapeHTML(value){return String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))}

document.addEventListener('click',event=>{
  const page=event.target.closest('[data-page]')?.dataset.page;if(page)switchPage(page);
  const link=event.target.closest('[data-page-link]')?.dataset.pageLink;if(link)switchPage(link);
  if(event.target.closest('[data-action="open-add"]'))openMCPDialog();
  if(event.target.closest('[data-action="close-dialog"]'))el('mcp-dialog').close();
  if(event.target.closest('[data-action="close-credential-dialog"]'))el('credential-dialog').close();
  if(event.target.closest('[data-action="close-import-dialog"]'))el('import-dialog').close();
});
el('auth-form').addEventListener('submit',submitAuth);
el('logout').onclick=async()=>{try{await api('/api/auth/logout',{method:'POST'})}finally{state.user=null;showAuth(false)}};
el('refresh').onclick=async()=>{try{await loadMCPs();toast('已刷新')}catch(error){toast(error.message)}};
el('probe-all').onclick=probeAll;
el('open-import').onclick=openImportDialog;
el('export-mcps').onclick=exportMCPs;
el('import-form').addEventListener('submit',importMCPs);
el('mcp-search').addEventListener('input',renderMCPs);
el('mcp-transport').addEventListener('change',updateTransportFields);
el('mcp-form').addEventListener('submit',saveMCP);
el('tool-mcp').addEventListener('change',event=>loadTools(event.target.value));
el('tool-call').onclick=callTool;
el('add-credential').onclick=()=>openCredentialDialog();
el('save-access-settings').onclick=saveAccessSettings;
el('credential-form').addEventListener('submit',saveCredential);
el('close-secret').onclick=()=>el('secret-dialog').close();
el('copy-secret').onclick=async()=>{try{await navigator.clipboard.writeText(el('created-secret').textContent);toast('密钥已复制')}catch{toast('复制失败，请手动复制')}};
el('refresh-audit').onclick=()=>loadAudit().then(()=>toast('日志已刷新')).catch(error=>toast(error.message));
el('clear-audit').onclick=clearAudit;
el('audit-search').addEventListener('input',renderAudit);
bootstrap();
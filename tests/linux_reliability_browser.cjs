const fs=require('fs'),path=require('path'),assert=require('assert');
const {execFileSync}=require('child_process'),{chromium}=require('playwright');
const root=path.resolve(__dirname,'..'),out=path.join(root,'docs/validation/reliability-rc3');
const cfg=path.join(root,'reference-sandbox/linux-validation/vm/ssh_config');
const users=['icwqb_a','icwqb_b'],origins=['http://127.0.0.1:18865','http://127.0.0.1:18866'];
const tokens=users.map(u=>JSON.parse(execFileSync('ssh',['-F',cfg,'-l','root','icw-vm','cat /home/'+u+'/.local/share/ic-workbench/production/.instance.json'],{encoding:'utf8',stdio:['pipe','pipe','pipe']})).token);
async function api(i,route,data){let r=await fetch(origins[i]+'/api/'+route,{method:data===undefined?'GET':'POST',headers:{Authorization:'Bearer '+tokens[i],'Content-Type':'application/json'},body:data===undefined?undefined:JSON.stringify(data)});assert(r.ok,await r.clone().text());return r.json()}
const act=(i,action,input={})=>api(i,'v2/action',{action,input});
(async()=>{
 const browser=await chromium.launch({headless:true,executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'});
 const context=await browser.newContext({viewport:{width:1360,height:900},acceptDownloads:true});
 const pages=[await context.newPage(),await context.newPage()],errors=[],checks=[];
 pages.forEach(p=>{p.setDefaultTimeout(90000);p.on('pageerror',e=>errors.push(e.message))});
 const click=(p,a)=>p.locator('[data-act="'+a+'"]').first().click();
 const nav=async(p,a)=>{await click(p,'nav-'+a);await p.waitForTimeout(120)};
 const saved=async p=>{await click(p,'save-workspace');await p.waitForFunction(()=>/Saved locally|已保存在本机/.test(document.querySelector('#workspace-status')?.textContent||''))};
 async function check(name,fn){let t=Date.now();await fn();checks.push({name,status:'PASS',seconds:(Date.now()-t)/1000});fs.writeFileSync(path.join(out,'browser.json'),JSON.stringify({mode:'CentOS production, two real UIDs, host Chrome via independent SSH origins; real Qwen2.5-Coder 1.5B',checks,errors},null,2));console.log('PASS',name)}
 try {
 const [a,b]=pages;
 await check('Expired token returns login form and accepts current terminal token',async()=>{
  await a.goto(origins[0]+'/#token=expired-test-token');await a.locator('#login-token').waitFor();await a.fill('#login-token',tokens[0]);await click(a,'login');await a.locator('#chat-input').waitFor();assert.equal(await a.evaluate(()=>sessionStorage.getItem('icw-run')),null);
 });
 await check('Same browser, two Linux users: projects, file notes, drafts and settings stay separate',async()=>{
  await b.goto(origins[1]+'/#token='+tokens[1]);await b.locator('#chat-input').waitFor();assert((await b.inputValue('#chat-input')).includes(users[1]));
  for(let i=0;i<2;i++){await nav(pages[i],'projects');let text=await pages[i].textContent('#content');assert(text.includes(users[i]));assert(!text.includes(users[1-i]));await pages[i].locator('[data-act="open-file"][data-path="design.txt"]').click();assert((await pages[i].textContent('#modal')).includes(users[i]+' private circuit'));await click(pages[i],'close-modal')}
  await nav(a,'calculator');await a.fill('#calc-expression','113*17');await click(a,'calculate');await a.waitForFunction(()=>document.querySelector('#calc-result')?.textContent.includes('1921'));await saved(a);
  assert.notEqual((await api(1,'workspace')).data.calculator?.expression,'113*17');
 });
 await check('Real model reconnect, chat and per-user model configuration',async()=>{
  let p=(await act(0,'preferences')).preferences;p.modelUrl='http://127.0.0.1:18881/v1/chat/completions';p.model='icw-coder';p.modelTools=true;p.configured=true;p.workspace='/home/'+users[0];p.shell='/bin/bash';p.setup='';p.compilers={python:{executable:'/usr/bin/python',args:['-m','py_compile','{file}']}};await act(0,'save-preferences',p);
  let connected=false;for(let j=0;j<150;j++){let v=(await act(0,'preferences')).validation;if(v.model==='connected'){connected=true;break}if(v.model==='failed')throw Error(JSON.stringify(v));await new Promise(r=>setTimeout(r,300))}assert(connected);
  assert.notEqual((await act(1,'preferences')).preferences.model,'icw-coder');
  await a.reload();await a.locator('#chat-input').waitFor();await click(a,'new-chat');await a.selectOption('#chat-scope','none');await a.fill('#chat-input','What is 7 + 8? Reply with the number only.');await click(a,'send-chat');await a.waitForFunction(()=>document.querySelector('#agent-progress .status')?.textContent==='completed');assert((await a.textContent('#messages')).includes('15'));
 });
 await check('Inline AI analysis and imported code improvement remain on their own pages',async()=>{
  await nav(a,'analysis');await a.locator('#analysis-file').setInputFiles({name:'reliability.log',mimeType:'text/plain',buffer:Buffer.from('ERROR: R1 resistance is negative (-100 ohm).\nSimulation stopped.\n')});await a.waitForFunction(()=>document.querySelector('#analysis-question')&&document.querySelector('#content')?.textContent.includes('reliability.log'));await a.fill('#analysis-question','Which component has a negative resistance? Quote the value.');await click(a,'analyze-ai');await a.waitForFunction(()=>document.querySelector('#analysis-ai-result')?.textContent.includes('R1'));assert.equal(await a.locator('#chat-input').count(),0);await saved(a);
  await nav(a,'code');const code='def add(a, b):\n    return a - b\n';await a.locator('#code-import').setInputFiles({name:'sum.py',mimeType:'text/plain',buffer:Buffer.from(code)});await a.waitForFunction(code=>document.querySelector('#hdl-code')?.value===code,code);await a.fill('#hdl-requirements','Change add to return a + b. Keep Python 2.6 syntax. Return JSON code and summary.');await click(a,'improve-code');await a.locator('#code-proposal').waitFor();assert.equal(await a.inputValue('#hdl-code'),code);assert((await a.inputValue('#code-proposal')).includes('a + b'));await click(a,'apply-code');await click(a,'compile-hdl');await click(a,'confirm-compile');await a.waitForFunction(()=>document.querySelector('#code-diagnostics .status')?.textContent==='succeeded');await saved(a);await a.screenshot({path:path.join(out,'code-review.png'),fullPage:true});
 });
 await check('All ten pages, bilingual UI, narrow layout and isolated persisted reload',async()=>{
  await a.reload();await a.locator('#chat-input').waitFor();await nav(a,'code');assert((await a.inputValue('#hdl-code')).includes('a + b'));
  await click(a,'language');await a.setViewportSize({width:760,height:900});
  for(const name of ['chat','projects','software','plugins','monitor','analysis','code','calculator','eda','settings']){await nav(a,name);await a.waitForTimeout(150);assert(await a.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'overflow '+name)}
  await a.screenshot({path:path.join(out,'narrow-settings.png'),fullPage:true});await b.reload();await b.locator('#chat-input').waitFor();assert.equal(await b.evaluate(()=>document.documentElement.lang),'zh-CN');assert((await b.inputValue('#chat-input')).includes(users[1]));assert.deepEqual(errors,[]);
 });
 } catch(e){await pages[0].screenshot({path:path.join(out,'browser-failure.png'),fullPage:true});throw e} finally {await browser.close()}
})().catch(e=>{console.error(e);process.exit(1)});

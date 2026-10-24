const fs=require('fs'),path=require('path'),assert=require('assert');
const {execFileSync}=require('child_process'),{chromium}=require('playwright');
const root=path.resolve(__dirname,'..'),out=path.join(root,'docs/validation/linux-agent-rc1'),cfg=path.join(root,'reference-sandbox/linux-validation/vm/ssh_config');
const users=['icwqb_a','icwqb_b'],origins=['http://127.0.0.1:18865','http://127.0.0.1:18866'];
const tokens=users.map(u=>JSON.parse(execFileSync('ssh',['-F',cfg,'-l','root','icw-vm','cat /home/'+u+'/linux-agent-data/.instance.json'],{encoding:'utf8',stdio:['pipe','pipe','pipe']})).token);
async function api(i,route,data){const r=await fetch(origins[i]+'/api/'+route,{method:data===undefined?'GET':'POST',headers:{Authorization:'Bearer '+tokens[i],'Content-Type':'application/json'},body:data===undefined?undefined:JSON.stringify(data)});assert(r.ok,await r.clone().text());return r.json()}
const act=(i,action,input={})=>api(i,'v2/action',{action,input});
(async()=>{
 for(let i=0;i<2;i++){let p=(await act(i,'preferences')).preferences;p.modelUrl='http://127.0.0.1:18882/v1/chat/completions';p.model='linux-contract';p.modelTools=true;p.configured=true;p.workspace='/home/'+users[i]+'/linux-agent-project';p.shell='/bin/bash';p.setup='';await act(i,'save-preferences',p)}
 const browser=await chromium.launch({headless:true,executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'}),context=await browser.newContext({viewport:{width:1360,height:960}}),pages=[await context.newPage(),await context.newPage()],checks=[],errors=[];
 pages.forEach(p=>{p.setDefaultTimeout(20000);p.on('pageerror',e=>errors.push(e.message))});
 const click=(p,a)=>p.locator('[data-act="'+a+'"]').first().click();
 async function task(i,text,mode){let p=pages[i];await click(p,'new-chat');await p.selectOption('#chat-scope','folder');await p.fill('#chat-folder','/home/'+users[i]+'/linux-agent-project');await p.selectOption('#linux-mode',mode);await p.fill('#chat-input',text);await click(p,'send-chat')}
 async function finished(p,status='completed'){await p.waitForFunction(status=>document.querySelector('#agent-progress .status')?.textContent===status,status);await p.waitForFunction(()=>!sessionStorage.getItem('icw-run'))}
 async function check(name,fn){const start=Date.now();await fn();checks.push({name,status:'PASS',seconds:(Date.now()-start)/1000});fs.writeFileSync(path.join(out,'browser.json'),JSON.stringify({mode:'CentOS 6.9 production, two real UIDs, deterministic model contract',checks,errors},null,2));console.log('PASS',name)}
 try{
 for(let i=0;i<2;i++){await pages[i].goto(origins[i]+'/#token='+tokens[i]);await pages[i].locator('#chat-input').waitFor()}
 await check('Confirmation holds command; live receipt includes real UID and exit code; authorization resets',async()=>{
  let p=pages[0];await task(0,'WRITE confirm','confirm');await p.locator('[data-act="approve-agent"]').waitFor();assert.equal(await p.locator('#linux-output').count(),0);await click(p,'approve-agent');await finished(p);assert((await p.textContent('#linux-output')).includes('uid=503'));assert((await p.textContent('#linux-output')).includes('icwqb_a'));assert.equal(await p.inputValue('#linux-mode'),'off');await p.screenshot({path:path.join(out,'agent-terminal.png'),fullPage:true});
 });
 await check('Autonomous execution under second UID; private file writes and cross-user denial',async()=>{
  let p=pages[1];await task(1,'WRITE auto','auto');await finished(p);assert((await p.textContent('#linux-output')).includes('uid=504'));assert((await p.textContent('#linux-output')).includes('icwqb_b'));await task(1,'OTHER_A','auto');await finished(p);assert((await p.textContent('#linux-output')).includes('Permission denied'));assert((await p.textContent('.linux-terminal')).includes('1'));await task(0,'OTHER_B','auto');await finished(pages[0]);assert((await pages[0].textContent('#linux-output')).includes('Permission denied'));
  const denied=await fetch(origins[1]+'/api/state',{headers:{Authorization:'Bearer '+tokens[0]}});assert.equal(denied.status,401);
 });
 await check('Next turn defaults off and refuses a model-requested Linux action',async()=>{let p=pages[0];await task(0,'WRITE without authorization','off');await finished(p);assert.equal(await p.locator('#linux-output').count(),0);assert((await p.textContent('#messages')).includes('Linux tools disabled'))});
 await check('Nonzero exit is retained and displayed; reload restores terminal receipt',async()=>{let p=pages[0];await task(0,'FAIL','auto');await finished(p);assert((await p.textContent('#linux-output')).includes('ACTUAL_ERROR'));assert((await p.textContent('.linux-terminal')).includes('7'));await p.reload();await p.locator('#linux-output').waitFor();assert((await p.textContent('#linux-output')).includes('ACTUAL_ERROR'))});
 await check('Running stdout is visible; stop cancels actual shell and shows cancelled status',async()=>{let p=pages[0];await task(0,'SLOW','auto');await p.waitForFunction(()=>document.querySelector('#linux-output')?.textContent.includes('LIVE_PROGRESS'));assert.equal(await p.locator('#agent-progress > .progress-panel > .row .status').textContent(),'running');await click(p,'stop-agent');await finished(p,'cancelled');assert(!(await p.textContent('#linux-output')).includes('SHOULD_NOT_RUN'))});
 await check('Bilingual and narrow layout remain usable',async()=>{let p=pages[0];await click(p,'language');await p.setViewportSize({width:760,height:980});await p.locator('#linux-mode').waitFor();assert((await p.textContent('.linux-controls')).includes('Autonomous'));assert(await p.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));await p.screenshot({path:path.join(out,'agent-narrow-en.png'),fullPage:true});assert.deepEqual(errors,[])});
 }catch(e){await pages[0].screenshot({path:path.join(out,'browser-failure.png'),fullPage:true});throw e}finally{await browser.close()}
})().catch(e=>{console.error(e);process.exit(1)});

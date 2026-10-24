const fs=require('fs'),path=require('path'),assert=require('assert'),cp=require('child_process'),{chromium}=require('playwright');
const root=path.resolve(__dirname,'..'),out=path.join(root,'docs/validation/terminal-gui-rc2');
(async()=>{
 const instance=JSON.parse(cp.execFileSync('ssh',['-F',path.join(root,'reference-sandbox/linux-validation/vm/ssh_config'),'-l','root','icw-vm','cat /home/icwgui/.local/share/ic-workbench/production/.instance.json'],{encoding:'utf8',stdio:['ignore','pipe','pipe']}));
 const demo=fs.readFileSync(path.join(root,'runtime-data/web-demo-preview.log'),'utf8').match(/WEB_DEMO_URL=(\S+)/)[1];
 const browser=await chromium.launch({headless:true,executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'}),results=[];
 try {
  for (const [name,url] of [['CentOS through SSH','http://127.0.0.1:18865/#token='+instance.token],['live web Demo',demo]]) {
   const page=await browser.newPage({viewport:{width:1360,height:1000}}),errors=[];page.on('pageerror',e=>errors.push(e.message));
   await page.goto(url);await page.locator('#chat-input').waitFor();assert((await page.textContent('.side-footer')).includes('0.6.0-rc2'));
   for(const tab of ['chat','projects','software','plugins','monitor','analysis','code','calculator','eda','settings']) {
    await page.locator('[data-act="nav-'+tab+'"]').click();await page.waitForTimeout(150);assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+2));
   }
   await page.locator('[data-act="nav-chat"]').click();assert.equal(await page.inputValue('#linux-mode'),'off');assert.deepEqual(errors,[]);
   await page.screenshot({path:path.join(out,name.startsWith('CentOS')?'ssh-chrome.png':'demo-chrome.png'),fullPage:true});
   results.push({name,version:'0.6.0-rc2',tenPages:true,loginAndAgentDefault:true,pageErrors:errors});await page.close();
  }
 } finally{await browser.close();}
 fs.writeFileSync(path.join(out,'modern-browser.json'),JSON.stringify(results,null,2)+'\n');console.log('PASS modern Chrome: SSH CentOS and live Demo, ten pages each');
})().catch(e=>{console.error(e);process.exit(1)});

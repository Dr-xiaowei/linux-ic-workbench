const fs=require('fs'),path=require('path'),assert=require('assert'),{chromium}=require('playwright');
const root=path.resolve(__dirname,'..'),out=path.join(root,process.env.ICW_EVIDENCE_DIR||'docs/validation/desktop-rc1'),version=process.env.ICW_EXPECT_VERSION||'0.7.0-rc1';
(async()=>{const log=fs.readFileSync(path.join(root,'runtime-data/web-demo-preview.log'),'utf8');const url=log.match(/WEB_DEMO_URL=(\S+)/)[1];
 const browser=await chromium.launch({headless:true,executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'});let errors=[];
 try {const page=await browser.newPage({viewport:{width:1360,height:1000}});page.on('pageerror',e=>errors.push(e.message));await page.goto(url);await page.locator('#chat-input').waitFor();assert((await page.textContent('.side-footer')).includes(version));assert((await page.textContent('.topbar')).includes('原生客户端')||(await page.textContent('.topbar')).includes('NATIVE LINUX CLIENT'));
 for(const tab of ['projects','software','plugins','monitor','analysis','code','calculator','eda','settings','chat']){await page.locator('[data-act="nav-'+tab+'"]').first().click();await page.waitForTimeout(150);assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+2));}
 assert.equal(await page.inputValue('#linux-mode'),'off');await page.screenshot({path:path.join(out,'web-demo-final.png'),fullPage:true});assert.deepEqual(errors,[]);
 fs.writeFileSync(path.join(out,'web-demo.json'),JSON.stringify({version:version,liveDemoOpens:true,nativeDistinction:true,tenPages:true,defaultLinuxOff:true,errors},null,2)+'\n');console.log('PASS: live web Demo ten pages and native-client distinction');
 }finally{await browser.close();}
})().catch(e=>{console.error(e);process.exit(1)});

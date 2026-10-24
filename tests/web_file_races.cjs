// Real Chromium DOM with deliberately reordered API receipts; no user data/server.
const fs=require('fs'),path=require('path'),http=require('http'),assert=require('assert');
const {chromium}=require('playwright');
const root=path.resolve(__dirname,'..'),out=path.join(root,'docs/validation/desktop-rc2');
const source=fs.readFileSync(path.join(root,'internal/workbench/web/app.js'),'utf8');
const at=source.lastIndexOf('\nboot().catch(');assert(at>0);
const hook=`
var delayed=[];
act=function(action,input){return new Promise(function(resolve){delayed.push({action:action,input:input,resolve:resolve})})};
window.raceTest={reset:function(){page='projects';project='p';root='/p';path='';offset=0;searchRows=[];searchCursor='';lastSearchScope='';delayed=[];fileRequestSerial++;document.body.innerHTML='<input id="file-search"><input id="search-all" type="checkbox"><div id="file-results"></div><div id="toast"></div>';},browse:function(p){path=p;return loadFiles()},search:function(q,more){$('file-search').value=q;return dispatchAction(more?'search-more':'search-files',{dataset:{}})},reply:function(i,r){delayed[i].resolve(r)},request:function(i){return delayed[i].input},result:function(){return $('file-results').textContent},leave:function(){page='code'},reenter:function(){page='projects';$('file-results').outerHTML='<div id="file-results"></div>'}};
})();`;
(async()=>{const server=http.createServer((q,r)=>r.end('<!doctype html><div id="app"></div>'));await new Promise(r=>server.listen(0,'127.0.0.1',r));let browser;
try{browser=await chromium.launch({headless:true,executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'});const page=await browser.newPage();let errors=[];page.on('pageerror',e=>errors.push(e.message));await page.goto('http://127.0.0.1:'+server.address().port);await page.addScriptTag({content:source.slice(0,at)+hook});
const checks=await page.evaluate(async()=>{const t=window.raceTest,checks=[];function check(ok,name){if(!ok)throw Error(name);checks.push(name)}const entries=name=>({entries:[{name,path:name}],more:false}),hits=(name,more=false)=>({hits:[{path:name,snippet:name,project:'p',root:'/p'}],more,cursor:more?'old-cursor':'',scanned:1,skipped:0});
t.reset();let a=t.browse('A'),b=t.browse('B');t.reply(1,entries('NEW'));await b;t.reply(0,entries('OLD'));await a;check(t.result().includes('NEW')&&!t.result().includes('OLD'),'newer directory wins');
t.reset();a=t.browse('A');b=t.search('new');await Promise.resolve();t.reply(1,hits('SEARCH'));await b;t.reply(0,entries('OLD'));await a;check(t.result().includes('SEARCH'),'search supersedes directory');
t.reset();a=t.search('old');await Promise.resolve();b=t.browse('B');t.reply(1,entries('DIRECTORY'));await b;t.reply(0,hits('OLD'));await a;check(t.result().includes('DIRECTORY')&&!t.result().includes('OLD'),'directory supersedes search');
t.reset();a=t.search('a');await Promise.resolve();b=t.search('b');await Promise.resolve();t.reply(1,hits('NEW'));await b;t.reply(0,hits('OLD'));await a;check(t.result().includes('NEW')&&!t.result().includes('OLD'),'newer search wins');
t.reset();a=t.browse('old');t.leave();t.reenter();t.reply(0,entries('OLD'));await a;check(t.result()==='','old page cannot paint recreated DOM');
t.reset();a=t.browse('A.v');b=t.browse('B.v');t.reply(1,{content:'B_CONTENT',size:9});await b;t.reply(0,{content:'A_CONTENT',size:9});await a;check(document.getElementById('modal').textContent.includes('B_CONTENT')&&!document.getElementById('modal').textContent.includes('A_CONTENT'),'newer file preview wins');
t.reset();a=t.search('gain');await Promise.resolve();t.reply(0,hits('GAIN',true));await a;b=t.search('power',true);await Promise.resolve();check(t.request(1).cursor==='','edited query resets cursor');t.reply(1,hits('POWER'));await b;check(t.result().includes('POWER')&&!t.result().includes('GAIN'),'edited query clears prior hits');return checks;});assert.deepEqual(errors,[]);fs.mkdirSync(out,{recursive:true});fs.writeFileSync(path.join(out,'web-file-races.json'),JSON.stringify({browser:'Chromium',mockedReceipts:true,checks,errors},null,2));console.log('PASS',checks.length,'real browser delayed-response checks');
}finally{if(browser)await browser.close();await new Promise(r=>server.close(r));}})().catch(e=>{console.error(e);process.exit(1)});

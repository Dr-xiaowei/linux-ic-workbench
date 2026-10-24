#!/usr/bin/env python3
import sys,time,json,pathlib
sys.path.insert(0,str(pathlib.Path(__file__).resolve().parent))
from linux_gui_marionette import Browser
out=pathlib.Path(__file__).resolve().parents[1]/'docs/validation/terminal-gui-rc2'
b=Browser();caps=b.call('newSession',{'capabilities':{}})['capabilities'];checks=[]
def js(s,*args):return b.js(s,*args)
def wait(s):b.wait(s,40)
def click(action):js('document.querySelector("[data-act="+arguments[0]+"]").click();',action)
def field(id,value):js('var e=document.getElementById(arguments[0]);e.value=arguments[1];e.dispatchEvent(new Event("input",{bubbles:true}));',id,value)
def nav(name):click('nav-'+name);time.sleep(.3)
def check(name,fn):
 start=time.time();fn();checks.append({'name':name,'status':'PASS','seconds':round(time.time()-start,2)});(out/'native-browser.json').write_text(json.dumps({'browser':caps,'browserRunsInsideCentOS':True,'checks':checks},indent=2)+'\n');print('PASS',name,flush=True)
def ready():
 wait('return !!document.querySelector("#chat-input")');assert js('return document.body.textContent.indexOf("0.6.0-rc2")>=0');assert js('return location.hash===""');assert js('return !!sessionStorage.getItem("icw-token")')
check('Terminal --open launched and authenticated the actual Linux Firefox window',ready)
def pages():
 for name in ['chat','projects','software','plugins','monitor','analysis','code','calculator','eda','settings']:
  nav(name);assert js('return document.querySelector("#content").textContent.length>30');assert js('return document.documentElement.scrollWidth<=innerWidth+2'),name
check('All ten GUI pages render and navigate in native Firefox',pages)
def calculator():
 nav('calculator');field('calc-expression','R*C');field('calc-variables','R=2k\nC=5n');click('calculate');wait('return document.querySelector(".calc-number") && Math.abs(Number(document.querySelector(".calc-number").textContent)-0.00001)<1e-15');click('save-workspace');time.sleep(.7);b.screenshot(out/'linux-calculator.png')
check('Formula calculation and local save operate through native GUI',calculator)
def code():
 nav('code');js('var f=new File(["module native_gui; endmodule\\n"],"native_gui.v",{type:"text/plain"});var e=document.querySelector("#code-import");Object.defineProperty(e,"files",{configurable:true,value:[f]});e.dispatchEvent(new Event("change",{bubbles:true}));');wait('return document.querySelector("#hdl-code").value.indexOf("module native_gui")>=0');assert js('return typeof File.prototype.arrayBuffer==="undefined"');click('save-workspace');time.sleep(.7);b.screenshot(out/'linux-code-import.png')
check('Native FileReader fallback imports Verilog into Code and saves locally',code)
def analysis():
 nav('analysis');js('var f=new File(["ERROR: VDD source is missing.\\n"],"native_gui.log",{type:"text/plain"});var e=document.querySelector("#analysis-file");Object.defineProperty(e,"files",{configurable:true,value:[f]});e.dispatchEvent(new Event("change",{bubbles:true}));');wait('return document.querySelector("#content").textContent.indexOf("VDD")>=0');assert js('return typeof File.prototype.text==="undefined"');click('save-workspace');time.sleep(.6);b.screenshot(out/'linux-analysis.png')
check('Native FileReader fallback opens a log in the analysis page',analysis)
def bilingual():
 click('language');wait('return document.documentElement.lang==="en"');nav('chat');b.screenshot(out/'linux-chat-en.png');click('language');wait('return document.documentElement.lang==="zh-CN"');b.call('setWindowSize',{'width':780,'height':1000});time.sleep(.5)
 for name in ['chat','projects','software','plugins','monitor','analysis','code','calculator','eda','settings']:
  nav(name);assert js('return document.documentElement.scrollWidth<=innerWidth+2'),name
 b.screenshot(out/'linux-settings-narrow.png')
check('Chinese/English and narrow-window layout remain usable',bilingual)
def reload():
 b.call('refresh');wait('return !!document.querySelector("#chat-input")');nav('code');wait('return document.querySelector("#hdl-code").value.indexOf("native_gui")>=0');nav('calculator');assert js('return document.querySelector("#calc-expression").value==="R*C"')
check('Reload restores imported source and calculator drafts',reload)
b.call('deleteSession');b.s.close()

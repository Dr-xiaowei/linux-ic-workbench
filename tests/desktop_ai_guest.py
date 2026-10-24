# -*- coding: utf-8 -*-
"""Native Tk + real small model acceptance. No mocked model replies."""
from __future__ import unicode_literals
import os,sys,time,json,traceback
sys.path.insert(0,'/opt/icw-desktop-test/desktop');sys.path.insert(0,'/tmp')
from runtime import tk,Lifecycle,set_text
from client import Workbench
from native_capture import capture
home=os.path.expanduser('~');out=home+'/desktop-ai-evidence';folder=home+'/desktop-ai-project'
if not os.path.isdir(out):os.makedirs(out,0700)
lc=Lifecycle('/opt/icw-desktop-test',['--data',home+'/desktop-ai-native-data','--listen','auto']);assert lc.claim();api,meta=lc.connect()
p=api.action('preferences')['preferences'];p.update(modelUrl='http://127.0.0.1:18883/v1/chat/completions',model='icw-small',modelTools=True,workspace=folder,configured=True)
p['compilers']={'python':{'executable':'/usr/bin/python','args':['-m','py_compile','{file}']}};api.action('save-preferences',p)
root=tk.Tk();app=Workbench(root,api,api.request('state'),lc);errors=[];checks=[];frames=[0]
def err(e):errors.append(unicode(e));print('GUI ERROR',unicode(e))
app.error=err;app.workers.on_error=err;root.report_callback_exception=lambda k,v,t:err(v)
def pump(seconds=.2):
 end=time.time()+seconds
 while time.time()<end:root.update();frames[0]+=1;time.sleep(.02)
def wait(fn,timeout=170):
 end=time.time()+timeout
 while time.time()<end:
  pump(.1)
  if fn():return
 raise AssertionError('Native UI timed out')
def put(key,value):
 w=app.widgets[key]
 if isinstance(w,tk.Text):set_text(w,value)
 else:w.delete(0,'end');w.insert(0,value)
 app.changed()
def check(name,fn):
 start=time.time();before=frames[0]
 try:fn();r={'name':name,'status':'PASS'}
 except Exception as e:r={'name':name,'status':'FAIL','error':unicode(e),'traceback':traceback.format_exc()}
 r.update(seconds=round(time.time()-start,2),uiFrames=frames[0]-before);checks.append(r)
 with open(out+'/native-ai.json','w') as f:json.dump({'checks':checks,'errors':errors,'model':'Qwen3-1.7B-Q8_0','nativeToolkit':'Tk 8.5.7'},f,indent=2)
 print(r['status'],name);sys.stdout.flush()
def chat():
 app.new_chat();pump();put('chat_input','What current flows through a 1000 ohm resistor at 1 volt? Answer with 0.001 A and 1 mA, briefly. No tools needed.')
 app.buttons['send'].invoke();wait(lambda:bool(app.agent));wait(lambda:not app.agent);wait(lambda:'0.001' in app.value('messages'));assert 'completed' in app.value('progress');capture(out+'/native-ai-chat.png')
check('Real model question and live progress in native chat',chat)
def analysis():
 path=folder+'/native-analysis.log'
 with open(path,'w') as f:f.write('Synthetic fixture, not actual EDA. ERROR (SFE-23): M1 references undefined model nmos_missing. Simulation stopped before transient analysis.\n')
 app.goto('analysis');app.import_analysis(path);wait(lambda:bool(app.analysis));put('analysis_question','Explain the missing model name and whether transient results exist. Do not invent success.')
 app.buttons['analysis_ai'].invoke();wait(lambda:'nmos_missing' in app.value('analysis_answer'));assert app.page=='analysis';capture(out+'/native-ai-analysis.png')
check('Real AI file analysis stays on current native page',analysis)
def code():
 path=folder+'/safe_divide.py'
 original='def safe_divide(a, b):\n    return a / b\n'
 with open(path,'w') as f:f.write(original)
 app.goto('code');app.import_code(path);put('requirements','Python 2.6: if b == 0 return None, otherwise return float(a) / b. Preserve safe_divide(a,b). No imports.')
 app.buttons['code_improve'].invoke();wait(lambda:bool(app.draft().get('proposal')));assert app.value('source')==original
 assert 'None' in app.value('proposal');app.buttons['code_apply'].invoke();assert 'None' in app.value('source')
 app.buttons['code_check'].invoke();wait(lambda:bool(app.code_run));wait(lambda:not app.code_run);assert 'succeeded' in app.value('code_log');assert open(path).read()==original
 capture(out+'/native-ai-code.png')
check('Native code import -> real AI proposal -> apply -> actual CentOS compiler',code)
def linux():
 app.goto('chat');app.new_chat();pump();app.set_scope({'project':'','folder':folder});pump()
 put('chat_input','Inspect Linux, then use linux_command to execute exactly: printf NATIVE_APPROVAL_OK . Report actual output. Do not use platform tools.');app.widgets['linux_mode'].set('confirm')
 app.buttons['send'].invoke();wait(lambda:bool(app.agent));wait(lambda:app.agent_snapshot and app.agent_snapshot.get('status')=='awaiting_approval')
 assert not app.agent_snapshot.get('terminal');app.buttons['approve'].invoke();wait(lambda:not app.agent)
 assert app.agent_snapshot['terminal']['exitCode']==0 and 'NATIVE_APPROVAL_OK' in app.agent_snapshot['terminal']['output'];capture(out+'/native-ai-linux.png')
check('Native Linux Agent approval precedes actual command and displays receipt',linux)
def cancel():
 app.new_chat();pump();put('chat_input','Write a detailed tutorial with 200 numbered examples about analog IC design, with long explanations.')
 app.buttons['send'].invoke();wait(lambda:bool(app.agent));identity=app.agent;app.buttons['cancel_agent'].invoke();wait(lambda:not app.agent)
 assert api.request('v2/progress?id='+identity)['status']=='cancelled'
check('Native UI can cancel real model inference',cancel)
def mcp():
 api.action('save-mcp',{'id':'native-fixture','name':'Explicit protocol fixture (no EDA)','transport':'stdio','command':{'executable':'/usr/bin/python','args':['/tmp/desktop_mcp_fixture.py']},'directory':folder,'allowedTools':['echo'],'timeoutSeconds':20})
 app.state=api.request('state')['state'];app.eda_id='native-fixture';app.goto('eda');app.buttons['mcp_discover'].invoke();wait(lambda:len(app.mcp_table.rows)==1)
 put('eda_input','Use echo with value NATIVE_MCP_OK. Return the actual tool receipt. This is a synthetic protocol test; no EDA software runs.')
 app.buttons['eda_send'].invoke();wait(lambda:bool(app.eda_run));identity=app.eda_run;wait(lambda:not app.eda_run)
 r=api.request('v2/progress?id='+identity)
 with open(out+'/native-mcp-real-model.json','w') as f:json.dump(r,f,indent=2)
 assert r['status']=='completed' and 'NATIVE_MCP_OK' in r['content'],r
 assert any(e['stage']=='progress' for e in r['events']),r
 assert any('FIXTURE_RECEIPT NATIVE_MCP_OK' in e['text'] for e in r['events']),r
 capture(out+'/native-ai-mcp.png')
check('Real AI controls allowed MCP fixture and shows actual progress (no EDA)',mcp)
def narrow():
 root.geometry('900x660');pump(.7)
 for page in ['code','settings','eda','chat']:
  app.goto(page);pump(.4);app.canvas.yview_moveto(1);app.canvas.xview_moveto(1);pump(.2)
  box=app.canvas.bbox('all');assert box and box[3]>=app.canvas.winfo_height()
 capture(out+'/native-narrow.png');assert not errors,errors
check('Narrow native window retains scroll access to every page control',narrow)
app.save_workspace();wait(lambda:not app.saving);app.finish_close();lc.close()
print('SUMMARY',sum(r['status']=='PASS' for r in checks),'/',len(checks));sys.exit(0 if all(r['status']=='PASS' for r in checks) and not errors else 1)

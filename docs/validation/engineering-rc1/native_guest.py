# -*- coding: utf-8 -*-
"""Actual Tk widget acceptance in CentOS, Python 2.6. Isolated private data."""
from __future__ import unicode_literals
import os,sys,time,json,subprocess,hashlib,traceback
sys.path.insert(0,'/home/icw/engineering-rc1/delivery-installed/current-production/desktop')
sys.path.insert(0,'/home/icw/engineering-rc1/tests')
from runtime import tk,Lifecycle,set_text,text_value
from client import Workbench,PAGES
from native_capture import capture
home=os.path.expanduser('~');folder=home+'/engineering-delivery-fixture';out=home+'/engineering-delivery-evidence'
for p in [folder,out]:
 if not os.path.isdir(p):os.makedirs(p,0o700)
with open(folder+'/design.txt','w') as f:f.write('NATIVE_SEARCH_MARKER gain=60\n')
with open(folder+'/native.v','w') as f:f.write('module native_gate(input a,b,output y); assign y=a&b; endmodule\n')
with open(folder+'/wave.csv','w') as f:f.write('time,vout\n0,0\n1,1\n2,0\n3,1\n')
lc=Lifecycle('/home/icw/engineering-rc1/delivery-installed/current-production',['--data',home+'/engineering-delivery-data','--listen','auto'])
assert lc.claim();api,meta=lc.connect();root=tk.Tk();app=Workbench(root,api,meta,lc);root.geometry('1360x920');root.lift();errors=[];checks=[]
def error(e):errors.append(str(e));print('GUI_ERROR',str(e))
app.error=error;app.workers.on_error=error
root.report_callback_exception=lambda kind,value,tb: error(value)
def pump(seconds=.4):
 end=time.time()+seconds
 while time.time()<end:root.update();time.sleep(.02)
def wait(fn,timeout=15):
 end=time.time()+timeout
 while time.time()<end:
  pump(.1)
  if fn():return
 raise AssertionError('UI condition timed out')
def put(key,value):
 w=app.widgets[key]
 if isinstance(w,tk.Text):set_text(w,value)
 else:w.delete(0,'end');w.insert(0,value)
 app.changed()
def form(values):
 for key,value in values.items():
  w=app.dialog_fields[key]
  if isinstance(w,tk.Text):set_text(w,value)
  elif isinstance(w,tk.BooleanVar):w.set(value)
  else:w.delete(0,'end');w.insert(0,value)
 app.dialog_submit();pump(.5)
def check(name,fn):
 start=time.time()
 try:fn();record={'name':name,'status':'PASS'}
 except Exception as e:record={'name':name,'status':'FAIL','error':str(e),'traceback':traceback.format_exc()}
 record['seconds']=round(time.time()-start,2);checks.append(record)
 with open(out+'/native-ui.json','w') as f:json.dump({'uid':os.getuid(),'toolkit':'Tk '+str(root.tk.call('info','patchlevel')),'nativeWidgets':True,'checks':checks,'errors':errors},f,indent=2)
 print(record['status'],name);sys.stdout.flush()
def navigation():
 for key,zh,en in PAGES:
  app.goto(key);pump(.7);assert root.winfo_exists();assert len(app.content.winfo_children())>=3
  capture(out+'/'+key+'.png')
 assert not errors,errors
check('All ten native workspaces render without Tk callback errors',navigation)
def projects():
 app.goto('projects');app.buttons['project_add'].invoke();pump();form({'name':'Native project','root':folder,'paths':'','tags':'analog,qa','notes':'Native note','description':'Client fixture'})
 wait(lambda:len(app.project_table.rows)>0);app.project_table.select(len(app.project_table.rows)-1);pump(.7);wait(lambda:len(app.file_table.rows)>=3)
 row=next(i for i,x in enumerate(app.file_table.rows) if x['name']=='design.txt');app.file_table.select(row);app.buttons['file_note'].invoke();pump();form({'notes':'Reviewed in native GUI','description':'Evidence fixture'})
 wait(lambda:any(x.get('notes')=='Reviewed in native GUI' for x in app.file_table.rows));app.buttons['file_open'].invoke();wait(lambda:'NATIVE_SEARCH_MARKER' in app.value('preview'))
 put('query','NATIVE_SEARCH_MARKER');app.buttons['search'].invoke();wait(lambda:any(x.get('path')=='design.txt' for x in app.file_table.rows));capture(out+'/project-flow.png')
check('Native project form, directory browser, annotation and keyword search',projects)
def code():
 app.goto('code');app.import_code(folder+'/native.v');pump(.5)
 assert 'module native_gate' in app.value('source');assert app.code_language=='verilog'
 d=app.draft();original=app.value('source');d.update(base=original,proposal=original+'// reviewed\n')
 app.buttons['code_apply'].invoke();assert '// reviewed' in app.value('source');app.buttons['code_undo'].invoke();assert app.value('source')==original
 d.update(base=original,proposal=original+'// proposal\n');put('source',original+'// independent edit\n')
 try:app.apply_code();raise AssertionError('stale proposal accepted')
 except ValueError:pass
 app.save_workspace();wait(lambda:not app.saving);capture(out+'/code-import.png')
check('Code native import, apply/undo and stale-proposal conflict protection',code)
def analysis():
 app.goto('analysis');app.import_analysis(folder+'/wave.csv');wait(lambda:app.analysis and app.analysis.get('signals'));pump(.7)
 assert len(app.wave_canvas.find_all())>2
 app.buttons['zoom_in'].invoke();assert app.wave_zoom>1;app.buttons['pan_right'].invoke();assert app.wave_pan>0
 capture(out+'/wave-native.png')
check('Native CSV waveform canvas supports zoom and pan',analysis)
def calculator():
 app.goto('calculator');put('expression','R*C');put('variables','R=2k\nC=5n');app.buttons['calculate'].invoke()
 wait(lambda:'1e-05' in app.value('calc_result') or '0.00001' in app.value('calc_result'));app.save_workspace();wait(lambda:not app.saving)
 assert abs(api.request('workspace')['data']['calculator']['result']['value']-1e-5)<1e-12
check('Calculator variables and disk-persisted result',calculator)
def software():
 app.goto('software');app.buttons['software_add'].invoke();pump();form({'name':'Native command fixture','command':'printf NATIVE_SOFTWARE_OK','shell':'/bin/sh'})
 wait(lambda:len(app.software_table.rows)>0);app.software_table.select(len(app.software_table.rows)-1);pump()
 app.buttons['version_add'].invoke();pump();form({'name':'v2','setup':'','toolPath':''});pump();app.software_table.select(len(app.software_table.rows)-1);pump()
 s=app.software_table.selected();assert s['active']=='v2'
 # Confirm only this known harmless fixture command.
 import client
 saved=client.messagebox.askyesno;client.messagebox.askyesno=lambda *a,**k:True
 try:app.buttons['software_run'].invoke();wait(lambda:'NATIVE_SOFTWARE_OK' in app.value('operation_log'))
 finally:client.messagebox.askyesno=saved
check('Software registration, version environment selection and actual command',software)
def plugin():
 app.goto('plugins');app.buttons['plugin_add'].invoke();pump();form({'name':'Native plugin fixture','version':'1','description':'No EDA','executable':'/bin/echo','args':'["NATIVE_PLUGIN_OK"]','timeoutSeconds':'10','enabled':True})
 wait(lambda:len(app.plugin_table.rows)>0);app.plugin_table.select(len(app.plugin_table.rows)-1);pump()
 import client
 saved=client.messagebox.askyesno;client.messagebox.askyesno=lambda *a,**k:True
 try:app.buttons['plugin_run'].invoke();wait(lambda:'NATIVE_PLUGIN_OK' in app.value('operation_log'))
 finally:client.messagebox.askyesno=saved
check('Native plugin registration and real bounded execution',plugin)
def monitor():
 app.goto('monitor');wait(lambda:len(app.process_table.rows)>0)
 assert 'N/A' in app.resource_labels['gpu'].cget('text');assert '%' in app.resource_labels['memory'].cget('text')
 assert api.action('monitor')['source']=='linux';capture(out+'/monitor-native.png')
check('Actual Linux resources and truthful unavailable GPU',monitor)
def settings():
 app.goto('settings');pump(.6);put('setting_threshold','85');app.buttons['settings_save'].invoke()
 wait(lambda:api.action('preferences')['preferences']['threshold']==85)
 wait(lambda:api.action('preferences')['validation']['environment']=='ready')
 app.switch_language();wait(lambda:app.lang=='en');assert app.nav['projects'].cget('text')=='Projects & files'
 app.goto('code');capture(out+'/native-en.png');app.switch_language();wait(lambda:app.lang=='zh')
check('Native settings save/automatic environment reload and bilingual UI',settings)
def persistence():
 app.save_workspace();wait(lambda:not app.saving);saved=api.request('workspace')['data'];assert 'native_gate' in saved['codeDrafts']['verilog']['code']
 assert saved['analyses'] and saved['calculator']['expression']=='R*C'
 req=__import__('urllib2').Request(api.base+'/')
 try:__import__('urllib2').urlopen(req);raise AssertionError('web UI unexpectedly served')
 except __import__('urllib2').HTTPError as e:assert e.code==404
 cmd=['/home/icw/engineering-rc1/delivery-installed/current-production/start.sh','--data',home+'/engineering-delivery-data'];p=subprocess.Popen(cmd,stdout=subprocess.PIPE,stderr=subprocess.PIPE);stdout,stderr=p.communicate();assert p.returncode==0,stderr
 assert api.instance['pid']==json.load(open(home+'/engineering-delivery-data/.instance.json'))['pid']
 assert not errors,errors
check('Persistent drafts, API-only backend and repeat command focuses same client',persistence)
app.capture();app.save_workspace();wait(lambda:not app.saving)
def engineering_templates():
 app.goto('chat');app.buttons['task_templates'].invoke();wait(lambda:hasattr(app,'template_fields'))
 field=app.template_fields['input'];field.delete(0,'end');field.insert(0,'results/spectre.log');app.template_preview();pump(1)
 app.template_use();pump(.4);assert 'results/spectre.log' in app.value('chat_input');assert app.engineering_data()['nextBudget']['maxRounds']==4
 capture(out+'/task-template.png')
check('Native parameter form, backend preview and bounded AI draft',engineering_templates)
def engineering_metrics():
 app.goto('analysis');app.buttons['metric_compare'].invoke();pump(.2);app.metric_sample();app.metric_run();wait(lambda:len(app.metric_table.rows)==2)
 assert app.metric_table.rows[0]['status']=='pass',app.metric_table.rows
 original=text_value(app.metric_fields['candidate']);set_text(app.metric_fields['candidate'],original.replace(',tt,',',ff,'));app.metric_run();wait(lambda:len(app.metric_table.rows)==4)
 assert all(x['status']=='not-comparable' for x in app.metric_table.rows)
 capture(out+'/metric-compare.png');app.metric_window.destroy()
check('Native CSV conversion, specification and mismatched PVT rejection',engineering_metrics)
def engineering_multifile():
 with open(folder+'/gain.py','w') as f:f.write('GAIN = 2\ndef amplify(value):\n    return value * GAIN\n')
 with open(folder+'/test_gain.py','w') as f:f.write('from gain import amplify\nassert amplify(2) == 4\n')
 p=api.action('preferences')['preferences'];p.update(configured=True,modelUrl='http://10.0.2.2:18883/v1/chat/completions',model='icw-small',modelTools=True)
 api.action('save-preferences',p)
 app.goto('code');app.buttons['change_sets'].invoke();wait(lambda:hasattr(app,'changes_fields'))
 f=app.changes_fields;f['root'].delete(0,'end');f['root'].insert(0,folder);set_text(f['paths'],'gain.py\ntest_gain.py');set_text(f['requirements'],'Change GAIN from 2 to 3 in gain.py and update test_gain.py so assert amplify(2) == 6. Keep every other statement and function unchanged. Return both complete files. Do not add imports or functions.')
 app.changes_generate();wait(lambda:hasattr(app,'review_set'),150)
 c=app.review_set;assert len(c['files'])==2;assert not c['demo'];assert open(folder+'/gain.py').read().startswith('GAIN = 2')
 import ast
 expected={'gain.py':'GAIN = 3\ndef amplify(value):\n    return value * GAIN\n','test_gain.py':'from gain import amplify\nassert amplify(2) == 6\n'}
 for f in c['files']:assert ast.dump(ast.parse(f['after']))==ast.dump(ast.parse(expected[f['path']])),f
 with open(out+'/real-model-change-set.json','w') as f:json.dump(c,f,ensure_ascii=True,indent=2)
 assert app.review_table.selected() and app.review_table.selected()['path']==c['files'][0]['path'];app.review_table.select(0);app.review_apply();wait(lambda:app.review_set['files'][0]['state']=='applied');assert open(folder+'/'+c['files'][0]['path']).read()==c['files'][0]['after']
 app.review_table.select(1);app.review_apply();wait(lambda:app.review_set['files'][1]['state']=='applied')
 assert subprocess.call(['/usr/bin/python',folder+'/test_gain.py'],cwd=folder)==0
 assert app.review_table.selected()['path']==c['files'][1]['path'];capture(out+'/multi-file-review.png')
 for idx in (0,1):
  app.review_table.select(idx);app.review_undo();wait(lambda:app.review_set['files'][idx]['state']=='undone')
 assert open(folder+'/gain.py').read().startswith('GAIN = 2');assert '== 4' in open(folder+'/test_gain.py').read()
 app.change_review_window.destroy()
check('Real Qwen3 coordinated two-file proposal, native review, apply, execution and undo',engineering_multifile)
def engineering_template_model():
 with open(folder+'/model-check.log','w') as f:f.write('ERROR (SFE-23): Instance M1 references undefined model nmos_missing.\nSimulation terminated before transient analysis.\n')
 task=api.action('preview-task-template',{'id':'log-review','values':{'input':'model-check.log','focus':'Quote the exact undefined model identifier; state whether transient data exists.','stop':'Report observed facts and three proposed next checks without writing files.'},'maxRounds':4,'maxSeconds':120})
 s=api.request('sessions',{'title':'Template real model acceptance','folder':folder})
 r=api.request('v2/agent',{'session':s['id'],'text':task['text'],'maxRounds':task['maxRounds'],'maxSeconds':task['maxSeconds'],'readOnly':True,'linuxMode':'off','templateID':'log-review','templateValues':{'input':'model-check.log','focus':'Quote nmos_missing and determine whether transient results exist','stop':'Three proposed next checks'}})
 result=[None]
 def finished():
  result[0]=api.request('v2/progress?id='+r['id']);return result[0]['status'] not in ('running','awaiting_approval')
 wait(finished,140);r=result[0]
 with open(out+'/real-model-template.json','w') as f:json.dump(r,f,ensure_ascii=True,indent=2)
 assert r['status']=='completed',r
 assert 'nmos_missing' in r['content'],r
 assert any(e['stage']=='template-input' and 'nmos_missing' in e['text'] for e in r['events']),r
check('Real Qwen3 parameterized read-only log task cites actual file evidence',engineering_template_model)

def engineering_chat():
 from engineering import readable_chat
 raw='已完成\n{"entries":[{"name":"amp.scs"}]}\n普通回答'
 visible=readable_chat(raw);assert '"entries"' not in visible;assert '普通回答' in visible
 app.goto('chat');set_text(app.widgets['messages'],visible);app.chat_raw=raw;app.buttons['chat_details'].invoke();pump(.3);capture(out+'/chat-readable.png')
 for w in root.winfo_children():
  if isinstance(w,tk.Toplevel):w.destroy()
check('Native readable answer keeps original structured receipts available',engineering_chat)

import resource
resources={'clientMaxRSSKiB':resource.getrusage(resource.RUSAGE_SELF).ru_maxrss,'stateBytes':os.path.getsize(home+'/engineering-delivery-data/state.json'),'workspaceBytes':os.path.getsize(home+'/engineering-delivery-data/workspace.json.gz')}
with open('/proc/%d/status'%api.instance['pid']) as f:resources['backendMemory']=[line.strip() for line in f if line.startswith(('VmRSS:','VmSize:','Threads:'))]
with open(out+'/resources.json','w') as f:json.dump(resources,f,indent=2)
backend_pid=api.instance['pid'];app.finish_close();lc.close()
assert not os.path.exists(home+'/engineering-delivery-data/.instance.json')
checks.append({'name':'Closing native client stops its owned backend','status':'PASS'})
with open(out+'/native-ui.json','w') as f:json.dump({'uid':os.getuid(),'version':'0.8.0-rc1','toolkit':'Tk 8.5.7','nativeWidgets':True,'checks':checks,'errors':errors,'backendStopped':True},f,indent=2)
print('FINISHED',len(checks));sys.exit(0 if all(r['status']=='PASS' for r in checks) and not errors else 1)

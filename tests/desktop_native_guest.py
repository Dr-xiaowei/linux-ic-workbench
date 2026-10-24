# -*- coding: utf-8 -*-
"""Actual Tk widget acceptance in CentOS, Python 2.6. Isolated private data."""
from __future__ import unicode_literals
import os,sys,time,json,subprocess,hashlib,traceback
sys.path.insert(0,'/opt/icw-desktop-test/desktop')
sys.path.insert(0,'/tmp')
from runtime import tk,Lifecycle,set_text,text_value
from client import Workbench,PAGES
from native_capture import capture
home=os.path.expanduser('~');folder=home+'/desktop-native-fixture';out=home+'/desktop-native-evidence'
for p in [folder,out]:
 if not os.path.isdir(p):os.makedirs(p,0o700)
with open(folder+'/design.txt','w') as f:f.write('NATIVE_SEARCH_MARKER gain=60\n')
with open(folder+'/native.v','w') as f:f.write('module native_gate(input a,b,output y); assign y=a&b; endmodule\n')
with open(folder+'/wave.csv','w') as f:f.write('time,vout\n0,0\n1,1\n2,0\n3,1\n')
lc=Lifecycle('/opt/icw-desktop-test',['--data',home+'/desktop-native-data','--listen','auto'])
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
 cmd=['/opt/icw-desktop-test/start.sh','--data',home+'/desktop-native-data'];p=subprocess.Popen(cmd,stdout=subprocess.PIPE,stderr=subprocess.PIPE);stdout,stderr=p.communicate();assert p.returncode==0,stderr
 assert api.instance['pid']==json.load(open(home+'/desktop-native-data/.instance.json'))['pid']
 assert not errors,errors
check('Persistent drafts, API-only backend and repeat command focuses same client',persistence)
app.capture();app.save_workspace();wait(lambda:not app.saving)
backend_pid=api.instance['pid'];app.finish_close();lc.close()
assert not os.path.exists(home+'/desktop-native-data/.instance.json')
checks.append({'name':'Closing native client stops its owned backend','status':'PASS'})
with open(out+'/native-ui.json','w') as f:json.dump({'uid':os.getuid(),'version':'0.7.0-rc1','toolkit':'Tk 8.5.7','nativeWidgets':True,'checks':checks,'errors':errors,'backendStopped':True},f,indent=2)
print('FINISHED',len(checks));sys.exit(0 if all(r['status']=='PASS' for r in checks) and not errors else 1)

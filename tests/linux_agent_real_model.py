#!/usr/bin/env python3
"""Real small-model / CentOS tool-loop smoke; no EDA or company data."""
import json,time,subprocess,urllib.request,pathlib,os
root=pathlib.Path(__file__).resolve().parents[1];out=root/'docs/validation/linux-agent-rc1'
instance=json.loads(subprocess.check_output(['ssh','-F',str(root/'reference-sandbox/linux-validation/vm/ssh_config'),'-l','root','icw-vm','cat /home/icwqb_a/linux-agent-data/.instance.json']))
def api(route,data=None):
 body=None if data is None else json.dumps(data).encode();req=urllib.request.Request('http://127.0.0.1:18865/api/'+route,data=body,headers={'Authorization':'Bearer '+instance['token'],'Content-Type':'application/json'});return json.load(urllib.request.urlopen(req,timeout=30))
def action(name,x={}):return api('v2/action',{'action':name,'input':x})
p=action('preferences')['preferences'];p.update(modelUrl=os.environ.get('ICW_TEST_MODEL_URL','http://127.0.0.1:18883/v1/chat/completions'),model='icw-coder',modelTools=True,configured=True);action('save-preferences',p)
for i in range(100):
 v=action('preferences')['validation']
 if v.get('model')=='connected':break
 time.sleep(.4)
session=api('sessions',{'title':'Real model Linux smoke','folder':'/home/icwqb_a/linux-agent-project'})
prompt='Use linux_action tools to complete this task, not a textual suggestion. First inspect the Linux system. Then create a NEW UTF-8 file real-model-object-note.txt in the working directory with exactly REAL_MODEL_OK followed by a newline, using write-file with expectedSHA256=new. Finally run the command cat real-model-object-note.txt; id -u and report the actual output and exit code. Do not modify other files, do not use platform_action, do not use sudo.'
start=time.time();run=api('v2/agent',{'session':session['id'],'text':prompt,'linuxMode':'auto'})
while time.time()-start<300:
 r=api('v2/progress?id='+run['id'])
 if r['status'] not in ('running','awaiting_approval'):break
 time.sleep(.5)
else:api('v2/cancel-agent',{'id':run['id']});raise RuntimeError('real model exceeded 300s')
report={'model':'Qwen3 0.6B Q4_0, llama.cpp b11146 on host; actual CentOS 6.9 commands as UID503','seconds':round(time.time()-start,2),'prompt':prompt,'run':r}
(out/'real-model.json').write_text(json.dumps(report,indent=2,ensure_ascii=False)+'\n')
assert r['status']=='completed',r
assert r.get('terminal') and r['terminal']['exitCode']==0 and 'REAL_MODEL_OK' in r['terminal']['output'] and '503' in r['terminal']['output'],r
actual=subprocess.check_output(['ssh','-F',str(root/'reference-sandbox/linux-validation/vm/ssh_config'),'-l','root','icw-vm','cat /home/icwqb_a/linux-agent-project/real-model-object-note.txt'])
assert actual==b'REAL_MODEL_OK\n',actual
print('PASS real model inspect -> write-file -> actual command/UID receipt',report['seconds'])

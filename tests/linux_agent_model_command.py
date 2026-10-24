#!/usr/bin/env python3
import json,time,subprocess,urllib.request,pathlib
root=pathlib.Path(__file__).resolve().parents[1];out=root/'docs/validation/linux-agent-rc1'
i=json.loads(subprocess.check_output(['ssh','-F',str(root/'reference-sandbox/linux-validation/vm/ssh_config'),'-l','root','icw-vm','cat /home/icwqb_a/linux-agent-data/.instance.json']))
def api(route,data=None):return json.load(urllib.request.urlopen(urllib.request.Request('http://127.0.0.1:18865/api/'+route,data=None if data is None else json.dumps(data).encode(),headers={'Authorization':'Bearer '+i['token'],'Content-Type':'application/json'}),timeout=30))
s=api('sessions',{'title':'Real model read-only command','folder':'/home/icwqb_a/linux-agent-project'})
prompt='Use linux_action once: action command, input object with command set to pwd; id -u. Omit all optional fields, including directory. Then summarize the actual tool receipt. Do not create or edit any files. /no_think'
start=time.time();task=api('v2/agent',{'session':s['id'],'text':prompt,'linuxMode':'auto'})
while time.time()-start<160:
 r=api('v2/progress?id='+task['id'])
 if r['status'] not in ['running','awaiting_approval']:break
 time.sleep(.5)
else:api('v2/cancel-agent',{'id':task['id']})
report={'model':'Qwen3 0.6B Q4_0','scope':'Single guided read-only command only; not a pass for autonomous multistep planning','prompt':prompt,'seconds':time.time()-start,'run':r}
(out/'real-model-command.json').write_text(json.dumps(report,indent=2,ensure_ascii=False)+'\n')
assert r['status']=='completed' and r.get('terminal') and r['terminal']['exitCode']==0 and '503' in r['terminal']['output'],r
print('PASS real small-model single read-only command:',report['seconds'])

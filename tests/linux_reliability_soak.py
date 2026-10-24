#!/usr/bin/env python
"""Three-minute bounded dual-user read/calculation load; Python 2.6 in QA VM."""
from __future__ import with_statement
import json, urllib2, os, time
users=['icwqb_a','icwqb_b']
instances={}
for user in users:
    with open('/home/'+user+'/.local/share/ic-workbench/production/.instance.json') as f:instances[user]=json.load(f)
samples=[];requests=0;start=time.time();last=-20
while time.time()-start<180:
    for user in users:
        v=instances[user]
        for route,body in [('state',None),('v2/action',json.dumps(dict(action='calculate',input=dict(expression='1/(2*pi*R*C)',variables='R=1k\nC=1n',angle='rad'))))]:
            req=urllib2.Request(v['url']+'/api/'+route,body,{'Authorization':'Bearer '+v['token'],'Content-Type':'application/json'})
            r=urllib2.urlopen(req,timeout=10);out=json.loads(r.read());r.close();requests+=1
            if route=='state':assert out['state']['projects'][0]['name']==user
            else:assert abs(out['value']-159154.94309189535)<.00001
    if time.time()-start-last>=15:
        row={'elapsed':round(time.time()-start,2),'users':{}}
        for user in users:
            pid=instances[user]['pid'];fields={}
            for line in open('/proc/%d/status'%pid):
                if line.startswith(('VmRSS:','Threads:')):k,val=line.split(':',1);fields[k]=int(val.split()[0])
            fields['fileDescriptors']=len(os.listdir('/proc/%d/fd'%pid));assert fields['VmRSS']<128*1024
            row['users'][user]=fields
        samples.append(row);last=time.time()-start
    time.sleep(.2)
result=dict(status='PASS',durationSeconds=round(time.time()-start,2),requests=requests,samples=samples,scope='Two real UIDs, production backend; repeated state reads and calculator requests, no model/EDA load')
with open('/var/tmp/icw-reliability-soak.json','w') as f:json.dump(result,f,indent=2)
print('PASS dual-user 180-second load, %d requests'%requests)

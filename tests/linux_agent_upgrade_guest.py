#!/usr/bin/env python
# Python 2.6 compatible; runs only inside the retained CentOS fixture.
import os,json,time,hashlib,subprocess,urllib2,sys
os.umask(0o077)
base='/home/icw/validation';data=base+'/data';config=base+'/config/config.json'
binary=sys.argv[1] if len(sys.argv)>1 else base+'/ic-workbench'
paths=[data+'/state.json',data+'/workspace.json.gz',config]
def hashes():return dict((p,hashlib.sha256(open(p,'rb').read()).hexdigest()) for p in paths)
before=hashes();log=open(base+'/linux-agent-upgrade.log','w')
process=subprocess.Popen([binary,'--config',config,'--data',data,'--listen','127.0.0.1:18763'],stdin=open('/dev/null'),stdout=log,stderr=subprocess.STDOUT)
try:
 for n in range(100):
  if process.poll() is not None:raise RuntimeError('candidate exited early')
  try:
   instance=json.load(open(data+'/.instance.json'))
   request=urllib2.Request('http://127.0.0.1:18763/api/state',headers={'Authorization':'Bearer '+instance['token']})
   response=json.load(urllib2.urlopen(request,timeout=3));break
  except (IOError,ValueError):time.sleep(.1)
 else:raise RuntimeError('candidate startup timeout')
 assert response['version']=='0.6.0-rc1',response['version']
 status=subprocess.Popen([binary,'--config',config,'--data',data,'--status'],stdout=subprocess.PIPE,stderr=subprocess.PIPE);stdout,stderr=status.communicate();assert status.returncode==0,stderr
 pid=process.pid
 memory=open('/proc/%d/status'%pid).read();rss=[line for line in memory.splitlines() if line.startswith('VmRSS:')][0]
 record={'version':response['version'],'uid':os.geteuid(),'terminalStatusPassed':True,'projects':len(response['state']['projects']),'sessions':len(response['state']['sessions']),'idleRSS':rss,'binary':binary,'before':before}
finally:
 process.terminate();process.wait();log.close()
record['after']=hashes();record['preserved']=before==record['after'];assert record['preserved'],record
print(json.dumps(record,indent=2))

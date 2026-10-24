#!/usr/bin/env python
# Python 2.6 compatible; dedicated CentOS GUI fixture only.
import os,sys,json,subprocess,time,signal,tempfile,urllib2
binary=sys.argv[1]
data=os.path.expanduser('~/.local/share/ic-workbench/production')
def instance():return json.load(open(data+'/.instance.json'))
original=instance();checks=[]
def run(args,expected=0,env=None,contains=None):
 p=subprocess.Popen(args,stdout=subprocess.PIPE,stderr=subprocess.PIPE,env=env)
 deadline=time.time()+12
 while p.poll() is None and time.time()<deadline:time.sleep(.1)
 if p.poll() is None:p.kill();p.wait();raise RuntimeError('terminal command timeout')
 out,err=p.communicate()
 assert p.returncode==expected,(args,p.returncode)
 if contains:assert contains in out+err,(args,'expected feedback missing')
 assert instance()==original,'repeat command changed active instance'
checks.append({'name':'running candidate','version':original['version'],'uid':os.getuid(),'pid':original['pid']})
for shell in ['/bin/sh','/bin/bash','/bin/csh','/bin/tcsh']:
 run([shell,'-c',binary+' --status']);checks.append({'name':shell+' terminal status','status':'PASS'})
run([binary,'--open'],contains='Browser open requested')
checks.append({'name':'repeat --open uses desktop opener and reuses backend','status':'PASS'})
run([binary,'--status','--open','--browser','/usr/bin/firefox'],contains='Browser open requested')
checks.append({'name':'--status --open explicit Firefox executable','status':'PASS'})
headless=dict(os.environ);headless.pop('DISPLAY',None);headless.pop('WAYLAND_DISPLAY',None)
run([binary,'--open'],1,headless,'no graphical session')
run([binary,'--open','--browser','/no/such/browser'],1,contains='no graphical browser launcher')
run([binary,'--open','--browser','/bin/false'],1,contains='browser launcher failed')
run([binary,'--browser','/usr/bin/firefox'],1,contains='requires --open')
for flag in ['--mcp','--version','--self-check']:run([binary,'--open',flag],1,contains='cannot be combined')
checks.append({'name':'headless, missing/failing browser and incompatible flags report errors without stopping service','status':'PASS'})
scratch=tempfile.mkdtemp(prefix='icw-headless-',dir=os.path.expanduser('~'))
log=open(scratch+'/startup.log','w')
p=subprocess.Popen([binary,'--open','--data',scratch+'/data','--listen','auto'],stdout=log,stderr=subprocess.STDOUT,env=headless)
try:
 deadline=time.time()+15
 while time.time()<deadline:
  try:
   value=json.load(open(scratch+'/data/.instance.json'))
   request=urllib2.Request(value['url']+'/api/state',headers={'Authorization':'Bearer '+value['token']})
   body=json.load(urllib2.urlopen(request,timeout=2))
   if 'backend remains available' in open(scratch+'/startup.log').read():break
  except (IOError,ValueError):pass
  time.sleep(.1)
 else:raise RuntimeError('headless fallback failed')
 assert body['version']=='0.6.0-rc2'
 assert value['pid']!=original['pid'] and value['url']!=original['url']
 os.kill(p.pid,signal.SIGINT)
 deadline=time.time()+10
 while p.poll() is None and time.time()<deadline:time.sleep(.1)
 assert p.poll()==0,'SIGINT failed'
 assert not os.path.exists(scratch+'/data/.instance.json')
 checks.append({'name':'first headless --open keeps HTTP available; auto port; Ctrl+C removes receipt','status':'PASS'})
finally:
 if p.poll() is None:p.terminate();p.wait()
 log.close()
assert instance()==original
print(json.dumps({'version':original['version'],'uid':os.getuid(),'checks':checks,'samePIDAndToken':True},indent=2))

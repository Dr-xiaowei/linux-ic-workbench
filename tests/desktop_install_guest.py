# -*- coding: utf-8 -*-
# Final installed native client validation, isolated CentOS fixture, Python 2.6.
import os,sys,time,json,subprocess,pwd,hashlib,shutil,signal,urllib2,ctypes as C
os.umask(0077);package=sys.argv[1];out='/home/icwgui/desktop-install-evidence'
if not os.path.isdir(out):os.makedirs(out,0700)
users=['icw','icwgui'];procs=[];report={'version':'0.7.0-rc1','users':{},'checks':[]}
def env(u):
 p=pwd.getpwnam(u);v=dict(os.environ,HOME=p.pw_dir,USER=u,LOGNAME=u,PATH='/usr/bin:/bin',DISPLAY=':99',XAUTHORITY=p.pw_dir+'/.Xauthority',LANG='en_US.UTF-8')
 for key in ['ICW_CONFIG_DIR','ICW_INSTALL_ROOT','ICW_BIN_DIR']:v.pop(key,None)
 if u=='icw':v['ICW_INSTALL_ROOT']='/home/icw/validation/app'
 return v
def uid(u):
 p=pwd.getpwnam(u)
 def setuid():os.setgroups([]);os.setgid(p.pw_gid);os.setuid(p.pw_uid)
 return setuid
def run(u,args,expected=0,e=None):
 p=subprocess.Popen(args,cwd=pwd.getpwnam(u).pw_dir,env=e or env(u),preexec_fn=uid(u),stdout=subprocess.PIPE,stderr=subprocess.PIPE);a,b=p.communicate()
 assert p.returncode==expected,(args,p.returncode,a[-700:],b[-700:]);return a+b
def paths(u):
 if u=='icw':return '/home/icw/validation/data','/home/icw/validation/config/config.json'
 return '/home/icwgui/.local/share/ic-workbench/production','/home/icwgui/.config/ic-workbench/production/config.json'
def args(u):
 d,c=paths(u);return ['--data',d,'--config',c,'--listen','auto']
def hashes(u):
 d,c=paths(u);return dict((p,hashlib.sha256(open(p,'rb').read()).hexdigest()) for p in [d+'/state.json',d+'/workspace.json.gz',c])
def request(i,route='state',token=None):
 r=urllib2.Request(i['url']+'/api/'+route,headers={'Authorization':'Bearer '+(token or i['token'])});return json.load(urllib2.urlopen(r,timeout=5))
def ready(u,p):
 end=time.time()+20
 while time.time()<end:
  assert p.poll() is None,'Native client exited early'
  try:
   i=json.load(open(paths(u)[0]+'/.instance.json'));v=request(i);assert v['version']=='0.7.0-rc1';return i,v
  except (IOError,ValueError):time.sleep(.1)
 raise AssertionError('Native startup timeout')
def stop(p):
 if p.poll() is None:p.terminate()
 end=time.time()+12
 while p.poll() is None and time.time()<end:time.sleep(.1)
 assert p.poll() is not None,'Native client failed to exit'
def titles():
 x=C.CDLL('libX11.so.6');x.XOpenDisplay.restype=C.c_void_p;x.XDefaultRootWindow.argtypes=[C.c_void_p];x.XDefaultRootWindow.restype=C.c_ulong
 x.XQueryTree.argtypes=[C.c_void_p,C.c_ulong,C.POINTER(C.c_ulong),C.POINTER(C.c_ulong),C.POINTER(C.POINTER(C.c_ulong)),C.POINTER(C.c_uint)]
 x.XFetchName.argtypes=[C.c_void_p,C.c_ulong,C.POINTER(C.c_char_p)];x.XFree.argtypes=[C.c_void_p];x.XCloseDisplay.argtypes=[C.c_void_p]
 os.environ['DISPLAY']=':99';os.environ['XAUTHORITY']='/home/icwgui/.Xauthority';d=x.XOpenDisplay(None);assert d;found=[]
 def walk(w,depth):
  name=C.c_char_p()
  if x.XFetchName(d,w,C.byref(name)) and name.value:found.append(name.value.decode('utf8','replace'));x.XFree(name)
  if depth>4:return
  r=C.c_ulong();p=C.c_ulong();children=C.POINTER(C.c_ulong)();n=C.c_uint()
  if x.XQueryTree(d,w,C.byref(r),C.byref(p),C.byref(children),C.byref(n)):
   for i in range(n.value):walk(children[i],depth+1)
   if children:x.XFree(children)
 walk(x.XDefaultRootWindow(d),0);x.XCloseDisplay(d);return found
def resource(pid):
 status=open('/proc/%d/status'%pid).read().splitlines();rss=[r for r in status if r.startswith('VmRSS:')][0];s=open('/proc/%d/stat'%pid).read().rsplit(')',1)[1].split()
 return {'pid':pid,'rssKiB':int(rss.split()[1]),'cpuTicks':int(s[11])+int(s[12]),'command':open('/proc/%d/cmdline'%pid).read().replace('\x00',' ')}
try:
 for u in users:
  p=pwd.getpwnam(u)
  if u=='icw':shutil.copyfile('/home/icwgui/.Xauthority',p.pw_dir+'/.Xauthority');os.chown(p.pw_dir+'/.Xauthority',p.pw_uid,p.pw_gid);os.chmod(p.pw_dir+'/.Xauthority',0600)
  before=hashes(u);text=run(u,[package+'/install.sh']);entry=p.pw_dir+'/.local/bin/ic-workbench'
  assert '0.7.0-rc1' in run(u,[entry,'--version'])
  e=env(u);e.pop('DISPLAY');noDisplay=run(u,[entry]+args(u),1,e);assert 'DISPLAY' in noDisplay
  log=open(out+'/'+u+'-native.log','w');process=subprocess.Popen([entry]+args(u),cwd=p.pw_dir,env=env(u),preexec_fn=uid(u),stdout=log,stderr=subprocess.STDOUT);procs.append(process)
  instance,state=ready(u,process)
  assert instance['uid']==p.pw_uid
  report['users'][u]={'uid':p.pw_uid,'nativePID':process.pid,'backendPID':instance['pid'],'before':before,'data':paths(u)[0],'projects':len(state['state'].get('projects') or []),'sessions':len(state['state'].get('sessions') or []),'headlessCorrect':True,'installationVerifiedFiles':text.count(': OK')}
  report['users'][u]['instance']=instance
  run(u,[entry]+args(u));assert json.load(open(paths(u)[0]+'/.instance.json'))['pid']==instance['pid']
  run(u,[entry,'--status']+args(u))
  try:urllib2.urlopen(instance['url']+'/');raise AssertionError('Desktop serves web UI')
  except urllib2.HTTPError as e:assert e.code==404
 process_list=subprocess.Popen(['ps','-eo','uid,args'],stdout=subprocess.PIPE).communicate()[0]
 assert not any(line.strip().startswith(('500 ','505 ')) and ('/firefox' in line or '/chrome' in line or '/electron' in line) for line in process_list.splitlines()),'Unexpected browser process'
 time.sleep(1);windows=titles();assert sum('IC Workbench' in t and 'Desktop' in t for t in windows)>=2,windows;report['nativeWindowTitles']=windows
 a=report['users']['icw']['instance'];b=report['users']['icwgui']['instance'];assert a['url']!=b['url'] and a['token']!=b['token']
 try:request(a,token=b['token']);raise AssertionError('Cross-user token accepted')
 except urllib2.HTTPError as e:assert e.code==401
 denied=run('icwgui',['/home/icwgui/.local/bin/ic-workbench','--service','--data',paths('icw')[0]],1);assert 'permission denied' in denied.lower() or 'owner' in denied.lower()
 report['checks']=['Both users installed from terminal','Native X11 windows opened by installed default command','No browser or WebView process','Distinct data/token/ports','Cross-user token and data path denied','Repeat command focuses same instance','API-only static route 404','No DISPLAY gives actionable error']
 beforeCPU={}
 for u in users:
  r=report['users'][u];r['processes']=[resource(r['nativePID']),resource(r['backendPID'])];beforeCPU[u]=sum(p['cpuTicks'] for p in r['processes'])
 start=time.time();time.sleep(5);duration=time.time()-start
 for u in users:
  r=report['users'][u];current=[resource(r['nativePID']),resource(r['backendPID'])];r['combinedRSSKiB']=sum(p['rssKiB'] for p in current);r['idleCPUPercentOneCore']=round(100*(sum(p['cpuTicks'] for p in current)-beforeCPU[u])/os.sysconf('SC_CLK_TCK')/duration,2)
 sys.path.insert(0,'/tmp');from native_capture import capture;capture(out+'/installed-native.png')
 for process in procs:stop(process)
 for u in users:
  r=report['users'][u];r['after']=hashes(u);r['preserved']=r['before']==r['after'];assert r['preserved'],u
  assert not os.path.exists(paths(u)[0]+'/.instance.json');del r['instance']
 report['checks'].append('Original state/draft/config bytes preserved across install/start/exit')
 # Explicit API process ownership: closing a native window must not kill it.
 u='icwgui';entry='/home/icwgui/.local/bin/ic-workbench';log=open(out+'/reused-service.log','w');service=subprocess.Popen([entry,'--service']+args(u),env=env(u),preexec_fn=uid(u),stdout=log,stderr=subprocess.STDOUT);procs.append(service);i,v=ready(u,service)
 ui=subprocess.Popen([entry,'--open']+args(u),env=env(u),preexec_fn=uid(u),stdout=open(out+'/reused-ui.log','w'),stderr=subprocess.STDOUT);procs.append(ui);time.sleep(3);assert ui.poll() is None;stop(ui);assert service.poll() is None;assert request(i)['version']=='0.7.0-rc1';stop(service);report['checks'].append('Closing UI preserves independently started API service')
 # No credentials appear in retained report.
 report['status']='PASS'
finally:
 for process in procs:
  if process.poll() is None:
   try:stop(process)
   except Exception:process.kill();process.wait()
 for u in report['users']:report['users'][u].pop('instance',None)
 with open(out+'/installed-native.json','w') as f:json.dump(report,f,indent=2)
print(json.dumps(report,indent=2))

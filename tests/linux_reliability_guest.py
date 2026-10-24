#!/usr/bin/env python
"""Python 2.6 acceptance in the disposable CentOS VM; root orchestrates real UIDs.
All paths/accounts are dedicated QA fixtures. Never run on a company server.
"""
from __future__ import with_statement
import os, sys, pwd, json, time, subprocess, urllib2, socket, signal, hashlib, threading, traceback, shutil, select

SHARED='/opt/icw-reliability-rc3'
OUT='/var/tmp/icw-reliability-rc3.json'
PREFIX=sys.argv[1] if len(sys.argv)>1 else 'icwqa'
USERS=[PREFIX+'_a',PREFIX+'_b']
PROCESSES=[]
RESULTS=[]
INSTANCES={}
def check(name, fn):
    t=time.time()
    try: evidence=fn(); r=dict(name=name,status='PASS',seconds=round(time.time()-t,3),evidence=evidence)
    except Exception: r=dict(name=name,status='FAIL',seconds=round(time.time()-t,3),error=traceback.format_exc())
    RESULTS.append(r)
    with open(OUT,'w') as f: json.dump(dict(platform=os.uname(),checks=RESULTS),f,indent=2)
    print(r['status']+' '+name); sys.stdout.flush()
    if r['status']=='FAIL': raise AssertionError(r['error'])
def home(u): return pwd.getpwnam(u).pw_dir
def data(u): return home(u)+'/.local/share/ic-workbench/production'
def impersonate(u):
    p=pwd.getpwnam(u)
    def before(): os.setsid(); os.setgroups([]); os.setgid(p.pw_gid); os.setuid(p.pw_uid)
    return before
def env(u): return dict(HOME=home(u),USER=u,LOGNAME=u,PATH='/usr/bin:/bin',SHELL='/bin/bash',LANG='en_US.UTF-8')
def command(u,args):
    p=subprocess.Popen(args,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,env=env(u),preexec_fn=impersonate(u),cwd=home(u))
    return (p.communicate()[0].decode('utf-8','replace'),p.returncode)
def launch(u,extra=None):
    log=home(u)+'/reliability-start-%d.log'%len(PROCESSES)
    f=open(log,'w');os.chmod(log,0600);os.chown(log,pwd.getpwnam(u).pw_uid,pwd.getpwnam(u).pw_gid)
    p=subprocess.Popen([SHARED+'/start.sh']+(extra or []),stdin=open('/dev/null'),stdout=f,stderr=subprocess.STDOUT,env=env(u),preexec_fn=impersonate(u),cwd=home(u))
    f.close();PROCESSES.append(p);return p,log
def api(v,route,payload=None,expected=200,token=None):
    req=urllib2.Request(v['url']+'/api/'+route, None if payload is None else json.dumps(payload),
      {'Authorization':'Bearer '+(v['token'] if token is None else token),'Content-Type':'application/json'})
    try:
        r=urllib2.urlopen(req,timeout=15);code=r.getcode();raw=r.read();r.close()
    except urllib2.HTTPError as e:code=e.code;raw=e.read()
    if expected is not None: assert code==expected,(route,code,raw[:400])
    try: body=json.loads(raw)
    except ValueError: body=raw
    return (code,body) if expected is None else body
def act(v,name,payload=None,expected=200):return api(v,'v2/action',dict(action=name,input=payload or {}),expected)
def wait_instance(u,p,path=None):
    for i in range(100):
        try:
            with open((path or data(u))+'/.instance.json') as f:v=json.load(f)
            assert v['pid']==p.pid
            assert api(v,'state')['version']=='0.5.0-rc3'
            return v
        except Exception:
            assert p.poll() is None,'startup exited %s'%p.returncode
            time.sleep(.1)
    raise AssertionError('startup timeout')
def stop(p,sig=signal.SIGTERM):
    if p.poll() is None:os.kill(p.pid,sig)
    for i in range(100):
        if p.poll() is not None:return
        time.sleep(.1)
    raise AssertionError('process did not stop')
def setup():
    assert os.geteuid()==0 and '6.9' in open('/etc/centos-release').read()
    for u in USERS:
        try: pwd.getpwnam(u)
        except KeyError: subprocess.check_call(['/usr/sbin/useradd','-m','-s','/bin/bash',u])
        os.chmod(home(u),0700)
        if os.path.exists(data(u)+'/state.json'):
            existing=json.load(open(data(u)+'/state.json'));assert existing['mode']=='production' and all(x['id']=='same-id' for x in (existing.get('projects') or []))
        p=pwd.getpwnam(u)
        folder=home(u)+'/project'
        if not os.path.exists(folder):os.mkdir(folder,0700)
        os.chown(folder,p.pw_uid,p.pw_gid)
        with open(folder+'/design.txt','w') as f:f.write(u+' private circuit fixture\n')
        os.chown(folder+'/design.txt',p.pw_uid,p.pw_gid);os.chmod(folder+'/design.txt',0600)
    return dict(users=[dict(name=u,uid=pwd.getpwnam(u).pw_uid) for u in USERS])
def concurrent_start():
    for u in USERS:INSTANCES[u]=dict(process=launch(u)[0])
    for u in USERS:INSTANCES[u]['instance']=wait_instance(u,INSTANCES[u]['process'])
    a,b=[INSTANCES[u]['instance'] for u in USERS]
    assert a['url']!=b['url'] and a['token']!=b['token'] and a['uid']!=b['uid']
    for u in USERS:
        v=INSTANCES[u]['instance'];uid=pwd.getpwnam(u).pw_uid
        for f in [data(u),data(u)+'/state.json',data(u)+'/.instance.json',home(u)+'/.config/ic-workbench/production/config.json']:
            st=os.stat(f);assert st.st_uid==uid and st.st_mode&0077==0,f
        act(v,'save-project',dict(id='same-id',name=u,root=home(u)+'/project',notes=u+' unique notes',description='personal fixture',tags=[u]))
        w=api(v,'workspace');w['data']['chat']={'text':u+' unsent text','scope':'general'};api(v,'workspace',w)
        prefs=act(v,'preferences')['preferences'];prefs['threshold']=81 if u==USERS[0] else 92;act(v,'save-preferences',prefs)
    return dict(addresses=[a['url'],b['url']],distinctUIDs=True,privatePermissions=True)
def isolation():
    a,b=[INSTANCES[u]['instance'] for u in USERS]
    api(a,'state',expected=401,token=b['token']);api(b,'workspace',expected=401,token=a['token'])
    for u in USERS:
        other=USERS[1] if u==USERS[0] else USERS[0]
        v=INSTANCES[u]['instance'];s=api(v,'state')['state'];w=api(v,'workspace')
        assert s['projects'][0]['notes']==u+' unique notes' and w['data']['chat']['text']==u+' unsent text'
        out,code=command(u,['/bin/cat',data(other)+'/state.json']);assert code!=0
        out,code=command(u,[SHARED+'/start.sh','--data',data(other)]);assert code!=0 and '#token=' not in out
        act(v,'save-project',dict(id='foreign',name='no access',root=home(other)+'/project'),400)
    return dict(crossToken401=True,crossUserFilesystemDenied=True,sharedDataRejected=True,projectNotesAndDraftsIndependent=True)
def duplicate_and_status():
    for u in USERS:
        v=INSTANCES[u]['instance']
        for args in [[],['--status']]:
            out,code=command(u,[SHARED+'/start.sh']+args);assert code==0 and v['url']+'/#token='+v['token'] in out,(code,out)
        assert INSTANCES[u]['process'].poll() is None
    return dict(existingInstanceReused=True,statusAuthenticated=True)
def mcp_auto_discovery():
    receipts=[]
    for u in USERS:
        p=subprocess.Popen([SHARED+'/start.sh','--mcp'],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=open('/dev/null','w'),env=env(u),preexec_fn=impersonate(u),cwd=home(u));PROCESSES.append(p)
        def exchange(req):
            p.stdin.write(json.dumps(req)+'\n');p.stdin.flush()
            if 'id' not in req:return
            ready=select.select([p.stdout],[],[],10)[0];assert ready,'MCP response timeout'
            result=json.loads(p.stdout.readline());assert result['id']==req['id'] and 'error' not in result,result
            return result['result']
        exchange(dict(jsonrpc='2.0',id=1,method='initialize',params=dict(protocolVersion='2024-11-05',capabilities={},clientInfo=dict(name='centos-qa',version='1'))))
        exchange(dict(jsonrpc='2.0',method='notifications/initialized'))
        tools=exchange(dict(jsonrpc='2.0',id=2,method='tools/list'))['tools'];assert len(tools)==3
        result=exchange(dict(jsonrpc='2.0',id=3,method='tools/call',params=dict(name='workspace_state',arguments={})))
        text=json.dumps(result);assert u+' unique notes' in text and not result.get('isError'),text
        p.stdin.close();stop(p);receipts.append(dict(user=u,readOnlyTools=len(tools),ownStateOnly=True))
    return receipts
def occupied_port():
    u=USERS[0];sock=socket.socket();sock.bind(('127.0.0.1',0));sock.listen(1);port=sock.getsockname()[1]
    folder=home(u)+'/occupied-data'
    out,code=command(u,[SHARED+'/start.sh','--listen','127.0.0.1:%d'%port,'--data',folder]);assert code!=0 and 'cannot listen' in out
    cfg=home(u)+'/.config/ic-workbench/production/config.json';raw=open(cfg).read();c=json.loads(raw);c['listen']='127.0.0.1:%d'%port
    with open(cfg,'w') as f:json.dump(c,f)
    try:
        p,log=launch(u,['--data',folder]);v=wait_instance(u,p,folder);assert int(v['url'].rsplit(':',1)[1])!=port;stop(p)
    finally:
        with open(cfg,'w') as f:f.write(raw)
        sock.close()
    return dict(explicitPortFailsClearly=True,oldFixedConfigFallsBack=True)
def concurrency():
    v=INSTANCES[USERS[0]]['instance'];w=api(v,'workspace');w['data']['chat']['text']='concurrent winner'
    replies=[]
    def save():replies.append(api(v,'workspace',w,expected=None)[0])
    ts=[threading.Thread(target=save) for _ in range(16)]
    for t in ts:t.start()
    for t in ts:t.join()
    assert replies.count(200)==1 and replies.count(409)==15,replies
    return dict(concurrentSaves=16,accepted=1,conflicts=15)
def crash_restart():
    u=USERS[0];v=INSTANCES[u]['instance'];p=INSTANCES[u]['process'];ack=[0];errors=[]
    def writer():
        for n in range(1,101):
            try:
                w=api(v,'workspace');w['data']['chat']['text']='crash-write-%d'%n;api(v,'workspace',w);ack[0]=n
            except Exception:break
    t=threading.Thread(target=writer);t.start()
    for i in range(100):
        if ack[0]>=8:break
        time.sleep(.02)
    assert ack[0]>=1
    stop(p,signal.SIGKILL);t.join();acknowledged=ack[0]
    p,log=launch(u);new=wait_instance(u,p);INSTANCES[u]=dict(process=p,instance=new)
    saved=api(new,'workspace')['data']['chat']['text'];saved_n=int(saved.rsplit('-',1)[1]);assert saved_n>=acknowledged,(saved_n,acknowledged)
    assert new['token']!=v['token'];api(new,'state',expected=401,token=v['token'])
    assert api(new,'state')['state']['projects'][0]['notes']==u+' unique notes'
    assert not [f for f in os.listdir(data(u)) if f.startswith('.atomic-')]
    other=INSTANCES[USERS[1]]['instance'];assert api(other,'workspace')['data']['chat']['text']==USERS[1]+' unsent text'
    return dict(acknowledgedWrites=acknowledged,recoveredWrite=saved_n,oldTokenRejected=True,otherUserUnaffected=True,orphanTempsCleared=True)
def disk_full():
    u=USERS[0];mount='/var/tmp/icw-qa-small-disk';os.mkdir(mount,0755)
    subprocess.check_call(['/bin/mount','-t','tmpfs','-o','size=2m,mode=0755','tmpfs',mount])
    p=None
    try:
        folder=mount+'/data';os.mkdir(folder,0700);pw=pwd.getpwnam(u);os.chown(folder,pw.pw_uid,pw.pw_gid)
        p,log=launch(u,['--data',folder]);v=wait_instance(u,p,folder)
        w=api(v,'workspace');w['data']['chat']={'text':'saved-before-full'};api(v,'workspace',w)
        old={}
        for name in ['state.json','workspace.json.gz']:old[name]=open(folder+'/'+name,'rb').read()
        f=open(mount+'/filler','wb',0)
        try:
            while True:f.write('X'*4096)
        except IOError as e:assert e.errno==28
        finally:f.close()
        w=api(v,'workspace');w['data']['chat']['text']='must-not-commit';api(v,'workspace',w,400)
        act(v,'save-project',dict(id='full',name='full',root=home(u)+'/project'),400)
        assert api(v,'workspace')['data']['chat']['text']=='saved-before-full'
        for name in old:assert open(folder+'/'+name,'rb').read()==old[name]
        os.unlink(mount+'/filler');w=api(v,'workspace');w['data']['chat']['text']='saved-after-space-restored';api(v,'workspace',w)
        stop(p);p=None
        return dict(realENOSPC=True,failedWritesVisible=True,previousBytesPreserved=True,writesRecoverAfterFreeSpace=True)
    finally:
        if p is not None:stop(p)
        subprocess.check_call(['/bin/umount',mount]);os.rmdir(mount)
def process_ownership():
    a,b=[INSTANCES[u]['instance'] for u in USERS]
    foreign=INSTANCES[USERS[1]]['process'].pid
    time.sleep(3.1);m=act(a,'monitor');rows=[r for r in m['processes'] if r['pid']==foreign]
    if not rows:
        # The monitor intentionally limits top rows. PID identity is read from /proc for this fixture.
        st=open('/proc/%d/stat'%foreign).read().rsplit(')',1)[1].split();start=st[19]
    else:start=rows[0]['start']
    act(a,'terminate',dict(pid=foreign,start=start),400)
    assert INSTANCES[USERS[1]]['process'].poll() is None
    return dict(otherUIDProcessTerminationRejected=True)
def malformed_and_permissions():
    u=USERS[0];root=home(u)
    for label,args in [('unsafe-dir',['--data',root+'/unsafe']),('broken-state',['--data',root+'/broken'])]:
        folder=args[1]
        if not os.path.exists(folder):os.mkdir(folder,0700)
        pw=pwd.getpwnam(u);os.chown(folder,pw.pw_uid,pw.pw_gid)
        if label=='unsafe-dir':os.chmod(folder,0777)
        else:
            with open(folder+'/state.json','w') as f:f.write('{broken preserved')
            os.chmod(folder+'/state.json',0600);os.chown(folder+'/state.json',pw.pw_uid,pw.pw_gid)
        out,code=command(u,[SHARED+'/start.sh']+args);assert code!=0
        if label=='broken-state':assert open(folder+'/state.json').read()=='{broken preserved'
    cfg=root+'/.config/ic-workbench/production/config.json';raw=open(cfg).read()
    with open(cfg,'w') as f:f.write(raw+'\n{}')
    out,code=command(u,[SHARED+'/start.sh']);assert code!=0 and 'single JSON' in out
    assert open(cfg).read()==raw+'\n{}'
    with open(cfg,'w') as f:f.write(raw)
    return dict(worldWritableDataRejected=True,corruptionNeverReset=True,trailingConfigurationRejected=True)
def soak():
    values={};requests=0
    for n in range(150):
        for u in USERS:
            v=INSTANCES[u]['instance'];s=api(v,'state');w=api(v,'workspace');assert s['state']['projects'][0]['name']==u;requests+=2
        if n%15==0:
            for u in USERS:act(INSTANCES[u]['instance'],'monitor');requests+=1
    for u in USERS:
        pid=INSTANCES[u]['process'].pid
        rows=open('/proc/%d/status'%pid).read().splitlines();values[u]=[r for r in rows if r.startswith(('VmRSS:','Threads:'))]
    return dict(requests=requests,allResponsesCorrect=True,processes=values)

try:
    for name,fn in [('real_uid_setup',setup),('two_users_default_terminal_start',concurrent_start),('data_token_and_project_isolation',isolation),('repeat_launch_and_terminal_status',duplicate_and_status),('mcp_cli_discovers_own_dynamic_port',mcp_auto_discovery),('occupied_port_and_old_config',occupied_port),('concurrent_workspace_compare_and_swap',concurrency),('sigkill_during_writes_and_restart',crash_restart),('actual_disk_full_and_recovery',disk_full),('cross_uid_process_control',process_ownership),('corrupt_data_config_and_permissions',malformed_and_permissions),('two_user_request_soak',soak)]:check(name,fn)
finally:
    for p in PROCESSES:
        try:stop(p)
        except Exception:pass

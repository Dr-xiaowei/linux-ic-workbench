#!/usr/bin/env python3
"""Production acceptance against the isolated QEMU guest; never uses Demo data.

Requires the explicitly prepared icw-vm SSH config and SSH forwarding. All writes
are confined to /home/icw/validation. Raw model replies are evidence, not commands.
"""
from pathlib import Path
import copy, json, math, os, re, subprocess, time, traceback, urllib.request, urllib.error

ROOT = Path(__file__).resolve().parents[1]
ENV = ROOT / 'reference-sandbox/linux-validation'
OUT = ROOT / 'docs/validation/linux-qemu'
OUT.mkdir(parents=True, exist_ok=True)
BASE = 'http://127.0.0.1:18865'
GUEST = '/home/icw/validation'
RESULTS = []
TOKEN = ''

def ssh(script, user='icw', check=True):
    r = subprocess.run(['ssh', '-F', str(ENV/'vm/ssh_config'), '-l', user,
                        'icw-vm', '/bin/bash', '-s'], input=script, text=True,
                       capture_output=True, timeout=180)
    if check and r.returncode:
        raise RuntimeError(r.stdout + r.stderr)
    return r

def refresh_token():
    global TOKEN
    text = ssh('cat '+GUEST+'/server.log').stdout
    TOKEN = re.search(r'#token=([a-f0-9]+)', text).group(1)

def api(route, data=None, expected=200, token=True):
    headers = {'Content-Type': 'application/json'}
    if token: headers['Authorization'] = 'Bearer ' + TOKEN
    req = urllib.request.Request(BASE+'/api/'+route,
        None if data is None else json.dumps(data).encode(), headers)
    try:
        with urllib.request.urlopen(req, timeout=180) as r:
            status, body = r.status, r.read()
    except urllib.error.HTTPError as e: status, body = e.code, e.read()
    assert status == expected, (route, status, body[:1200].decode(errors='replace'))
    try: return json.loads(body)
    except ValueError:
        if expected == 200: raise
        return {'error': body.decode(errors='replace')}

def act(action, data=None, expected=200):
    return api('v2/action', {'action': action, 'input': data or {}}, expected)

def op_wait(op, timeout=180):
    end = time.monotonic()+timeout
    while time.monotonic() < end:
        r = act('operation', {'id':op['id']})
        if r['status'] not in ('queued','running'): return r
        time.sleep(.3)
    raise AssertionError('operation timeout '+op['id'])

def agent(text, project='', folder='', allow=False, pending=False):
    s = api('sessions', {'title':'QEMU acceptance', 'project':project, 'folder':folder})
    r = api('v2/agent', {'session':s['id'], 'text':text, 'allowWrite':allow})
    return agent_wait(r, pending)

def agent_wait(r, pending=False):
    end = time.monotonic()+180
    while time.monotonic() < end:
        p = api('v2/progress?id='+r['id'])
        if p['status'] not in ('running','queued'):
            if pending or p['status'] != 'awaiting_approval': return p
            raise AssertionError('unexpected approval '+str(p))
        time.sleep(.3)
    raise AssertionError('agent timeout')

def check(name, fn):
    start=time.monotonic()
    try:
        evidence=fn()
        r={'name':name,'status':'PASS','seconds':round(time.monotonic()-start,3),'evidence':evidence}
    except Exception as e:
        r={'name':name,'status':'FAIL','seconds':round(time.monotonic()-start,3),
           'error':str(e),'traceback':traceback.format_exc()}
    RESULTS.append(r)
    (OUT/'api.json').write_text(json.dumps({'mode':'production, CentOS 6.9 x86_64 under QEMU',
        'model':'Qwen3-0.6B-Q4_0, llama.cpp b11146 on host via SSH loopback',
        'checks':RESULTS},ensure_ascii=False,indent=2)+'\n')
    print(r['status'],name,r.get('error',''),flush=True)

def setup():
    ssh('''set -eu
cd /home/icw/validation
mkdir -p projects/alpha/sub projects/beta fixtures/bin
printf 'gain=61 dB\nQEMU_UNIQUE_NEEDLE\n' > projects/alpha/design.txt
printf 'QEMU_UNIQUE_NEEDLE second-root\n' > projects/beta/extra.log
printf 'private\n' > private.txt
ln -sf ../../private.txt projects/alpha/escape
printf 'unreadable\n' > projects/alpha/denied.txt
chmod 000 projects/alpha/denied.txt
printf '\\000\\001\\002' > projects/alpha/binary.bin
printf 'export ICW_VERSION=one\n' > fixtures/v1.sh
printf 'export ICW_VERSION=two\n' > fixtures/v2.sh
printf 'setenv ICW_VERSION csh_three\n' > fixtures/v3.csh
printf 'export ICW_GLOBAL=loaded\n' > fixtures/global.sh
printf '# untouched personal shell configuration\n' > .cshrc.sentinel
''')
    p=act('preferences')['preferences']
    p.update(configured=True,modelUrl='http://127.0.0.1:18881/v1/chat/completions',
        model='icw-small',modelTools=True,keyEnv='ICW_VALIDATION_UNUSED_KEY',
        shell='/bin/bash',setup=GUEST+'/fixtures/global.sh',workspace=GUEST,
        threshold=90,pollSeconds=5,
        compilers={'python':{'executable':'/usr/bin/python','args':['-m','py_compile','{file}']},
                   'shell':{'executable':'/bin/bash','args':['-n','{file}']}},
        formatters={'python':{'executable':'/usr/bin/python','args':[GUEST+'/fixtures/format.py','{file}','{startLine}','{endLine}']}})
    act('save-preferences',p)
    for _ in range(80):
        v=act('preferences')['validation']
        if v['environment']!='checking' and v['model']!='checking': break
        time.sleep(.3)
    assert v['environment']=='ready' and v['model']=='connected',v
    return v

def projects():
    p={'id':'qemu-project','name':'模拟 IC 验证项目','root':GUEST+'/projects/alpha',
       'paths':[GUEST+'/projects/beta'],'notes':'备注 retained','description':'two registered roots','tags':['analog','验证']}
    act('save-project',p)
    act('note',{'project':p['id'],'path':'design.txt','notes':'已审阅','description':'TT corner'})
    listing=act('browse',{'project':p['id']})
    assert 'design.txt' in json.dumps(listing)
    assert '已审阅' in json.dumps(listing,ensure_ascii=False)
    text=act('browse',{'project':p['id'],'path':'design.txt'})
    assert 'gain=61' in text['content']
    search=act('search',{'query':'QEMU_UNIQUE_NEEDLE'})
    assert len(search['hits'])==2,search
    for path in ['../private.txt','escape','denied.txt','binary.bin']:
        act('browse',{'project':p['id'],'path':path},400)
    act('save-project',dict(p,id='invalid',root='/does/not/exist'),400)
    return {'search':search,'fileNoteVisible':True,'permissionAndPathBoundaries':True}

def software():
    sw={'id':'qemu-tool','name':'Environment probe','command':'printf "%s:%s" "$ICW_VERSION" "$ICW_GLOBAL"',
        'shell':'/bin/sh','active':'one','versions':[{'name':'one','setup':GUEST+'/fixtures/v1.sh'}, {'name':'two','setup':GUEST+'/fixtures/v2.sh'}]}
    act('save-software',sw); results=[]
    for version in ['one','two']:
        r=op_wait(act('launch-software',{'id':sw['id'],'version':version}))
        assert r['status']=='succeeded' and r['log']==version+':loaded',r
        results.append(r)
    sw.update(id='qemu-csh',name='CSH probe',shell='/bin/csh',active='three',
        versions=[{'name':'three','setup':GUEST+'/fixtures/v3.csh'}])
    act('save-software',sw)
    r=op_wait(act('launch-software',{'id':sw['id']}))
    assert r['status']=='succeeded' and r['log']=='csh_three:loaded',r
    assert ssh('cat '+GUEST+'/.cshrc.sentinel').stdout=='# untouched personal shell configuration\n'
    return results+[r]

def plugins():
    def plugin(pid,cmd,timeout=5,enabled=True):
        return {'id':pid,'name':pid,'version':'1','description':'owned acceptance fixture',
            'command':{'executable':'/bin/sh','args':['-c',cmd]},'enabled':enabled,'timeoutSeconds':timeout}
    ps=[plugin('qemu-echo','printf "plugin:%s" "$ICW_GLOBAL"'),
        plugin('qemu-timeout','sleep 30',1),plugin('qemu-cancel','sleep 30'),plugin('qemu-disabled','echo no',enabled=False)]
    act('install-plugins',{'plugins':ps})
    r=op_wait(act('run-plugin',{'id':'qemu-echo'}));assert r['status']=='succeeded' and r['log']=='plugin:loaded',r
    timed=op_wait(act('run-plugin',{'id':'qemu-timeout'}));assert timed['status'] in ['failed','cancelled'],timed
    running=act('run-plugin',{'id':'qemu-cancel'});act('cancel-operation',{'id':running['id']})
    cancelled=op_wait(running);assert cancelled['status']=='cancelled',cancelled
    act('run-plugin',{'id':'qemu-disabled'},400)
    act('install-plugins',{'plugins':[dict(ps[0],timeoutSeconds=0)]},400)
    manifest={'plugins':[plugin('qemu-python26','echo python26-script-import')]}
    payload=json.dumps(manifest)
    script="cat > "+GUEST+"/fixtures/plugins.json <<'ICWJSON'\n"+payload+"\nICWJSON\n"
    script+='ICW_TOKEN='+TOKEN+' /usr/bin/python '+GUEST+'/app/current-production/install_plugins.py --url http://127.0.0.1:8765 '+GUEST+'/fixtures/plugins.json\n'
    imported=ssh(script).stdout
    assert 'qemu-python26' in imported
    return {'successful':r,'timeout':timed['status'],'cancelled':cancelled['status'],'python26BatchImport':True}

def monitor():
    first=act('monitor');time.sleep(3.1);m=act('monitor')
    assert m['source']=='linux' and m['cpu'] is not None and m['memory'] is not None and m['storage'] is not None,m
    assert m['gpus']==[] and any('GPU unavailable' in n for n in m['notes']),m
    assert all(0<=m[k]<=100 for k in ('cpu','memory','storage'))
    p=act('preferences')['preferences'];p['threshold']=1;act('save-preferences',p);time.sleep(3.1)
    alerts=act('monitor')['alerts'];assert alerts,alerts
    p['threshold']=90;act('save-preferences',p)
    pid=int(ssh('nohup /bin/sleep 120 >/dev/null 2>&1 & echo $!').stdout.strip())
    time.sleep(3.1);m=act('monitor');proc=next(x for x in m['processes'] if x['pid']==pid)
    assert proc['own']
    act('terminate',{'pid':pid,'start':'wrong-start'},400)
    rootproc=next(x for x in m['processes'] if not x['own'])
    act('terminate',{'pid':rootproc['pid'],'start':rootproc['start']},400)
    act('terminate',{'pid':pid,'start':proc['start']})
    time.sleep(.5)
    state=ssh('ps -p '+str(pid)+' -o stat=',check=False).stdout.strip()
    assert not state or state.startswith('Z'),state
    return {'sample':m,'thresholdAlerts':alerts,'ownedProcessTermination':True,'staleAndForeignPIDRejected':True}

def calculator():
    cases=[('1/(2*pi*R*C)','R=1k\nC=1n','rad',159154.94309189535),
           ('gain','gain=20*log10(b/a)\na=10m\nb=1','rad',40),('sin(30)+cos(60)','','deg',1),('-2^2+2^3^2','','rad',508)]
    out=[]
    for expression,variables,angle,want in cases:
        r=act('calculate',dict(expression=expression,variables=variables,angle=angle))
        assert math.isclose(r['value'],want,rel_tol=1e-10),r;out.append(r)
    for expression,variables in [('1/0',''),('sqrt(-1)',''),('a','a=b;b=a'),('process.exit()','')]:
        act('calculate',dict(expression=expression,variables=variables),400)
    return out

def analysis():
    csv=act('analysis',{'name':'analog.csv','content':'time_s,voltage\n0,0\n1e-9,0.5\n2e-9,1\n'})
    assert len(csv['signals'])==1 and csv['signals'][0]['points'][1]['v']==.5,csv
    vcd='$timescale 1ns $end\n$scope module tb $end\n$var wire 1 ! clk $end\n$upscope $end\n$enddefinitions $end\n#0\n0!\n#5\n1!\n#10\nx!\n'
    wave=act('analysis',{'name':'digital.vcd','content':vcd})
    assert len(wave['signals'][0]['points'])==3 and wave['signals'][0]['points'][2]['v'] is None,wave
    for name in ['virtuoso.log','dc.rpt','calibre.report','vcs.log','netlist.sp']:
        r=act('analysis',{'name':name,'content':'Line 1\nWARNING: fixture\nERROR: convergence failure at 2 ns\n'})
        assert 'ERROR' in r['text'],r
    for item in [{'name':'private.fsdb','content':'binary'},{'name':'bad.csv','content':'time_s,v\n1,0\n0,1'}, {'name':'bad.vcd','content':'$timescale'}]:act('analysis',item,400)
    before=len(api('state')['state']['sessions'] or [])
    ai=act('analyze-ai',{'name':'fixture.log','content':'Start\nERROR: R1 resistance is negative (-100 ohm).\nSimulation stopped.\n',
        'question':'用中文指出日志中明确报告的问题，并引用原文。不需要调用工具。'})
    assert ai['demo'] is False and ai['content'].strip(),ai
    assert len(api('state')['state']['sessions'] or [])==before
    return {'csv':csv,'vcd':wave,'inlineAI':ai,'noChatNavigationOrSessionCreated':True}

def code():
    r=act('generate-code',{'language':'python','requirements':'Write a Python 2.6 compatible function add(a, b) returning a + b. Source only.'})
    assert r['demo'] is False and 'def add' in r['code'],r
    checked=op_wait(act('code-workflow',{'language':'python','code':r['code'],'mode':'check'}))
    assert checked['status']=='succeeded',checked
    original='def add(a, b):\n    return a - b\n'
    improved=act('improve-code',{'language':'python','code':original,'requirements':'Change add to return a + b. Return valid JSON with code and summary strings.'})
    assert improved['demo'] is False and 'a + b' in improved['code'],improved
    bad='def add(a, b)\n    return a + b\n'
    repaired=op_wait(act('code-workflow',{'language':'python','code':bad,'mode':'repair','maxRepairs':2,
        'requirements':'Fix only the missing colon. Must run on Python 2.6.'}))
    assert repaired['status']=='succeeded' and repaired['codeResult']['base']==bad,repaired
    assert len(repaired['codeResult']['checks'])>=2 and not repaired['codeResult']['checks'][0]['passed'],repaired
    source='def add(a, b):  \n    return a + b   \n'
    formatted=op_wait(act('code-workflow',{'language':'python','code':source,'mode':'format','startLine':2,'endLine':2}))
    assert formatted['status']=='succeeded' and formatted['codeResult']['code']=='def add(a, b):  \n    return a + b\n',formatted
    save={'project':'qemu-project','path':'generated.py','language':'python','code':improved['code']}
    act('save-code',save);act('save-code',save,400)
    act('save-code',dict(save,path='../escape.py'),400)
    act('save-code',dict(save,path='wrong.v'),400)
    return {'generated':r,'realPython26Check':checked,'improved':improved,'realAIRepairAndRecheck':repaired,
            'selectedFormatFixture':formatted,'exclusiveSave':True}

def chat():
    q=agent('用一句话解释欧姆定律。不要调用工具。')
    assert q['status']=='completed' and q['content'],q
    calc=agent('请调用 platform_action，action=calculate，input 为 JSON 字符串 {"expression":"7*8"}，再告诉我结果。')
    assert calc['status']=='completed' and any('calculate' in e['text'] for e in calc['events']),calc
    browse=agent('请使用 platform_action 调用 browse，input 为 JSON 字符串 {"path":"design.txt"}。读取文件后报告 gain 值。','qemu-project')
    assert browse['status']=='completed' and '61' in browse['content'],browse
    folder=agent('Use platform_action browse with input {"path":"extra.log"}. Report the contents.',folder=GUEST+'/projects/beta')
    assert folder['status']=='completed' and any('browse' in e['text'] for e in folder['events']),folder
    return {'ordinaryQuestion':q,'calculatorTool':calc,'projectScope':browse,'folderScope':folder}

def agent_control():
    text='Use platform_action action run-plugin with input JSON string {"id":"qemu-echo"}. Execute only this plugin.'
    denied=agent(text,allow=False)
    assert denied['status'] in ('completed','failed'),denied
    pending=agent(text,allow=True,pending=True)
    assert pending['status']=='awaiting_approval',pending
    api('v2/approve',{'id':pending['id'],'approve':True})
    done=agent_wait(pending)
    assert done['status']=='completed',done
    p2=agent(text,allow=True,pending=True)
    assert p2['status']=='awaiting_approval',p2
    api('v2/approve',{'id':p2['id'],'approve':False})
    rejected=agent_wait(p2)
    s=api('sessions',{'title':'Cancel model request'})
    r=api('v2/agent',{'session':s['id'],'text':'Write a long explanation of transistor operating regions.'})
    api('v2/cancel-agent',{'id':r['id']})
    cancelled=agent_wait(r)
    assert cancelled['status']=='cancelled',cancelled
    return {'readonly':denied,'approval':done,'rejected':rejected,'cancelled':cancelled}

def storage():
    w=api('workspace');old=copy.deepcopy(w)
    w['data']['calculator']={'expression':'R*C','variables':'R=1k;C=1n','angle':'rad'}
    w['data']['chat']={'text':'unsent survives restart','scope':'project','project':'qemu-project'}
    w['data']['codeLanguage']='python'
    w['data']['codeDrafts']={'python':{'code':'# draft\n'*10000,'original':'# original\n','requirements':'keep'}}
    saved=api('workspace',w);api('workspace',old,409)
    usage=act('storage-usage')
    api('backup',{},400);act('backup',{},400)
    sizes=ssh('wc -c '+GUEST+'/data/state.json '+GUEST+'/data/workspace.json.gz; find '+GUEST+'/data -maxdepth 1 -type f -printf "%f\\n"').stdout
    assert 'generated.py' not in sizes and 'backup-' not in sizes,sizes
    perms=ssh('stat -c "%a %n" '+GUEST+'/data/state.json '+GUEST+'/data/workspace.json.gz').stdout
    assert all(x.startswith('600 ') for x in perms.splitlines()),perms
    return {'usage':usage,'sizes':sizes,'permissions':perms,'revision':saved['revision'],'conflictRejected':True}

def auth():
    api('state',expected=401,token=False)
    state=api('state');assert state['state']['mode']=='production',state
    return {'version':state.get('version'),'unauthenticatedRejected':True}

def main():
    refresh_token()
    for name,fn in [('settings-auto-load-and-real-model',setup),('auth-and-production-mode',auth),
        ('projects-files-search-permissions',projects),('software-shell-and-csh-versions',software),
        ('plugins-batch-python26-timeout-cancel',plugins),('real-linux-monitor-alerts-process-control',monitor),
        ('calculator-formulas-variables-errors',calculator),('analysis-formats-waveforms-inline-ai',analysis),
        ('code-real-ai-python-check-repair-format-save',code),('chat-real-model-tools-and-scopes',chat),
        ('agent-readonly-approval-denial-cancel',agent_control),('local-storage-compression-conflict-no-backup',storage)]:
        check(name,fn)
    raise SystemExit(any(r['status']!='PASS' for r in RESULTS))

if __name__=='__main__': main()

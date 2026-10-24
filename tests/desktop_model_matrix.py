#!/usr/bin/env python3
"""Real model acceptance with independent outcome assertions, no response fixtures."""
from pathlib import Path
import os,json,time,subprocess,urllib.request,urllib.error,traceback,re,ast
ROOT=Path(__file__).resolve().parents[1];OUT=ROOT/'docs/validation/desktop-rc1';OUT.mkdir(exist_ok=True,parents=True)
SSH=['ssh','-F',str(ROOT/'reference-sandbox/linux-validation/vm/ssh_config'),'-l','root','icw-vm']
instance=json.loads(subprocess.check_output(SSH+['cat /home/icwgui/.local/share/ic-workbench/production/.instance.json'],stderr=subprocess.DEVNULL))
MODEL=os.environ.get('ICW_TEST_MODEL','Qwen3-0.6B-Q4_0');REPORT=OUT/(os.environ.get('ICW_TEST_REPORT','small-model-baseline')+'.json');checks=[]
def api(route,data=None):
 req=urllib.request.Request('http://127.0.0.1:18865/api/'+route,None if data is None else json.dumps(data).encode(),{'Authorization':'Bearer '+instance['token'],'Content-Type':'application/json'})
 try:
  with urllib.request.urlopen(req,timeout=160) as r:return json.load(r)
 except urllib.error.HTTPError as e:raise RuntimeError(e.read().decode())
def action(a,d=None):return api('v2/action',{'action':a,'input':d or {}})
def record(name,fn):
 start=time.time();result={'name':name}
 try:result.update(status='PASS',evidence=fn())
 except Exception as e:result.update(status='FAIL',error=str(e),traceback=traceback.format_exc())
 result['seconds']=round(time.time()-start,2);checks.append(result)
 REPORT.write_text(json.dumps({'model':MODEL,'backend':'CentOS 6.9 production native desktop API','inference':'host llama.cpp b11146 via loopback SSH','checks':checks},ensure_ascii=False,indent=2)+'\n')
 print(result['status'],name,result.get('error','')[:180],flush=True)
def wait_agent(run,limit=200):
 end=time.time()+limit
 while time.time()<end:
  r=api('v2/progress?id='+run['id'])
  if r['status'] not in ('running','awaiting_approval'):return r
  time.sleep(.4)
 api('v2/cancel-agent',{'id':run['id']});time.sleep(.5);return api('v2/progress?id='+run['id'])
def agent(prompt,mode='off'):
 s=api('sessions',{'title':'Small model acceptance','folder':'/home/icwgui/desktop-ai-project'})
 r=wait_agent(api('v2/agent',{'session':s['id'],'text':prompt,'linuxMode':mode}))
 (OUT/('model-run-'+r['id']+'.json')).write_text(json.dumps(r,ensure_ascii=False,indent=2)+'\n')
 return r
def setup():
 for i in range(100):
  try:
   with urllib.request.urlopen('http://127.0.0.1:18883/health',timeout=2) as r:json.load(r)
   break
  except Exception:time.sleep(.3)
 p=action('preferences')['preferences'];p.update(modelUrl='http://127.0.0.1:18883/v1/chat/completions',model='icw-small',modelTools=True,workspace='/home/icwgui/desktop-ai-project',configured=True)
 p['compilers']=p.get('compilers') or {};p['compilers']['python']={'executable':'/usr/bin/python','args':['-m','py_compile','{file}']}
 action('save-preferences',p)
 for i in range(100):
  r=action('preferences')['validation']
  if r['model']=='connected':return r
  time.sleep(.3)
 raise AssertionError(r)
def question():
 r=agent('A resistor has R=1000 ohms and V=1 volt. State the current in amperes and mA and the formula. No tools are needed. Answer briefly.')
 assert r['status']=='completed',r
 assert re.search(r'0\.001|1\s*mA',r['content']),r
 assert not any(e['stage']=='tool' for e in r.get('events',[])),r
 return r
def analyze():
 text='Synthetic validation log (not a real EDA run)\nERROR (SFE-23): Instance M1 references undefined model nmos_missing.\nSimulation terminated before transient analysis.\n'
 r=action('analyze-ai',{'name':'spectre-test.log','content':text,'question':'Explain the observed fatal cause, quote the missing model name and state whether transient results exist. Separate facts from hypotheses. Do not invent a successful run.'})
 assert 'nmos_missing' in r['content'],r
 assert re.search(r'not|no |before|没有|未|无',r['content'],re.I),r
 return r
def generate():
 r=action('generate-code',{'language':'verilog','requirements':'Create Verilog-2001 module and_gate(input wire a, input wire b, output wire y); assign y=a & b; endmodule. Combinational AND, no clock, no testbench, no markdown.'})
 (OUT/'generated-and-gate.v').write_text(r['code'])
 tool=ROOT/'reference-sandbox/verible-v0.0-4296-g0f262651-macOS/bin/verible-verilog-syntax'
 p=subprocess.run([str(tool),str(OUT/'generated-and-gate.v')],capture_output=True,text=True)
 assert p.returncode==0,(r,p.stdout,p.stderr)
 sim=ROOT/'reference-sandbox/linux-validation/tools/icarus/icarus-verilog/13.0'
 bench=OUT/'and-gate-tb.v';binary=OUT/'and-gate.vvp'
 bench.write_text('module tb; reg a,b; wire y; integer i; and_gate dut(a,b,y); initial begin for(i=0;i<4;i=i+1) begin {a,b}=i; #1; if(y !== (a & b)) $fatal(1,"AND mismatch"); end $display("AND_TRUTH_TABLE_4_PASS"); $finish; end endmodule\n')
 compile_result=subprocess.run([str(sim/'bin/iverilog'),'-B',str(sim/'lib/ivl'),'-o',str(binary),str(OUT/'generated-and-gate.v'),str(bench)],capture_output=True,text=True)
 assert compile_result.returncode==0,(r,compile_result.stdout,compile_result.stderr)
 simulation=subprocess.run([str(sim/'bin/vvp'),str(binary)],capture_output=True,text=True)
 assert simulation.returncode==0 and 'AND_TRUTH_TABLE_4_PASS' in simulation.stdout,(r,simulation.stdout,simulation.stderr)
 return {'response':r,'syntaxExit':p.returncode,'simulationExit':simulation.returncode,'simulationOutput':simulation.stdout,'semanticScope':'all four AND input combinations; real Icarus 13.0 on host, no proprietary EDA'}
def improve():
 original='def safe_divide(a, b):\n    return a / b\n'
 r=action('improve-code',{'language':'python','code':original,'requirements':'Python 2.6 compatible. If b == 0, return None; otherwise return float(a) / b. Preserve function name safe_divide and arguments a,b. No imports or other functions.'})
 tree=ast.parse(r['code'])
 forbidden=(ast.Import,ast.ImportFrom,ast.With,ast.AsyncWith,ast.ClassDef)
 assert not any(isinstance(n,forbidden) for n in ast.walk(tree)),r
 for n in ast.walk(tree):
  if isinstance(n,ast.Call):assert isinstance(n.func,ast.Name) and n.func.id=='float',r
 scope={};exec(compile(tree,'model-proposal','exec'),{'__builtins__':{'float':float}},scope)
 f=scope['safe_divide'];assert f(3,2)==1.5 and f(7,0) is None and f(-3,2)==-1.5,r
 return {'original':original,'response':r,'numericCases':[1.5,None,-1.5],'sourceFileNotOverwritten':True}
def repair():
 r=action('code-workflow',{'language':'python','code':'def add(a,b)\n    return a+b\n','requirements':'Fix syntax only; keep add(a,b) returning a+b, Python 2.6.','mode':'repair','maxRepairs':2})
 end=time.time()+240
 while time.time()<end:
  result=action('operation',{'id':r['id']})
  if result['status'] not in ('queued','running'):break
  time.sleep(.4)
 else:action('cancel-operation',{'id':r['id']});raise AssertionError('repair timeout')
 (OUT/'model-code-repair.json').write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n')
 c=result.get('codeResult',{});checks=c.get('checks') or []
 assert result['status']=='succeeded' and checks and not checks[0]['passed'] and checks[-1]['passed'],result
 assert all(v['executed'] for v in checks),result
 return result
def calculator():
 r=agent('Use platform_action calculate to evaluate R*C with R=2k and C=5n. Report the actual calculator result in seconds. Do not estimate it mentally.')
 assert r['status']=='completed',r
 receipts=[e['text'] for e in r['events'] if e['stage']=='receipt' and 'calculate' in e['text']]
 assert receipts and any('0.00001' in x or '1e-05' in x or '1e-5' in x for x in receipts),r
 return r
def linux_command():
 r=agent('Use Linux tools: inspect the system, then execute the command in this block:\n```sh\npwd; id -u\n```\nReport the real output and exit code. Do not run EDA tools.', 'auto')
 t=r.get('terminal') or {}
 assert r['status']=='completed' and t.get('exitCode')==0 and '/home/icwgui/desktop-ai-project' in t.get('output','') and '505' in t.get('output',''),r
 return r
def linux_multistep():
 name='model-note-'+str(int(time.time()))+'.txt'
 r=agent('Use Linux tools to inspect the system, create a NEW file '+name+' in the selected working directory containing exactly MODEL_CHAIN_OK followed by a newline, then read that file and verify its content. Use relative paths and write-file with expectedSHA256=new. Report actual receipts. Do not touch other files.','auto')
 out=subprocess.check_output(SSH+['cat /home/icwgui/desktop-ai-project/'+name],stderr=subprocess.DEVNULL)
 assert r['status']=='completed' and out==b'MODEL_CHAIN_OK\n',r
 assert any('read-file' in e['text'] and e['stage']=='receipt' for e in r['events']),r
 return r
record('model configuration auto-load',setup)
record('general engineering question with numeric reference',question)
record('EDA text analysis with grounded missing-model evidence',analyze)
record('Verilog generation and actual syntax checker',generate)
record('imported source optimization and independent numeric tests',improve)
record('compiler diagnostics -> AI repair -> real CentOS recheck',repair)
record('platform Agent calculator tool result',calculator)
record('Linux Agent actual command and UID',linux_command)
record('Linux Agent multi-step file creation and verification',linux_multistep)
print('SUMMARY',sum(x['status']=='PASS' for x in checks),'/',len(checks),flush=True)

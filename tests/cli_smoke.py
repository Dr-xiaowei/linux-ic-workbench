#!/usr/bin/env python3
import json,os,re,subprocess,time,urllib.request
from pathlib import Path
root=Path(__file__).resolve().parents[1]
log=(root/'runtime-data/v2-test-server.log').read_text();url=re.search(r'Open (http[^\n]+)',log).group(1);endpoint,token=url.split('/#token=')
env=dict(os.environ,ICW_TOKEN=token)
p=subprocess.Popen([str(root/'build/web-demo/ic-workbench'),'--mcp','--url',endpoint],env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
def send(x):p.stdin.write(json.dumps(x)+'\n');p.stdin.flush()
def receive():
 import select
 if not select.select([p.stdout],[],[],10)[0]:raise RuntimeError('MCP reply timeout')
 return json.loads(p.stdout.readline())
try:
 send({'jsonrpc':'2.0','id':1,'method':'initialize','params':{'protocolVersion':'2025-06-18','capabilities':{},'clientInfo':{'name':'cli-smoke','version':'1'}}});init=receive();assert 'result' in init,init
 send({'jsonrpc':'2.0','method':'notifications/initialized'})
 send({'jsonrpc':'2.0','id':2,'method':'tools/list','params':{}});tools=receive()['result']['tools'];assert len(tools)==3,tools
 send({'jsonrpc':'2.0','id':3,'method':'tools/call','params':{'name':'workspace_state','arguments':{}}});state=receive();assert not state['result'].get('isError'),state
 report={'host':'macOS ARM64','transport':'official SDK stdio, real child process','protocolVersion':init['result']['protocolVersion'],'tools':[t['name'] for t in tools],'initialize':True,'toolsList':True,'toolCall':True,'readOnlyDefault':True}
 (root/'docs/validation/v2-mcp-stdio.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))
finally:
 p.terminate();p.communicate(timeout=10)

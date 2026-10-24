# Test-only MCP endpoint, no EDA or shell execution. Python 2.6 compatible.
import sys,json,time

def emit(v):
 sys.stdout.write(json.dumps(v)+'\n');sys.stdout.flush()
while True:
 line=sys.stdin.readline()
 if not line:break
 req=json.loads(line);method=req.get('method');identity=req.get('id');p=req.get('params') or {}
 if identity is None:continue
 if method=='initialize':result={'protocolVersion':p.get('protocolVersion','2024-11-05'),'capabilities':{'tools':{}},'serverInfo':{'name':'explicit-non-EDA-test-fixture','version':'1'}}
 elif method=='tools/list':result={'tools':[{'name':'echo','description':'Return the supplied value. Synthetic protocol test, no EDA execution.','inputSchema':{'type':'object','properties':{'value':{'type':'string'}},'required':['value'],'additionalProperties':False}}]}
 elif method=='tools/call':
  token=(p.get('_meta') or {}).get('progressToken')
  if token is not None:
   emit({'jsonrpc':'2.0','method':'notifications/progress','params':{'progressToken':token,'progress':1,'total':2,'message':'FIXTURE progress: request received'}});time.sleep(.5)
  result={'content':[{'type':'text','text':'FIXTURE_RECEIPT '+p['arguments']['value']+' (no EDA run)'}]}
 elif method=='ping':result={}
 else:
  emit({'jsonrpc':'2.0','id':identity,'error':{'code':-32601,'message':'Unknown fixture method'}});continue
 emit({'jsonrpc':'2.0','id':identity,'result':result})

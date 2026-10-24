#!/usr/bin/env python3
"""Host driver: actual rc2 -> rc3 install in the retained CentOS QA account."""
import hashlib,json,subprocess,time,urllib.request,urllib.error
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
CFG=ROOT/'reference-sandbox/linux-validation/vm/ssh_config'
OUT=ROOT/'docs/validation/reliability-rc3'
PACKAGE='/opt/icw-delivery-rc3/ic-workbench-0.5.0-rc3-linux-amd64-production'
def ssh(script,user='icw',check=True):
 r=subprocess.run(['ssh','-F',str(CFG),'-l',user,'icw-vm','/bin/bash','-s'],input=script,text=True,capture_output=True,timeout=90)
 if check and r.returncode:raise RuntimeError(r.stdout+r.stderr)
 return r
def digest():return ssh('sha256sum /home/icw/validation/data/state.json /home/icw/validation/data/workspace.json.gz /home/icw/validation/config/config.json').stdout
def instance():return json.loads(ssh('cat /home/icw/validation/data/.instance.json').stdout)
def api(v,route,data=None):
 r=urllib.request.Request('http://127.0.0.1:18867/api/'+route,None if data is None else json.dumps(data).encode(),{'Authorization':'Bearer '+v['token'],'Content-Type':'application/json'})
 with urllib.request.urlopen(r,timeout=30) as x:return json.load(x)
before=digest()
r=ssh('set -eu\nICW_INSTALL_ROOT=/home/icw/validation/app '+PACKAGE+'/install.sh > /home/icw/validation/install-rc3.log\nreadlink "$HOME/.local/bin/ic-workbench"\nreadlink /home/icw/validation/app/current-production\n')
assert 'releases/0.5.0-rc3-production' in r.stdout
def start():
 ssh('export ICW_CONFIG_DIR=/home/icw/validation/config\nnohup "$HOME/.local/bin/ic-workbench" --data /home/icw/validation/data --listen 127.0.0.1:8765 > /home/icw/validation/server-rc3.log 2>&1 </dev/null &\nexit 0\n')
 for _ in range(80):
  try:
   v=instance();assert api(v,'state')['version']=='0.5.0-rc3';return v
  except Exception:time.sleep(.2)
 raise RuntimeError('startup timeout')
v=start();assert before==digest(),'upgrade changed existing persistent bytes'
for _ in range(100):
 validation=api(v,'v2/action',{'action':'preferences','input':{}})['validation']
 if validation['model']!='checking':break
 time.sleep(.2)
assert validation['model']=='connected' and validation['environment']=='ready',validation
status=ssh('ICW_CONFIG_DIR=/home/icw/validation/config "$HOME/.local/bin/ic-workbench" --data /home/icw/validation/data --status').stderr
assert v['url']+'/#token='+v['token'] in status
for shell in ['/bin/sh','/bin/bash','/bin/csh','/bin/tcsh']:
 result=ssh(shell+" -c '$HOME/.local/bin/ic-workbench --version'")
 assert '0.5.0-rc3' in result.stdout,(shell,result.stdout)
# A repeated install and an unrelated terminal entry must leave existing data/links intact.
retry=ssh('ICW_INSTALL_ROOT=/home/icw/validation/app '+PACKAGE+'/install.sh',check=False)
assert retry.returncode!=0 and 'already exists' in retry.stdout
sentinel=ssh('set -eu\nmkdir -p /home/icw/validation/foreign-bin\nprintf "UNRELATED_COMMAND\\n" > /home/icw/validation/foreign-bin/ic-workbench\nICW_INSTALL_ROOT=/home/icw/validation/refused-app ICW_BIN_DIR=/home/icw/validation/foreign-bin '+PACKAGE+'/install.sh',check=False)
assert sentinel.returncode!=0 and 'unrelated terminal command' in sentinel.stderr
assert ssh('cat /home/icw/validation/foreign-bin/ic-workbench').stdout=='UNRELATED_COMMAND\n'
# Actual SIGINT handling through the terminal command, then restart with identical data.
ssh('kill -INT '+str(v['pid']))
for _ in range(70):
 if ssh('test ! -e /home/icw/validation/data/.instance.json',check=False).returncode==0:break
 time.sleep(.1)
else:raise AssertionError('SIGINT did not stop instance')
assert before==digest(),'graceful stop changed prior data'
new=start();assert new['token']!=v['token'];assert before==digest()
archive=ROOT/'release/ic-workbench-0.5.0-rc3-linux-amd64-production.tar.gz'
sha=hashlib.sha256(archive.read_bytes()).hexdigest();assert sha in ssh('sha256sum /home/icw/validation/v5-rc3.tar.gz').stdout
binary_sha=hashlib.sha256((ROOT/'build/linux-amd64/ic-workbench').read_bytes()).hexdigest();assert binary_sha in ssh('sha256sum /home/icw/validation/app/current-production/ic-workbench').stdout
checks=ssh('cat /home/icw/validation/install-rc3.log').stdout.count(': OK')
result=dict(status='PASS',version='0.5.0-rc3',archiveSHA256=sha,archiveBytes=archive.stat().st_size,binarySHA256=binary_sha,verifiedPackageFiles=checks,
 terminalLink=r.stdout.strip().splitlines(),shells=['sh','bash','csh','tcsh'],oldStateWorkspaceConfigBytesPreserved=True,repeatInstallRejected=True,unrelatedCommandPreserved=True,SIGINTStopsAndRestartWorks=True,newTokenAfterRestart=True,startupValidation=validation)
(OUT/'installed-package.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result,indent=2))

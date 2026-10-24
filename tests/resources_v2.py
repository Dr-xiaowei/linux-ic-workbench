#!/usr/bin/env python3
import json,os,re,subprocess,time,urllib.request
from pathlib import Path
root=Path(__file__).resolve().parents[1]
log=(root/'runtime-data/v2-test-server.log').read_text();url=re.search(r'Open (http\S+)',log).group(1);base,token=url.split('/#token=')
pid=subprocess.check_output(['lsof','-t','-iTCP:18767','-sTCP:LISTEN'],text=True).strip().splitlines()[0]
def sample():
 s=subprocess.check_output(['ps','-p',pid,'-o','%cpu=,rss=,vsz='],text=True).split();return {'cpuPercent':float(s[0]),'rssMiB':int(s[1])/1024.,'virtualMiB':int(s[2])/1024.}
start=sample();time.sleep(10);end=sample()
assets={n:(root/'internal/workbench/web'/n).stat().st_size for n in ['app.js','style.css','index.html']}
report={'host':'macOS ARM64','mode':'offline demo after browser tests','samplingSeconds':10,'start':start,'end':end,'assetsBytes':assets,'totalFrontendBytes':sum(assets.values()),'limits':'Server process only; not browser process tree. Not CentOS, GPU, real EDA or shared-server certification.'}
(root/'docs/validation/v2-resources.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))

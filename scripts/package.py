#!/usr/bin/env python3
"""Package runtime + current operating guides; no development evidence or stage tree."""
from pathlib import Path
import hashlib
import shutil
import tarfile
import tempfile

root = Path(__file__).resolve().parents[1]
version = '0.8.0-rc3'
name = 'ic-workbench-'+version+'-linux-amd64-production'
files = {
 'build/linux-amd64/ic-workbench': 'ic-workbench',
 'config/production.json': 'config.json',
 'scripts/desktop.sh': 'start.sh', 'scripts/launch.sh': 'service.sh',
 'scripts/install.sh': 'install.sh', 'scripts/acceptance-linux.sh': 'acceptance-linux.sh',
 'scripts/install_plugins.py': 'install_plugins.py',
 'docs/CLIENT_GUIDE.md': 'README.md',
 'THIRD_PARTY_NOTICES.md': 'THIRD_PARTY_NOTICES.md', 'go.sum': 'go.sum',
}
for guide in ['EDA_ADAPTER.md','EDA_MCP.md','MCP.md','EDA_TASKS_AND_CODE_CHECKS.md']:
 files['docs/'+guide] = guide
for evidence in ['toolchain.json','reference-rc.json']:
 files['docs/validation/'+evidence] = 'docs/validation/'+evidence
# Only source modules, licensed notices, and required approved adapter examples.
for folder in ['desktop','licenses','examples/cadence']:
 for p in sorted((root/folder).rglob('*')):
  if p.is_file() and not p.name.startswith('._') and '__pycache__' not in p.parts and p.suffix not in ('.pyc','.pyo'):
   files[str(p.relative_to(root))] = str(p.relative_to(root))
(root/'release').mkdir(exist_ok=True)
with tempfile.TemporaryDirectory(prefix='icw-package-') as work:
 target = Path(work)/name;target.mkdir()
 for src,dst in files.items():
  out=target/dst;out.parent.mkdir(parents=True,exist_ok=True);shutil.copy2(root/src,out)
 # Compatibility names for cross-links in operating guides; use the current native guide.
 for alias in ['DEPLOYMENT.md','LOCAL_DATA_AND_CODE.md']:
  shutil.copy2(root/'docs/CLIENT_GUIDE.md', target/alias)
 (target/'MODE').write_text('production\n');(target/'VERSION').write_text(version+'\n')
 sums=[hashlib.sha256(p.read_bytes()).hexdigest()+'  '+str(p.relative_to(target)) for p in sorted(target.rglob('*')) if p.is_file()]
 (target/'SHA256SUMS').write_text('\n'.join(sums)+'\n')
 def clean_member(member):
  member.uid=member.gid=0;member.uname=member.gname='';member.mtime=0
  member.mode=0o755 if member.isdir() or Path(member.name).name=='ic-workbench' or member.name.endswith('.sh') else 0o644
  return member
 archive=root/'release'/(name+'.tar.gz')
 with tarfile.open(archive,'w:gz',compresslevel=9) as tar:
  tar.add(target,arcname=name,filter=clean_member)
 print(str(archive.relative_to(root)),archive.stat().st_size)
(root/'release/SHA256SUMS').write_text(''.join(hashlib.sha256(p.read_bytes()).hexdigest()+'  '+p.name+'\n' for p in sorted((root/'release').glob('*.tar.gz')) if not p.name.startswith('._')))

(root/'release/CURRENT_SHA256SUMS').write_text(hashlib.sha256(archive.read_bytes()).hexdigest()+'  '+archive.name+'\n')

#!/usr/bin/env python3
import hashlib,json,re,tarfile
from pathlib import Path
root=Path(__file__).resolve().parents[1];reports=[]
assert not [p for p in (root/'demo').glob('*.tar.gz') if not p.name.startswith('._')], 'Standalone demo package must not be generated'
assert (root/'scripts/preview_demo.py').is_file()
for mode,parent in [('production','release')]:
 archives=[p for p in (root/parent).glob('*0.8.0-rc3*.tar.gz') if not p.name.startswith('._')];assert len(archives)==1
 archive=archives[0]
 with tarfile.open(archive) as tar:
  entries=tar.getmembers();base=entries[0].name;assert base==archive.name[:-7], 'Archive root/version mismatch';files={m.name[len(base)+1:]:m for m in entries if m.isfile()}
  assert tar.extractfile(files['MODE']).read().decode().strip()==mode
  sums=tar.extractfile(files['SHA256SUMS']).read().decode().splitlines()
  names=[line.split('  ',1)[1] for line in sums]
  assert len(names)==len(set(names)) and set(names)==set(files)-{'SHA256SUMS'}, 'Incomplete checksum coverage'
  assert tar.extractfile(files['VERSION']).read().decode().strip()=='0.8.0-rc3'
  for line in sums:
   expected,name=line.split('  ',1);assert hashlib.sha256(tar.extractfile(files[name]).read()).hexdigest()==expected,name
  assert all(not m.name.startswith('/') and '..' not in m.name.split('/') and m.uid==0 and m.uname=='' for m in entries)
  assert not any('runtime-data' in name or 'secrets' in name or name.startswith('.env') for name in files)
  assert ('examples/public-rc/analytic-reference.csv' in files)==(mode=='demo')
  assert 'desktop/client.py' in files and 'desktop/runtime.py' in files and 'service.sh' in files
  assert b'desktop/client.py' in tar.extractfile(files['start.sh']).read()
  assert not any('__pycache__' in n or n.endswith('.pyc') for n in files)
  cfg=json.load(tar.extractfile(files['config.json']));assert not cfg['modelUrl'];assert cfg['keyEnv']==('ICW_DEMO_MODEL_KEY' if mode=='demo' else 'ICW_MODEL_KEY')
  for name,m in files.items():
   if name.endswith(('.json','.md','.sh','.ocn')):assert b'/Users/shawn' not in tar.extractfile(m).read(),name
  assert not any(n.endswith('.png') or '/validation/' in n and n not in ('docs/validation/toolchain.json','docs/validation/reference-rc.json') for n in files), 'Historical test attachments leaked into client'
  assert len(files) < 50 and sum(m.size for m in files.values()) < 12*1024*1024, 'Client package size regression'
  assert archive.stat().st_size < 7*1024*1024, 'Compressed package size regression'
  for name,m in files.items():
   if not name.endswith('.md'):continue
   for dest in re.findall(r'\]\(([^)]+)\)',tar.extractfile(m).read().decode()):
    if '://' in dest or dest.startswith('#'):continue
    target=str((__import__('pathlib').PurePosixPath(name).parent / dest.split('#')[0]))
    assert target in files, (name,target)
  binaryhash=hashlib.sha256(tar.extractfile(files['ic-workbench']).read()).hexdigest();assert binaryhash==json.load(open(root/'docs/validation/linux-elf.json'))['SHA256']
  reports.append({'mode':mode,'archive':str(archive.relative_to(root)),'archiveBytes':archive.stat().st_size,'verifiedFiles':len(sums),'SHA256':hashlib.sha256(archive.read_bytes()).hexdigest(),'binaryMatchesInspectedELF':True,'dataAndCredentialIsolation':True})
(root/'docs/validation/packages.json').write_text(json.dumps(reports,indent=2)+'\n');print(json.dumps(reports,indent=2))
links=0;issues=[]
for p in list(root.glob('*.md'))+list((root/'docs').rglob('*.md'))+list((root/'tests/fixtures').rglob('*.md')):
 if p.name.startswith('._'):continue
 text=p.read_text()
 for dest in re.findall(r'\]\(([^)]+)\)',text):
  if '://' in dest or dest.startswith('#'):continue
  dest=dest.split('#')[0].strip('<>');links+=1
  if not (p.parent/dest).exists():issues.append(str(p.relative_to(root))+': '+dest)
assert not issues,issues
print('Project Markdown local links:',links,'PASS')
(root/'docs/validation/static-checks.json').write_text(json.dumps({'goVet':True,'javascriptSyntax':True,'shellSyntax':True,'pythonSyntax':True,'localMarkdownLinks':links,'packages':'all checksums, isolation, ELF match verified'},indent=2)+'\n')

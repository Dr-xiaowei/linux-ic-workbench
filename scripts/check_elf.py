#!/usr/bin/env python3
"""Inspect the Linux ELF without pretending to execute it on macOS."""
import hashlib,json,struct
from pathlib import Path
root=Path(__file__).resolve().parents[1]
p=root/'build/linux-amd64/ic-workbench';b=p.read_bytes()
assert b[:4]==b'\x7fELF' and b[4:6]==b'\x02\x01'
machine=struct.unpack_from('<H',b,18)[0];assert machine==62
phoff=struct.unpack_from('<Q',b,32)[0];entsize,count=struct.unpack_from('<HH',b,54)
types=[struct.unpack_from('<I',b,phoff+i*entsize)[0] for i in range(count)]
assert 3 not in types, 'PT_INTERP: runtime loader dependency'
assert 2 not in types, 'PT_DYNAMIC: dynamic linking found'
assert b'go1.23.12' in b
report={'format':'ELF64 little-endian x86_64','static':True,'interpreter':None,'dynamicSegment':False,'sizeBytes':len(b),'SHA256':hashlib.sha256(b).hexdigest(),'toolchain':'go1.23.12','target':'GOOS=linux GOARCH=amd64 GOAMD64=v1 CGO_ENABLED=0','execution':'Structural inspection only; runtime evidence is recorded separately in docs/validation/linux-qemu/'}
(root/'docs/validation/linux-elf.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))

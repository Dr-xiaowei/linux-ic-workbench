"""Archive trust/path guards for the temporary developer toolchain."""
import hashlib, importlib.util, io, tarfile, tempfile, unittest
from pathlib import Path
spec = importlib.util.spec_from_file_location('dev_go', Path(__file__).resolve().parents[1]/'scripts/dev_go.py')
dev = importlib.util.module_from_spec(spec); spec.loader.exec_module(dev)
class ArchiveTests(unittest.TestCase):
 def archive(self, folder, name='go/bin/go', kind=tarfile.REGTYPE):
  path=folder/'fixture.tgz'
  with tarfile.open(path,'w:gz') as tar:
   m=tarfile.TarInfo(name);m.type=kind
   if kind==tarfile.REGTYPE:m.size=2;tar.addfile(m,io.BytesIO(b'ok'))
   else:m.linkname='../../escape';tar.addfile(m)
  return path,hashlib.sha256(path.read_bytes()).hexdigest()
 def test_verified_archive(self):
  with tempfile.TemporaryDirectory() as d:
   d=Path(d);p,h=self.archive(d);dev.unpack(p,d/'out',h)
   self.assertEqual((d/'out/go/bin/go').read_bytes(),b'ok')
 def test_wrong_checksum_no_extraction(self):
  with tempfile.TemporaryDirectory() as d:
   d=Path(d);p,h=self.archive(d)
   with self.assertRaises(ValueError):dev.unpack(p,d/'out','0'*64)
   self.assertFalse((d/'out').exists())
 def test_paths_and_links_rejected_before_extraction(self):
  for name,kind in [('go/../../escape',tarfile.REGTYPE),('/go/file',tarfile.REGTYPE),('other/file',tarfile.REGTYPE),('go/link',tarfile.SYMTYPE),('go/link',tarfile.LNKTYPE)]:
   with self.subTest(name=name,kind=kind),tempfile.TemporaryDirectory() as d:
    d=Path(d);p,h=self.archive(d,name,kind)
    with self.assertRaises(ValueError):dev.unpack(p,d/'out',h)
    self.assertFalse((d/'out').exists())
if __name__=='__main__':unittest.main()

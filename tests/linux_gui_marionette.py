#!/usr/bin/env python3
"""Tiny fixture client for ESR52's documented Marionette protocol (no runtime dependency)."""
import socket,json,time,pathlib,base64
class Browser:
 def __init__(self):
  self.s=socket.create_connection(('127.0.0.1',18868),timeout=60);self.id=0;self.hello=self.read()
 def read(self):
  n=b''
  while True:
   c=self.s.recv(1)
   if not c:raise RuntimeError('browser disconnected')
   if c==b':':break
   n+=c
  n=int(n);b=b''
  while len(b)<n:b+=self.s.recv(n-len(b))
  return json.loads(b)
 def call(self,name,args={}):
  self.id+=1;b=json.dumps([0,self.id,name,args]).encode();self.s.sendall(str(len(b)).encode()+b':'+b);r=self.read()
  if r[2]:raise RuntimeError(str(r[2]))
  return r[3]
 def js(self,s,*args):return self.call('executeScript',{'script':s,'args':args,'newSandbox':True})['value']
 def wait(self,s,timeout=30):
  end=time.time()+timeout
  while time.time()<end:
   if self.js(s):return
   time.sleep(.2)
  raise RuntimeError('GUI wait timed out: '+s)
 def screenshot(self,path):
  v=self.call('takeScreenshot',{'full':False});pathlib.Path(path).write_bytes(base64.b64decode(v['value'] if isinstance(v,dict) else v))
if __name__=='__main__':
 b=Browser();print(b.hello);print(b.call('newSession',{'capabilities':{}}));print(b.call('getCurrentUrl'));print(b.js('return {title:document.title,text:document.body.textContent.slice(0,1500)}'));b.screenshot(pathlib.Path(__file__).resolve().parents[1]/'docs/validation/terminal-gui-rc2/linux-firefox-initial.png')

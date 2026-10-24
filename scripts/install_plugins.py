#!/usr/bin/env python
"""Install one or many workbench plugin manifests. Python 2.6+/3, stdlib only."""
from __future__ import print_function
import json, os, sys
from optparse import OptionParser
try:
 from urllib.request import Request, build_opener, HTTPRedirectHandler
 from urllib.parse import urlsplit
except ImportError:
 from urllib2 import Request, build_opener, HTTPRedirectHandler
 from urlparse import urlsplit
class NoRedirect(HTTPRedirectHandler):
 def redirect_request(self,req,fp,code,msg,headers,newurl):
  raise ValueError('redirect rejected')
urlopen=build_opener(NoRedirect()).open
parser=OptionParser('%prog [--url http://127.0.0.1:8765] manifest.json')
parser.add_option('--url',default='http://127.0.0.1:8765')
options,args=parser.parse_args()
if len(args)!=1:parser.error('provide one JSON file with a plugins array')
try:
 parsed=urlsplit(options.url)
 valid=parsed.scheme=='http' and parsed.hostname in ('127.0.0.1','localhost','::1') and not parsed.username and not parsed.password and parsed.path in ('','/') and not parsed.query and not parsed.fragment and parsed.port and 1<=parsed.port<=65535
except ValueError:valid=False
if not valid:parser.error('loopback URL required; use SSH forwarding')
token=os.environ.get('ICW_TOKEN')
if not token:parser.error('set ICW_TOKEN from the workbench startup URL')
with open(args[0],'r') as f:manifest=json.load(f)
if 'plugins' not in manifest:manifest={'plugins':[manifest]}
data=json.dumps({'action':'install-plugins','input':manifest}).encode('utf-8')
try:
 response=urlopen(Request(options.url.rstrip('/')+'/api/v2/action',data,{'Authorization':'Bearer '+token,'Content-Type':'application/json'}),timeout=15)
 print(response.read().decode('utf-8'))
except Exception as error:
 print('Plugin import failed: '+str(error),file=sys.stderr);sys.exit(1)

#!/usr/bin/env python3
"""Serve the existing full web UI with an isolated offline demo backend. No installer."""
import argparse, hashlib, os, platform, re, shutil, signal, subprocess, sys, time
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

def build():
    directory = ROOT / 'build/web-demo'
    directory.mkdir(parents=True, exist_ok=True)
    binary, stamp = directory / 'ic-workbench', directory / 'source.sha256'
    digest = hashlib.sha256((platform.system() + platform.machine()).encode())
    sources = [ROOT/'go.mod', ROOT/'go.sum']
    for folder in ['cmd', 'internal', 'vendor']:
        sources.extend(p for p in (ROOT/folder).rglob('*') if p.is_file() and not p.name.startswith('._'))
    for p in sorted(sources):
        digest.update(str(p.relative_to(ROOT)).encode()); digest.update(p.read_bytes())
    current = digest.hexdigest()
    if binary.exists() and stamp.exists() and stamp.read_text() == current:
        return binary
    go = os.environ.get('GO') or str(ROOT/'scripts/go.sh')
    env = dict(os.environ, GOTOOLCHAIN='local', CGO_ENABLED='0')
    for key in ['GOOS', 'GOARCH', 'GOAMD64']:
        env.pop(key, None)
    subprocess.run([go, 'build', '-mod=vendor', '-o', str(binary), './cmd/workbench'], cwd=str(ROOT), env=env, check=True)
    stamp.write_text(current)
    return binary

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--port', type=int, default=18766)
    args = parser.parse_args()
    if not 1024 <= args.port <= 65535:
        parser.error('port must be 1024..65535')
    os.umask(0o077)
    binary = build()
    # No production or model configuration is loaded. This URL is always loopback.
    base = 'http://127.0.0.1:' + str(args.port)
    data = ROOT/'runtime-data/web-demo'
    child = subprocess.Popen([str(binary), '--mode', 'demo', '--listen', '127.0.0.1:'+str(args.port), '--data', str(data)],
                             cwd=str(ROOT), stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, bufsize=1)
    def stop(signum, frame):
        if child.poll() is None: child.terminate()
    signal.signal(signal.SIGTERM, stop); signal.signal(signal.SIGINT, stop)
    try:
        for line in child.stdout:
            match = re.match(r'Open (http://127\.0\.0\.1:[0-9]+/#token=([a-f0-9]+))', line)
            if not match:
                if not line.startswith('MCP token:'): print(line, end='', flush=True)
                continue
            url, token = match.groups()
            request = urllib.request.Request(base+'/api/demo-seed', data=b'{}', headers={
                'Authorization':'Bearer '+token, 'Content-Type':'application/json'})
            for attempt in range(30):
                try:
                    with urllib.request.urlopen(request, timeout=2) as response:
                        response.read()
                    break
                except OSError:
                    if child.poll() is not None or attempt == 29: raise
                    time.sleep(0.1)
            print('WEB_DEMO_URL='+url, flush=True)
            print('Offline web demo ready. Keep this process running; Ctrl+C stops it.', flush=True)
        return child.wait()
    finally:
        if child.poll() is None:
            child.terminate()
            try: child.wait(timeout=5)
            except subprocess.TimeoutExpired: child.kill(); child.wait()

if __name__ == '__main__':
    try: sys.exit(main())
    except (OSError, RuntimeError, subprocess.CalledProcessError) as e:
        print('Preview failed: '+str(e), file=sys.stderr); sys.exit(1)

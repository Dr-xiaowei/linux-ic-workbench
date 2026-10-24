#!/usr/bin/env python3
"""Run the pinned offline development toolchain without a permanent expanded copy.
Only used on the development host; never shipped to the Linux client.
"""
import hashlib
import os
from pathlib import Path
import platform
import subprocess
import sys
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[1]
ARCHIVE = ROOT / 'reference-sandbox/toolchain/go1.23.12.darwin-arm64.tar.gz'
SHA256 = '5bfa117e401ae64e7ffb960243c448b535fe007e682a13ff6c7371f4a6f0ccaa'


def unpack(archive, destination, expected):
    if hashlib.sha256(archive.read_bytes()).hexdigest() != expected:
        raise ValueError('Go archive checksum mismatch; no files extracted')
    with tarfile.open(archive) as tar:
        members = tar.getmembers()
        for member in members:
            parts = Path(member.name).parts
            if not parts or parts[0] != 'go' or '..' in parts or Path(member.name).is_absolute():
                raise ValueError('Unsafe archive path')
            if not (member.isfile() or member.isdir()):
                raise ValueError('Archive links/devices are excluded')
        tar.extractall(destination, members=members)


def main():
    if platform.system() != 'Darwin' or platform.machine() not in ('arm64', 'aarch64'):
        raise ValueError('Set GO to Go 1.23.12 for this development host; bundled archive is macOS ARM64')
    # Each command owns its temporary GOROOT and caches; concurrent callers do not delete each other.
    with tempfile.TemporaryDirectory(prefix='icw-go-') as directory:
        unpack(ARCHIVE, Path(directory), SHA256)
        env = dict(os.environ, GOTOOLCHAIN='local', GOROOT=str(Path(directory)/'go'))
        env.setdefault('GOCACHE', str(Path(directory)/'cache'))
        env.setdefault('GOPATH', str(Path(directory)/'gopath'))
        env.setdefault('GOPROXY', 'off')
        env.setdefault('GOSUMDB', 'off')
        try:
            return subprocess.call([str(Path(directory)/'go/bin/go')] + sys.argv[1:], env=env)
        except KeyboardInterrupt:
            return 130


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (OSError, ValueError, tarfile.TarError) as error:
        print('Development Go: '+str(error), file=sys.stderr)
        sys.exit(1)

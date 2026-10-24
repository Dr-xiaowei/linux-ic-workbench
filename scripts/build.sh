#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
GO=${GO:-"$PWD/scripts/go.sh"}
case "$("$GO" version)" in *go1.23.12*) ;; *) echo 'Use the verified Go 1.23.12 toolchain for Linux 2.6.32.' >&2; exit 1;; esac
mkdir -p build/linux-amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOAMD64=v1 "$GO" build -trimpath -ldflags='-s -w' -o build/linux-amd64/ic-workbench ./cmd/workbench
python3 scripts/check_elf.py
python3 scripts/package.py

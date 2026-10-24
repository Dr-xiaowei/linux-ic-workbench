#!/bin/sh
# Development host only. Expanded Go and default caches are removed after each command.
set -eu
exec python3 "$(dirname "$0")/dev_go.py" "$@"

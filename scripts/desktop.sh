#!/bin/sh
set -eu
umask 077
self=$(readlink -f -- "$0")
package_dir=$(CDPATH= cd -- "$(dirname -- "$self")" && pwd -P)
case "${1:-}" in
 --service) shift; exec "$package_dir/service.sh" --desktop-api "$@" ;;
 --status|--version|--self-check|--mcp) exec "$package_dir/service.sh" "$@" ;;
 --help|-h)
  printf '%s\n' 'IC Workbench native Linux desktop' 'Usage: ic-workbench [--open] [--data PATH] [--config PATH]' '       ic-workbench --status | --version | --self-check' '       ic-workbench --service  (local API for automation; no web interface)' 'Requires an X11 desktop/VNC and Python tkinter (Python 2.6+ or 3).'
  exit 0 ;;
esac
python_bin=${ICW_PYTHON:-}
if [ -z "$python_bin" ]; then
 for candidate in python3 python; do
  if command -v "$candidate" >/dev/null 2>&1 && "$candidate" -c 'try: import tkinter
except ImportError: import Tkinter' >/dev/null 2>&1; then python_bin=$candidate; break; fi
 done
fi
if [ -z "$python_bin" ]; then
 printf '%s\n' 'Native desktop needs the system tkinter package.' 'CentOS 6.9: ask your administrator to install tkinter from the company repository.' >&2
 exit 1
fi
exec "$python_bin" "$package_dir/desktop/client.py" "$@"

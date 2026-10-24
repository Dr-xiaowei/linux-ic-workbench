#!/bin/sh
# Linux target acceptance: shell + curl + ps from the host; no network downloads.
set -eu
umask 077
package_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
binary="$package_dir/ic-workbench"
report=${1:-"$PWD/icw-acceptance-$(date +%Y%m%d-%H%M%S)"}
mkdir -p "$report"
work=$(mktemp -d /tmp/icw-acceptance.XXXXXX)
pid=''
cleanup(){ if [ -n "$pid" ]; then kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; fi; }
trap cleanup EXIT HUP INT TERM
{
 uname -a
 if [ -r /etc/centos-release ]; then cat /etc/centos-release; fi
 getconf GNU_LIBC_VERSION || true
 "$binary" --version
 "$binary" --self-check
} > "$report/platform.txt" 2>&1
"$binary" --mode demo --listen auto --data "$work/data" > "$work/server.log" 2>&1 &
pid=$!
i=0
while [ "$i" -lt 20 ]; do
 token=$(sed -n 's/.*#token=//p' "$work/server.log" | head -1)
 base=$(sed -n 's|^Open \(http://[^/]*\)/#token=.*|\1|p' "$work/server.log" | head -1)
 if [ -n "$token" ] && [ -n "$base" ] && curl -fsS -H "Authorization: Bearer $token" "$base/api/state" > "$report/state.json" 2>/dev/null; then break; fi
 i=$((i+1)); sleep 1
done
[ "$i" -lt 20 ] || { cp "$work/server.log" "$report/startup-error.txt"; echo 'FAIL: startup/API'; exit 1; }
curl -fsS -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d '{}' "$base/api/demo-seed" > "$report/seed.json"
curl -fsS -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d '{"project":"demo-rc","kind":"simulate","params":{"R":1000,"C":1e-9}}' "$base/api/jobs" > "$report/job.json"
sleep 2
curl -fsS -H "Authorization: Bearer $token" "$base/api/state" > "$report/state-after.json"
if ! grep -q '"status":"completed"' "$report/state-after.json"; then echo 'FAIL: demo execution'; exit 1; fi
ps -p "$pid" -o pid,ppid,%cpu,rss,vsz,etime,comm > "$report/resources-start.txt"
sleep 10
ps -p "$pid" -o pid,ppid,%cpu,rss,vsz,etime,comm > "$report/resources-idle.txt"
printf '%s\n' 'PASS: binary start, local API, persistent store, analytical demo job.' 'NOT TESTED: actual GLM, Virtuoso/Spectre/PDK, browser process tree, company server load.' > "$report/result.txt"
cleanup;pid=''
printf 'Acceptance evidence: %s\nTemporary demo data retained: %s\n' "$report" "$work"

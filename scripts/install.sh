#!/bin/sh
# Install a new version alongside the existing version; user data/config stay outside releases.
set -eu
umask 077
package_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
install_root=${ICW_INSTALL_ROOT:-"$HOME/.local/opt/ic-workbench"}
bin_dir=${ICW_BIN_DIR:-"$HOME/.local/bin"}
mode=$(cat "$package_dir/MODE")
version=$(cat "$package_dir/VERSION")
case "$mode" in production|demo) ;; *) exit 1;; esac
case "$version" in *[!a-zA-Z0-9._-]*|'') echo 'Invalid version' >&2; exit 1;; esac
case "$install_root:$bin_dir" in /*:/*) ;; *) echo 'Install and bin directories must be absolute' >&2; exit 1;; esac
entry="$bin_dir/ic-workbench"
expected="$install_root/current-$mode/start.sh"
if [ -e "$entry" ] || [ -L "$entry" ]; then
 if [ ! -L "$entry" ] || [ "$(readlink -- "$entry")" != "$expected" ]; then
  echo "Refusing to replace an unrelated terminal command: $entry" >&2; exit 1
 fi
fi
(cd "$package_dir" && sha256sum -c SHA256SUMS)
target="$install_root/releases/$version-$mode"
mkdir -p "$install_root/releases"
if [ -e "$target" ]; then echo "Version already exists: $target; no data changed"; exit 1; fi
stage=$(mktemp -d "$install_root/.install-$mode.XXXXXX")
link_tmp="$install_root/.current-$mode-$$"
entry_tmp="$bin_dir/.ic-workbench-$$"
cleanup(){ rm -rf -- "$stage"; rm -f -- "$link_tmp" "$entry_tmp"; }
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
cp -R "$package_dir/." "$stage/"
(cd "$stage" && sha256sum -c SHA256SUMS >/dev/null)
# GNU mv -n does not replace a concurrently installed release.
mv -Tn "$stage" "$target"
if [ -d "$stage" ]; then echo "Concurrent installation detected: $target" >&2; exit 1; fi
ln -s "releases/$version-$mode" "$link_tmp"
# mv -T is provided by CentOS GNU coreutils; avoid following an old directory symlink.
mv -Tf "$link_tmp" "$install_root/current-$mode"
mkdir -p "$bin_dir"
ln -s "$expected" "$entry_tmp"
mv -Tf "$entry_tmp" "$entry"
printf 'Installed %s\nTerminal + GUI: %s --open\nStatus: %s --status\nStop: Ctrl+C in the startup terminal\nIf ~/.local/bin is on PATH, use: ic-workbench\n' "$target" "$entry" "$entry"

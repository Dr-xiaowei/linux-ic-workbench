#!/bin/sh
set -eu
umask 077
# GNU readlink is part of CentOS 6 coreutils; also resolves the ~/.local/bin entry.
self=$(readlink -f -- "$0")
package_dir=$(CDPATH= cd -- "$(dirname -- "$self")" && pwd -P)
mode=$(cat "$package_dir/MODE")
case "$mode" in production|demo) ;; *) echo 'Invalid package mode' >&2; exit 1;; esac
config_dir=${ICW_CONFIG_DIR:-"$HOME/.config/ic-workbench/$mode"}
exec "$package_dir/ic-workbench" --config "$config_dir/config.json" --config-template "$package_dir/config.json" "$@" --mode "$mode"

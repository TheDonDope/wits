#!/usr/bin/env bash
# The phone's engine against the CLI, through a shared folder, both ways:
# the engine starts the folder and records; wits pull takes it, records, and
# pushes; the engine pulls that in. Every journal must verify, and the two must
# end the same. Run by `make wasm-check`, after `make build wasm`.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
wits="$here/bin/wits"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/share" "$work/deck"
check() {
  node --disable-warning=MODULE_TYPELESS_PACKAGE_JSON "$here/wits-ui/scripts/engine-check.mjs" \
    "$here/wits-ui/public/wits.wasm" "$here/wits-ui/public/wasm_exec.js" "$work/share" "$@"
}

echo "· the phone starts the folder"
check >"$work/first.json"
cd "$work/deck"
"$wits" init . >/dev/null
"$wits" remote add "$work/share" --key "$work/share/phone.key" >/dev/null
"$wits" pull
"$wits" grind wedding 2 >/dev/null
"$wits" push
"$wits" fsck | tail -1

echo "· the phone pulls the deck's grind"
check "$work/share/phone.key" >"$work/second.json"  # the same phone, its files kept
"$wits" pull
"$wits" fsck | tail -1
fetched=$(node -p "require('$work/second.json').pulled.fetched")
total=$(node -p "require('$work/second.json').overview")
deck=$("$wits" log --oneline | wc -l)
echo "phone pulled $fetched, holds $total; deck holds $deck"
[ "$fetched" -eq 1 ] && [ "$total" -eq "$deck" ] || { echo "✗ the copies differ"; exit 1; }
echo "✓ the phone and the deck agree"

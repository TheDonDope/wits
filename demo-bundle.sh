#!/usr/bin/env bash
# Makes the demo (assets/demo-seed.wits): the workbook imported into a new
# repository, then wits-ui/scripts/demo-seed.ts run against it through
# wits-server — prices off the invoices, devices, a year of sessions and
# ratings, terpenes looked up, the stashes left behind cleared — and the
# result bundled. "Seed with demo data", on the browser's menu, restores it.
#
#   ./demo-bundle.sh [invoice folder]     # default: assets/invoices
#
# The invoices name the patient and are not committed; without them the demo
# has no prices. Nothing here is written to the repository in the working tree.
set -euo pipefail

root="$(cd "$(dirname "$0")" && pwd)"
invoices="${1:-$root/assets/invoices}"
port=7349
work="$(mktemp -d)"
trap 'kill "${server:-0}" 2>/dev/null || true; rm -rf "$work"' EXIT

make -C "$root" build server >/dev/null
"$root/bin/wits" init "$work/demo" >/dev/null
(cd "$work/demo" && "$root/bin/wits" import "$root/assets/Tracking.2022.cleaned.xlsx" --commit | tail -1)

"$root/bin/wits-server" --repo "$work/demo" --addr "127.0.0.1:$port" >"$work/server.log" 2>&1 &
server=$!
for _ in $(seq 50); do
  curl -sf "http://127.0.0.1:$port/api/repo" >/dev/null && break
  sleep 0.2
done

# Built inside node_modules, so pdf.js, left out of the bundle, is found from it.
cd "$root/wits-ui"
node_modules/.bin/esbuild scripts/demo-seed.ts --bundle --platform=node --format=esm \
  --external:pdfjs-dist --log-level=warning --outfile=node_modules/.cache/wits/demo-seed.mjs
args=("http://127.0.0.1:$port")
[ -d "$invoices" ] && args+=("$invoices")
node node_modules/.cache/wits/demo-seed.mjs "${args[@]}"

kill "$server"
wait "$server" 2>/dev/null || true
(cd "$work/demo" && "$root/bin/wits" fsck | tail -1 && "$root/bin/wits" bundle --out "$root/assets/demo-seed.wits")
ls -l "$root/assets/demo-seed.wits"

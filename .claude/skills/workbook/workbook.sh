#!/usr/bin/env bash
#
# The workbook, read the way wits reads it.
#
#   workbook.sh compare [old-rev] [new-rev]   do two versions import alike?
#   workbook.sh figures [rev]                 the figures README and ROADMAP pin
#
# Revisions default to HEAD~1 and HEAD; "work" means the file on disk. A
# re-saved workbook reshuffles its XML without changing a cell, so a byte
# diff says nothing — this imports each version into a throwaway repository
# and compares the journals with the timestamps and hashes taken out.

set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

book=assets/Tracking.2022.cleaned.xlsx
scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT
make build >/dev/null
wits="$PWD/bin/wits"

# extract writes one revision of the workbook to $scratch/<name>.xlsx.
extract() {
	if [ "$1" = work ]; then cp "$book" "$scratch/$2.xlsx"; else git show "$1:$book" >"$scratch/$2.xlsx"; fi
}

# import replays a workbook into a fresh repository and prints the verdict.
import() {
	mkdir -p "$scratch/$1"
	(cd "$scratch/$1" && "$wits" init . >/dev/null && "$wits" import "../$1.xlsx" --commit)
}

# events prints a journal's entries without what differs between two imports
# of the same data: when it was recorded, and the hash chain that covers it.
events() {
	python3 -I -c '
import json, sys
for line in open(sys.argv[1]):
    e = json.loads(line)
    for k in ("recorded_at", "hash", "prev"):
        e.pop(k, None)
    print(json.dumps(e, sort_keys=True))
' "$scratch/$1/.wits/journal.ndjson"
}

compare() {
	old="${1:-HEAD~1}"
	new="${2:-HEAD}"
	extract "$old" old
	extract "$new" new
	echo "== $old"
	import old | tail -3
	echo "== $new"
	import new | tail -3
	echo
	if diff <(events old) <(events new) >"$scratch/diff"; then
		echo "identical ledgers: nothing the importer reads has changed"
	else
		echo "the ledgers differ ($(grep -c '^[<>]' "$scratch/diff") lines); first differences:"
		head -40 "$scratch/diff"
	fi
}

figures() {
	extract "${1:-work}" book
	import book | tail -3
	cd "$scratch/book"
	"$wits" bundle --out ../plain.wits >/dev/null 2>&1
	"$wits" bundle --gzip --out ../packed.wits >/dev/null 2>&1
	python3 -I - .wits/journal.ndjson ../plain.wits ../packed.wits <<'PY'
import json, sys
journal, plain, packed = (len(open(p, "rb").read()) for p in sys.argv[1:4])
events = [json.loads(l) for l in open(sys.argv[1])]
products = {e.get("product") for e in events if e.get("product")}
ground = sum(e["grams"] for e in events if e["type"] == "grind")
bought = sum(e["grams"] for e in events if e["type"] == "purchase")
print(f"entries      {len(events)}")
print(f"products     {len(products)}")
print(f"dispensed    {bought:.2f} g")
print(f"ground       {ground:.2f} g")
print(f"last entry   {events[-1]['occurred_at'][:10]}")
print(f"journal      {journal:,} bytes")
print(f"bundle       {plain:,} bytes  {journal / plain:.0f}x")
print(f"bundle gzip  {packed:,} bytes  {journal / packed:.0f}x   (wits bundle --gzip)")
PY
	"$wits" status | head -1
}

case "${1:-compare}" in
compare) compare "${2:-}" "${3:-}" ;;
figures) figures "${2:-}" ;;
*)
	echo "usage: workbook.sh [compare [old] [new] | figures [rev]]"
	exit 2
	;;
esac

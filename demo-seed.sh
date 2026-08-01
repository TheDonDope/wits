#!/usr/bin/env bash
#
# Seeds a throwaway repository for the demo tapes.
#
# The tapes record `wits` reading a .wits directory, and a source checkout has
# none — without this they would only capture the words "not a wits repository".
# It lives in one place so that every clip shows the same shelf.

set -euo pipefail

dir="${1:-/tmp/wits-demo}"
rm -rf "$dir"
mkdir -p "$dir"
cd "$dir"

wits init . >/dev/null
wits device add 'Volcano Hybrid' --kind desktop --min 40 --max 230 --default 185 >/dev/null
wits device add 'Mighty+' --kind portable --min 40 --max 210 --default 180 >/dev/null

# An earlier, finished cycle, so the analysis view has history to scope out to:
# the rhythm heatmap and the cycle comparison only appear past the current one.
wits buy 'Enua 22/1 Wedding Cake' 10g --date 2026-06-09 >/dev/null
wits buy 'Cannamedical 28/1 Lemon Cookie' 10g --date 2026-06-09 >/dev/null

while read -r day verb product amount rest; do
  # shellcheck disable=SC2086 # rest carries optional flags and is meant to split
  wits "$verb" "$product" "$amount" --date "2026-06-$day" $rest >/dev/null
done <<'ENTRIES'
10 grind wcake 0.85
11 grind lcook 1.10
12 grind wcake 0.70
14 grind lcook 1.35
15 grind wcake 0.95
17 grind lcook 0.80
18 grind wcake 1.20
20 grind lcook 1.00
21 grind wcake 0.75
23 grind lcook 1.25
24 grind wcake 0.90
26 grind lcook 1.15
27 grind wcake 1.05
29 grind lcook 0.85
30 grind wcake 0.80
ENTRIES

wits buy 'Enua 22/1 Wedding Cake' 20g --date 2026-07-09 >/dev/null
wits buy 'Cannamedical 28/1 Lemon Cookie' 20g --date 2026-07-09 >/dev/null
wits buy 'Cantourage 25/1 MAC1+' 20g --date 2026-07-09 >/dev/null

# A month of use, so the charts have a shape rather than a spike.
#
# Seeded in date order because that is the order entries are really made in:
# the journal reads back in the order it was written, so grinding out the whole
# month and only then adding the sessions would stack three weeks of sessions
# above grinds that happened before them. That would be an artefact of the
# seeding rather than anything the tool does.
while read -r day verb product amount rest; do
  # shellcheck disable=SC2086 # rest carries optional flags and is meant to split
  wits "$verb" "$product" "$amount" --date "2026-07-$day" $rest >/dev/null
done <<'ENTRIES'
10 grind wcake 0.75
10 sesh  wcake 0.30 --device volcano
11 grind lcook 1.25
12 grind mac1  0.90
13 grind wcake 1.10
14 grind lcook 0.85
14 sesh  lcook 0.35 --device mighty
16 grind mac1  1.40
16 sesh  mac1  0.50 --device volcano --temp 195
17 grind wcake 0.95
18 grind lcook 1.05
20 grind mac1  0.80
21 grind wcake 1.20
21 sesh  wcake 0.45 --device volcano --temp 190
23 grind lcook 0.95
24 grind mac1  1.15
26 grind wcake 0.85
27 grind lcook 1.30
27 sesh  lcook 0.40 --device mighty
29 grind mac1  0.75
30 grind wcake 1.05
ENTRIES

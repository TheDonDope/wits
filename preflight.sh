#!/usr/bin/env bash
#
# Everything the CI and the Codacy gate will say, said here first.
#
# The gate refuses any new issue, and its rules are learned the hard way one
# push at a time: a function past 50 lines or complexity 15, a new .go file
# flagged for a package comment revive cannot see in doc.go. This runs the
# same checks against what changed, before the round trip.
#
#   ./preflight.sh             check the working tree against main
#   ./preflight.sh watch 49    watch a pull request's checks and Codacy delta
#
# BASE overrides what "main" means, for a checkout whose remote is unusual.

set -euo pipefail

repo="TheDonDope/wits"
say() { printf '%s\n' "$*"; }
fail=0

# base resolves the commit the changes are measured against.
base() {
	if [ -n "${BASE:-}" ]; then
		echo "$BASE"
	elif git rev-parse --verify -q origin/main >/dev/null; then
		echo origin/main
	elif git rev-parse --verify -q main >/dev/null; then
		echo main
	else
		echo HEAD
	fi
}

check() {
	b="$(base)"
	say "preflight against $b"
	say ""

	# The same three the workflow runs, in the same spirit.
	if out="$(gofmt -l . 2>/dev/null)" && [ -n "$out" ]; then
		say "✗ gofmt wants a word with:"
		say "$out"
		fail=1
	else
		say "✓ gofmt"
	fi

	if go vet ./... >/dev/null 2>&1; then
		say "✓ go vet"
	else
		say "✗ go vet:"
		go vet ./... 2>&1 | head -20 || true
		fail=1
	fi

	if go test ./... >/dev/null 2>&1; then
		say "✓ go test"
	else
		say "✗ go test:"
		go test ./... 2>&1 | grep -v '^ok' | grep -v 'no test files' | head -20 || true
		fail=1
	fi

	# What changed, staged or not, plus anything untracked.
	changed="$( (git diff --name-only "$b"...HEAD -- '*.go' 2>/dev/null; \
	             git diff --name-only -- '*.go'; \
	             git ls-files --others --exclude-standard -- '*.go') | sort -u)"
	changed="$(echo "$changed" | while read -r f; do [ -f "$f" ] && echo "$f"; done || true)"

	# Codacy's Lizard thresholds: a function past 50 lines of code or a
	# cyclomatic complexity past 15 becomes a new issue the gate refuses.
	if [ -n "$changed" ]; then
		if python3 -c 'import lizard' 2>/dev/null; then
			# shellcheck disable=SC2086 # the file list is meant to split
			warnings="$(python3 -m lizard -T nloc=50 -T cyclomatic_complexity=15 \
				--warnings_only $changed 2>/dev/null | grep 'warning:' || true)"
			if [ -n "$warnings" ]; then
				say "✗ Codacy will flag these functions (nloc > 50 or ccn > 15):"
				say "$warnings"
				fail=1
			else
				say "✓ lizard (changed files inside Codacy's limits)"
			fi
		else
			say "· lizard not installed (pip install lizard); skipping the complexity check"
		fi
	else
		say "· no Go changes to lint"
	fi

	# Codacy runs revive one file at a time, so a brand-new .go file cannot
	# see the package comment in doc.go and gets flagged for lacking one.
	# Code that can live in an existing file dodges the whole conversation.
	fresh="$( (git diff --name-only --diff-filter=A "$b"...HEAD -- '*.go' 2>/dev/null; \
	           git ls-files --others --exclude-standard -- '*.go') | sort -u | grep -v '_test\.go$' || true)"
	if [ -n "$fresh" ]; then
		say "! new non-test .go files — Codacy's revive will ask each for a package comment:"
		while IFS= read -r f; do say "    $f"; done <<<"$fresh"
		say "  prefer growing an existing file, or expect the flag"
	fi

	# Codacy runs shellcheck too — this very script was its first catch.
	scripts="$( (git diff --name-only "$b"...HEAD -- '*.sh' 2>/dev/null; \
	             git diff --name-only -- '*.sh'; \
	             git ls-files --others --exclude-standard -- '*.sh') | sort -u)"
	scripts="$(while IFS= read -r f; do [ -f "$f" ] && echo "$f"; done <<<"$scripts" || true)"
	if [ -n "$scripts" ]; then
		if command -v shellcheck >/dev/null; then
			# shellcheck disable=SC2086 # the file list is meant to split
			if out="$(shellcheck $scripts 2>&1)"; then
				say "✓ shellcheck (changed scripts)"
			else
				say "✗ shellcheck:"
				say "$out" | head -20
				fail=1
			fi
		else
			say "· shellcheck not installed; skipping the script check"
		fi
	fi

	say ""
	if [ "$fail" -eq 0 ]; then
		say "ready to push"
	else
		say "not yet"
	fi
	exit "$fail"
}

# watch polls a pull request until its checks finish, then reads the Codacy
# delta the way the gate does: only Added issues count against it.
watch() {
	pr="$1"
	say "watching $repo #$pr"
	for _ in $(seq 1 60); do
		state="$(gh pr view "$pr" --repo "$repo" \
			--json statusCheckRollup \
			--jq '[.statusCheckRollup[] | select(.status != "COMPLETED")] | length')"
		[ "$state" = "0" ] && break
		sleep 20
	done

	gh pr view "$pr" --repo "$repo" \
		--json mergeStateStatus,statusCheckRollup \
		--jq '.statusCheckRollup[] | "\(.conclusion // .status)\t\(.name)"'

	added="$(curl -s "https://app.codacy.com/api/v3/analysis/organizations/gh/${repo/\//\/repositories\/}/pull-requests/$pr/issues" |
		python3 -c '
import json, sys
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(0)
for i in d.get("data", []):
    if i.get("deltaType") != "Added":
        continue
    c = i["commitIssue"]
    path, line, msg = c.get("filePath"), c.get("lineNumber"), c.get("message")
    print(f"{path}:{line}  {msg}")
')"
	if [ -n "$added" ]; then
		say ""
		say "Codacy will refuse these new issues:"
		say "$added"
		exit 1
	fi
	say ""
	say "no new Codacy issues"
}

case "${1:-check}" in
check) check ;;
watch) watch "${2:?which pull request?}" ;;
*)
	say "usage: ./preflight.sh [check | watch <pr>]"
	exit 2
	;;
esac

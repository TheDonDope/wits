#!/usr/bin/env bash
#
# Everything the CI and the Codacy gate will say, said here first.
#
# The gate refuses any new issue, and its rules are learned the hard way one
# push at a time: a function past 50 lines or complexity 15, a new .go file
# without a package comment of its own (revive cannot see doc.go). This runs the
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

# package_comment says whether a Go file has a package comment revive will
# accept on the file alone: revive's own verdict when it is installed,
# otherwise whether the line above the package clause is a comment and the
# comment says "Package <name> ".
package_comment() {
	local gobin
	gobin="$(go env GOPATH)/bin"
	if command -v revive >/dev/null 2>&1 || [ -x "$gobin/revive" ]; then
		local conf
		conf="$(mktemp)"
		printf '[rule.package-comments]\n' >"$conf"
		local out
		out="$(PATH="$PATH:$gobin" revive -config "$conf" "$1" 2>&1)"
		rm -f "$conf"
		[ -z "$out" ]
		return
	fi
	awk '/^package /{ok = (prev ~ /^\/\//) && seen; exit} /^\/\/ Package [A-Za-z0-9_]+ /{seen=1} {prev=$0} END{exit !ok}' "$1"
}

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

# lizard_warnings prints lizard's complaints about one file against Codacy's
# thresholds: past 50 lines of code, or a cyclomatic complexity past 15.
lizard_warnings() {
	"$1" -m lizard -T nloc=50 -T cyclomatic_complexity=15 --warnings_only "$2" 2>/dev/null |
		grep 'warning:' || true
}

# function_names reduces lizard warnings to the names of the functions flagged.
function_names() { sed -n 's/.*warning: \([^ ]*\) has .*/\1/p'; }

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
		# ./doctor.sh install puts lizard in a venv of its own, since the
		# system python may have no pip to install it with.
		py="${WITS_TOOLS:-$HOME/.cache/wits-dev}/venv/bin/python"
		[ -x "$py" ] || py=python3
		if "$py" -c 'import lizard' 2>/dev/null; then
			# Codacy refuses only what is new, so a function already over the
			# limits on the base is reported, not failed: touching a file must
			# not make its old debts look like yours.
			fresh_warnings="" old=0
			while IFS= read -r f; do
				now="$(lizard_warnings "$py" "$f")"
				[ -n "$now" ] || continue
				before=""
				if git cat-file -e "$b:$f" 2>/dev/null; then
					tmp="$(mktemp --suffix=".${f##*.}")"
					git show "$b:$f" >"$tmp"
					before="$(lizard_warnings "$py" "$tmp" | function_names)"
					rm -f "$tmp"
				fi
				while IFS= read -r w; do
					if grep -qxF "$(function_names <<<"$w")" <<<"$before"; then
						old=$((old + 1))
					else
						fresh_warnings+="$w"$'\n'
					fi
				done <<<"$now"
			done <<<"$changed"
			if [ -n "$fresh_warnings" ]; then
				say "✗ Codacy will flag these functions (nloc > 50 or ccn > 15):"
				say "${fresh_warnings%$'\n'}"
				fail=1
			else
				say "✓ lizard (nothing new past Codacy's limits)"
			fi
			if [ "$old" -gt 0 ]; then
				say "  $old function(s) in the changed files were already past them on $b"
			fi
		else
			say "· lizard not installed (./doctor.sh install); skipping the complexity check"
		fi
	else
		say "· no Go changes to lint"
	fi

	# Codacy runs revive one file at a time, so a brand-new .go file cannot
	# see the package comment in doc.go: it needs one of its own, attached to
	# the package clause and of the form "// Package <name> ...". A comment
	# with a blank line before the clause, or "Package name:", is not one —
	# fourteen files of #133 learned that. revive itself says, when it is
	# installed; otherwise the line above the clause is read.
	fresh="$( (git diff --name-only --diff-filter=A "$b"...HEAD -- '*.go' 2>/dev/null; \
	           git ls-files --others --exclude-standard -- '*.go') | sort -u | grep -v '_test\.go$' || true)"
	bare=""
	while IFS= read -r f; do
		if [ -n "$f" ] && [ -f "$f" ] && ! package_comment "$f"; then
			bare="$bare$f"$'\n'
		fi
	done <<<"$fresh"
	if [ -n "$bare" ]; then
		say "✗ new .go files without a package comment revive can see — Codacy will flag them:"
		while IFS= read -r f; do [ -n "$f" ] && say "    $f"; done <<<"$bare"
		say "  put \"// Package <name> ...\" directly above the package clause"
		fail=1
	elif [ -n "$fresh" ]; then
		say "✓ new .go files carry their own package comment"
	fi

	# The browser client is linted with its types, which Codacy cannot see —
	# the wits-ui workflow and this are where its lint happens. It needs the
	# client's dependencies; without them there is nothing to lint against.
	if [ -n "$( (git diff --name-only "$b"...HEAD -- wits-ui 2>/dev/null; git status --porcelain -- wits-ui) | head -1)" ]; then
		if [ -d wits-ui/node_modules ]; then
			if out="$(cd wits-ui && npx ng lint 2>&1 && npm run --silent lint:styles 2>&1 && npm run --silent typecheck 2>&1)"; then
				say "✓ ng lint, stylelint and the models held to the API (wits-ui, with types)"
			else
				say "✗ ng lint:"
				say "$out" | grep -E 'error|warning' | head -20
				fail=1
			fi
		else
			say "· wits-ui changed but its packages are not installed (cd wits-ui && npm ci); skipping its lint"
		fi
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
			say "· shellcheck not installed (./doctor.sh install); skipping the script check"
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

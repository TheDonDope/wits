---
name: ship
description: Take a change in wits from the working tree to a published GitHub release — branch, commit in the repo's voice, preflight, PR, watch the checks, rebase-merge, make release, and gh release notes in the house style. Use when asked to "open a PR", "push it", "merge it", "cut the release", "ship it", or any step of that loop.
---

# Ship

The loop is **branch → commit → preflight → PR → green → rebase-merge →
release**. Nothing merges red. The maintainer usually asks for it a step at a
time ("open the PR" … "merge it and cut the release"); do the step asked, report,
and offer the next one. Merging and releasing each need their own go-ahead.

Run `./doctor.sh` first if anything below says "command not found".

## 1. Branch and commit

Never commit to main. Branch names: `fix/…`, `feat/…`, `chore/…`, `docs/…`,
`test/…`, short and specific (`fix/excelize-advisories`).

Read `git log --format='%s%n%b' -15` before writing a message. The voice:
conventional prefix with scope, lowercase, declarative, a little wry — the
subject says what is now true, not what was done.

- `test(importer): the records add up, so say so`
- `fix(deps): take excelize from master, where its fifteen advisories are fixed`
- `feat(ledger)!: cycles close on empty, and every gram knows its fill`

The body says *why*, with the figures that were verified. End with the
`Co-Authored-By` trailer the session's attribution reminder gives.

## 2. Preflight and PR

```sh
./preflight.sh                      # must end in "ready to push"
git push -u origin <branch>
gh pr create --title "<the commit subject>" --body-file <file>
```

The PR body: what and why in a paragraph, then a short "verified" list (tests,
the real workbook import, preflight), then the `🤖 Generated with` line from the
attribution reminder.

## 3. Watch

```sh
./preflight.sh watch <pr>
gh pr checks <pr> --watch --interval 15   # if `build` was still pending
```

`preflight.sh watch` can finish while `build` is still running and print it
with an empty status — that is not a pass. Confirm with `gh pr checks`.

## 4. Merge

```sh
gh pr merge <pr> --rebase --delete-branch
git checkout main && git pull && git remote prune origin
```

Rebase-merge rewrites the SHAs. Quote the new ones from `git log` on main,
never the branch's.

## 5. Release

Every merged PR so far has shipped as its own **minor** release. The exception
is a docs-only change (a diary entry), which waits for the next release.

```sh
git tag --sort=-v:refname | head -1      # vX.Y.0 → next is vX.(Y+1).0
make release VERSION=vX.Y.0              # changelog commit, tag, push main + tags
```

Run it from an up-to-date main. If it stops at `git-chglog: No such file`,
nothing was committed or tagged yet: `./doctor.sh install`, then rerun.

Then the GitHub release. Read the last two first (`gh release view vX.Y.0
--json name,body`) and match them:

- **Title**: `vX.Y.0 — <a short, lowercase, poetic phrase>` — "a graph per
  cycle", "the records add up", "fifteen ways to break a reader".
- **Lede**: one or two sentences saying what changed for the person using wits.
- **Bullets** that lead with a **bold** clause, each a fact that was checked.
- A fenced block of **real** output when there is something to show (witsnap,
  `wits status`, an import verdict) — never invented.
- When a GIF changed, embed it from
  `https://raw.githubusercontent.com/TheDonDope/wits/main/assets/<name>.gif`.
- Last line: the `🤖 Generated with` line.

Write the notes to a file in the scratchpad and pass `--notes-file`; heredocs
inside `$(…)` mangle backticks.

```sh
gh release create vX.Y.0 --title "vX.Y.0 — …" --notes-file <file>
```

## 6. Afterwards

- Dependency fixes: GitHub re-scans a minute or more after the push. Confirm the
  alerts closed with
  `gh api 'repos/TheDonDope/wits/dependabot/alerts?state=open' --jq length`.
- A significant day ends with a diary entry — the `diary` skill.

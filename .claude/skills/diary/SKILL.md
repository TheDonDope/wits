---
name: diary
description: Write the day's DEVDIARY.md entry for wits in the house style — a titled narrative of the day's work, the day's commits, and notes. Use when asked for a diary / devdiary entry, or at the end of a day that merged PRs or cut releases.
---

# Diary

`DEVDIARY.md` is a per-day narrative, newest first. `dev-diary.sh` is the
interactive version and needs a terminal (gum prompts); write the entry directly
instead, in the same shape.

```markdown
## YYYY-MM-DD

### <a title — a phrase, not a summary: "The records add up">

<One paragraph, 4–8 sentences: what the day did and why, in the voice of the
commit messages. Name the figures that were verified and the releases shipped.>

- `<sha>` <subject>
- …

### Notes for YYYY-MM-DD

<What a future reader needs that the commits don't say: tooling found missing,
a workaround, something left open.>
```

## The commit list

The commits the script would list, oldest first, **from main**:

```sh
git checkout main && git pull
git log --since=midnight --reverse --format='- `%h` %s'
```

Rebase-merging rewrites SHAs, so list them only after merging — the branch's
SHAs point to nothing on main. `--since` reads the commit date, so a Dependabot
bump merged today counts even if it was written last week. Leave those in; the
script would too.

## Where it goes

Directly below `# Development Diary`. If today already has an entry, extend it
rather than adding a second `## <today>`.

Ship it as `docs(diary): <the entry's title, lowercased>` on a `docs/devdiary-<date>`
branch. A diary-only PR gets merged but **not released**.

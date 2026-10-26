# Contributing to wits

## Found a bug, or want something?

Wits doesn't use GitHub Issues. Features, bugs and refactorings live in
[ROADMAP.md](./ROADMAP.md), so the plan and the code stay in one place.

- **Check the roadmap first** — it may already be known, with the reasoning
  behind it.
- If it isn't there, open a pull request that adds it to the roadmap, or one
  that fixes it. Say what you expected, what happened instead, and enough to
  reproduce it.

## Sending a patch

```sh
./doctor.sh      # the tools; only Go is needed to build and test
make test
./preflight.sh   # what CI and the Codacy gate will say, said first
```

- Branch from `main` and open a pull request; the template asks what changed,
  why, and how it was verified.
- Commit messages use conventional prefixes in the repository's own voice —
  read `git log` for a few before writing one.
- If behaviour changes, update README.md (or the doc in `docs/` it touches),
  ROADMAP.md and, for a page of the browser interface, its section of the user
  guide (`wits-ui/src/app/data/guide.ts`) in the same pull request. Both are claims checked against the code, not aspirations.
- If the change is visible on screen, re-record the GIFs it affects
  (`./render-vhs-tapes.sh <tape>`).

Pull requests are rebase-merged once the checks are green, and each one ships
as its own release.

Wits is a volunteer effort. Thank you for pitching in! :heart:

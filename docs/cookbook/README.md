# The developer cookbook

Recipes for working **on** wits rather than with it. The
[README](../../README.md), [docs/terminal.md](../terminal.md) and
[docs/browser.md](../browser.md) are for people using wits. These pages are
for people who want to change it.

Each chapter explains one part of the codebase, then gives worked examples
you can copy, run and change.

| Chapter | What it covers | Status |
| --- | --- | --- |
| [The command line](cli.md) | Go as wits uses it, how the command line is wired, Cobra, the two grammars, and recipes for new commands | ✅ |
| [Plugins](plugins.md) | A command of your own without touching wits: `wits-<name>` on the PATH, the contract, a plugin in shell and in Go, and the three that come with wits | ✅ |
| The browser | Angular, signals, the API's shapes, the phone's engine | planned |
| The terminal interface | Bubble Tea v2, screens, modals, witsnap | planned |
| The ledger | the journal, the fold, the three scopes | planned |

> [!TIP]
> New to the code? Read [The command line](cli.md) first. A command is the
> shortest path from your keyboard to the journal, so it shows every layer of
> wits on the way.

## Before you start

```sh
./doctor.sh            # what is installed; ./doctor.sh install fetches the rest
make build             # ./bin/wits
make test              # every Go test, with the race detector
./preflight.sh         # what CI and Codacy will say, before you push
```

Building and testing need only Go. [CLAUDE.md](../../CLAUDE.md) has the
house rules that CI enforces, and [ROADMAP.md](../../ROADMAP.md) has what is
built and what is planned.

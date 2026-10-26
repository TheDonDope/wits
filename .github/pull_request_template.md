# Pull request

## What and why

Describe the change and the reason for it. If it fixes or implements something
from [ROADMAP.md](../ROADMAP.md), say which entry.

## How it was verified

`./preflight.sh` should say "ready to push". If the change is visible in the
interface, say what you looked at (`witsnap screens` renders any screen as
text). If it changes what a demo tape shows, re-record it with
`./render-vhs-tapes.sh <tape>`.

## Checklist

- [ ] Tests cover the change
- [ ] `./preflight.sh` passes
- [ ] README.md, docs/, the user guide and ROADMAP.md still tell the truth

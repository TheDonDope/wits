# Pull request

## What and why

Describe the change and the reason for it. If it fixes or implements something
from [ROADMAP.md](../ROADMAP.md), say which entry.

## How it was verified

`make test` and `make vet` should pass. If the change is visible in the
interface, say what you looked at; if it changes what the demo tapes show,
re-record them with `make render-tapes`.

## Checklist

- [ ] Tests cover the change
- [ ] `make test` and `make vet` pass
- [ ] Documentation (README, ROADMAP) still tells the truth

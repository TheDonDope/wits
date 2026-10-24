---
name: tapes
description: Re-record the demo GIFs in wits/assets from the *.tape files with vhs, and verify them by looking at the frames. Use when a change is visible on screen, when a figure a tape shows has changed, or when asked to re-record, render or update a GIF or tape.
---

# Tapes

Each `assets/*.gif` is recorded from a `*.tape` at the repository root and must
show what the README beside it says.

| tape | shows | depends on |
| --- | --- | --- |
| `wits-tour.tape` | the interface, screen by screen | every screen; `demo-seed.sh` |
| `wits-cli.tape` | recording a fill and a session | `buy`, `grind`, `sesh`, `status` output |
| `wits-import.tape` | importing the workbook | the importer's report |
| `wits-reconcile.tape` | weighing against the scale | `reconcile`; `demo-seed.sh` |
| `wits-bundle.tape` | bundle and restore | the workbook's byte counts |

## Render

```sh
./doctor.sh                                  # vhs, ttyd, ffmpeg, Hack, Chromium libraries
./render-vhs-tapes.sh wits-bundle.tape       # name the tapes; no argument renders all
```

The script rebuilds `./bin/wits` first (the tapes run it, not the installed
`wits`), loads the Chromium libraries `./doctor.sh env` points at, renders **one
tape at a time**, clears the tapes' `/tmp/wits-*` folders, and fails if the GIF
was not rewritten.

**Run nothing heavy alongside a render.** Each one starts a Chromium, and this
machine — a Steam Deck with ~3 GB free on a good day — has been OOM-killed by
parallel renders. Don't render tapes in the background while building or
testing.

## Verify by looking

A render that exits 0 can still show the wrong thing. Extract the last frame
and read it:

```sh
ffmpeg -loglevel error -y -sseof -0.5 -i assets/wits-bundle.gif -frames:v 1 -update 1 <scratchpad>/last.png
```

Then open the PNG with Read and check every figure on screen against README.md.
For the tour, pull a few frames from the middle as well (`-ss <seconds>`).

## Don't commit a render that changed nothing

Re-rendering an unchanged tape still rewrites the GIF (timestamps, `ls -l`
dates). If you rendered only to test the tooling, restore it with
`git checkout -- assets/<name>.gif`.

Commit as `chore(vhs): re-record <the gif> <because>`.

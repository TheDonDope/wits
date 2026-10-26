---
name: workbook
description: Handle a new or re-saved tracking workbook (assets/Tracking.2022.cleaned.xlsx) in wits — tell whether anything the importer reads changed, reconcile the pinned test figures against the raw cells, and update every place that repeats them. Use when the maintainer says they committed, updated or exported new data to the workbook / seed sheet / spreadsheet.
---

# The workbook

`assets/Tracking.2022.cleaned.xlsx` is the real record the importer is tested
against. The maintainer keeps it current and commits it; your job is to say
what changed and keep every figure that depends on it true.

## 1. Did anything change?

A workbook re-saved in a spreadsheet app reshuffles its XML (shared strings,
style ids) without touching a cell, so the byte diff is noise. Ask the importer:

```sh
.claude/skills/workbook/workbook.sh compare            # HEAD~1 against HEAD
.claude/skills/workbook/workbook.sh compare <rev> work # a revision against the file on disk
```

"identical ledgers" means nothing wits reads has changed: say so, say where the
data ends (`figures` prints the last entry), and stop. Do not invent work.

## 2. When the data did change

1. **Read the cells without the importer.** The tests' figures must be
   reconciled independently, or they only agree with the code:

   ```sh
   ~/.cache/wits-dev/venv/bin/python -I .claude/skills/workbook/cells.py
   ```

   (`./doctor.sh install` provides the venv.) It prints sheets, fills, grinds,
   grams dispensed and ground.

2. **Read them through wits** and compare:

   ```sh
   .claude/skills/workbook/workbook.sh figures
   go test ./pkg/importer -run TestImportsTheRealWorkbook -v
   ```

   If the two readings disagree, that is a finding. Investigate before changing
   any assertion.

3. **Update every place that repeats a figure.** Find them all:

   ```sh
   grep -rn -E '<old entries>|<old journal bytes>|<old grams ground>' \
     --include=*.go --include=*.md --include=*.tape . | grep -v -e CHANGELOG -e DEVDIARY
   ```

   As of v0.28.0 that is `pkg/importer/integration_test.go` (sheets, products,
   fills, grinds, grams, entries), `docs/terminal.md` (grams ground, the bundle table),
   `ROADMAP.md` (the bundle figures, the cached-fold entry count) and the
   comment in `wits-bundle.tape`. The bundle figures come from `figures`:
   journal, bundle, and `wits bundle --gzip`, with ratios rounded to whole ×.

4. **Importer findings.** If `wits import` now reports problems, or stops
   reporting ones ROADMAP lists under "Findings in the imported records",
   update that section. The maintainer fixes records at the source.

5. **Re-record `wits-bundle.tape`**, since it shows the byte counts on screen —
   the `tapes` skill.

6. Ship it: `test(importer): reconcile against the workbook through <month year>`
   — the `ship` skill.

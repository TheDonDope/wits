"""
The workbook read from its cells, without the importer.

The importer's tests pin figures that must be reconciled independently of
it — this is that independent reading. Run with the doctor's venv:

    ~/.cache/wits-dev/venv/bin/python -I .claude/skills/workbook/cells.py [file]

Each sheet is one fill: product rows (a name, then grams dispensed) above a
row headed Date/Datum, and below it one row per grinding, the grams in C.
"""
import sys
from collections import Counter

import openpyxl

path = sys.argv[1] if len(sys.argv) > 1 else "assets/Tracking.2022.cleaned.xlsx"
wb = openpyxl.load_workbook(path, data_only=True)

fills = grinds = 0
dispensed = ground = 0.0
names = Counter()
for ws in wb.worksheets:
    rows = list(ws.iter_rows(values_only=True))
    head = next((i for i, r in enumerate(rows) if r and r[0] in ("Date", "Datum")), None)
    if head is None:
        continue
    for r in rows[:head]:
        if isinstance(r[0], str) and isinstance(r[1], (int, float)):
            fills += 1
            dispensed += r[1]
            names[r[0]] += 1
    for r in rows[head + 1:]:
        if len(r) > 2 and isinstance(r[2], (int, float)) and r[2] > 0:
            grinds += 1
            ground += r[2]

print(f"sheets       {len(wb.worksheets)}")
print(f"fills        {fills}")
print(f"grinds       {grinds}")
print(f"dispensed    {dispensed:.2f} g")
print(f"ground       {ground:.2f} g")
print(f"names        {len(names)} as written (the importer merges spellings, so its product count may be lower)")

# The whole repository as one plain-text file, restoring byte for byte.

Output assets/wits-bundle.gif

Require wits

Set Shell "bash"
Set FontSize 18
Set FontFamily "Hack"
Set Width 1400
Set Height 380
Set Padding 20
Set TypingSpeed 35ms

Hide
Type "export PATH=$PWD/bin:$PATH" Enter
Type "rm -rf /tmp/wits-bun && mkdir -p /tmp/wits-bun" Enter
Type "cp assets/Tracking.2022.cleaned.xlsx /tmp/wits-bun/Tracking.xlsx" Enter
Type "cd /tmp/wits-bun && wits init . >/dev/null" Enter
Type "wits import Tracking.xlsx --commit >/dev/null" Enter
Type "clear" Enter
Show

# Nearly three years of history, 1369 entries.
Type "wits bundle --out history.wits" Sleep 400ms Enter
Sleep 2s
Type "ls -l .wits/journal.ndjson history.wits" Sleep 400ms Enter
Sleep 4s

# Restore it into an empty repository and compare the journals.
Type "mkdir /tmp/wits-copy && cd /tmp/wits-copy && wits init . >/dev/null" Sleep 300ms Enter
Sleep 1500ms
Type "wits restore /tmp/wits-bun/history.wits" Sleep 400ms Enter
Sleep 3s

# Identical, hash chain included.
Type "diff .wits/journal.ndjson /tmp/wits-bun/.wits/journal.ndjson && echo 'identical, hash chain included'" Sleep 400ms Enter
Sleep 4s

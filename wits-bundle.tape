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
Type "rm -rf /tmp/wits-bun /tmp/wits-copy && mkdir -p /tmp/wits-bun" Enter
Type "cp assets/Tracking.2022.cleaned.xlsx /tmp/wits-bun/Tracking.xlsx" Enter
Type "cd /tmp/wits-bun && wits init . >/dev/null" Enter
Type "wits import Tracking.xlsx --commit >/dev/null" Enter
Type "clear" Enter
Show

# Three years of history, 1429 entries.
Type "wits bundle create history.wits" Sleep 400ms Enter
Sleep 2s
Type "ls -l .wits/journal.ndjson history.wits" Sleep 400ms Enter
Sleep 4s

# Clone it into a new repository and compare the journals.
Type "wits clone history.wits /tmp/wits-copy" Sleep 400ms Enter
Sleep 3s
Type "cd /tmp/wits-copy" Sleep 300ms Enter
Sleep 1s

# Identical, hash chain included.
Type "diff .wits/journal.ndjson /tmp/wits-bun/.wits/journal.ndjson && echo 'identical, hash chain included'" Sleep 400ms Enter
Sleep 4s

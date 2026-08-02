# When the ledger and the scale disagree, the difference is recorded rather
# than the past edited.

Output assets/wits-reconcile.gif

Require wits

Set Shell "bash"
Set FontSize 18
Set FontFamily "Hack"
Set Width 1400
Set Height 720
Set Padding 20
Set TypingSpeed 35ms

Hide
Type "export PATH=$PWD/bin:$PATH" Enter
Type "./demo-seed.sh /tmp/wits-rec && cd /tmp/wits-rec" Enter
Sleep 3s
Type "clear" Enter
Show

# The ledger's opinion.
Type "wits status" Sleep 400ms Enter
Sleep 3s

# One jar against the scale, account first. Nothing is written by a dry run.
Type "wits reconcile storage wcake 13.40 --dry-run" Sleep 400ms Enter
Sleep 3500ms

# Record the difference as an adjustment, which is a transfer like any other.
Type "wits reconcile storage wcake 13.40 --reason 'spilled on the desk'" Sleep 400ms Enter
Sleep 3s
Hide
Type "clear" Enter
Show

# Weighing day: the interactive form walks every stash past the scale.
Type "wits reconcile stash" Sleep 400ms Enter
Sleep 2500ms
# Every jar is ticked; take them all.
Enter
Sleep 1500ms
# lcook reads a little light.
Type "11.90" Sleep 600ms Enter
Sleep 1200ms
# mac1 stays on the shelf.
Enter
Sleep 1200ms
# wcake matches the ledger.
Type "12.35" Sleep 600ms Enter
Sleep 3500ms

# It all stands in the log beside everything else.
Type "wits log --oneline -n 4" Sleep 400ms Enter
Sleep 4s

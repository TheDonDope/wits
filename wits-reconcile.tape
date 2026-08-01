# When the ledger and the scale disagree, the difference is recorded rather
# than the past edited.

Output assets/wits-reconcile.gif

Require wits

Set Shell "bash"
Set FontSize 18
Set FontFamily "Hack"
Set Width 1400
Set Height 590
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

# The scale's. Nothing is written by a dry run.
Type "wits reconcile wcake 13.40 --dry-run" Sleep 400ms Enter
Sleep 4s

# Record the difference as an adjustment, which is a transfer like any other.
Type "wits reconcile wcake 13.40 --reason 'spilled on the desk'" Sleep 400ms Enter
Sleep 3s

# It stands in the log beside everything else.
Type "wits log --oneline -n 3" Sleep 400ms Enter
Sleep 4s

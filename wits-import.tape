# Four years of a tracking spreadsheet, read before anything is written.

Output assets/wits-import.gif

Require wits

Set Shell "bash"
Set FontSize 17
Set FontFamily "Hack"
Set Width 1500
Set Height 760
Set Padding 20
Set TypingSpeed 35ms

Hide
Type "export PATH=$PWD/bin:$PATH" Enter
Type "rm -rf /tmp/wits-imp && mkdir -p /tmp/wits-imp" Enter
Type "cp assets/Tracking.2022.cleaned.xlsx /tmp/wits-imp/Tracking.xlsx" Enter
Type "cd /tmp/wits-imp && wits init . >/dev/null" Enter
Type "clear" Enter
Show

# The default is a dry run: what would be imported, and what does not add up.
Type "wits import Tracking.xlsx" Sleep 400ms Enter
Sleep 9s

Type "clear" Sleep 300ms Enter

# Only --commit writes. A repository that already holds entries is refused.
Type "wits import Tracking.xlsx --commit" Sleep 400ms Enter
Sleep 5s
Type "wits status" Sleep 400ms Enter
Sleep 5s

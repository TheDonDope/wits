# The interface: five screens, read from the ledger and nothing else.
#
# Seeded by demo-seed.sh, because `wits` reads a .wits directory and a source
# checkout has none — without it this would record the words "not a wits
# repository". Render with `make render-tapes`.

Output assets/wits-tour.gif

Require wits

Set Shell "bash"
Set FontSize 18
# Hack is a plain terminal font that is widely installed. A font that is absent
# falls back to one whose metrics do not match the cell width, which renders
# every character with a gap beside it.
Set FontFamily "Hack"
Set Width 1500
Set Height 760
Set Padding 20
Set TypingSpeed 35ms

Hide
Type "export PATH=$PWD/bin:$PATH" Enter
Type "./demo-seed.sh /tmp/wits-tour && cd /tmp/wits-tour" Enter
Sleep 3s
Type "clear" Enter
Show

Type "wits" Sleep 400ms Enter
# Dashboard: what is left, and how long it lasts.
Sleep 4s
# Journal.
Tab
Sleep 3500ms
# Analysis.
Tab
Sleep 4s
# Products.
Tab
Sleep 3s
Down
Sleep 2s
Down
Sleep 2500ms
# Devices.
Tab
Sleep 3500ms
Type "q"
Sleep 1s

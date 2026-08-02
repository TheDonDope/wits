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
# Tall enough for the year-scoped analysis: the rhythm calendar is seven rows
# plus its legend, and the analysis screen clips rather than scrolls.
Set Height 900
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
# Analysis: the cycle first, then scoped out to the year, where the rhythm
# heatmap and the cycle comparison appear.
Tab
Sleep 4s
Type "s"
Sleep 4s
# Storage: the shelf above, the history of finished jars below.
Tab
Sleep 3s
Down
Sleep 2s
# Tick two jars; r would weigh them together.
Space
Sleep 1200ms
Down
Space
Sleep 2500ms
# Stash: the stashes holding ground product, and the ones already consumed.
Tab
Sleep 3s
# Sessions: what came out of the stash, through which device, how hot.
Tab
Sleep 3500ms
# Devices.
Tab
Sleep 3500ms
Type "q"
Sleep 1s

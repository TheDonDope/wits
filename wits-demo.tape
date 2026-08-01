# A tour of Wits: what is left, and the four screens.
#
# A throwaway repository is set up first. `wits` reads a .wits directory and a
# source checkout has none, so without this the recording would only capture the
# words "not a wits repository".
#
# Render with `make render-tapes`, which needs vhs, gum and ttyd (`make install`
# covers the first two).

Output assets/wits-demo.gif

Require wits

Set Shell "bash"
Set FontSize 20
# Hack is a plain terminal font that is widely installed. A font that is not
# present falls back to one whose metrics do not match the cell width, which
# renders every character with a gap beside it.
Set FontFamily "Hack"
Set Width 1500
Set Height 800
Set Padding 20
Set TypingSpeed 35ms

Hide
Type "export PATH=$PWD/bin:$PATH" Enter
Type "rm -rf /tmp/wits-demo && mkdir -p /tmp/wits-demo && cd /tmp/wits-demo" Enter
Type "wits init . >/dev/null" Enter
Type "wits device add 'Volcano Hybrid' --kind desktop --min 40 --max 230 --default 185 >/dev/null" Enter
Type "wits buy 'Enua 22/1 Wedding Cake' 20g --date 2026-07-09 >/dev/null" Enter
Type "wits buy 'Cannamedical 28/1 Lemon Cookie' 20g --date 2026-07-09 >/dev/null" Enter
Type "wits buy 'Cantourage 25/1 MAC1+' 20g --date 2026-07-09 >/dev/null" Enter
Type "wits grind wedding 0.75 --date 2026-07-10 >/dev/null" Enter
Type "wits sesh wedding 0.30 --device volcano --date 2026-07-10 >/dev/null" Enter
Type "wits grind lemon 1.25 --date 2026-07-11 >/dev/null" Enter
Type "wits grind mac1 0.90 --date 2026-07-12 >/dev/null" Enter
Type "wits grind wedding 1.10 --date 2026-07-13 >/dev/null" Enter
Type "wits grind lemon 0.85 --date 2026-07-14 >/dev/null" Enter
Type "wits grind mac1 1.40 --date 2026-07-15 >/dev/null" Enter
Type "wits sesh mac1 0.50 --device volcano --temp 195 --date 2026-07-15 >/dev/null" Enter
Type "wits grind wedding 0.95 --date 2026-07-16 >/dev/null" Enter
Type "clear" Enter
Show

# Record the day's grind.
Type "wits grind lemon 1.10" Sleep 400ms Enter
Sleep 2s

# What is left, and how long it will last.
Type "wits status" Sleep 400ms Enter
Sleep 4500ms

Type "clear" Sleep 300ms Enter

# The interface: dashboard, journal, analysis, devices.
Type "wits" Sleep 400ms Enter
Sleep 3500ms
Tab
Sleep 3500ms
Tab
Sleep 3500ms
Tab
Sleep 3500ms
Type "q"
Sleep 1s

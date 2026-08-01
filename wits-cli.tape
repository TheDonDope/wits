# The command line: a repository, a fill, a grind, a session, and what is left.

Output assets/wits-cli.gif

Require wits

Set Shell "bash"
Set FontSize 18
Set FontFamily "Hack"
Set Width 1400
Set Height 470
Set Padding 20
Set TypingSpeed 35ms

Hide
Type "export PATH=$PWD/bin:$PATH" Enter
Type "rm -rf /tmp/wits-cli && mkdir -p /tmp/wits-cli && cd /tmp/wits-cli" Enter
Type "clear" Enter
Show

# A repository is made the way a git one is.
Type "wits init ." Sleep 400ms Enter
Sleep 2s

# A prescription fill. The handle is generated and reported back.
Type "wits buy 'Enua 22/1 Wedding Cake' 20g" Sleep 400ms Enter
Sleep 2500ms
Type "wits buy 'Cantourage 25/1 MAC1+' 20g" Sleep 400ms Enter
Sleep 2500ms

# Register the vaporizer, so a session can name it.
Type "wits device add 'Volcano Hybrid' --kind desktop --max 230 --default 185" Sleep 400ms Enter
Sleep 2500ms

# Grind into the tin, then a session out of it.
Type "wits grind wcake 0.75" Sleep 400ms Enter
Sleep 2s
Type "wits sesh wcake 0.30 --device volcano --temp 185" Sleep 400ms Enter
Sleep 2500ms

Type "clear" Sleep 300ms Enter

# What is left, and how long it will last.
Type "wits status" Sleep 400ms Enter
Sleep 5s

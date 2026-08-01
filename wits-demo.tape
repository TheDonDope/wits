# VHS documentation
#
# Output:
#   Output <path>.gif               Create a GIF output at the given <path>
#   Output <path>.mp4               Create an MP4 output at the given <path>
#   Output <path>.webm              Create a WebM output at the given <path>
#
# Require:
#   Require <string>                Ensure a program is on the $PATH to proceed
#
# Settings:
#   Set FontSize <number>           Set the font size of the terminal
#   Set FontFamily <string>         Set the font family of the terminal
#   Set Height <number>             Set the height of the terminal
#   Set Width <number>              Set the width of the terminal
#   Set LetterSpacing <float>       Set the font letter spacing (tracking)
#   Set LineHeight <float>          Set the font line height
#   Set LoopOffset <float>%         Set the starting frame offset for the GIF loop
#   Set Theme <json|string>         Set the theme of the terminal
#   Set Padding <number>            Set the padding of the terminal
#   Set Framerate <number>          Set the framerate of the recording
#   Set PlaybackSpeed <float>       Set the playback speed of the recording
#   Set MarginFill <file|#000000>   Set the file or color the margin will be filled with.
#   Set Margin <number>             Set the size of the margin. Has no effect if MarginFill isn't set.
#   Set BorderRadius <number>       Set terminal border radius, in pixels.
#   Set WindowBar <string>          Set window bar type. (one of: Rings, RingsRight, Colorful, ColorfulRight)
#   Set WindowBarSize <number>      Set window bar size, in pixels. Default is 40.
#   Set TypingSpeed <time>          Set the typing speed of the terminal. Default is 50ms.
#
# Sleep:
#   Sleep <time>                    Sleep for a set amount of <time> in seconds
#
# Type:
#   Type[@<time>] "<characters>"    Type <characters> into the terminal with a
#                                   <time> delay between each character
#
# Keys:
#   Escape[@<time>] [number]        Press the Escape key
#   Backspace[@<time>] [number]     Press the Backspace key
#   Delete[@<time>] [number]        Press the Delete key
#   Insert[@<time>] [number]        Press the Insert key
#   Down[@<time>] [number]          Press the Down key
#   Enter[@<time>] [number]         Press the Enter key
#   Space[@<time>] [number]         Press the Space key
#   Tab[@<time>] [number]           Press the Tab key
#   Left[@<time>] [number]          Press the Left Arrow key
#   Right[@<time>] [number]         Press the Right Arrow key
#   Up[@<time>] [number]            Press the Up Arrow key
#   Down[@<time>] [number]          Press the Down Arrow key
#   PageUp[@<time>] [number]        Press the Page Up key
#   PageDown[@<time>] [number]      Press the Page Down key
#   Ctrl+<key>                      Press the Control key + <key> (e.g. Ctrl+C)
#
# Display:
#   Hide                            Hide the subsequent commands from the output
#   Show                            Show the subsequent commands in the output

Output vhs-output/wits-demo.gif

Set Shell "bash"
Set FontSize 24
Set FontFamily "JetBrainsMono Nerd Font"
Set Width 1400
Set Height 900

# A throwaway repository first. `wits` reads a .wits directory and a source
# checkout has none, so without this the tape would only record the words
# "not a wits repository".
Hide
Type "export PATH=$PWD/bin:$PATH" Enter
Type "rm -rf /tmp/wits-demo && mkdir -p /tmp/wits-demo && cd /tmp/wits-demo" Enter
Type "wits init ." Enter
Type "wits device add 'Volcano Hybrid' --kind desktop --min 40 --max 230 --default 185" Enter
Type "wits buy 'Enua 22/1 Wedding Cake' 20g --date 2026-07-09" Enter
Type "wits buy 'Cannamedical 28/1 Lemon Cookie' 20g --date 2026-07-09" Enter
Type "wits buy 'Cantourage 25/1 MAC1+' 20g --date 2026-07-09" Enter
Type "wits grind wedding 0.75 --date 2026-07-10" Enter
Type "wits sesh wedding 0.30 --device volcano --date 2026-07-10" Enter
Type "wits grind lemon 1.25 --date 2026-07-11" Enter
Type "wits grind mac1 0.90 --date 2026-07-12" Enter
Type "wits grind wedding 1.10 --date 2026-07-13" Enter
Type "wits grind lemon 0.85 --date 2026-07-14" Enter
Type "clear" Enter
Show

# What is left, and how long it will last.
Type "wits status" Sleep 500ms Enter
Sleep 4s
Type "clear" Sleep 300ms Enter

# The interface: dashboard, journal, analysis, devices.
Type "wits" Sleep 500ms Enter
Sleep 3s
Tab Sleep 3s
Tab Sleep 3s
Tab Sleep 3s
Type "q"
Sleep 1s

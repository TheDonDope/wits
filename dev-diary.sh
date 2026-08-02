#!/bin/bash

# Create diary entry using gum
TODAY=$(date "+%Y-%m-%d")
HEADER="# Development Diary"

# Check for existing entry. Confirming falls through to write a second one for
# the day, to be merged by hand; declining stops here.
if grep -q "## $TODAY" DEVDIARY.md; then
    gum confirm "Entry for today already exists! Continue? (You will have to manually merge your entries afterwards)" || exit 1
fi

# Get user inputs
TITLE=$(gum input --placeholder "Enter today's diary entry title")
SUMMARY=$(gum input --placeholder "Enter entry summary (4-6 sentences)")
NOTES=$(gum input --placeholder "Enter additional notes")

# Get today's git commits in chronological order
COMMITS=$(git log --since="midnight" --until="23:59:59" --reverse --format="%h %s")

# Build commit list
COMMIT_ENTRIES=""
while read -r commit; do
    sha=$(echo "$commit" | cut -d' ' -f1)
    msg=$(echo "$commit" | cut -d' ' -f2-)
    COMMIT_ENTRIES+="- \`$sha\` $msg"$'\n'
done <<< "$COMMITS"

# Create entry template
ENTRY="$HEADER

## $TODAY

### $TITLE

$SUMMARY

$COMMIT_ENTRIES
### Notes for $TODAY

$NOTES
"

# File handling
if [ ! -f DEVDIARY.md ]; then
    echo "$HEADER" > DEVDIARY.md
fi

# Preview and confirmation
echo -e "$ENTRY" | gum format -t markdown
gum confirm "Does this look correct?" || exit 1

# Build new file with entry + existing content
{ echo -e "$ENTRY"; tail -n +3 DEVDIARY.md; } > tmp.md && mv tmp.md DEVDIARY.md

gum style --foreground 212 "Diary entry added for $TODAY!"

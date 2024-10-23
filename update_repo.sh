#!/bin/bash
set -e

# Check if the commit message file argument is provided
if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <commit_message_file_path>"
    exit 1
fi

COMMIT_MSG_FILE="$1"

if [ -f "$COMMIT_MSG_FILE" ]; then
    echo "Changes detected. Proceeding with git operations."

    # Read the first line as the PR title
    PR_TITLE=$(head -n 1 "$COMMIT_MSG_FILE")
    echo "PR Title: $PR_TITLE"

    # Read the remaining lines as the PR body
    PR_BODY=$(tail -n +2 "$COMMIT_MSG_FILE")
    echo "PR Body: $PR_BODY"

    echo "Pushing changes..."
    git add .
    git checkout -b update_fdroid_apps
    git commit -F "$COMMIT_MSG_FILE"
    git push -f -u origin update_fdroid_apps

    echo "Creating label if it doesn't exist..."
    if ! gh label list | grep -q "automated pr"; then
        gh label create "automated pr" --description "Automatically created by GitHub Actions"
    fi

    echo "Creating PR..."
    PR_URL=$(gh pr create --title "$PR_TITLE" \
        --base main \
        --label "automated pr" \
        --body "$PR_BODY")
    echo "pr_number=${PR_URL##*/}"

    gh pr merge $PR_URL --squash --admin --delete-branch

    # Clean up the temporary commit message file
    rm "$COMMIT_MSG_FILE"

else
    echo "Error: Commit message file does not exist or could not be found: $COMMIT_MSG_FILE" >&2
    exit 1
fi

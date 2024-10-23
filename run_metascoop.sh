#!/bin/bash

# Check if the commit message file argument is provided
if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <commit_message_file_path>"
    exit 1
fi

COMMIT_MSG_FILE="$1"

echo "::group::Building metascoop executable"
cd metascoop
go build -o metascoop
echo "::endgroup::"

echo "::group::Running metascoop"
./metascoop -rp=../repos.yaml -rd=../fdroid/repo -pat="$GH_ACCESS_TOKEN" -cm="$COMMIT_MSG_FILE"
EXIT_CODE=$?
cd ..
echo "::endgroup::"

echo "Metascoop had an exit code of $EXIT_CODE"

if [ $EXIT_CODE -eq 2 ]; then
    echo "There were no significant changes"
    exit 2
elif [ $EXIT_CODE -eq 0 ]; then
    echo "Changes detected"
    exit 0
else
    echo "This is an unexpected error"
    exit 1
fi

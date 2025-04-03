#!/bin/bash

# get-version.sh - A script to determine the current version from Git tags
# If no tag is available, it will use dev-$git_short_sha

set -e

# Function to get current git commit short SHA
function get_git_short_sha {
    git rev-parse --short HEAD
}

# Try to get version from VERSION file first
if [ -f VERSION ]; then
    cat VERSION
    exit 0
fi

# Try to get the latest tag
latest_tag=$(git describe --tags --abbrev=0 2>/dev/null || echo "")

if [ -n "$latest_tag" ]; then
    # We have a tag, use it
    echo "$latest_tag"
else
    # No tag found, use dev-$git_short_sha
    echo "dev-$(get_git_short_sha)"
fi 
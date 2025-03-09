#!/bin/bash

# release.sh - A script to help increment semver and release the next version
# Works only on main branch and tracks versions from git tags

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to display usage information
function show_usage {
    echo -e "${BLUE}Usage:${NC} $0 [major|minor|patch|auto]"
    echo ""
    echo "Arguments:"
    echo "  major  - Increment the major version (x.0.0)"
    echo "  minor  - Increment the minor version (0.x.0)"
    echo "  patch  - Increment the patch version (0.0.x)"
    echo "  auto   - Automatically determine version increment based on commit messages"
    echo ""
    echo "Examples:"
    echo "  $0 patch  # Increment patch version"
    echo "  $0 auto   # Auto-determine version increment"
    exit 1
}

# Function to check if we're on the main branch
function check_main_branch {
    current_branch=$(git rev-parse --abbrev-ref HEAD)
    if [ "$current_branch" != "main" ]; then
        echo -e "${RED}Error:${NC} This script can only be run on the main branch."
        echo "Current branch: $current_branch"
        exit 1
    fi
}

# Function to check for uncommitted changes
function check_uncommitted_changes {
    if ! git diff-index --quiet HEAD --; then
        echo -e "${RED}Error:${NC} You have uncommitted changes."
        echo "Please commit or stash your changes before running this script."
        exit 1
    fi
}

# Function to get the latest version from git tags
function get_latest_version {
    # Get the latest tag that follows semver pattern (vX.Y.Z)
    latest_tag=$(git tag -l "v[0-9]*.[0-9]*.[0-9]*" | sort -V | tail -n 1)
    
    # If no tags exist, start with v0.1.0
    if [ -z "$latest_tag" ]; then
        echo "v0.1.0"
    else
        echo "$latest_tag"
    fi
}

# Function to increment version
function increment_version {
    local version=$1
    local increment_type=$2
    
    # Remove 'v' prefix if present
    version=${version#v}
    
    # Split version into components
    IFS='.' read -r major minor patch <<< "$version"
    
    case $increment_type in
        major)
            major=$((major + 1))
            minor=0
            patch=0
            ;;
        minor)
            minor=$((minor + 1))
            patch=0
            ;;
        patch)
            patch=$((patch + 1))
            ;;
        *)
            echo -e "${RED}Error:${NC} Invalid increment type: $increment_type"
            exit 1
            ;;
    esac
    
    echo "v$major.$minor.$patch"
}

# Function to determine version increment type based on commit messages
function determine_increment_type {
    local latest_tag=$1
    
    # If no tags exist, default to minor
    if [[ "$latest_tag" == "v0.1.0" && $(git tag -l "v0.1.0" | wc -l) -eq 0 ]]; then
        echo "minor"
        return
    fi
    
    # Get commits since the latest tag
    commits=$(git log --pretty=format:"%s" $latest_tag..HEAD)
    
    # Check for breaking changes or major feature additions
    if echo "$commits" | grep -i -E "BREAKING CHANGE|feat!:|fix!:|refactor!:|perf!:|major:" > /dev/null; then
        echo "major"
        return
    fi
    
    # Check for new features
    if echo "$commits" | grep -i -E "^feat:|^feature:|minor:" > /dev/null; then
        echo "minor"
        return
    fi
    
    # Default to patch for bug fixes, refactoring, etc.
    echo "patch"
}

# Function to generate release notes
function generate_release_notes {
    local latest_tag=$1
    local new_version=$2
    
    echo "# $new_version Release Notes"
    echo ""
    echo "## Changes since $latest_tag"
    echo ""
    
    # Group commits by type
    echo "### Features"
    git log --pretty=format:"- %s (%h)" $latest_tag..HEAD | grep -i -E "^- feat|^- feature" || echo "- No new features"
    echo ""
    
    echo "### Bug Fixes"
    git log --pretty=format:"- %s (%h)" $latest_tag..HEAD | grep -i "^- fix" || echo "- No bug fixes"
    echo ""
    
    echo "### Performance Improvements"
    git log --pretty=format:"- %s (%h)" $latest_tag..HEAD | grep -i "^- perf" || echo "- No performance improvements"
    echo ""
    
    echo "### Refactoring"
    git log --pretty=format:"- %s (%h)" $latest_tag..HEAD | grep -i "^- refactor" || echo "- No refactoring"
    echo ""
    
    echo "### Documentation"
    git log --pretty=format:"- %s (%h)" $latest_tag..HEAD | grep -i "^- docs" || echo "- No documentation changes"
    echo ""
    
    echo "### Other Changes"
    git log --pretty=format:"- %s (%h)" $latest_tag..HEAD | grep -i -v -E "^- (feat|fix|perf|refactor|docs|test|chore|style|ci|build)" || echo "- No other changes"
}

# Main script execution starts here

# Check if we have at least one argument
if [ $# -lt 1 ]; then
    show_usage
fi

# Parse command line arguments
increment_type=$1

# Validate increment type
if [[ "$increment_type" != "major" && "$increment_type" != "minor" && "$increment_type" != "patch" && "$increment_type" != "auto" ]]; then
    echo -e "${RED}Error:${NC} Invalid increment type: $increment_type"
    show_usage
fi

# Check if we're on the main branch
check_main_branch

# Check for uncommitted changes
check_uncommitted_changes

# Make sure we have the latest changes
echo -e "${BLUE}Fetching latest changes...${NC}"
git fetch --tags

# Get the latest version
latest_version=$(get_latest_version)
echo -e "${BLUE}Latest version:${NC} $latest_version"

# Determine increment type if auto
if [ "$increment_type" == "auto" ]; then
    increment_type=$(determine_increment_type "$latest_version")
    echo -e "${BLUE}Auto-determined increment type:${NC} $increment_type"
fi

# Increment version
new_version=$(increment_version "$latest_version" "$increment_type")
echo -e "${GREEN}New version:${NC} $new_version"

# Generate release notes
echo -e "${BLUE}Generating release notes...${NC}"
release_notes=$(generate_release_notes "$latest_version" "$new_version")

# Ask for confirmation
echo ""
echo -e "${YELLOW}Release notes preview:${NC}"
echo "$release_notes"
echo ""
read -p "Do you want to create this release? (y/n): " confirm
if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
    echo -e "${YELLOW}Release cancelled.${NC}"
    exit 0
fi

# Create release notes file
echo "$release_notes" > RELEASE_NOTES.md
echo -e "${BLUE}Release notes saved to RELEASE_NOTES.md${NC}"

# Create a new tag
echo -e "${BLUE}Creating new tag: $new_version${NC}"
git tag -a "$new_version" -m "Release $new_version"

# Push the tag
echo -e "${BLUE}Pushing tag to remote...${NC}"
git push origin "$new_version"

echo -e "${GREEN}Release $new_version created successfully!${NC}"
echo "Don't forget to update your changelog and documentation."

# Optional: Create GitHub release if gh CLI is installed
if command -v gh &> /dev/null; then
    echo -e "${BLUE}Creating GitHub release...${NC}"
    gh release create "$new_version" --title "$new_version" --notes-file RELEASE_NOTES.md
    echo -e "${GREEN}GitHub release created successfully!${NC}"
    # Clean up release notes file
    rm RELEASE_NOTES.md
fi

exit 0 
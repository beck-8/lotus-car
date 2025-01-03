#!/bin/bash

# This script bumps the version number in version/version.go
# Usage: ./bump_version.sh <major|minor|patch>

set -e

VERSION_FILE="version/version.go"
CURRENT_VERSION=$(grep 'Version = ' "$VERSION_FILE" | cut -d'"' -f2)

# Remove 'v' prefix if present
CURRENT_VERSION=${CURRENT_VERSION#v}

# Split version into components
IFS='.' read -r -a VERSION_PARTS <<< "$CURRENT_VERSION"
MAJOR="${VERSION_PARTS[0]}"
MINOR="${VERSION_PARTS[1]}"
PATCH="${VERSION_PARTS[2]}"

case "$1" in
  major)
    MAJOR=$((MAJOR + 1))
    MINOR=0
    PATCH=0
    ;;
  minor)
    MINOR=$((MINOR + 1))
    PATCH=0
    ;;
  patch)
    PATCH=$((PATCH + 1))
    ;;
  *)
    echo "Usage: $0 <major|minor|patch>"
    exit 1
    ;;
esac

NEW_VERSION="v$MAJOR.$MINOR.$PATCH"

# Update version in version.go
sed -i.bak "s/Version = \".*\"/Version = \"$NEW_VERSION\"/" "$VERSION_FILE"
rm -f "${VERSION_FILE}.bak"

echo "$NEW_VERSION"

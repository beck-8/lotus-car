#!/bin/bash

# Check if version argument is provided
if [ $# -ne 1 ]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 v1.0.5"
    exit 1
fi

VERSION=$1

# Validate version format
if [[ ! $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Error: Version must be in format vX.Y.Z (e.g., v1.0.5)"
    exit 1
fi

# Check if we are in the repository root
if [ ! -f "go.mod" ]; then
    echo "Error: Please run this script from the repository root"
    exit 1
fi

echo "Starting release process for version $VERSION..."

# Stage all changes
git add .

# Commit changes
git commit -m "chore: prepare for $VERSION release"
if [ $? -ne 0 ]; then
    echo "Error: Failed to commit changes"
    exit 1
fi

# Create release using make
make release-common NEW_VERSION=$VERSION
if [ $? -ne 0 ]; then
    echo "Error: Failed to create release"
    exit 1
fi

# Push the new tag
git push origin $VERSION
if [ $? -ne 0 ]; then
    echo "Error: Failed to push tag"
    exit 1
fi

# Verify the version
../lotus-car version

echo "Release $VERSION completed successfully!"
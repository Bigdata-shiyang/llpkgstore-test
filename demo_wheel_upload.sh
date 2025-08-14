#!/bin/bash

# Demo script for wheel upload functionality
# This script demonstrates how to use the wheel upload feature

echo "=== llpkgstore Wheel Upload Demo ==="
echo

# Check if we're in the right directory
if [ ! -f "cmd/llpkgstore/llpkgstore" ]; then
    echo "Building llpkgstore..."
    cd cmd/llpkgstore
    go build -o llpkgstore .
    cd ../..
fi

echo "Available commands:"
echo "1. Show help: ./cmd/llpkgstore/llpkgstore --help"
echo "2. Show wheel-upload help: ./cmd/llpkgstore/llpkgstore wheel-upload --help"
echo "3. Process a PR: ./cmd/llpkgstore/llpkgstore wheel-upload <PR_NUMBER>"
echo

echo "Environment variables needed:"
echo "- GITHUB_TOKEN: Your GitHub personal access token"
echo "- TARGET_REPO_OWNER: Target repository owner (default: Bigdata-shiyang)"
echo "- TARGET_REPO_NAME: Target repository name (default: test)"
echo "- GITHUB_REPOSITORY_OWNER: Source repository owner (default: goplus)"
echo "- GITHUB_REPOSITORY: Source repository name (default: llpkgstore)"
echo

echo "Example usage:"
echo "export GITHUB_TOKEN=your_token_here"
echo "./cmd/llpkgstore/llpkgstore wheel-upload 123"
echo

echo "PR format example:"
echo "Title: Add missing wheel: numpy"
echo "Description:"
echo "## Wheel Request"
echo "- **Library Name**: numpy"
echo "- **Version**: latest"
echo "- **Platform**: macos"
echo "- **Architecture**: x86_64"
echo "- **Use Case**: Need numpy for numerical computing"
echo

echo "The system will:"
echo "1. Parse PR title to extract 'numpy'"
echo "2. Search PyPI for numpy package"
echo "3. Download the best matching wheel file"
echo "4. Create/update GitHub Release: numpy/v<version>"
echo "5. Upload wheel file to the release"
echo "6. Add success comment to the PR"
echo

echo "Demo completed!" 
#!/bin/bash
set -e # Exit immediately if a command exits with a non-zero status

# --- Pre-checks ---
# Check if jq is installed
if ! command -v jq &> /dev/null
then
    echo "Error: jq is not installed. Please install jq to run this script."
    exit 1
fi

# Check if docker is installed
if ! command -v docker &> /dev/null
then
    echo "Error: Docker is not installed. Please install Docker to run this script."
    exit 1
fi

# Check if git is installed
if ! command -v git &> /dev/null
then
    echo "Error: Git is not installed. Please install Git to run this script."
    exit 1
fi

# Check if current branch is 'release'
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ "$CURRENT_BRANCH" != "release" ]; then
    echo "Error: This script must be run on the 'release' branch. Current branch is '$CURRENT_BRANCH'."
    exit 1
fi

# 1. Argument Parsing
if [ "$1" != "--semver" ] || [ -z "$2" ]; then
  echo "Usage: $0 --semver <major|minor|patch>"
  exit 1
fi
SEMVER_PART=$2

echo "Current Git branch: $CURRENT_BRANCH"
echo "Incrementing version part: $SEMVER_PART"

# Get current version and then update package.json using jq
PACKAGE_JSON="frontend/package.json"

# Read current version for comparison
CURRENT_VERSION=$(jq -r '.version' "$PACKAGE_JSON")
echo "Current version: $CURRENT_VERSION"

NEW_VERSION=""
case "$SEMVER_PART" in
    "major")
        # Increment major, set minor and patch to 0
        jq '.version |= (split(".") | (([.[0]|tonumber] + 1 | tostring) + ".0.0"))' "$PACKAGE_JSON" > "$PACKAGE_JSON.tmp" && mv "$PACKAGE_JSON.tmp" "$PACKAGE_JSON"
        NEW_VERSION=$(jq -r '.version' "$PACKAGE_JSON")
        ;;
    "minor")
        # Increment minor, set patch to 0
        jq '.version |= (split(".") as $v | ($v[0] | tostring) + "." + ([$v[1]|tonumber] + 1 | tostring) + ".0")' "$PACKAGE_JSON" > "$PACKAGE_JSON.tmp" && mv "$PACKAGE_JSON.tmp" "$PACKAGE_JSON"
        NEW_VERSION=$(jq -r '.version' "$PACKAGE_JSON")
        ;;
    "patch")
        # Increment patch
        jq '.version |= (split(".") as $v | ($v[0] | tostring) + "." + ($v[1] | tostring) + "." + ([$v[2]|tonumber] + 1 | tostring))' "$PACKAGE_JSON" > "$PACKAGE_JSON.tmp" && mv "$PACKAGE_JSON.tmp" "$PACKAGE_JSON"
        NEW_VERSION=$(jq -r '.version' "$PACKAGE_JSON")
        ;;
    *)
        echo "Invalid semver part: $SEMVER_PART. Must be 'major', 'minor', or 'patch'."
        exit 1
        ;;
esac

echo "Version updated in $PACKAGE_JSON to $NEW_VERSION"

# 3. Build Docker images
echo "Building backend image: valiant-backend:$NEW_VERSION"
DOCKER_BUILDKIT=1 docker build -f backend/Dockerfile.release -t "valiant-backend:$NEW_VERSION" .

echo "Building frontend image: valiant-frontend:$NEW_VERSION"
DOCKER_BUILDKIT=1 docker build -f frontend/Dockerfile.release -t "valiant-frontend:$NEW_VERSION" .

echo "Release images built successfully with tags: valiant-backend:$NEW_VERSION and valiant-frontend:$NEW_VERSION"

# 4. Create Git tag
echo "Staging changes and committing version bump..."
git add "$PACKAGE_JSON" # Stage the package.json change
git commit -m "chore: Release v$NEW_VERSION"
echo "Creating Git tag v$NEW_VERSION..."
git tag -a "v$NEW_VERSION" -m "Release v$NEW_VERSION"
echo "Git tag v$NEW_VERSION created."
git push --tags
echo "Git tag pushed to repository."
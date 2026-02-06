param(
    [Parameter(Mandatory=$true)]
    [ValidateSet('major', 'minor', 'patch')]
    [string]$semver
)

# --- Pre-checks ---
# Check if Docker is installed
if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    Write-Error "Error: Docker is not installed. Please install Docker to run this script."
    exit 1
}

# Check if Git is installed
if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
    Write-Error "Error: Git is not installed. Please install Git to run this script."
    exit 1
}

# Check if current branch is 'release'
$currentBranch = (git rev-parse --abbrev-ref HEAD).Trim()
if ($currentBranch -ne "release") {
    Write-Error "Error: This script must be run on the 'release' branch. Current branch is '$currentBranch'."
    exit 1
}

$ProjectRoot = Join-Path $PSScriptRoot ".."
$packageJsonPath = Join-Path $ProjectRoot "frontend\package.json"

if (-not (Test-Path $packageJsonPath)) {
    Write-Error "Error: frontend/package.json not found at $packageJsonPath"
    exit 1
}

# Read the file content as a single string
$packageJsonContent = Get-Content $packageJsonPath -Raw

# Safely get the current version number without altering the file
$currentVersion = ($packageJsonContent | ConvertFrom-Json).version

if (-not $currentVersion) {
    Write-Error "Error: Could not find 'version' field in $packageJsonPath"
    exit 1
}

$versionParts = $currentVersion.Split('.')
[int]$major = $versionParts[0]
[int]$minor = $versionParts[1]
[int]$patch = $versionParts[2]

Write-Host "Current Git branch: $currentBranch"
Write-Host "Current version: $currentVersion"
Write-Host "Incrementing version part: $semver"

switch ($semver) {
    'major' { $major++; $minor = 0; $patch = 0 }
    'minor' { $minor++; $patch = 0 }
    'patch' { $patch++ }
}

$newVersion = "$major.$minor.$patch"

# Construct the simple strings for replacement to preserve formatting
$stringToFind = '"version": "' + $currentVersion + '"'
$stringToReplace = '"version": "' + $newVersion + '"'

# Perform a direct string replacement on the file's content
$newPackageJsonContent = $packageJsonContent -replace $stringToFind, $stringToReplace

# Write the updated content back to the file
$newPackageJsonContent | Set-Content -NoNewline $packageJsonPath

Write-Host "Version updated in frontend/package.json to $newVersion"

# Docker build logic
$BackendDockerfile = Join-Path $ProjectRoot "backend\Dockerfile.release"
$FrontendDockerfile = Join-Path $ProjectRoot "frontend\Dockerfile.release"

Write-Host "Building backend image: valiant-backend:$newVersion"
docker build -f $BackendDockerfile -t "valiant-backend:$newVersion" $ProjectRoot

Write-Host "Building frontend image: valiant-frontend:$newVersion"
docker build -f $FrontendDockerfile -t "valiant-frontend:$newVersion" $ProjectRoot

Write-Host "Release images built successfully with tags: valiant-backend:$newVersion and valiant-frontend:$newVersion"

# Create Git tag
Write-Host "Staging changes and committing version bump..."
git add "$packageJsonPath"
git commit -m "chore: Release v$newVersion"
Write-Host "Creating Git tag v$newVersion..."
git tag -a "v$newVersion" -m "Release v$newVersion"
Write-Host "Git tag v$newVersion created."
git push --tags
Write-Host "Git tag pushed to repository."
#!/usr/bin/env pwsh
# Requires PowerShell 7 or later for pwsh shebang and some cmdlets

# Exit immediately if a command exits with a non-zero status.
$ErrorActionPreference = "Stop"

# Colors for output
$GREEN = ""
$RED = ""
$YELLOW = ""
$NC = ""
if ($Host.UI.SupportsVirtualTerminal) {
    $GREEN = "`e[0;32m"
    $RED = "`e[0;31m"
    $YELLOW = "`e[1;33m"
    $NC = "`e[0m" # No Color
}

Write-Host "=================================="
Write-Host "Valiant Event Links Tests"
Write-Host "=================================="

# Check if Docker is running
try {
    docker info | Out-Null
} catch {
    Write-Host "$($RED)Error: Docker is not running$($NC)"
    exit 1
}

# Start PostgreSQL container
Write-Host "$($YELLOW)Starting PostgreSQL container...$($NC)"
# Check if container already exists and remove it
$containerName = "valiant-test-postgres-" + (Get-Random)
docker rm -f $containerName | Out-Null 3>&1 | Out-Null
docker run -d `
  --name $containerName `
  -e POSTGRES_USER=testuser `
  -e POSTGRES_PASSWORD=testpass `
  -e POSTGRES_DB=valiant_test `
  -p 5432:5432 `
  postgres:15-alpine | Out-Null

# Wait for PostgreSQL to be ready
Write-Host "$($YELLOW)Waiting for PostgreSQL to be ready...$($NC)"
Start-Sleep -Seconds 3
$maxAttempts = 30
for ($i = 1; $i -le $maxAttempts; $i++) {
    try {
        docker exec $containerName pg_isready -U testuser | Out-Null
        Write-Host "$($GREEN)PostgreSQL is ready!$($NC)"
        break
    } catch {
        if ($i -eq $maxAttempts) {
            Write-Host "$($RED)PostgreSQL failed to start$($NC)"
            docker stop $containerName | Out-Null
            docker rm $containerName | Out-Null
            exit 1
        }
    }
    Start-Sleep -Seconds 1
}

# Set environment variables
$env:DB_HOST = "localhost"
$env:DB_PORT = "5432"
$env:DB_USER = "testuser"
$env:DB_PASSWORD = "testpass"
$env:DB_NAME = "valiant_test"

# Run tests
Write-Host "$($YELLOW)Running tests...$($NC)"
Write-Host ""
go test ./... -v -count=1
if ($LASTEXITCODE -eq 0) {
    Write-Host ""
    Write-Host "$($GREEN)All tests passed!$($NC)"
    $EXIT_CODE = 0
} else {
    Write-Host ""
    Write-Host "$($RED)Tests failed!$($NC)"
    $EXIT_CODE = 1
}

# Cleanup
Write-Host "$($YELLOW)Cleaning up...$($NC)"
docker stop $containerName | Out-Null
docker rm $containerName | Out-Null

Write-Host "$($GREEN)Cleanup complete$($NC)"
exit $EXIT_CODE

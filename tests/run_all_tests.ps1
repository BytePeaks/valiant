#!/usr/bin/env pwsh
# Requires PowerShell 7+

$ErrorActionPreference = "Continue"

# ------------------------------------------------------------
# Configuration
# ------------------------------------------------------------

$SUITES = @(
    "correlator-engine",
    "intent-execution-linking",
    "time-windows",
    "kubernetes-collector",
    "deep-linking",
    "timestamp-guardrail"
)

$BACKEND_UNIT_TEST_NAME = "backend-unit-tests"
$BACKEND_UNIT_TEST_CMD  = "go test ./... -v -count=1"

# ------------------------------------------------------------
# State
# ------------------------------------------------------------

$RESULTS = @{}
$DURATIONS = @{}
$TOTAL_FAILED_COUNT = 0
$TOTAL_TESTS = 0
$TOTAL_TIMER = [System.Diagnostics.Stopwatch]::StartNew()

# ------------------------------------------------------------
# Colors (PowerShell-native)
# ------------------------------------------------------------

$GREEN  = $PSStyle.Foreground.Green
$RED    = $PSStyle.Foreground.Red
$YELLOW = $PSStyle.Foreground.Yellow
$CYAN   = $PSStyle.Foreground.Cyan
$NC     = $PSStyle.Reset

# ------------------------------------------------------------
# Header
# ------------------------------------------------------------

Write-Host "${CYAN}==========================================${NC}"
Write-Host "${CYAN}  RUN ALL TESTS${NC}"
Write-Host "${CYAN}==========================================${NC}"
Write-Host "Date: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')"
Write-Host "Suites: $($SUITES -join ', ')"
Write-Host "Backend Unit Tests: $BACKEND_UNIT_TEST_CMD"
Write-Host "${CYAN}==========================================${NC}"
Write-Host ""

Push-Location

# ------------------------------------------------------------
# Integration test suites
# ------------------------------------------------------------

foreach ($suite in $SUITES) {
    $TOTAL_TESTS++

    Write-Host "${YELLOW}------------------------------------------${NC}"
    Write-Host "${YELLOW} Running suite: $suite${NC}"
    Write-Host "${YELLOW}------------------------------------------${NC}"

    $scriptPath = Join-Path $suite "docker-test.ps1"

    if (Test-Path $scriptPath) {
        $timer = [System.Diagnostics.Stopwatch]::StartNew()

        Push-Location $suite
        & .\docker-test.ps1
        $exitCode = $LASTEXITCODE
        Pop-Location

        $timer.Stop()
        $DURATIONS[$suite] = $timer.Elapsed

        if ($exitCode -eq 0) {
            $RESULTS[$suite] = "PASS"
            Write-Host ""
            Write-Host "${GREEN}OK! Suite '$suite' PASSED in $($timer.Elapsed.ToString('mm\:ss'))${NC}"
        } else {
            $RESULTS[$suite] = "FAIL"
            $TOTAL_FAILED_COUNT++
            Write-Host ""
            Write-Host "${RED}FAIL! Suite '$suite' FAILED in $($timer.Elapsed.ToString('mm\:ss')) (Exit Code: $exitCode)${NC}"
        }
    }
    else {
        $RESULTS[$suite] = "SKIP"
        $DURATIONS[$suite] = [TimeSpan]::Zero
        Write-Host "${YELLOW}!  Skipping '$suite': docker-test.ps1 not found${NC}"
    }

    Write-Host ""
}

# ------------------------------------------------------------
# Backend unit tests
# ------------------------------------------------------------

$TOTAL_TESTS++

Write-Host "${YELLOW}------------------------------------------${NC}"
Write-Host "${YELLOW} Running backend unit tests: $BACKEND_UNIT_TEST_NAME${NC}"
Write-Host "${YELLOW}------------------------------------------${NC}"

$timer = [System.Diagnostics.Stopwatch]::StartNew()

# tests/ -> repo root -> backend
$backendPath = Resolve-Path (Join-Path $PSScriptRoot "..\backend")

Push-Location $backendPath
& go test ./... -v -count=1
$exitCode = $LASTEXITCODE
Pop-Location

$timer.Stop()
$DURATIONS[$BACKEND_UNIT_TEST_NAME] = $timer.Elapsed

if ($exitCode -eq 0) {
    $RESULTS[$BACKEND_UNIT_TEST_NAME] = "PASS"
    Write-Host ""
    Write-Host "${GREEN}OK! $BACKEND_UNIT_TEST_NAME PASSED in $($timer.Elapsed.ToString('mm\:ss'))${NC}"
}
else {
    $RESULTS[$BACKEND_UNIT_TEST_NAME] = "FAIL"
    $TOTAL_FAILED_COUNT++
    Write-Host ""
    Write-Host "${RED}FAIL! $BACKEND_UNIT_TEST_NAME FAILED in $($timer.Elapsed.ToString('mm\:ss')) (Exit Code: $exitCode)${NC}"
}

Write-Host ""
$TOTAL_TIMER.Stop()

# ------------------------------------------------------------
# Summary
# ------------------------------------------------------------

Write-Host "${CYAN}==========================================${NC}"
Write-Host "${CYAN} SUITES SUMMARY${NC}"
Write-Host "${CYAN}==========================================${NC}"
Write-Host ("{0,-30} | {1,-10} | {2,-10}" -f "TEST SUITE", "RESULT", "DURATION")
Write-Host "------------------------------------------------------"

foreach ($suite in $SUITES + $BACKEND_UNIT_TEST_NAME) {
    $res = $RESULTS[$suite]
    $dur = $DURATIONS[$suite].ToString("mm\:ss")

    $color = switch ($res) {
        "PASS" { $GREEN }
        "FAIL" { $RED }
        "SKIP" { $YELLOW }
        default { $NC }
    }

    Write-Host "${color}$("{0,-30} | {1,-10} | {2,-10}" -f $suite, $res, $dur)${NC}"
}

Write-Host "------------------------------------------------------"
Write-Host "Total Duration: $($TOTAL_TIMER.Elapsed.ToString('mm\:ss'))"
Write-Host "${CYAN}==========================================${NC}"

Pop-Location

# ------------------------------------------------------------
# Exit code
# ------------------------------------------------------------

if ($TOTAL_FAILED_COUNT -eq 0) {
    Write-Host "${GREEN}ALL SYSTEMS GO! All tests passed.${NC}"
    exit 0
}
else {
    Write-Host "${RED}$TOTAL_FAILED_COUNT test suite(s) failed.${NC}"
    exit 1
}

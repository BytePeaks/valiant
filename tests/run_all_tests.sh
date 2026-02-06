#!/bin/bash
set -euo pipefail

# ------------------------------------------------------------
# Resolve paths (CWD-independent)
# ------------------------------------------------------------

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BACKEND_DIR="$REPO_ROOT/backend"

# ------------------------------------------------------------
# Colors
# ------------------------------------------------------------

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# ------------------------------------------------------------
# Configuration
# ------------------------------------------------------------

SUITES=(
    "correlator-engine"
    "intent-execution-linking"
    "time-windows"
    "kubernetes-collector"
)

BACKEND_UNIT_TEST_NAME="backend-unit-tests"

# ------------------------------------------------------------
# State
# ------------------------------------------------------------

declare -A RESULTS
declare -A DURATIONS
TOTAL_FAILED_COUNT=0
TOTAL_TESTS=0

TOTAL_TIMER_START=$(date +%s%N)

# ------------------------------------------------------------
# Header
# ------------------------------------------------------------

echo -e "${CYAN}==========================================${NC}"
echo -e "${CYAN}  RUN ALL TESTS ${NC}"
echo -e "${CYAN}==========================================${NC}"
echo "Date: $(date +'%Y-%m-%d %H:%M:%S')"
echo "Suites: ${SUITES[*]}"
echo "Backend Unit Tests: go test ./... -v -count=1"
echo -e "${CYAN}==========================================${NC}"
echo ""

# ------------------------------------------------------------
# Integration test suites
# ------------------------------------------------------------

for suite in "${SUITES[@]}"; do
    TOTAL_TESTS=$((TOTAL_TESTS + 1))

    echo -e "${YELLOW}------------------------------------------${NC}"
    echo -e "${YELLOW} Running suite: $suite${NC}"
    echo -e "${YELLOW}------------------------------------------${NC}"

    SUITE_DIR="$SCRIPT_DIR/$suite"
    SUITE_START=$(date +%s%N)

    if [[ -f "$SUITE_DIR/docker-test.sh" ]]; then
        (cd "$SUITE_DIR" && ./docker-test.sh)
        EXIT_CODE=$?
    else
        RESULTS[$suite]="SKIP"
        DURATIONS[$suite]="N/A"
        echo -e "\n${YELLOW}!  Skipping '$suite': docker-test.sh not found${NC}\n"
        continue
    fi

    SUITE_END=$(date +%s%N)
    DURATION_NS=$((SUITE_END - SUITE_START))
    DURATION_S=$(awk "BEGIN {printf \"%.2f\", $DURATION_NS/1000000000}")

    if [[ $EXIT_CODE -eq 0 ]]; then
        RESULTS[$suite]="PASS"
        DURATIONS[$suite]="${DURATION_S}s"
        echo -e "\n${GREEN}OK! Suite '$suite' PASSED in ${DURATION_S}s${NC}"
    else
        RESULTS[$suite]="FAIL"
        DURATIONS[$suite]="${DURATION_S}s"
        TOTAL_FAILED_COUNT=$((TOTAL_FAILED_COUNT + 1))
        echo -e "\n${RED}FAIL! Suite '$suite' FAILED in ${DURATION_S}s (Exit Code: $EXIT_CODE)${NC}"
    fi

    echo ""
done

# ------------------------------------------------------------
# Backend unit tests
# ------------------------------------------------------------

TOTAL_TESTS=$((TOTAL_TESTS + 1))

echo -e "${YELLOW}------------------------------------------${NC}"
echo -e "${YELLOW} Running backend unit tests: $BACKEND_UNIT_TEST_NAME${NC}"
echo -e "${YELLOW}------------------------------------------${NC}"

if [[ ! -d "$BACKEND_DIR" ]]; then
    echo -e "${RED}Backend directory not found: $BACKEND_DIR${NC}"
    exit 2
fi

BACKEND_START=$(date +%s%N)

(
    cd "$BACKEND_DIR"
    go test ./... -v -count=1
)
EXIT_CODE=$?

BACKEND_END=$(date +%s%N)
DURATION_NS=$((BACKEND_END - BACKEND_START))
DURATION_S=$(awk "BEGIN {printf \"%.2f\", $DURATION_NS/1000000000}")

if [[ $EXIT_CODE -eq 0 ]]; then
    RESULTS[$BACKEND_UNIT_TEST_NAME]="PASS"
    DURATIONS[$BACKEND_UNIT_TEST_NAME]="${DURATION_S}s"
    echo -e "\n${GREEN}OK! $BACKEND_UNIT_TEST_NAME PASSED in ${DURATION_S}s${NC}"
else
    RESULTS[$BACKEND_UNIT_TEST_NAME]="FAIL"
    DURATIONS[$BACKEND_UNIT_TEST_NAME]="${DURATION_S}s"
    TOTAL_FAILED_COUNT=$((TOTAL_FAILED_COUNT + 1))
    echo -e "\n${RED}FAIL! $BACKEND_UNIT_TEST_NAME FAILED in ${DURATION_S}s (Exit Code: $EXIT_CODE)${NC}"
fi

echo ""

# ------------------------------------------------------------
# Summary
# ------------------------------------------------------------

TOTAL_TIMER_END=$(date +%s%N)
TOTAL_DURATION_NS=$((TOTAL_TIMER_END - TOTAL_TIMER_START))
TOTAL_DURATION_S=$(awk "BEGIN {printf \"%.2f\", $TOTAL_DURATION_NS/1000000000}")

echo -e "${CYAN}==========================================${NC}"
echo -e "${CYAN} TESTS SUMMARY${NC}"
echo -e "${CYAN}==========================================${NC}"
printf "%-30s | %-10s | %-10s\n" "TEST SUITE" "RESULT" "DURATION"
echo "------------------------------------------------------"

for suite in "${SUITES[@]}" "$BACKEND_UNIT_TEST_NAME"; do
    case "${RESULTS[$suite]}" in
        PASS) COLOR="$GREEN" ;;
        FAIL) COLOR="$RED" ;;
        SKIP) COLOR="$YELLOW" ;;
        *)    COLOR="$NC" ;;
    esac
    printf "${COLOR}%-30s | %-10s | %-10s${NC}\n" \
        "$suite" "${RESULTS[$suite]}" "${DURATIONS[$suite]}"
done

echo "------------------------------------------------------"
echo "Total Duration: ${TOTAL_DURATION_S}s"
echo -e "${CYAN}==========================================${NC}"

# ------------------------------------------------------------
# Exit
# ------------------------------------------------------------

if [[ $TOTAL_FAILED_COUNT -eq 0 ]]; then
    echo -e "${GREEN}ALL SYSTEMS GO! All tests passed.${NC}"
    exit 0
else
    echo -e "${RED}${TOTAL_FAILED_COUNT} test suite(s) failed.${NC}"
    exit 1
fi

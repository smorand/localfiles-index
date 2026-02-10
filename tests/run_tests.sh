#!/bin/bash
# LocalFiles Index - Functional Test Runner
# Runs all test_*.sh scripts in the tests/ directory

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PASS=0
FAIL=0
SKIP=0
ERRORS=""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m'

echo "========================================="
echo "  LocalFiles Index - Functional Tests"
echo "========================================="
echo ""

# Find and run all test scripts
for test_file in "$SCRIPT_DIR"/test_*.sh; do
    [ -f "$test_file" ] || continue

    test_name=$(basename "$test_file" .sh)
    echo -n "Running $test_name... "

    if [ ! -x "$test_file" ]; then
        chmod +x "$test_file"
    fi

    output=$("$test_file" 2>&1) && status=0 || status=$?

    if [ $status -eq 0 ]; then
        echo -e "${GREEN}PASS${NC}"
        PASS=$((PASS + 1))
    elif [ $status -eq 2 ]; then
        echo -e "${YELLOW}SKIP${NC}"
        SKIP=$((SKIP + 1))
    else
        echo -e "${RED}FAIL${NC}"
        FAIL=$((FAIL + 1))
        ERRORS="$ERRORS\n--- $test_name ---\n$output\n"
    fi
done

echo ""
echo "========================================="
echo "Results: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped"
echo "========================================="

if [ -n "$ERRORS" ]; then
    echo ""
    echo "Failures:"
    echo -e "$ERRORS"
fi

exit $FAIL

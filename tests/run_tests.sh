#!/bin/bash
# LocalFiles Index - Functional Test Runner
# Runs all test suites in a fixed order (lightest API usage first)
# Retries failed tests once (Gemini API rate limits can cause transient failures)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DB_URL="postgresql://localfiles:localfiles@localhost:5432/localfiles?sslmode=disable"
PASS=0
FAIL=0
SKIP=0
ERRORS=""
FAILED_TESTS=()

# Temporarily disable auto-tag rules to reduce Gemini API calls during tests.
# This prevents personal tag rules from triggering LLM calls on every index operation.
SAVED_RULES=$(psql "$DB_URL" -t -A -c "SELECT name || '|' || rule FROM tags WHERE rule != '';" 2>/dev/null || true)
if [ -n "$SAVED_RULES" ]; then
    psql "$DB_URL" -c "UPDATE tags SET rule = '' WHERE rule != '';" >/dev/null 2>&1 || true
fi

restore_rules() {
    if [ -n "$SAVED_RULES" ]; then
        while IFS='|' read -r name rule; do
            [ -z "$name" ] && continue
            psql "$DB_URL" -c "UPDATE tags SET rule = \$\$${rule}\$\$ WHERE name = '${name}';" >/dev/null 2>&1 || true
        done <<< "$SAVED_RULES"
    fi
}
trap restore_rules EXIT

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m'

echo "========================================="
echo "  LocalFiles Index - Functional Tests"
echo "========================================="
echo ""

# Test suites ordered by Gemini API usage (lightest first)
TEST_ORDER=(
    test_tags           # No API calls (tag CRUD only)
    test_search         # 3 index + few searches
    test_text_pdf       # 4 index operations
    test_index          # Image + text indexing
    test_update         # Re-index + conversion
    test_cli_workflow   # Full workflow (heaviest)
    test_mcp            # MCP server + REST API
)

for test_name in "${TEST_ORDER[@]}"; do
    test_file="$SCRIPT_DIR/${test_name}.sh"
    [ -f "$test_file" ] || continue

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
        echo -e "${YELLOW}FAIL (will retry)${NC}"
        FAILED_TESTS+=("$test_file")
    fi

    # Brief pause between test suites (app has internal retry with exponential backoff)
    sleep 3
done

# Retry failed tests once (API rate limits are often transient)
if [ ${#FAILED_TESTS[@]} -gt 0 ]; then
    echo ""
    echo "Retrying ${#FAILED_TESTS[@]} failed test(s) after cooldown..."
    sleep 20

    for test_file in "${FAILED_TESTS[@]}"; do
        test_name=$(basename "$test_file" .sh)
        echo -n "Retrying $test_name... "

        output=$("$test_file" 2>&1) && status=0 || status=$?

        if [ $status -eq 0 ]; then
            echo -e "${GREEN}PASS${NC}"
            PASS=$((PASS + 1))
        else
            echo -e "${RED}FAIL${NC}"
            FAIL=$((FAIL + 1))
            ERRORS="$ERRORS\n--- $test_name ---\n$output\n"
        fi

        sleep 5
    done
fi

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

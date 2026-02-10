#!/bin/bash
# Test: Category Management (Lot 5)
# Validates: FR-013, FR-015
# Test Scenarios: TS-013, TS-042

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$SCRIPT_DIR/../bin/localfiles-index-darwin-arm64"
FIXTURES="$SCRIPT_DIR/fixtures/generated"
DB_URL="postgresql://localfiles:localfiles@localhost:5432/localfiles?sslmode=disable"

db_query() {
    psql "$DB_URL" -t -A -c "$1" 2>/dev/null
}

cleanup() {
    db_query "DELETE FROM images WHERE document_id IN (SELECT id FROM documents WHERE file_path LIKE '%/tests/fixtures/generated/%');" >/dev/null 2>&1 || true
    db_query "DELETE FROM chunks WHERE document_id IN (SELECT id FROM documents WHERE file_path LIKE '%/tests/fixtures/generated/%');" >/dev/null 2>&1 || true
    db_query "DELETE FROM documents WHERE file_path LIKE '%/tests/fixtures/generated/%';" >/dev/null 2>&1 || true
    db_query "DELETE FROM categories WHERE name IN ('cat_test', 'cat_temp', 'cat_force', 'cat_migrate');" >/dev/null 2>&1 || true
}

assert_eq() {
    if [ "$2" != "$3" ]; then echo "FAIL: $1 (expected='$2', got='$3')"; exit 1; fi
}

cleanup

ERRORS=0
PASS=0
run_test() { echo -n "  $1: $2... "; }
pass_test() { echo "OK"; PASS=$((PASS + 1)); }
fail_test() { echo "FAIL: $1"; ERRORS=$((ERRORS + 1)); }

echo "=== Lot 5: Category Management Tests ==="

# ---------------------------------------------------------------
# TS-013: Category CRUD via CLI
# ---------------------------------------------------------------
run_test "TS-013" "Category CRUD lifecycle"

# 1. Create category
OUTPUT=$($BIN categories add cat_test --description "Test category" 2>/dev/null) && RC=0 || RC=$?
assert_eq "create category exit" "0" "$RC"
if ! echo "$OUTPUT" | grep -q "cat_test"; then
    fail_test "Create output missing category name"
else
    # 2. List categories
    LIST_OUTPUT=$($BIN categories list 2>/dev/null)
    if ! echo "$LIST_OUTPUT" | grep -q "cat_test"; then
        fail_test "List does not show cat_test"
    else
        # 3. Update category
        $BIN categories update cat_test --description "Updated test category" >/dev/null 2>&1 && RC=0 || RC=$?
        assert_eq "update exit" "0" "$RC"

        # 4. Verify updated description
        DESC=$(db_query "SELECT description FROM categories WHERE name = 'cat_test';")
        assert_eq "updated description" "Updated test category" "$DESC"

        # 5. Remove category (no documents reference it)
        $BIN categories remove cat_test >/dev/null 2>&1 && RC=0 || RC=$?
        assert_eq "remove exit" "0" "$RC"

        # 6. Verify removed
        LIST_OUTPUT2=$($BIN categories list 2>/dev/null)
        if echo "$LIST_OUTPUT2" | grep -q "cat_test"; then
            fail_test "Category still exists after removal"
        else
            pass_test
        fi
    fi
fi

# ---------------------------------------------------------------
# TS-042: Category Remove with Document Migration
# ---------------------------------------------------------------
run_test "TS-042" "Category remove with --new-category migrates documents"

# Setup: create categories and index a file
$BIN categories add cat_force --description "Force test" >/dev/null 2>&1
$BIN categories add cat_migrate --description "Migration target" >/dev/null 2>&1
ABS_PATH=$(cd "$FIXTURES" && pwd)/diagram.png
$BIN index "$ABS_PATH" --category cat_force >/dev/null 2>&1

# 1. Remove without --new-category — should fail
OUTPUT=$($BIN categories remove cat_force 2>&1) && RC=0 || RC=$?
if [ $RC -eq 0 ]; then
    fail_test "Remove should fail when docs reference it without --new-category"
else
    # 2. Remove with --new-category — should succeed and migrate docs
    $BIN categories remove cat_force --new-category cat_migrate >/dev/null 2>&1 && RC=0 || RC=$?
    assert_eq "remove with migration exit" "0" "$RC"

    # 3. Document should now be in cat_migrate
    CAT_NAME=$(db_query "SELECT c.name FROM documents d JOIN categories c ON c.id = d.category_id WHERE d.file_path = '$ABS_PATH';")
    assert_eq "document migrated to cat_migrate" "cat_migrate" "$CAT_NAME"

    pass_test
fi

# ---------------------------------------------------------------
# Cleanup
# ---------------------------------------------------------------
cleanup

echo ""
echo "=== Results: $PASS passed, $ERRORS failed ==="
if [ $ERRORS -gt 0 ]; then exit 1; fi
exit 0

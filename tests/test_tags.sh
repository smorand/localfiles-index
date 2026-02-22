#!/bin/bash
# Test: Tag Management (Lot 5)
# Validates: FR-013, FR-015
# Test Scenarios: TS-013, TS-042, TS-043, TS-044

set -eo pipefail

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
    db_query "DELETE FROM tags WHERE name IN ('tag_test', 'tag_temp', 'tag_force', 'tag_merge_src', 'tag_merge_dst', 'testmixedcase', 'tag_rule');" >/dev/null 2>&1 || true
}

trap cleanup EXIT

assert_eq() {
    if [ "$2" != "$3" ]; then echo "FAIL: $1 (expected='$2', got='$3')"; exit 1; fi
}

cleanup

ERRORS=0
PASS=0
run_test() { echo -n "  $1: $2... "; }
pass_test() { echo "OK"; PASS=$((PASS + 1)); }
fail_test() { echo "FAIL: $1"; ERRORS=$((ERRORS + 1)); }

echo "=== Lot 5: Tag Management Tests ==="

# ---------------------------------------------------------------
# TS-013: Tag CRUD via CLI
# ---------------------------------------------------------------
run_test "TS-013" "Tag CRUD lifecycle"

# 1. Create tag
OUTPUT=$($BIN tags add tag_test --description "Test tag" 2>/dev/null) && RC=0 || RC=$?
assert_eq "create tag exit" "0" "$RC"
if ! echo "$OUTPUT" | grep -q "tag_test"; then
    fail_test "Create output missing tag name"
else
    # 2. List tags
    LIST_OUTPUT=$($BIN tags list 2>/dev/null)
    if ! echo "$LIST_OUTPUT" | grep -q "tag_test"; then
        fail_test "List does not show tag_test"
    else
        # 3. Update tag
        $BIN tags update tag_test --description "Updated test tag" >/dev/null 2>&1 && RC=0 || RC=$?
        assert_eq "update exit" "0" "$RC"

        # 4. Verify updated description
        DESC=$(db_query "SELECT description FROM tags WHERE name = 'tag_test';")
        assert_eq "updated description" "Updated test tag" "$DESC"

        # 5. Remove tag (no documents reference it)
        $BIN tags remove tag_test >/dev/null 2>&1 && RC=0 || RC=$?
        assert_eq "remove exit" "0" "$RC"

        # 6. Verify removed
        LIST_OUTPUT2=$($BIN tags list 2>/dev/null)
        if echo "$LIST_OUTPUT2" | grep -q "tag_test"; then
            fail_test "Tag still exists after removal"
        else
            pass_test
        fi
    fi
fi

# ---------------------------------------------------------------
# TS-042: Tag Merge
# ---------------------------------------------------------------
run_test "TS-042" "Tag merge moves documents from source to target"

# Setup: create tags and index a file with source tag
$BIN tags add tag_merge_src --description "Merge source" >/dev/null 2>&1
$BIN tags add tag_merge_dst --description "Merge destination" >/dev/null 2>&1
ABS_PATH=$(cd "$FIXTURES" && pwd)/diagram.png
$BIN index "$ABS_PATH" --tags tag_merge_src >/dev/null 2>&1

# 1. Verify document has source tag
SRC_TAG=$(db_query "SELECT t.name FROM document_tags dt JOIN tags t ON t.id = dt.tag_id WHERE dt.document_id = (SELECT id FROM documents WHERE file_path = '$ABS_PATH') AND t.name = 'tag_merge_src';")
assert_eq "document has source tag" "tag_merge_src" "$SRC_TAG"

# 2. Merge source into destination
$BIN tags merge tag_merge_src tag_merge_dst >/dev/null 2>&1 && RC=0 || RC=$?
assert_eq "merge exit" "0" "$RC"

# 3. Document should now have destination tag
DST_TAG=$(db_query "SELECT t.name FROM document_tags dt JOIN tags t ON t.id = dt.tag_id WHERE dt.document_id = (SELECT id FROM documents WHERE file_path = '$ABS_PATH') AND t.name = 'tag_merge_dst';")
assert_eq "document has destination tag" "tag_merge_dst" "$DST_TAG"

# 4. Source tag should be deleted
SRC_EXISTS=$(db_query "SELECT count(*) FROM tags WHERE name = 'tag_merge_src';")
assert_eq "source tag deleted" "0" "$SRC_EXISTS"

pass_test

# ---------------------------------------------------------------
# TS-043: Case-Insensitive Tag Names
# ---------------------------------------------------------------
run_test "TS-043" "Tags are normalized to lowercase"

# 1. Create with mixed case
OUTPUT=$($BIN tags add TestMixedCase --description "Mixed case test" 2>/dev/null) && RC=0 || RC=$?
assert_eq "create mixed case exit" "0" "$RC"

# 2. Verify stored as lowercase
STORED_NAME=$(db_query "SELECT name FROM tags WHERE name = 'testmixedcase';")
if [ "$STORED_NAME" != "testmixedcase" ]; then
    fail_test "Tag not stored as lowercase (got '$STORED_NAME')"
else
    # 3. Look up with different casing
    LOOKUP_OUTPUT=$($BIN tags update TESTMIXEDCASE --description "Updated via uppercase" 2>/dev/null) && RC=0 || RC=$?
    assert_eq "update via uppercase exit" "0" "$RC"

    DESC=$(db_query "SELECT description FROM tags WHERE name = 'testmixedcase';")
    assert_eq "description updated via uppercase lookup" "Updated via uppercase" "$DESC"

    # 4. Remove with different casing
    $BIN tags remove TestMixedCase >/dev/null 2>&1 && RC=0 || RC=$?
    assert_eq "remove via mixed case exit" "0" "$RC"

    pass_test
fi

# ---------------------------------------------------------------
# TS-044: Tag Rule Management
# ---------------------------------------------------------------
run_test "TS-044" "Tag rule management (create with rule, update rule)"

# 1. Create tag with rule
$BIN tags add tag_rule --description "Rule test" --rule "Apply when document is a rule test" >/dev/null 2>&1 && RC=0 || RC=$?
assert_eq "create tag with rule exit" "0" "$RC"

# 2. Verify rule stored
RULE=$(db_query "SELECT rule FROM tags WHERE name = 'tag_rule';")
assert_eq "rule stored" "Apply when document is a rule test" "$RULE"

# 3. Update rule
$BIN tags update tag_rule --rule "Updated rule text" >/dev/null 2>&1 && RC=0 || RC=$?
assert_eq "update rule exit" "0" "$RC"

RULE2=$(db_query "SELECT rule FROM tags WHERE name = 'tag_rule';")
assert_eq "rule updated" "Updated rule text" "$RULE2"

pass_test

# ---------------------------------------------------------------
# Cleanup
# ---------------------------------------------------------------
cleanup

echo ""
echo "=== Results: $PASS passed, $ERRORS failed ==="
if [ $ERRORS -gt 0 ]; then exit 1; fi
exit 0

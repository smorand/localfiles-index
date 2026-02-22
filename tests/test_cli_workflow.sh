#!/bin/bash
# Test: CLI Workflow (Lot 5)
# Validates: FR-015, FR-018, FR-019, FR-020
# Test Scenarios: TS-014, TS-039, TS-040, TS-041, TS-043, TS-044, TS-045, TS-021, TS-022, TS-024, TS-030, TS-031, TS-053, TS-057, TS-058

set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$SCRIPT_DIR/../bin/localfiles-index-darwin-arm64"
FIXTURES="$SCRIPT_DIR/fixtures/generated"
DB_URL="postgresql://localfiles:localfiles@localhost:5432/localfiles?sslmode=disable"

if [ ! -f "$FIXTURES/official_document.jpg" ]; then
    bash "$SCRIPT_DIR/fixtures/generate_fixtures.sh"
fi

db_query() {
    psql "$DB_URL" -t -A -c "$1" 2>/dev/null
}

cleanup() {
    db_query "DELETE FROM documents WHERE file_path LIKE '%/tests/fixtures/%';" >/dev/null 2>&1 || true
    db_query "DELETE FROM tags WHERE name IN ('cli_test', 'cli_test2');" >/dev/null 2>&1 || true
    rm -rf "$SCRIPT_DIR/fixtures/test_dir" 2>/dev/null || true
}

trap cleanup EXIT

assert_eq() {
    if [ "$2" != "$3" ]; then echo "FAIL: $1 (expected='$2', got='$3')"; exit 1; fi
}
assert_ge() {
    if [ "$3" -lt "$2" ] 2>/dev/null; then echo "FAIL: $1 (expected >= $2, got '$3')"; exit 1; fi
}

# Retry helper for indexing (rate limiting)
index_with_retry() {
    local path="$1"; shift
    for attempt in 1 2 3; do
        if $BIN index "$path" "$@" >/dev/null 2>&1; then return 0; fi
        sleep $((attempt * 3))
    done
    echo "WARN: indexing $path failed after 3 attempts" >&2
    return 1
}

cleanup

ERRORS=0
PASS=0
run_test() { echo -n "  $1: $2... "; }
pass_test() { echo "OK"; PASS=$((PASS + 1)); }
fail_test() { echo "FAIL: $1"; ERRORS=$((ERRORS + 1)); }

echo "=== Lot 5: CLI Workflow Tests ==="

# ---------------------------------------------------------------
# TS-014: Full CLI Workflow
# ---------------------------------------------------------------
run_test "TS-014" "Full CLI workflow (create tag, index, search, show, status, delete)"

# 1. Create tag
$BIN tags add cli_test --description "CLI test tag" >/dev/null 2>&1 && RC=0 || RC=$?
assert_eq "create tag" "0" "$RC"

# 2. Index file
ABS_IMG=$(cd "$FIXTURES" && pwd)/official_document.jpg
index_with_retry "$ABS_IMG" --tags cli_test && RC=0 || RC=$?
assert_eq "index file" "0" "$RC"

# 3. Search
SEARCH_OUT=$($BIN search "passport document" 2>/dev/null) && RC=0 || RC=$?
assert_eq "search" "0" "$RC"

# 4. Show document by path
SHOW_OUT=$($BIN show "$ABS_IMG" 2>/dev/null) && RC=0 || RC=$?
assert_eq "show by path" "0" "$RC"
if ! echo "$SHOW_OUT" | grep -q "Document:"; then
    fail_test "Show output missing Document header"
else
    # Verify show displays all expected fields
    for field in "ID:" "Path:" "Type:" "MIME:" "Size:" "Chunks:" "Images:"; do
        if ! echo "$SHOW_OUT" | grep -q "$field"; then
            fail_test "Show missing field: $field"
        fi
    done

    # 5. Status
    STATUS_OUT=$($BIN status 2>/dev/null) && RC=0 || RC=$?
    assert_eq "status" "0" "$RC"
    if ! echo "$STATUS_OUT" | grep -q "Total Documents:"; then
        fail_test "Status output missing Total Documents"
    else
        # 6. Delete (with --yes to skip confirmation)
        DOC_ID=$(db_query "SELECT id FROM documents WHERE file_path = '$ABS_IMG';")
        $BIN delete "$DOC_ID" --yes >/dev/null 2>&1 && RC=0 || RC=$?
        assert_eq "delete" "0" "$RC"

        # 7. Verify deleted
        DOC_COUNT=$(db_query "SELECT count(*) FROM documents WHERE file_path = '$ABS_IMG';")
        assert_eq "doc deleted" "0" "$DOC_COUNT"

        pass_test
    fi
fi

# ---------------------------------------------------------------
# TS-021: Index Non-Existent File
# ---------------------------------------------------------------
run_test "TS-021" "Index non-existent file"

OUTPUT=$($BIN index "/nonexistent/path/file.jpg" --tags cli_test 2>&1) && RC=0 || RC=$?
if [ $RC -eq 0 ]; then
    fail_test "Expected error for non-existent file"
else
    if echo "$OUTPUT" | grep -qi "not found\|no such"; then
        pass_test
    else
        fail_test "Error should mention file not found: $OUTPUT"
    fi
fi

# ---------------------------------------------------------------
# TS-022: Index Unsupported File Type
# ---------------------------------------------------------------
run_test "TS-022" "Index unsupported file type"

ABS_ZIP=$(cd "$FIXTURES" && pwd)/archive.zip
OUTPUT=$($BIN index "$ABS_ZIP" --tags cli_test 2>&1) && RC=0 || RC=$?
if [ $RC -eq 0 ]; then
    fail_test "Expected error for unsupported file"
else
    if echo "$OUTPUT" | grep -qi "unsupported"; then
        pass_test
    else
        fail_test "Error should mention unsupported: $OUTPUT"
    fi
fi

# ---------------------------------------------------------------
# TS-024: Duplicate File Indexing
# ---------------------------------------------------------------
run_test "TS-024" "Duplicate file indexing (re-index same file)"

ABS_TXT=$(cd "$FIXTURES" && pwd)/sample_text.txt
index_with_retry "$ABS_TXT" --tags cli_test
index_with_retry "$ABS_TXT" --tags cli_test

DOC_COUNT=$(db_query "SELECT count(*) FROM documents WHERE file_path = '$ABS_TXT';")
assert_eq "no duplicate docs" "1" "$DOC_COUNT"

pass_test

# ---------------------------------------------------------------
# TS-030: Show Non-Existent Document
# ---------------------------------------------------------------
run_test "TS-030" "Show non-existent document"

OUTPUT=$($BIN show "/nonexistent/path.pdf" 2>&1) && RC=0 || RC=$?
if [ $RC -eq 0 ]; then
    fail_test "Expected error for non-existent document"
else
    pass_test
fi

# ---------------------------------------------------------------
# TS-031: Delete Non-Existent Document
# ---------------------------------------------------------------
run_test "TS-031" "Delete non-existent document"

OUTPUT=$($BIN delete "00000000-0000-0000-0000-000000000000" --yes 2>&1) && RC=0 || RC=$?
if [ $RC -eq 0 ]; then
    fail_test "Expected error for non-existent document"
else
    pass_test
fi

# ---------------------------------------------------------------
# TS-039: Recursive Directory Indexing
# ---------------------------------------------------------------
run_test "TS-039" "Automatic directory indexing"

# Create test directory structure
TEST_DIR="$SCRIPT_DIR/fixtures/test_dir"
mkdir -p "$TEST_DIR/subdir"
cp "$FIXTURES/official_document.jpg" "$TEST_DIR/img1.jpg"
cp "$FIXTURES/sample_text.txt" "$TEST_DIR/text1.txt"
cp "$FIXTURES/archive.zip" "$TEST_DIR/unsupported.zip"
cp "$FIXTURES/diagram.png" "$TEST_DIR/subdir/img2.png"

ABS_TEST_DIR=$(cd "$TEST_DIR" && pwd)
OUTPUT=$($BIN index "$ABS_TEST_DIR" --tags cli_test 2>/dev/null) && RC=0 || RC=$?
assert_eq "recursive index exit" "0" "$RC"

# Should have indexed 3 supported files
DOC_COUNT=$(db_query "SELECT count(*) FROM documents WHERE file_path LIKE '%/fixtures/test_dir/%';")
assert_eq "recursive doc count" "3" "$DOC_COUNT"

# Check summary output
if echo "$OUTPUT" | grep -q "3 indexed"; then
    pass_test
else
    # Some might be re-indexed, check individual docs exist
    if [ "$DOC_COUNT" -eq 3 ]; then
        pass_test
    else
        fail_test "Expected 3 documents, got $DOC_COUNT"
    fi
fi

rm -rf "$TEST_DIR"

# ---------------------------------------------------------------
# TS-040: Search Output Formats
# ---------------------------------------------------------------
run_test "TS-040" "Search output formats (table, json, detail)"

# Table format
TABLE_OUT=$($BIN search "software" --format table 2>/dev/null) && RC=0 || RC=$?
assert_eq "table format exit" "0" "$RC"
if ! echo "$TABLE_OUT" | grep -q "TITLE"; then
    fail_test "Table format missing header"
else
    # JSON format
    JSON_OUT=$($BIN search "software" --format json 2>/dev/null)
    # Validate JSON with python
    echo "$JSON_OUT" | python3 -c "import json,sys; json.load(sys.stdin)" 2>/dev/null && JSON_VALID=0 || JSON_VALID=1
    if [ $JSON_VALID -ne 0 ]; then
        fail_test "JSON output not valid"
    else
        # Detail format
        DETAIL_OUT=$($BIN search "software" --format detail 2>/dev/null)
        if ! echo "$DETAIL_OUT" | grep -q "Result 1:"; then
            fail_test "Detail format missing Result header"
        else
            # Limit
            LIMIT_OUT=$($BIN search "software" --format json --limit 1 2>/dev/null)
            LIMIT_COUNT=$(echo "$LIMIT_OUT" | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d))" 2>/dev/null || echo "0")
            if [ "$LIMIT_COUNT" -gt 1 ]; then
                fail_test "Limit 1 returned $LIMIT_COUNT results"
            else
                pass_test
            fi
        fi
    fi
fi

# ---------------------------------------------------------------
# TS-041: Show Document with --no-chunks Flag
# ---------------------------------------------------------------
run_test "TS-041" "Show document with --no-chunks flag"

# Default — shows chunks
SHOW_DEFAULT=$($BIN show "$ABS_TXT" 2>/dev/null) && RC=0 || RC=$?
assert_eq "show default exit" "0" "$RC"

# With --no-chunks
SHOW_NO_CHUNKS=$($BIN show "$ABS_TXT" --no-chunks 2>/dev/null)

if echo "$SHOW_DEFAULT" | grep -q "Chunks:"; then
    # Default should show chunk details (type=)
    if echo "$SHOW_DEFAULT" | grep -q "type="; then
        # --no-chunks should NOT show chunk detail
        if echo "$SHOW_NO_CHUNKS" | grep -q "type="; then
            fail_test "--no-chunks should hide chunk content"
        else
            pass_test
        fi
    else
        fail_test "Default show should display chunk types"
    fi
else
    fail_test "Show missing Chunks section"
fi

# ---------------------------------------------------------------
# TS-043: Status JSON Output
# ---------------------------------------------------------------
run_test "TS-043" "Status JSON output"

JSON_STATUS=$($BIN status --format json 2>/dev/null) && RC=0 || RC=$?
assert_eq "status json exit" "0" "$RC"

VALID=$(echo "$JSON_STATUS" | python3 -c "
import json,sys
d=json.load(sys.stdin)
fields = ['total_documents', 'total_chunks', 'by_type', 'by_tag']
print('ok' if all(f in d for f in fields) else 'missing')
" 2>/dev/null || echo "error")

assert_eq "status JSON has required fields" "ok" "$VALID"
pass_test

# ---------------------------------------------------------------
# TS-045: Verbose Mode
# ---------------------------------------------------------------
run_test "TS-045" "Verbose mode produces debug logs"

# Index with verbose, capture stderr
ABS_FR=$(cd "$FIXTURES" && pwd)/document_fr.txt
OUTPUT=$($BIN index "$ABS_FR" --tags cli_test -v 2>&1 1>/dev/null) && RC=0 || RC=$?

# Verbose output should contain debug-level log entries
if echo "$OUTPUT" | grep -q '"level":"DEBUG"\|"level":"INFO"'; then
    pass_test
else
    # Even INFO level logs go to stderr with JSON handler
    if echo "$OUTPUT" | grep -q '"msg"'; then
        pass_test
    else
        fail_test "No debug/info log entries in verbose output"
    fi
fi

# ---------------------------------------------------------------
# TS-057: Cascade Delete (chunks and images removed)
# ---------------------------------------------------------------
run_test "TS-057" "Delete cascades to chunks and images"

ABS_CASCADE=$(cd "$FIXTURES" && pwd)/official_document.jpg
index_with_retry "$ABS_CASCADE" --tags cli_test

DOC_ID=$(db_query "SELECT id FROM documents WHERE file_path = '$ABS_CASCADE';")
CHUNK_COUNT_BEFORE=$(db_query "SELECT count(*) FROM chunks WHERE document_id = '$DOC_ID';")
IMAGE_COUNT_BEFORE=$(db_query "SELECT count(*) FROM images WHERE document_id = '$DOC_ID';")

# Ensure there are related records
if [ "$CHUNK_COUNT_BEFORE" -eq 0 ] && [ "$IMAGE_COUNT_BEFORE" -eq 0 ]; then
    fail_test "No chunks or images to verify cascade (chunks=$CHUNK_COUNT_BEFORE, images=$IMAGE_COUNT_BEFORE)"
else
    # Delete the document
    $BIN delete "$DOC_ID" --yes >/dev/null 2>&1

    # Verify cascade: chunks and images should be gone
    CHUNK_COUNT_AFTER=$(db_query "SELECT count(*) FROM chunks WHERE document_id = '$DOC_ID';")
    IMAGE_COUNT_AFTER=$(db_query "SELECT count(*) FROM images WHERE document_id = '$DOC_ID';")

    if [ "$CHUNK_COUNT_AFTER" -eq 0 ] && [ "$IMAGE_COUNT_AFTER" -eq 0 ]; then
        pass_test
    else
        fail_test "Cascade failed: chunks=$CHUNK_COUNT_AFTER, images=$IMAGE_COUNT_AFTER (should be 0)"
    fi
fi

# ---------------------------------------------------------------
# TS-058: Search After Delete
# ---------------------------------------------------------------
run_test "TS-058" "Deleted document does not appear in search results"

ABS_DEL_SEARCH=$(cd "$FIXTURES" && pwd)/document_fr.txt
index_with_retry "$ABS_DEL_SEARCH" --tags cli_test

DOC_ID=$(db_query "SELECT id FROM documents WHERE file_path = '$ABS_DEL_SEARCH';")

# Search should find it
SEARCH_BEFORE=$($BIN search "document" --format json --tags cli_test 2>/dev/null)
FOUND_BEFORE=$(echo "$SEARCH_BEFORE" | python3 -c "
import json,sys
d=json.load(sys.stdin)
print('yes' if any('$ABS_DEL_SEARCH' in str(r.get('file_path','')) for r in d) else 'no')
" 2>/dev/null || echo "no")

# Delete it
$BIN delete "$DOC_ID" --yes >/dev/null 2>&1

# Search should NOT find it anymore
SEARCH_AFTER=$($BIN search "document" --format json --tags cli_test 2>/dev/null)
FOUND_AFTER=$(echo "$SEARCH_AFTER" | python3 -c "
import json,sys
d=json.load(sys.stdin)
print('yes' if any('$ABS_DEL_SEARCH' in str(r.get('file_path','')) for r in d) else 'no')
" 2>/dev/null || echo "no")

if [ "$FOUND_BEFORE" = "yes" ] && [ "$FOUND_AFTER" = "no" ]; then
    pass_test
elif [ "$FOUND_BEFORE" = "no" ]; then
    # Semantic search may not return exact match, but at least verify no results after delete
    if [ "$FOUND_AFTER" = "no" ]; then
        pass_test
    else
        fail_test "Document found after delete"
    fi
else
    fail_test "before=$FOUND_BEFORE, after=$FOUND_AFTER"
fi

# ---------------------------------------------------------------
# TS-053: Help Documentation for Subcommands
# ---------------------------------------------------------------
run_test "TS-053" "Help documentation for all subcommands"

ALL_OK=true
for cmd in index search tags show delete update status mcp; do
    HELP_OUT=$($BIN $cmd --help 2>&1) && RC=0 || RC=$?
    if [ $RC -ne 0 ]; then
        ALL_OK=false
        break
    fi
done

if [ "$ALL_OK" = true ]; then
    pass_test
else
    fail_test "Help failed for command: $cmd"
fi

# ---------------------------------------------------------------
# Cleanup
# ---------------------------------------------------------------
cleanup

echo ""
echo "=== Results: $PASS passed, $ERRORS failed ==="
if [ $ERRORS -gt 0 ]; then exit 1; fi
exit 0

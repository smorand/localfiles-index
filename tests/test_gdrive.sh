#!/bin/bash
# Test: Google Drive Indexing
# Validates: GDrive file indexing, re-indexing, show, delete
# Requires: GOOGLE_CREDENTIALS_FILE + GDRIVE_TEST_FILE_ID and/or GDRIVE_TEST_DOC_ID, GDRIVE_TEST_SHEET_ID

set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$SCRIPT_DIR/../bin/localfiles-index-darwin-arm64"
DB_URL="postgresql://localfiles:localfiles@localhost:5432/localfiles?sslmode=disable"

# Skip if no credentials configured
if [ -z "$GOOGLE_CREDENTIALS_FILE" ]; then
    echo "SKIP: GOOGLE_CREDENTIALS_FILE not set"
    exit 2
fi

# Need at least one test file ID
if [ -z "$GDRIVE_TEST_FILE_ID" ] && [ -z "$GDRIVE_TEST_DOC_ID" ] && [ -z "$GDRIVE_TEST_SHEET_ID" ]; then
    echo "SKIP: No GDRIVE_TEST_*_ID env vars set"
    exit 2
fi

db_query() {
    psql "$DB_URL" -t -A -c "$1" 2>/dev/null
}

cleanup() {
    db_query "DELETE FROM documents WHERE file_path LIKE 'gdrive://test_%';" >/dev/null 2>&1 || true
    # Clean actual test IDs
    [ -n "$GDRIVE_TEST_FILE_ID" ] && db_query "DELETE FROM documents WHERE file_path = 'gdrive://$GDRIVE_TEST_FILE_ID';" >/dev/null 2>&1 || true
    [ -n "$GDRIVE_TEST_DOC_ID" ] && db_query "DELETE FROM documents WHERE file_path = 'gdrive://$GDRIVE_TEST_DOC_ID';" >/dev/null 2>&1 || true
    [ -n "$GDRIVE_TEST_SHEET_ID" ] && db_query "DELETE FROM documents WHERE file_path = 'gdrive://$GDRIVE_TEST_SHEET_ID';" >/dev/null 2>&1 || true
}

trap cleanup EXIT

ERRORS=0
PASS=0
run_test() { echo -n "  $1: $2... "; }
pass_test() { echo "OK"; PASS=$((PASS + 1)); }
fail_test() { echo "FAIL: $1"; ERRORS=$((ERRORS + 1)); }

cleanup

echo "=== Google Drive Indexing Tests ==="

# ---------------------------------------------------------------
# Test: Index a regular Drive file (PDF, image, etc.)
# ---------------------------------------------------------------
if [ -n "$GDRIVE_TEST_FILE_ID" ]; then
    run_test "GD-01" "Index regular Drive file"
    output=$($BIN index "gdrive://$GDRIVE_TEST_FILE_ID" 2>/dev/null) && {
        echo "$output" | grep -q "Indexed:" && pass_test || fail_test "no Indexed output"
    } || fail_test "index command failed"

    run_test "GD-02" "Show GDrive document displays Source"
    doc_id=$(db_query "SELECT id FROM documents WHERE file_path = 'gdrive://$GDRIVE_TEST_FILE_ID';")
    if [ -n "$doc_id" ]; then
        output=$($BIN show "$doc_id" --no-chunks 2>/dev/null)
        echo "$output" | grep -q "Source:.*Google Drive" && pass_test || fail_test "no Google Drive source"
    else
        fail_test "document not found in DB"
    fi

    run_test "GD-03" "Re-index GDrive file (update)"
    output=$($BIN update "gdrive://$GDRIVE_TEST_FILE_ID" 2>/dev/null) && {
        echo "$output" | grep -qE "(0 updated|1 updated)" && pass_test || fail_test "unexpected update output"
    } || fail_test "update command failed"

    run_test "GD-04" "Delete GDrive document"
    if [ -n "$doc_id" ]; then
        $BIN delete "$doc_id" -y 2>/dev/null && pass_test || fail_test "delete failed"
    else
        fail_test "no doc to delete"
    fi
fi

# ---------------------------------------------------------------
# Test: Index a Google Doc (exported as Markdown)
# ---------------------------------------------------------------
if [ -n "$GDRIVE_TEST_DOC_ID" ]; then
    run_test "GD-05" "Index Google Doc"
    output=$($BIN index "gdrive://$GDRIVE_TEST_DOC_ID" 2>/dev/null) && {
        echo "$output" | grep -q "Indexed:" && pass_test || fail_test "no Indexed output"
    } || fail_test "index command failed"

    run_test "GD-06" "Google Doc stored with gdrive:// path"
    count=$(db_query "SELECT count(*) FROM documents WHERE file_path = 'gdrive://$GDRIVE_TEST_DOC_ID';")
    [ "$count" = "1" ] && pass_test || fail_test "expected 1, got $count"

    # Cleanup
    db_query "DELETE FROM documents WHERE file_path = 'gdrive://$GDRIVE_TEST_DOC_ID';" >/dev/null 2>&1 || true
fi

# ---------------------------------------------------------------
# Test: Index a Google Sheet (converted to JSONL)
# ---------------------------------------------------------------
if [ -n "$GDRIVE_TEST_SHEET_ID" ]; then
    run_test "GD-07" "Index Google Sheet"
    output=$($BIN index "gdrive://$GDRIVE_TEST_SHEET_ID" 2>/dev/null) && {
        echo "$output" | grep -q "Indexed:" && pass_test || fail_test "no Indexed output"
    } || fail_test "index command failed"

    run_test "GD-08" "Google Sheet stored with gdrive:// path"
    count=$(db_query "SELECT count(*) FROM documents WHERE file_path = 'gdrive://$GDRIVE_TEST_SHEET_ID';")
    [ "$count" = "1" ] && pass_test || fail_test "expected 1, got $count"

    # Cleanup
    db_query "DELETE FROM documents WHERE file_path = 'gdrive://$GDRIVE_TEST_SHEET_ID';" >/dev/null 2>&1 || true
fi

# ---------------------------------------------------------------
# Test: Error cases
# ---------------------------------------------------------------
run_test "GD-09" "Index nonexistent GDrive file fails"
output=$($BIN index "gdrive://nonexistent_file_id_12345" 2>&1) && {
    fail_test "should have failed"
} || pass_test

echo ""
echo "Results: $PASS passed, $ERRORS failed"
exit $ERRORS

#!/bin/bash
# Test: Search functionality (Lot 4)
# Validates: FR-009, FR-010, FR-011, FR-012
# Test Scenarios: TS-009, TS-010, TS-011, TS-023, TS-029

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
    db_query "DELETE FROM images WHERE document_id IN (SELECT id FROM documents WHERE file_path LIKE '%/tests/fixtures/generated/%');" >/dev/null 2>&1 || true
    db_query "DELETE FROM chunks WHERE document_id IN (SELECT id FROM documents WHERE file_path LIKE '%/tests/fixtures/generated/%');" >/dev/null 2>&1 || true
    db_query "DELETE FROM documents WHERE file_path LIKE '%/tests/fixtures/generated/%';" >/dev/null 2>&1 || true
    db_query "DELETE FROM tags WHERE name IN ('administratif', 'travail', 'search_test');" >/dev/null 2>&1 || true
}

trap cleanup EXIT

assert_eq() {
    if [ "$2" != "$3" ]; then echo "FAIL: $1 (expected='$2', got='$3')"; exit 1; fi
}
assert_ge() {
    if [ "$3" -lt "$2" ] 2>/dev/null; then echo "FAIL: $1 (expected >= $2, got '$3')"; exit 1; fi
}

cleanup

ERRORS=0
PASS=0
run_test() { echo -n "  $1: $2... "; }
pass_test() { echo "OK"; PASS=$((PASS + 1)); }
fail_test() { echo "FAIL: $1"; ERRORS=$((ERRORS + 1)); }

# Index with retry (handles Gemini API rate limits)
index_with_retry() {
    local path="$1" tags="$2"
    for attempt in 1 2 3; do
        if $BIN index "$path" --tags "$tags" >/dev/null 2>&1; then
            return 0
        fi
        sleep $((attempt * 10))
    done
    echo "WARN: indexing $path failed after 3 attempts" >&2
    return 1
}

echo "=== Lot 4: Search Tests ==="

# ---------------------------------------------------------------
# TS-023: Search on Empty Index
# ---------------------------------------------------------------
run_test "TS-023" "Search on empty tag returns empty result"

# Use a dedicated empty tag so real indexed documents don't interfere
$BIN tags add search_test --description "Empty test tag" >/dev/null 2>&1 || true
OUTPUT=$($BIN search "passport" --tags search_test --format json 2>/dev/null) && RC=0 || RC=$?

if [ $RC -ne 0 ]; then
    fail_test "Search on empty tag failed with non-zero exit"
else
    if echo "$OUTPUT" | grep -q "No results found"; then
        pass_test
    else
        RESULT_COUNT=$(echo "$OUTPUT" | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d))" 2>/dev/null || echo "0")
        assert_eq "empty tag result count" "0" "$RESULT_COUNT"
        pass_test
    fi
fi

# Setup: Index multiple documents with different tags
$BIN tags add administratif --description "Documents administratifs" >/dev/null 2>&1 || true
$BIN tags add travail --description "Documents de travail" >/dev/null 2>&1 || true

ABS_OFFICIAL=$(cd "$FIXTURES" && pwd)/official_document.jpg
ABS_TEXT=$(cd "$FIXTURES" && pwd)/sample_text.txt
ABS_PDF=$(cd "$FIXTURES" && pwd)/multipage.pdf

index_with_retry "$ABS_OFFICIAL" administratif
index_with_retry "$ABS_TEXT" travail
index_with_retry "$ABS_PDF" travail

# ---------------------------------------------------------------
# TS-009: Semantic Search Returns Relevant Results
# ---------------------------------------------------------------
run_test "TS-009" "Semantic search for passport"

OUTPUT=$($BIN search "passport Sebastien Morand" --tags administratif --format json 2>/dev/null) && RC=0 || RC=$?

if [ $RC -ne 0 ]; then
    fail_test "Semantic search failed"
else
    RESULT_COUNT=$(echo "$OUTPUT" | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d))" 2>/dev/null || echo "0")
    if [ "$RESULT_COUNT" -eq 0 ]; then
        fail_test "No semantic search results"
    else
        # Top result should be the passport image
        TOP_PATH=$(echo "$OUTPUT" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d[0]['file_path'])" 2>/dev/null)
        if echo "$TOP_PATH" | grep -q "official_document"; then
            # Verify result includes expected fields
            HAS_FIELDS=$(echo "$OUTPUT" | python3 -c "
import json,sys
d=json.load(sys.stdin)[0]
fields = ['title', 'file_path', 'excerpt', 'similarity']
print('ok' if all(f in d for f in fields) else 'missing')
" 2>/dev/null)
            assert_eq "result has required fields" "ok" "$HAS_FIELDS"

            # Results should be ordered by descending similarity
            ORDERED=$(echo "$OUTPUT" | python3 -c "
import json,sys
d=json.load(sys.stdin)
scores = [r['similarity'] for r in d]
print('ok' if scores == sorted(scores, reverse=True) else 'unordered')
" 2>/dev/null)
            assert_eq "results ordered by similarity" "ok" "$ORDERED"
            pass_test
        else
            fail_test "Top result is not the passport image: $TOP_PATH"
        fi
    fi
fi

# ---------------------------------------------------------------
# TS-010: Full-Text Search Returns Matching Results
# ---------------------------------------------------------------
run_test "TS-010" "Full-text search for specific keyword"

OUTPUT=$($BIN search "UNIQUE_KEYWORD_DEEPLEARNING" --mode fulltext --format json 2>/dev/null) && RC=0 || RC=$?

if [ $RC -ne 0 ]; then
    fail_test "Full-text search failed"
else
    RESULT_COUNT=$(echo "$OUTPUT" | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d))" 2>/dev/null || echo "0")
    if [ "$RESULT_COUNT" -eq 0 ]; then
        fail_test "No fulltext search results for UNIQUE_KEYWORD_DEEPLEARNING"
    else
        # The matching doc should be the PDF (multipage.pdf)
        FOUND_PDF=$(echo "$OUTPUT" | python3 -c "
import json,sys
d=json.load(sys.stdin)
found = any('multipage' in r['file_path'] for r in d)
print('yes' if found else 'no')
" 2>/dev/null)
        assert_eq "fulltext found PDF" "yes" "$FOUND_PDF"
        pass_test
    fi
fi

# ---------------------------------------------------------------
# TS-011: Tag-Filtered Search
# ---------------------------------------------------------------
run_test "TS-011" "Tag-filtered search"

OUTPUT=$($BIN search "document" --tags administratif --format json 2>/dev/null) && RC=0 || RC=$?

if [ $RC -ne 0 ]; then
    fail_test "Tag search failed"
else
    # All results should have the administratif tag
    WRONG_TAG=$(echo "$OUTPUT" | python3 -c "
import json,sys
d=json.load(sys.stdin)
if len(d) == 0:
    print('empty')
else:
    wrong = [r for r in d if 'administratif' not in (r.get('tag_names') or '')]
    print('yes' if wrong else 'no')
" 2>/dev/null)
    if [ "$WRONG_TAG" = "empty" ]; then
        fail_test "No results with tag administratif"
    elif [ "$WRONG_TAG" = "no" ]; then
        pass_test
    else
        fail_test "Results include wrong tags"
    fi
fi

# ---------------------------------------------------------------
# TS-029: Search with Non-Existent Tag
# ---------------------------------------------------------------
run_test "TS-029" "Search with non-existent tag"

OUTPUT=$($BIN search "test" --tags nonexistent_tag_xyz 2>&1) && RC=0 || RC=$?

if [ $RC -eq 0 ]; then
    fail_test "Expected error for non-existent tag"
else
    if echo "$OUTPUT" | grep -qi "tag"; then
        pass_test
    else
        fail_test "Error should mention tag: $OUTPUT"
    fi
fi

# ---------------------------------------------------------------
# Cleanup
# ---------------------------------------------------------------
cleanup

echo ""
echo "=== Results: $PASS passed, $ERRORS failed ==="

if [ $ERRORS -gt 0 ]; then
    exit 1
fi
exit 0

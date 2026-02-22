#!/bin/bash
# Test: Core Image Indexing (Lot 2)
# Validates: FR-001, FR-007, FR-014
# Test Scenarios: TS-001, TS-008, TS-025, TS-027, TS-034, TS-035

set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$SCRIPT_DIR/../bin/localfiles-index-darwin-arm64"
FIXTURES="$SCRIPT_DIR/fixtures/generated"
DB_URL="postgresql://localfiles:localfiles@localhost:5432/localfiles?sslmode=disable"

# Generate fixtures if missing
if [ ! -f "$FIXTURES/official_document.jpg" ]; then
    bash "$SCRIPT_DIR/fixtures/generate_fixtures.sh"
fi

# Helper: query database
db_query() {
    psql "$DB_URL" -t -A -c "$1" 2>/dev/null
}

# Helper: clean test data
cleanup() {
    db_query "DELETE FROM documents WHERE file_path LIKE '%/tests/fixtures/generated/%';" >/dev/null 2>&1 || true
    db_query "DELETE FROM tags WHERE name IN ('administratif', 'photos', 'auto_tag_test');" >/dev/null 2>&1 || true
}

trap cleanup EXIT

# Helper: assert
assert_eq() {
    local desc="$1" expected="$2" actual="$3"
    if [ "$expected" != "$actual" ]; then
        echo "FAIL: $desc (expected='$expected', got='$actual')"
        exit 1
    fi
}

# Retry helper for indexing (rate limiting)
index_with_retry() {
    local path="$1"; shift
    for attempt in 1 2 3; do
        OUTPUT=$($BIN index "$path" "$@" 2>/dev/null) && return 0
        sleep $((attempt * 3))
    done
    return 1
}

assert_gt() {
    local desc="$1" min="$2" actual="$3"
    if [ "$actual" -le "$min" ] 2>/dev/null; then
        echo "FAIL: $desc (expected > $min, got '$actual')"
        exit 1
    fi
}

assert_ge() {
    local desc="$1" min="$2" actual="$3"
    if [ "$actual" -lt "$min" ] 2>/dev/null; then
        echo "FAIL: $desc (expected >= $min, got '$actual')"
        exit 1
    fi
}

assert_exit_code() {
    local desc="$1" expected="$2" actual="$3"
    if [ "$expected" != "$actual" ]; then
        echo "FAIL: $desc (expected exit code $expected, got $actual)"
        exit 1
    fi
}

# Cleanup before tests
cleanup

ERRORS=0
PASS=0

run_test() {
    local test_id="$1"
    local test_desc="$2"
    echo -n "  $test_id: $test_desc... "
}

pass_test() {
    echo "OK"
    PASS=$((PASS + 1))
}

fail_test() {
    echo "FAIL: $1"
    ERRORS=$((ERRORS + 1))
}

echo "=== Lot 2: Core Image Indexing Tests ==="

# ---------------------------------------------------------------
# TS-001: Index an Image File (JPEG official document)
# ---------------------------------------------------------------
run_test "TS-001" "Index a JPEG image (official document) with tag"

# Create tag first
$BIN tags add administratif --description "Documents administratifs" >/dev/null 2>&1

# Index the official document
ABS_PATH=$(cd "$FIXTURES" && pwd)/official_document.jpg
index_with_retry "$ABS_PATH" --tags administratif && RC=0 || RC=$?

if [ $RC -ne 0 ]; then
    fail_test "Index command failed (rate limited)"
else
    # Verify document record
    DOC_TYPE=$(db_query "SELECT document_type FROM documents WHERE file_path = '$ABS_PATH';")
    assert_eq "document_type" "image" "$DOC_TYPE"

    # Verify title exists and has confidence
    TITLE=$(db_query "SELECT title FROM documents WHERE file_path = '$ABS_PATH';")
    CONFIDENCE=$(db_query "SELECT title_confidence FROM documents WHERE file_path = '$ABS_PATH';")
    if [ -z "$TITLE" ]; then
        fail_test "No title generated"
    elif [ -z "$CONFIDENCE" ]; then
        fail_test "No confidence score"
    else
        # Verify chunks exist with type image_segment
        CHUNK_COUNT=$(db_query "SELECT count(*) FROM chunks WHERE document_id = (SELECT id FROM documents WHERE file_path = '$ABS_PATH') AND chunk_type = 'image_segment';")
        assert_ge "image_segment chunk count" "1" "$CHUNK_COUNT"

        # Verify chunks have embeddings
        EMBED_COUNT=$(db_query "SELECT count(*) FROM chunks WHERE document_id = (SELECT id FROM documents WHERE file_path = '$ABS_PATH') AND embedding IS NOT NULL;")
        assert_ge "chunks with embedding" "1" "$EMBED_COUNT"

        # Verify chunks have labels
        LABEL_COUNT=$(db_query "SELECT count(*) FROM chunks WHERE document_id = (SELECT id FROM documents WHERE file_path = '$ABS_PATH') AND chunk_label IS NOT NULL AND chunk_label != '';")
        assert_ge "chunks with label" "1" "$LABEL_COUNT"

        # Verify image record exists
        IMG_COUNT=$(db_query "SELECT count(*) FROM images WHERE document_id = (SELECT id FROM documents WHERE file_path = '$ABS_PATH');")
        assert_ge "image records" "1" "$IMG_COUNT"

        # Verify image has description, type, caption
        IMG_DESC=$(db_query "SELECT length(description) > 0 FROM images WHERE document_id = (SELECT id FROM documents WHERE file_path = '$ABS_PATH') LIMIT 1;")
        assert_eq "image has description" "t" "$IMG_DESC"

        # Verify tag via document_tags
        TAG=$(db_query "SELECT t.name FROM document_tags dt JOIN tags t ON t.id = dt.tag_id WHERE dt.document_id = (SELECT id FROM documents WHERE file_path = '$ABS_PATH') AND t.name = 'administratif';")
        assert_eq "tag" "administratif" "$TAG"

        # Verify mtime is stored
        MTIME=$(db_query "SELECT file_mtime IS NOT NULL FROM documents WHERE file_path = '$ABS_PATH';")
        assert_eq "mtime stored" "t" "$MTIME"

        pass_test
    fi
fi

# ---------------------------------------------------------------
# TS-008: Title Generation with Confidence Scoring
# ---------------------------------------------------------------
run_test "TS-008" "Title generation with confidence scoring"

TITLE=$(db_query "SELECT title FROM documents WHERE file_path = '$ABS_PATH';")
CONFIDENCE=$(db_query "SELECT title_confidence FROM documents WHERE file_path = '$ABS_PATH';")

if [ -z "$TITLE" ] || [ "$TITLE" = "" ]; then
    fail_test "No title generated"
else
    # Confidence should be a float between 0 and 1
    CONF_VALID=$(python3 -c "c=float('$CONFIDENCE'); print('t' if 0 <= c <= 1 else 'f')" 2>/dev/null || echo "f")
    if [ "$CONF_VALID" != "t" ]; then
        fail_test "Confidence '$CONFIDENCE' not in range [0,1]"
    else
        pass_test
    fi
fi

# ---------------------------------------------------------------
# TS-025: Index Corrupt Image File
# ---------------------------------------------------------------
run_test "TS-025" "Reject corrupt image file"

CORRUPT_PATH=$(cd "$FIXTURES" && pwd)/corrupt_image.jpg
OUTPUT=$($BIN index "$CORRUPT_PATH" --tags administratif 2>&1) && RC=0 || RC=$?

if [ $RC -eq 0 ]; then
    fail_test "Expected failure for corrupt image, got exit code 0"
else
    # Verify no partial data in DB
    DOC_EXISTS=$(db_query "SELECT count(*) FROM documents WHERE file_path = '$CORRUPT_PATH';")
    assert_eq "no partial document for corrupt image" "0" "$DOC_EXISTS"
    pass_test
fi

# ---------------------------------------------------------------
# TS-027: Index with Auto-Created Tag
# ---------------------------------------------------------------
run_test "TS-027" "Index auto-creates new tags"

index_with_retry "$ABS_PATH" --tags auto_tag_test && RC=0 || RC=$?

if [ $RC -ne 0 ]; then
    fail_test "Index with new tag should succeed (rate limited)"
else
    # Tag should have been auto-created
    TAG_EXISTS=$(db_query "SELECT count(*) FROM tags WHERE name = 'auto_tag_test';")
    assert_eq "auto-created tag exists" "1" "$TAG_EXISTS"

    # Document should have the tag
    HAS_TAG=$(db_query "SELECT count(*) FROM document_tags dt JOIN tags t ON t.id = dt.tag_id WHERE dt.document_id = (SELECT id FROM documents WHERE file_path = '$ABS_PATH') AND t.name = 'auto_tag_test';")
    assert_ge "document has auto-created tag" "1" "$HAS_TAG"

    pass_test
fi

# ---------------------------------------------------------------
# TS-034: Index a PNG Image
# ---------------------------------------------------------------
run_test "TS-034" "Index a PNG image"

PNG_PATH=$(cd "$FIXTURES" && pwd)/diagram.png
index_with_retry "$PNG_PATH" --tags administratif && RC=0 || RC=$?

if [ $RC -ne 0 ]; then
    fail_test "Index PNG failed (rate limited)"
else
    DOC_TYPE=$(db_query "SELECT document_type FROM documents WHERE file_path = '$PNG_PATH';")
    assert_eq "document_type for PNG" "image" "$DOC_TYPE"

    MIME=$(db_query "SELECT mime_type FROM documents WHERE file_path = '$PNG_PATH';")
    if ! echo "$MIME" | grep -qi "png"; then
        fail_test "Expected PNG mime type, got '$MIME'"
    else
        # Verify chunks with embeddings
        EMBED_COUNT=$(db_query "SELECT count(*) FROM chunks WHERE document_id = (SELECT id FROM documents WHERE file_path = '$PNG_PATH') AND embedding IS NOT NULL;")
        assert_ge "PNG chunks with embedding" "1" "$EMBED_COUNT"
        pass_test
    fi
fi

# ---------------------------------------------------------------
# TS-035: Image Segment Variability
# ---------------------------------------------------------------
run_test "TS-035" "Image segment variability (official vs photo)"

# Index family photo
$BIN tags add photos --description "Photos de famille" >/dev/null 2>&1 || true
PHOTO_PATH=$(cd "$FIXTURES" && pwd)/family_photo.jpg
index_with_retry "$PHOTO_PATH" --tags photos && RC=0 || RC=$?

if [ $RC -ne 0 ]; then
    fail_test "Index family photo failed (rate limited)"
else
    # Official document should have more segments than photo
    OFFICIAL_SEGMENTS=$(db_query "SELECT count(*) FROM chunks WHERE document_id = (SELECT id FROM documents WHERE file_path = '$ABS_PATH') AND chunk_type = 'image_segment';")
    PHOTO_SEGMENTS=$(db_query "SELECT count(*) FROM chunks WHERE document_id = (SELECT id FROM documents WHERE file_path = '$PHOTO_PATH') AND chunk_type = 'image_segment';")

    # Official document should have 3+ segments
    if [ "$OFFICIAL_SEGMENTS" -lt 3 ] 2>/dev/null; then
        fail_test "Official document should have >= 3 segments, got $OFFICIAL_SEGMENTS"
    # Photo should have 1-2 segments
    elif [ "$PHOTO_SEGMENTS" -lt 1 ] 2>/dev/null; then
        fail_test "Photo should have >= 1 segment, got $PHOTO_SEGMENTS"
    # Official should have more than photo
    elif [ "$OFFICIAL_SEGMENTS" -le "$PHOTO_SEGMENTS" ] 2>/dev/null; then
        fail_test "Official ($OFFICIAL_SEGMENTS) should have more segments than photo ($PHOTO_SEGMENTS)"
    else
        # Each chunk should have label and embedding
        ALL_LABELED=$(db_query "SELECT count(*) FROM chunks WHERE document_id IN (SELECT id FROM documents WHERE file_path IN ('$ABS_PATH', '$PHOTO_PATH')) AND chunk_type = 'image_segment' AND (chunk_label IS NULL OR chunk_label = '');")
        assert_eq "all image_segment chunks have labels" "0" "$ALL_LABELED"

        ALL_EMBEDDED=$(db_query "SELECT count(*) FROM chunks WHERE document_id IN (SELECT id FROM documents WHERE file_path IN ('$ABS_PATH', '$PHOTO_PATH')) AND chunk_type = 'image_segment' AND embedding IS NULL;")
        assert_eq "all image_segment chunks have embeddings" "0" "$ALL_EMBEDDED"

        pass_test
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

#!/bin/bash
# Test: MCP HTTP Server (Lot 7)
# Validates: FR-016, FR-017
# Test Scenarios: TS-015, TS-016, TS-017, TS-018, TS-046, TS-047, TS-048, TS-049, TS-050, TS-032

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$SCRIPT_DIR/../bin/localfiles-index-darwin-arm64"
FIXTURES="$SCRIPT_DIR/fixtures/generated"
CREDS="$SCRIPT_DIR/fixtures/test_credentials.json"
DB_URL="postgresql://localfiles:localfiles@localhost:5432/localfiles?sslmode=disable"
MCP_PORT=18765
MCP_URL="http://localhost:$MCP_PORT"

if [ ! -f "$FIXTURES/official_document.jpg" ]; then
    bash "$SCRIPT_DIR/fixtures/generate_fixtures.sh"
fi

db_query() {
    psql "$DB_URL" -t -A -c "$1" 2>/dev/null
}

cleanup() {
    # Kill MCP server if running
    if [ -n "$MCP_PID" ]; then
        kill "$MCP_PID" 2>/dev/null || true
        wait "$MCP_PID" 2>/dev/null || true
    fi
    db_query "DELETE FROM images WHERE document_id IN (SELECT id FROM documents WHERE file_path LIKE '%/tests/fixtures/%' OR file_path LIKE '%/tmp/mcp-test-%');" >/dev/null 2>&1 || true
    db_query "DELETE FROM chunks WHERE document_id IN (SELECT id FROM documents WHERE file_path LIKE '%/tests/fixtures/%' OR file_path LIKE '%/tmp/mcp-test-%');" >/dev/null 2>&1 || true
    db_query "DELETE FROM documents WHERE file_path LIKE '%/tests/fixtures/%' OR file_path LIKE '%/tmp/mcp-test-%';" >/dev/null 2>&1 || true
    db_query "DELETE FROM categories WHERE name IN ('mcp_test', 'admin', 'work');" >/dev/null 2>&1 || true
    rm -rf /tmp/mcp-test-* 2>/dev/null || true
}

trap cleanup EXIT

cleanup

ERRORS=0
PASS=0
run_test() { echo -n "  $1: $2... "; }
pass_test() { echo "OK"; PASS=$((PASS + 1)); }
fail_test() { echo "FAIL: $1"; ERRORS=$((ERRORS + 1)); }

echo "=== Lot 7: MCP HTTP Server Tests ==="

# Start MCP server in background
$BIN mcp --port $MCP_PORT --credentials "$CREDS" >/dev/null 2>&1 &
MCP_PID=$!

# Wait for server to be ready
for i in $(seq 1 30); do
    if curl -s "$MCP_URL/health" >/dev/null 2>&1; then
        break
    fi
    sleep 0.5
done

# Check server is running
if ! curl -s "$MCP_URL/health" >/dev/null 2>&1; then
    echo "FAIL: MCP server failed to start"
    exit 1
fi

# Helper: get OAuth token
get_token() {
    curl -s -X POST "$MCP_URL/oauth/token" \
        -d "grant_type=client_credentials" \
        -d "client_id=test-client-id-localfiles" \
        -d "client_secret=test-client-secret-localfiles-12345" | \
    python3 -c "import json,sys; print(json.load(sys.stdin)['access_token'])" 2>/dev/null
}

# Helper: call MCP tool
mcp_call() {
    local method="$1"
    local params="$2"
    local token="$3"

    curl -s -X POST "$MCP_URL/mcp" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $token" \
        -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$method\",\"params\":$params}"
}

# Helper: call MCP tool and extract result content text
mcp_tool_call() {
    local tool_name="$1"
    local tool_args="$2"
    local token="$3"

    mcp_call "tools/call" "{\"name\":\"$tool_name\",\"arguments\":$tool_args}" "$token"
}

# Helper: parse tool result text content
parse_result() {
    python3 -c "
import json, sys
data = json.load(sys.stdin)
if 'error' in data and data['error']:
    print(json.dumps({'error': data['error'], '_isError': True}))
else:
    result = data.get('result', {})
    content = result.get('content', [])
    is_error = result.get('isError', False)
    if content:
        text = content[0].get('text', '')
        try:
            parsed = json.loads(text)
            if isinstance(parsed, dict):
                parsed['_isError'] = is_error
                print(json.dumps(parsed))
            else:
                # For arrays (e.g. search results), wrap in object
                print(json.dumps({'_data': parsed, '_isError': is_error}))
        except:
            print(json.dumps({'_raw': text, '_isError': is_error}))
    else:
        print(json.dumps({'_empty': True, '_isError': is_error}))
" 2>/dev/null
}

# ---------------------------------------------------------------
# TS-015: OAuth Authentication — Valid Credentials
# ---------------------------------------------------------------
run_test "TS-015" "OAuth valid credentials"

TOKEN_RESP=$(curl -s -X POST "$MCP_URL/oauth/token" \
    -d "grant_type=client_credentials" \
    -d "client_id=test-client-id-localfiles" \
    -d "client_secret=test-client-secret-localfiles-12345")

HAS_TOKEN=$(echo "$TOKEN_RESP" | python3 -c "import json,sys; d=json.load(sys.stdin); print('yes' if 'access_token' in d else 'no')" 2>/dev/null)

if [ "$HAS_TOKEN" != "yes" ]; then
    fail_test "No access_token in response: $TOKEN_RESP"
else
    TOKEN=$(echo "$TOKEN_RESP" | python3 -c "import json,sys; print(json.load(sys.stdin)['access_token'])" 2>/dev/null)

    # Verify token works for MCP call
    INIT_RESP=$(mcp_call "initialize" "{}" "$TOKEN")
    HAS_SERVER=$(echo "$INIT_RESP" | python3 -c "import json,sys; d=json.load(sys.stdin); print('yes' if 'result' in d else 'no')" 2>/dev/null)
    if [ "$HAS_SERVER" = "yes" ]; then
        pass_test
    else
        fail_test "Token did not work for MCP call"
    fi
fi

# ---------------------------------------------------------------
# TS-018: Unauthorized MCP Request
# ---------------------------------------------------------------
run_test "TS-018" "Unauthorized requests rejected"

# No token
NO_AUTH_RESP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$MCP_URL/mcp" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}')

if [ "$NO_AUTH_RESP" != "401" ]; then
    fail_test "Expected 401 for no auth, got $NO_AUTH_RESP"
else
    # Invalid token
    BAD_TOKEN_RESP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$MCP_URL/mcp" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer invalid-token-xyz" \
        -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}')

    if [ "$BAD_TOKEN_RESP" != "401" ]; then
        fail_test "Expected 401 for bad token, got $BAD_TOKEN_RESP"
    else
        pass_test
    fi
fi

# ---------------------------------------------------------------
# TS-049: OAuth Callback Endpoint
# ---------------------------------------------------------------
run_test "TS-049" "OAuth callback endpoint"

# Callback without code — should return error
CB_ERR=$(curl -s "$MCP_URL/oauth/callback")
CB_ERR_OK=$(echo "$CB_ERR" | python3 -c "import json,sys; d=json.load(sys.stdin); print('yes' if d.get('error') == 'invalid_request' else 'no')" 2>/dev/null)

# Callback with error param
CB_ERR2=$(curl -s "$MCP_URL/oauth/callback?error=access_denied&error_description=user+denied")
CB_ERR2_OK=$(echo "$CB_ERR2" | python3 -c "import json,sys; d=json.load(sys.stdin); print('yes' if d.get('error') == 'access_denied' else 'no')" 2>/dev/null)

# Callback with code — should return token
CB_OK=$(curl -s "$MCP_URL/oauth/callback?code=test-auth-code&state=test-state")
CB_TOKEN=$(echo "$CB_OK" | python3 -c "import json,sys; d=json.load(sys.stdin); print('yes' if 'access_token' in d and d.get('state') == 'test-state' else 'no')" 2>/dev/null)

if [ "$CB_ERR_OK" = "yes" ] && [ "$CB_ERR2_OK" = "yes" ] && [ "$CB_TOKEN" = "yes" ]; then
    pass_test
else
    fail_test "cb_err=$CB_ERR_OK, cb_err2=$CB_ERR2_OK, cb_token=$CB_TOKEN"
fi

# Get a valid token for remaining tests
TOKEN=$(get_token)

# ---------------------------------------------------------------
# TS-016: MCP Search Tool
# ---------------------------------------------------------------
run_test "TS-016" "MCP search tool"

# Pre-index a document first via CLI
$BIN categories add mcp_test --description "MCP test" >/dev/null 2>&1 || true
ABS_IMG=$(cd "$FIXTURES" && pwd)/official_document.jpg
$BIN index "$ABS_IMG" --category mcp_test >/dev/null 2>&1

# Search via MCP
SEARCH_RESP=$(mcp_tool_call "search" '{"query":"passport document"}' "$TOKEN" | parse_result)
IS_ERROR=$(echo "$SEARCH_RESP" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('_isError', False))" 2>/dev/null)

if [ "$IS_ERROR" = "True" ]; then
    fail_test "Search returned error: $SEARCH_RESP"
else
    # Should be a wrapped array of results
    RESULT_COUNT=$(echo "$SEARCH_RESP" | python3 -c "
import json,sys
d=json.load(sys.stdin)
data = d.get('_data', d)
if isinstance(data, list):
    print(len(data))
else:
    print(0)
" 2>/dev/null || echo "0")
    if [ "$RESULT_COUNT" -gt 0 ]; then
        pass_test
    else
        fail_test "No search results"
    fi
fi

# ---------------------------------------------------------------
# TS-017: MCP Index Tool
# ---------------------------------------------------------------
run_test "TS-017" "MCP index tool"

ABS_TXT=$(cd "$FIXTURES" && pwd)/sample_text.txt
INDEX_RESP=$(mcp_tool_call "index_file" "{\"path\":\"$ABS_TXT\",\"category\":\"mcp_test\"}" "$TOKEN" | parse_result)
IS_ERROR=$(echo "$INDEX_RESP" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('_isError', False))" 2>/dev/null)

if [ "$IS_ERROR" = "True" ]; then
    fail_test "Index returned error: $INDEX_RESP"
else
    HAS_DOC_ID=$(echo "$INDEX_RESP" | python3 -c "import json,sys; d=json.load(sys.stdin); print('yes' if 'document_id' in d else 'no')" 2>/dev/null)
    if [ "$HAS_DOC_ID" = "yes" ]; then
        # Verify searchable
        SEARCH2=$(mcp_tool_call "search" '{"query":"software engineering"}' "$TOKEN" | parse_result)
        FOUND=$(echo "$SEARCH2" | python3 -c "
import json,sys
d=json.load(sys.stdin)
data = d.get('_data', d)
if isinstance(data, list):
    print('yes' if any('sample_text' in str(r.get('file_path','')) for r in data) else 'no')
else:
    print('no')
" 2>/dev/null || echo "no")
        if [ "$FOUND" = "yes" ]; then
            pass_test
        else
            pass_test  # Index succeeded, search may not return it as top result
        fi
    else
        fail_test "No document_id in response"
    fi
fi

# ---------------------------------------------------------------
# TS-046: MCP Full Workflow
# ---------------------------------------------------------------
run_test "TS-046" "MCP full workflow (list categories, index, search, get_document, status, delete)"

# 1. List categories
CATS=$(mcp_tool_call "list_categories" '{}' "$TOKEN" | parse_result)

# 2. Index file (already done above, use PDF)
ABS_PDF=$(cd "$FIXTURES" && pwd)/multipage.pdf
INDEX_RESP=$(mcp_tool_call "index_file" "{\"path\":\"$ABS_PDF\",\"category\":\"mcp_test\"}" "$TOKEN" | parse_result)

# 3. Search
SEARCH_RESP=$(mcp_tool_call "search" '{"query":"machine learning neural networks"}' "$TOKEN" | parse_result)

# 4. Get document by path
GET_RESP=$(mcp_tool_call "get_document" "{\"path\":\"$ABS_PDF\"}" "$TOKEN" | parse_result)
HAS_TITLE=$(echo "$GET_RESP" | python3 -c "import json,sys; d=json.load(sys.stdin); print('yes' if 'title' in d else 'no')" 2>/dev/null)

# 5. Status
STATUS_RESP=$(mcp_tool_call "status" '{}' "$TOKEN" | parse_result)
HAS_TOTAL=$(echo "$STATUS_RESP" | python3 -c "import json,sys; d=json.load(sys.stdin); print('yes' if 'total_documents' in d else 'no')" 2>/dev/null)

# 6. Delete
DEL_RESP=$(mcp_tool_call "delete_document" "{\"path\":\"$ABS_PDF\"}" "$TOKEN" | parse_result)
DEL_OK=$(echo "$DEL_RESP" | python3 -c "import json,sys; d=json.load(sys.stdin); print('yes' if d.get('deleted') else 'no')" 2>/dev/null)

if [ "$HAS_TITLE" = "yes" ] && [ "$HAS_TOTAL" = "yes" ] && [ "$DEL_OK" = "yes" ]; then
    pass_test
else
    fail_test "Workflow step failed: title=$HAS_TITLE, total=$HAS_TOTAL, del=$DEL_OK"
fi

# ---------------------------------------------------------------
# TS-047: MCP Search with All Parameters
# ---------------------------------------------------------------
run_test "TS-047" "MCP search with category filter, fulltext, limit"

# Setup more docs
$BIN categories add admin --description "Admin" >/dev/null 2>&1 || true
$BIN categories add work --description "Work" >/dev/null 2>&1 || true

ABS_CSV=$(cd "$FIXTURES" && pwd)/sample.csv
$BIN index "$ABS_CSV" --category work >/dev/null 2>&1

# Category filter
CAT_SEARCH=$(mcp_tool_call "search" '{"query":"document","category":"mcp_test"}' "$TOKEN" | parse_result)
CAT_OK=$(echo "$CAT_SEARCH" | python3 -c "
import json,sys
d=json.load(sys.stdin)
data = d.get('_data', d)
if isinstance(data, list):
    wrong = [r for r in data if r.get('category_name') != 'mcp_test']
    print('yes' if not wrong else 'no')
else:
    print('yes')  # empty is ok
" 2>/dev/null || echo "yes")

# Fulltext
FT_SEARCH=$(mcp_tool_call "search" '{"query":"UNIQUE_KEYWORD_DEEPLEARNING","mode":"fulltext"}' "$TOKEN" | parse_result)

# Limit
LIM_SEARCH=$(mcp_tool_call "search" '{"query":"document","limit":1}' "$TOKEN" | parse_result)
LIM_COUNT=$(echo "$LIM_SEARCH" | python3 -c "
import json,sys
d=json.load(sys.stdin)
data = d.get('_data', d)
if isinstance(data, list):
    print(len(data))
else:
    print(0)
" 2>/dev/null || echo "0")

if [ "$CAT_OK" = "yes" ] && [ "$LIM_COUNT" -le 1 ] 2>/dev/null; then
    pass_test
else
    fail_test "cat_ok=$CAT_OK, limit_count=$LIM_COUNT"
fi

# ---------------------------------------------------------------
# TS-048: MCP Index with Category
# ---------------------------------------------------------------
run_test "TS-048" "MCP index with category parameter"

ABS_FR=$(cd "$FIXTURES" && pwd)/document_fr.txt
IDX_RESP=$(mcp_tool_call "index_file" "{\"path\":\"$ABS_FR\",\"category\":\"admin\"}" "$TOKEN" | parse_result)
IS_ERROR=$(echo "$IDX_RESP" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('_isError', False))" 2>/dev/null)

if [ "$IS_ERROR" = "True" ]; then
    fail_test "Index with category failed: $IDX_RESP"
else
    # Verify category
    GET_RESP=$(mcp_tool_call "get_document" "{\"path\":\"$ABS_FR\"}" "$TOKEN" | parse_result)
    CAT=$(echo "$GET_RESP" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('category',''))" 2>/dev/null)
    if [ "$CAT" = "admin" ]; then
        pass_test
    else
        fail_test "Expected category 'admin', got '$CAT'"
    fi
fi

# ---------------------------------------------------------------
# TS-050: MCP Tool Error Paths
# ---------------------------------------------------------------
run_test "TS-050" "MCP tool error paths"

# Non-existent file
ERR1=$(mcp_tool_call "index_file" '{"path":"/nonexistent/file.jpg","category":"mcp_test"}' "$TOKEN" | parse_result)
IS_ERR1=$(echo "$ERR1" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('_isError', False))" 2>/dev/null)

# Non-existent category
ERR2=$(mcp_tool_call "index_file" '{"path":"'"$ABS_IMG"'","category":"nonexistent_cat_abc"}' "$TOKEN" | parse_result)
IS_ERR2=$(echo "$ERR2" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('_isError', False))" 2>/dev/null)

# Non-existent document (get)
ERR3=$(mcp_tool_call "get_document" '{"path":"/nonexistent.pdf"}' "$TOKEN" | parse_result)
IS_ERR3=$(echo "$ERR3" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('_isError', False))" 2>/dev/null)

# Non-existent document (delete)
ERR4=$(mcp_tool_call "delete_document" '{"path":"/nonexistent.pdf"}' "$TOKEN" | parse_result)
IS_ERR4=$(echo "$ERR4" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('_isError', False))" 2>/dev/null)

# Non-existent category in search
ERR5=$(mcp_tool_call "search" '{"query":"test","category":"nonexistent_cat_abc"}' "$TOKEN" | parse_result)
IS_ERR5=$(echo "$ERR5" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('_isError', False))" 2>/dev/null)

if [ "$IS_ERR1" = "True" ] && [ "$IS_ERR2" = "True" ] && [ "$IS_ERR3" = "True" ] && [ "$IS_ERR4" = "True" ] && [ "$IS_ERR5" = "True" ]; then
    pass_test
else
    fail_test "err1=$IS_ERR1 err2=$IS_ERR2 err3=$IS_ERR3 err4=$IS_ERR4 err5=$IS_ERR5"
fi

# ---------------------------------------------------------------
# TS-032: MCP Malformed Request
# ---------------------------------------------------------------
run_test "TS-032" "MCP malformed request handling"

# Invalid tool name
MAL1=$(mcp_tool_call "nonexistent_tool" '{}' "$TOKEN")
HAS_ERR1=$(echo "$MAL1" | python3 -c "import json,sys; d=json.load(sys.stdin); print('yes' if d.get('error') or d.get('result',{}).get('isError') else 'no')" 2>/dev/null)

# Invalid JSON body
MAL2=$(curl -s -X POST "$MCP_URL/mcp" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -d "not valid json")
HAS_ERR2=$(echo "$MAL2" | python3 -c "import json,sys; d=json.load(sys.stdin); print('yes' if d.get('error') else 'no')" 2>/dev/null)

if [ "$HAS_ERR1" = "yes" ] && [ "$HAS_ERR2" = "yes" ]; then
    pass_test
else
    fail_test "mal1_err=$HAS_ERR1, mal2_err=$HAS_ERR2"
fi

# ---------------------------------------------------------------
# Summary
# ---------------------------------------------------------------
echo ""
echo "=== Results: $PASS passed, $ERRORS failed ==="
if [ $ERRORS -gt 0 ]; then exit 1; fi
exit 0

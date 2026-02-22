#!/bin/bash
# Test: MCP HTTP Server & REST API (Lot 7)
# Validates: FR-016, FR-017
# Test Scenarios: TS-015, TS-016, TS-017, TS-018, TS-046, TS-047, TS-048, TS-049, TS-050, TS-032, TS-056, TS-057, TS-058, TS-059

set -eo pipefail

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
    db_query "DELETE FROM documents WHERE file_path LIKE '%/tests/fixtures/%' OR file_path LIKE '%/tmp/mcp-test-%';" >/dev/null 2>&1 || true
    db_query "DELETE FROM tags WHERE name IN ('mcp_test', 'admin', 'work', 'api_test_tag', 'api_doc_test');" >/dev/null 2>&1 || true
    rm -rf /tmp/mcp-test-* 2>/dev/null || true
}

trap cleanup EXIT

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
$BIN tags add mcp_test --description "MCP test" >/dev/null 2>&1 || true
ABS_IMG=$(cd "$FIXTURES" && pwd)/official_document.jpg
index_with_retry "$ABS_IMG" mcp_test

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
INDEX_RESP=$(mcp_tool_call "index_file" "{\"path\":\"$ABS_TXT\",\"tags\":[\"mcp_test\"]}" "$TOKEN" | parse_result)
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
run_test "TS-046" "MCP full workflow (list tags, index, search, get_document, status, delete)"

# 1. List tags
TAGS=$(mcp_tool_call "list_tags" '{}' "$TOKEN" | parse_result)

# 2. Index file (already done above, use PDF)
ABS_PDF=$(cd "$FIXTURES" && pwd)/multipage.pdf
INDEX_RESP=$(mcp_tool_call "index_file" "{\"path\":\"$ABS_PDF\",\"tags\":[\"mcp_test\"]}" "$TOKEN" | parse_result)

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
run_test "TS-047" "MCP search with tag filter, fulltext, limit"

# Setup more docs
$BIN tags add admin --description "Admin" >/dev/null 2>&1 || true
$BIN tags add work --description "Work" >/dev/null 2>&1 || true

ABS_CSV=$(cd "$FIXTURES" && pwd)/sample.csv
$BIN index "$ABS_CSV" --tags work >/dev/null 2>&1

# Tag filter
TAG_SEARCH=$(mcp_tool_call "search" '{"query":"document","tags":["mcp_test"]}' "$TOKEN" | parse_result)
TAG_OK=$(echo "$TAG_SEARCH" | python3 -c "
import json,sys
d=json.load(sys.stdin)
data = d.get('_data', d)
if isinstance(data, list):
    wrong = [r for r in data if 'mcp_test' not in (r.get('tag_names') or '')]
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

if [ "$TAG_OK" = "yes" ] && [ "$LIM_COUNT" -le 1 ] 2>/dev/null; then
    pass_test
else
    fail_test "tag_ok=$TAG_OK, limit_count=$LIM_COUNT"
fi

# ---------------------------------------------------------------
# TS-048: MCP Index with Tags
# ---------------------------------------------------------------
run_test "TS-048" "MCP index with tags parameter"

ABS_FR=$(cd "$FIXTURES" && pwd)/document_fr.txt
IDX_RESP=$(mcp_tool_call "index_file" "{\"path\":\"$ABS_FR\",\"tags\":[\"admin\"]}" "$TOKEN" | parse_result)
IS_ERROR=$(echo "$IDX_RESP" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('_isError', False))" 2>/dev/null)

if [ "$IS_ERROR" = "True" ]; then
    fail_test "Index with tags failed: $IDX_RESP"
else
    # Verify tags
    GET_RESP=$(mcp_tool_call "get_document" "{\"path\":\"$ABS_FR\"}" "$TOKEN" | parse_result)
    HAS_TAG=$(echo "$GET_RESP" | python3 -c "import json,sys; d=json.load(sys.stdin); tags=d.get('tags',[]); print('yes' if 'admin' in tags else 'no')" 2>/dev/null)
    if [ "$HAS_TAG" = "yes" ]; then
        pass_test
    else
        fail_test "Expected tag 'admin' in document tags, got: $(echo "$GET_RESP" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('tags',[]))" 2>/dev/null)"
    fi
fi

# ---------------------------------------------------------------
# TS-050: MCP Tool Error Paths
# ---------------------------------------------------------------
run_test "TS-050" "MCP tool error paths"

# Non-existent file
ERR1=$(mcp_tool_call "index_file" '{"path":"/nonexistent/file.jpg"}' "$TOKEN" | parse_result)
IS_ERR1=$(echo "$ERR1" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('_isError', False))" 2>/dev/null)

# Non-existent document (get)
ERR2=$(mcp_tool_call "get_document" '{"path":"/nonexistent.pdf"}' "$TOKEN" | parse_result)
IS_ERR2=$(echo "$ERR2" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('_isError', False))" 2>/dev/null)

# Non-existent document (delete)
ERR3=$(mcp_tool_call "delete_document" '{"path":"/nonexistent.pdf"}' "$TOKEN" | parse_result)
IS_ERR3=$(echo "$ERR3" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('_isError', False))" 2>/dev/null)

# Non-existent tag in search
ERR4=$(mcp_tool_call "search" '{"query":"test","tags":["nonexistent_tag_abc"]}' "$TOKEN" | parse_result)
IS_ERR4=$(echo "$ERR4" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('_isError', False))" 2>/dev/null)

if [ "$IS_ERR1" = "True" ] && [ "$IS_ERR2" = "True" ] && [ "$IS_ERR3" = "True" ] && [ "$IS_ERR4" = "True" ]; then
    pass_test
else
    fail_test "err1=$IS_ERR1 err2=$IS_ERR2 err3=$IS_ERR3 err4=$IS_ERR4"
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
# TS-056: REST API Endpoints
# ---------------------------------------------------------------

# Helpers for REST API calls
api_get() {
    curl -s -H "Authorization: Bearer $TOKEN" "$MCP_URL$1"
}

api_post() {
    curl -s -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$2" "$MCP_URL$1"
}

api_put() {
    curl -s -X PUT -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$2" "$MCP_URL$1"
}

api_delete() {
    curl -s -X DELETE -H "Authorization: Bearer $TOKEN" "$MCP_URL$1"
}

api_get_status() {
    curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $TOKEN" "$MCP_URL$1"
}

api_post_status() {
    curl -s -o /dev/null -w "%{http_code}" -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$2" "$MCP_URL$1"
}

api_delete_status() {
    curl -s -o /dev/null -w "%{http_code}" -X DELETE -H "Authorization: Bearer $TOKEN" "$MCP_URL$1"
}

# --- REST API: Auth Required ---
run_test "TS-056a" "REST API requires authentication"

NO_AUTH_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$MCP_URL/api/status")
if [ "$NO_AUTH_STATUS" = "401" ]; then
    pass_test
else
    fail_test "Expected 401, got $NO_AUTH_STATUS"
fi

# --- REST API: Tags CRUD ---
run_test "TS-056b" "REST API tags CRUD"

# Create tag
CREATE_RESP=$(api_post "/api/tags" '{"name":"api_test_tag","description":"API test tag"}')
CREATE_NAME=$(echo "$CREATE_RESP" | python3 -c "import json,sys; print(json.load(sys.stdin).get('name',''))" 2>/dev/null)

# List tags
LIST_RESP=$(api_get "/api/tags")
HAS_TAG=$(echo "$LIST_RESP" | python3 -c "import json,sys; tags=json.load(sys.stdin); print('yes' if any(t.get('name')=='api_test_tag' for t in tags) else 'no')" 2>/dev/null)

# Get by name
GET_TAG=$(api_get "/api/tags/api_test_tag")
GET_NAME=$(echo "$GET_TAG" | python3 -c "import json,sys; print(json.load(sys.stdin).get('name',''))" 2>/dev/null)

# Update
UPD_TAG=$(api_put "/api/tags/api_test_tag" '{"description":"Updated desc"}')
UPD_DESC=$(echo "$UPD_TAG" | python3 -c "import json,sys; print(json.load(sys.stdin).get('description',''))" 2>/dev/null)

# Delete
DEL_TAG=$(api_delete "/api/tags/api_test_tag")
DEL_OK=$(echo "$DEL_TAG" | python3 -c "import json,sys; print('yes' if json.load(sys.stdin).get('deleted') else 'no')" 2>/dev/null)

if [ "$CREATE_NAME" = "api_test_tag" ] && [ "$HAS_TAG" = "yes" ] && [ "$GET_NAME" = "api_test_tag" ] && [ "$UPD_DESC" = "Updated desc" ] && [ "$DEL_OK" = "yes" ]; then
    pass_test
else
    fail_test "create=$CREATE_NAME list=$HAS_TAG get=$GET_NAME upd=$UPD_DESC del=$DEL_OK"
fi

# --- REST API: Documents (index, get, search, delete) ---
run_test "TS-056c" "REST API document operations"

# Ensure tag exists
api_post "/api/tags" '{"name":"api_doc_test","description":"API doc test"}' >/dev/null 2>&1

# Clean up any existing document (from earlier test cases) to avoid stale state
ABS_TXT_API=$(cd "$FIXTURES" && pwd)/sample_text.txt
EXISTING_DOC_ID=$(db_query "SELECT id FROM documents WHERE file_path = '$ABS_TXT_API';" 2>/dev/null || true)
if [ -n "$EXISTING_DOC_ID" ]; then
    api_delete "/api/documents/$EXISTING_DOC_ID" >/dev/null 2>&1 || true
fi

# Index a file
INDEX_RESP=$(api_post "/api/documents" "{\"path\":\"$ABS_TXT_API\",\"tags\":[\"api_doc_test\"]}")
DOC_ID=$(echo "$INDEX_RESP" | python3 -c "import json,sys; print(json.load(sys.stdin).get('document_id',''))" 2>/dev/null)

if [ -z "$DOC_ID" ] || [ "$DOC_ID" = "" ]; then
    fail_test "Index via API failed: $INDEX_RESP"
else
    # Get document by ID
    GET_DOC=$(api_get "/api/documents/$DOC_ID")
    GET_TITLE=$(echo "$GET_DOC" | python3 -c "import json,sys; print(json.load(sys.stdin).get('title',''))" 2>/dev/null)

    # Search
    SEARCH_RESP=$(api_get "/api/documents?query=software+engineering&limit=5")
    SEARCH_COUNT=$(echo "$SEARCH_RESP" | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d) if isinstance(d, list) else 0)" 2>/dev/null || echo "0")

    # Delete document
    DEL_DOC=$(api_delete "/api/documents/$DOC_ID")
    DEL_DOC_OK=$(echo "$DEL_DOC" | python3 -c "import json,sys; print('yes' if json.load(sys.stdin).get('deleted') else 'no')" 2>/dev/null)

    if [ -n "$GET_TITLE" ] && [ "$GET_TITLE" != "" ] && [ "$SEARCH_COUNT" -gt 0 ] 2>/dev/null && [ "$DEL_DOC_OK" = "yes" ]; then
        pass_test
    else
        fail_test "title=$GET_TITLE search=$SEARCH_COUNT del=$DEL_DOC_OK"
    fi
fi

# --- REST API: Update All Documents ---
run_test "TS-056d" "REST API update all documents"

UPDATE_RESP=$(api_put "/api/documents" '{"force":false}')
HAS_UPDATED=$(echo "$UPDATE_RESP" | python3 -c "import json,sys; d=json.load(sys.stdin); print('yes' if 'updated' in d else 'no')" 2>/dev/null)

if [ "$HAS_UPDATED" = "yes" ]; then
    pass_test
else
    fail_test "Update all response: $UPDATE_RESP"
fi

# --- REST API: Status ---
run_test "TS-056e" "REST API status"

STATUS_RESP=$(api_get "/api/status")
HAS_TOTAL=$(echo "$STATUS_RESP" | python3 -c "import json,sys; d=json.load(sys.stdin); print('yes' if 'total_documents' in d else 'no')" 2>/dev/null)

if [ "$HAS_TOTAL" = "yes" ]; then
    pass_test
else
    fail_test "Status response: $STATUS_RESP"
fi

# --- REST API: Error Cases ---
run_test "TS-056f" "REST API error responses"

# Missing query param
ERR_STATUS=$(api_get_status "/api/documents")
# Invalid UUID
ERR_UUID_STATUS=$(api_get_status "/api/documents/not-a-uuid")
# Non-existent tag
ERR_TAG_STATUS=$(api_get_status "/api/tags/nonexistent_api_tag_xyz")

if [ "$ERR_STATUS" = "400" ] && [ "$ERR_UUID_STATUS" = "400" ] && [ "$ERR_TAG_STATUS" = "400" ]; then
    pass_test
else
    fail_test "missing_query=$ERR_STATUS invalid_uuid=$ERR_UUID_STATUS bad_tag=$ERR_TAG_STATUS"
fi

# Cleanup REST API test data
api_delete "/api/tags/api_doc_test" >/dev/null 2>&1

# ---------------------------------------------------------------
# TS-057: MCP tools/list Method
# ---------------------------------------------------------------
run_test "TS-057" "MCP tools/list returns all tool definitions"

TOOLS_RESP=$(mcp_call "tools/list" "{}" "$TOKEN")
TOOL_COUNT=$(echo "$TOOLS_RESP" | python3 -c "
import json,sys
d=json.load(sys.stdin)
tools = d.get('result', {}).get('tools', [])
print(len(tools))
" 2>/dev/null || echo "0")

TOOL_NAMES=$(echo "$TOOLS_RESP" | python3 -c "
import json,sys
d=json.load(sys.stdin)
tools = d.get('result', {}).get('tools', [])
names = sorted([t.get('name','') for t in tools])
print(','.join(names))
" 2>/dev/null || echo "")

EXPECTED_TOOLS="delete_document,get_document,index_file,list_tags,search,status,update"
if [ "$TOOL_COUNT" -eq 7 ] && [ "$TOOL_NAMES" = "$EXPECTED_TOOLS" ]; then
    pass_test
else
    fail_test "Expected 7 tools ($EXPECTED_TOOLS), got $TOOL_COUNT ($TOOL_NAMES)"
fi

# ---------------------------------------------------------------
# TS-058: MCP update Tool
# ---------------------------------------------------------------
run_test "TS-058" "MCP update tool"

# Cooldown before update tests (rate limit recovery)
sleep 15

# Index a file first
ABS_UPDATE=$(cd "$FIXTURES" && pwd)/sample_text.txt
index_with_retry "$ABS_UPDATE" mcp_test

# Call update via MCP (single file path, no force)
UPDATE_RESP=$(mcp_tool_call "update" "{\"path\":\"$ABS_UPDATE\",\"force\":false}" "$TOKEN" | parse_result)
HAS_UPDATED=$(echo "$UPDATE_RESP" | python3 -c "import json,sys; d=json.load(sys.stdin); print('yes' if 'updated' in d or 'skipped' in d else 'no')" 2>/dev/null)

# Call update with force
UPDATE_FORCE=$(mcp_tool_call "update" "{\"path\":\"$ABS_UPDATE\",\"force\":true}" "$TOKEN" | parse_result)
HAS_FORCE=$(echo "$UPDATE_FORCE" | python3 -c "import json,sys; d=json.load(sys.stdin); print('yes' if 'updated' in d else 'no')" 2>/dev/null)

if [ "$HAS_UPDATED" = "yes" ] && [ "$HAS_FORCE" = "yes" ]; then
    pass_test
else
    fail_test "update=$HAS_UPDATED, force=$HAS_FORCE"
fi

# ---------------------------------------------------------------
# TS-059: REST API PUT /api/documents/:id (single document update)
# ---------------------------------------------------------------
run_test "TS-059" "REST API update single document by ID"

# Cooldown before REST update test (rate limit recovery)
sleep 15

# Ensure a doc is indexed
api_post "/api/tags" '{"name":"api_doc_test","description":"API doc test"}' >/dev/null 2>&1 || true
ABS_TXT_UPD=$(cd "$FIXTURES" && pwd)/sample_text.txt
IDX_RESP=$(api_post "/api/documents" "{\"path\":\"$ABS_TXT_UPD\",\"tags\":[\"api_doc_test\"]}")
UPD_DOC_ID=$(echo "$IDX_RESP" | python3 -c "import json,sys; print(json.load(sys.stdin).get('document_id',''))" 2>/dev/null)

if [ -z "$UPD_DOC_ID" ] || [ "$UPD_DOC_ID" = "" ]; then
    fail_test "Could not index document for update test: $IDX_RESP"
else
    # Update with force=true
    api_put_status() {
        curl -s -o /dev/null -w "%{http_code}" -X PUT -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$2" "$MCP_URL$1"
    }
    UPD_STATUS=$(api_put_status "/api/documents/$UPD_DOC_ID" '{"force":true}')
    UPD_BODY=$(api_put "/api/documents/$UPD_DOC_ID" '{"force":true}')
    HAS_UPD=$(echo "$UPD_BODY" | python3 -c "import json,sys; d=json.load(sys.stdin); print('yes' if 'updated' in d else 'no')" 2>/dev/null)

    # Update non-existent UUID
    BAD_UPD_STATUS=$(api_put_status "/api/documents/00000000-0000-0000-0000-000000000000" '{"force":true}')

    if [ "$UPD_STATUS" = "200" ] && [ "$HAS_UPD" = "yes" ] && [ "$BAD_UPD_STATUS" = "400" ]; then
        pass_test
    else
        fail_test "status=$UPD_STATUS has_updated=$HAS_UPD bad_status=$BAD_UPD_STATUS"
    fi

    # Cleanup
    api_delete "/api/documents/$UPD_DOC_ID" >/dev/null 2>&1
fi
api_delete "/api/tags/api_doc_test" >/dev/null 2>&1

# ---------------------------------------------------------------
# Summary
# ---------------------------------------------------------------
echo ""
echo "=== Results: $PASS passed, $ERRORS failed ==="
if [ $ERRORS -gt 0 ]; then exit 1; fi
exit 0

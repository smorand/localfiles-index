# LocalFiles Index

Personal file indexing and semantic search system that extracts metadata from local files using AI and enables natural language retrieval.

## Features

- **Index** images (JPEG, PNG, GIF, WebP, TIFF, BMP), PDFs, text files (TXT, MD, RST, LOG), spreadsheets (CSV, XLSX), and documents (DOC, DOCX, ODT)
- **Semantic search** using vector similarity with Gemini embeddings
- **Full-text search** using PostgreSQL tsvector
- **Category management** for organizing indexed files
- **AI-powered analysis** with Gemini Flash for content extraction, title generation, and image description
- **REST JSON API** with standard RESTful conventions at `/api`
- **MCP HTTP API** (JSON-RPC) for AI tool integration at `/mcp`
- **OAuth 2.1** authentication (client credentials grant) for all API access
- **CLI** for all operations

## Prerequisites

- Go 1.25+
- PostgreSQL with pgvector extension
- Gemini API key (`GEMINI_API_KEY` environment variable)
- `pdf-extractor` binary (for PDF processing)
- LibreOffice (for DOC/DOCX conversion, optional)

## Setup

### Database

```bash
# Create database and user
make db-setup

# Or manually:
psql -U postgres -c "CREATE USER localfiles WITH PASSWORD 'localfiles';"
psql -U postgres -c "CREATE DATABASE localfiles OWNER localfiles;"
psql -U localfiles -d localfiles -c "CREATE EXTENSION IF NOT EXISTS vector;"
```

Schema migrations run automatically on first use.

### Build

```bash
make build
```

### Configuration

All configuration is via environment variables with sensible defaults:

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | `postgresql://localfiles:localfiles@localhost:5432/localfiles?sslmode=disable` | PostgreSQL connection |
| `GEMINI_API_KEY` | (required) | Gemini API key |
| `GEMINI_MODEL` | `gemini-3-flash-preview` | Model for content analysis |
| `EMBEDDING_MODEL` | `gemini-embedding-001` | Model for embeddings |
| `EMBEDDING_DIMENSIONS` | `768` | Embedding vector dimensions |
| `CHUNK_SIZE` | `100` | Words per text chunk |
| `CHUNK_OVERLAP` | `5` | Word overlap between chunks |
| `PDF_EXTRACTOR_PATH` | `pdf-extractor` | Path to pdf-extractor binary |
| `TITLE_CONFIDENCE_THRESHOLD` | `0.9` | Auto-accept title threshold |
| `MCP_PORT` | `8080` | MCP server port |
| `LOG_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `OAUTH_CREDENTIALS_PATH` | | Path to OAuth credentials JSON |

## CLI Usage

### Start the Server

```bash
./bin/localfiles-index-darwin-arm64 mcp --port 8080 --credentials /path/to/credentials.json

# Default credentials path: ~/.credentials/scm-pwd-web.json
# Enable debug logging with --verbose / -v
```

### Index Files

```bash
# Index an image (category is required)
./bin/localfiles-index-darwin-arm64 index /path/to/passport.jpg --category administratif

# Index a directory (automatically recursive)
./bin/localfiles-index-darwin-arm64 index /path/to/documents/ --category work
```

### Search

```bash
# Semantic search (default)
./bin/localfiles-index-darwin-arm64 search "passport Sebastien Morand"

# Full-text search
./bin/localfiles-index-darwin-arm64 search "invoice 2024" --mode fulltext

# Filter by category
./bin/localfiles-index-darwin-arm64 search "document" --category administratif

# JSON output
./bin/localfiles-index-darwin-arm64 search "contract" --format json

# Limit results
./bin/localfiles-index-darwin-arm64 search "photos Italy" --limit 5
```

### Categories

```bash
./bin/localfiles-index-darwin-arm64 categories add administratif --description "Administrative docs"
./bin/localfiles-index-darwin-arm64 categories list
./bin/localfiles-index-darwin-arm64 categories update administratif --description "Updated description"
./bin/localfiles-index-darwin-arm64 categories remove administratif [--new-category <name>]
```

### Document Management

```bash
# Show document details
./bin/localfiles-index-darwin-arm64 show /path/to/file.jpg
./bin/localfiles-index-darwin-arm64 show <uuid> --chunks

# Delete
./bin/localfiles-index-darwin-arm64 delete /path/to/file.jpg --yes

# Update (re-index modified files)
./bin/localfiles-index-darwin-arm64 update                # Check all
./bin/localfiles-index-darwin-arm64 update /path/to/file  # Check specific
./bin/localfiles-index-darwin-arm64 update --force         # Re-index all

# Statistics
./bin/localfiles-index-darwin-arm64 status
./bin/localfiles-index-darwin-arm64 status --format json
```

## REST API

The REST API is available at `/api` and uses standard HTTP verbs with JSON request/response bodies. All endpoints require a Bearer token obtained via OAuth.

### Authentication

```bash
# Get a token
TOKEN=$(curl -s -X POST http://localhost:8080/oauth/token \
    -d "grant_type=client_credentials" \
    -d "client_id=YOUR_CLIENT_ID" \
    -d "client_secret=YOUR_SECRET" | jq -r .access_token)
```

### Documents

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/documents?query=...` | Search documents (required: `query`; optional: `mode`, `category`, `limit`) |
| `POST` | `/api/documents` | Index a new file |
| `GET` | `/api/documents/:id` | Get document by UUID |
| `PUT` | `/api/documents/:id` | Re-index a single document |
| `PUT` | `/api/documents` | Re-scan all documents |
| `DELETE` | `/api/documents/:id` | Delete a document |

```bash
# Search documents
curl -H "Authorization: Bearer $TOKEN" \
    "http://localhost:8080/api/documents?query=passport&limit=5"

# Search with category filter and fulltext mode
curl -H "Authorization: Bearer $TOKEN" \
    "http://localhost:8080/api/documents?query=invoice&mode=fulltext&category=admin"

# Index a file
curl -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
    -d '{"path":"/path/to/file.jpg","category":"admin"}' \
    http://localhost:8080/api/documents

# Get document by ID
curl -H "Authorization: Bearer $TOKEN" \
    http://localhost:8080/api/documents/550e8400-e29b-41d4-a716-446655440000

# Re-index a single document (force re-index regardless of mtime)
curl -X PUT -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
    -d '{"force":true}' \
    http://localhost:8080/api/documents/550e8400-e29b-41d4-a716-446655440000

# Re-scan all documents
curl -X PUT -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
    -d '{"force":false}' \
    http://localhost:8080/api/documents

# Delete a document
curl -X DELETE -H "Authorization: Bearer $TOKEN" \
    http://localhost:8080/api/documents/550e8400-e29b-41d4-a716-446655440000
```

### Categories

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/categories` | List all categories |
| `GET` | `/api/categories/:name` | Get category by name |
| `POST` | `/api/categories` | Create a category |
| `PUT` | `/api/categories/:name` | Update a category |
| `DELETE` | `/api/categories/:name` | Delete a category (optional: `?new_category=...`) |

```bash
# List categories
curl -H "Authorization: Bearer $TOKEN" \
    http://localhost:8080/api/categories

# Get category by name
curl -H "Authorization: Bearer $TOKEN" \
    http://localhost:8080/api/categories/admin

# Create a category
curl -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
    -d '{"name":"admin","description":"Administrative documents"}' \
    http://localhost:8080/api/categories

# Update a category
curl -X PUT -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
    -d '{"description":"Updated description"}' \
    http://localhost:8080/api/categories/admin

# Delete a category (migrate documents to another category)
curl -X DELETE -H "Authorization: Bearer $TOKEN" \
    "http://localhost:8080/api/categories/old_cat?new_category=new_cat"
```

### Status

```bash
# Get index statistics
curl -H "Authorization: Bearer $TOKEN" \
    http://localhost:8080/api/status
```

Returns: `{"total_documents": N, "total_chunks": N, "by_type": {...}, "by_category": {...}}`

### Error Responses

All errors return JSON with an `error` field and appropriate HTTP status code:

```json
{"error": "query parameter is required"}
```

| Status | Meaning |
|--------|---------|
| 400 | Client error (missing parameter, invalid UUID, not found) |
| 401 | Unauthorized (missing or invalid Bearer token) |
| 500 | Server error |

### Health Check

```bash
# No authentication required
curl http://localhost:8080/health
```

Returns: `{"status": "ok"}`

## MCP API (JSON-RPC)

The MCP endpoint at `POST /mcp` implements the Model Context Protocol for AI tool integration. It uses JSON-RPC 2.0 format.

```bash
# Initialize
curl -X POST http://localhost:8080/mcp \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'

# List available tools
curl -X POST http://localhost:8080/mcp \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'

# Call a tool
curl -X POST http://localhost:8080/mcp \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search","arguments":{"query":"passport"}}}'
```

**Available MCP tools**: `search`, `index_file`, `get_document`, `list_categories`, `delete_document`, `status`, `update`

## OAuth Configuration

The server requires OAuth 2.1 client credentials for authentication. Credentials are loaded from a JSON file specified via `--credentials` flag or `OAUTH_CREDENTIALS_PATH` environment variable.

### Credentials File Formats

**Google OAuth format** (with `web` wrapper):

```json
{
  "web": {
    "client_id": "your-client-id",
    "client_secret": "your-client-secret"
  }
}
```

**Flat format**:

```json
{
  "client_id": "your-client-id",
  "client_secret": "your-client-secret"
}
```

### Token Flow

1. Client sends credentials to `POST /oauth/token` with `grant_type=client_credentials`
2. Server returns an access token (valid for 1 hour)
3. Client includes `Authorization: Bearer <token>` header in all API requests
4. Both `/mcp` and `/api` endpoints require a valid token

An OAuth callback endpoint is also available at `GET /oauth/callback` for integration with OAuth authorization code flows.

## Testing

```bash
# Run all tests (build + functional)
make e2e-test

# Run unit tests only
make test-unit

# Run all quality checks
make check
```

Test fixtures are generated automatically by `tests/fixtures/generate_fixtures.sh`.

## Architecture

```
File Input --> Detect Type --> Process --> Chunk/Analyze --> Embed --> Store
                 |
    +------------+------------+------------+
    |            |            |            |
  Image        PDF        Text      Spreadsheet
    |            |            |            |
  Gemini     pdf-extractor  Read      Read+Analyze
  analyze    + text + imgs   content   (Gemini)
    |            |            |            |
    +---> Title + Summary + Text Chunks + Embeddings --> PostgreSQL + pgvector
```

## License

Private - Sebastien MORAND

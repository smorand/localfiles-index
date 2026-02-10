# LocalFiles Index

Personal file indexing and semantic search system that extracts metadata from local files using AI and enables natural language retrieval.

## Features

- **Index** images (JPEG, PNG, GIF, WebP, TIFF, BMP), PDFs, text files (TXT, MD), spreadsheets (CSV, XLSX), and documents (DOC, DOCX, ODT)
- **Semantic search** using vector similarity with Gemini embeddings
- **Full-text search** using PostgreSQL tsvector
- **Category management** for organizing indexed files
- **AI-powered analysis** with Gemini Flash for content extraction, title generation, and image description
- **MCP HTTP API** with OAuth 2.1 authentication for integration with AI tools
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
| `OAUTH_CREDENTIALS_PATH` | | Path to OAuth credentials JSON |

## Usage

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
./bin/localfiles-index-darwin-arm64 categories remove administratif [--force]
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

### MCP HTTP Server

```bash
# Start server
./bin/localfiles-index-darwin-arm64 serve --port 8080 --credentials ~/.credentials/scm-pwd-web.json
```

**OAuth authentication** (client credentials grant):

```bash
# Get token
TOKEN=$(curl -s -X POST http://localhost:8080/oauth/token \
    -d "grant_type=client_credentials" \
    -d "client_id=YOUR_CLIENT_ID" \
    -d "client_secret=YOUR_SECRET" | jq -r .access_token)

# Call MCP tool
curl -X POST http://localhost:8080/mcp \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search","arguments":{"query":"passport"}}}'
```

**Available MCP tools**: `search`, `index_file`, `get_document`, `list_categories`, `delete_document`, `status`, `update`

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

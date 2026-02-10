# LocalFiles Index

## Overview
Personal file indexing and semantic search system. Indexes local files (images, PDFs, text, spreadsheets) using Gemini AI and enables natural language retrieval via CLI and MCP HTTP API.

**Tech stack**: Go 1.25, PostgreSQL + pgvector, Gemini AI (Flash + embedding-001), Cobra CLI, Fiber HTTP, Bun ORM

## Key Commands

```bash
make build          # Build binary to bin/
make e2e-test       # Build + run all functional tests
make test-unit      # Go unit tests
make check          # fmt + vet + lint + all tests
make db-setup       # Create PostgreSQL database
```

## Binary Usage

```bash
./bin/localfiles-index-darwin-arm64 index <path> [-c category] [-r]
./bin/localfiles-index-darwin-arm64 search <query> [-m semantic|fulltext] [-c category] [-f table|json|detail] [-l limit]
./bin/localfiles-index-darwin-arm64 categories add|list|update|remove <name>
./bin/localfiles-index-darwin-arm64 show <path|id> [--chunks]
./bin/localfiles-index-darwin-arm64 delete <path|id> [-y]
./bin/localfiles-index-darwin-arm64 update [path] [-f]
./bin/localfiles-index-darwin-arm64 status [-f table|json]
./bin/localfiles-index-darwin-arm64 serve [-p port] [--credentials path]
```

## Project Structure

```
cmd/localfiles-index/main.go    # Entry point
internal/
  cli/                           # Cobra CLI subcommands
  config/config.go               # Env-based configuration
  storage/                       # Bun ORM models + DB operations
    models.go                    # Category, Document, Chunk, Image
    storage.go                   # CRUD, search, stats queries
    migrations/                  # Auto-run schema migrations
  indexer/                       # File indexing pipeline
    indexer.go                   # Orchestrator (image/pdf/text/spreadsheet/doc)
    detector.go                  # File type detection
    chunker.go                   # Text chunking (100 words, 5 overlap)
    pdfparser.go                 # pdf-extractor JSON output parser
  analyzer/analyzer.go           # Gemini AI analysis
  embedding/embedding.go         # Gemini embedding generation
  searcher/searcher.go           # Semantic + fulltext search
  mcp/                           # MCP HTTP Streamable server
    server.go                    # Fiber server + OAuth + JSON-RPC handler
    tools.go                     # MCP tool definitions
    oauth.go                     # OAuth 2.1 client credentials
tests/
  run_tests.sh                   # Test runner (finds test_*.sh)
  test_index.sh                  # Image indexing tests (Lot 2)
  test_text_pdf.sh               # PDF/text/spreadsheet tests (Lot 3)
  test_search.sh                 # Search tests (Lot 4)
  test_categories.sh             # Category CRUD tests (Lot 5)
  test_cli_workflow.sh           # Full CLI workflow tests (Lot 5)
  test_update.sh                 # Update & conversion tests (Lot 6)
  test_mcp.sh                    # MCP HTTP server tests (Lot 7)
  fixtures/generate_fixtures.sh  # Test fixture generator
```

## Conventions
- Go coding standards in `.agent_docs/golang.md`
- Functional tests are bash scripts in `tests/test_*.sh`, exit 0=pass, 1=fail
- All env config with sensible defaults (see `internal/config/config.go`)
- Database: `postgresql://localfiles:localfiles@localhost:5432/localfiles`
- API key: `GEMINI_API_KEY` env var required
- PDF extraction: `pdf-extractor` binary (at `~/.local/bin/pdf-extractor`)

## Documentation Index
- `.agent_docs/golang.md` - Go coding standards and conventions
- `specifications.md` - Full project specifications with FR/TS references

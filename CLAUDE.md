# LocalFiles Index

## Overview
Personal file indexing and semantic search system. Indexes local files (images, PDFs, text, spreadsheets) using Gemini AI and enables natural language retrieval via CLI, REST JSON API (`/api`), and MCP JSON-RPC API (`/mcp`).

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
./bin/localfiles-index-darwin-arm64 index <path> [-t tag1,tag2]       # tags optional; auto-tagging runs; directories auto-recurse
./bin/localfiles-index-darwin-arm64 search <query> [-m semantic|fulltext] [-t tag1,tag2] [-f table|json|detail] [-l limit]
./bin/localfiles-index-darwin-arm64 tags add|list|update|remove|merge <name> [--description "..."] [--rule "..."]
./bin/localfiles-index-darwin-arm64 show <path|id> [--no-chunks]    # id supports short prefix (8+ hex chars)
./bin/localfiles-index-darwin-arm64 delete <path|id> [-y]          # id supports short prefix (8+ hex chars)
./bin/localfiles-index-darwin-arm64 update [path] [-f]
./bin/localfiles-index-darwin-arm64 status [-f table|json]
./bin/localfiles-index-darwin-arm64 mcp [-p port] [--credentials path]  # default: ~/.credentials/scm-pwd-web.json
```

**Global flags**: `--verbose` / `-v` enables debug-level logging.

## Project Structure

```
cmd/localfiles-index/main.go    # Entry point
internal/
  cli/                           # Cobra CLI subcommands
  config/config.go               # Env-based configuration
  storage/                       # Bun ORM models + DB operations
    models.go                    # Tag, DocumentTag, Document, Chunk, Image
    storage.go                   # CRUD, search, stats queries
    migrations/                  # Auto-run schema migrations (001_initial, 002_tags)
  indexer/                       # File indexing pipeline
    indexer.go                   # Orchestrator (image/pdf/text/spreadsheet/doc) + auto-tagging
    detector.go                  # File type detection
    chunker.go                   # Text chunking (100 words, 5 overlap)
    pdfparser.go                 # pdf-extractor JSON output parser
  analyzer/analyzer.go           # Gemini AI analysis + SuggestTags
  embedding/embedding.go         # Gemini embedding generation
  searcher/searcher.go           # Semantic + fulltext search
  mcp/                           # MCP HTTP Streamable server + REST API
    server.go                    # Fiber server + OAuth + JSON-RPC handler + route setup
    api.go                       # REST JSON API handlers (/api endpoints)
    tools.go                     # MCP tool definitions
    oauth.go                     # OAuth 2.1 client credentials
tests/
  run_tests.sh                   # Test runner (ordered by API usage, retries failures)
  test_index.sh                  # Image indexing tests (Lot 2)
  test_text_pdf.sh               # PDF/text/spreadsheet tests (Lot 3)
  test_search.sh                 # Search tests (Lot 4)
  test_tags.sh                   # Tag CRUD + merge + rules tests (Lot 5)
  test_cli_workflow.sh           # Full CLI workflow tests (Lot 5)
  test_update.sh                 # Update & conversion tests (Lot 6)
  test_mcp.sh                    # MCP HTTP server + REST API tests (Lot 7)
  fixtures/generate_fixtures.sh  # Test fixture generator
```

## Conventions
- Go coding standards in `.agent_docs/golang.md`
- Functional tests are bash scripts in `tests/test_*.sh`, exit 0=pass, 1=fail
- All env config with sensible defaults (see `internal/config/config.go`)
- Database: `postgresql://localfiles:localfiles@localhost:5432/localfiles?sslmode=disable`
- API key: `GEMINI_API_KEY` env var required
- PDF extraction: `pdf-extractor` binary (at `~/.local/bin/pdf-extractor`)
- Tags: many-to-many via `document_tags` junction table; auto-created during indexing
- Auto-tagging: tags with non-empty `rule` field are evaluated by LLM during indexing
- Search tag filtering uses AND logic (results must have ALL specified tags)
- Embeddings use batch API calls (all chunks in one request) for rate limit efficiency

## Maintenance Rules
- Whenever you modify code, maintain specifications consistency and always update documentation (README.md and CLAUDE.md if relevant)
- Always commit your modifications
- Always ensure the e2e tests are up to date; before closing and committing, ensure `make e2e-test` is 100% compliant

## Intentional Omissions
- **No Go unit tests (`_test.go`)** — by design; all testing is done via E2E bash scripts in `tests/`
- **No CI/CD pipeline** — by design; local-only project (see specifications.md §2.3 Non-Goals)

## Documentation Index
- `.agent_docs/golang.md` - Go coding standards and conventions
- `.agent_docs/makefile.md` - Makefile targets and usage documentation

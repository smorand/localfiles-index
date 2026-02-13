# Makefile Documentation

## Overview

Generic Makefile for Go projects with multi-command support. Requires `cmd/` directory structure.

## Project Structure Requirements

```
project/
├── cmd/
│   └── localfiles-index/main.go
├── internal/
└── Makefile
```

## Key Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `COMMANDS` | Auto-detected from `cmd/` subdirectories | - |
| `DEFAULT_BINARY_NAME` | First command in `cmd/` | - |
| `MODULE_NAME` | Go module name | `$(DEFAULT_BINARY_NAME)` |
| `BUILD_DIR` | Output directory for binaries | `bin` |

## Build Targets

| Target | Description |
|--------|-------------|
| `build` | Build all commands for current platform |
| `build-all` | Build all commands for all platforms (linux, darwin, windows) |
| `rebuild` | Clean and rebuild for current platform |
| `rebuild-all` | Clean and rebuild for all platforms |

## Run Target

```bash
make run CMD=<command> ARGS='<arguments>'
```

- `CMD` is **required** - specifies which command to run
- `ARGS` is optional - arguments passed to the command

## Install Targets

| Target | Description |
|--------|-------------|
| `install` | Install binaries to `~/.local/bin` (or `/usr/local/bin` as root) |
| `uninstall` | Remove installed binaries |

Override install directory with `TARGET`:
```bash
make install TARGET=/custom/path
```

## Test Targets

| Target | Description |
|--------|-------------|
| `test` | Run functional tests (`tests/run_tests.sh`) |
| `test-unit` | Run Go unit tests (`go test ./...`) |
| `test-all` | Run both unit and functional tests |
| `e2e-test` | Build + run functional tests |

## Code Quality Targets

| Target | Description |
|--------|-------------|
| `fmt` | Format code with `go fmt` |
| `vet` | Run `go vet` |
| `lint` | Run `golangci-lint` (falls back to `go vet`) |
| `check` | Run fmt, vet, lint, then test-all (unit + functional) |

## Docker Targets

| Target | Description |
|--------|-------------|
| `docker-build` | Build Docker image (`MODULE_NAME:latest`) |
| `docker-run` | Run Docker image with `GEMINI_API_KEY`, `DATABASE_URL`, `OAUTH_CREDENTIALS_PATH` |

## Database Target

| Target | Description |
|--------|-------------|
| `db-setup` | Create PostgreSQL user, database, and pgvector extension |

## Other Targets

| Target | Description |
|--------|-------------|
| `clean` | Remove build artifacts |
| `clean-all` | Remove build artifacts, go.mod, and go.sum |
| `init-mod` | Initialize go.mod |
| `init-deps` | Initialize go.mod and download dependencies |
| `list-commands` | List all available commands |
| `info` | Show platform and project information |
| `help` | Show help message |

## Platform Support

Binaries are created with platform suffixes:
- `-linux-amd64`
- `-darwin-amd64`
- `-darwin-arm64`
- `-windows-amd64.exe`

On macOS, binaries are automatically signed with `codesign`.

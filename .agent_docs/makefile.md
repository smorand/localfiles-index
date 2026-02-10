# Makefile Documentation

## Overview

Generic Makefile for Go projects with multi-command support. Requires `cmd/` directory structure.

## Project Structure Requirements

```
project/
├── cmd/
│   ├── command1/main.go
│   └── command2/main.go
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
| `MAKE_DOCKER_PREFIX` | Docker registry prefix | empty |
| `DOCKER_TAG` | Docker image tag | `latest` |

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
| `install-launcher` | Install launcher scripts with all platform binaries |
| `uninstall` | Remove installed binaries |

Override install directory with `TARGET`:
```bash
make install TARGET=/custom/path
```

## Test Targets

| Target | Description |
|--------|-------------|
| `test` | Run functional tests (`tests/run_tests.sh`) |
| `test-unit` | Run Go unit tests |
| `test-all` | Run both functional and unit tests |

## Code Quality Targets

| Target | Description |
|--------|-------------|
| `fmt` | Format code with `go fmt` |
| `vet` | Run `go vet` |
| `lint` | Run `golangci-lint` (falls back to `go vet`) |
| `check` | Run fmt, vet, lint, and test |

## Docker Targets

| Target | Description |
|--------|-------------|
| `docker` | Build and push all Docker images |
| `docker-build` | Build Docker images for all commands |
| `docker-push` | Push Docker images to registry |

Example with custom registry:
```bash
MAKE_DOCKER_PREFIX=gcr.io/my-project/ DOCKER_TAG=v1.0.0 make docker
```

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

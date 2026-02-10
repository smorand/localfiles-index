# Go Coding Standards

## Project Structure

```
name/
├── cmd/                      # Main applications (one directory per binary)
│   └── name/main.go          # Entry point only (minimal logic)
├── internal/                 # Private application code
│   ├── cli/                  # CLI-specific implementation
│   └── {domain}/             # Domain/feature packages
├── pkg/                      # Public library code (only if needed)
├── tests/                    # Functional tests (shell scripts)
│   ├── run_tests.sh
│   └── test_*.sh
├── Makefile
├── CLAUDE.md
└── README.md
```

**Rules:**
- `cmd/`: Only entry points. Keep main.go minimal (just wiring/initialization)
- `internal/`: All private code. Organize by domain/feature, not by layer
- `pkg/`: Only for libraries meant to be imported by other projects
- **NEVER use a `/src` directory**

## Coding Conventions

### Naming
- Clear purpose while being concise
- No abbreviations outside standards (id, api, db, err)
- Boolean: `is`, `has`, `should` prefixes
- Functions: verbs or verb+noun
- Plurals: `users` (slice), `userList` (wrapped struct), `userMap` (specific structure)

### Functions
- One function, one responsibility
- If name needs "and"/"or", split it
- Limit conditional/loop depth to 2 levels (use early return)
- Order functions by call order (top-to-bottom)

### Error Handling
- Handle where meaningful response is possible
- Use `%w` for error chains, `%v` for simple logging
- Never ignore return errors
- Sentinel errors for expected conditions: `var ErrNotFound = errors.New("not found")`

## File Structure Order

1. package declaration
2. import statements (grouped)
3. Constants (const)
4. Variables (var)
5. Type/Interface/Struct definitions
6. Constructor functions (New*)
7. Methods (grouped by receiver, alphabetically)
8. Helper functions (alphabetically)

## Interfaces and Structs

- Define interfaces in the package that uses them ("Accept interfaces, return structs")
- Pointer receivers for: state modification, large structs (3+ fields), consistency
- Value receivers otherwise

## Context

- Always pass as first parameter
- Use `context.Background()` only in main and tests

## Testing

### Unit Tests
- Prefer standard library's `if + t.Errorf` over assertion libraries
- Prefer manual mocking over gomock
- Run with `make test-unit`

### Functional Tests
- Shell scripts in `tests/` directory
- Each script: self-contained, executable, exit 0=pass, 1=fail
- Run with `make test`

## Forbidden Practices

- **init() functions**: Prefer explicit initialization
- **nil injection**: Never pass nil as argument, never return nil for non-error values
- **Magic numbers/strings**: Convert to constants

## Recommended Libraries

| Purpose | Library |
|---------|---------|
| Web | Fiber |
| DB | Bun, SQLBoiler |
| Logging | slog |
| CLI | cobra |
| Utilities | samber/lo, golang.org/x/sync |
| Config | koanf (viper if cobra needed) |
| Validation | go-playground/validator/v10 |
| Scheduling | github.com/go-co-op/gocron |
| Image | github.com/h2non/bimg |

# sb_info Copilot Instructions

## Project Overview
CLI tool for retrieving data from SonnenBatterie devices via their HTTP APIs (v1 and v2). The tool handles authentication using SHA-512 and PBKDF2-based challenge-response protocol.

## Architecture
- **[main.go](../main.go)**: CLI entry point, parses flags, coordinates API calls, outputs formatted JSON
- **[sb/sb.go](../sb/sb.go)**: Core `SonnenBatterie` client with authentication and API request methods
- Two API versions:
  - v1: `/api/` endpoints (default) - `battery_system`, `powermeter`, `inverter`, `system_data`, `v1/status`, `battery`
  - v2: `/api/v2/` endpoints (with `-2` flag) - `configurations`, `battery`, `inverter`, `latestdata`, `powermeter`, `status`

## Authentication Pattern
The SonnenBatterie supports two authentication methods in `Login()`:

### New Method (firmware ≥1.18)
Triggered when `/api/salt/{username}` returns HTTP 200:
1. Hash password with SHA-512 → hex encode
2. GET `/api/challenge` returns challenge (JSON or plain text)
3. GET `/api/salt/{username}` returns salt value
4. Derive key: `pbkdf2(sha512_hex_password, salt, 7500 iterations, 64 bytes, SHA-512)` → hex
5. Create response: `HMAC-SHA256(derived_key, challenge)` → hex
6. POST form-encoded payload to `/api/session`
7. Handle potential `new_challenge` in response (retry loop, max 2 attempts)
8. Extract `authentication_token` from JSON response
9. Use token in `Auth-Token` header for subsequent requests

### Old Method (legacy firmware)
Triggered when salt endpoint fails or returns non-200:
1. Hash password with SHA-512 → hex encode
2. GET `/api/challenge` returns quoted challenge string (strip quotes)
3. Derive response: `pbkdf2(sha512_hex_password, challenge, 7500 iterations, 64 bytes, SHA-512)`
4. POST form-encoded payload to `/api/session`
5. Extract `authentication_token` from JSON response
6. Use token in `Auth-Token` header for subsequent requests

**Critical**: 
- Old method: Challenge response quotes must be stripped (see [sb/sb.go](../sb/sb.go#L58-L59))
- New method: Challenge may be JSON object or plain text; handle both formats
- New method: Server may return `new_challenge` requiring retry with updated challenge

## Error Handling
- Methods return `(string, bool)` - second value indicates success
- [main.go](../main.go) prints error messages but continues execution
- Login errors call `os.Exit(1)` after printing error from JSON response

## Build System
[Makefile](../Makefile) provides cross-compilation for 6 platforms:
- `make all` - builds all targets
- `make mactel` (default), `macarm`, `amd64`, `arm64`, `wintel`, `winarm`
- Output: `bin/<os>/<arch>/sb_info[.exe]`
- Creates versioned tarballs: `sb_info-<VERSION>-<os>-<arch>.tar.gz`
- Version extracted from latest git tag

## Development Workflow
```bash
# Build for current platform (macOS Intel by default)
make

# Build all platforms
make all

# Run locally
go run main.go -u User -p password -h 192.168.1.100
go run main.go -u User -p password -h 192.168.1.100 -2  # API v2
```

## Dependencies
- `golang.org/x/crypto/pbkdf2` - authentication key derivation
- Standard library only otherwise (net/http, encoding/json, crypto/sha512)

## Conventions
- Package structure: `main` + single `sb` package for SonnenBatterie client
- HTTP client: Use `http.DefaultClient` with custom requests (not `http.Get/Post` for authenticated calls)
- JSON handling: Unmarshal to `map[string]interface{}`, then marshal with indentation for output
- No tests currently in codebase
- Commented-out debug `fmt.Printf` statements left in code for reference

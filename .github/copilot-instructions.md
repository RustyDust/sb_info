# sb_info Copilot Instructions

## Project Overview
CLI tool that reads everything the local API of a SonnenBatterie reveals (API v1 and v2, plus the cascade endpoints the battery's web dashboard uses) and writes it as one JSON report, e.g. for bug reports of the Home Assistant integration [ha_sonnenbatterie](https://github.com/weltmeyer/ha_sonnenbatterie) (which keeps a copy in `tools/sb_info`). It only reads (HTTP GET); it must never change anything on the battery.

## Architecture
- **[main.go](../main.go)**: CLI flags, the endpoint lists (`v1Endpoints`, `v2Endpoints`, `cascadeEndpoints`), the `Report` structure and the cascade analysis (`analyseCascade`, `membersFrom`)
- **[sb/sb.go](../sb/sb.go)**: `SonnenBatterie` client: login and `Get(path)`, which returns a `Result` (HTTP status plus decoded JSON data or an error)
- **[sb/redact.go](../sb/redact.go)**: `Redact()`: credentials always, IP/MAC addresses unless `-no-redact`
- Endpoints (paths relative to `/api/`):
  - v1: `battery_system`, `system_data`, `v1/status`, `battery`, `inverter`, `powermeter`, `json_api/json_api_configuration`, `v1/commissioning_assistant/special_function`
  - v2: `v2/status`, `v2/latestdata`, `v2/configurations`, `v2/battery`, `v2/inverter`, `v2/powermeter`, `v2/io` (with `-2` only these)
  - cascade (always): `stacking_status`, `v1/commissioning_assistant/special_function/stacking/protect`, `v1/commissioning_assistant/inverter/stacking/status`

## Authentication Pattern
The SonnenBatterie supports two authentication methods in `Login()`:

### New Method (firmware ≥1.18)
Triggered when `/api/salt/{username}` returns HTTP 200:
1. Hash password with SHA-512 → hex encode
2. GET `/api/challenge` returns challenge (JSON string, JSON object or plain text)
3. GET `/api/salt/{username}` returns salt value
4. Derive key: `pbkdf2(sha512_hex_password, salt, 7500 iterations, 64 bytes, SHA-512)` → hex
5. Create response: `HMAC-SHA256(derived_key, challenge)` → hex
6. POST form-encoded payload to `/api/session`
7. Handle potential `new_challenge` in response (retry loop, max 2 attempts)
8. Extract `authentication_token` from JSON response
9. Use token in `Auth-Token` header for subsequent requests

### Old Method (legacy firmware)
Triggered when the salt endpoint returns non-200:
1. Hash password with SHA-512 → hex encode
2. GET `/api/challenge` returns a quoted challenge string (quotes are stripped)
3. Derive response: `pbkdf2(sha512_hex_password, challenge, 7500 iterations, 64 bytes, SHA-512)`
4. POST form-encoded payload to `/api/session`
5. Extract `authentication_token` from JSON response

**Critical**:
- A failed login may return a `block_time`: after too many failed logins the battery blocks logins for a while. Never retry a failed login.
- The battery's web server handles one request at a time: keep the pause between requests.

## Error Handling
- `Login()` returns an `error`; on failure the report contains only `error` and the tool exits with 1
- `Get()` never aborts: HTTP errors and invalid JSON end up in the endpoint's `Result.Error`
- No `panic`/`os.Exit` inside the `sb` package
- Error messages are scrubbed of the host unless `-no-redact`

## Output
One JSON document on stdout (see README): `tool`, `generated_at`, `host`, `user`, `apis`, `redacted`, `error`, `cascade`, `endpoints` (map path → `{status, data | error}`). JSON is decoded with `UseNumber()` so numbers stay exactly as the battery sent them. Never print the password; tokens are always redacted.

## Build System
[Makefile](../Makefile) provides cross-compilation for 6 platforms:
- `make test` - `go vet` and unit tests
- `make all` - builds all targets
- `make mactel` (default), `macarm`, `amd64`, `arm64`, `wintel`, `winarm`
- Output: `bin/<os>/<arch>/sb_info[.exe]`, packed as `sb_info-<VERSION>-<os>-<arch>.tar.gz`
- Version from the latest git tag, overridable with `VERSION=...` (the release workflow sets it from the tag and runs `make test` first)

## Development Workflow
```bash
make test
go run . -h 192.168.1.100 -p password            # API v1 + v2
SB_PASSWORD=password go run . -h 192.168.1.100 -2  # API v2 only
```

## Dependencies
- `golang.org/x/crypto/pbkdf2` - authentication key derivation
- Standard library only otherwise (net/http, encoding/json, crypto/sha512)

## Conventions
- Package structure: `main` + single `sb` package for the SonnenBatterie client
- HTTP client with a 15 s timeout; GET only
- Unit tests next to the code (`main_test.go`, `sb/redact_test.go`); cascade tests use values from real reports

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.4.0] - 2026-11-10

Also maintained in the `tools/` folder of
[ha_sonnenbatterie](https://github.com/weltmeyer/ha_sonnenbatterie).

### Added
- Cascade check: the read-only endpoints the battery's web dashboard uses for
  cascades (`stacking_status`, `.../stacking/protect`, `.../inverter/stacking/status`)
  and a `cascade` summary (role, cascade totals, unit estimates, members with serials)
- More endpoints: `json_api/json_api_configuration`, the commissioning settings,
  `v2/inverter` and `v2/io`; by default v1 and v2 are read in one run
- `-no-redact` to keep IP and MAC addresses
- Password from the environment variable `SB_PASSWORD`
- Unit tests (`make test`), also run by the release workflow before building

### Changed
- The report is one JSON document: one entry per endpoint with its HTTP status
  and the data or the error, instead of text sections
- `-u` defaults to `User`
- `-2` still reads only the v2 data endpoints, but the cascade check always runs
- The Makefile's version can be overridden (`VERSION=...`); the release workflow sets it from the tag

### Fixed
- The password is no longer printed
- Credentials (e.g. `IN_LocalAPIToken`) are always redacted; IP and MAC
  addresses by default
- Power meter data: list responses failed to parse, so `powermeter` was
  always reported as unavailable
- HTTP timeout (15 s) instead of waiting forever; errors are reported in the
  JSON instead of aborting with a panic

## [0.3.0] - 2025-12-17

### Added
- Support for new authentication method (firmware ≥1.18)
  - Salt-based authentication using PBKDF2 + HMAC-SHA256
  - Automatic detection and fallback to legacy authentication
  - Support for `new_challenge` retry mechanism
- Version information with `-version` flag
  - Version injected at build time from git tags
  - Shows version string when invoked with `-version`
- GitHub Actions workflow for automated release builds
  - Cross-platform builds for Linux, macOS, and Windows (amd64/arm64)
  - Automatic asset upload to GitHub releases
  - ZIP packaging for Windows, tar.gz for Linux/macOS
- Build instructions in README.md
- AI coding agent instructions in `.github/copilot-instructions.md`

### Changed
- Updated Go version to 1.25.5
- Enhanced authentication flow to support both old and new firmware versions
- Improved error handling for login failures with detailed HTTP status codes

### Fixed
- Challenge parsing now handles both JSON and plain text formats
- Better error messages for authentication failures

## [Previous Releases]

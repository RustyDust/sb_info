# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

_Historical changelog entries to be added as needed_

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Open sourced

Janus is now MIT licensed. The licensing service, the management portal and every
license check are gone: there is no license key, no activation and no phoning
home. A clean clone builds with no credentials.

**Breaking:**
- The `--license` flag is removed from `janus`, `janus mcp server` and the
  executor (`--executor-license`, `$JANUS_LICENSE`). Passing them is now an error.
- Container image labels moved from `io.pharmalytica.janus.*` /
  `com.pharmalytica.janus.*` to `io.github.shairozan.janus.*`. Images built with
  the old labels are no longer discovered and must be relabelled.
- The Go module is now `github.com/shairozan/janus`.
- The macOS package identifier and Windows registry keys changed namespace, so an
  upgrade installs alongside an existing install rather than replacing it.

### Added
- `janus keys` — generate, import, list, export and fingerprint the run-log
  signing key, stored in the OS credential store (Keychain, Credential Manager,
  Secret Service) with a file backend for headless hosts
- Multi-user run-log verification. A user- or org-maintained keyring
  (`signing.keyring_path`) replaces the license as the trust anchor, so colleagues
  can verify each other's records and a rotated key keeps its history verifiable —
  previously documented as unsupported
- `SECURITY.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, issue and PR templates
- `documentation/features/signing_keys.md`

### Added
- Real-time output streaming for active NONMEM jobs
- Live Output button in Active Local Executions table
- Cross-platform binary path resolution for Windows and Linux
- GitLab CI/CD pipeline with automated testing and releases
- Comprehensive unit tests for path construction

### Changed
- Enhanced version package with detailed build information
- Improved error handling throughout execution system
- Updated active runs table layout with additional spacing

### Fixed
- NONMEM binary execution issues on Windows (.exe extension requirement)
- Path construction and resolution across different platforms
- Variable scoping issues in streaming output implementation

## [v0.1.0] - Initial Release

### Added
- Initial GUI application using Fyne framework
- Model loading and validation
- Local NONMEM execution support
- Run history tracking with JSON persistence
- Basic configuration management
- Audit trail foundation
- Self-validating state management

[Unreleased]: https://gitlab.com/your-namespace/janus/-/compare/v0.1.0...HEAD
[v0.1.0]: https://gitlab.com/your-namespace/janus/-/tags/v0.1.0
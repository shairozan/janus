# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
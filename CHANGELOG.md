# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/)
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Changed

- Replace custom flag parsing with standard library `flag` package

## [0.1.0] - 2026-03-18

### Added

- First public release of `claude-statusline`! 🎉
- Configuration via TOML file
- Modules: Model, Context bar, Context tokens, Context percentage, Cost, Duration, Status (using status.claude.com)
- Responsive layout with auto-wrapping and terminal width detection
- Performance optimizations for fast rendering

[Unreleased]: https://github.com/TheoBrigitte/claude-statusline/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/TheoBrigitte/claude-statusline/releases/tag/v0.1.0

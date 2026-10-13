# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/)
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.3.0] - 2026-10-13

### Added

- Show upstream status message when Claude API is degraded
- `--debug-log` flag to append debug logs to a file
- `--input-log` flag to append raw status JSON updates to a `.jsonl` file
- Debug logs for terminal width, module widths, context bar shrink and output lines

### Deprecated

- `--debug` flag, use `--debug-log`
- `--log-file` flag, use `--input-log`

### Fixed

- Increase status request timeout to 30 seconds
- Return an error in case of invalid/non-existing config file
- Shrink auto-sized context bar to fit the status line on one line
- Measure module width in terminal columns instead of bytes

## [0.2.0] - 2026-06-23

### Added

- `--log-file` flag to log raw status JSON updates to a `.jsonl` file
- Add rate limit modules (`$rate_5h`, `$rate_7d`) with usage %, reset countdown, threshold colors, and 󰊚 icon

### Changed

- Replace custom flag parsing with standard library `flag` package
- Replace custom hex color parsing with github.com/go-playground/colors library

### Fixed

- Fix empty status component rendering as blank segment
- Fix terminal width detection
- Fix CVE-2026-39824 and upgrade to golang.org/x/sys@v0.46.0

## [0.1.0] - 2026-03-18

### Added

- First public release of `claude-statusline`! 🎉
- Configuration via TOML file
- Modules: Model, Context bar, Context tokens, Context percentage, Cost, Duration, Status (using status.claude.com)
- Responsive layout with auto-wrapping and terminal width detection
- Performance optimizations for fast rendering

[Unreleased]: https://github.com/TheoBrigitte/claude-statusline/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/TheoBrigitte/claude-statusline/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/TheoBrigitte/claude-statusline/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/TheoBrigitte/claude-statusline/releases/tag/v0.1.0

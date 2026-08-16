# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/)
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- `pkg/width` — measures a rendered string in terminal cells: ANSI escape
  sequences are skipped and text is counted per grapheme cluster

### Changed

- Fork of [TheoBrigitte/claude-statusline](https://github.com/TheoBrigitte/claude-statusline):
  module path is now `github.com/keyamasabaya/claude-statusline`, so
  `go install github.com/keyamasabaya/claude-statusline@latest` resolves
- `status.Fetch` now returns `(string, error)`: the indicator stays short enough
  for a status line and the failure detail travels in the error

### Fixed

- Layout measured segment widths in bytes instead of terminal cells, so any
  Nerd Font glyph, emoji or block character inflated the measurement and
  wrapped the status line early. The documented Nerd Font config was split
  across three lines on an 80 column terminal while occupying 67 cells
- Panic on a `used_percentage` outside 0-100: the context bar asked
  `strings.Repeat` for a negative count, which crashed the binary and left the
  prompt with no status line at all
- API status cache was never written on a first run, so every render performed
  a blocking HTTP request for a full cache duration
- API status cache file was left open on the cache-hit path
- Network and decode failures injected the raw error text into the status line
  (over 70 characters); the indicator is now `🔴` and the detail is returned as
  an error
- API status did not check the HTTP status code, reporting a non-2xx response
  as degraded rather than as a failure
- `--debug` wrote its terminal width warning to stdout, mixing diagnostics into
  the status line; it now goes to stderr as the flag help states
- `{value}`, `{symbol}` and `{reset}` were only substituted at their first
  occurrence in a `format` string
- A module hidden by `min_term_width` left the spaces around its token behind
  as a visible gap
- `fg:<name>` was silently ignored for named colors, while `bg:<name>` worked
- Layout wrapped content measuring exactly the available width
- `terminal.Width` did not fall back to `COLUMNS` when `/dev/tty` opened but
  reported no size, and accepted a non-positive width
- Build workflow filtered branches on `'*'`, which does not match `/`, so no
  build ever ran for a branch named like `claude/foo` or `fix/bar`

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

[Unreleased]: https://github.com/keyamasabaya/claude-statusline/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/keyamasabaya/claude-statusline/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/keyamasabaya/claude-statusline/releases/tag/v0.1.0

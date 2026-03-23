# Changelog

All notable changes to cmdvault are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.6.0] - 2026-03-22

### Added
- Local CI pipeline via `make ci` (fmt, vet, lint, vuln, test, coverage, build)
- Expanded golangci-lint config with gocyclo, gocognit, funlen, nestif, dupl,
  errname, exhaustive, godot linters
- `make tools` target to install dev tools (golangci-lint, govulncheck, goreleaser, goimports)
- `make vuln`, `make coverage`, `make release-dry` targets
- Picker package tests with extracted `parseSelection` helper
- Loader test coverage improved from 40% to 83%
- Package-level doc comments on all packages
- Full documentation on `Status` type and constants

### Fixed
- Shell injection in fzf `--prompt` — placeholder names are now escaped
- Child process cleanup on SIGINT/SIGTERM via `signal.NotifyContext`
- File descriptor leak — stdin pipe now closed after process exits
- Duplicate alias resolution is now deterministic (first loaded wins, warning on stderr)
- Descriptor.Args no longer mutated in-place during placeholder resolution

### Changed
- **Breaking (internal):** Extracted `internal/resolve`, `internal/shell`, and
  `internal/prompt` packages from main.go — reduces `main()` cognitive complexity
  from 46 to under 15
- Extracted `normalizeDescriptor` helper in loader to reduce `loadFileWithBase`
  cognitive complexity from 19 to under 15
- GitHub Actions workflows commented out (CI runs locally for now)

### Fixed (docs)
- Incorrect example showing multiple `-f` flags (only one is supported)
- All 66 doc comments now end with a period (godot convention)

## [0.5.0] - 2026-02-18

### Added
- `--print` flag to output resolved command instead of executing
- File picker placeholder type (`type: file`) with description and default support
- 236 real-world example commands across 19 categories
- Comprehensive test suite for command, history, orchestrator, and main
- GitHub Actions CI and GoReleaser config
- Detailed documentation in `docs/`

### Fixed
- Trailing newline in `--print` mode when stdout is a TTY
- Promote description to name when name is missing
- Sanitize command names/descriptions (collapse newlines/tabs)
- Recursive loading for `-f` directory paths
- Symlink resolution in `LoadDirRecursive`

## [0.4.0] - 2026-02-18

### Changed
- **Complete rewrite from TypeScript to Go**
- Renamed project from tuizer to cmdvault

### Added
- Aliases and placeholder system (`{{name}}` syntax)
- Categories derived from directory structure
- Zsh and bash shell integrations
- Recursive directory loading
- Example files copied on first run
- Execution history logging (JSONL)
- `--version` flag
- Makefile for build/install

## [0.3.0] - 2021-06-07

### Added
- YAML file support for command definitions

## [0.2.0] - 2021-06-04

### Added
- Directory scanning for command files
- Running status indicator with spinner
- Parameter/input support for commands
- Demo GIF in README

### Fixed
- Reset running status on command error

## [0.1.0] - 2021-05-30

### Added
- Initial release (TypeScript/Ink)
- Interactive command picker with fuzzy finder
- JSON command file format
- Command execution with output display

[0.6.0]: https://github.com/thassiov/cmdvault/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/thassiov/cmdvault/compare/0.4.0...v0.5.0
[0.4.0]: https://github.com/thassiov/cmdvault/compare/0.3.0...0.4.0
[0.3.0]: https://github.com/thassiov/cmdvault/compare/0.2.0...0.3.0
[0.2.0]: https://github.com/thassiov/cmdvault/compare/0.1.0...0.2.0
[0.1.0]: https://github.com/thassiov/cmdvault/releases/tag/0.1.0

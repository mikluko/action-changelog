# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/2.0.0/)
with one departure: `Breaking` is a section heading of its own. This project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- A flag that reports the resolved configuration and exits.

## [2.1.0] - 2026-06-18

### Added

- A second transport, chosen by configuration rather than by build tag.

### Fixed

- The retry budget is no longer spent on requests that never left the process.

## [2.0.0] - 2026-04-02

### Breaking

- The configuration file replaces the environment variables it used to read.
  A run finding neither exits non-zero rather than falling back to defaults.

### Changed

- Defaults are resolved once at startup instead of per request.

### Removed

- The single-argument constructor deprecated in 1.3.0.

## [1.4.1] - 2026-01-15

### Security

- Credentials are no longer written to the debug log.

[unreleased]: https://git.example.invalid/repository/compare/v2.1.0...HEAD
[2.1.0]: https://git.example.invalid/repository/compare/v2.0.0...v2.1.0
[2.0.0]: https://git.example.invalid/repository/compare/v1.4.1...v2.0.0
[1.4.1]: https://git.example.invalid/repository/releases/tag/v1.4.1

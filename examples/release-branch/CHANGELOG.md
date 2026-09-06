# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/2.0.0/)
with one departure: `Breaking` is a section heading of its own. This project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.3.0-rc.1]

### Added

- A second transport, chosen by configuration rather than by build tag.

### Fixed

- The retry budget is no longer spent on requests that never left the process.

## [1.2.0] - 2026-05-21

### Added

- A flag that reports the resolved configuration and exits.

## [1.1.0] - 2026-03-04

### Changed

- Defaults are resolved once at startup instead of per request.

[unreleased]: https://git.example.invalid/repository/compare/v1.3.0-rc.1...HEAD
[1.3.0-rc.1]: https://git.example.invalid/repository/compare/v1.2.0...v1.3.0-rc.1
[1.2.0]: https://git.example.invalid/repository/compare/v1.1.0...v1.2.0
[1.1.0]: https://git.example.invalid/repository/releases/tag/v1.1.0

# Changelog

This copy of the example departs from the policy in four ways, one per check it
provokes. It exists so the example is executed rather than described: a test
runs both files under the configuration the workflows carry and holds this one
to the findings the README lists.

## [Unreleased]

### Notes

- `Notes` is outside the accepted vocabulary, which is the Keep a Changelog six
  plus `Breaking`.

## [2.2.0-rc.1] - 2026-07-30

### Added

- A pre-release entry, which the policy keeps out of the changelog even where
  the repository's tags carry one.

## [2.0.0] - 2026-04-02

### Breaking

- The configuration file replaces the environment variables it used to read.

## [2.1.0] - 2026-03-01

### Added

- An entry above the version that precedes it, so the versions no longer
  descend. Its date still descends, so only the version order is at fault.

## [1.4.1] - 2026-01-15

### Security

- An entry with no link reference definition, while every entry above it has
  one, so its heading renders as literal text.

[unreleased]: https://git.example.invalid/repository/compare/v2.2.0-rc.1...HEAD
[2.2.0-rc.1]: https://git.example.invalid/repository/compare/v2.1.0...v2.2.0-rc.1
[2.1.0]: https://git.example.invalid/repository/compare/v2.0.0...v2.1.0
[2.0.0]: https://git.example.invalid/repository/compare/v1.4.1...v2.0.0

# Changelog

This copy of the example departs from the release-branch policy in three ways.
It exists so the example is executed rather than described: a test runs both
files under the configuration the four workflows carry and holds this one to the
findings the README lists.

## [Unreleased]

## [1.3.0]

### Added

- The open entry, with no link reference definition, while every entry below it
  has one. Its definition is written in released form the moment the entry is
  opened and stays broken until the tag is cut; withholding it until then is
  what this file does wrong.

## [1.2.0]

### Fixed

- A second entry carrying no date. The relaxation is scoped to the newest entry,
  so this one is heading-form's whatever undated-entry is set to.

## [1.1.0-rc.3] - 2026-03-04

### Added

- A candidate written into the document. Under this strategy the changelog names
  the release it is heading for and never an attempt at one, so an identifier in
  a heading is a defect on every invocation rather than on some of them. The tag
  `v1.1.0-rc.3` was a real thing; the entry naming it never should have been.

## [1.0.0] - 2026-01-15

### Added

- The first release.

[unreleased]: https://git.example.invalid/repository/compare/v1.3.0...HEAD
[1.2.0]: https://git.example.invalid/repository/compare/v1.1.0-rc.3...v1.2.0
[1.1.0-rc.3]: https://git.example.invalid/repository/compare/v1.0.0...v1.1.0-rc.3
[1.0.0]: https://git.example.invalid/repository/releases/tag/v1.0.0

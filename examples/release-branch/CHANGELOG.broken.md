# Changelog

This copy of the example departs from the release-branch policy in three ways.
It exists so the example is executed rather than described: a test runs both
files under the configuration the three workflows carry and holds this one to
the findings the README lists, which differ per invocation because the policy
does.

## [Unreleased]

## [1.3.0]

### Added

- The open entry, with no link reference definition, while every entry below it
  has one. An open entry's definition is written in released form the moment the
  entry is opened and stays broken until the tag is cut; withholding it until
  then is what this file does wrong.

## [1.2.0]

### Fixed

- A second entry carrying no date. The relaxation is scoped to the newest entry,
  so this one is heading-form's whatever undated-entry is set to.

## [1.1.0-pre.3] - 2026-03-04

### Added

- A candidate left in the file as history. The newest entry may name one, which
  is how this strategy cuts a candidate at all, but an old one below it was
  never rewritten into the release it became. Only the trunk invocation reports
  it: the branch switches prerelease-entry off, and the check is a judgement
  about the whole document rather than about one entry, so switching it off to
  permit the newest heading permits this one too.

## [1.0.0] - 2026-01-15

### Added

- The first release.

[unreleased]: https://git.example.invalid/repository/compare/v1.3.0...HEAD
[1.2.0]: https://git.example.invalid/repository/compare/v1.1.0-pre.3...v1.2.0
[1.1.0-pre.3]: https://git.example.invalid/repository/compare/v1.0.0...v1.1.0-pre.3
[1.0.0]: https://git.example.invalid/repository/releases/tag/v1.0.0

# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/2.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-09-04

### Added

- `-validate` reports where a Keep a Changelog document departs from the format,
  emitting each finding as a workflow annotation under GitHub Actions.
- `-sections` accepts a level-3 heading vocabulary other than the Keep a
  Changelog six.
- A Docker container action packaging the validator, running a prebuilt image
  from `ghcr.io/mikluko/action-changelog` on a `scratch` base.

[Unreleased]: https://github.com/mikluko/action-changelog/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/mikluko/action-changelog/releases/tag/v0.1.0

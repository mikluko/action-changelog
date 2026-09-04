# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/2.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Every check carries a name — `heading-form`, `date-format`, `version-order`,
  `empty-entry`, `unknown-section` — which appears in the finding and as the
  title of the workflow annotation.
- `-error`, `-warn` and `-off` move a named check between severities, and
  `-fail-on error|warning|never` decides what turns the step red.
- `-list-checks` prints the register of checks, and the README's table is
  generated from that same register.
- `date-order` reports an entry dated later than the one above it, and
  `date-future` one dated later than today, both errors by default.
- `partial-link-refs` requires a link reference definition on every versioned
  entry once any entry carries one, and says nothing about a document carrying
  none.
- `prerelease-entry` reports a pre-release version heading. It encodes a policy
  rather than catching a defect, so it is the one check that is off by default.
- Seven outputs — `valid`, `version`, `tag`, `previous`, `previous-tag`, `notes`
  and `already-tagged` — written to `$GITHUB_OUTPUT` and printed on stdout for a
  local run. `notes` is the entry body verbatim, under a delimiter drawn at
  random per value so a changelog cannot declare outputs of its own.
- Four checks that read the repository's tags: `no-git-tags` reports a checkout
  carrying none, `version-behind-tag` reports a newest entry behind the newest
  tag, and `release-entry-modified` and `prerelease-entry-modified` report a
  released entry that has changed or gone since that tag.

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

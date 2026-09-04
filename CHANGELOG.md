# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/2.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0-rc2] - 2026-09-04

### Added

- A `prerelease` output: whether the version the newest entry names carries a
  pre-release part. It is a fact about that entry, where the `prerelease-entry`
  check is a judgement about the whole document, and the difference is what a
  workflow gating on the release it is about to cut actually needs. The check
  fires on entries released long ago, which never stop being pre-releases, so a
  repository that had ever shipped a candidate would fail it forever.

### Fixed

- The published image carries `linux/amd64` and `linux/arm64`. v1.0.0-rc1 was
  built on an x64 runner and carried that platform alone, so pulling it on an
  arm64 machine failed. The action itself only ever runs on a Linux x64 runner,
  which is why the gap went unnoticed until somebody ran the image by hand.

### Changed

- The build stage cross-compiles from the builder's own architecture rather than
  running under emulation once per platform. A static Go binary cross-compiles
  for nothing, so QEMU would have bought an identical artefact at several
  minutes a platform.
- CI builds both platforms on every pull request, so an architecture that breaks
  only at release stops breaking at the moment a tag is cut.
- The test job runs on Linux alone. The action is a container action and runs
  nowhere else, so a three-platform matrix was testing a promise the project
  does not make.
- **Release candidates are cut from the pull request and final versions from
  `main`.** Every pull request builds and publishes under its commit sha, which
  is what proves the code and what fills the build cache so the build on `main`
  is a retag rather than a build. Where the changelog names a candidate, that
  build gains its version tag too, so the candidate can be pulled and tested
  while the pull request is open. The changelog appends a tag here; it does not
  decide whether to build.
- **A candidate reaching `main` is refused.** The release workflow fails on a
  newest entry naming a pre-release, so a candidate that was merged rather than
  promoted stops before anything is published.
- The build cache is a registry ref rather than the Actions cache, which is
  branch-scoped and so unreadable from `main` in exactly the direction the reuse
  has to travel.

## [1.0.0-rc1] - 2026-09-04

### Added

- Every check carries a name (`heading-form`, `date-format`, `version-order`,
  `empty-entry`, `unknown-section`), which appears in the finding and as the
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
- Five outputs (`valid`, `version`, `notes`, `already-tagged` and `latest-tag`),
  written to `$GITHUB_OUTPUT` and printed on stdout for a local run. Every one
  is something the action read: `latest-tag` is the reference tag as the
  repository spells it, and nothing proposes a tag for `version`. `notes` is the
  entry body verbatim, under a delimiter drawn at random per value so a
  changelog cannot declare outputs of its own.
- Four checks that read the repository's tags: `no-git-tags` reports a checkout
  whose tag history cannot be read, `version-behind-tag` reports a newest entry
  behind the reference tag, and `release-entry-modified` and
  `prerelease-entry-modified` report a released entry that has changed or gone
  since that tag.
- One reference tag behind all four of those and behind `latest-tag`: the newest
  version tag reachable from HEAD, which is what `git describe --tags` names.
  Reachability takes no setting, because a tag on a branch the checkout is not on
  is never the right baseline, and it is what lets a maintained support line
  compare against its own last release. `reference-tags: final|all` decides
  whether a pre-release may serve as one, defaulting to `final`, so a release
  candidate staged above the newest entry stops reporting `version-behind-tag`.
  A repository that tags no pre-release reads the same under either setting.
- `examples/` carries one worked policy: a changelog written to it, the
  main-branch and pull-request workflow invocations that enforce it, and a
  deliberately broken copy. `go test ./...` validates both documents under the
  inputs those workflows carry.
- This repository releases itself with its own action. A push to `main` runs the
  action against this changelog, and where it reports a version no tag carries,
  publishes the image, cuts the tag, creates the release from `notes` and moves
  the major tag for a final version. Nothing else cuts a tag here, and a tag
  pushed by hand is not a path. It is one workflow rather than two because a tag
  pushed with `GITHUB_TOKEN` triggers no workflow, so a cutting job and a job
  triggered by its tag would wait on each other forever.

## [0.1.0] - 2026-09-04

### Added

- `-validate` reports where a Keep a Changelog document departs from the format,
  emitting each finding as a workflow annotation under GitHub Actions.
- `-sections` accepts a level-3 heading vocabulary other than the Keep a
  Changelog six.
- A Docker container action packaging the validator, running a prebuilt image
  from `ghcr.io/mikluko/action-changelog` on a `scratch` base.

[Unreleased]: https://github.com/mikluko/action-changelog/compare/v1.0.0-rc2...HEAD
[1.0.0-rc2]: https://github.com/mikluko/action-changelog/compare/v1.0.0-rc1...v1.0.0-rc2
[1.0.0-rc1]: https://github.com/mikluko/action-changelog/compare/v0.1.0...v1.0.0-rc1
[0.1.0]: https://github.com/mikluko/action-changelog/releases/tag/v0.1.0

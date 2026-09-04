# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/2.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- `no-git-tags` fires where the repository is present and cannot be read. A
  `.git` file naming a git directory that does not resolve here (a linked
  worktree away from its parent, a submodule without the superproject, a
  container that mounted the working tree and not the repository) read as a
  repository holding no tags, so every tag-dependent check passed on a history
  nothing had read. The finding's message now carries the remedy for both
  causes and names the one this run met.

## [1.0.0] - 2026-09-04

First release. The candidates that preceded it were staging posts and are not
recorded separately: nothing consumed them but this repository.

### Added

- A GitHub Action, and a command, that reads a Keep a Changelog document and
  reports where it departs from the format. Each finding lands as a workflow
  annotation on the offending line of the diff. Run with no argument it
  validates `CHANGELOG.md`.
- **Thirteen checks, each with a name** that appears in the finding, in the
  annotation's title, and in the flags that reconfigure it. `-list-checks`
  prints the register, and the README's table is generated from that same
  register with CI failing on a stale copy, so the documentation cannot
  disagree with the binary.
- **Severity is the practitioner's, not the tool's.** `-error`, `-warn` and
  `-off` move a named check, and `-fail-on error|warning|never` decides what
  turns the step red. A check that catches a defect is an error by default; a
  check that encodes a policy is off by default, because a policy check's
  absence is the majority case being correct.
- **Six outputs**, written to `$GITHUB_OUTPUT` and printed on stdout for a local
  run: `valid`, `version`, `notes`, `already-tagged`, `prerelease` and
  `latest-tag`. Every one is something the action read, never a convention it
  chose: nothing proposes how to spell a tag, because that belongs to whatever
  cuts them. `notes` is the entry body verbatim, under a delimiter drawn at
  random per value so that a changelog cannot declare outputs of its own.
- **One reference tag** behind every tag-dependent check and behind
  `latest-tag`: the newest version tag reachable from HEAD, which is what
  `git describe --tags` names. Reachability takes no setting, because a tag on a
  branch the checkout is not on is never the right baseline, and it is what lets
  a maintained support line compare against its own last release rather than
  another line's. `reference-tags: final|all` decides whether a pre-release may
  serve as one, defaulting to `final`. A repository that tags no pre-release
  reads the same under either setting.
- Checks that read the repository as well as the document: `no-git-tags` for a
  checkout whose tag history cannot be read, `version-behind-tag` for a newest
  entry behind the reference tag, and `release-entry-modified` and
  `prerelease-entry-modified` for a released entry that has changed or gone
  since it. A released entry is immutable; an entry added for a version already
  tagged is not, because recording history nobody wrote down is a repair rather
  than a rewrite.
- A Docker container action on a `scratch` base, published for `linux/amd64` and
  `linux/arm64`. It reads git through go-git rather than by shelling out, so the
  image carries the binary and nothing else.
- [`examples/`](examples/) carries one worked policy: a changelog written to it,
  the two workflow invocations that enforce it, and a deliberately broken copy.
  A test resolves the inputs out of those workflow files and runs the linter
  under them, so the example is executed rather than described.
- **This repository releases itself with its own action.** A push to `main` runs
  the action against this changelog and, where it names a version no tag
  carries, publishes the image, cuts the tag and creates the release from
  `notes`. Release candidates are cut from the pull request instead, so a
  candidate can be pulled and tested before it merges, and one reaching `main`
  is refused rather than published.

[Unreleased]: https://github.com/mikluko/action-changelog/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/mikluko/action-changelog/releases/tag/v1.0.0

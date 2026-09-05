# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/2.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **`unreadable-version`, a check of its own for an entry heading naming no
  version that can be read**, at `error`, which leaves `heading-form` the shape
  question its name is about. One check was doing both jobs and only one of them
  is a policy: `heading-form` is about the form of a heading, which a repository
  may reasonably differ on and switch off, and switching it off also switched
  off the only thing standing between a heading nothing can read and a release
  described from the wrong entry. It is the split `undated-entry` and
  `undated-release` made, for the same reason. Sixteen checks.

### Fixed

- **The `release-branch` example cuts the version its changelog names, and
  composes nothing.** It derived a pre-release tag from `github.run_number` and
  from the branch name's last hyphenated segment, and both were wrong: the run
  number counts a workflow file's runs and resets when the file is renamed, and
  `${GITHUB_REF_NAME##*-}` returns the whole ref on a branch carrying no hyphen,
  so `main` composed the legal and meaningless `v1.3.0-main.7`. Neither is
  repaired, because composing at all contradicted this action's thesis that
  everything downstream follows from the document. Writing `1.3.0-rc.2` in the
  heading is what cuts `v1.3.0-rc.2`; the branch and trunk invocations are the
  same two guards with `prerelease` inverted; and a push that changes no version
  cuts nothing, where the run number cut a candidate on every push.
- `prerelease-entry` moves to the trunk invocation alone. A candidate is a
  heading under this strategy now, so raising it on the branch or on pull
  requests would refuse the shape the strategy is built on. The cost is that the
  check judges the whole document, so the branch cannot report a stale candidate
  below the newest entry either.
- Where the newest entry's heading names no version that can be read, `version`
  and `notes` report nothing rather than describing the entry below it. That
  entry was being read as the newest, so a workflow reading those cut a tag for
  the wrong release and published the previous release's notes under it. Nothing
  under such a heading is the newest entry, and reporting nothing is what a
  document naming no version already does, so a workflow guarding on an empty
  `version` lands on the path it already has. `already-tagged` and `prerelease`
  answer `false` beside it, as they do for a document naming no version.

## [1.1.0] - 2026-09-05

### Added

- **`undated-entry` and `undated-release`, a pair the repository's tags decide
  between** where the newest entry names a version and carries no date. No tag
  names that version yet, and the entry is open: a release still accumulating on
  a branch of its own, which names its version before its date is known.
  `undated-entry` reports it, defaulting to `error` because an entry with no
  date is illegal under Keep a Changelog, and **a branch accumulating a release
  is the one invocation that switches it off** in `off:`, the way a trunk
  invocation switches `prerelease-entry` on. A tag already names that version,
  and the release shipped and nobody dated it: `undated-release` reports that,
  also at `error`, and switching the other check off does not silence it.
  Fifteen checks.
- The two are separate because one setting cannot serve both. A stabilization
  branch has to admit its open entry on every run, and a release that reached a
  tag with no date is a defect on exactly that workflow, so a single check would
  be switched off for the whole of the window in which the defect occurs. The
  same tag question answers both, and it is the one `already-tagged` reports, so
  the check and the output cannot disagree about which case a run is in.
- Where the tag history cannot be read, the answer is unknowable and the entry
  is reported as open. `no-git-tags` already names that cause, and accusing a
  repository of shipping an undated release on the strength of tags nobody could
  read is the more expensive of the two mistakes.

### Changed

- `heading-form` accepts `## [1.2.3]` with no date on the newest versioned entry
  only, which is the entry the two checks above then answer for. Every entry
  below it has shipped and carries a date, so one missing there still fires
  `heading-form`.

## [1.0.2] - 2026-09-04

### Fixed

- Where two tags name one version, the reference tag is the one that spells it
  most fully. A repository following the GitHub Actions convention carries `v1`
  beside `v1.0.0`, both read as `v1.0.0`, and which of them won was whatever
  order the refs happened to arrive in. `latest-tag` could therefore report the
  moving major tag, which is documented as the ref a workflow can check out and
  is the one that moves to the next release under whoever checked it out.

## [1.0.1] - 2026-09-04

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

[Unreleased]: https://github.com/mikluko/action-changelog/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/mikluko/action-changelog/compare/v1.0.2...v1.1.0
[1.0.2]: https://github.com/mikluko/action-changelog/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/mikluko/action-changelog/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/mikluko/action-changelog/releases/tag/v1.0.0

# action-changelog

Reads a [Keep a Changelog](https://keepachangelog.com/en/2.0.0/) document and
reports where it departs from the format. Each finding is emitted as a workflow
annotation, so a failing check lands on the offending line of the diff.

## Usage

```yaml
- uses: actions/checkout@v5
  with:
    fetch-depth: 0
- uses: mikluko/action-changelog/actions/validate@v2
  with:
    changelog: CHANGELOG.md
```

`fetch-depth: 0` brings the tags the action compares the changelog against. A
default checkout is shallow and carries none, and `no-git-tags` fires.

On Forgejo the same step names the action by its full URL:

```yaml
- uses: https://sarak.sh/mikluko/changelog/actions/validate@v2
```

The action is developed at
[sarak.sh/mikluko/changelog](https://sarak.sh/mikluko/changelog),
and [`mikluko/action-changelog`](https://github.com/mikluko/action-changelog) on
GitHub is its mirror, carrying the same tags and releases.

Each action lives in a directory of its own under `actions/`: `validate` reads
the changelog, and [`append` and `release`](#writing-the-changelog) write it.

A composite action that downloads a static binary from its release and runs it.
It runs on Linux and macOS runners, X64 or ARM64, and needs bash, curl, and
`sha256sum` or `shasum`. It reaches the network only to download its own binary:
from GitHub first on GitHub and from Forgejo first everywhere else, falling back
to the other where the first fails.

[`examples/`](examples/) carries complete workflows for
[two release strategies](#two-worked-policies).

## Why

Changelog tooling writes the file and re-parses none. The version comes from
commit messages, from fragment files, or from a tag somebody already cut, and the
changelog is the record written afterwards. Of 29 widely used projects surveyed,
exactly one inverts that, and it does so by hand.

This takes the other direction. The author writes `## [1.2.3] - 2026-09-04`, and
everything downstream follows from the file: the version, the notes, whether it
is already tagged. A changelog that falls behind its tags then blocks the
pipeline rather than drifting quietly, which is the failure this exists to make
impossible.

It performs no release itself. It reports what it found, and the ceremony
belongs downstream, to whatever automation consumes that metadata.

## Inputs

Every input is optional.

| Input | Default | Description |
|---|---|---|
| `changelog` | `CHANGELOG.md` | Path to the changelog, relative to the workspace. |
| `fragments` | `.changelog` | Directory of [fragments](#fragments) to check, relative to the workspace. Empty checks none. |
| `base` | `${{ github.base_ref }}` | The branch a pull request merges into, which `missing-fragment` reads the merge base with. Empty outside a pull request. |
| `sections` | *(empty)* | Comma-separated level-3 headings to accept. Empty accepts the Keep a Changelog six: Added, Changed, Deprecated, Removed, Fixed, Security. |
| `error` | *(empty)* | Comma-separated checks to raise as errors. |
| `warn` | *(empty)* | Comma-separated checks to raise as warnings. |
| `off` | *(empty)* | Comma-separated checks to switch off. |
| `fail-on` | `error` | What turns the step red: `error`, `warning`, or `never`. |
| `reference-tags` | `final` | Which tags may be the reference tag: `final`, or `all` to admit pre-releases. |
| `due-prerelease` | `false` | Whether a pre-release counts as a release that is due: `true` or `false`. |

`Breaking` is not in the default vocabulary. Keep a Changelog 2.0.0 marks a
breaking change inline as `**Breaking:**` inside the section it belongs to;
repositories that use it as a heading pass it in `sections`.

`error`, `warn` and `off` are applied in that order, so a check named in two of
them takes the later spelling. A name no check carries is refused rather than
ignored. `fail-on: never` reports every finding and exits 0.

## The reference tag

Everything that compares the changelog against the repository's history reads
one tag: the newest version tag **reachable from HEAD**, which is what
`git describe --tags` names. `version-behind-tag` compares against it,
`release-entry-modified` and `prerelease-entry-modified` take the changelog as
it stood there as their baseline, and `latest-tag` reports it.

`undated-release` is the one exception. It asks whether any tag names the newest
entry's version, reachable or not, because a release is tagged whether or not
the tag naming it is reachable from here. That is the set `already-tagged` is
answered from, so the check and that output cannot disagree about which case a
run is in.

Reachability takes no setting, because a tag on a branch this checkout is not on
is never the right baseline. It is what makes a maintained support line work: on
`support/1.x` the reference is that line's own newest release, and the trunk's
`v2.0.0` is simply unreachable.

`reference-tags` decides only whether a pre-release may serve as one. Under
`final`, a `v2.2.0-rc.1` staged above the newest entry is not a baseline, so a
changelog topping out at `2.1.0` is not behind anything; under `all` it is. A
repository that tags no pre-release reads the same either way.

A shallow checkout is the case this cannot see past: it carries the history it
was given rather than the one that exists, so a reference it cannot reach fires
`no-git-tags`, which names `fetch-depth: 0` as the fix.

`no-git-tags` also fires where the repository is present and cannot be read. A
linked worktree, a submodule and a container mount all reach the repository
through a `.git` file naming a directory elsewhere, and where that directory is
out of reach the tags are unreadable rather than absent. The finding names the
cause it met, and the fix there is to bring that directory into reach rather
than to fetch anything.

## Outputs

| Output | Example | Description |
|---|---|---|
| `valid` | `true` | Whether the document conforms: `true` when nothing was found at error severity. Reported whatever `fail-on` is set to. |
| `version` | `1.2.3` | The version the newest entry names. |
| `notes` | | The body of the newest entry, verbatim. |
| `already-tagged` | `false` | Whether a tag naming `version` already exists. |
| `prerelease` | `false` | Whether `version` carries a pre-release part, as in `1.2.3-rc1`. |
| `latest-tag` | `v1.2.2` | The reference tag, as the repository spells it. |
| `due` | `true` | Whether the newest entry is a release to cut. |
| `due-reason` | `1.2.3 is already tagged` | One sentence saying why `due` is `false`. Empty while it is `true`. |

Every output is something the action read. None of them proposes a tag: how a
repository spells its tags belongs to whatever cuts them, so a workflow wanting
a ref reads `latest-tag` and a workflow cutting a new one writes the spelling it
has chosen.

`prerelease` is a fact about the newest entry, where the `prerelease-entry`
check is a judgment about the whole document. A workflow gating on what it is
about to release wants the fact: the check also fires on entries released long
ago, which never stop being pre-releases, so a repository that has ever shipped
a candidate would fail that check forever.

`latest-tag` is [the reference tag](#the-reference-tag), so `already-tagged` says
which case a consumer is in: the previous release while it is `false`, and the
release just cut once it is `true`. Nothing strips the `v`, because the value
names a ref that has to resolve. It comes from the repository's tags rather than
from the entry below the newest, because a changelog whose history begins partway
through has no second entry to offer. Tags are compared by the version they name,
so a repository tagging `1.2.3` is read the same as one tagging `v1.2.3`.

`already-tagged` is answered from every tag the checkout carries rather than from
the reference alone, so a release cut as a pre-release still reports `true` for
the entry naming it.

`due` is `true` when the newest entry names a version, no tag names that
version, and it is not a pre-release; `due-prerelease: true` lets a pre-release
count. Where it is `false`, `due-reason` says which of those failed and the same
sentence is written to the step summary. It is also `false` where the
repository's tags cannot be read, because no tag seen is not the same as no tag.
It says nothing about `valid`: under `fail-on: never` a workflow reads both.

It is also `false` while `pending-date-mismatch` fires, and `due-reason` is that
check's message. A release PR merged on a later day than its heading names would
otherwise be tagged anyway, and then `date-mismatch` would ask for the tag's day
while `release-entry-modified` forbade the edit. The day a tag cut on `HEAD`
would carry is known only for a lightweight tag, which is dated by the committer
date of the commit it names, in the offset that commit records. For an annotated
tag, dated when it is cut, the check is silent, and so is it where the repository
has no version tag to say which kind it cuts, and where the changelog differs
from the one `HEAD` records, as a release folded in and not yet committed does:
the tag is cut on the commit that will carry it, not on `HEAD`. A repository whose release merged
late repairs the entry before any tag exists: it opens a pull request dating the
heading with the day the merge commit will carry, and merges it that day.

```yaml
- id: changelog
  uses: mikluko/action-changelog/actions/validate@v2
- if: steps.changelog.outputs.due == 'true'
  env:
    VERSION: ${{ steps.changelog.outputs.version }}
  run: |
    git tag "v$VERSION"
    git push origin "refs/tags/v$VERSION"
```

A document naming no version is still validated: `valid`, `already-tagged` and
`due` answer, and `version` and `notes` are empty. A document whose newest entry
carries a heading naming no version that can be read answers the same way, and
`unreadable-version` reports that heading: nothing under it is the newest entry,
so no output describes the entry below it.

The values are written to `$GITHUB_OUTPUT` and printed on stdout, which is where
a local run reads them.

## Checks

Each check carries a name, which appears in the finding and in the workflow
annotation, and which is what `error`, `warn` and `off` take.

<!-- checks:start -->

| Check | Default | Description |
|---|---|---|
| `unreadable-version` | `error` | An entry heading is [Unreleased] or names a Semantic Versioning 2.0.0 version, as in [1.2.3]: three components, no leading v. Where the newest one names none, nothing is the newest entry and version and notes report nothing. |
| `heading-form` | `error` | An entry heading states a date beside its version, as in [1.2.3] - 2006-01-02. The newest entry may omit the date, which undated-entry and undated-release answer for. |
| `date-format` | `error` | An entry's date is written YYYY-MM-DD. |
| `version-order` | `error` | Entries run newest first, each version strictly below the one above it. |
| `empty-entry` | `error` | A released entry carries something under it. |
| `unknown-section` | `error` | A level-3 heading is one of the accepted section vocabulary. |
| `duplicate-section` | `error` | A level-3 heading names no section that another one under the same entry already named. Silent on an entry the reference tag has frozen, whose repair release-entry-modified forbids. |
| `date-order` | `error` | Entry dates run newest first, matching the version order above them. |
| `date-future` | `error` | No entry is dated later than today. |
| `date-mismatch` | `error` | A released entry's date is the day its tag was cut. Silent for an entry no tag names, which is a release pending rather than a date that disagrees. |
| `partial-link-refs` | `error` | Every versioned entry has a link reference definition, once any entry does. |
| `prerelease-entry` | `off` | Policy: no entry names a pre-release version. Off by default; pre-release headings are legal. |
| `oci-incompatible-version` | `error` | A version can be an OCI tag. This is the one check whose subject is outside the document: build metadata is valid Semantic Versioning and the OCI tag grammar [a-zA-Z0-9_][a-zA-Z0-9._-]{0,127} has no +, so such a version cuts a git tag and is then not a name a registry takes. Switch it off where nothing is published. |
| `undated-entry` | `error` | Policy: the newest entry states a date, where no final tag yet names its version. Switched off, that entry is open: a release still accumulating on a branch of its own, whether or not a candidate has been cut from it. |
| `fragment-form` | `error` | Each file in the fragments directory is a fragment: a Markdown file carrying level-3 section headings, each with a bullet list of notes, and nothing else. Silent where no fragments directory is given. |
| `fragments-release` | `error` | Releasing what is pending today, with the fragments folded in, gives a changelog that raises nothing the changelog as it stands does not. What is pending is the open entry, closed with today's date, where there is one, and otherwise [Unreleased] at the version its notes call for. Silent where nothing is pending, and where no fragments directory is given. |
| `undated-release` | `error` | The newest entry states a date, where a final tag already names its version. The release shipped and nobody dated it. A candidate tag is not one: it ships undated, which is what a stabilization branch is for. |
| `no-git-tags` | `error` | The repository's tag history can be read, which every check comparing against it needs. |
| `version-behind-tag` | `error` | The newest entry is not behind the reference tag. |
| `release-entry-modified` | `error` | A released entry is unchanged since the reference tag. |
| `prerelease-entry-modified` | `error` | A released pre-release entry is unchanged since the reference tag. |
| `pending-date-mismatch` | `error` | A release that is due is dated the day a tag cut on HEAD would carry: the newest entry, where no tag names its version, states the committer date of HEAD, in the offset HEAD records, which is the day a lightweight tag cut there carries. While it fires the release is not due. Silent where the day is not known before the tag is cut, which is an annotated tag's, where the repository has no version tag to say which kind it cuts, for an entry the reference tag has frozen or one stating no date, and where the changelog is not the one HEAD records, as a release folded in and not yet committed is, since HEAD is then not the commit the tag will be cut on. |
| `missing-fragment` | `error` | A pull request carries notes: a fragment its merge base with the base branch lacks, or a note the changelog there lacks, which is how a release branch assembling its entry carries them. Silent where no base branch is given, which is outside a pull request, and where no fragments directory is. |

<!-- checks:end -->

## SemVer compliance

`version` is a Semantic Versioning 2.0.0 string. Several widely used tools
diverge from that specification in ways documented where each tool lives and
nowhere near the specification itself, so a reader holding a valid version has
no way to meet them except by running into one.

Each entry below states what a named tool does, under what input. What that
means for any particular arrangement is the reader's to judge.

### What this action accepts

A version heading is read as Semantic Versioning 2.0.0 exactly: three
components, no shorthand, no leading `v`.

| Heading | Reading | Reason |
|---|---|---|
| `## [1.2.3] - 2026-01-01` | accepted | three numeric components |
| `## [1.2.3-rc.1] - 2026-01-01` | accepted | a pre-release is a version |
| `## [1.2.3+build.1] - 2026-01-01` | accepted | build metadata is valid SemVer |
| `## [1.2]` | rejected | a version states all three components |
| `## [v1.2.3]` | rejected | the leading `v` belongs to a tag, not to the specification |
| `## [01.2.3]` | rejected | a numeric identifier carries no leading zero |

Two departures from the specification are this project's own:

- **A component past 2^64-1 is rejected**, where the specification sets no upper
  bound on major, minor or patch.
- **A git tag is read more loosely than a heading.** A tag namespace carries a
  leading `v`, and the GitHub Actions convention keeps a moving `v1` beside the
  `v1.0.0` it points at, so `v1`, `v1.2` and `v1.2.3` all read as versions when
  they name a tag. Where two tags name one version, the fuller spelling is the
  one taken to have cut the release.

### `golang.org/x/mod/semver`

Its package documentation declares two deviations from Semantic Versioning
2.0.0: it **requires a leading `v`**, and it recognizes **`vMAJOR` and
`vMAJOR.MINOR`** — with no pre-release or build suffix — as shorthands for
`vMAJOR.0.0` and `vMAJOR.MINOR.0`. It describes itself as implementing
*comparison* of version strings rather than validation.

`Canonical` fills in a missing `.MINOR` or `.PATCH` and **discards build
metadata**, so `v1.2.3+build.1` canonicalizes to `v1.2.3`, and the package
documents that two versions compare equal only if their canonical forms are an
identical string.

Read against `golang.org/x/mod` v0.40.0.

### `Masterminds/semver/v3`

A constraint admits a pre-release version only where the **constraint's own
literal carries a pre-release token**. The width of the range does not enter
into it.

| constraint | `1.2.3` | `1.2.3-rc.1` | `1.2.3+build.1` |
|---|---|---|---|
| `*` | ✓ | ✗ | ✓ |
| `>=0.0.0` | ✓ | ✗ | ✓ |
| `1.2.x` | ✓ | ✗ | ✓ |
| `^1.0.0` | ✓ | ✗ | ✓ |
| `>=1.0.0 <2.0.0` | ✓ | ✗ | ✓ |
| `>=0.0.0-0` | ✓ | ✓ | ✓ |
| `1.2.x-0` | ✓ | ✓ | ✓ |
| `^1.0.0-0` | ✓ | ✓ | ✓ |
| `1.2.3-rc.1` | ✗ | ✓ | ✗ |

The library documents this as deliberate, following npm and Cargo rather than
strict SemVer precedence. Build metadata is ignored throughout, per section 10.

Measured against v3.5.0.

### Selecting from a range is not matching against one

Where a tool resolves a range by taking the highest version that matches, two
rules apply in order: the constraint decides what matches at all, and section 11
then ranks a pre-release **below** the version it qualifies. So a range that
admits both a final release and its pre-releases resolves to the final.

FluxCD's `OCIRepository.spec.ref.semver` is one such. `getTagBySemver` in
`internal/controller/ocirepository_controller.go` builds a constraint with
`Masterminds/semver/v3`, keeps every tag that satisfies it, sorts the matches in
reverse and takes the first. Both rules above therefore apply to it, the first
because of which library it is: `semver: "*"` collects no pre-release tag at all.

Read against `fluxcd/source-controller` v1.9.5.

### OCI tag grammar

The OCI distribution specification requires a tag to be at most 128 characters
and to match:

```
[a-zA-Z0-9_][a-zA-Z0-9._-]{0,127}
```

`+` is not in that grammar, so a version carrying build metadata is not an OCI
tag verbatim. Git ref syntax does permit `+`: `git check-ref-format
refs/tags/v1.2.3+build.1` succeeds. The two disagree about the same string.

`oci-incompatible-version` is where this action acts on that, and it is the one
check whose subject is outside the document. A repository that publishes no
image or chart switches it off.

## Two worked policies

[`examples/`](examples/) carries two named release strategies, each as a
complete worked example: a changelog written to it, the workflow invocations
that enforce it, and a deliberately broken copy.

[`release-trunk/`](examples/release-trunk/) releases on the trunk. The newest
entry names a version and a date, a push to the trunk finds that version
untagged, and the tag is cut. One branch, one ceremony.

```
one entry, one tag

## [1.2.0] - 2026-08-01   ──▷   v1.2.0
## [1.3.0] - 2026-09-12   ──▷   v1.3.0

the entry is written complete, date and all, and merging cuts what it names
```

[`release-branch/`](examples/release-branch/) opens a release on a branch of its
own. The entry names the version it is heading for and carries no date while the
branch accumulates, every pull request into that branch cuts a numbered
candidate — `v1.3.0-rc.1`, `v1.3.0-rc.2` — and merging to the trunk dates the
entry and cuts `v1.3.0`. The document names the release; the machine numbers the
attempts, so a heading never carries an identifier.

```
one entry, many tags

## [1.3.0]                ──▷   v1.3.0-rc.1     one per pull request
  no date, for as long    ──▷   v1.3.0-rc.2     into release/v1.3.0-rc
  as the branch lives     ──▷   v1.3.0-rc.3

## [1.3.0] - 2026-09-12   ──▷   v1.3.0          the merge dates that
                                                same entry, and that is
                                                the release
```

`go test ./...` validates both trees under the inputs their workflows carry, so
the examples are executed rather than described.

## Writing the changelog

Two more actions in this repository edit the document rather than read it. Each
changes the file in the workspace and nothing else: committing the result is the
workflow's to do. Every byte outside the lines they write is preserved.

### Append

Adds a note under a section of the `[Unreleased]` entry:

```yaml
- uses: mikluko/action-changelog/actions/append@v2
  with:
    section: Fixed
    note: The parser no longer reads a fenced heading as an entry.
```

| Input | Default | Description |
|---|---|---|
| `changelog` | `CHANGELOG.md` | Path to the changelog, relative to the workspace. |
| `section` | *(required)* | The level-3 heading to add the note under. |
| `note` | *(required)* | The note, without its list marker. |

A missing section is created in Keep a Changelog order, and a missing
`[Unreleased]` entry above the newest entry. A multi-line note stays one list
item. A section already carrying the same item is left as it is, so a rerun
adds nothing.

### Release

Turns the `[Unreleased]` entry into a versioned one and opens a new, empty
`[Unreleased]` above it:

```yaml
- uses: mikluko/action-changelog/actions/release@v2
  id: release
```

| Input | Default | Description |
|---|---|---|
| `changelog` | `CHANGELOG.md` | Path to the changelog, relative to the workspace. |
| `fragments` | `.changelog` | Directory of [fragments](#fragments) to fold into `[Unreleased]` first and delete. Empty folds none. |
| `version` | *(empty)* | The version to release. Empty computes it from the notes. |
| `date` | *(empty)* | The release date as `YYYY-MM-DD`. Empty is today in UTC. |
| `open` | `false` | `true` writes an [open entry](#two-worked-policies), naming the version and no date. |

| Output | Example | Description |
|---|---|---|
| `version` | `1.3.0` | The version released. |
| `notes` | | The body of the released entry, verbatim. |
| `prerelease` | `false` | Whether `version` carries a pre-release part. |

A computed version is bumped from the newest entry naming a final version:

| The `[Unreleased]` entry carries | Bump |
|---|---|
| a `Breaking` section, or a note opening with `**Breaking:**` | major; minor below 1.0.0 |
| any section other than `Fixed` and `Security` | minor |
| `Fixed` and `Security` only, or no sections | patch |

Where the `[Unreleased]` link reference is a compare URL ending in `...HEAD`, it
is moved to start at the new tag and a link reference for the release is added
below it. The tag is spelled with a leading `v` exactly when the previous one in
that URL is.

The action refuses an `[Unreleased]` entry with no notes, a version not above
every version the document names, and a document whose newest versioned entry
is still open.

### Fragments

Pull requests that each add a note under `[Unreleased]` conflict with one
another on the same lines. A fragment moves the note out of `CHANGELOG.md`
into a file of its own, which nothing else touches, and the release stacks
every fragment into the changelog.

A fragment is a Markdown file in one directory, `.changelog/` unless the
`fragments` input names another, carrying level-3 section headings and the
notes listed under them, exactly as they would read under `[Unreleased]`:

```markdown
### Fixed

- The parser no longer reads a fenced heading as an entry.
```

The file name is the author's to choose, and it orders the fragments: the
release folds them in name order. Anything else in the file, a note under no
section, or a file in the directory not named `*.md`, is refused rather than
dropped. Names starting with a dot are skipped, so a `.gitkeep` can hold the
directory open.

The reader holds the fragments to three checks, so a fragment that would not
release fails the pull request adding it rather than the release:

| Check | Reports |
|---|---|
| `fragment-form` | a file in the directory that is not a fragment, against that file |
| `unknown-section` | a fragment's heading outside `sections`, against that file |
| `fragments-release` | what releasing what is pending today, fragments folded in, would raise that the changelog as it stands does not, against the heading of the entry released |

What is pending is the [open entry](#two-worked-policies), closed with today's
date, where there is one, and otherwise `[Unreleased]` at the version its notes
call for.

A fourth, `missing-fragment`, fails a pull request that carries no notes: it
adds no fragment missing from the merge base with the `base` branch, and the
changelog carries no note it lacked there. It is silent outside a pull request,
and needs the base branch in the checkout, which `fetch-depth: 0` brings. A
repository writing its notes into `CHANGELOG.md` rather than into fragments sets
`fragments` empty, which silences all four:

```yaml
- uses: mikluko/action-changelog/actions/validate@v2
  with:
    fragments: ""
```

The release action folds every fragment into `[Unreleased]` as `append` would
one note at a time, releases the result, and deletes the fragments it folded:

```yaml
- uses: mikluko/action-changelog/actions/release@v2
```

One fragment it cannot read refuses the release with nothing written. A
fragment carries notes and never a version: the version is computed from the
folded notes and written into the changelog, and the tag still follows the file.

#### On a release branch

A release prepared on a branch of its own names its version in an open entry
from the start, and its notes arrive as fragments, as edits to that entry, or
both. The `fold` subcommand appends every fragment to the open entry, or to
`[Unreleased]` where no entry is open, and deletes the fragments:

```
go run sarak.sh/mikluko/changelog/v2/cmd/changelog@latest fold
```

The branch stays under all four checks. `fragments-release` forecasts the open
entry closed today with the remaining fragments folded in, and the pull request
merging the branch, which consumes fragments rather than adding them, passes
`missing-fragment` by the notes it writes into the changelog.
[`examples/release-branch/`](examples/release-branch/) is the strategy worked
in full.

## Migrating from 1.x

| 1.x | 2.x |
|---|---|
| `mikluko/action-changelog@v1` | `mikluko/action-changelog/actions/validate@v2` |
| `mikluko/action-changelog/append@v1` | `mikluko/action-changelog/actions/append@v2` |
| `mikluko/action-changelog/release@v1` | `mikluko/action-changelog/actions/release@v2` |
| `https://sarak.sh/mikluko/changelog@v1` | `https://sarak.sh/mikluko/changelog/actions/validate@v2` |
| `sarak.sh/mikluko/changelog` | `sarak.sh/mikluko/changelog/v2` |

The Forgejo paths of the writers move the same way. `@v1` keeps resolving to
the 1.x paths. The Go module carries the `/v2` suffix a major version above 1
requires.

[Fragments](#fragments) are on by default: the reader and the release action
read `.changelog/`, and on a pull request `missing-fragment` fails one carrying
no notes. A repository that writes its notes into `CHANGELOG.md` keeps doing so
with `fragments: ""` on each invocation. The checks added in 2.0.0, which
`error`, `warn` and `off` now accept, are `fragment-form`, `fragments-release`
and `missing-fragment`.

## Local use

The action is a Go command, and every input is a flag of the same name.

To validate `CHANGELOG.md` in the current directory:

```
go run sarak.sh/mikluko/changelog/v2/cmd/changelog@latest
```

Findings go to stderr and the [outputs](#outputs) to stdout. The exit status is
1 where a finding reaches `-fail-on`.

To validate a changelog elsewhere, where `<path>` is its path:

```
go run sarak.sh/mikluko/changelog/v2/cmd/changelog@latest -changelog <path>
```

To print the checks and their default severities:

```
go run sarak.sh/mikluko/changelog/v2/cmd/changelog@latest -list-checks
```

The writers are subcommands, taking their inputs as flags of the same name:

```
go run sarak.sh/mikluko/changelog/v2/cmd/changelog@latest append -section Fixed -note "<note>"
go run sarak.sh/mikluko/changelog/v2/cmd/changelog@latest fold
go run sarak.sh/mikluko/changelog/v2/cmd/changelog@latest release
go run sarak.sh/mikluko/changelog/v2/cmd/changelog@latest release -fragments .changelog
```

The Go packages the command is built on are importable:
`sarak.sh/mikluko/changelog/v2/pkg/changelog` parses, checks and edits a
document, and `sarak.sh/mikluko/changelog/v2/pkg/semver` and
`sarak.sh/mikluko/changelog/v2/pkg/git` read versions and a repository's tags.

## Releasing

This repository releases itself with its own action, which is the shortest
statement of what the action is for.

`actions/VERSION` names the release whose binary every action downloads, so that
reference has to be immutable and correct at the moment it is committed: a
workflow pinning an action by SHA reads the file as it stands at that commit,
and nothing is rewritten onto `main` afterwards. A release is therefore one
commit carrying two things a test holds together: `CHANGELOG.md` naming the
version in its newest entry, and `actions/VERSION` naming the same one.

Every pull request adds a [fragment](#fragments), and the release workflow on
Forgejo, dispatched by hand from `main`, writes that commit: it folds the
fragments into `CHANGELOG.md` at the version their notes call for, or at the
one the dispatch names, writes `actions/VERSION`, validates the changelog, runs
`go test ./...` and builds the binaries for Linux and macOS on amd64 and arm64.
Only then does it push the commit to `main`, cut the tag, create the release
from the entry's notes with the binaries and their `checksums.txt` attached,
run the actions from that release, and move the major tag. Nothing else cuts a
tag here, and a tag pushed by hand is not a path.

A run that fails after pushing the commit leaves `main` naming a version no tag
carries, and dispatching again resumes at the tag rather than folding anew.

The tag reaches GitHub through the mirror, and a workflow there copies the
Forgejo release's notes and assets onto a GitHub release of the same tag.

This works because the changelog names the version before the tag does, which is
the inversion the tool exists to support.

## Contributing

Contributions are welcome. This is maintained in spare time and there is no
commitment to review a pull request promptly, so an issue first is usually the
cheaper way to find out whether a change is wanted.

Every pull request carries its notes as a [fragment](#fragments) in
`.changelog/`, never as an edit to `CHANGELOG.md`, which only a release writes.

AI-assisted contributions are welcome too, and are asked to be attributed: name
the tool in the pull request, or leave a `Co-authored-by:` trailer on the
commits. What matters is that a reviewer knows what they are reading.

## License

MIT. See [LICENSE](LICENSE).

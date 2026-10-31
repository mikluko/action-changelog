# Changelog-driven releases

A Keep a Changelog document as the source a release's version comes from: what
reading it reports, what writing to it changes, and the two ceremonies that turn
its newest entry into a tag.

## Language

### The document

**Changelog**:
The document a release is written in: a title, an `[Unreleased]` entry, and the
versioned entries below it, newest first.
_Avoid_: release notes (one entry's body), history file

**Entry**:
One level-2 heading and everything under it, up to the next level-2 heading.
_Avoid_: release (an entry describes one), version (an entry names one), section

**Version heading**:
The level-2 heading line of an entry, naming its version and, once it has one,
its date, as in `## [1.2.3] - 2006-01-02`.
_Avoid_: title, header

**Section**:
A level-3 heading under an entry, grouping its notes by the kind of change:
Added, Changed, Deprecated, Removed, Fixed or Security.
_Avoid_: category, group, change type

**Note**:
One list item under a section, stating one change.
_Avoid_: line, bullet, change (which is what a note states)

**Breaking marker**:
The `**Breaking:**` opening a note whose change breaks compatibility, kept
inside the section the change belongs to. A repository may instead give such
notes a `Breaking` section of their own.
_Avoid_: breaking label

**Link reference definition**:
A `[1.2.3]: https://…` line at the foot of the document turning a bracketed
version heading into a link; a document defines one per versioned entry or none.
_Avoid_: compare link (what one usually points at), footnote

**`[Unreleased]`**:
The entry naming no version, standing permanently at the top of the document
and collecting notes no release carries yet.
_Avoid_: unreleased version, next release, pending entry

**Open entry**:
An entry naming a version and no date: a release being written whose ship date
is not known yet, and only ever the newest versioned entry.
_Avoid_: staging entry, draft entry, unreleased version, release candidate

**Released entry**:
An entry naming a version and a date, which no longer changes.
_Avoid_: published entry, historical entry, closed entry

**Pre-release**:
A version carrying a pre-release part, as in `1.2.3-rc1`, and every entry and
tag naming one.
_Avoid_: release candidate (one spelling of one), beta, unstable

### Reading

**Reading**:
One pass over a changelog that reports its findings and the facts its newest
entry and the repository's tags give, and changes nothing.
_Avoid_: lint (the findings alone), validation

**Check**:
One named thing a reading looks for: a defect, on by default, or a policy, off
by default.
_Avoid_: rule, validator, lint

**Register**:
Every check there is, with its description and default severity.
_Avoid_: list, catalog, ruleset

**Severity**:
What a check's finding counts as: error, warning, or off, which is no finding at
all.
_Avoid_: level, priority

**Finding**:
One departure from the format, at the line that carries it, naming the check
that raised it.
_Avoid_: error, warning (those are severities), violation, annotation (what a
finding becomes on a diff)

**Reference tag**:
The newest version tag reachable from HEAD, pre-releases admitted or not: the one
tag a reading compares the changelog against.
_Avoid_: previous tag, baseline (bare), latest release

**Due**:
Whether the newest entry is a release to cut: it names a version no tag names,
and is not a pre-release unless pre-releases are admitted.
_Avoid_: releasable, pending, needs release

### Writing

**Writer**:
An edit to the changelog that leaves everything outside the lines it writes as
it was, and changes nothing but the file.
_Avoid_: formatter, generator (neither leaves the rest alone)

**Append**:
The writer adding a note under a section of `[Unreleased]`.
_Avoid_: add entry (it adds a note, not an entry)

**Release** (the writer):
The writer turning `[Unreleased]` into a released or open entry and leaving an
empty `[Unreleased]` above it.
_Avoid_: promote, cut (a ceremony cuts the tag)

**Bump**:
The version release computes when given none: from the newest final version,
major for a breaking marker or a `Breaking` section, minor for any section but Fixed and Security, patch
otherwise.
_Avoid_: next version (bare), increment

### Ceremonies

**Ceremony**:
What turns the newest entry into a release downstream of a reading: finding its
version untagged, cutting the tag, publishing the release.
_Avoid_: release process, pipeline, release job

**Trunk**:
The branch a repository's history converges on, and the only one a final tag is
cut from.
_Avoid_: master, mainline, default branch (a repository setting)

**Stabilization branch**:
A short-lived branch carrying one release while it accumulates, deleted once it
merges to the trunk.
_Avoid_: release branch (release-branch is the ceremony), pre-release branch, RC
branch, support line

**Candidate**:
A numbered pre-release tag a release-branch ceremony cuts per pull request into
a stabilization branch, composed from the open entry's version and never written
as a heading.
_Avoid_: RC (one spelling of one), snapshot

**Release-trunk**:
The ceremony with one branch: a push to the trunk finds the newest released
entry untagged and cuts its tag.
_Avoid_: continuous release, trunk-based release

**Release-branch**:
The ceremony with two: an open entry on a stabilization branch yields a candidate
per pull request, and merging to the trunk dates the entry and cuts the release.
_Avoid_: release train, GitFlow release, pre-release branch (the branch, not the
ceremony)

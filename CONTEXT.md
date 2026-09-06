# Changelog-driven releases

The domain this repository works in: a Keep a Changelog document, what a reading
of it reports, and the two ceremonies that turn its newest entry into a tag.
This file fixes what the words mean and how the things they name relate; how the
code does any of it is the code's to say.

Two ceremonies share the vocabulary. Where a word belongs to one of them alone,
the entry says which.

## Shape

```
changelog         holds     entries, newest first
entry             is        a version heading and everything under it
version heading   names     a version, and a date once the release has one
[Unreleased]      names     no version, so no ceremony acts on it
entry             groups    its notes under sections
link ref          turns     a bracketed heading into a link
document          defines   one per versioned entry, or none at all
open entry        names     a version and no date
released entry    names     a version and a date
check             raises    a finding against a line, at a severity
register          fixes     every check and the severity it carries by default
reference tag     is        the newest version tag reachable from HEAD
release-trunk     cuts      one tag on the trunk, from the newest entry
release-branch    cuts      the version the entry names, on a stabilization branch
merge to trunk    dates     the open entry, and ends a release-branch release
ceremony          reads     version, prerelease and already-tagged, and cuts the tag
```

## The document

**Changelog**:
The document a release is written in: a title, an `[Unreleased]` entry, and the
versioned entries below it, newest first. It is where the version comes from, not
the record written after one was decided elsewhere.
_Avoid_: release notes (which is one entry's body), history file

**Entry**:
One level-2 heading and everything under it, up to the next level-2 heading. It
names a version, or it is `[Unreleased]`.
_Avoid_: release (an entry describes one), version (an entry names one), section

**Version heading**:
The level-2 heading line itself, `## [1.2.3] - 2006-01-02`: the version this
entry names, and the date its release carries once it has one.
_Avoid_: title, header

**Section**:
A level-3 heading under an entry, grouping its notes by the kind of change:
Added, Changed, Deprecated, Removed, Fixed, Security. A breaking change is marked
inline inside the section it belongs to rather than given a heading of its own.
_Avoid_: category, group, change type

**Link reference definition**:
The `[1.2.3]: https://…` line at the foot of the document that turns a bracketed
heading into a live link. It belongs to the document rather than to the entry
above it, and a document defines one per versioned entry or defines none at all.
_Avoid_: compare link (which is what one usually points at), footnote

**`[Unreleased]`**:
The entry naming no version, standing permanently at the top of the document.
Nothing releases it: a ceremony acts on the newest entry that names a version,
and this one never does.
_Avoid_: unreleased version, next release, pending entry

**Open entry**:
An entry naming a version and carrying no date: the release it names is being
written, and the date it will ship on is not known yet. It is the state between
`[Unreleased]`, which names no version at all, and a released entry, which
carries both.

```
## [Unreleased]             names no version                    permanent
## [1.2.3]                  names a version, carries no date    an open entry
## [1.2.3] - 2006-01-02     names a version and a date          a released entry
```

Only the newest entry is ever open. An entry below it shipped, and carries the
date it shipped on.
_Avoid_: staging entry, draft entry, unreleased version, release candidate

**Released entry**:
An entry naming a version and a date: the release shipped and the entry is
history. It does not change afterwards, which is what lets anything downstream
take it as a baseline.
_Avoid_: published entry, historical entry, closed entry

**Pre-release**:
A version carrying a pre-release part, as in `1.2.3-rc1`. It is a property of the
version, and so of every entry and tag naming it: an entry that named a
pre-release once goes on naming one forever.
_Avoid_: release candidate (one spelling of one), beta, unstable

## The reading

**Check**:
One thing a reading looks for, carrying a name that appears in the finding it
raises and in whatever reconfigures it. A check that catches a defect defaults to
error; a check that encodes a policy defaults to off, because a policy check's
absence is the majority case being correct.
_Avoid_: rule, validator, lint (which is the whole reading)

**Register**:
Every check the tool knows, each with its one-line description and the severity
it carries unless a caller says otherwise. It is the single source: a check added
to it needs no second declaration anywhere.
_Avoid_: list, catalogue, ruleset

**Severity**:
What a check's finding counts as: error, warning, or off. Off is not a quiet
finding, it is no finding at all.
_Avoid_: level, priority

**Finding**:
One departure from the format, addressed to the line that carries it and naming
the check that raised it, so a reader knows which one to reconfigure.
_Avoid_: error, warning (those are severities), violation, annotation (which is
what a finding becomes on a diff)

**Reference tag**:
The one tag everything reading the repository compares against: the newest
version tag reachable from HEAD, pre-releases admitted or not as the run was
told. Reachability is not a setting, because a tag on a branch this checkout is
not on is never the right baseline.
_Avoid_: previous tag, baseline (bare), latest release

## The release

**Ceremony**:
What turns an entry into a release: reading the document, finding the version it
names untagged, cutting the tag and publishing whatever the release consists of.
It happens downstream. Reading the changelog reports what it found and releases
nothing itself.
_Avoid_: release process, pipeline, release job

**Trunk**:
The branch a repository's history converges on, and the only branch a
release-trunk ceremony cuts a final tag from.
_Avoid_: master, mainline, default branch (which names a repository setting)

**Stabilization branch**:
A branch opened to carry one release while it is still accumulating, deleted once
it merges. It is where a release-branch ceremony cuts its candidates, and it
is not a support line: nothing is maintained on it after the merge.
_Avoid_: release branch (release-branch is the ceremony), pre-release branch,
RC branch

**Release-trunk**:
The ceremony with one branch: the newest entry names a version and a date, a push
to the trunk finds that version untagged, and the tag is cut. One entry, one tag,
one ceremony.
_Avoid_: continuous release, trunk-based release

**Release-branch**:
The ceremony with two: a release is opened as an open entry on a stabilization
branch, the version that entry names is cut as a tag whenever it is untagged, and
merging to the trunk dates the entry and cuts the final tag. A candidate is that
entry naming one, so writing 1.3.0-rc.2 in the heading is what cuts it.
_Avoid_: release train, GitFlow release, pre-release branch (which names the
branch, not the ceremony)

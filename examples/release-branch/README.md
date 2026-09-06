# release-branch

A release opens on a branch of its own. Its entry names a version and carries no
date, because the release date is not known while the branch is still
accumulating. The version that entry names is the tag: writing `1.3.0-rc.2` in
the heading is what cuts `v1.3.0-rc.2`. Merging to the trunk dates the entry and
cuts the final tag.

A complete policy: a changelog written to it, and the three workflow invocations
that hold a repository to it. Copy the workflows into `.github/workflows/`, copy
the changelog's shape, and adjust the vocabulary and the link references.
[`../release-trunk/`](../release-trunk/) is the other strategy, where the release
happens on the trunk and every entry carries its date from the moment it is
written.

## The version is written, never derived

Neither the action nor the workflow proposes a tag spelling. Both cut the version
the newest entry names, and they differ in one word:

```yaml
# workflows/release-branch.yaml            # workflows/main.yaml
if: >-                                     if: >-
  ...prerelease == 'true' &&                 ...prerelease == 'false' &&
  ...already-tagged == 'false'               ...already-tagged == 'false'
```

`prerelease` says whether the version carries an identifier, so the branch cuts
candidates and the trunk cuts releases, and dropping `-rc.2` from the heading is
how the branch says the release is ready. `already-tagged` says the version is
already cut, so **a push that does not change the version cuts nothing**: the
file advances the sequence, not the pushing. A second candidate is that heading
rewritten. Nothing reads the branch name and nothing counts runs.

**Pick stage names that sort in the order the stages run, and check them.** Under
[SemVer §11](https://semver.org/spec/v2.0.0.html#spec-item-11) identifiers compare
in ASCII order, so `alpha < beta < pre < rc` and all of them below the release —
but `candidate` sorts below `early`, and `snapshot` sorts *above* `rc`, which is
why Maven's `SNAPSHOT` needs a comparison rule of its own. It matters when a third
stage is added, which is the one moment nobody is reading a workflow that works.

## The open entry

| Heading | What it is |
|---|---|
| `## [Unreleased]` | names no version: unchanged, permanent |
| `## [1.3.0-rc.1]` | names a version, carries no date: an **open entry** |
| `## [1.3.0] - 2026-09-12` | names a version and a date: a released entry |

An open entry is illegal under Keep a Changelog, which is why `undated-entry`
defaults to `error`; switching it off is how one branch says its newest entry is
open rather than malformed. Two things do not move with it. **`undated-release`
stays at `error` everywhere**: it fires where the newest entry carries no date
*and a final tag already names its version*, which is a release that shipped and
nobody dated. A candidate tag naming the candidate this branch is accumulating is
not that, and does not fire it. And **`heading-form` keeps erroring on any other undated entry**, the
relaxation being scoped to the newest one.

An open entry carries its link reference definition in released form from the
moment it is opened, `[1.3.0-rc.1]: .../compare/v1.2.0...v1.3.0-rc.1`. That link
is broken until the tag is cut and nothing exempts it: `partial-link-refs` tests
that a definition exists and never resolves the URL.

## `prerelease-entry` is the trunk's, and only the trunk's

A candidate is a heading here, so the branch and pull-request invocations must not
raise it; the trunk must, because a release reaching it still carrying an
identifier is one nobody finished. The cost, stated rather than hidden: the check
judges the whole document, so switching it off to permit the newest heading
permits a stale candidate below it too.

This is where the two strategies part. Under
[`../release-trunk/`](../release-trunk/) a candidate can only be an entry on the
trunk, so the check is off for the pull request and on for the push. Here the axis
is the branch rather than the trigger.

## `reference-tags: final`, and why it is a requirement

Under `final` a pre-release is never the reference tag the repository-reading
checks compare against; under `all` it may be. The stabilization branch is where
raising it to `all` looks right and is the one place it must not be: that branch
cuts `v1.3.0-rc.N`, so the reference becomes a candidate tag cut on the branch
itself, whose changelog already carries the entry that named it. Rewriting that
heading for the next candidate then trips `release-entry-modified` — and under
this strategy rewriting it is the mechanism rather than an accident.

`final` is the default, so the other two invocations rely on it without saying so.

## What it costs in configuration

| | [`release-branch.yaml`](workflows/release-branch.yaml) | [`main.yaml`](workflows/main.yaml) | [`pull-request.yaml`](workflows/pull-request.yaml) |
|---|---|---|---|
| runs on | a push to `release/*` | a push to `main` | a pull request |
| `sections` | the six plus `Breaking` | the six plus `Breaking` | the six plus `Breaking` |
| `off` | `undated-entry` | *(unset)* | `undated-entry` |
| `error` | *(unset)* | `prerelease-entry` | *(unset)* |
| `reference-tags` | `final` | *(default)* | *(default)* |

## The two documents

[`CHANGELOG.md`](CHANGELOG.md) is the branch's own state. It passes under the
branch and pull-request invocations and raises `undated-entry` under the trunk
one, which is correct: it never reaches the trunk in that state, because the merge
dates the entry first.

[`CHANGELOG.broken.md`](CHANGELOG.broken.md) departs three times, and what each
costs depends on which invocation reads it:

| Departure | Check | Reported by |
|---|---|---|
| the open entry has no link reference definition | `partial-link-refs` | all three |
| `1.2.0`, below the newest entry, also carries no date | `heading-form` | all three |
| `## [1.1.0-pre.3]`, a candidate never rewritten into its release | `prerelease-entry` | the trunk |

The trunk reports a fourth, `undated-entry`, for the same reason it reports it on
`CHANGELOG.md`. `go test ./...` runs both documents under the inputs all three
workflows carry and holds the broken one to exactly those lists, so the example is
executed rather than described.

The five checks that read the repository are not among them: they compare a
document against the tags of the repository it lives in, and these live in this
one. `undated-release` is one of the five, so what holds it is a test asserting
the branch invocation leaves it at `error` rather than a document provoking it.

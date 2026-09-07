# release-branch

A release opens on a branch of its own and accumulates there until it is ready.

**The changelog names the release it is heading for, and never an attempt at
one.** The entry says `1.3.0` from the moment the branch opens and carries no
date, because the date is not known yet. Every revision of a pull request into
that branch cuts a numbered candidate — `v1.3.0-rc.1`, `v1.3.0-rc.2` — and
merging to the trunk dates the entry and cuts `v1.3.0`.

So a heading never carries `-rc.2`. The document states the destination; the
machine numbers the attempts.

A complete policy: a changelog written to it, and four workflow invocations that
hold a repository to it. Copy them into `.github/workflows/`, copy the
changelog's shape, and adjust the vocabulary and the link references.
[`../release-trunk/`](../release-trunk/) is the other strategy, where the release
happens on the trunk and every entry carries its date from the moment it is
written.

## Four events, and what each yields

| Workflow | Runs on | Yields |
|---|---|---|
| [`ci.yaml`](workflows/ci.yaml) | any pull request | a verdict — nothing is cut |
| [`release-candidate.yaml`](workflows/release-candidate.yaml) | a pull request into `release/*` | `v1.3.0-rc.4` |
| [`release.yaml`](workflows/release.yaml) | a push to `main` | `v1.3.0` |
| [`publish.yaml`](workflows/publish.yaml) | a tag | artifacts, and a GitHub release for a final tag |

Read the last column and you have the strategy. [`suite.yaml`](workflows/suite.yaml)
is the fifth file and yields nothing: it is the tests, called by everything that
must not cut past a broken tree.

**The changelog decides whether a run happens at all.** The two cutting
workflows carry `paths: [CHANGELOG.md]`, so a change that proposes no release
starts nothing. On a pull request that filter is a three-dot diff over the whole
request, so once a revision names the release, every later revision still matches
and the ordinal advances.

## Nothing is published from a push to the branch

A release branch has **no push trigger**. Its tip is never published in its own
right; a candidate comes from a pull request into it, which is what gives the
suite somewhere to stand:

```yaml
  tag:
    needs: [changelog, suite]
```

A push arrives with nothing in front of it, and a pull request does not. That is
the whole reason the trigger is what it is, and the reason the cut is a job
rather than a step: `needs:` is job-level, so a step has no boundary a gate can
attach to.

## The open entry

| Heading | What it is |
|---|---|
| `## [Unreleased]` | names no version: unchanged, permanent |
| `## [1.3.0]` | names a version, carries no date: an **open entry** |
| `## [1.3.0] - 2026-09-12` | names a version and a date: a released entry |

An open entry is illegal under Keep a Changelog, which is why `undated-entry`
defaults to `error`; switching it off is how an invocation says a release is in
flight rather than that the document is malformed. It fires only on the newest
*versioned* entry, so an ordinary pull request that touches no release never
meets it — which is why `ci.yaml` can switch it off for every pull request
without weakening anything.

Two things do not move with it. **`undated-release` stays at `error`
everywhere**: it fires where the newest entry carries no date *and a final tag
already names its version*, which is a release that shipped and nobody dated.
And **`heading-form` keeps erroring on any other undated entry**, the relaxation
being scoped to the newest one.

An open entry carries its link reference definition in released form from the
moment it is opened, `[1.3.0]: .../compare/v1.2.0...v1.3.0`. That link is broken
until the tag is cut and nothing exempts it: `partial-link-refs` tests that a
definition exists and never resolves the URL.

## `prerelease-entry` is on everywhere

A candidate is a tag this strategy composes, never a heading, so an identifier in
the document is a defect wherever it is read. There is no invocation that relaxes
it and no document state that wants it relaxed.

This is where the two strategies part, and it is the reverse of what a reader
might expect: the strategy with candidates is the one whose *document* never
names one.

## `reference-tags: final`, and why it is a requirement

Under `final` a pre-release is never the reference tag the repository-reading
checks compare against; under `all` it may be. The candidate invocation is where
raising it to `all` looks right and is the one place it must not be: that
invocation cuts `v1.3.0-rc.N`, so the reference would become a candidate cut from
the same branch, whose changelog already carries the entry that named it.
Rewriting that entry for the next candidate then trips
`release-entry-modified` — and rewriting it is the mechanism rather than an
accident.

`final` is the default, so the other three rely on it without saying so.

## What it costs in configuration

One input, on one of the four.

| | `ci` | `release-candidate` | `release` | `publish` |
|---|---|---|---|---|
| `sections` | the six plus `Breaking` | the six plus `Breaking` | the six plus `Breaking` | the six plus `Breaking` |
| `error` | `prerelease-entry` | `prerelease-entry` | `prerelease-entry` | `prerelease-entry` |
| `off` | `undated-entry` | `undated-entry` | *(unset)* | `undated-entry` |
| `reference-tags` | *(default)* | `final` | *(default)* | *(default)* |

`release.yaml` is the one that keeps `undated-entry`, because the merge is what
dates the entry and one reaching the trunk still open is a release nobody
finished. A test holds the four to exactly that difference.

## The two documents

[`CHANGELOG.md`](CHANGELOG.md) is the branch's own state. It passes under three
invocations and raises `undated-entry` under `release.yaml`, which is correct: it
never reaches the trunk in that state, because the merge dates the entry first.

[`CHANGELOG.broken.md`](CHANGELOG.broken.md) departs three times, and every
invocation reports all three:

| Departure | Check |
|---|---|
| the open entry has no link reference definition | `partial-link-refs` |
| `1.2.0`, below the newest entry, also carries no date | `heading-form` |
| `## [1.1.0-rc.3]`, a candidate written into the document | `prerelease-entry` |

`release.yaml` reports a fourth, `undated-entry`, for the same reason it reports
it on `CHANGELOG.md`. `go test ./...` runs both documents under the inputs all
four workflows carry and holds the broken one to exactly those lists, so the
example is executed rather than described.

The checks that read the repository are not among them: they compare a document
against the tags of the repository it lives in, and these live in this one.
`undated-release` is one of them, so what holds it is a test asserting the
candidate invocation leaves it at `error` rather than a document provoking it.

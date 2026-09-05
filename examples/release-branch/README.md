# release-branch

A release opens on a branch of its own. Its entry names a version and carries no
date, because the release date is not known while the branch is still
accumulating. Every run on that branch composes a pre-release tag,
`X.Y.Z-pre.N`, for whatever publishes downstream. Merging to the trunk dates the
entry and cuts the final tag.

A complete policy: a changelog written to it, and the three workflow invocations
that hold a repository to it. Everything here is generic. Copy the workflows
into `.github/workflows/`, copy the changelog's shape, and adjust the vocabulary
and the link references to the repository.

[`../release-trunk/`](../release-trunk/) is the other strategy, where the release
happens on the trunk and every entry carries its date from the moment it is
written.

## The open entry

Three states, and the middle one is what this strategy turns on.

| Heading | What it is |
|---|---|
| `## [Unreleased]` | names no version: unchanged, permanent |
| `## [1.3.0]` | names a version, carries no date: an **open entry** |
| `## [1.3.0] - 2026-09-12` | names a version and a date: a released entry |

An open entry is illegal under Keep a Changelog, which is why `undated-entry`
defaults to `error`. Switching it off is how one branch says its newest entry
is open rather than malformed, and that switch belongs to that branch alone.

Two things do not move with it.

- **`undated-release` stays at `error` everywhere**, the stabilization branch
  included. It fires where the newest entry carries no date **and a tag already
  names its version**, which is a release that shipped and nobody dated. No
  strategy wants that, so it is never named in `off:`.
- **`heading-form` keeps erroring on any other undated entry.** The relaxation is
  scoped to the newest entry, so a second undated entry below it is still a
  defect and is reported as one whatever `undated-entry` is set to.

`## [Unreleased]` stays permanent and empty on this strategy. Work in progress
goes under the open entry, which is what a version being open means.

## The link reference definition

An open entry carries its definition in released form from the moment the entry
is opened:

    [1.3.0]: https://git.example.invalid/repository/compare/v1.2.0...v1.3.0

`v1.3.0` does not exist yet, so that link is broken for as long as the branch
runs. Nothing exempts it and nothing is added when the branch closes:
`partial-link-refs` tests that a definition exists and never resolves the URL.
The definition is written once, in the form it will keep forever, and cutting
the final tag is what makes it resolve.

`[unreleased]` points at `v1.3.0...HEAD` for the same reason and is broken for
the same window.

## The pre-release tag

The action proposes no tag spelling. It reports `version` off the newest entry
and `already-tagged` off the repository's tags, and how a repository spells a tag
belongs to whatever cuts them. So the workflow composes the spelling, out of
what it already knows:

```yaml
- name: The pre-release tag for this run
  id: pre
  if: steps.changelog.outputs.already-tagged == 'false'
  env:
    VERSION: ${{ steps.changelog.outputs.version }}
  run: |
    suffix="${GITHUB_REF_NAME##*-}"
    tag="v$VERSION-$suffix.$GITHUB_RUN_NUMBER"
    echo "tag=$tag" >>"$GITHUB_OUTPUT"
```

`version` is `1.3.0`, the branch is `release/v1.3-pre`, the run is number 7, and
the tag is `v1.3.0-pre.7`. The next run composes `v1.3.0-pre.8`. Every one of
them sorts below `1.3.0`, so the final tag is ahead of all of them when it is
cut.

The suffix comes off the branch name rather than a literal, and that is what
leaves a second stage free: `release/v1.3-rc` beside `release/v1.3-pre` is
matched by the same `release/*` filter and composes `v1.3.0-rc.N` with no change
to the workflow at all.

`already-tagged` is `false` for the whole life of the branch, because the tags
cut there name `1.3.0-pre.N` and the entry names `1.3.0`. It turns `true` once
the final tag exists, which is the workflow's own signal that the branch is
spent.

Because a candidate is a tag here, it is never a heading, and that is why
`prerelease-entry` is raised to `error` on all three invocations. It is the
second policy separating the two strategies. Under
[`../release-trunk/`](../release-trunk/) a candidate can only be expressed as an
entry, so a release pull request may legitimately carry a pre-release heading
while it is under discussion, and the check stays off there. Under
release-branch no such moment exists, so a pre-release heading anywhere in the
document is a mistake and every invocation says so.

## `reference-tags: final`, and why it is a requirement

`reference-tags` decides which tags may be the reference tag the checks that read
the repository compare against. Under `final` a pre-release is never one; under
`all` it may be.

The stabilization branch is where raising it to `all` looks right, and it is the
one place it must not be. That branch cuts `v1.3.0-pre.N` on every run, so under
`all` the reference on the next run is a pre tag cut on the branch itself, whose
changelog already carries the open entry. Every commit that rewrites that entry
then trips `release-entry-modified`, and the branch fails continuously in the
confusing direction: an immutability check complaining about an entry that has
never been released.

`final` is the default, so the other two invocations rely on it without saying
so. It is written down on the branch invocation for the reason above and nowhere
else.

## What it costs in configuration

Four inputs across three files, and no two files carry the same set.

| | [`workflows/release-branch.yaml`](workflows/release-branch.yaml) | [`workflows/main.yaml`](workflows/main.yaml) | [`workflows/pull-request.yaml`](workflows/pull-request.yaml) |
|---|---|---|---|
| runs on | a push to `release/*` | a push to `main` | a pull request |
| `sections` | the six plus `Breaking` | the six plus `Breaking` | the six plus `Breaking` |
| `off` | `undated-entry` | *(unset)* | `undated-entry` |
| `error` | `prerelease-entry` | `prerelease-entry` | `prerelease-entry` |
| `reference-tags` | `final` | *(default)* | *(default)* |

`off` is quoted in both files that carry it, because YAML 1.1 reads a bare `off`
as `false` and the input name has to survive whichever parser reads the file.

The pull-request invocation carries the same relaxation as the branch one and
keeps it for a different reason: a pull request may target the stabilization
branch, where the newest entry is legitimately open, or the trunk, where it is
not, and nothing in the binary knows which. The two push invocations decide once
the commit has landed somewhere. A repository that would rather keep one file may
fold the two into a single workflow whose `on:` carries the push filter and
`pull_request` together: the only input they differ in is `reference-tags`, and
the value written there is the default the pull-request file already relies on.
What it gives up is the place where each relaxation's reason is written down.

## The two documents

[`CHANGELOG.md`](CHANGELOG.md) is the branch's own state: `[Unreleased]`
permanent and empty, an open entry under it, and released entries below that. It
passes under the branch and pull-request invocations. Under the trunk invocation
it raises `undated-entry`, which is correct: that file never reaches the trunk in
that state, because the merge dates the entry first.

[`CHANGELOG.broken.md`](CHANGELOG.broken.md) departs from the policy three times,
one per check:

| Departure | Check |
|---|---|
| the open entry has no link reference definition, while every entry below it does | `partial-link-refs` |
| `1.2.0`, below the newest entry, also carries no date | `heading-form` |
| `## [1.1.0-pre.3]`, a pre-release entry where this strategy keeps pre-releases in tags | `prerelease-entry` |

All three invocations report all three, because all three raise
`prerelease-entry`. The trunk invocation reports a fourth, `undated-entry` on the
open entry, for the same reason it reports it on `CHANGELOG.md`.

`go test ./...` runs both documents under the inputs all three workflows carry
and holds the broken one to exactly those lists. An example that rots is worse
than none, so the example is executed rather than described.

## What the tests here do not cover

The five checks that read the repository compare a document against the tags of
the repository it lives in, and these documents live in this one, whose tags
describe the action rather than the project the example is written for. They run
in a consuming repository and not here.

`undated-release` is one of those five, so the test holds the branch invocation
to leaving it at `error` rather than watching it fire. That is the assertion the
policy needs: what would break the strategy is naming it in `off:` beside
`undated-entry`, and a workflow either names it there or does not.

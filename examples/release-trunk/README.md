# release-trunk

The release happens on the trunk. The newest entry names a version and a date,
a push to the trunk finds that version untagged, and the tag is cut. One
branch, one ceremony.

A complete policy: a changelog written to it, and the two workflow invocations
that hold a repository to it. Everything here is generic. Copy the workflows
into `.github/workflows/`, copy the changelog's shape, and adjust the
vocabulary and the link references to the repository.

[`../release-branch/`](../release-branch/) is the other strategy, where a
release opens on a branch of its own and its entry carries no date until that
branch merges.

## The policy

- `Breaking` is a section heading beside the Keep a Changelog six. This is the
  one place the policy departs from the specification, which marks a breaking
  change inline as `**Breaking:**` inside the section it belongs to.
- `## [Unreleased]` is permanent. A release is written below it as a new
  version heading, and the Unreleased heading itself is never consumed.
- Every version has a link reference definition at the foot of the file, so
  every heading renders as a link rather than as literal text.
- Version headings are `## [X.Y.Z] - YYYY-MM-DD`, versions descending.
- No entry names a pre-release version, though the repository's tags may carry
  one.

## What it costs in configuration

Two inputs, and the second only on one of the two invocations.

| | [`workflows/main.yaml`](workflows/main.yaml) | [`workflows/pull-request.yaml`](workflows/pull-request.yaml) |
|---|---|---|
| `sections` | the six plus `Breaking` | the six plus `Breaking` |
| `error` | `prerelease-entry` | *(unset)* |

`sections` replaces the default vocabulary rather than adding to it, so the six
are spelled out beside `Breaking`.

`prerelease-entry` is off by default, because a pre-release heading is legal
under the format and this policy is the one that forbids it. Raising it to an
error on the main branch and leaving it off on pull requests is the whole of
how the two branches differ: a release pull request may legitimately carry such
a heading while it is under discussion, and nothing in the binary knows what a
pull request is. The two files are otherwise identical, and a test holds them
to that.

`undated-entry` and `undated-release` are both at their default, which is
`error`, and between them they are what holds the newest entry to naming a
date. The other strategy switches the first of them off on one branch;
this one never switches either off anywhere.

## The two documents

[`CHANGELOG.md`](CHANGELOG.md) is written to the policy and passes under both
invocations.

[`CHANGELOG.broken.md`](CHANGELOG.broken.md) departs from it four times, one
per check:

| Departure | Check |
|---|---|
| `### Notes`, a heading outside the vocabulary | `unknown-section` |
| `## [2.2.0-rc.1]`, a pre-release entry | `prerelease-entry` |
| `2.1.0` filed above `2.0.0` | `version-order` |
| `1.4.1` with no link reference definition | `partial-link-refs` |

`go test ./...` runs both documents under the inputs the two workflows carry
and holds the broken one to exactly that list, minus `prerelease-entry` on the
pull-request invocation. An example that rots is worse than none, so the
example is executed rather than described.

## What the tests here do not cover

The four tag-dependent checks compare a document against the tags of the
repository it lives in, and these documents live in this one, whose tags
describe the action rather than the project the example is written for. They
run in a consuming repository and not here.

One of them interacts with the last policy bullet. `version-behind-tag`
compares the newest entry against the newest version tag, and a pre-release tag
sorts above the release below it, so a repository whose tags carry pre-releases
while its changelog does not will find the newest entry reported as behind the
newest tag.

# Two worked policies

Two release strategies, each a complete worked example: a changelog written to
it, the workflow invocations that hold a repository to it, a deliberately broken
copy, and a README explaining both. `go test ./...` validates every document
under the inputs those workflows carry, so the examples are executed rather than
described.

Everything here is generic. No example names a repository, an organisation or a
product.

## Which one

| | [`release-trunk/`](release-trunk/) | [`release-branch/`](release-branch/) |
|---|---|---|
| Where a release is prepared | on the trunk | on `release/vX.Y-pre` |
| The newest entry | names a version and a date | names a version and no date while the branch is open |
| What a run publishes | the release, once the trunk finds the version untagged | `vX.Y.Z-pre.N`, on every run on the branch |
| Workflow invocations | two | three |
| `undated-entry` | at its default, `error` | switched off on the stabilization branch, and nowhere else |
| `undated-release` | at its default, `error` | at its default, `error` |

**release-trunk** is for a repository whose releases are decided in one commit:
the entry is written with its date, it merges, and the tag is cut. One branch,
one ceremony, and the changelog is never in an intermediate state.

**release-branch** is for a repository that stabilizes a release over days while
the trunk keeps moving: a version is opened, testable builds come off the branch
under their own pre-release tags, and the release date is written when the branch
merges, because that is the first moment anybody knows it.

The state in the middle is the one thing the format has no word for, and it is
what separates the two:

    ## [Unreleased]            names no version                    unchanged, permanent
    ## [1.3.0]                 names a version, carries no date     an open entry
    ## [1.3.0] - 2026-09-12    names a version and a date           a released entry

A repository on release-trunk that has never wanted a stabilization branch needs
nothing from the other tree. Going the other way costs one branch, one workflow
file, and one input.

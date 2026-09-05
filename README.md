# action-changelog

Reads a [Keep a Changelog](https://keepachangelog.com/en/2.0.0/) document and
reports where it departs from the format. Each finding is emitted as a workflow
annotation, so a failing check lands on the offending line of the diff.

```yaml
- uses: mikluko/action-changelog@v0
  with:
    changelog: CHANGELOG.md
```

A Docker container action, so it runs on Linux runners. It reaches the network
only to pull its own image.

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

| Input | Default | Description |
|---|---|---|
| `changelog` | `CHANGELOG.md` | Path to the changelog, relative to the workspace. |
| `sections` | *(empty)* | Comma-separated level-3 headings to accept. Empty accepts the Keep a Changelog six: Added, Changed, Deprecated, Removed, Fixed, Security. |
| `error` | *(empty)* | Comma-separated checks to raise as errors. |
| `warn` | *(empty)* | Comma-separated checks to raise as warnings. |
| `off` | *(empty)* | Comma-separated checks to switch off. |
| `fail-on` | `error` | What turns the step red: `error`, `warning`, or `never`. |
| `reference-tags` | `final` | Which tags may be the reference tag: `final`, or `all` to admit pre-releases. |

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
| `version` | `1.2.3` | The version the newest versioned entry names. |
| `notes` | | The body of the newest versioned entry, verbatim. |
| `already-tagged` | `false` | Whether a tag naming `version` already exists. |
| `prerelease` | `false` | Whether `version` carries a pre-release part, as in `1.2.3-rc1`. |
| `latest-tag` | `v1.2.2` | The reference tag, as the repository spells it. |

Every output is something the action read. None of them proposes a tag: how a
repository spells its tags belongs to whatever cuts them, so a workflow wanting
a ref reads `latest-tag` and a workflow cutting a new one writes the spelling it
has chosen.

`prerelease` is a fact about the newest entry, where the `prerelease-entry`
check is a judgement about the whole document. A workflow gating on what it is
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

A document naming no version is still validated: `valid` and `already-tagged`
answer, and `version` and `notes` are empty.

The values are written to `$GITHUB_OUTPUT` and printed on stdout, which is where
a local run reads them.

## Checks

Each check carries a name, which appears in the finding and in the workflow
annotation, and which is what `error`, `warn` and `off` take.

<!-- checks:start -->

| Check | Default | Description |
|---|---|---|
| `heading-form` | `error` | An entry heading states a version and a date, as in [1.2.3] - 2006-01-02. The newest entry may omit the date, which undated-entry and undated-release answer for. |
| `date-format` | `error` | An entry's date is written YYYY-MM-DD. |
| `version-order` | `error` | Entries run newest first, each version strictly below the one above it. |
| `empty-entry` | `error` | A released entry carries something under it. |
| `unknown-section` | `error` | A level-3 heading is one of the accepted section vocabulary. |
| `date-order` | `error` | Entry dates run newest first, matching the version order above them. |
| `date-future` | `error` | No entry is dated later than today. |
| `partial-link-refs` | `error` | Every versioned entry has a link reference definition, once any entry does. |
| `prerelease-entry` | `off` | Policy: no entry names a pre-release version. Off by default; pre-release headings are legal. |
| `undated-entry` | `error` | Policy: the newest entry states a date, where no tag yet names its version. Switched off, that entry is open: a release still accumulating on a branch of its own. |
| `undated-release` | `error` | The newest entry states a date, where a tag already names its version. The release shipped and nobody dated it. |
| `no-git-tags` | `error` | The repository's tag history can be read, which every check comparing against it needs. |
| `version-behind-tag` | `error` | The newest entry is not behind the reference tag. |
| `release-entry-modified` | `error` | A released entry is unchanged since the reference tag. |
| `prerelease-entry-modified` | `error` | A released pre-release entry is unchanged since the reference tag. |

<!-- checks:end -->

## Two worked policies

[`examples/`](examples/) carries two named release strategies, each as a
complete worked example: a changelog written to it, the workflow invocations
that enforce it, and a deliberately broken copy.

[`release-trunk/`](examples/release-trunk/) releases on the trunk. The newest
entry names a version and a date, a push to the trunk finds that version
untagged, and the tag is cut. One branch, one ceremony.

[`release-branch/`](examples/release-branch/) opens a release on a branch of its
own. The entry names a version and carries no date while the branch accumulates,
`undated-entry` is switched off there and nowhere else, every run on the branch
publishes under a pre-release tag the workflow composes from `version` and its
own run number, and merging to the trunk dates the entry and cuts the final tag.

`go test ./...` validates both trees under the inputs their workflows carry, so
the examples are executed rather than described.

## Local use

```
go run github.com/mikluko/action-changelog@latest
go run github.com/mikluko/action-changelog@latest -changelog docs/CHANGELOG.md
go run github.com/mikluko/action-changelog@latest -list-checks
```

## Releasing

This repository releases itself with its own action, which is the shortest
statement of what the action is for.

`action.yaml` names the image it runs, so that reference has to be immutable and
correct at the moment it is committed: a workflow pinning this action by SHA
reads the file as it stands at that commit, and nothing is rewritten onto `main`
afterwards. A release is therefore written in the pull request, in two places a
test holds together:

1. `CHANGELOG.md` gains `## [1.2.3] - YYYY-MM-DD` above the previous entry.
2. `action.yaml` runs `docker://ghcr.io/mikluko/action-changelog:v1.2.3`.

`go test ./...` fails while those disagree. Merging is the whole of it. The
release workflow runs the action against this repository's own changelog, and
where it reports a version no tag carries, publishes the image, cuts the tag,
creates the release from `notes`, and moves the major tag for a final version.
Nothing else cuts a tag here, and a tag pushed by hand is not a path.

This works because the changelog names the version before the tag does, which is
the inversion the tool exists to support.

## Contributing

Contributions are welcome. This is maintained in spare time and there is no
commitment to review a pull request promptly, so an issue first is usually the
cheaper way to find out whether a change is wanted.

AI-assisted contributions are welcome too, and are asked to be attributed: name
the tool in the pull request, or leave a `Co-authored-by:` trailer on the
commits. What matters is that a reviewer knows what they are reading.

## License

MIT. See [LICENSE](LICENSE).

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

This takes the other direction. The author writes `## [1.2.3] - 2026-09-04` and
everything downstream follows from the file — the version, the notes, whether it
is already tagged. A changelog that falls behind its tags then blocks the
pipeline rather than drifting quietly, which is the failure this exists to make
impossible.

It performs no release itself. It reports what it found — the version, its notes,
whether that version is already tagged — and the ceremony belongs downstream, to
whatever automation consumes that metadata.

## Inputs

| Input | Default | Description |
|---|---|---|
| `changelog` | `CHANGELOG.md` | Path to the changelog, relative to the workspace. |
| `validate` | `true` | Report where the changelog departs from the format. |
| `sections` | *(empty)* | Comma-separated level-3 headings to accept. Empty accepts the Keep a Changelog six: Added, Changed, Deprecated, Removed, Fixed, Security. |
| `error` | *(empty)* | Comma-separated checks to raise as errors. |
| `warn` | *(empty)* | Comma-separated checks to raise as warnings. |
| `off` | *(empty)* | Comma-separated checks to switch off. |
| `fail-on` | `error` | What turns the step red: `error`, `warning`, or `never`. |

`Breaking` is not in the default vocabulary. Keep a Changelog 2.0.0 marks a
breaking change inline as `**Breaking:**` inside the section it belongs to;
repositories that use it as a heading pass it in `sections`.

`error`, `warn` and `off` are applied in that order, so a check named in two of
them takes the later spelling. A name no check carries is refused rather than
ignored. `fail-on: never` reports every finding and exits 0.

## Checks

Each check carries a name, which appears in the finding and in the workflow
annotation, and which is what `error`, `warn` and `off` take.

<!-- checks:start -->

| Check | Default | Description |
|---|---|---|
| `heading-form` | `error` | An entry heading states a version and a date, as in [1.2.3] - 2006-01-02. |
| `date-format` | `error` | An entry's date is written YYYY-MM-DD. |
| `version-order` | `error` | Entries run newest first, each version strictly below the one above it. |
| `empty-entry` | `error` | A released entry carries something under it. |
| `unknown-section` | `error` | A level-3 heading is one of the accepted section vocabulary. |
| `date-order` | `error` | Entry dates run newest first, matching the version order above them. |
| `date-future` | `error` | No entry is dated later than today. |
| `partial-link-refs` | `error` | Every versioned entry has a link reference definition, once any entry does. |
| `prerelease-entry` | `off` | Policy: no entry names a pre-release version. Off by default; pre-release headings are legal. |
| `no-git-tags` | `error` | The repository's tag history can be read, which every check below needs. |
| `version-behind-tag` | `error` | The newest entry is not behind the newest tag. |
| `release-entry-modified` | `error` | A released entry is unchanged since the newest tag. |
| `prerelease-entry-modified` | `error` | A released pre-release entry is unchanged since the newest tag. |

<!-- checks:end -->

## Local use

```
go run github.com/mikluko/action-changelog@latest -validate -changelog CHANGELOG.md
go run github.com/mikluko/action-changelog@latest -list-checks
```

## Releasing

`action.yml` names the image it runs, so that reference has to be immutable and
correct at the moment it is committed: a workflow pinning this action by SHA
reads the file as it stands at that commit, and nothing is rewritten onto `main`
afterwards. The version is therefore written in the release pull request, in
three places a test holds together:

1. `CHANGELOG.md` gains `## [1.2.3] - YYYY-MM-DD` above the previous entry.
2. `VERSION` becomes `v1.2.3`.
3. `action.yml` runs `docker://ghcr.io/mikluko/action-changelog:v1.2.3`.

`go test ./...` fails while those disagree. Merge the pull request, then push
the tag `v1.2.3` at the merge commit. The release workflow re-checks all three
against the tag, builds and pushes the image, and moves the major tag `v1` onto
that commit once the image exists.

This works because the changelog names the version before the tag does, which is
the same inversion the tool exists to support.

## Contributing

Contributions are welcome. This is maintained in spare time and there is no
commitment to review a pull request promptly, so an issue first is usually the
cheaper way to find out whether a change is wanted.

AI-assisted contributions are welcome too, and are asked to be attributed: name
the tool in the pull request, or leave a `Co-authored-by:` trailer on the
commits. What matters is that a reviewer knows what they are reading.

## License

MIT. See [LICENSE](LICENSE).

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

## Inputs

| Input | Default | Description |
|---|---|---|
| `changelog` | `CHANGELOG.md` | Path to the changelog, relative to the workspace. |
| `validate` | `true` | Report where the changelog departs from the format. |
| `sections` | *(empty)* | Comma-separated level-3 headings to accept. Empty accepts the Keep a Changelog six: Added, Changed, Deprecated, Removed, Fixed, Security. |

`Breaking` is not in the default vocabulary. Keep a Changelog 2.0.0 marks a
breaking change inline as `**Breaking:**` inside the section it belongs to;
repositories that use it as a heading pass it in `sections`.

## Local use

```
go run github.com/mikluko/action-changelog@latest -validate -changelog CHANGELOG.md
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

# action-changelog

Reads a [Keep a Changelog](https://keepachangelog.com/) document and reports
where it departs from the format. Each finding is emitted as a workflow
annotation, so a failing check lands on the offending line of the diff.

```yaml
- uses: mikluko/action-changelog@v0
  with:
    changelog: CHANGELOG.md
```

## Inputs

| Input | Default | Description |
|---|---|---|
| `changelog` | `CHANGELOG.md` | Path to the changelog, relative to the workspace. |
| `validate` | `true` | Report where the changelog departs from the format. |
| `sections` | *(empty)* | Comma-separated level-3 headings to accept. Empty accepts the Keep a Changelog six: Added, Changed, Deprecated, Removed, Fixed, Security. |

`Breaking` is not in the default vocabulary. Keep a Changelog 2.0.0 marks a
breaking change inline as `**Breaking:**` inside the section it belongs to;
repositories that use it as a heading pass it in `sections`.

The action runs on Linux runners only, which is what a Docker container action
is. A repository whose changelog check has to run on `windows-latest` or
`macos-latest` cannot use it.

## Local use

```
go run github.com/mikluko/action-changelog@latest -validate -changelog CHANGELOG.md
```

## Releasing

The version is named in the release pull request, in two places that a test
holds together:

1. `CHANGELOG.md` gains `## [1.2.3] - YYYY-MM-DD` above the previous entry.
2. `action.yml` pins `image: docker://ghcr.io/mikluko/action-changelog:v1.2.3`.

`go test ./...` fails while those two disagree. Merge the pull request, then
push the tag `v1.2.3` at the merge commit. The release workflow re-checks the
pair against the tag, builds and pushes the image, moves the major tag `v1`
onto that commit, and cuts the release.

The image is published only after the tag is pushed, so `v1.2.3` is
unresolvable for as long as that workflow takes to run. Nothing consumes a tag
before it is announced, and the major tag — which is what `uses:` normally
names — moves last, after the image is in the registry.

`main` pins the previous release's image between releases, so
`uses: mikluko/action-changelog@main` runs the last release rather than the
branch. Name a tag.

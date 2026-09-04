# action-changelog

Reads a [Keep a Changelog](https://keepachangelog.com/) document and reports
where it departs from the format. Each finding is emitted as a workflow
annotation, so a failing check lands on the offending line of the diff.

```yaml
- uses: mikluko/action-changelog@v0
  with:
    changelog: CHANGELOG.md
```

It runs on `ubuntu-*`, `macos-*` and `windows-*` runners, and reaches the
network only to download its own binary.

## Inputs

| Input | Default | Description |
|---|---|---|
| `changelog` | `CHANGELOG.md` | Path to the changelog, relative to the workspace. |
| `validate` | `true` | Report where the changelog departs from the format. |
| `sections` | *(empty)* | Comma-separated level-3 headings to accept. Empty accepts the Keep a Changelog six: Added, Changed, Deprecated, Removed, Fixed, Security. |
| `version` | *(empty)* | Release of this action to run. Empty resolves the ref named in `uses:`. |

`Breaking` is not in the default vocabulary. Keep a Changelog 2.0.0 marks a
breaking change inline as `**Breaking:**` inside the section it belongs to;
repositories that use it as a heading pass it in `sections`.

## How the version resolves

The action is a composite that downloads a released binary, so it has to know
which release it is. `script/install.sh` takes the first of:

1. the `version` input, where a workflow names one;
2. `github.action_ref`, which is the tag when `uses:` names a tag;
3. the `VERSION` file committed beside `action.yml`, which is what a workflow
   pinning a SHA gets.

The download is checked against the release's `checksums.txt` before it runs.

## Local use

```
go run github.com/mikluko/action-changelog@latest -validate -changelog CHANGELOG.md
```

## Releasing

The version is named in the release pull request, in two places that a test
holds together:

1. `CHANGELOG.md` gains `## [1.2.3] - YYYY-MM-DD` above the previous entry.
2. `VERSION` becomes `v1.2.3`.

`go test ./...` fails while those two disagree. Merge the pull request, then
push the tag `v1.2.3` at the merge commit. The release workflow re-checks the
pair against the tag, builds the six platform archives with GoReleaser, and
moves the major tag `v1` onto that commit once those assets exist.

A commit on `main` between releases carries the previous release's `VERSION`,
so a workflow pinning `main` by SHA runs the last release rather than the
branch. Name a tag.

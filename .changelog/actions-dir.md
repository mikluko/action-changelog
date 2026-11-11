### Changed

- **Breaking:** every action lives under `actions/`. The reader moves from the
  repository root to `actions/validate`, and the writers from `append` and
  `release` to `actions/append` and `actions/release`:
  `mikluko/action-changelog/actions/validate@v2`, or
  `https://sarak.sh/mikluko/changelog/actions/validate@v2` on Forgejo.
  `@v1` keeps the old paths.
- **Breaking:** the Go module is `sarak.sh/mikluko/changelog/v2`, the suffix
  a major version above 1 requires, run as
  `go run sarak.sh/mikluko/changelog/v2/cmd/changelog@latest`.

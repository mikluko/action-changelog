### Changed

- **Breaking:** every action lives under `actions/`. The reader moves from the
  repository root to `actions/validate`, and the writers from `append` and
  `release` to `actions/append` and `actions/release`:
  `mikluko/action-changelog/actions/validate@v2`, or
  `https://sarak.sh/mikluko/changelog/actions/validate@v2` on Forgejo.
  `@v1` keeps the old paths.

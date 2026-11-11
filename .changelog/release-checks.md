### Added

- `fragments-release` check: releasing `[Unreleased]` with the fragments folded
  in raises nothing the changelog as it stands does not.
- `missing-fragment` check: a pull request adds a fragment, read against the
  merge base with the branch it merges into.
- `base` input of the reader, the branch a pull request merges into, defaulting
  to `github.base_ref`.

### Changed

- **Breaking:** `fragments` defaults to `.changelog` in the reader and the
  `release` action, so `missing-fragment` fails a pull request that adds no
  fragment unless switched off.

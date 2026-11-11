### Added

- **Fragments**: notes kept as `*.md` files in a directory of their own,
  `.changelog/` by convention, rather than under `[Unreleased]`, so pull
  requests adding notes do not conflict. The `fragments` input of the `release`
  action folds them into `[Unreleased]` before releasing it and deletes them;
  the `fragments` input of the reader checks them.
- `fragment-form` check, for a file in the fragments directory that is not a
  fragment.
- `-fragments` flag of the command and of its `release` subcommand.

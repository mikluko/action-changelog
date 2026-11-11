### Added

- `fold` subcommand, which appends the fragments to the open entry, or to
  `[Unreleased]` where none is open, and deletes them: how a release branch
  assembles its entry.
- `AppendOpen`, `FoldOpen`, `Close`, `ReleasePending` and `Entry.Notes` in
  `pkg/changelog`, and `Repo.MergeBase` in `pkg/git`.

### Changed

- `fragments-release` forecasts the open entry, closed today with the
  fragments folded in, where there is one.
- `missing-fragment` also passes a pull request whose changelog carries a note
  the merge base lacks, as one merging a release branch does.

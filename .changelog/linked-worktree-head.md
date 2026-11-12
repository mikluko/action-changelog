### Fixed

- In a linked git worktree, `latest-tag` and the merge base of `missing-fragment`
  followed the main checkout's HEAD instead of the worktree's own. HEAD is now
  read from the worktree's own git directory, and refs and objects from the
  shared one.

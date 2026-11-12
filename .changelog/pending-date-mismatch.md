### Added

- `pending-date-mismatch` check: the newest entry, where no tag names its version,
  states the day a tag cut on HEAD would carry. `date-mismatch` and
  `release-entry-modified` leave a release PR merged on a later day than its
  heading names red for good once it is tagged; this raises it while the entry
  can still be edited. For a lightweight tag that day is the committer date of
  HEAD in the offset HEAD records, read by the code that dates an existing tag.
  It is silent for an annotated tag, dated when it is cut, and where the
  repository has no version tag to say which kind it cuts.
- `Git.NextTagDay` and `Options.DuePrerelease` in `pkg/changelog`, and
  `Repo.NextDay` in `pkg/git`.

### Changed

- `due` is `false` while `pending-date-mismatch` fires, and `due-reason` is its
  message, so a workflow gating its release step on `due` does not cut a tag the
  changelog would then disagree with. Switch the check off to hold nothing back.

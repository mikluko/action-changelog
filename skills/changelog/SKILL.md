---
name: changelog
description: "Writing a changelog in the Keep a Changelog 2.0.0 shape as mikluko/changelog v2 checks it: what an entry is, the six change types, breaking and yanked markers, fragments in .changelog/ or notes in CHANGELOG.md, and cutting a version. Use when a change needs its changelog note, when adding a fragment or an entry, starting a changelog, cutting a version, reviewing a changelog, or drafting release notes from one."
---

# Changelog

A changelog is a curated record of the notable changes in each version, written for the people who
use the software. The shape is [Keep a Changelog 2.0.0](https://keepachangelog.com/en/2.0.0/), and
[mikluko/changelog](https://sarak.sh/mikluko/changelog) v2 is what checks it in CI: its `validate`
action reads the file, `append` and `release` write it. Which version a release gets, and when, is
the repository's release contract: its own workflows and README decide that, this skill does not.

## Where a note goes

Each pull request with a notable change carries its note, in one of two places:

| The repository                                              | A note goes                                  |
| ----------------------------------------------------------- | -------------------------------------------- |
| keeps fragments: a `.changelog/` directory, or the one its `validate` step's `fragments` input names | a new fragment file there; `CHANGELOG.md` is left to the release |
| has no fragments directory, or sets `fragments: ""`         | under `## [Unreleased]` in `CHANGELOG.md`    |

`missing-fragment` fails a pull request that carries neither: no new fragment, and no note the
changelog lacked at the merge base. When nothing in the change seems notable, ask the human what it
records rather than invent a note.

## The entry

An entry is a **statement** of what is different for the reader. It carries no reason, no
motivation and no account of the work: the commit and the PR hold those, beside the diff. Keep a
Changelog's brief for a drafting model asks for the reason in the text. This one does not.

```
- HTTP checks take a per-monitor retry budget.                                   statement
- Added a per-monitor retry budget, because the fixed count caused flapping.     motivation
- Reworked the retry loop so the budget can be configured.                       the work
```

The tells are _because_, _so that_, _in order to_, _to improve_, _which allows_, _as requested_.
Cut the clause. If nothing is left, there was no change to record.

- One change per bullet, one sentence, a period at the end. Plain words: many readers are not
  native speakers.
- Name what the reader touches: the flag, the field, the endpoint, the command. Internals stay out.
- A change is not a commit. One spanning several commits is one entry; a commit the reader cannot
  see (a test, a refactor, CI, a dependency bump with no visible effect) is none.
- A dependency update is described by its effect, under the type that effect has, or left out.

## The six types

| Type         | For                                                              |
| ------------ | ---------------------------------------------------------------- |
| `Added`      | a new feature                                                    |
| `Changed`    | behaviour that worked as intended and now works differently      |
| `Deprecated` | a feature a later version removes                                |
| `Removed`    | a feature now gone                                               |
| `Fixed`      | behaviour that was wrong and is now correct                      |
| `Security`   | a vulnerability; the entry leads with the CVE where there is one |

Was the old behaviour a bug? `Fixed`. Was it intended? `Changed`. Six is the whole set
`unknown-section` accepts unless the repository's `sections` input names another: `Performance` and
`Improved` are `Changed`, `New` is `Added`, `Internal` is not notable. A type with no entries is
omitted, never left as an empty heading.

The types also set the version a computed release takes: a `**Breaking:**` note makes it major
(minor below 1.0.0), any type other than `Fixed` and `Security` minor, and `Fixed` or `Security`
alone a patch. Pick the type by the change, never by the bump it buys.

```
- CVE-2024-12345: out-of-bounds read when parsing malformed input.
```

## Breaking and deprecated

A breaking change stays under its type, `Changed` or `Removed`, and opens with `**Breaking:**`. It
says what breaks and, where one clause covers it, what to do. A longer procedure is linked, never
inlined.

```
- **Breaking:** `parse()` returns a result object instead of raising.
- **Breaking:** the `color` option is renamed `theme`.
```

A removal is announced first: `Deprecated` in one version, naming the version that removes it, then
`Removed` in that later one. Deprecations, removals and breaking changes are the entries that are
never skipped.

## A fragment

A Markdown file in the fragments directory holding level-3 type headings and the bullets under
them, exactly as they would read under `[Unreleased]`, and nothing else: no title, no prose, no
note outside a type. `fragment-form` refuses anything more.

```markdown
### Fixed

- The parser no longer reads a fenced heading as an entry.
```

- One file per pull request, named for the change: `retry-budget.md`. The release folds fragments
  in name order, so the name is also where the note lands among the others.
- Only `*.md` files belong there; a name starting with a dot, such as `.gitkeep`, is skipped.
- A fragment carries notes and never a version. The release computes the version from what it
  folds.

## The file

`CHANGELOG.md` at the repository root, newest version first:

```markdown
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/2.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.1.0] - 2026-09-05

### Added

- ...

## [1.0.0] - 2026-08-01 [YANKED]

[Unreleased]: https://github.com/org/repo/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/org/repo/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/org/repo/releases/tag/v1.0.0
```

- The preamble names the format, pinned to the version followed, and the versioning scheme.
- A version heading is `## [X.Y.Z] - YYYY-MM-DD`: a Semantic Versioning 2.0.0 version, three
  components, no leading `v`, which belongs to the tag.
- The date is the day the tag is cut. Once the tag exists another day fails `date-mismatch`; before
  it, in a repository cutting lightweight tags, `pending-date-mismatch` holds the release back until
  the date is the day `HEAD` carries.
- The brackets make each heading a reference link, resolved once at the foot: each version compares
  against the one before, the oldest links to its tag, and `[Unreleased]` compares the latest tag
  to `HEAD`. Once one version has a foot link, every one needs it (`partial-link-refs`).
- A version pulled for a serious bug or a vulnerability stays listed, with `[YANKED]` after the date.
- A version may open with a sentence or two on its theme before the typed sections. Most do not.
- A released entry is frozen: changing it after its tag fails `release-entry-modified`. A forgotten
  note goes into the next release, not into the old one.

## Cutting a version

Whether this happens by hand, by a dispatched workflow or on merge is the repository's release
contract. The command is the one the `release` action runs:

```
go run sarak.sh/mikluko/changelog/v2/cmd/changelog@latest release
```

It folds every fragment into `[Unreleased]` and deletes them, renames `[Unreleased]` to the version
its notes call for, dated today in UTC, opens a fresh `[Unreleased]` above it, and, where the
`[Unreleased]` foot link compares against `HEAD`, moves it to the new tag and adds the version's.
`-version X.Y.Z` names the version instead of computing it, `-date YYYY-MM-DD` dates it, and
`-fragments ""` folds nothing. It refuses an `[Unreleased]` with no notes and a version not above
every one the file names. The tag is cut afterwards, from the file.

A release prepared on a branch of its own is an open entry: `## [X.Y.Z]` with no date, which
`release -open` writes and the pull request merging the branch to the trunk dates. The branch's
workflows switch `undated-entry` off. Notes arrive there as fragments, as edits to the open entry,
or both; `fold` moves the fragments into it:

```
go run sarak.sh/mikluko/changelog/v2/cmd/changelog@latest fold
```

Release notes, where the host wants some, are the released section copied out, never a second
record.

## Checking

Before pushing, run the checker the repository's CI runs, from a checkout that has its tags:

```
go run sarak.sh/mikluko/changelog/v2/cmd/changelog@latest -base main
```

`-base` names the branch the pull request merges into; without it `missing-fragment` is silent.
Findings go to stderr with the check's name, and `-list-checks` prints every check with its default
severity. The repository's workflow may raise, lower or switch off a check through `error`, `warn`
and `off`, and widen the types through `sections`; pass the same flags, and read the workflow before
arguing with a finding. A repository still on the v1 action (`@v1`) is checked by v1, which knows no
fragments: its notes go under `[Unreleased]`.

A repository whose notes live in `CHANGELOG.md` says so in its `validate` step:

```yaml
- uses: mikluko/changelog/actions/validate@v2
  with:
    fragments: ""
```

## Drafting

The draft is written from the diff and the PR, from the reader's side. A git log is not a draft and
a reworded commit message is not an entry: most commits are not notable, and a notable change usually
spans several. An entry exists because a reader would want it, never because a check wanted a line.
The human reads the result before anyone else does.

## Less common

- Old versions are never deleted. A file grown unwieldy archives versions no longer edited, with the
  main file and the archive linked both ways.
- Issue and PR links are prose with reference-style links at the foot, beside the version links. A
  tag or a commit travels between hosts; a `#1234` does not.
- Credits are optional and brief; a name or profile URL travels, a `@handle` belongs to one host.
- One product across a monorepo keeps a changelog per component and one central summary. Unrelated
  projects sharing a repository each keep their own.

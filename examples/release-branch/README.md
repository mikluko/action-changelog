# release-branch

A release opens on a branch of its own. Its entry names a version and carries no
date, because the release date is not known while the branch is still
accumulating. Every run on that branch cuts a pre-release tag, `X.Y.Z-pre.N`,
for whatever publishes downstream. Merging to the trunk dates the entry and cuts
the final tag.

A complete policy: a changelog written to it, and the four workflow invocations
that hold a repository to it. Everything here is generic. Copy the workflows
into `.github/workflows/`, copy the changelog's shape, and adjust the vocabulary
and the link references to the repository.

[`../release-trunk/`](../release-trunk/) is the other strategy, where the release
happens on the trunk and every entry carries its date from the moment it is
written.

## The open entry

Three states, and the middle one is what this strategy turns on.

| Heading | What it is |
|---|---|
| `## [Unreleased]` | names no version: unchanged, permanent |
| `## [1.3.0]` | names a version, carries no date: an **open entry** |
| `## [1.3.0] - 2026-09-12` | names a version and a date: a released entry |

An open entry is illegal under Keep a Changelog, which is why `undated-entry`
defaults to `error`. Switching it off is how one invocation says its newest entry
is open rather than malformed.

Two things do not move with it.

- **`undated-release` stays at `error` everywhere**, the stabilization branch
  included. It fires where the newest entry carries no date **and a tag already
  names its version**, which is a release that shipped and nobody dated. No
  strategy wants that, so it is never named in `off:`.
- **`heading-form` keeps erroring on any other undated entry.** The relaxation is
  scoped to the newest entry, so a second undated entry below it is still a
  defect and is reported as one whatever `undated-entry` is set to.

`## [Unreleased]` stays permanent and empty on this strategy. Work in progress
goes under the open entry, which is what a version being open means.

## The link reference definition

An open entry carries its definition in released form from the moment the entry
is opened:

    [1.3.0]: https://git.example.invalid/repository/compare/v1.2.0...v1.3.0

`v1.3.0` does not exist yet, so that link is broken for as long as the branch
runs. Nothing exempts it and nothing is added when the branch closes:
`partial-link-refs` tests that a definition exists and never resolves the URL.
The definition is written once, in the form it will keep forever, and cutting
the final tag is what makes it resolve.

`[unreleased]` points at `v1.3.0...HEAD` for the same reason and is broken for
the same window.

## The pre-release tag

The action proposes no tag spelling. It reports `version` off the newest entry
and `already-tagged` off the repository's tags, and how a repository spells a tag
belongs to whatever cuts them. So the workflow composes the spelling and cuts it,
out of what it already knows:

```sh
STAGE="${GITHUB_REF_NAME##*-}"

prev=$(git tag --list "v$VERSION-$STAGE.*" \
       | sed "s|^v$VERSION-$STAGE\.||" \
       | grep -E '^[0-9]+$' | sort -n | tail -1)
tag="v$VERSION-$STAGE.$(( ${prev:-0} + 1 ))"
```

`version` is `1.3.0`, the branch is `release/v1.3-pre`, the highest ordinal
already cut for that version and stage is 6, and the tag is `v1.3.0-pre.7`. Every
one of them sorts below `1.3.0`, so the final tag is ahead of all of them when it
is cut.

**The ordinal comes from the tags rather than from `github.run_number`.** That
counts a workflow *file's* runs, which is not the sequence being numbered:
renaming the file resets it to 1 and the next run dies on a tag an earlier name
already cut, and splitting the stages into two files gives each its own counter
that starts at 1 wherever the other one got to. Reading it off the tags scopes it
per version and per stage, so two stages number independently with nothing
coordinating them, and neither a rename nor a new file can move it. It costs
nothing, because `fetch-depth: 0` is already required for the tag-dependent
checks and so the tags are in the checkout.

**The `grep -E '^[0-9]+$'` is load-bearing.** A repository that once numbered its
pre-releases in two segments carries tags like `v1.3.0-pre.350.1148`. The glob
matches them and `sort -n` ranks that line above every single-segment ordinal, so
without the filter the next tag is `v1.3.0-pre.351`.

Four conditions stop the step before it cuts anything, and each says why in the
run summary:

| Condition | What it means |
|---|---|
| `valid` is not `true` | the document does not conform, so nothing is composed from it |
| `version` is empty | the document names no version to compose a tag out of |
| `prerelease` is `true` | the entry carries a pre-release identifier of its own, and this step appends the only one the strategy uses, so the two would compose into nonsense |
| `already-tagged` is `true` | the final tag exists, so the branch is spent |

The last is the branch's **end signal** rather than a precondition.
`already-tagged` is `false` for the whole life of the branch, because the tags cut
there name `1.3.0-pre.N` and the entry names `1.3.0`. It turns `true` once the
final tag exists, and from that moment there is nothing left on the branch to
stabilize.

## Where the list of stages lives

The stage comes off the branch name, and the branches the workflow accepts are
enumerated in its trigger:

```yaml
on:
  push:
    branches: ['release/*-pre', 'release/*-rc']
```

A branch filter's `*` does not span `/`, so those two patterns match
`release/v1.3-pre` and `release/v1.3-rc` and nothing else. **That is what makes
the derivation safe, and it is the reason not to simplify the trigger to
`release/*`.** `${GITHUB_REF_NAME##*-}` returns the whole ref when the branch
carries no hyphen:

    release/v1.3-pre  ->  pre
    release/v1.3-rc   ->  rc
    release/v1.3      ->  release/v1.3
    main              ->  main

`release/v1.3` composes a tag with a slash in it, which fails at push and is
loud. `main` composes `v1.3.0-main.7`, which is a legal tag and nonsense, and
that is the dangerous one. Under the trigger above neither branch reaches the
workflow: a branch naming no stage does not fail the run, it does not start one.

Adding a stage is one more pattern in that list. Nothing forces a line through
both stages either: a release that never needs a candidate stays on `-pre` until
it merges, and one that opens straight as a candidate never has a `-pre` branch.

**One file serves every stage because the derivation is the point of it.** A
repository whose stages differ in more than their suffix has already answered the
question the derivation exists to answer, and splits: one file per stage, each
with a single-pattern trigger and `STAGE: pre` or `STAGE: rc` in the job's `env:`
in place of the line that derives it. Once there are two files the derivation
computes a constant, which is a clause a reader could work out from the trigger
already in view, dressed up as configuration.

## Choosing stage names

`pre` sorting below `rc` is a property of the vocabulary rather than an accident.
Semantic Versioning 2.0.0 section 11 compares alphanumeric pre-release
identifiers in ASCII order, and the stage names that stuck sort in the order
their stages run:

    1.0.0-alpha.1
    1.0.0-beta.1
    1.0.0-candidate.1
    1.0.0-early.1
    1.0.0-pre.1
    1.0.0-rc.1
    1.0.0-snapshot.1
    1.0.0

The plausible alternatives are where it stops holding. `candidate` sorts below
both `early` and `pre`, so a line running `pre` and then `candidate` would go
backwards at the moment it stabilized. `snapshot` sorts **above** `rc`, so a
nightly would rank ahead of the candidate it precedes. Maven's `SNAPSHOT` is the
one widely used convention that does not fit, and Maven does not compare
qualifiers lexically at all: it orders them by a table of its own.

**So the rule is: pick stage names that sort in the order the stages run, and
check them.** Sorting the tags a line would cut is the whole check. It is worth
doing when a third stage is added, which is the one moment nobody is reading the
workflow that already works.

## The publish-time invocation

The stabilization branch cuts `v1.3.0-pre.N`, and whatever builds and publishes a
tag therefore runs on those tags as well as on the final one. If that workflow
reads the changelog for its release notes, it meets the open entry too, and there
the failure is worse than a red check: the **build** fails, on a document that is
correct.

So [`workflows/tag.yaml`](workflows/tag.yaml) carries the same relaxation the
branch does. `undated-release` stays on, as it does everywhere, and this is the
invocation where it earns its place most plainly: a tag already names the
version, so an undated newest entry there is a release that shipped and nobody
dated.

The step that files the release is guarded on the entry naming this tag:

```yaml
- name: File the release
  if: format('v{0}', steps.changelog.outputs.version) == github.ref_name
```

`1.3.0` never equals `1.3.0-pre.3`, so a pre-release tag files no release: it is
a build of a version the changelog has not released yet, and the notes belong to
the final tag. The same comparison refuses a tag pushed by hand at a commit the
newest entry does not describe, which is a hazard on any repository that
publishes from a changelog and has nothing to do with this strategy. **So a
publish step already carrying that guard needs one input, not a redesign.**

## `reference-tags: final`, and why it is a requirement

`reference-tags` decides which tags may be the reference tag the checks that read
the repository compare against. Under `final` a pre-release is never one; under
`all` it may be.

The stabilization branch is where raising it to `all` looks right, and it is the
one place it must not be. That branch cuts `v1.3.0-pre.N` on every run, so under
`all` the reference on the next run is a pre tag cut on the branch itself, whose
changelog already carries the open entry. Every commit that rewrites that entry
then trips `release-entry-modified`, and the branch fails continuously in the
confusing direction: an immutability check complaining about an entry that has
never been released.

`final` is the default, so the other three invocations rely on it without saying
so. It is written down on the branch invocation for the reason above and nowhere
else.

## What it costs in configuration

Four inputs across four files. `sections` and `error` are the same in all of
them; `off` and `reference-tags` are what separate them, and the trunk invocation
is the only one carrying neither.

| Workflow | Runs on | `sections` | `error` | `off` | `reference-tags` |
|---|---|---|---|---|---|
| [`release-branch.yaml`](workflows/release-branch.yaml) | a push to `release/*-pre` or `release/*-rc` | the six plus `Breaking` | `prerelease-entry` | `undated-entry` | `final` |
| [`tag.yaml`](workflows/tag.yaml) | a push of a version tag | the six plus `Breaking` | `prerelease-entry` | `undated-entry` | *(default)* |
| [`main.yaml`](workflows/main.yaml) | a push to `main` | the six plus `Breaking` | `prerelease-entry` | *(unset)* | *(default)* |
| [`pull-request.yaml`](workflows/pull-request.yaml) | a pull request | the six plus `Breaking` | `prerelease-entry` | `undated-entry` | *(default)* |

`off` is quoted in all three files that carry it, because YAML 1.1 reads a bare
`off` as `false` and the input name has to survive whichever parser reads the
file.

The three files that relax `undated-entry` do it for three different reasons. The
branch is where the entry is open. The publish invocation runs on the tags that
branch cuts, so it meets the same entry one step downstream. The pull-request
invocation relaxes it because a pull request may target the stabilization branch,
where the newest entry is legitimately open, or the trunk, where it is not, and
nothing in the binary knows which; the two push invocations decide once the
commit has landed somewhere.

A repository that would rather keep fewer files may fold the branch and
pull-request invocations into a single workflow whose `on:` carries the push
filter and `pull_request` together: the only input they differ in is
`reference-tags`, and the value written there is the default the pull-request
file already relies on. What it gives up is the place where each relaxation's
reason is written down.

## The two documents

[`CHANGELOG.md`](CHANGELOG.md) is the branch's own state: `[Unreleased]`
permanent and empty, an open entry under it, and released entries below that. It
passes under the branch, publish and pull-request invocations. Under the trunk
invocation it raises `undated-entry`, which is correct: that file never reaches
the trunk in that state, because the merge dates the entry first.

[`CHANGELOG.broken.md`](CHANGELOG.broken.md) departs from the policy three times,
one per check:

| Departure | Check |
|---|---|
| the open entry has no link reference definition, while every entry below it does | `partial-link-refs` |
| `1.2.0`, below the newest entry, also carries no date | `heading-form` |
| `## [1.1.0-pre.3]`, a pre-release entry where this strategy keeps pre-releases in tags | `prerelease-entry` |

All four invocations report all three, because all four raise `prerelease-entry`.
The trunk invocation reports a fourth, `undated-entry` on the open entry, for the
same reason it reports it on `CHANGELOG.md`.

`go test ./...` runs both documents under the inputs all four workflows carry and
holds the broken one to exactly those lists. An example that rots is worse than
none, so the example is executed rather than described.

## What the tests here do not cover

The five checks that read the repository compare a document against the tags of
the repository it lives in, and these documents live in this one, whose tags
describe the action rather than the project the example is written for. They run
in a consuming repository and not here.

`undated-release` is one of those five, so the test holds every invocation that
switches `undated-entry` off to leaving `undated-release` at `error`, rather than
watching it fire. That is the assertion the policy needs: what would break the
strategy is naming it in `off:` beside `undated-entry`, and a workflow either
names it there or does not.

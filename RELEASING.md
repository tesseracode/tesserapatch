# Releasing tpatch

This document describes how to cut a new tpatch release. It codifies the
process that shipped v0.8.0 through v0.11.0 and closes the tag/GitHub-Release
drift that motivated v0.11.1 Slice 3.

## Overview

A tpatch release consists of three artifacts, in order:

1. **A `CHANGELOG.md` entry** describing what shipped.
2. **A signed git tag** of the form `vX.Y.Z` pointing at the release commit.
3. **A GitHub Release** publishing the CHANGELOG entry as release notes.

Each artifact depends on the previous one. Skipping any of the three creates
drift; the whole point of this document is that the three MUST stay in lock-step.

There is no version constant to bump in code: `internal/buildinfo/buildinfo.go`
derives the runtime version from `debug.BuildInfo` (ldflags-injected or git-tag
resolved) and only falls back to `dev` for unlabeled builds.

## Prerequisites

- Clean working tree on `main`.
- All work targeted for the release has landed on `main` and passed review.
- `gh` CLI authenticated for the `tesseracode/tesserapatch` repo.
- Full gates green:
  ```bash
  gofmt -l .
  go vet ./...
  go build ./cmd/tpatch
  go test ./...
  ```

## Step 1 — Write the `CHANGELOG.md` entry

Add a new section at the top of `CHANGELOG.md` immediately below the top-level
`# Changelog` header.

**Format**:

```markdown
## vX.Y.Z — YYYY-MM-DD — Short scope name

One-paragraph summary of what shipped. Reference the driving whitepaper /
cluster / PRD block by name.

### Subsection A

Bulleted list of concrete changes with file references where useful.

### Subsection B

...
```

**Style rules that keep releases consistent**:

- The heading line MUST match `## vX.Y.Z — YYYY-MM-DD — <short scope>` so
  `gh release create --notes-file` can consume the extracted section verbatim.
- Group changes by feature/subsystem, not by commit.
- Cite PRD and ADR names inline (`ADR-025 D3`, `PRD-reconcile-verdict-evidence
  §4`) so future readers can trace decisions.
- Do NOT paste raw commit lists; the tag will carry that.
- Do NOT mention unreleased in-flight work; that lives under a
  `## vX.Y.Z (unreleased)` header that graduates to a dated header at release
  time.

Commit the CHANGELOG entry:

```bash
git -c commit.gpgsign=false add CHANGELOG.md
git -c commit.gpgsign=false commit --no-verify -m "release: vX.Y.Z — <short scope>" \
  -m "Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
git push origin main
```

## Step 2 — Tag the release commit

Tags are annotated (not lightweight) so `internal/buildinfo` can resolve them
and so GitHub surfaces the annotation as release metadata:

```bash
git tag -a vX.Y.Z -m "vX.Y.Z — <short scope>"
git push origin vX.Y.Z
```

The tag MUST point at the release commit from Step 1. Do NOT tag intermediate
commits.

## Step 3 — Publish the GitHub Release

Extract the CHANGELOG entry as release notes:

```bash
# Replace vX.Y.Z below with your actual version, escaping the dots:
awk '/^## v0\.11\.1 —/,/^## v0\.11\.0 —/' CHANGELOG.md | sed '$d' > /tmp/release-notes.md
```

The end-of-range regex must reference the PREVIOUS release header (not a
generic `/^## v/`) because the em-dash in `— YYYY-MM-DD` combined with
awk's greedy range matching can otherwise return zero lines. Verify the
notes file starts with the expected `## vX.Y.Z` header and ends immediately
before the previous release section.

The `sed '$d'` drops the trailing previous-release header line that `awk`
picked up as the range terminator.

Create the GitHub Release:

```bash
gh release create vX.Y.Z \
  --title "vX.Y.Z — <short scope>" \
  --notes-file /tmp/release-notes.md \
  --latest \
  --verify-tag
```

Flags:

- `--verify-tag` refuses if the tag doesn't exist on the remote; a hard-stop
  against tagging drift.
- `--latest` promotes this release to the repo's "Latest" surface. Omit for
  patch releases that ship out-of-order or for prereleases.
- `--notes-file` is preferred over `--generate-notes`; the CHANGELOG entry is
  the canonical release-notes source, and `--generate-notes` would pull
  commit-log summaries that don't match the CHANGELOG.

Verify the release exists:

```bash
gh release view vX.Y.Z
```

## Optional post-release checks

- `gh release list --limit 5` — confirm the new release is at the top with
  `Latest` if you used `--latest`.
- Spot-check `docs/handoff/CURRENT.md` — if the release closes a work-plan
  cluster, update the handoff and archive the previous CURRENT snapshot to
  `docs/handoff/HISTORY.md`.
- Spot-check `docs/ROADMAP.md` — if the release closes a milestone, flip the
  status marker (✅ / 🚧 / ⬜) so the roadmap stays truthful.

## Anti-drift guardrails

The v0.11.1 Slice 3 cleanup exists because tags v0.8.0 through v0.11.0 shipped
without GitHub Releases. To prevent recurrence:

1. **Never tag without publishing** — Steps 2 and 3 above are a single logical
   action. Do not push a tag unless you intend to publish the release in the
   same session. If you must delay, use a `-dev` or `-rc` suffix so the tag is
   clearly not a shippable release.
2. **CHANGELOG entry is the single source of truth** — release notes on GitHub
   MUST be extracted from the CHANGELOG. Do not hand-write release-note bodies
   that diverge from the CHANGELOG.
3. **Sanity check every release** — before considering the release done, run
   `gh release list --limit 3` and confirm the new release is present with the
   expected title.
4. **CI check candidate** — a pre-tag script or CI job that fails if the tag
   being pushed lacks a matching `## vX.Y.Z` CHANGELOG entry AND a GitHub
   Release within 24 hours would harden this further. Not implemented as of
   v0.11.1; queued as a doctor-command candidate under Slice 4.

## Version-derivation reminder

`internal/buildinfo/buildinfo.go` resolves the runtime `tpatch --version`
string via:

1. `-ldflags "-X ...=vX.Y.Z"` at build time (used by release binaries).
2. `runtime/debug.ReadBuildInfo()` for modules built with `go install
   github.com/tesseracode/tesserapatch/cmd/tpatch@vX.Y.Z`.
3. Literal `"dev"` for local `go build ./cmd/tpatch` runs.

There is NO version constant to edit in source. Do not add one.

## Historical release cadence (v0.8.0 through v0.11.0)

- Each release aligns with a whitepaper cluster or clearly-scoped feature
  bundle (see `docs/whitepapers/` and `docs/handoff/HISTORY.md`).
- Minor versions (v0.X.0) ship completed WP clusters or M-plan milestones.
- Patch versions (v0.X.Y where Y > 0) ship stabilization slices or follow-up
  fixes on top of the last minor.
- Every release since v0.7.0 has been co-authored with `Copilot` per the
  repository's commit-trailer convention.

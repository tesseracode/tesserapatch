# PRD — Record Auto Base — `feat-record-auto-base`

**Status**: Draft  
**Date**: 2026-04-30  
**Owner**: G55  
**Milestone**: Record boundary-capture guardrails, post-WP-001  
**Depends on**: None. This is the first guardrail and should ship before collision detection becomes strict.

## Related

- [WP-001 Feature Slice Gap](../whitepapers/WP-001-feature-slice-gap.md) — graduated source paper; especially §5.2 rows 1-2 and §9.
- [Recording Patches](../record.md) — current operator-facing record workflow.
- [Feature Layout](../feature-layout.md) — `artifacts/post-apply.patch` is canonical and is described as a full diff against `status.json:apply.base_commit`.
- [Personas](../market-research/personas.md) — Platform Pat's audit pressure and Maintainer Mira's reasoning-preservation JTBD make correct capture boundaries product-critical, not just CLI polish.
- [Competitive Landscape](../market-research/competitive-landscape.md) — SMART v0.7 remediation target, gbp-pq `--time-machine N` upstream-search prior art, and Strategy Canvas axis 4/§12 refresh-primitive weakness.
- Backlog: `feat-record-auto-base`, `feat-record-scoped-files`, `feat-noncontiguous-feature-commits`.
- Follow-on PRD: [PRD-record-collision-detection.md](./PRD-record-collision-detection.md).

## 0. Claims Audit

This PRD was drafted after re-reading the current post-Wave-3 / Slice-C-in-progress code and docs:

| Claim | Evidence |
|---|---|
| `recordCmd()` supports working-tree capture, `--from`, `--to`, `--commit-range`, `--files`, `--lenient`, recipe autogen flags, and empty-working-tree refusal. | `internal/cli/cobra.go:794-1014` |
| `CapturePatchScoped` captures tracked changes plus untracked files via intent-to-add, while excluding `.tpatch/` and skill assets. | `internal/gitutil/gitutil.go:216-303` |
| `CapturePatchFromCommitsScoped` captures committed ranges, supports pathspecs, excludes `.tpatch/`, and never includes untracked working-tree files. | `internal/gitutil/gitutil.go:313-328` |
| Focused tests pin `--from + --files`, `--commit-range + --files`, `--to`, mutex errors, and legacy unscoped behavior. | `internal/cli/record_range_scoped_test.go:90-270`; `internal/gitutil/capture_from_commits_scoped_test.go:34-196` |
| `FeatureStatus.Apply.BaseCommit` is stored as `base_commit`. | `internal/store/types.go:230` |
| Current `record` writes `status.Apply.BaseCommit = HEAD` after recording, even for committed-range captures. | `internal/cli/cobra.go:946-951` |
| `docs/feature-layout.md` says `post-apply.patch` is the current full diff against `status.json:apply.base_commit`. | `docs/feature-layout.md:34-38` |

No implementation has been changed by this PRD.

## Summary

Add `tpatch record <slug> --auto`, an explicit opt-in mode that infers the committed-range lower bound when a feature has already been committed before `record` runs. The command chooses a safe baseline from `.tpatch/upstream.lock` and Git merge-base information, prints the decision, records the canonical patch from that baseline to `HEAD` unless capped, and persists the resolved baseline in `status.json:apply.base_commit`.

The goal is prevention: guide operators away from broad `--from upstream/main` captures that accidentally include multiple local features and produce repeated canonical patches. WP-001 Cases A1 and A2 show that the repeated `post-apply.patch` collisions were boundary-capture failures, not a missing feature-slice data model.

Market grounding: Platform Pat's audit/compliance JTBD makes repeated feature patches a trust blocker, and the persona doc explicitly calls WP-001 collisions gating before Pat can be comfortable. Maintainer Mira's "future self plus coding agent" JTBD depends on `record.md` and `status.json` preserving the correct feature boundary. The competitive landscape's v0.7 SMART target requires zero new post-v0.7 collisions and at least 50% legacy collision-group remediation with auto-base; gbp-pq's `--time-machine N` validates deterministic upstream-history search as useful prior art, though v1 intentionally limits itself to explicit refs and merge-bases.

## 1. Problem Statement

Current `record` has the right primitives but leaves baseline selection to the operator:

```bash
tpatch record <slug> --from <base> [--to <ref>] [--files <paths>]
tpatch record <slug> --commit-range <a>..<b> [--files <paths>]
```

That is powerful, but the common recovery path is still cognitively sharp: after committing feature work, the user must know which commit is just before the feature. In multi-feature branches, picking `upstream/main` or another too-old base captures unrelated feature commits. WP-001 records two live case studies with this exact shape:

- `copilot-api`: 10 of 11 recorded features fell into repeated patch-hash groups; three Path-B features shared the same canonical patch hash.
- `t3code`: 11 recorded features shared one byte-identical canonical patch, size 137285 bytes.

The shipped `--files`, `--to`, and `--commit-range` support helps once the operator knows what to ask for. It does not solve the first question: "what base should I use?"

## 2. Goals / Non-Goals

### Goals

- Add explicit `--auto` baseline inference for `record`.
- Use `.tpatch/upstream.lock` when it is valid, and Git merge-base when the lock is absent, empty, stale, or not an ancestor of `HEAD`.
- Prefer remote default branches before hard-coded branch names, so single-remote forks using `origin/main` or a non-`main` default branch are handled without guesswork.
- Refuse ambiguous inference rather than silently choosing a broad range.
- Refuse multi-commit inferred ranges on merge-base fallback paths; those are too close to the A1/A2 broad-capture failure mode.
- Print the chosen lower bound, source, ahead count, and exact equivalent command.
- Persist the resolved lower bound as `status.apply.base_commit` for committed-range captures.
- Preserve all existing explicit range flags and pathspec scoping.
- Improve the empty-clean-tree diagnostic so it points at `--auto` as the preferred recovery path when applicable.

### Non-Goals

- Automatically decomposing a branch into multiple features. That belongs to `feat-feature-import` and possibly `feat-noncontiguous-feature-commits`.
- Inferring file ownership. `--files` remains the user-facing scoping primitive; automatic file ownership can be a later extension of `feat-record-scoped-files`.
- Changing the canonical patch model. `artifacts/post-apply.patch` remains the current feature diff and `patches/NNN-*.patch` remains audit history.
- Adding a provider dependency to `record`. Provider-assisted hints are optional and never required for the baseline decision.
- Implementing `tpatch land`. That is owned by `PRD-tpatch-land.md`.

## 3. User-Facing Contract

### 3.1 New CLI Surface

```bash
tpatch record <slug> --auto [--files <paths>] [--to <ref>]
```

Rules:

- `--auto` is mutually exclusive with `--from` and `--commit-range`.
- `--to <ref>` is allowed with `--auto`; it caps the upper bound exactly as it does with `--from`. If omitted, `toRef = HEAD`.
- `--files` is allowed with `--auto`; it narrows the inferred committed range exactly as it does with `--from`.
- `--auto` always uses committed-range capture. It does not include untracked working-tree files.
- If the working tree has uncommitted changes, `--auto` refuses unless `--include-working-tree` is introduced by a future PRD. The v1 contract is deliberately clean: inferred committed range only.

Example output:

```text
$ tpatch record model-picker --auto --files src/models/,docs/models.md
record --auto selected base 1a2b3c4d from upstream.lock (ancestor of HEAD)
  equivalent: tpatch record model-picker --from 1a2b3c4d --files src/models/,docs/models.md
  range: 1a2b3c4d..HEAD (3 commits ahead)
  Saved patch: patches/002-record.patch
  Patch validated: round-trips cleanly against working tree
Recorded patch for model-picker (12491 bytes, 5 files)
```

### 3.2 Baseline Inference Algorithm

Resolve `toRef` first (`HEAD` by default). Then resolve the baseline in this order:

1. Read `.tpatch/upstream.lock`.
2. If `commit:` is non-empty, resolves, and is an ancestor of `toRef`, choose that commit directly. Source label: `upstream.lock`.
3. If the lock commit resolves but is not an ancestor of `toRef`, compute `git merge-base <toRef> <lock-commit>`. Choose it only if non-empty and different from `toRef`, then apply the merge-base fallback safety gate below. Source label: `merge-base(toRef, upstream.lock)`.
4. If the lock has `remote:` and `branch:`, resolve `<remote>/<branch>`. If that ref is an ancestor of `toRef`, choose it directly. Otherwise compute `git merge-base <toRef> <remote>/<branch>` and apply the merge-base fallback safety gate.
5. If the lock is empty or unusable, discover a default upstream candidate in this order:
   1. the branch pointed to by `refs/remotes/upstream/HEAD`;
   2. the branch pointed to by `refs/remotes/origin/HEAD`;
   3. conventional refs `upstream/main`, `upstream/master`, `origin/main`, `origin/master`, only when exactly one resolves.
6. For the discovered candidate ref, choose it directly if it is an ancestor of `toRef`; otherwise compute `git merge-base <toRef> <candidate>` and apply the merge-base fallback safety gate.
7. If no candidate resolves, or if candidates at the same priority are ambiguous, refuse and print concrete commands using explicit `--from`.

**Merge-base fallback safety gate.** When the selected base came from a merge-base fallback because the candidate upstream ref or lock commit was not itself an ancestor of `toRef`, count commits in `<base>..<toRef>`. If that count is greater than 1, refuse by default. Do not introduce a threshold knob in v1. The diagnostic should print:

```text
record --auto inferred merge-base <base> against <source>, but the range contains <n> commits.
This is too broad to trust automatically; it may include multiple feature commits.
Inspect with:
  git log --oneline <base>..<toRef>
Then rerun with one of:
  tpatch record <slug> --from <precise-base> --to <feature-tip>
  tpatch record <slug> --from <base> --to <feature-tip> --files <feature-paths>
```

Strict refusal on `n > 1` is intentional: a stale or unrelated upstream ref plus merge-base fallback can silently recreate the whole-branch capture that produced the A1/A2 collisions. If evidence later shows legitimate multi-commit fallback ranges are common, add an explicit override PRD; do not weaken v1's default.

Refusal conditions:

- selected base is empty;
- selected base equals `toRef`;
- selected base is not an ancestor of `toRef` after merge-base fallback;
- selected base came from merge-base fallback and `<base>..<toRef>` contains more than one commit;
- multiple upstream candidates exist and `.tpatch/upstream.lock` does not disambiguate them;
- the inferred range contains zero textual diff after pathspec filtering;
- working tree is dirty and `--auto` was requested.

### 3.3 Persisted Metadata

For committed-range record modes (`--from`, `--commit-range`, and `--auto`), `status.apply.base_commit` must be the resolved lower bound used to produce `artifacts/post-apply.patch`, not `HEAD`.

For working-tree record mode, `status.apply.base_commit` remains the current `HEAD` at record time.

This aligns persisted state with `docs/feature-layout.md`: the canonical patch is the full diff against `status.json:apply.base_commit`.

No new status field is required for v1. Human-friendly provenance can live in `record.md`:

```markdown
**Capture mode**: auto committed range
**Base commit**: 1a2b3c4d (source: upstream.lock)
**Upper bound**: HEAD
**Pathspecs**: src/models/, docs/models.md
```

If implementation finds that future commands need source labels programmatically, add a follow-up ADR before adding fields like `apply.capture_mode` or `apply.base_source`.

### 3.4 Edge Cases

**No upstream lock.** Prefer the upstream remote's default branch (`refs/remotes/upstream/HEAD`), then the origin remote's default branch (`refs/remotes/origin/HEAD`). If those are absent, try conventional `main` / `master` refs only when exactly one plausible candidate resolves. Otherwise refuse.

**Empty upstream lock.** Treat the scaffolded lock from `tpatch init` (`commit: ""`) as absent, not as an error.

**Multiple upstreams.** Do not guess. Refuse with a message showing the candidate refs and an explicit remedy:

```bash
tpatch record <slug> --from <base>
tpatch record <slug> --auto --to <ref>   # only after upstream.lock is populated
```

If this becomes common, add a narrow `record --auto-ref <ref>` follow-up; do not overload v1 before evidence.

**Commits ahead of upstream.** This is the normal happy path when the selected upstream/base ref is an ancestor of `toRef`. Print how many commits are included. If the count is greater than one, include a warning that unrelated feature commits may be present and suggest `--files` or `--to` when the feature is not contiguous. If the selected base came from merge-base fallback, apply the strict safety gate instead: count greater than one refuses.

**Feature branch not based on current upstream.lock.** If the lock commit is not an ancestor of `HEAD`, use merge-base rather than diffing directly against the lock commit. Direct diff against a non-ancestor upstream head can include reverse upstream changes. If that merge-base fallback spans more than one commit, refuse and require an explicit `--from` / `--to` / `--files` choice.

**Dirty working tree.** Refuse in v1. The operator can either record the working tree without `--auto`, commit/stash first, or use explicit `--from` with a clean tree.

**Binary-only or mode-only changes.** Preserve current behavior: if the resulting patch is empty, report that no textual diff was produced.

**Agent-assisted ambiguity.** If a provider is configured, a future `--auto --explain` mode may ask it to rank candidate base commits using commit subjects and changed files. It must never auto-select from a provider-only answer. In v1, ambiguous means refuse.

## 4. Backwards Compatibility

`--auto` is opt-in for the first release. Plain `tpatch record <slug>` remains working-tree capture and still refuses a clean tree with no range. The refusal message should add:

```text
  If the feature has already been committed and this branch tracks upstream, try:
    tpatch record <slug> --auto
```

Existing explicit invocations keep their current semantics except for the `status.apply.base_commit` correction in committed-range modes. That correction is intentional because it makes persisted metadata match the canonical patch. Existing feature directories are not migrated.

## 5. Implementation Notes

- Add a small parser for `.tpatch/upstream.lock`; the existing file is YAML-like but simple enough for the repo's current zero-dep style.
- Reuse existing git helpers where possible. Add focused helpers only if needed: `IsAncestor`, `MergeBase`, and `ResolveRef` already exist in nearby code paths.
- Preserve pathspec behavior by calling `CapturePatchFromCommitsScoped` after inference.
- Update `generateRecordMD` so it can describe capture mode, resolved base, upper bound, and pathspecs.
- Update `docs/record.md` and shipped skills after implementation, because the user-facing recommendation changes from "find a base manually" to "try `--auto`, then refine with `--files` / `--to`."

## 6. Acceptance Criteria

- `record --auto` chooses `upstream.lock commit` when it is a valid ancestor of `HEAD`.
- `record --auto` falls back to `merge-base(HEAD, upstream.lock commit)` when the lock commit resolves but is not an ancestor and the inferred range contains exactly one commit.
- `record --auto` refuses the same merge-base fallback when `<base>..HEAD` contains more than one commit, with a diagnostic that includes `git log --oneline <base>..HEAD` and explicit rerun commands.
- `record --auto` prefers `refs/remotes/upstream/HEAD`, then `refs/remotes/origin/HEAD`, before conventional `upstream/main` / `origin/main` fallback when the lock is empty.
- `record --auto` refuses when multiple upstream candidates exist and the lock is empty.
- `record --auto --files <path>` produces the same bytes as `record --from <resolved-base> --files <path>`.
- `record --auto --to <ref>` caps the upper bound and prints the equivalent explicit command.
- `record --auto` refuses a dirty working tree with an actionable message.
- Committed-range record modes persist `status.apply.base_commit` as the resolved lower bound.
- Working-tree record still persists `status.apply.base_commit = HEAD`.
- Existing `--from`, `--to`, `--commit-range`, and working-tree tests remain green.
- `docs/record.md` documents `--auto`, `--to`, `--commit-range`, `--files`, and the clean-tree recovery path.
- Skill assets are updated and parity guard passes if implementation touches shipped skill text.

## 7. Open Questions

- Should a later release make `--auto` the default clean-tree recovery path, or should it remain explicit forever?
- Is `record --auto-ref <ref>` needed, or is explicit `--from` enough for unusual upstreams?
- Should `record --auto` support a future `--include-working-tree` mode that diffs `<base>..HEAD` plus uncommitted changes, or would that blur the clean committed-range contract too much?
- Should the `status.apply.base_commit` correction be documented as a bug fix in the changelog even though it changes persisted metadata for explicit range captures?

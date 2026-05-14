# PRD - Record Capture Modes - `feat-record-capture-modes`

**Status**: Draft
**Date**: 2026-05-13
**Owner**: Core
**Byline**: T55
**Milestone**: Capture and metadata foundation. Not yet roadmap-committed.
**Depends on**: [PRD-feature-file-claims](./PRD-feature-file-claims.md) for `--claimed-only`.

## Related

- [Recording Patches](../record.md)
- [Feature Layout](../feature-layout.md)
- [Patch capture prior art](../state-of-the-art/patch-capture-prior-art-and-hooks.md)
- [Patch identity metadata research](../state-of-the-art/tpatch-metadata-for-patch-identity.md)
- [Research roadmap](../state-of-the-art/research-roadmap.md)
- [WP-001 Feature Slice Gap](../whitepapers/WP-001-feature-slice-gap.md)
- [PRD-record-auto-base](./PRD-record-auto-base.md)
- [PRD-record-collision-detection](./PRD-record-collision-detection.md)
- [PRD-reconcile-lock-guard](./PRD-reconcile-lock-guard.md)
- [PRD-tpatch-land](./PRD-tpatch-land.md)
- [PRD-feature-file-claims](./PRD-feature-file-claims.md)

## Cluster Position

This is PRD 2 of 4 in the capture-and-metadata foundation cluster:

1. `PRD-feature-file-claims`
2. `PRD-record-capture-modes`
3. `PRD-feature-patch-identity-metadata`
4. `PRD-feature-patch-amend`

See [research-roadmap.md](../state-of-the-art/research-roadmap.md) for the
cluster tracker. This PRD builds on the WP-001 / M17 boundary-capture outcomes:
auto-base, collision detection, lock guard, and `land` trailers. Supervisor
acceptance is still required before implementation.

## 0. Claims Audit

This PRD is a proposal. It changes nothing.

| Claim | Evidence |
|---|---|
| `tpatch record` currently supports working-tree capture, `--auto`, `--from`, `--to`, `--commit-range`, `--files`, `--lenient`, recipe autogen flags, collision override, and dependent-amend guard. | `internal/cli/cobra.go` record command and flags. |
| Current default working-tree capture captures the full worktree diff against `HEAD`, scoped by `--files` when provided. | `internal/cli/cobra.go` calls `gitutil.CapturePatchScoped` when no range mode is active. |
| Committed-range captures use `CapturePatchFromCommitsScoped` and never include untracked working-tree files. | `internal/cli/cobra.go` and `docs/record.md`. |
| Current docs recommend `tpatch land` as the composed `record -> safe staging -> commit` operation. | `docs/record.md`; `docs/land.md`. |
| Git's staging area is a natural capture boundary, but tpatch has no `record --staged` contract today. | `patch-capture-prior-art-and-hooks.md` section 3.3. |

No code, schema, command behavior, or asset text is changed by this PRD.

## Summary

Make `tpatch record` capture boundaries explicit. Keep the current default
behavior, but add named modes for the Git index and claim-aware scope:

```bash
tpatch record <slug>                         # current default, full working tree
tpatch record <slug> --all                   # explicit alias for current default
tpatch record <slug> --staged                # capture only the Git index
tpatch record <slug> --unstaged              # capture only unstaged working-tree edits
tpatch record <slug> --claimed-only          # restrict any compatible mode to feature claims
tpatch record <slug> --files <pathspecs>     # existing explicit path filter
```

Existing committed-range modes remain:

```bash
tpatch record <slug> --auto [--to <ref>] [--files <paths>] [--claimed-only]
tpatch record <slug> --from <base> [--to <ref>] [--files <paths>] [--claimed-only]
tpatch record <slug> --commit-range <a>..<b> [--files <paths>] [--claimed-only]
```

The goal is not more ways to be clever. The goal is for humans and agents to say
which boundary they intend, and for tpatch to record that boundary in a way
future patch identity metadata can audit. `--all` is intentionally included even
though it is byte-equivalent to today's default: it lets agents and scripts state
the intended capture boundary explicitly and gives future metadata a stable
`working-tree-all` provenance signal.

## 1. Problem Statement

tpatch has strong capture primitives, but their intent is implicit:

- default `record` means "everything in the working tree against `HEAD`";
- `--from` means "a committed range";
- `--auto` means "infer the committed-range base";
- `--files` means "narrow by these pathspecs";
- collision detection catches one broad-capture failure shape after capture.

What tpatch lacks is explicit vocabulary for common operator and agent intent:

- "I already curated the hunk set with `git add -p`; record exactly the index."
- "Ignore staged work; record only these remaining unstaged edits."
- "Record every dirty change; I know this is the feature."
- "Record only the paths this feature claimed earlier."

Without explicit modes, future metadata has to infer capture semantics from flag
combinations and prose. That makes patch identity, amendment, and replay audits
harder than they need to be.

## 2. Goals / Non-goals

### Goals

1. Add explicit `record` capture modes for all worktree changes, staged-only
   changes, and unstaged-only changes.
2. Add `--claimed-only` as a path filter driven by feature claims.
3. Preserve the current default `tpatch record <slug>` behavior.
4. Keep existing `--auto`, `--from`, `--to`, `--commit-range`, and `--files`
   semantics.
5. Emit structured capture-mode provenance for future patch identity metadata.
6. Fail closed when mixed staged/unstaged state would make a staged-only or
   unstaged-only patch ambiguous.
7. Prefer Git-native workflows: users can use `git add -p` for hunk selection,
   then `tpatch record --staged`.

### Non-goals

1. No interactive hunk picker in tpatch v1.
2. No automatic feature decomposition.
3. No provider-assisted capture selection.
4. No hook installation.
5. No change to `artifacts/post-apply.patch` as canonical replay input.
6. No change to collision detection policy.
7. No raw agent-context capture.

## 3. User-facing Contract

### 3.1 Worktree-all mode

Current default remains:

```bash
tpatch record <slug> [--files <pathspecs>]
tpatch record <slug> --all [--files <pathspecs>]
```

`--all` is an explicit alias for the current default: capture the full textual
diff from `HEAD` to the working tree, including staged changes, unstaged
changes, and untracked files that the current capture helper can represent.

Rules:

- `--all` is mutually exclusive with `--staged`, `--unstaged`, `--auto`,
  `--from`, and `--commit-range`.
- `--files` narrows the capture exactly as it does today.
- `--claimed-only` may further narrow the capture to active feature claims.

### 3.2 Untracked-file policy by mode

| Mode | Untracked files |
|---|---|
| default / `--all` | Included using the existing working-tree capture behavior. The implementation must preserve index state when surfacing untracked files. |
| `--staged` | Included only if the file is represented in the index. Plain untracked files are excluded. |
| `--unstaged` | Included when they are not represented in the index and pass filters. |
| `--auto`, `--from`, `--commit-range` | Excluded; committed-range modes read commits, not working-tree untracked files. |

### 3.3 Staged mode

```bash
tpatch record <slug> --staged [--files <pathspecs>] [--claimed-only]
```

Captures the Git index as the feature boundary. This is the recommended path
when the user wants hunk-level curation:

```bash
git add -p src/server.go
tpatch record auth-timeout --staged
```

Rules:

- Generate a patch from `HEAD` to the index.
- Include staged additions, modifications, deletions, and renames to the extent
  the existing patch machinery supports them.
- Include new files only when they are represented in the index.
- Refuse if unstaged edits touch any path included in the staged patch, because
  reverse-apply validation against the live working tree would be ambiguous.
- Warn, but do not refuse, when unrelated unstaged edits exist on other paths.
- Refuse when the staged patch is empty.
- `--staged` is mutually exclusive with `--all`, `--unstaged`, `--auto`,
  `--from`, and `--commit-range`.

Validation default: build the staged patch from `HEAD` to the index and validate
it with a cached apply check against a temporary index seeded from `HEAD`. If a
temporary-index implementation proves impractical, direct `git apply --cached
--check` on the staged patch is the fallback. Do not silently downgrade to a
live-working-tree validation that cannot prove staged-only correctness.

Example refusal:

```text
record --staged refuses: staged and unstaged edits both touch src/server.go.
Commit, stash, or split the unstaged edits, then rerun.
```

### 3.4 Unstaged mode

```bash
tpatch record <slug> --unstaged [--files <pathspecs>] [--claimed-only]
```

Captures only unstaged working-tree edits, excluding staged paths.

Rules:

- If a path has both staged and unstaged changes, refuse.
- If staged changes exist on unrelated paths, ignore them and print a one-line
  note that they were not captured.
- Include untracked files only when they are not represented in the index and
  pass the same path filters.
- Refuse when the unstaged patch is empty.
- `--unstaged` is mutually exclusive with `--all`, `--staged`, `--auto`,
  `--from`, and `--commit-range`.

### 3.5 Claimed-only filter

```bash
tpatch record <slug> --claimed-only
tpatch record <slug> --auto --claimed-only
tpatch record <slug> --staged --claimed-only
```

`--claimed-only` intersects the selected capture mode with the feature's active
claims from `claims.json`.

Rules:

- Refuse if the feature has no active claims.
- If `--files` and `--claimed-only` are both provided, capture the intersection
  of explicit pathspecs and claims.
- If the intersection is empty, refuse with a diagnostic listing the claims and
  the explicit paths.
- Strict claims are deferred by `PRD-feature-file-claims`; all active v1 claims
  are advisory and eligible.
- `--claimed-only` is a filter, not a capture mode. It can combine with
  worktree, staged, unstaged, and committed-range modes.

### 3.6 Existing committed-range modes

Existing modes keep their current behavior:

```bash
tpatch record <slug> --auto [--to <ref>] [--files <paths>]
tpatch record <slug> --from <base> [--to <ref>] [--files <paths>]
tpatch record <slug> --commit-range <a>..<b> [--files <paths>]
```

`--claimed-only` can be layered on top, but `--staged`, `--unstaged`, and `--all`
cannot combine with committed-range modes.

### 3.7 Mode precedence and mutexes

| Flag | Mutually exclusive with | Notes |
|---|---|---|
| `--all` | `--staged`, `--unstaged`, `--auto`, `--from`, `--commit-range` | Explicit alias for default worktree capture. |
| `--staged` | `--all`, `--unstaged`, `--auto`, `--from`, `--commit-range` | Index-only. |
| `--unstaged` | `--all`, `--staged`, `--auto`, `--from`, `--commit-range` | Working-tree-only excluding staged paths. |
| `--auto` | `--all`, `--staged`, `--unstaged`, `--from`, `--commit-range` | Existing auto committed-range mode; may combine with `--to`. |
| `--from` | `--all`, `--staged`, `--unstaged`, `--auto`, `--commit-range` | Existing committed-range mode; may combine with `--to`. |
| `--to` | `--all`, `--staged`, `--unstaged`, `--commit-range` | Requires `--from` or `--auto`. |
| `--commit-range` | `--all`, `--staged`, `--unstaged`, `--auto`, `--from`, `--to` | Existing explicit committed-range mode. |
| `--files` | none | Path filter. |
| `--claimed-only` | none | Claim filter. |

## 4. Capture-mode Provenance

Every successful record should expose a normalized capture mode string for
future `PRD-feature-patch-identity-metadata.md`:

| Invocation shape | Capture mode |
|---|---|
| `record <slug>` | `working-tree-all` |
| `record <slug> --all` | `working-tree-all` |
| `record <slug> --staged` | `staged-index` |
| `record <slug> --unstaged` | `unstaged-worktree` |
| `record <slug> --auto` | `auto-committed-range` |
| `record <slug> --from` | `committed-range` |
| `record <slug> --commit-range` | `explicit-committed-range` |

Additional provenance:

- `pathspecs`: normalized explicit `--files`, if any;
- `claim_ids`: active claim IDs used by `--claimed-only`, if any;
- `base_commit`: lower bound for canonical patch;
- `upper_commit`: `HEAD`, resolved `--to`, or `working-tree`;
- `dirty_state`: summary for staged/unstaged modes, not raw diff content.

This PRD can write these fields to `record.md` first. Persisted machine metadata
belongs to `PRD-feature-patch-identity-metadata.md`.

## 5. Backwards Compatibility

Existing invocations behave the same:

- `tpatch record <slug>` remains full working-tree capture.
- `--files` keeps current pathspec behavior.
- `--auto`, `--from`, `--to`, and `--commit-range` keep current behavior.
- collision detection still runs after capture and before writes.
- `--force-amend` keeps its existing dependent-orphan meaning; this PRD does not
  rename or repurpose it.

No existing feature directory is migrated by this PRD.

## 6. Implementation Notes

- Prefer small gitutil helpers for index-only and unstaged-only patch capture.
- Keep `.tpatch/` and installed skill exclusions consistent with current capture.
- Reuse existing empty-patch refusal diagnostics where possible.
- The staged/unstaged overlap check should operate on path sets before writing
  any feature artifact.
- `record.md` should print capture mode, filters, and claim IDs when used.
- Validate staged-only patches against a temporary index seeded from `HEAD`
  where possible; direct `git apply --cached --check` is the fallback. Do not
  weaken staged validation silently.
- Do not add a provider fallback or auto-selection prompt.

## 7. Acceptance Criteria

- `record --all` produces the same patch bytes as default `record` in the same
  repository state and records explicit `working-tree-all` provenance.
- `record --staged` captures staged changes and ignores unrelated unstaged
  paths with a note.
- `record --staged` refuses when captured staged paths also have unstaged edits.
- `record --staged` includes new files only when they are represented in the
  index.
- `record --unstaged` captures unstaged changes and ignores unrelated staged
  paths with a note.
- `record --unstaged` refuses when staged and unstaged edits overlap on a path.
- `record --unstaged` includes plain untracked files that pass filters.
- `record --claimed-only` refuses when no claims exist.
- `record --claimed-only` captures only claimed paths in default worktree mode.
- `record --auto --claimed-only` intersects auto committed-range capture with
  active claims.
- `--files` and `--claimed-only` combine as an intersection and refuse when the
  intersection is empty.
- All mode mutexes return clear errors before patch capture.
- Successful records include capture-mode provenance in `record.md`.
- Existing record tests, auto-base tests, collision tests, and dependent-amend
  tests stay green.
- Docs and skill assets update only if the user-facing recommendation changes.

## 8. Open Questions

- Should future hook guards make reserved strict claims refuse by default?

## 9. Disputes

None logged.

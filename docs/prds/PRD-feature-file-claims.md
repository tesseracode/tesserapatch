# PRD - Feature File Claims - `feat-feature-file-claims`

**Status**: Draft
**Date**: 2026-05-13
**Owner**: Core
**Byline**: T55
**Milestone**: Capture and metadata foundation. Not yet roadmap-committed.
**Depends on**: None for v1. Future free-text reasons or context-retention
fields require `ADR-capture-context-privacy-boundary`.

## Related

- [Patch capture prior art](../state-of-the-art/patch-capture-prior-art-and-hooks.md)
- [Patch capture research brief](../state-of-the-art/patch-capture-context-research-brief.md)
- [Patch identity metadata research](../state-of-the-art/tpatch-metadata-for-patch-identity.md)
- [Research roadmap](../state-of-the-art/research-roadmap.md)
- [WP-001 Feature Slice Gap](../whitepapers/WP-001-feature-slice-gap.md)
- [Recording Patches](../record.md)
- [Feature Layout](../feature-layout.md)
- [PRD-record-auto-base](./PRD-record-auto-base.md)
- [PRD-record-collision-detection](./PRD-record-collision-detection.md)
- [PRD-reconcile-lock-guard](./PRD-reconcile-lock-guard.md)
- [PRD-tpatch-land](./PRD-tpatch-land.md)

## Cluster Position

This is PRD 1 of 4 in the capture-and-metadata foundation cluster:

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
| `tpatch record` already supports path scoping with `--files`, but the scope is invocation-local and not persisted as feature ownership. | `internal/cli/cobra.go` record help lists `--files`; `docs/record.md` describes capture choices but no persistent claims. |
| Feature directories have no `claims.json` or equivalent first-party ownership artifact today. | `docs/feature-layout.md` lists the canonical feature layout; no claim manifest appears. |
| Current cross-feature collision detection is byte-identical canonical-patch detection, not ownership tracking. | `docs/record.md` "Cross-feature collision detection (v0.8.0)"; `PRD-record-collision-detection.md`. |
| Quilt's strongest capture lesson is explicit file ownership before refresh. | `patch-capture-prior-art-and-hooks.md` section 3.1. |
| Current metadata is weak for moving patch identity and structural anchors. | `tpatch-metadata-for-patch-identity.md` executive findings. |

No code, schema, command behavior, or asset text is changed by this PRD.

## Summary

Add a first-party feature claim manifest that lets a user or agent declare
"this feature owns these paths" before `record`, `land`, reconcile, hooks, or
future capture automation try to infer ownership from diffs after the fact.

The v1 user-facing surface is advisory-only:

```bash
tpatch feature claim add <slug> <pathspec...>
tpatch feature claim list <slug> [--json]
tpatch feature claim remove <slug> <claim-id-or-pathspec...>
tpatch feature claim clear <slug>
```

Claims are not patches. They are scope metadata. They should start as
advisory by default, because existing repositories may already contain broad
features, shared files, generated files, or legitimate overlap. Strict claims,
claim reasons, and any free-text context retention are deferred until the
privacy and enforcement ADRs exist.

## 1. Problem Statement

tpatch can capture a patch, validate it, detect exact cross-feature collisions,
and project it into Git history with trailers. It still has a weak "during work"
scope story:

- `record --files` is powerful, but only for the current invocation.
- `record --auto` can infer the baseline, but not file ownership.
- collision detection catches exact duplicate canonical patches after capture,
  not partial over-capture before it happens.
- `land` can safely stage the feature-touched files once the patch exists, but
  it cannot know which future edits were intended for the feature before record.
- agent workflows often know intent early, but tpatch currently stores that
  context mostly as prose.

Quilt solves the analogous patch-stack problem with `quilt add <file>` before
editing. tpatch should not copy Quilt's backup-file implementation, but it
should adopt the stronger concept: explicit scope is better than reverse
engineering scope from one large diff.

## 2. Goals / Non-goals

### Goals

1. Add a persistent claim manifest per feature.
2. Let users and agents add, list, remove, and clear path claims.
3. Support advisory path claims in v1.
4. Make `record` and `land` able to read claims for warnings and future
   `--claimed-only` capture.
5. Reserve schema space for symbol and structural-anchor claims without
   requiring language parsers in v1.
6. Keep claims deterministic, local, and reviewable in the feature directory.
7. Treat claims as scope signals, not authorization to overwrite path safety or
   dependency checks.

### Non-goals

1. No automatic file ownership inference in v1.
2. No IDE, agent, or Git hook installation in this PRD.
3. No hunk-level interactive UI.
4. No symbol parser implementation in v1.
5. No global claim lock that prevents two features from touching the same file.
6. No replacement for `--files`, `--auto`, collision detection, or dependency
   validation.
7. No strict enforcement or persisted free-text reasons in v1.

## 3. User-facing Contract

### 3.1 Add claims

```bash
tpatch feature claim add model-picker src/models/ docs/models.md
tpatch feature claim add model-picker "src/**/*.go"
```

Rules:

- `<slug>` must identify an existing feature.
- Each `<pathspec>` is normalized with the same repository path-safety rules as
  other tpatch path inputs.
- Paths under `.tpatch/`, installed skill surfaces, and paths outside the repo
  are rejected.
- Duplicate normalized claims are idempotent.
- All v1 claims are advisory.
- `--mode`, `strict`, and `--reason` are deferred. This avoids persisting
  free-text or decorative enforcement metadata before the privacy and amendment
  ADRs define those boundaries.

Example output:

```text
feature claim: model-picker
  added claim 8f31c0a19b2d  path src/models/      advisory
  added claim 47a0de331851  path docs/models.md   advisory
```

### 3.2 List claims

```bash
tpatch feature claim list model-picker
tpatch feature claim list model-picker --json
```

Human output:

```text
Claims for model-picker:
  8f31c0a19b2d  advisory  path  src/models/
  47a0de331851  advisory  path  docs/models.md
```

JSON output should be stable-sorted by `claim_id`, then kind, then value.

### 3.3 Remove and clear claims

```bash
tpatch feature claim remove model-picker 8f31c0a19b2d
tpatch feature claim remove model-picker docs/models.md
tpatch feature claim clear model-picker
```

Removal updates the current manifest. Historical patch-generation metadata, once
implemented, should retain the claim IDs that were active when the generation
was recorded.

### 3.4 Advisory v1; strict deferred

| Mode | Meaning | v1 behavior |
|---|---|---|
| `advisory` | This feature is expected to touch the claimed path. | Used for warnings, `list`, and future provenance. Does not refuse by itself. |
| `strict` | Captures for this feature should stay inside claims unless explicitly overridden. | Deferred until at least one enforcing path ships. Not written by v1. |

Strict mode does not mean exclusive ownership across the whole repo. Two
features may legitimately claim the same file; overlap should be visible, not
silently forbidden. The value is reserved so future `record --claimed-only` or
hook-guard PRDs can introduce enforcement without redefining claim identity.

## 4. Persisted Manifest

Recommended path:

```text
.tpatch/features/<slug>/claims.json
```

Schema:

```json
{
  "version": 1,
  "feature": "model-picker",
  "claims": [
    {
      "claim_id": "8f31c0a19b2d",
      "kind": "path",
      "value": "src/models/",
      "mode": "advisory",
      "source": "manual"
    }
  ]
}
```

Field rules:

| Field | Rule |
|---|---|
| `version` | Manifest schema version. Required. |
| `feature` | Feature slug. Must match directory slug. |
| `claim_id` | Deterministic ID derived from `feature`, `kind`, normalized `value`, and `mode`. |
| `kind` | v1 supports `path`; reserves `glob`, `symbol`, and `anchor` for future PRDs. |
| `value` | Normalized repo-relative path or pathspec. No absolute paths. |
| `mode` | v1 writes only `advisory`; `strict` is reserved for a future enforcement PRD. |
| `source` | `manual`, `agent`, `imported`, or `generated`. v1 writes `manual`. |

The manifest should be deterministic:

- stable sort claims by `claim_id`;
- no wall-clock timestamps in v1;
- no raw prompts, transcripts, or tool inputs;
- no source snippets beyond the normalized path/pattern itself.

## 5. Command Interactions

### 5.1 `record`

This PRD does not change `record` by itself. It creates the metadata that
`PRD-record-capture-modes.md` can consume.

Expected future behavior:

- `record <slug>` can warn when changed files are outside advisory claims.
- `record <slug> --claimed-only` can restrict capture to active claims.
- `record <slug> --files <paths>` can warn when explicit files and claims do
  not intersect.

### 5.2 `land`

`land` should eventually use claims as an additional staging guard:

- if claims exist, compare staged/touched files to claims;
- warn on unclaimed files by default;
- refuse only when a future strict-claims or hook-guard policy requires it.

This PRD does not reopen the `land` trailer ADR.

### 5.3 `status`

Optional v1 addition:

```bash
tpatch status --claims
```

This is not required for the first implementation. `feature claim list` is
enough for operator visibility.

### 5.4 Hooks and agent sessions

Hooks and agent sessions are out of scope, but claims are designed to be their
input:

- IDE save hooks can say "this edit touches a claimed path".
- Git hooks can warn when staged files are unclaimed.
- agent event logs can attach `claim_id` to file edits.

## 6. Backwards Compatibility

Existing feature directories have no `claims.json`; that is equivalent to "no
claims declared." All existing commands must continue to behave as they do now
unless the user explicitly invokes claim commands or a claim-aware flag.

Adding claims does not migrate old patches, rewrite `status.json`, or re-record
features.

## 7. Implementation Notes

- Reuse existing safe repo path validation helpers.
- Keep claim parsing and matching in a small package so `record`, `land`, and
  future hooks can share it.
- Claim matching should operate on normalized repo-relative paths.
- Directory claims should match all descendants.
- Glob claims can remain schema-reserved until a dedicated glob/pathspec PRD
  chooses exact semantics.
- Do not persist claim reasons in v1. A future privacy ADR must decide whether
  reasons are tracked text, local-only notes, or omitted.
- Do not install hooks or alter skill assets in this PRD unless the command
  surface is exposed to users.

## 8. Acceptance Criteria

- `tpatch feature claim add <slug> <pathspec...>` writes
  `.tpatch/features/<slug>/claims.json`.
- Adding the same normalized claim twice is idempotent.
- `feature claim list` prints stable human output and `--json` emits stable JSON.
- `feature claim remove` accepts either claim IDs or normalized path values.
- `feature claim clear` removes all active claims for the feature.
- Invalid paths outside the repo are rejected.
- `.tpatch/` and installed skill surfaces cannot be claimed.
- Existing `record`, `reconcile`, `apply`, and `land` behavior is unchanged when
  no claim-aware flag is used.
- Claims are stable-sorted and contain no timestamps.
- Tests cover add/list/remove/clear, duplicate add, invalid path rejection,
  advisory-only persistence, and empty-manifest behavior.
- Docs explain that claims are scope metadata, not proof that the patch is
  correct or exclusive.

## 9. Open Questions

- Should v1 accept Git pathspec syntax or only literal paths/directories?
- Should claim overlap across features warn at claim-add time?
- Should the final command namespace be `tpatch feature claim`, plural
  `tpatch feature claims`, or top-level `tpatch claim`?
- What is the first enforcing path that should activate reserved `strict` mode:
  `record --claimed-only`, `land`, Git hooks, or all of them?
- Should reasons be tracked text, gitignored local notes, or omitted after the
  privacy ADR lands?
- Should symbol/anchor claims require an ADR before becoming active schema?

## 10. Disputes

None logged.

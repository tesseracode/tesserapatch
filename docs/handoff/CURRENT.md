# Current Handoff

## Active Task

- **Task ID**: `v0.9.0-alpha-1-file-claims` (kickoff)
- **Milestone**: v0.9.0 Wave alpha — capture & metadata foundation (slice 1 of 2)
- **Description**: Implement `PRD-feature-file-claims` v1: `tpatch feature claim <add|list|remove|clear>` subcommands writing a deterministic, advisory-only `.tpatch/features/<slug>/claims.json` manifest. Pure CLI surface add — no ADR required per codified process rule (no defaults flipped, no lifecycle automation changes).
- **Status**: Not started — awaiting implementer dispatch.
- **Assigned**: 2026-05-13.

## Session Summary

Opening v0.9.0 Wave alpha. v0.8.1 shipped 2026-05-13 (Wave D detector tails: `--check-applied-only` + `--auto-drop-merged` flags + ADR-022/023 deferral records). Followed by a small `provider-model-routing-audit` slice (`18fd668`) that refreshed `internal/provider/` comments to match empirical localhost:4141 findings and added `docs/MODEL-ROUTING.md` + `tests/scripts/model-routing-matrix.sh` + `tests/integration/provider_model_matrix_test.go` — zero behavior change, no external review needed. Now picking up the drafted foundation PRDs.

Wave alpha = two slices, both pure CLI surface adds:
1. **alpha-1** (THIS): `PRD-feature-file-claims` v1 — advisory-only claim manifest.
2. **alpha-2** (next): `PRD-record-capture-modes` v1 — `--all`/`--staged`/`--unstaged`/`--claimed-only` flags on `record`.

Both ship under the **v0.9.0** tag once the cluster completes.

## Settled Design Choices (user input this session)

- **Command namespace**: `tpatch feature claim` (singular; nested under existing `feature` subcommand). Matches PRD examples.
- **Pathspec syntax**: literal paths + directory-recursive only in v1. Full Git pathspec syntax (`:(glob)`, `:(top)`, `:!exclude`) deferred to a future ADR/PRD.
- **Overlap policy**: silent in v1. `feature claim add` does NOT warn at add-time when another feature already claims the same normalized path. Overlap is expected and normal; claims are scope metadata, not ownership locks.
- **Sequencing vs ADRs**: ship as pure CLI surface; defer `ADR-024-capture-context-privacy-boundary` until file-claims v2 needs it (when `--reason` / context fields land). Matches codified rule.

## v1 Contract (binding for implementer)

### Commands (under existing `feature` cobra subcommand)

```
tpatch feature claim add <slug> <path...>
tpatch feature claim list <slug> [--json]
tpatch feature claim remove <slug> <claim-id-or-path...>
tpatch feature claim clear <slug>
```

### Manifest

- Path: `.tpatch/features/<slug>/claims.json`
- Schema: `{ version: 1, feature: <slug>, claims: [ { claim_id, kind, value, mode, source } ] }`
- `kind` accepted in v1: only `"path"`. (`"glob"`, `"symbol"`, `"anchor"` are RESERVED schema values — do NOT accept them as input; reject with a clear error in v1.)
- `mode` written in v1: only `"advisory"`. (`"strict"` is RESERVED — do NOT accept it as input; reject with "strict mode is deferred; see PRD-feature-file-claims §3.4".)
- `source` written in v1: always `"manual"`. (`"agent"`, `"imported"`, `"generated"` are RESERVED — not user-settable in v1.)
- `claim_id`: deterministic hex digest derived from `feature || "\x00" || kind || "\x00" || normalized_value || "\x00" || mode`. Recommend SHA-256, truncated to 12 hex chars (matches PRD example `8f31c0a19b2d`).
- Claims are **stable-sorted** by `claim_id` in both the file and `list` output.
- **No wall-clock timestamps in v1.** No raw prompts/transcripts/source-snippets.

### Path normalization & rejection rules

- Reuse existing safe-repo-path helpers (e.g. `EnsureSafeRepoPath` / equivalent from `internal/safety/` or wherever `record`/`land` validate today).
- Reject: absolute paths, paths escaping the repo via `..`, paths under `.tpatch/`, paths matching installed skill surface roots, paths whose normalized form is empty.
- Directories: store the normalized path WITH a trailing `/` if and only if the input was a directory (or had a trailing `/`). Directory claims match all descendants when later consumed.
- Duplicate add (same normalized `value` + `kind` + `mode` for the same feature): IDEMPOTENT — no-op on the manifest, but print "already claimed" to stdout.

### `list` output

- Human (default): table-ish, e.g. `<claim_id>  advisory  path  <value>` one per line, prefaced by `Claims for <slug>:`. Empty case prints `Claims for <slug>: (none)`.
- `--json`: emits the full manifest JSON object, stable-sorted by `claim_id`. Pretty-printed (two-space indent) is fine for v1.

### `remove`

- Accepts EITHER a claim_id (any unambiguous prefix ≥ 7 chars matches; ambiguous prefix errors) OR a normalized path value.
- Removing a non-existent claim is a no-op with a "no such claim" note (exit 0). (Optional: gate with `--strict` if you want exit-2 on miss — but v1 acceptance does NOT require it; default no-op is fine.)
- Multiple arguments in one invocation: process each; print one line per claim removed.

### `clear`

- Removes all claims; writes a manifest with empty `claims: []` (still version 1, still keyed to the feature). Does NOT delete the file.

### Cross-cutting

- Slug must reference an existing feature (use the same lookup `record`/`land` use). Unknown slug → exit 1 with "no such feature".
- Manifest write must be ATOMIC (write to `claims.json.tmp` + rename) to avoid corruption on interrupted operations.
- Run `gofmt -l .`, `go vet ./...`, `go build ./cmd/tpatch`, `go test ./... -race -count=1` after impl.
- Update `CHANGELOG.md`: open `## v0.9.0 (in development)` section with a `### Feature claims` subsection listing the new commands.
- No changes to skill assets in this slice (claims aren't user-facing through skills yet). Parity guard must stay green.

## Acceptance Criteria (from PRD §8, restated for the implementer)

- `feature claim add <slug> <path...>` writes `.tpatch/features/<slug>/claims.json`.
- Adding the same normalized claim twice is idempotent.
- `feature claim list` prints stable human output and `--json` emits stable JSON.
- `feature claim remove` accepts either claim IDs or normalized path values.
- `feature claim clear` removes all active claims for the feature.
- Invalid paths outside the repo are rejected.
- `.tpatch/` and installed skill surfaces cannot be claimed.
- Existing `record`, `reconcile`, `apply`, and `land` behavior is unchanged when no claim-aware flag is used. (Verify no test regressions.)
- Claims are stable-sorted and contain no timestamps.
- Tests cover: add, list (human + JSON), remove (by id and by path), clear, duplicate add (idempotent), invalid path rejection (.tpatch, outside repo, absolute, skill surfaces), unknown slug rejection, empty manifest behavior, strict-mode-input rejection, reserved-kind rejection, atomic-write behavior.

## Frozen Regions (do NOT touch in this slice)

- `internal/cli/record_auto*.go` (Wave A1).
- `internal/cli/record_collision*.go` (Wave B).
- `internal/workflow/reconcile.go` lines ~196-236 (phase 1.5) and ~560-700 (lock guard).
- `internal/workflow/patch_id_detector*.go`.
- `internal/workflow/reconcile_check_applied.go` (Wave D rev-2 contract).
- `internal/cli/reconcile_check_applied.go` / `internal/cli/reconcile_auto_drop.go` (Wave D).
- `Config.PatchIDDetectorEnabled` default = `false` (ADR-022).
- `Manifest.Kind == "hotfix"` auto-drop default = deferred (ADR-023).
- ADR-019 trailer schema, ADR-020 inline-minimal skill-doc policy, ADR-021 carve-out scope.
- `internal/provider/{errors,responses,router}.go` (audit slice `18fd668` just refreshed these).
- The `## Side Research` section in this CURRENT.md (preserve byte-identical across handoff resets).

## Files Expected to Change (implementer guidance)

- `internal/cli/feature_claim*.go` (new) — cobra subcommand handlers.
- `internal/cli/cobra.go` — wire the new `feature claim` subtree onto the existing `feature` command.
- `internal/store/claims*.go` (new) — manifest type + read/write/sort + claim_id derivation. Place it in `internal/store/` since other manifests (FEATURES.md, status.json, config.yaml) already live there.
- `internal/store/claims_test.go` (new) — unit tests for the store layer.
- `internal/cli/feature_claim_test.go` (new) — end-to-end CLI tests using the existing test harness pattern from `internal/cli/reconcile_check_applied_test.go` or similar.
- `CHANGELOG.md` — open `## v0.9.0 (in development)` with `### Feature claims` subsection.
- `docs/handoff/CURRENT.md` — implementer updates Status / Files Changed / Test Results blocks on completion.

## Test Results

n/a — implementation not yet started.

## Next Steps

1. Implementer (general-purpose, background) implements per this contract.
2. Sub-agent code-review on the resulting commit.
3. Supervisor (me) reviews verdict + commits LOG entry.
4. External supervisor review.
5. On approval: tracking-close kickoff for alpha-2 (`PRD-record-capture-modes`).
6. After alpha-2 ships and approves: cluster tracking-close + `v0.9.0` tag.

## Blockers

None.

## Context for Next Agent

- Recent tip: `18fd668` (provider-model-routing audit). Tag: `v0.8.1` at `e0b7ee9`.
- The four foundation PRDs in `docs/prds/` are draft status:
  - `PRD-feature-file-claims.md` ← THIS slice. Authoritative contract.
  - `PRD-record-capture-modes.md` ← alpha-2.
  - `PRD-feature-patch-identity-metadata.md` ← beta (deferred; depends on ADR-026 patch-generation-manifest-boundary).
  - `PRD-feature-patch-amend.md` ← gamma (deferred; depends on ADR-025 patch-amendment-policy).
- Reserved future ADR slots: ADR-024 (capture-context-privacy-boundary), ADR-025 (patch-amendment-policy), ADR-026 (patch-generation-manifest-boundary), later ADR-027 (capture-metadata-branch), ADR-028 (structural-middle-pass-boundary). None are blocking Wave alpha.
- The keypoint/structural-fingerprint experiment runs in parallel; it does NOT gate Wave alpha. Outputs feed later waves only.
- Cobra command tree: existing `feature` parent subcommand is in `internal/cli/cobra.go` (look for `featureCmd` or similar). Add the `claim` subtree under it. Follow the existing pattern of subcommand registration (see `record`, `apply`, `reconcile`, `land`).
- The `## Side Research — State-of-the-art middle pass (2026-05-10)` section below is preserved verbatim across handoff resets.

---

## Side Research — State-of-the-art middle pass (2026-05-10)

Paper-only exploratory pass completed for a non-LLM middle layer between
deterministic reconcile heuristics and full provider/coding-agent workflows.
This does **not** change code, schema, CLI behavior, roadmap status, PRDs, or
ADRs.

### Research packet

Created `docs/state-of-the-art/` with docs modeled after the existing market
research / PRD conventions: header block, related links, refresh triggers,
references, open questions, and disputes.

Files:

- `docs/state-of-the-art/README.md`
- `docs/state-of-the-art/patch-theory-and-commutation.md`
- `docs/state-of-the-art/patch-identity-and-structural-fingerprints.md`
- `docs/state-of-the-art/search-based-patch-application.md`
- `docs/state-of-the-art/experiment-guide-structural-middle-pass.md`
- `docs/state-of-the-art/tpatch-metadata-for-patch-identity.md`
- `docs/state-of-the-art/patch-capture-context-research-brief.md`
- `docs/state-of-the-art/patch-capture-prior-art-and-hooks.md`
- `docs/state-of-the-art/research-roadmap.md`
- `docs/state-of-the-art/tpatch-middle-pass-synthesis.md`

### Findings

1. Patch theory is useful as vocabulary for identity, inverse, composition,
   commutation, dependency, and conflict, but tpatch should not claim
   Darcs/Pijul guarantees on top of unified diffs.
2. Patch identity should be treated as a ladder: exact bytes, `git patch-id`,
   token fingerprints, AST/CFG/PDG similarity, behavioral checks, and finally
   provider/human intent judgment.
3. Computer-vision feature matching maps to code relocation: detect salient
   code keypoints, compute local descriptors, match across old/new upstream,
   reject outliers, then attempt relocated apply in a shadow tree.
4. Search-based application should operate only on uncertain patch clusters,
   after deterministic dependency/commutation pre-passes shrink the search
   space.
5. Beam search is the likely first practical non-LLM planner; MCTS and
   evolutionary algorithms remain candidates for larger uncertain clusters.
6. Vector retrieval / RAG fits as a distinct middle layer: dense retrieval can
   rank likely patch/hunk/code-region matches below full provider reasoning,
   while generation over retrieved context still belongs to the provider tier.
7. The experiment guide defines collection formats for feature metadata, hunks,
   keypoints, fingerprints, retrieval results, commutation relations,
   candidate apply attempts, metrics, and ground-truth labels.
8. First-party tpatch metadata should be the happy path for tpatch-aware repos:
   current metadata is good for lifecycle/DAG reasoning, but future patch
   generations, dependency version snapshots, operation IDs/read-write sets,
   structural anchors, relation artifacts, and vector manifests would make
   identity and ordering easier before fuzzy fallback.
9. A new patch-capture research brief preserves this PRD/ADR queue and defines
   the next front: Quilt-style explicit file claims, Git index/hook boundaries,
   IDE hooks, coding-agent event logs, and privacy-safe agent context capture.
10. Entire is verified as a concrete prior-art target. Its model uses Git hooks,
    agent hooks, commit trailers, a separate `entire/checkpoints/v1` metadata
    branch, shadow checkpoints, full transcript/session storage, redaction, and
    optional checkpoint remotes. tpatch should borrow the Git-native linking
    pattern but default toward summaries/references over raw transcripts.
11. `docs/state-of-the-art/research-roadmap.md` is now the durable exploratory
    tracker so research can advance independently if `docs/handoff/CURRENT.md`
    is reassigned to implementation work.
12. Amendment models differ by tool: Quilt/StGit usually refresh the managed
    patch, Git supports both amend and fixup/squash-forward workflows, Aider
    favors small commits plus undo, and Entire preserves context links around
    rewrites. tpatch likely needs canonical-current patch plus append-only
    generations, with explicit amend/fixup/fold/fork semantics.

### PRD drafts promoted from research (2026-05-13)

The first capture/metadata foundation PRDs were drafted as paper-only planning
docs:

- `docs/prds/PRD-feature-file-claims.md`
- `docs/prds/PRD-record-capture-modes.md`
- `docs/prds/PRD-feature-patch-identity-metadata.md`
- `docs/prds/PRD-feature-patch-amend.md`

`docs/state-of-the-art/research-roadmap.md` is updated to point at these drafts.
The remaining gate before implementation is review/acceptance of the queued
capture privacy and amendment-policy ADRs plus PRD review.

### Candidate follow-up names

These are research outputs only, not queued roadmap work. Four items below now
have draft PRDs as noted above.

- `PRD-structural-patch-fingerprints`
- `PRD-feature-patch-identity-metadata`
- `PRD-dependency-version-snapshots`
- `PRD-recipe-operation-identity`
- `PRD-structural-anchor-manifest`
- `PRD-patch-vector-index`
- `PRD-reconcile-commutation-graph`
- `PRD-reconcile-search-planner`
- `ADR-structural-middle-pass-boundary`
- `PRD-reconcile-planner-audit-artifacts`
- `PRD-feature-file-claims`
- `PRD-record-capture-modes`
- `ADR-patch-amendment-policy`
- `PRD-feature-patch-amend`
- `PRD-active-feature-session`
- `PRD-agent-event-log`
- `PRD-ide-capture-hooks`
- `PRD-git-hook-capture-guards`
- `ADR-capture-context-privacy-boundary`
- `ADR-capture-metadata-branch`
- `PRD-record-context-summary`

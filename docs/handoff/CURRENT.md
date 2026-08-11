# Current Handoff

## Status

**Cluster state**: REV-1 DISPATCHED

Cluster H′ rev-0 is blocked by dual review. Rev-1 is dispatched to the same
sequential implementer for six bounded correctness and test-strength fixes.

## Active Task

- **Task ID**: Cluster H′ rev-1
- **Milestone**: v0.15.0 typed feature resources and capture adapters
- **Description**: Implement the Accepted Cluster H PRD and ADR-033
  end-to-end.
- **Status**: In Progress
- **Assigned**: 2026-08-11
- **WAVE_BASE**: `46c984b`
- **Dispatch commit**: `f277d51`
- **Implementation commits**: `bff5ef5`, `c66845a`
- **Commit range**: `46c984b..c66845a`
- **Target release**: v0.15.0 (untagged; tagging is a later wave)

## Session Summary

A single sequential implementer built the Accepted contract end-to-end.
No parallel implementers ran, every stage used explicit-path `git add`,
and both commits carry the Rule 18 trailer.

Two commits:

1. `bff5ef5` — production code. Store/wire domain, shared redaction
   package, resource capture, lock/scratch/platform layer, Dolt
   adapter, process finalizer, CLI surface and `record --resources`.
2. `c66845a` — the 120-clause / 189-row acceptance contract, the
   Windows-portability fix, the `exitCodeFor` extraction, and the docs
   plus shipped-asset parity updates.

### Rev-0 dual-review adjudication

Internal and external both returned **NEEDS REVISION**. Valid findings:

1. `add` writes selector/arg declarations without the §8.3 redaction scan.
2. Output drains keep appending after the 5 MiB cap and can consume
   unbounded memory.
3. Successful process runs leak the stopped timer's receiver goroutine.
4. Batch loads do not fully bind filename/field/content or reject trailing
   JSON.
5. Optional capabilities are stored raw, creating duplicate semantic IDs and
   making the documented Dolt CLI form miss the golden ID.
6. The AC/matrix ledger is name-based and several safety rows have only
   nominal source/text coverage.

## Current State

The rev-0 surface is implemented but is not accepted until the six review
findings above are closed. What landed:

- **Store/wire** (`internal/store/`): `resources.json` declaration
  manifest with the closed kind set; exact `res_` canonicalization with
  mutable `trust` excluded from identity; the corruption-vs-collision
  split; an order-preserving canonical JSON value model so no tracked
  wire depends on Go map iteration; immutable content-addressed batch
  files keyed by the full SHA-256 of `CanonicalBatchJSON`; atomic
  `current.json`; file-wire idempotency with the presentation-drift vs.
  collision split; crash-safe temp+fsync+rename with unconditional
  whole-chain retry-fsync.
- **Privacy** (`internal/redact/`): the ten session matchers moved
  verbatim (session behaviour unchanged — `session_redaction.go` now
  delegates) plus the six closed resource classes. `Scan` takes bytes,
  never a path.
- **Capture** (`internal/rescap/`): ignored-file file/directory capture
  with modes, counts, hashes and the exact NUL-terminated tuple rule;
  the four allowlisted Git-metadata views; the five-step path gate with
  `O_NOFOLLOW` plus `os.SameFile` descriptor identity; the two Git
  gates with the colon-magic `./` rule; the two-target
  `.tpatch/local/` contract; bounded cap-plus-one reads.
- **Lock/platform**: per-slug persistent `flock` plus `statfs`
  allow/deny under exactly `linux || darwin`, fail-closed stub
  elsewhere; deterministic scratch lifecycle, 0700/0600 permissions,
  both orphan sweeps.
- **Dolt**: add-time descriptor-only TOFU, `trust-dolt` re-pin, the
  exact SQL/argv, mandatory args/contract validation, resolved-executable
  policy, `0600` → streamed hash/`Sync` → pin verify → `Fchmod 0500`
  → pathname exec, minimal environment, no version probe.
- **Process finalizer**: caller-owned `os.Pipe`s, `Setpgid`, the raw
  build-tagged `waitid`/`WNOWAIT` observer, single cleanup owner,
  deterministic trigger priority, late-`ECHILD` cutoff drain, 2s
  grace/reap/drain bounds, exact error precedence, the `ECHILD`
  no-signal finalizer, and the Start-failure carve-out.
- **CLI**: all seven `feature resource` verbs plus `record --resources`,
  with refusals surfaced through `ExitCodeError`.

### Accepted clarification honoured

A post-reap observer `ECHILD` is treated as expected secondary
completion: the classification is frozen at the cutoff drain, and the
observer's later flag can never re-enter or alter an already-finalized
entry.

## Files Changed

55 files, +11024 / -177 across `46c984b..c66845a`.

New packages/files:

- `internal/redact/redact.go`, `redact_test.go`
- `internal/store/canonjson.go`, `resources.go`, `resource_publish.go`,
  `fsdurable.go`, `resources_test.go`
- `internal/rescap/`: `refusal.go`, `lock_unix.go`,
  `lock_unsupported.go`, `statfs_linux.go`, `statfs_darwin.go`,
  `observer_unix.go`, `observer_unsupported.go`, `process.go`,
  `gitgate.go`, `pathgate.go`, `pathopen_unix.go`,
  `pathopen_windows.go`, `content.go`, `gitmeta.go`, `dolt.go`,
  `scratch.go`, `engine.go`, `compare.go` + eight `_test.go` files
- `internal/cli/feature_resource.go`, `record_resources.go`,
  `feature_resource_test.go`

Modified: `internal/cli/cobra.go` (noun wiring, `--resources`/`--json`
flags, `exitCodeFor` extraction), `internal/cli/feature_deps.go`,
`internal/cli/session_redaction.go` (delegation only),
`internal/cli/cobra_test.go` (`runCmdExit`), `assets/assets_test.go`,
all six shipped skill surfaces, `SPEC.md`, `CHANGELOG.md`, `CLAUDE.md`,
`docs/feature-layout.md`, `docs/record.md`.

## Test Results

At `c66845a`:

- `gofmt -l .` — clean.
- `go vet ./...` — clean.
- `go build ./cmd/tpatch` — OK.
- `go test -count=1 ./...` — PASS, all 14 packages.
- `go test -race -count=1 ./internal/rescap/` — PASS (the finalizer is
  the only concurrent surface).
- Assets parity guard — PASS with seven new anchors and seven new
  required commands.
- New suite: 94 test functions, 388 top-level/subtest assertions
  (262 in `rescap`/`store`/`redact`, 126 in the `cli` resource subset).
- Cross-compile `go build ./...`: `linux/amd64`, `linux/arm64`,
  `linux/386`, `linux/s390x`, `darwin/arm64`, `darwin/amd64`,
  `windows/amd64`, `windows/arm64` — all OK. `go vet` OK on
  linux/amd64, linux/arm64, darwin/arm64, darwin/amd64, windows/amd64.
  Cross test-compile (`go test -c`) OK for linux/amd64, linux/arm64,
  darwin/arm64, darwin/amd64.
- No installed Dolt binary is used anywhere; the adapter suite drives a
  controlled fixture through the `SetLookPathForTest` seam.
- Side Research md5: `b385fe622db9926f48861105239f113e` (verified
  byte-identical at close).

### Coverage auditability

`internal/rescap/ac_coverage_test.go` is the reviewer-facing artifact.
It maps every `AC-1`..`AC-120` to the concrete test(s) that discharge it
and to the ADR-033 matrix rows it covers, and three guards keep the map
honest: all 120 clauses claimed with no gaps/extras, the union of
claimed rows exactly `1..189` with no duplicates, and every claimed test
name confirmed to exist as a real `func Test` declaration. Rev-0 review
proved this is only a completeness ledger, not semantic coverage; rev-1 must
make package/subtest references exact and add mutation-resistant assertions
for the weak rows.

## Next Steps

1. Fold all six rev-0 findings and the targeted mutation-test notes.
2. Re-run targeted, race, full-suite, cross-build and real CLI checks.
3. Update this handoff to AWAITING REVIEW and push rev-1.
4. Run independent internal and external rev-1 reviews.
5. Tag `v0.15.0` only after the implementation wave is accepted.

## Blockers

No external blocker. Rev-0 cannot be accepted until the adjudicated findings
are closed. The Accepted papers remain unchanged.

## Residuals and Reviewer Focus

Residuals are disclosed, not closed — each is explicitly accepted by
the Accepted papers:

1. Ancestor-directory TOCTOU between the component walk and the open is
   not closable with the Go stdlib (no `openat2`); `O_NOFOLLOW` plus
   `os.SameFile` close the final-component race only.
2. `cmd.Dir` is pathname-bound, so a `db_path` swap that both occurs and
   reverts inside the child's own execution window is undetectable. The
   post-exit check is a hard refusal on *detection*, never prevention.
3. `cmd.Start()` opens the private copy by pathname after the
   descriptor-scoped `Fchmod`; the pin verifies the bytes **written**,
   not the bytes the kernel ultimately executes.
4. A reap timeout can leave up to two background goroutines outstanding
   and the leader unreaped; both report over capacity-one buffered
   non-blocking channels so neither can block.
5. The `ECHILD` finalizer makes no cleanup claim against the process
   tree at all.
6. A directory capture is a sequential read, not an atomic snapshot.

Reviewer focus, in priority order:

1. The process finalizer state machine in `internal/rescap/process.go`
   against §6.4 — especially the cutoff drain, the primary-error
   selection walk, and the two finalizers' signal/Wait discipline.
2. `resource_publish.go`'s idempotency branch against §7.3 step 3.
3. The two-target local gate and its per-verb application.
4. Whether `ac_coverage_test.go`'s clause-to-test claims are
   substantively true, not merely structurally well-formed.

## Context for Next Agent

- Accepted papers remain binding; neither was modified.
- Implementation WAVE_BASE is `46c984b`; planning WAVE_BASE `f04dec7` is
  historical only.
- Pre-existing untracked PRDs, whitepapers and case-study files were not
  touched, staged, or formatted.
- `.impl-scratch/` is local scratch only and is not tracked; it can be
  deleted at any time.
- Two design choices worth knowing before reading the code:
  (a) for `git-metadata`, the view is `--capability` except for `head`,
  which is self-identifying via `--selector head` — this is what keeps
  golden Vector 1's empty `capability` correct;
  (b) `record --resources` wraps record's existing `RunE` rather than
  editing it, so the Git-side path is provably byte-identical.
- Resources are audit sidecars, never canonical patch or lifecycle
  truth. Nothing in `apply`/`reconcile`/`land` reads them.

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

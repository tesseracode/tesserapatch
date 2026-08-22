# Tessera Patch — Unified Implementation Roadmap

## Legend

| Symbol | Meaning |
|--------|---------|
| ⬜ | Not started |
| 🔨 | In progress |
| ✅ | Complete |
| 🚫 | Blocked |

---

## M0 — Bootstrap ✅

**Goal**: Go module, CLI skeleton, build pipeline.

See `docs/milestones/M0-bootstrap.md` for task list.

## M1 — Core Store & Init ✅

**Goal**: `.tpatch/` data model, `init`, `feature add`, `status`, `config`.

See `docs/milestones/M1-core-store.md` for task list.

## M2 — Provider & Analysis ✅

**Goal**: Provider abstraction, `provider check`, `analyze`, `define`, `explore` with heuristic fallback.

See `docs/milestones/M2-provider-analysis.md` for task list.

## M3 — Apply & Record ✅

**Goal**: Deterministic apply recipe, `implement`, `apply`, `record`, patch capture (tracked + untracked).

See `docs/milestones/M3-apply-record.md` for task list.

## M4 — Reconciliation ✅

**Goal**: 4-phase reconciliation (`reconcile`), `upstream.lock`, provider-assisted semantic detection.

See `docs/milestones/M4-reconciliation.md` for task list.

## M5 — Skill System ✅

**Goal**: 6 harness formats embedded, CLI-driven installation, parity guard test.

See `docs/milestones/M5-skill-system.md` for task list.

## M6 — Bug Bash Validation ✅

**Goal**: Pass the reconciliation bug bash end-to-end against tesseracode/copilot-api.

**Result**: Full pass. Feature A → upstream_merged (Phase 3), Feature B → reapplied (Phase 4 with 3-way merge). All 26 tests pass, typecheck clean. See `../tests/tpatch/BUG-BASH-REPORT.md`.

See `docs/milestones/M6-bug-bash.md` for task list.

---

## Future Milestones (Post-MVP)

## M7 — Provider Investigation & Integration ✅

**Goal**: Evaluate Ollama, OpenRouter, and Anthropic as provider options. Implement the best candidate.

**Result**: Anthropic Messages API adapter added alongside the existing OpenAI-compatible provider. Ollama and OpenRouter confirmed to work with the existing provider. Auto-detection extended. See `docs/adrs/ADR-002-provider-strategy.md`.

See `docs/milestones/M7-provider-investigation.md` for task list.

## M8 — LLM Output Validation & Retry ✅

**Goal**: Structured validation of LLM responses, retry with corrective feedback, quality metrics.

**Result**: `GenerateWithRetry` helper with per-phase validators (JSON for analyze/implement, non-empty for define/explore). Raw responses logged to `artifacts/raw-<phase>-response-N.txt`. Config key `max_retries` + `--no-retry` CLI flag.

See `docs/milestones/M8-llm-validation.md` for task list.

## M9 — Interactive Mode & Harness Integration ✅

**Goal**: `tpatch cycle --interactive` for human-driven flow + `tpatch next` protocol for harness-backed (Claude Code, Copilot CLI, OpenCode) integration.

**Result**: `cycle`, `test`, `next` commands shipped. `tpatch next --format harness-json` emits structured tasks (phase, instructions, context_files, on_complete). `tpatch test <slug>` runs the configured `test_command` and records validation status. All 6 skill formats updated, parity guard extended. Harness integration guides for codex (`docs/harnesses/codex.md`) and Copilot CLI (`docs/harnesses/copilot.md`) written.

See `docs/milestones/M9-interactive-harness.md` for task list.

## Refinement (2026-04-17) — SDK evaluation + presets + tracking cadence ✅

**Goal**: Evaluate OpenRouter/OpenAI/Anthropic Go SDKs and codex/copilot-cli harnesses; adopt the simplest integration with proven parity; strengthen tracking cadence.

**Result**: No third-party provider SDKs adopted (stdlib suffices for our narrow `Check`+`Generate` surface). Added `tpatch provider set --preset` for one-line vendor switching. Wrote harness integration guides for codex and Copilot CLI. Rewrote AGENTS.md context-preservation rules with a per-trigger cadence cheatsheet. See `docs/adrs/ADR-003-sdk-evaluation.md`.

## Distribution (2026-04-17) — `go install` + CI workflow + v0.3.0 release ✅

**Goal**: Make `go install github.com/tesseracode/tesserapatch/cmd/tpatch@latest` work and add a free CI workflow.

**Result**: Renamed the module path to match the repo (`github.com/tesseracode/tesserapatch`). Added `.github/workflows/ci.yml` (matrix ubuntu+macOS, `go-version-file: go.mod`, gofmt/vet/build/test/install smoke). Tagged `v0.3.0` locally; ready to push.

## Planning (2026-04-17) — Native Copilot auth research + PRD ✅

**Goal**: Plan a "native" Copilot auth provider. Confirm whether the existing `copilot-api` proxy is officially supported and whether `github/copilot-cli` is open source.

**Result**: Confirmed `copilot-api` is reverse-engineered and explicitly unsupported by GitHub; confirmed `github/copilot-cli` is closed-source (only README/install/changelog/LICENSE published). Wrote `docs/prds/PRD-native-copilot-auth.md` with a two-phase recommendation (M10 managed proxy, M11 opt-in native PAT provider). Shelling out to the `copilot` CLI is explicitly rejected — it burns premium requests and re-runs its own agent loop. M11 is soft-blocked on a ToS question (can tpatch send editor headers against `api.githubcopilot.com`?).

## M10 — Managed Copilot Proxy UX ✅ (delivered 2026-04-17, pending review)

**Goal**: One-command access to GitHub Copilot via the `ericc-ch/copilot-api` proxy, without tpatch taking on process-supervision responsibilities. **See ADR-004 for the locked-in decisions.**

**Scope**: Global config file at `~/.config/tpatch/config.yaml` (XDG-honouring; macOS defaults to `~/Library/Application Support/tpatch/config.yaml`), reachability probe (`GET /v1/models`, 2s timeout), warn-but-continue on `init`/`provider set`, hard-fail on workflow commands (`analyze|define|explore|implement|cycle`), first-run AUP warning persisted once per user, no log piping, Windows deferred.

**Delivered**:
- `internal/store/global.go` + `types.go::CopilotAUPAckAt`
- `internal/provider/probe.go` (`Reachable`, `IsLocalEndpoint`, `IsCopilotProxyEndpoint`)
- `internal/cli/copilot.go` + `cobra.go::loadAndProbeProvider`
- CI release automation in `.github/workflows/ci.yml` (tag-triggered GitHub Release via `softprops/action-gh-release@v2`, free)
- Tests in `internal/store/global_test.go` (6) and `internal/provider/probe_test.go` (5)
- `docs/harnesses/copilot.md` refresh

**Opt-out**: `TPATCH_NO_PROBE=1` for offline/CI steps.

## M11 — Native Copilot Provider (opt-in) ✅ (delivered, pending review)

**Goal**: First-party Go provider speaking directly to `api.githubcopilot.com` — port of the copilot-api/litellm pattern (session-token exchange via `copilot_internal/v2/token`). Removes the Node/Bun dependency. **See ADR-005 for the locked-in decisions.**

**Blueprint**: ericc-ch/copilot-api's `src/lib/api-config.ts` + `src/services/github/` — client ID `Iv1.b507a08c87ecfe98`, VS Code Copilot Chat editor headers, session-token refresh on ~25-min cadence. ~350–400 LOC of Go.

**Gate**: Requires `provider.copilot_native_optin: true` in global config + acceptance of AUP warning. Editor-header policy ships as-is; will switch to an official compatibility endpoint once GitHub documents one.

**Delivered**:
- `internal/provider/copilot_{auth,login,headers,native}.go` — device-code flow, session-token exchange, on-disk auth store (0600, atomic write, symlink-reject), editor headers matching copilot-api 0.26.7, 401-retry-once semantics.
- CLI: `tpatch provider copilot-login`, `copilot-logout`, `--preset copilot-native`, opt-in gate enforced in `provider set` + `config set`.
- `docs/faq.md` (macOS config-path note, auth-file locations).
- Harness doc updated with "Native path (experimental)" section.

**Opt-in**: `tpatch config set provider.copilot_native_optin true` → `tpatch provider copilot-login` → `tpatch provider set --preset copilot-native`.

## M12 — Provider-Assisted Conflict Resolver ✅ (Tranche B2, v0.5.0)

**Goal**: Phase 3.5 of reconcile — when 3-way merge fails, provider resolves each conflicted file in a shadow `git worktree`. Validation gates output; accept/reject is atomic.

**Scope doc**: `docs/milestones/M12-provider-conflict-resolver.md` (PRD: `docs/prds/PRD-provider-conflict-resolver.md`, ADR: `docs/adrs/ADR-010-provider-conflict-resolver.md`)

**Ships**: shadow worktree plumbing, per-file sequential resolver, Go-in-tree + configurable syntax validation, `reconciling-shadow` state, 6-skill update, ≥5 golden scenarios.

**Out of scope** (v0.5.x follow-ups): parallel calls, chunked context for huge files, opt-in heuristic fallback, auto spec-drift refresh.

## M13 — UX Polish & Quick Wins (Tranche C1, v0.5.1) ✅

## M13.5 — Correctness Fix Pass (Tranche C2, v0.5.2) ✅

Six confirmed findings from the v0.4.3..v0.5.1 delta review, shipped as a focused correctness-only release before starting M14.

- c2-resolve-apply-truthful — silent correctness bug on `reconcile --resolve --apply` (shadow → real tree copy was never happening for auto-apply). Fix: shared `workflow.AcceptShadow` helper used by both manual and auto paths.
- c2-refresh-index-clean — `DiffFromCommitForPaths` no longer leaks intent-to-add entries (uses `GIT_INDEX_FILE` temp index).
- c2-recipe-hash-provenance — stale guard now detects recipe content drift (sha256), not only HEAD drift. Legacy sidecars still accepted.
- c2-remove-piped-stdin — piped stdin auto-confirms remove, matching shipped v0.5.1 contract.
- c2-amend-append-flag — new `amend --append`; replace stays default; mutex with `--reset`.
- c2-max-conflicts-drift — 8 doc sites corrected to match runtime default of 10.

8 regression tests added. Code-review verdict: APPROVED. See `docs/supervisor/LOG.md`.

## M13.6 — Shadow Accept Accounting Fixes (Tranche C3, v0.5.3) ✅

Three external-reviewer findings on the v0.5.2 shadow-accept flow, shipped as a focused correctness release before M14.1.

- c3-separate-resolution-artifact — dual-writer schema collision on `reconcile-session.json` split into `resolution-session.json` (resolver-owned) + `reconcile-session.json` (reconcile summary). Manual `reconcile --accept` works again post-shadow-awaiting.
- c3-accept-stamps-reconcile-outcome — `AcceptShadow` now stamps `Reconcile.Outcome=reapplied`. Unblocks ADR-011 D6 label composition.
- c3-manual-accept-regression-test — `TestGoldenReconcile_ManualAcceptFlow` end-to-end guard.

Code-review verdict: APPROVED. See `docs/supervisor/LOG.md`.

**Goal**: Low-risk, high-daily-use-impact improvements. 8 items: apply default mode, stdin add, progress spinner, editor integration, feature amend/remove, recipe stale guard, record lenient mode.

**Scope**: Inline — no separate milestone file for polish tranches.

## M14 — Feature Dependencies / DAG (Tranche D, v0.6.0) ✅

**Goal**: Stacked / dependent features with hard vs soft semantics, topological reconcile, composable derived labels (`waiting-on-parent` + `blocked-by-parent`), cascade-remove, and amend-invalidation tracking.

**PRD**: `docs/prds/PRD-feature-dependencies.md` (commit `fa4bbb6`) — APPROVED WITH NOTES after 3 review cycles.

**ADR**: `docs/adrs/ADR-011-feature-dependencies.md` — **REQUIRED before M14.1 coding starts**. Locks 8 architectural decisions (storage in status.json, DFS cycle detection, Kahn traversal, labels-not-states, created_by hard-only, upstream_merged satisfies deps, cascade on remove, no parent-patch injection to M12 resolver in v0.6).

**Gated by**: `features.dependencies: true` config flag (default false) until M14.4 lands. Single atomic v0.6.0 flip — no half-shipped intermediate releases.

**Scope (4 sub-milestones, ~1350 LOC total)**:
- **M14.1** — data model + validation (~300 LOC). Dependency struct, cycle DFS, 5 validation rules. ✅ APPROVED (commits `02f1ba9`, `d166281`, `7dd5941`).
- **M14.2** — apply gate + `created_by` recipe op + 6-skill parity-guard rollout (~250 LOC). Coordinated atomic change to recipe schema + all 6 skill formats + `docs/agent-as-provider.md`. ✅ APPROVED (commits `24baf92`, `9a5f2f3`, `4dfe0f1`, `cdd5484`). **Note**: the apply-time `created_by` integrity gate was not wired in M14.2 — closed in the M14 correctness pass below.
- **M14.3** — reconcile topological traversal + composable labels + compound verdict (~500 LOC). Kahn planner, label composition matrix, `blocked-by-parent-and-needs-resolution` compound verdict, M12 interaction. ✅ APPROVED (commits `7c9aee4`, `bccf5e2`, `b9efd07`, `a232a7b`, `4e39384`).
- **M14 correctness pass** — three external-reviewer findings before the M14.4 cutover. F1 (HIGH, was cutover-blocking): wire `created_by` apply-time gate via new `ErrPathCreatedByParent` sentinel, closing the M14.2 gap. F2: clear stale-parent-applied label after a clean reconcile (label/AttemptedAt consistency). F3: suppress parent-derived labels when child outcome is `ReconcileUpstreamed`. ✅ APPROVED (commits `cbe2873`, `071c5ed`, `cc95cbb`, `1e0d064`).
- **C5 fix-pass** — re-review caught two real gaps in the correctness pass. F1 (HIGH): F3 only fired on already-persisted upstreamed status, not the in-flight reconcile-time persistence path; fixed by short-circuiting label composition in `saveReconcileArtifacts` on `result.Outcome ∈ childRetiredOutcomes`. F2 (MEDIUM): PRD §4.3 contract drift — dry-run was erroring on hard-parent `created_by` misses; now downgrades to a warning while apply still errors. ✅ APPROVED (commits `c84c7a6`, `dd72c2c`, `ea94fb7`).
- **M14.4** — user-facing cutover (~1100 LOC across 7 chunks A–G). Chunk A: `tpatch status --dag` (ASCII + `--json`, scoped + full, cycle-safe). Chunk B: `features_dependencies` default flipped to true. Chunk C: `tpatch feature deps` verb tree + `amend --depends-on/--remove-depends-on` + `remove --cascade` (with the `--force ≠ bypass` rule per PRD §3.7 / ADR-011 D7). Chunk D: status-time `ValidateAllFeatures` warnings inline. Chunk E: 6-skill rollout with `created_by` reframed as live apply-time gate. Chunk F: `docs/dependencies.md` user reference. Chunk G: v0.6.0 release (version bump, CHANGELOG, ROADMAP). Tag is supervisor's closeout action. ✅ implementation complete (commits `d1aca5f`, `ca23b35`, `5d5f594`, `97a994f`, `e0a7d47`, plus this release commit).

**Out of scope** (v0.6.x follow-ups): auto-rebase on parent drift (`feat-feature-autorebase`), parent-patch context for resolver (`feat-resolver-dag-context`), per-dep version ranges (`feat-patch-compatibility`), stacked-PR delivery (`feat-delivery-modes`).

## M15 Wave 3 — Verify freshness overlay (v0.6.2) ✅

Successor to the superseded `feat-feature-tested-state` design (ADR-012).
Replaced the tested-as-state model with a Git-like freshness overlay per
ADR-013 / PRD-verify-freshness.md after external review surfaced
lifecycle/freshness conflation. Lifecycle and verification stay separate.

- **Slice A** — `tpatch verify <slug>` cobra shell + V0-V2 real + V3-V9
  stubs + minimal `Verify` sub-record + minimal EXPERIMENTAL skill
  stubs. ✅ APPROVED WITH NOTES (commits `8e2aabe`-`bce2252`,
  ~2026-04-27).
- **Slice B** — `ReconcileLabel` extension + `composeLabelsFromStatus`
  freshness derivation + `tpatch status` / `--dag` / `--json`
  rendering + `tpatch amend` invalidation + truth-table tests.
  ✅ APPROVED (commits `a07acc7` original + `53a4d9a` revision-1
  fixing amend invalidation dead-branch reproduced live by external
  supervisor, ~2026-04-28).
- **Slice C** — V3-V9 real implementations including hard-parent
  topological closure replay (V7/V8). ✅ APPROVED (commits `32f50c8`
  original + `5892ae0` revision-1 fixing V8-must-run-on-closure-replayed-baseline-when-recipe-absent-but-patch-present
  + `23af23e` revision-2 fixing V8-precondition-is-file-presence-not-non-empty-content,
  ~2026-04-29).
- **Slice D** — `tpatch verify --all` + 6-skill rollout + parity-guard
  anchors + `docs/dependencies.md` cross-link + CHANGELOG v0.6.2.
  ✅ APPROVED (commits `19271f7` original + `67730de` revision-1
  surfacing-malformed-status-as-error-row + `e7f8661` revision-2
  ENOENT-vs-stat-error-split + `d390322` revision-3
  workspace-corruption-on-missing-features-dir + `fa93536` revision-4
  defensive-3-way-branch-on-tpatch-stat, ~2026-05-10).

## M16 — Operator polish bundle (v0.6.3 / v0.6.4) ✅

Three small, user-facing fixes. Originally planned as a single
release; split after Slice 2 landed and the user opted to ship
the data-bug fix immediately rather than wait for Slice 3.

- **Slice 1** — `chore-gitignore-tpatch-binary`: add `/tpatch` to
  `.gitignore` to ignore bare build output at the repo root. ✅
  Already in place from a prior cycle (`.gitignore:34: /tpatch`,
  rooted, anchored, `cmd/tpatch/` shadowing avoided per 2026-04-17
  incident comment). `git check-ignore -v tpatch` confirms ignore.
  No commit needed.
- **Slice 2** — `bug-record-roundtrip-false-positive-markdown`:
  fix the A3 ValidatePatchReverse false-positive. Turned out to be
  a real data bug: `gitutil.CapturePatchScoped` /
  `CapturePatchFromCommitsScoped` used `strings.TrimSpace(patch) +
  "\n"` to normalize the tail, eating semantically-significant
  trailing whitespace on the final hunk line (e.g. `+> ` markdown
  blockquote continuations). Replaced with `normalizePatchTail`
  helper that preserves content bytes. Shipped as v0.6.3. ✅
  APPROVED (commit `eba35bf` + sub-agent verdict `84cdac1`,
  external supervisor pass 2026-05-10).
- **Slice 3** — `feat-apply-default-execute` +
  `feat-skills-apply-auto-default` (unified): on inspection the
  CLI default at `internal/cli/cobra.go:586` is already `auto`
  (runs prepare→execute→done), so the simple invocation
  `tpatch apply <slug>` already does the right thing. Real work
  was doc/skill alignment: 6 skill surfaces updated so the
  recommended user-facing invocation is `tpatch apply <slug>`,
  with the four-mode ladder kept as advanced/state-machine
  fallback. New parity-guard regex anchor
  `apply-default-auto/simple-invocation` locks the contract.
  Shipped as v0.6.4. ✅ APPROVED (commit `eab2c3c` + sub-agent
  verdict `4556387`; external NEEDS REVISION on parity-anchor
  false-pass risk → revision `38d13fc` strengthened both weak
  surfaces and tightened anchor to a regex; external pass
  2026-05-10).

## M17 — boundary-capture cluster (v0.8.0) ✅ (shipped 2026-05-12 as v0.8.0; tag at `29a6732`, M17 cluster ship-stack tip at `34815e8`)

Multi-agent paper-design cluster accepted in `docs/supervisor/LOG.md` →
"Review — v0.7 Cluster PRDs (land + auto-base + collision + lock-guard) —
**2026-05-10**". Sliced into Wave A / Wave B / Wave C / Wave D mirroring
the sequencing recorded in that entry. All four waves shipped as v0.8.0
(see per-Wave ship commits below); the cluster archive lives in
`docs/handoff/HISTORY.md`.

**Shipped as v0.8.0** (supervisor decision 2026-05-10; tagged 2026-05-12).
v0.7.0 was reserved for `feat-amend-dependent-warning` — the M15 W3
freshness continuation — which landed first to keep the freshness work
contiguous. The cluster name "v0.7 cluster" is preserved in the LOG
entry's title for historical fidelity, but the implementation milestone
is v0.8.0.

- **Wave A — independent, parallel** ✅ (shipped 2026-05-11, released as part of v0.8.0)
  - **Slice A1 — `impl-record-auto-base`** ✅
    - PRD: [`docs/prds/PRD-record-auto-base.md`](prds/PRD-record-auto-base.md)
    - ADR placeholder: [ADR-016](adrs/ADR-016-record-auto-base-resolution.md)
    - Ship commits: `1d6179c` (v0) + `4484e04` (rev-1: zero-diff refusal + lock-fallback discovery)
    - Wrote shared `internal/store/upstream_lock.go` parser.
    - Follow-up captured: `m17-wave-a1-followup-ambig-discovery-diag` (Low — ambiguous discovery candidate list not surfaced when post-unusable-lock discovery fails; PRD §3.4/§3.5).
  - **Slice A2 — `impl-reconcile-lock-guard` + writer-normalization fix** ✅
    - PRD: [`docs/prds/PRD-reconcile-lock-guard.md`](prds/PRD-reconcile-lock-guard.md)
    - ADR placeholder: [ADR-017](adrs/ADR-017-reconcile-lock-guard-and-writer-normalization.md)
    - Ship commit: `8fc2e4e`
    - Bundled the HIGH-severity writer-normalization fix at `internal/workflow/reconcile.go:596-613` (`updateUpstreamLock()`) per `PRD-reconcile-lock-guard §5.3`.
    - Second parser at `internal/gitutil/lock_guard.go` due to verified `store → gitutil` import cycle (parsers line-for-line equivalent on shared keys; no drift risk). Follow-up cleanup PRD captured in CURRENT.md.
- **Wave B — depends on Wave A (auto-base)** ✅ (shipped 2026-05-11, externally approved 2026-05-12)
  - **Slice B — `impl-record-collision-detection`** ✅
    - PRD: [`docs/prds/PRD-record-collision-detection.md`](prds/PRD-record-collision-detection.md)
    - ADR: [ADR-018](adrs/ADR-018-record-collision-detection-signature.md)
    - Ship commit: `b0a434a`
    - Cross-feature canonical-patch collision detection in `tpatch
      record`; refusal exit 1 with `--allow-collision "<reason>"`
      escape hatch persisted into `record.md`. PRD §8 acceptance map:
      11/11 tests in `internal/cli/record_collision_test.go`.
- **Wave C — depends on Wave A + Wave B** ✅ (shipped 2026-05-11..14, externally approved 2026-05-12)
  - **Slice C — `impl-tpatch-land`** ✅
    - PRD: [`docs/prds/PRD-tpatch-land.md`](prds/PRD-tpatch-land.md)
    - ADRs: [ADR-019](adrs/ADR-019-tpatch-land-trailer-block-schema.md) (Accepted) + [ADR-021](adrs/ADR-021-tpatch-land-global-metadata-carve-out.md) (Accepted, rev-3)
    - Ship stack: `fb5e6ff` (core) + `73a81ed` (skill assets + parity
      guard) + `266dfb4` (ADR + CHANGELOG + handoff) + `32ad3a5`
      (rev-1: ADR ref typo + hard-parent test + `docs/land.md`) +
      `c6f4402` (rev-2: scope global metadata staging + clean tree
      on `--no-record`) + `876c584` (rev-3: PRD carve-out + ADR-021)
      + `19a335e` (rev-4: dry-run carve-out alignment + stale
      wording cleanup).
    - 5-revision history: contract sharpened from rev-0 through
      rev-4; final external supervisor verdict APPROVED end-to-end
      on rev-4.
    - New `tpatch land <slug>` flagship verb composing record →
      safe path-set staging → one Git commit, with the locked
      four-trailer block (`Tpatch-Feature`, `Tpatch-Patch-SHA`,
      `Tpatch-Recipe-SHA`, `Tpatch-Base-Commit`) + repo
      `Co-authored-by:` trailer. Documented in `docs/land.md`.
- **Wave D — independent reconcile fast-path (default-OFF)** ✅ (shipped 2026-05-11, externally approved 2026-05-12)
  - **Slice D — `impl-patch-already-upstream-detector`** ✅
    - PRD: [`docs/prds/PRD-patch-already-upstream-detector.md`](prds/PRD-patch-already-upstream-detector.md)
    - ADR: still not opened (PRD §6 keeps the feature behind
      `Config.PatchIDDetectorEnabled` default-`false` until the
      v0.8.x line decides to flip it on; ADR can be authored when
      the flag flips).
    - Ship stack: `c07e4e2` (v0) + `1d4a89f` (rev-1: phase-1.5
      reads canonical `post-apply.patch` per PRD §5.1).
    - Phase-1.5 deterministic patch-already-upstream detector
      slotted between phase 1 (reverse-apply) and phase 2
      (operation-level) in `internal/workflow/reconcile.go`,
      gated by `Config.PatchIDDetectorEnabled` (default `false`).
    - **Deferred to v0.8.1+** (called out in CHANGELOG):
      PRD §3.2 `--check-applied-only` CLI verb/flag, PRD §3.3
      `--auto-drop-merged` CLI flag, PRD §3.3 hotfix-kind
      auto-drop default gating.

## v0.7.0 — `feat-amend-dependent-warning` ✅

Continuation of the M15 W3 freshness overlay work. Shipped before M17 to
keep the freshness UX contiguous. Adds an amend-detection guard to
`record` that refuses (exit 1) when capturing a feature would orphan a
dependent's `base_commit` or `satisfied_by` SHA via a force-pushed amend,
plus a `--force-amend` escape hatch. Surfaces a `dependent-broken` label
on the affected features in `status`, `status --json`, `status --dag`,
and `status --dag --json`, with a single coalesced diagnostic line per
affected feature listing deduped abbrev SHAs and a recovery hint. New
`internal/store/dependents.go` exports `CollectDependentSHAs`,
`IsAmendBreaking`, `CollectBrokenRefs`. Reachability uses
`gitutil.IsAncestor`; amend signal derived from `HEAD@{1}^ == HEAD^`
when reflog available. Six skill surfaces updated by parity guard.

Ship commits: `8306367` (impl), `6e78eac` (rev-1), `a5e7de0` (sub-agent
verdict), tracking commit + tag `v0.7.0`. External supervisor required
one revision (DAG renderers were missing the overlay; plain-text emitted
one line per ref instead of per feature) — both addressed in `6e78eac`.

## WP-003 — Reconcile Safety and Middle-Pass Foundation ✅

Nine-PRD cluster shipping the reconcile evidence/revision schema (ADR-025)
plus middle-pass detectors, classifiers, audits, and dev-only study
validation. Governed end-to-end by ADR-025. Zero new lifecycle states.
Zero schema drift across four waves. All approved via three-way review
(internal + supervisor-external + user-external).

- **Wave α — reconcile-verdict-evidence + file-novelty** ✅ (approved 2026-05-26)
  - PRDs: [PRD-reconcile-verdict-evidence](prds/PRD-reconcile-verdict-evidence.md), [PRD-reconcile-file-novelty-classifier](prds/PRD-reconcile-file-novelty-classifier.md)
  - ADR: [ADR-025](adrs/ADR-025-reconcile-evidence-and-revision-schema.md)
  - Adds append-only `reconcile-evidence.jsonl`, `file_novelty` classifier
    (`clean-additive` / `overlap-suspect` / `unknown-novelty`), evidence
    artifact reference in status JSON + human evidence hints.

- **Wave β — confirmation-gate + revision-pass-log + hunk-overlap** ✅ (approved 2026-06-28)
  - PRDs: [PRD-upstreamed-confirmation-gate](prds/PRD-upstreamed-confirmation-gate.md), [PRD-reconcile-revision-pass-log](prds/PRD-reconcile-revision-pass-log.md), [PRD-reconcile-hunk-overlap-detector](prds/PRD-reconcile-hunk-overlap-detector.md)
  - Adds confirmation gate before `upstreamed` verdict (evidence-derived
    downgrade to `blocked` with `review_verdict=rejected-upstreamed` and
    human display `[upstreamed-candidate]`); append-only
    `reconcile-revisions.jsonl` with lenient loader + `corrupt_entries`
    JSON envelope; deterministic hunk-overlap pass after file-novelty
    with default `nearby-window=3`.

- **Wave γ-1 — retirement-audit + study-validation + blocked-taxonomy** ✅ (approved 2026-07-10)
  - PRDs: [PRD-reconcile-retirement-state-audit](prds/PRD-reconcile-retirement-state-audit.md), [PRD-reconcile-study-validation](prds/PRD-reconcile-study-validation.md), [PRD-reconcile-blocked-verdict-taxonomy](prds/PRD-reconcile-blocked-verdict-taxonomy.md)
  - Read-only `tpatch reconcile audit-retirement <slug>` + new PRD-named
    `tpatch reconcile confirm-upstreamed <slug>` trigger for auto-audit
    (Path A per γ-1 rev-1 F1). Dev-only case-study validator at
    `internal/tools/studyvalidator/` (not in public CLI, stdlib-only,
    per-corrected-verdict linkage enforcement). 8-category blocked
    taxonomy with deterministic precedence (`dependency-blocked >
    validation-blocked > target-deleted > structural-conflict >
    edit-overlap > shifted-context > clean-additive > unknown-blocked`).

- **Wave γ-2 — path-restructure-detector** ✅ (approved 2026-07-16)
  - PRD: [PRD-reconcile-path-restructure-detector](prds/PRD-reconcile-path-restructure-detector.md)
  - Detector emits `path-restructure` evidence
    (`prefix-move`/`prefix-split`/`target-deleted`/`mixed`/`none`/`unknown`)
    consumed by PRD 8 blocked-taxonomy to upgrade generic `blocked` to
    `structural-conflict` or `target-deleted`. Thresholds config-driven
    (prefix-split ≥3 files ≥2 prefixes; prefix-move ≥5 files).
    Candidate prefixes capped at 5, sorted support-desc + path-asc.
    No provider integration.

**Process artifacts**: 15 carry-forward dispatch rules codified.
Two-opinion external review protocol (supervisor + user-parallel)
caught HIGH BLOCKERs in α rev-0, β rev-0, γ-1 rev-0; confirmed fixes
in β rev-1, γ-1 rev-1, γ-2 rev-0. Full per-wave snapshots archived
to [`docs/handoff/HISTORY.md`](handoff/HISTORY.md).

## v0.11.1 — Stabilization cluster ✅

Post-v0.11.0 stabilization slice addressing release-quality
inconsistencies flagged by external teams + reviewer agents. Four
slices shipped 2026-07-19..2026-07-23. Two-opinion review protocol
continued: 11 consecutive rev cycles with three-way concurrence at
final acceptance; user-external uniquely blocked in 5 of 11.

- **Slice 1 — Asset/CLI parity fixes** ✅ (three-way APPROVED 2026-07-19)
  - Ship stack: `359cd6a`, `fbd8244`, `67ee41a`, `dd2d12b`.
  - Fixed skills recipe schema drift (missing `feature` field / phantom
    `version: 1`), removed nonexistent `feature patch fixup --target`
    flag from all 6 skill formats, and updated `verify --help` text
    for post-Slice-C reality.
  - Anti-drift bonus: `TestSkillRecipeSchemaMatchesCLI` extended to
    decode skill recipe examples into `workflow.ApplyRecipe` directly
    with `DisallowUnknownFields`.

- **Slice 2 — Reconcile docs refresh** ✅ (three-way APPROVED 2026-07-23
  after rev-1 fix of user-external F1 blocker)
  - Ship stack: rev-0 `8a2c632`, `ac00905`; rev-1 `3c8fec5`.
  - Rewrote `docs/reconcile.md` for the v0.11 evidence system covering
    ADR-025 D1-D13 and all 9 WP-003 PRDs. User-external caught F1
    (flag-surface overclaim contradicting cobra persistent-flag
    inheritance); rev-1 added an explicit inheritance note plus the
    human `evidence:` hint description and a CHANGELOG prefix fix.
  - New carry-forward rule 11 (flag-surface accuracy) binding.

- **Slice 3 — Release ops cleanup** ✅ (supervisor-direct 2026-07-23)
  - Ship commit: `19b9969`.
  - Published GH Releases for the 5 missing tags: v0.8.0, v0.8.1,
    v0.9.0, v0.10.0, v0.11.0 (marked `Latest`). Notes extracted from
    CHANGELOG.md via `awk … | sed '$d'`.
  - Added [`RELEASING.md`](../RELEASING.md) documenting the 3-artifact
    release lock-step (CHANGELOG → tag → `gh release create --verify-tag
    --notes-file --latest`) with anti-drift guardrails and a
    doctor-command candidate for pre-tag CI verification.

- **Slice 4 — `PRD-tpatch-doctor` paper-only draft** ✅ (three-way APPROVED
  2026-07-23 post-amend)
  - Ship stack: `e1ed73e` + `4523cb8` (F1/F2 amend).
  - Drafted [`docs/prds/PRD-tpatch-doctor.md`](prds/PRD-tpatch-doctor.md)
    with D1-D8 detection clauses (metadata drift, `patch-generations.json`
    presence, in-tree skill assets, lock formats, evidence artifacts,
    release drift, recipe schema, and hard invariants) and 29 acceptance
    criteria. Status: `Proposed`. Future implementation slice will ship
    the actual `tpatch doctor` command.
  - New carry-forward rule 17 (totality-claim verification) binding.

**Cluster process artifacts** (non-shipping):
- 17 dispatch-brief carry-forward rules codified (up from 15+1 at
  cluster start; rules 11 and 17 added).
- Full per-slice snapshots archived to
  [`docs/handoff/HISTORY.md`](handoff/HISTORY.md).

## v0.11.2 — `tpatch doctor` implementation cluster ✅

Ships the `tpatch doctor` command implementing PRD-tpatch-doctor D1-D8
in a 4-wave cluster (α/β/γ/δ). Read-only detection by default; opt-in
`--fix` with mandatory backups + idempotence + refuse-on-collision;
deterministic `--json` output; exit codes 0/1/2 per PRD §6.24.
Full-cluster acceptance sweep 29/29 §6 MET at cluster close 2026-07-29.

- **Wave α — Scaffold + D1 + D2 + D8** ✅ (three-way APPROVED WITH NOTES
  2026-07-27, F-EXT-1 malformed-trailer caught by supervisor-external)
  - Ships: doctor CLI shell (`--dry-run` default, `--fix` opt-in,
    `--json`, `--check <id>`); D1 feature metadata drift; D2
    `patch-generations.json` presence/schema; D8 hard-invariant +
    malformed-artifact handling. §6.1-§6.7 + §6.20-§6.29.

- **Wave β — D3 + D7** ✅ (three-way APPROVED zero findings 2026-07-28)
  - Ships: D3 stale in-tree skill assets via byte comparison across
    the six `installSkills` paths, marker-check + refuse-on-
    unrecognized-user-content + refuse-on-backup-collision; D7 recipe
    schema drift via shared `DecodeApplyRecipeStrict` decoder helper
    (rule 16 anti-drift consolidation with `TestSkillRecipeSchemaMatchesCLI`).
    §6.8, §6.9, §6.18, §6.19. First wave with mutating fixers.

- **Wave γ — D4 + D5** ✅ (three-way APPROVED WITH NOTES 2026-07-28;
  F1 folded to Wave δ)
  - Ships: D4 lock format detection + safe format-only normalization
    (refuses commit-advance + branch-guess; no remote fetches); D5
    missing `reconcile-evidence.jsonl` detection + malformed JSONL
    reporting on evidence + revisions. §6.10-§6.13.

- **Wave δ — D6 + F1 fold-in + F2 close + F3 pre-ship** ✅
  (three-way APPROVED 2026-07-29)
  - Ships: D6 CHANGELOG/tag/GH-Release drift detection with local
    `--release-metadata <file>` input (no GH API calls, no auth
    prompts) — auto-gated to tpatch-authored release context via
    `isTpatchStyleReleaseContext` + semver tag filter, remediations
    self-contained per ADR-020 inline-minimal principle. §6.14-§6.17.
  - Also: F1 fold-in from Wave γ (documented behavior change to
    shipped `tpatch reconcile review list` per ADR-025 D11); F2
    close (upstream-workspace false-positive drift + ADR-020-class
    docs-reference defect); F3 pre-ship (semver guard on
    `release-gh-release-unknown` warning loop).

**Cluster process artifacts** (non-shipping):
- 20 dispatch-brief carry-forward rules codified (up from 17 at
  cluster start; rules 18 structural trailer verification, 19
  loader-caller-tracing, and 20 empirical user-workspace
  reproduction added).
- Two-opinion protocol scoreboard: 15 consecutive rev cycles at
  three-way concurrence at final acceptance; user-external uniquely
  blocked or caught real production-behavior findings in 7 of 15
  rev cycles at rev-0; supervisor-external uniquely caught F-EXT-1
  in Wave α.
- Full per-wave snapshots archived to
  [`docs/handoff/HISTORY.md`](handoff/HISTORY.md).

## v0.11.3 — verify V8 double-apply fix ✅

Stabilization slot fixing [GH #2](https://github.com/tesseracode/tesserapatch/issues/2):
`tpatch verify` V7 (`recipe_replay_clean`) applied the target feature's
`apply-recipe.json` into the shadow tree, then V8
(`post_apply_patch_replay_clean`) checked the equivalent canonical
`post-apply.patch` against that already-modified tree. For correct
recipe/patch pairs whose two artifacts encoded the same change, V8
was double-applying and failing.

- Ship stack: `801db13` (fix) → `0a42641` (regression test) → `be374a1`
  (CHANGELOG + handoff) → `311e25e` (internal review) → `b1b197b`
  (supervisor-external) → `84a2f88` (release commit).
- Tag: `v0.11.3` on `origin/v0.11.3`. GH Release marked `Latest` at
  https://github.com/tesseracode/tesserapatch/releases/tag/v0.11.3.
- Fix (Option A): snapshot the closure-replayed baseline after parent
  replay via `git add -A -f` + `git write-tree`, run V7, then — only
  when a recipe was applied — reset the shadow back to that tree with
  `git read-tree --reset -u` + `git clean -fdx` before V8. V7 and V8
  now validate independently against the same baseline.
- Three-way review APPROVED with zero adversarial findings across all
  three passes (16 consecutive rev cycles at three-way concurrence).
  User-external demonstrated a stronger Rule 20 application: detached
  worktree at pre-fix commit + test-copy + FAIL verification — pattern
  documented as an optional rigor extension.
- Reporter: t3code `session-search` migration on v0.11.1. GH Issue #2
  closed with fix reference.
- No ADR-013 amendment needed. Three new unexported helpers
  (`snapshotShadowTree`, `resetShadowToTree`, `runShadowGit`) added
  in `internal/workflow/verify.go`. Rule 19 trace clean — no exported
  API surface change.

## Cluster G planning — 2026-08-05 — v0.14.0 candidate: `tpatch feature unapply` (PRD + ADR pair) ✅ SHIPPED

Planning-phase cluster for v0.14.0 `tpatch feature unapply` (PRD-feature-unapply target). Data-model extension (new `StateUnapplied` state) with lifecycle-command scope; planning phase separated from implementation phase mirroring Cluster F. Docs-only. Dual review at rev-0/1/2/3. **4 review revs, 8 review turns (4 int + 4 ext), 5 implementer commits + 4 supervisor tracking commits**.

**Deliverables** (both flipped Proposed → Accepted at consolidation `e1a5898`):
- `docs/prds/PRD-feature-unapply.md` — refreshed 587 → ~950 lines; moved from `.wave-close-allowlist` untracked to tracked.
- `docs/adrs/ADR-032-feature-unapply-state-boundary.md` — new ~1100 lines. D1-D8 with 61-row test matrix.

**Key locked decisions**:
- **D7 composition (Alt A)**: `unapplied` and `rejected` are parallel independent states, mutually exclusive. Resolves ADR-031 D6's **data-model composition sub-question**; retirement-command sub-question remains explicitly deferred to future `tpatch retire` (`tpatch remove` is the destructive current workaround).
- **D6 atomicity**: 8-step transactional protocol. status.json write via `os.CreateTemp` + `os.Rename` (POSIX-atomic). **Cluster G' pre-req: upgrade `SaveFeatureStatus` (`store.go:368`) to atomic-rename before v1 unapply lands.**
- **Impl Note 4 guard placement**: first statement of `applyConfirmUpstreamedTransition` (caller, `cobra.go:2626`). NOT `saveConfirmUpstreamedStatus` (callee, `cobra.go:2699`). Verbatim byte-match to `cobra.go:2627-2634` source comment.
- **D3 wire schema**: `unapply-session.json` byte-identical PRD §7.1 vs ADR D3.
- **D8 command shape**: noun-scoped `tpatch feature unapply`, parallels ADR-031 D10 with inverse decision (Cluster F' shipped bare verbs `reject`/`reopen`).
- **§5.1 dependency invariant**: best-effort gate + DAG warning (matches D2), NOT absolute closure.
- **§3.5 exit envelope**: full 0/1/2/3 coverage per case class.
- **§15 acceptance**: 39 items, 1:1 mirror in matrix.

**Rev arc scoreboard**:
| Rev | Verdicts (int / ext) | Key finding class |
|-----|----------------------|-------------------|
| rev-0 (`ea1d01a`) | BLOCKED 8H+2M / NEEDS REVISION 10 | Composition oversell, wire-schema divergence, 7/13 fabricated citations, exit envelope missing, matrix count mismatch, symmetric-invariant contradiction |
| rev-1 (`7ff55ee`) | BLOCKED 3H+1L / NEEDS REVISION 2M | **9/10 rev-0 external findings closed byte-for-byte, 16/16 anchors verified — citation-fabrication vector fully neutralized.** Impl Note 4 caller/callee inverted; matrix false completeness; status.json atomicity gap |
| rev-2 (`6771544`) | BLOCKED 1H+1M / APPROVED WITH NOTES 1L+1I | AC-35 row 43 semantic contradiction with PRD §3.5:271; AC-10c missing |
| rev-3 (`e1a5898`) | **APPROVED clean / APPROVED clean** | Convergent-close arc terminates. Zero residuals. State partition exhaustive (12 states × once each). |

**Range**: `99a1e06..e1a5898`. WAVE_BASE: `2c8a207`. Side Research md5 preserved: `b385fe622db9926f48861105239f113e`.

**Implementation**: shipped in Cluster G' below. The final implementation
follows this corrected scope exactly and is tagged v0.14.0.

> Where this summary and the accepted papers disagree, `ADR-032` + `PRD-feature-unapply` govern. This line previously named `UnappliedStatus` and a Rule-7-parallel `ErrUnappliedParent` — neither exists in either paper; corrected 2026-08-10.

## Cluster G' implementation — 2026-08-10 — v0.14.0 `tpatch feature unapply` ✅ SHIPPED

Implementation phase for the Accepted Cluster G paper package. **Six review
revisions, 21 pre-consolidation wave commits, three-way APPROVED at rev-5.**
WAVE_BASE `9e77617`; implementation tip `6941d41`; release tag `v0.14.0`
points at the consolidation commit.

**Delivered**:
- `StateUnapplied` as the twelfth real lifecycle state.
- Atomic `SaveFeatureStatus` (`CreateTemp` + fsync + same-directory rename).
- `tpatch feature unapply <slug>` with dry-run, dependency/worktree
  preflight, strict reverse check, detached preview, path snapshot/restore,
  D3 fixed audit envelope and D6 rollback.
- Direct canonical-patch reapply via `tpatch apply`, preserving patch,
  generation and base metadata.
- Full status/JSON/FEATURES/next/apply/reconcile/land/record/amend/reject/
  reopen/confirm-upstreamed/dependency integration.
- Corrected D2 split: dependency edge creation onto unapplied parents remains
  allowed; unapplied hard parents do not satisfy apply.
- SPEC + dependency docs + six shipped skill surfaces + parity anchors.
- All 61 ADR matrix rows; 1022 top-level tests PASS / 0 FAIL.

**Review arc**:

| Rev | Verdict | Fold |
|-----|---------|------|
| rev-0 | internal NEEDS REVISION; external no usable verdict | literal pathspecs + complete asset source set |
| rev-1 | internal NEEDS REVISION | canonical direct reapply, gate ordering, file↔directory rollback, amend preflight |
| rev-2 | internal APPROVED; external NEEDS REVISION | mode-only warning strictness + canonical-path-scoped dirty comparison |
| rev-3 | internal NEEDS REVISION | complete staged/unstaged/untracked HEAD projection |
| rev-4 | internal NEEDS REVISION | linked-worktree effective-index resolution |
| rev-5 | **internal APPROVED / external APPROVED** | terminal clean close |

**Safety patterns closed before ship**: inverse canonical capture through
record/cycle/feature-patch/apply; partial reapply residue; rename/copy side
omission; spaces/Unicode/pathspec magic; symlink and traversal; file↔directory
transitions; mode-only false success; unrelated and staged owned-path dirt;
linked-worktree index layouts.

**Post-release close-claim review**: APPROVED WITH NOTES. Headline claims and
release invariants confirmed. Disclosure/test-name/SPEC prose notes folded on
`main` without moving the v0.14.0 tag.

## v0.15.1 correctness batch — GH #7 + GH #8 ✅ SHIPPED

**Dispatch**: 2026-08-12.
**WAVE_BASE**: `5d15fcf`.
**Mode**: sequential; one implementer per wave.
**Release**: v0.15.1.
**Pre-consolidation range**: `5d15fcf..99adbc9` (73 commits across three
independently gated waves).

Validated against current `main`:

- **GH #7**: nested registered linked worktrees are captured as mode-160000
  gitlinks by manual apply/default record, and remain in scoped land's
  outside-path plan.
- **GH #8**: after land, verify's V7/V8 shadow starts at HEAD where the
  feature is already materialized; V8 forward-check therefore fails even
  after a correct committed-range re-record. This is distinct from GH #2's
  V7→V8 same-shadow double-apply defect.

Sequential plan:

1. **Wave A — GH #7**: shared nested-worktree discovery/exclusion for
   apply/record capture and land dirty-path planning.
2. **Wave B — GH #8 contract**: amend verify/land semantics for landed
   already-materialized overlays; record the architecture decision.
3. **Wave C — GH #8 implementation**: implement the accepted contract while
   preserving GH #2's regression.

Released as annotated tag v0.15.1 after all three waves passed dual review.

**Wave A rev-0** (`469e9dd`): original reproducer APPROVED; internal NEEDS
REVISION. Rev-1 covers exact trailing-space/tab path bytes, strict legacy
porcelain dequoting/fail-closed behavior, diffstat pathspec exclusion and
CLI-level discovery-failure regression.

**Wave A rev-1** (`3c583e6`, diagnostic fold `04ac7f2`): original reproducer
APPROVED; internal NEEDS REVISION. Rev-2 removes the fundamentally ambiguous
legacy fallback, filters reconcile refresh, and makes apply/record discovery
transactional before the first artifact write.

**Wave A rev-2** (`bc85956`): original reproducer APPROVED; internal NEEDS
REVISION. Rev-3 caches land discovery before embedded record and introduces
strict quoted diff-header parsing so stale newline-worktree patches cannot
broaden refresh to the full tree.

**Wave A rev-3** (`38c237b`): original reproducer APPROVED; internal NEEDS
REVISION. Rev-4 hardens strict parsing against headerless/Go-only/a-side
malformation and revalidates linked worktrees immediately before land staging.

**Wave A rev-4** (`bf58acc`): original reproducer APPROVED; internal NEEDS
REVISION. Rev-5 adds an exact index snapshot/stage/audit/rollback transaction
so a worktree registered after planning cannot enter the commit.

**Wave A rev-5** (`972c859`): original reproducer APPROVED; internal NEEDS
REVISION. Rev-6 replaces live-index rollback with isolated temporary-index
staging and guarded publication, preserving concurrent operator state.

**Wave A rev-6** (`0b557ef`): original reproducer APPROVED; internal NEEDS
REVISION. Rev-7 adds durable index publication and crash recovery while
retaining owned-lock cleanup across every failure.

**Wave A rev-7** (`32602d5`): original reproducer APPROVED; internal NEEDS
REVISION. Rev-8 locks and compares recovery state, retains evidence on failed
publish, and commits directly from the durable retained index.

**Wave A rev-8** (`9314bbc`): external APPROVED WITH NOTES; internal NEEDS
REVISION. Rev-9 rolls back successful hook-contaminated commits, derives lock
paths solely from the effective index, and restores Windows test portability.

**Wave A rev-9** (`24e92e0`): internal APPROVED WITH NOTES, external
APPROVED. GH #7 accepted; original reproduction and all adversarial path,
capture, land, index-transaction and crash-recovery scenarios closed. Wave A
terminal range: `5d15fcf..54580d5`.

**Wave A close**: ACCEPTED at `ad39e4a`; GH #7 closed; 8/8 gate PASS.

**Wave B — GH #8 contract** ✅ ACCEPTED. Dispatch 2026-08-12,
WAVE_BASE `ad39e4a`. Amend verify/land planning to define how V7/V8 treat a
feature and hard parents whose patches are already materialized in reachable
land commits. Implementation remains Wave C.

**Wave B rev-0** (`d9cc323`): dual NEEDS REVISION. Rev-1 adds landed V10,
strict evidence grammar/cardinality/topology, patch-authoritative
materialization with non-vacuous recipe diagnostics, total parent states,
immutable snapshots and a rebuilt schema/matrix.

**Wave B rev-1** (`f4cabe4`): dual NEEDS REVISION. Rev-2 isolates Anchor C
from worktree dirt, hardens C0, separates landing attestation from replay
anchor selection, makes parent arbitration patch-based, and rebuilds
V10/evidence/schema/matrix parity.

**Wave B rev-2** (`fdc309f`): internal NEEDS REVISION, external APPROVED WITH
NOTES. Rev-3 makes anchor selection exhaustive/forward-applicability-based,
adds per-parent V10 baselines and full metadata snapshots, validates land
Base-Commit, handles shallow history, and normalizes duplicate landings.

**Wave B rev-3** (`f2cd7ae`): internal NEEDS REVISION, external APPROVED WITH
NOTES. Rev-4 fixes parent-tree syntax, uses error-preserving full inventory,
defines producer Base-Commit timing, uses `-C1` anchor qualification,
normalizes hunk positions, and makes partial-clone checks offline.

**Wave B rev-4** (`96cfe2a`): internal NEEDS REVISION, external APPROVED WITH
NOTES. Rev-5 sets the effective Git floor to 2.36, removes unreachable
artifact-state rows, clarifies recovery-before-validation, and completes
11-check cross-document parity.

**Wave B rev-5** (`b6d4f89`): external APPROVED, internal NEEDS REVISION on
stale authoritative prose only. Rev-6 performs the final totality sweep.

**Wave B rev-6** (`c893d66`): substantive contract confirmed; external found
one G5 whitelist self-test defect. Rev-7 is a one-line guard correction.

**Wave B rev-7** (`8412161`): internal APPROVED, external APPROVED. Amendment
1 D8–D19 accepted with 161 executable rows. Terminal planning range:
`ad39e4a..8412161`.

**Wave B close**: ACCEPTED at `b768602`; 8/8 gate PASS.

**Wave C — GH #8 implementation** ✅ ACCEPTED. Dispatch 2026-08-12,
WAVE_BASE `b768602`. Implement ADR-013 Amendment 1 rev-7 and the accepted
161-row verify/land contract. One sequential implementer; no v0.15.1 tag
until dual review and release close.

**Wave C rev-0** (`a4c1f51`..`82f1a0a`): external APPROVED, internal NEEDS
REVISION. Rev-1 closes offline Git bypasses, enforces a single immutable
inventory, propagates history/artifact read errors, emits the reachability
advisory, and rejects whitespace BaseCommit metadata.

**Wave C rev-1** (`24374a9`..`dfeb129`): dual NEEDS REVISION. Rev-2 removes
the remaining recipe live-read, classifies attestation and shadow probe
failures instead of reporting drift/ambiguity, prevents replay after an
unanswerable parent-presence probe, and detects artifact-readability changes.

**Wave C rev-2** (`61ccae1`..`aeef3a4`): external APPROVED WITH NOTES,
internal NEEDS REVISION. Rev-3 replaces the broad locale-dependent apply
diagnostic matcher with an LC_ALL=C, exit-128-only anchored grammar that
cannot convert wrapper, fatal or signalled failures into patch answers.

**Wave C rev-3** (`2348fea`..`8fc7e33`): internal APPROVED, external
APPROVED. The deterministic locale and narrowed apply grammar held under
real-Git and adversarial black-box probes.

**Wave C close**: ACCEPTED at `99adbc9`; 161/161 rows, original GH #8,
real filtered-remote AC-L68/L69, full/race/vet/build and four cross-builds
green. Tagged v0.15.1 at release consolidation; final Wave C gate uses
WAVE_BASE `b768602`.

**Batch close**: GH #7 and GH #8 shipped in v0.15.1. No successor is
dispatched; record a fresh `origin/main` WAVE_BASE before the next wave.

**Post-release review fold** ✅ COMPLETE (2026-08-13; `15560af..64010bf`;
v0.15.1 tag fixed): refreshed the full verify-family contract anchor set,
added a non-vacuous source-bounds guard, and replaced the 161-row acceptance
ledger's raw-text scan with exact Go-package/runnable-test/literal-subtest AST
resolution plus false-positive sensitivity fixtures. Three internal review
passes closed follow-up package/signature/subtest and stale-prose findings.
The nested-repository blind spot in the wave-close source sentinel is
explicitly deferred as [GH #9](https://github.com/tesseracode/tesserapatch/issues/9).

**WP-005 council fold rev-1** ✅ ACCEPTED (2026-08-13; range
`76ed78b..2018fd7`): Turn 3 closed every rev-0 finding; Turn 4 folded LOW
attribution and `--manual` acceptance-boundary notes. Internal APPROVED,
external APPROVED WITH NOTES. Prior turns remain immutable. Next:
artifact-validation/provenance PRD, with prepare-intent-bundle blocked behind
it. No implementation is authorized.

**Artifact-validation/provenance PRD rev-5 + ADR-034 rev-2** ✅ ACCEPTED
(2026-08-13, WAVE_BASE `0aa0d95`, contract tip `cd15165`, close tracking
`cb771ce`,
[GH #10](https://github.com/tesseracode/tesserapatch/issues/10)): dual
APPROVED after five PRD revisions. Accepted contract: 208 AVP rows, 95
repository + 24 stdlib claims, rooted-inspection ADR D1–D18. No implementation
has begun. Gate 8/8 passes; GH #10 is closed. The prepare-intent-bundle PRD is
now unblocked for planning.

**GitHub CI stabilization** ✅ COMPLETE (2026-08-13; `efd96c8`,
`35e8080`): Ubuntu shell shims now emit the unit separator with POSIX octal
`\037`, and test Git subprocesses disable detached auto-maintenance as well as
legacy auto-GC. Actions run
[31733541355](https://github.com/tesseracode/tesserapatch/actions/runs/31733541355)
passes on Ubuntu and macOS. Production code and v0.15.1 tag unchanged.

**Prepare intent-bundle PRD rev-14 + ADR-035 rev-14** ✅ ACCEPTED
(2026-08-14, WAVE_BASE `d060ff4`, writer `409710c`, final writer tip `8f1cc8a`,
errata/close tip `0dd36e6`,
[GH #11](https://github.com/tesseracode/tesserapatch/issues/11)): 234 PIB
rows and 142 claims define Path A missing-only generation, Path B adoption,
archived regeneration, undo recovery and honest T0/T1/T2 publication limits.
Proposed ADR-035 selects the immutable intent archive. Three-way review found
stale-lock ownership, rollback CAS, rooted-write, archive privacy/retention,
provider fallback and compatibility gaps. Rev-1 at `91dea32` adds
`os.Root.Rename`, ADR-027 D3 redaction/removal, 394 PIB rows and ADR D1–D21.
Rev-1 review accepted a bounded follow-up for lock-path unlink safety,
manual-write rooting/CAS, tombstone rehydration, purge index CAS, exact
platform/Git/root guarantees, and doctor residue consistency. Rev-2 landed at
`faf055e` with 409 PIB rows, 165 claims, ADR D1–D21 and CP0–CP13. Rev-2 review
then rejected raw-response retention and the cache-located authority and found
purge/rehydration truth gaps. Rev-3 at `efcddc6` replaces the cache with a
Linux/Darwin workspace-root directory lock, removes raw transcript retention,
and folds the bounded archive/privacy corrections into 432 PIB rows. Rev-3
review retained that architecture but found descriptor-lifetime, purge
recovery, root-rename, dry-run/Git-gate and filesystem-policy contradictions.
Rev-4 landed at `c5f7fd8` with 448 PIB rows. Review retained the architecture
but found unreachable abandon/dangling repair, a path-based journal writer,
dry-run/partial-purge gaps and stale guard/slice wiring. Rev-5 landed at
`eec458c` with 482 PIB rows and 175 claims. Acceptance review found final
recovery/zero-write, pending-journal purge, abandon-gate and flag/retry
ordering contradictions. Rev-6 landed at `7af5092` with 505 PIB rows and 176
claims. Acceptance review found final archive-divergence/abandon-gate and
conditional purge-retry drift. Rev-7 landed at `751d817` with 520 PIB rows.
Acceptance review found a final archive-state classification contradiction
plus preview/ledger drift. Rev-8 landed at `837f28a` with 530 PIB rows.
Acceptance review found a final global-hash residue split plus editorial parity
drift. Rev-9 landed at `ebd1be8` with 536 PIB rows. Acceptance review found a
final global pending-hash invariant and bounded X11 scope/guard drift; rev-10
landed at `a9ad7c0` with 545 PIB rows. Acceptance review found final global
claim/X11 recovery-exception and type-total removal gaps. Rev-11 landed at
`f06c2fd` with 551 PIB rows. Acceptance review found a final inter-class repair
deadlock plus command/state-map parity gaps. Rev-12 landed at `f6bab00` with
560 PIB rows. Acceptance review found a final corrupt-class ordering
contradiction plus stale matrix/broad-route drift. Rev-13 landed at `8f1cc8a`
with 567 PIB rows. Joint acceptance review returned no product finding —
internal **APPROVED WITH ERRATA**, external **NEEDS ERRATA** — against a
record-accuracy set: a rev-13 ledger claim listing `PIB-524` among its amended
matrix rows when the diff touched only that ID's semantic fixture, an
unqualified “corrected throughout” wording claim, two corrupt-object sentences
broad enough to read as classifying **owned** hashes, and a stale “twelfth”
`outcome` ordinal. Rev-14 is the errata-only fold of that set, landed at
`0dd36e6`: still 567 PIB rows, 176 claims and ADR D1–D21, with no product
decision, exit code, state machine or count changed. **Rev-14 joint review
returned dual APPROVED with no findings**, so the PRD and ADR-035 are both
**Accepted 2026-08-14**. Accepted contract: `PIB-001`…`PIB-567`, claims
`C1`…`C176`, ADR decisions D1–D21. Close range `d060ff4..0dd36e6`; no release
tag — this wave shipped documents only. GH #11 is closed. **No implementation
is authorized**: every mutating slice remains blocked on the accepted
`prepare --check` contract being implemented and landed (PRD §17.1/§19(3)),
which is the next dispatch.

**Adjacent-hunk semantic replay + feature absorption/reorder research rev-3** ✅ ACCEPTED
(2026-08-15):
[GH #13](https://github.com/tesseracode/tesserapatch/issues/13) tracks ADR-010
phase-2 fidelity and safe candidate replay before provider resolution;
[GH #15](https://github.com/tesseracode/tesserapatch/issues/15) tracks the
anchored/preimage-complete recipe-generation prerequisite;
[GH #12](https://github.com/tesseracode/tesserapatch/issues/12) tracks
post-upstream/local-baseline retention and compaction tiers; and
[GH #14](https://github.com/tesseracode/tesserapatch/issues/14) tracks
commutation-verified reparent/reorder. The synthetic Go CLI case study
reproduces, under default Git behavior, adjacent additions versus intentional
deletions conflicting under both merge and rebase, while an anchor-based
operation produces a clean candidate. Rev-0 review corrected swapped issue
IDs, unsafe replay assumptions, ADR-010
fidelity, local-versus-upstream absorption, existing unapply/refresh
composition, and load-bearing session-only evidence. Research only; no
command/state/schema implementation authorized. Rev-1 re-review added the
correct delete-first expected tree, SPEC §7 fidelity, all-or-nothing candidate
coverage, GH #15 recipe-generation/preimage prerequisite, tracked applicable-
operation silence, and issue boundary links. `implement-prepare-check` remains
next after this review closes. Rev-2 was approved with notes; rev-3 makes the
Git fixture hermetic and prevents legacy phase-2 conflict short-circuit from
suppressing stronger downstream evidence.
Both rev-3 reviewers approved the research. Issues #12–#15 remain open as
planning backlog; no implementation or architecture decision is authorized.
`implement-prepare-check` is restored as the next task.

**Implement read-only `tpatch prepare --check` rev-4** ✅ ACCEPTED
(2026-08-17; WAVE_BASE `9a8c1d0`;
[GH #16](https://github.com/tesseracode/tesserapatch/issues/16)): implement the
accepted artifact-validation/provenance PRD rev-5 + ADR-034 rev-2, including
the 208-row AVP matrix, rooted bounded reads, nine structural states, full
three-Markdown readiness, constant unknown provenance, exact output/exit
contracts and native Windows coverage. This is the hard prerequisite for every
mutating prepare slice. Rev-0 implementation landed at `0440337`; joint review
returned NEEDS REVISION on status-schema fidelity, the unsupported-platform
open-flags contract, AVP/guard evidence, native Windows tests and pre-change
routing goldens. Rev-1 is dispatched.
Rev-1 implementation landed across `2cbccf6`, `755b31e`, and `b98fac9`.
Internal review approved; external review requires a CI-scoped rev-2: keep
GH #16's native Windows surface blocking, move the unrelated full suite behind
temporary `continue-on-error` owned by
[GH #17](https://github.com/tesseracode/tesserapatch/issues/17), preserve
tagged releases, and tighten narrow guard holes.
Rev-2 landed across `36f23b3`, `69dfe7c`, and `40ae5c2`; CI run
[32093250847](https://github.com/tesseracode/tesserapatch/actions/runs/32093250847)
is green on Ubuntu, macOS and Windows. Joint review is pending.
Rev-2 product behavior is approved; rev-3 closes a nondeterministic pre-existing
land-test failure, job-level/condition/package/release guard vacuity, the last
untracked source scan, and stale Windows inventory wording.
Rev-3 landed at `54ab8b4` with CI result fold `a4748a9`; run
[32097102290](https://github.com/tesseracode/tesserapatch/actions/runs/32097102290)
is green on Ubuntu, macOS and Windows. Final review is pending.
Rev-3 external review approved; internal review found one expression-valued
`continue-on-error` hole. Rev-4 is a CI-guard-only fold.
Rev-4 landed at `9b8efc5` with CI result fold `cacaaf8`; run
[32101270327](https://github.com/tesseracode/tesserapatch/actions/runs/32101270327)
is green on Ubuntu, macOS and Windows.
**Accepted 2026-08-17.** Rev-4 internal review **APPROVED**; rev-4 external
review **APPROVED** with two LOW nonblocking notes on the AVP-175 YAML subset
parser (flow-mapping step form, and the decoy-leaf floor's first-match
selection) — no product finding in either verdict. Implementation range
`9a8c1d0..cacaaf8`: writer `0440337`, coverage/ledger `2cbccf6`, Windows
CI fixes `755b31e`, `36f23b3`, `54ab8b4`, `9b8efc5`, final tip `cacaaf8`.
Evidence: **208** acceptance rows (224 references, zero duplicates), **43**
registered guards each with an executed sensitivity fixture, and **12**
pre-change routing goldens recorded from the `WAVE_BASE` binary. Native
Windows `TestAVPNativeWindows` executes and passes on `windows-latest`. The
unrelated pre-existing full-suite Windows failures remain **visible** behind a
`continue-on-error` step owned by
[GH #17](https://github.com/tesseracode/tesserapatch/issues/17) — a
non-blocking backlog item, not a regression of this wave; AVP-175 fails if that
demotion is ever relocated or made non-literal. **No release tag**: this
prerequisite ships with the later mutating-prepare release. GH #16 is closed.
PRD-artifact-validation-and-provenance rev-6 errata and ADR-034 rev-3 errata
(both Accepted retained) record the three build-tagged `openFlags()` halves.
The §19(3) prerequisite for `PRD-prepare-intent-bundle` rev-14 + ADR-035 rev-14
is now **satisfied**; `implement-prepare-intent-bundle` is the next dispatch.

**copilot-api cumulative verification/migration feedback** ✅ TRIAGED / REVIEWED
(2026-08-18; evidence commit `e6901a2`, range `7206dab..e6901a2`): exact
`tpatch v0.15.1` reproduction at downstream commit `e2d7ce4` confirms 0 pass /
53 fail / 3 skip while typecheck, lint, 352 tests and build pass. A read-only
own-base probe recovers 29 of 38 V8 failures but leaves nine, so isolated
own-base success cannot become a blanket verifier pass. The four recent V10
failures are
`recipe-provenance-unavailable` on Path B recipes, not measured stale hashes;
all 11 non-empty preimages match their recorded bases. [GH #18](https://github.com/tesseracode/tesserapatch/issues/18)
tracks cumulative verify/migration semantics,
[GH #19](https://github.com/tesseracode/tesserapatch/issues/19) manual recipe
provenance,
[GH #20](https://github.com/tesseracode/tesserapatch/issues/20) honest legacy
generation adoption with candidate-base validation,
[GH #21](https://github.com/tesseracode/tesserapatch/issues/21) guarded doctor
schema migrations, and
[GH #22](https://github.com/tesseracode/tesserapatch/issues/22) later-touch
acknowledgement without replay bypass. Research/backlog only; the accepted
`implement-prepare-intent-bundle` queue head is unchanged.

**Mutating prepare intent bundle implementation** 🔨 IN PROGRESS — REV-15 ACCEPTED
(2026-08-18; [GH #23](https://github.com/tesseracode/tesserapatch/issues/23);
WAVE_BASE `3b579fc`): implement the accepted
`PRD-prepare-intent-bundle` rev-15 + ADR-035 rev-15 contract. Pre-change
goldens land first; strict order is S1b directory authority → S1 transaction
core → S3 archive → S4 CLI → S4b retention, with S2 generator extraction
joined before S4, then S5 doctor, S6 public parity and sequential S7
cross-cutting hardening. The acceptance ledger is 567 contiguous PIB rows
partitioned 15/75/42/24/142/17/48/31/173 across S1b/S1/S3/S2/S4/S4b/S5/S6/S7.
Golden review found one impossible rev-14 premise: GH #16 never committed the
standalone check-output paths PIB-391 required to pre-exist in its closed
range. Rev-15 preserves the anti-no-op guarantee by VCS-binding record mode to
GH #16 frozen implementation content `cacaaf8` (production-identical to formal
acceptance `7206dab`) and committing those fixtures before any
mutating production edit. No behavior, ADR decision, matrix row/count or
public surface changes. Review round 0: internal NEEDS REVISION, external
APPROVED WITH NOTES. Round 1 aligned normative ADR-035 D14, the mandatory
revision ledger, current prerequisite truth, permanent re-audit disclosure and
an ordering sensitivity. Joint re-review returned internal APPROVED and
external APPROVED WITH NOTES; the sole terminology note was folded. Rev-15 is
accepted, with no product/decision/count change. No tag before joint
implementation acceptance.

Pre-change golden phase ✅ COMPLETE at `f9208c7` + maintenance-race pin
`977b9d5`: 51 VCS-bound fixtures cover accepted `prepare --check`, all twelve
`next` states, phase Path A/B, never-prepared lifecycle commands, doctor D1–D8
and feature resources. CI
[32178723042](https://github.com/tesseracode/tesserapatch/actions/runs/32178723042)
is green on all three platforms; GH #23's native Windows resource refusal ran
in its own blocking step. S1b is active.

S1b ✅ COMPLETE at `1f35605` (held-root directory authority, exact filesystem
denial, live-path identity, explicit release; rescap unchanged), CI
[32185709105](https://github.com/tesseracode/tesserapatch/actions/runs/32185709105)
green. S1 ✅ COMPLETE at `f0ae54b` (strict journal, rooted durable transaction,
semantic CAS, rollback and terminal recovery), CI
[32202082897](https://github.com/tesseracode/tesserapatch/actions/runs/32202082897)
green. S3 ✅ COMPLETE at `4c3dbfe` (strict deterministic archive schema,
global hash ownership, append planning, rehydration and purge state machine);
CI run
[32220278819](https://github.com/tesseracode/tesserapatch/actions/runs/32220278819)
passed every platform test job. S2 ✅ COMPLETE at `16d614a` (pure generators,
bounded control metadata and in-memory retry with legacy wrapper parity); CI
run
[32229096085](https://github.com/tesseracode/tesserapatch/actions/runs/32229096085)
is green on Ubuntu, macOS and Windows. S4 mutating CLI is in revised review
after closing its first review's transaction-order, authority-snapshot,
abandon-rollback, deadline-classification and report-shape findings. Focused
re-review caught and closed one exit-6 staging-error demotion, then returned
APPROVED. S4 is pushed at `5853ba7`; blocking CI run 32280073787 failed only
the stale pre-S4 AVP-134/AVP-141 tracked-source allowlists after
`prepare_publish.go` became tracked. Their exact S4 importer/root-open
population correction landed at `49301eb`; product behavior is unchanged.
Follow-up CI
[32281269945](https://github.com/tesseracode/tesserapatch/actions/runs/32281269945)
is green on all three platforms. S4 ✅ COMPLETE; S4b retention commands are
active. S4b rev-0 is implemented but returned NEEDS REVISION on pending-preview
precedence, corrupt-object remediation, terminal recovery error handling,
partial repair-class reporting, index-divergence shape, retry headings and
direct safety spies. Rev-1 closes those seven findings; re-review remains
NEEDS REVISION on selector sanitization before terminal recovery, total
dangling-class retries, unindexed corrupt-object list truth, JSON stage shape,
duplicate divergence retry rendering and disconnected journal spies. Rev-2
closes those six; re-review remains NEEDS REVISION on shell-safe destructive
path rendering and restoring workspace/platform/authority precedence ahead of
selector normalization. Rev-3 closes both; final re-review found one
control-character path injection/line-forgery residual in corrupt-object
reporting. Rev-4 closes that final finding with a bounded managed-path
predicate and real control-filename regressions; focused re-review APPROVED.
Tracked-state full validation then found only AVP-134's stale exact importer
set; its sensitivity-preserving S4b correction now passes the full uncached
and CLI race gates. S4b ✅ COMPLETE at `e3099d5`; CI
[32291924127](https://github.com/tesseracode/tesserapatch/actions/runs/32291924127)
is green on Ubuntu, macOS and Windows. S5 doctor D9 rev-0 is implemented but
returned NEEDS REVISION on no-follow workspace reads, non-destructive read
failure handling, one-class/one-route aggregation and the 48-row ledger's
runnable/semantic resolution. Rev-1 folds all four with confined reads,
exact-D9 loader bypass, aggregate findings and a PRD-bound AST ledger;
re-review remains NEEDS REVISION on pending ownership beside unrelated read
failure, three non-covering ledger mappings, resolver false positives, docs
sensitivity and native Windows PIB-222 execution. Rev-2 closes those five;
production D9 is clean, while re-review remains NEEDS REVISION on
scope-aware AST binding, complete PIB-316…322 runtime observables and exact
PIB-143/145 authoritative-surface guards. Rev-3 folds all three and adds the
AVP PRD rev-7 no-decision-change forbidden-source erratum already mandated by
ADR-035/PIB-143. Re-review remains NEEDS REVISION on table reassignment
handling, normative-section binding for PIB-145 and complete
PIB-316/318/320…322 observables. Rev-4 closes those; re-review remains NEEDS
REVISION on alias/range-body table mutation and copied historical heading
blocks in normative-section resolution. Rev-5 closes those; two parser edges
remain for direct range-variable mutation and CommonMark fence/ATX indentation
strictness. Rev-6 closes both; focused re-review APPROVED. Full validation,
then found only a stale AVP-134 D9 importer set and non-hermetic PIB-146
provider config; both corrections now pass the full uncached and CLI/workflow
race gates. S5 ✅ COMPLETE at `f7ccd61`; CI
[32304087548](https://github.com/tesseracode/tesserapatch/actions/runs/32304087548)
is green on Ubuntu, macOS and Windows. S6 rev-0 public parity is implemented
but returned NEEDS REVISION on systemic acceptance-row misattribution,
non-biting totality/sensitivity guards, three public behavior misstatements
and incomplete Unreleased deltas. Its rebuild is blocked on confirmed
production prerequisites: missing §18.1 named seams, missing rename-time
leaf/directory revalidation for PIB-148…151, and undocumented public
`archive-selector-invalid`. One sequential hardening revision is active before
S6 resumes. Prerequisite rev-0 review remains NEEDS REVISION on FEATURES index
expected identity, exact status/control seam roles, purge failure-seam branch
ownership and committed-error after-index observation. Rev-1 folds all four;
re-review remains NEEDS REVISION on descriptor/name temp cleanup coherence,
fail-closed write roles and between-hashes mutation ownership. Rev-2 closes
those three; re-review remains NEEDS REVISION on post-seam same-inode temp
content verification and descriptor cleanup coverage for every supported Linux
architecture. Rev-3 closes those; re-review remains NEEDS REVISION on
pre/post-read mtime/ctime validation and mips64 kernel-stat layout handling.
The coarse-timestamp finding is withdrawn under accepted T1 verification;
two MEDIUM mappings remain for committed raw-preimage and archive-blob
post-publication divergence currently downgraded to exit 5. Rev-5 preserves
both plus manual-status exit 6; focused review APPROVED. Isolated full
validation passes across full/race/vet and all supported build targets. Commit
`b37ba4c` and blocking CI
[32316113750](https://github.com/tesseracode/tesserapatch/actions/runs/32316113750)
are green on Ubuntu, macOS and Windows. The prerequisite is complete; S6 rev-1
ledger/docs rebuild closes row attribution and prose findings, but re-review
remains NEEDS REVISION on eleven AST/runtime/totality/catalog/sensitivity/
golden guard blind spots. Rev-2 folds those with production-derived catalogs,
exact runtime mappings and complete sensitivity fixtures; re-review remains
NEEDS REVISION on descriptor-relative sink inventory, production vocabulary
order, actual refusal fixture evidence and atomic-visibility negation.
Rev-3 closes the surface cases but remains NEEDS REVISION on type-aware sink
dataflow, clause-level atomic polarity, actual vocabulary ordering/constant
resolution, below-classifier catalog states, human rendering and complete
public-emission propagation. The rebuilt guards now pass for all other
rows/catalog entries; one micro-prerequisite remains for raw denied-filesystem
classifier injection without fabricating the final public error. Its guarded internal entry point
S6 rev-4 closes runtime catalog reachability but remains NEEDS REVISION on
concessive-clause polarity, complete wire vocabulary fields, interprocedural
public-emission tracing and receiver/flags/syscall-aware sink taint. Its guarded internal entry point
Rev-5 closes those but remains NEEDS REVISION on whole-function sink
exemptions, program-point/method-value flow, additional concessive forms,
tuple-result guarded fields and callable emitter aliases. Its guarded internal entry point
Rev-6 closes those but remains NEEDS REVISION on reassigned delegate
parameters, function-valued fields/parameters, comma-in-claim polarity,
straight-line flag kills and nested tuple forwarding. Its guarded internal entry point
Rev-7 closes those but remains NEEDS REVISION on closure/loop/fallthrough
state, arbitrary helper return/callable/method-expression flow, callable
factories, named bare tuple returns and quoted prohibition context. Its guarded internal entry point
Rev-8 closes those but remains NEEDS REVISION on defer/branch semantics,
fail-closed convergence, aliased method receivers, fenced prohibition context
and analyzer runtime caching. Its guarded internal entry point
Rev-9 closes those at ~375s but remains NEEDS REVISION on named defers,
goroutine concurrency, range/select bindings and goto label routing. Its guarded internal entry point
Rev-10 closes those but remains NEEDS REVISION on compound deferred-argument
freezing, goroutine pointee alias taint and named/pointer range domains. Its guarded internal entry point
Rev-11 closes those but remains NEEDS REVISION on left-to-right deferred
argument side effects, interface-converted pointer aliases and precise
free-variable write/escape analysis for goroutines. Its guarded internal entry point
Rev-12 closes those but remains NEEDS REVISION on nested goroutine helper
mutation, shared map/slice backing aliases and deferred short-circuit effects. Its guarded internal entry point
Rev-13 closes those but remains NEEDS REVISION on reslice/channel backing
identity, guaranteed append reallocation and recursively nested short-circuit
evaluation. Its guarded internal entry point
Rev-14 closes those but remains NEEDS REVISION on method/callback effect-cache
purity, append over-allocation capacity and deterministic channel FIFO
consumption. Its guarded internal entry point
Rev-15 closes those but remains NEEDS REVISION on ordinary local helper
mutation effects and consumed channel values reappearing after concurrency. Its guarded internal entry point
Rev-16 closes those but remains NEEDS REVISION on transitive callback/helper
writes to package globals without guarded arguments. Its guarded internal entry point
Rev-17 closes that but remains NEEDS REVISION on global pointer/container
aliases, package-init effects and imported global selectors. Its guarded internal entry point
Rev-18 closes those but remains NEEDS REVISION on dependency-ordered frozen
global initializers, synchronous opaque callback effects and real init order. Its guarded internal entry point
Rev-19 closes those but remains NEEDS REVISION on write/escape initializer
dependencies and identity-aware `sync.Once` done-state semantics. Its guarded internal entry point
Rev-20 closes those but remains NEEDS REVISION on Once receiver evaluation,
pointer aliases, aggregate copies and recovered panic control flow. Its guarded internal entry point
Rev-21 closes those but remains NEEDS REVISION on path-sensitive recovery,
Once aggregate channel/defer copies, builtin identity and deferred Once.Do. Its guarded internal entry point
Rev-22 closes those but remains NEEDS REVISION on direct-recover privilege
inside Once callbacks, used-Once copy destinations and range/map-key copies. Its guarded internal entry point
Rev-23 closes those but remains NEEDS REVISION on append copies, zero/partial
copy intervals, key-only ranges and generic union constraints.
Rev-24 closes those but remains NEEDS REVISION on independent destination
identities for copy, zero/in-place append precision and compatible generic
collection union structure.
Rev-25 closes those but remains NEEDS REVISION on pointer-element backing
copies, retained self-append snapshots, generic type arguments and constraint
intersections.
Rev-26 closes those but remains NEEDS REVISION on large pointer-copy modeling,
selector/index self-append snapshots and inferred generic function results.
Rev-27 closes those but remains NEEDS REVISION on generic receiver
substitution, partial explicit inference, variadic parameters and method-only
constraint intersections.
Rev-28 closes those but remains NEEDS REVISION on untyped inference, dependent
constraints, exact-vs-approximate terms and method signatures; custom generic
inference will be replaced with go/types authority.
Rev-29 replaces it with go/types and leaves one MEDIUM mixed-error allowlist
bug in partial type checking.
Rev-30 closes that and leaves one MEDIUM type-graph cache key omission for the
exact partial-error registry.
Rev-31 closes the final cache finding; focused re-review APPROVED. Full
tracked-state validation, commit and blocking CI remain.
The first tracked-state gate then exposed provider-config leakage, duplicate
catalog runtime exceeding the 10-minute package timeout, and an older S5
resolver nil dereference. Integration is closed: TestS6 ~101s, full CLI ~281s,
provider fixtures hermetic and baseline catalogs cached once. Final gate and
all race/vet/cross-build checks pass. S6 ✅ COMPLETE at `65e876a`; CI
[32447914694](https://github.com/tesseracode/tesserapatch/actions/runs/32447914694)
is green on Ubuntu, macOS and Windows. S7 final sequential hardening is active.
S7 rev-0 maps all rows arithmetically but returned NEEDS REVISION systemically:
synthetic G toggles, unrelated ledger wrappers, incomplete CLI/archive/process
scenarios, non-blocking Windows coverage and hard-coded PIB-567 revision
surrogates.
S7 rev-1 completes 14/15 AM rows exactly. PIB-402 exposes stale early
rehydration wording contradicted by later single-owner pending-purge rules and
current production; a no-decision PRD/ADR rev-16 erratum over §9.3 and
PIB-402/403/425 is active before AM closes and AN–AX continue.
S7 rev-2 completes AM–AO: 54/173 rows and 13 G sensitivities. The rev-16
erratum and shared-blob remediation correction are under block review; AP–AX
119 rows remain. Block review returned NEEDS REVISION on aggregate ledger
resolution, five proxy guards, raw-sink observability, real rev-16 diff
derivation, BSD/filesystem CLI truth and executable shared-blob commands.
S7 rev-3 folds those findings and awaits re-review; its reported AN20/AO19
split must be reconciled with the PRD's AN23/AO16 arithmetic before AP starts.
Rev-3 review remains NEEDS REVISION on dead/aggregate ledger assertions,
unbounded rev-16 diffs, incomplete raw-sink observability, three missing BSD
cross-compile lanes and incomplete §7.13 runtime Git scrubbing. The category
split was a report error; PRD and manifest remain correctly AM15/AN23/AO16.
S7 rev-4 closes those findings but review remains NEEDS REVISION on unreachable
parent subtest registrations, temp-file/file-method response sinks outside
cwd, and an unauthorized `GIT_NAMESPACE` scrub beyond §7.13's closed list.
S7 rev-5 review remains NEEDS REVISION on parent-registration dominance and
independent leaves, alias/method-expression create-write-unlink temp sinks, AO
empty-target acceptance/missing PIB-446 sensitivity, and oversized indexed
Git-variable suffixes escaping the exact §7.13 scrub.
S7 rev-6 review remains NEEDS REVISION on static registration false-passes
(infinite-loop/short-circuit/terminator aliases) and intraprocedural raw-sink
provenance across helper/value/interface boundaries.
S7 rev-7 review remains NEEDS REVISION on forgeable test2json registration
events, incomplete descendant timeout cleanup, and omitted `init`/imported-
interface raw-sink reachability.
S7 rev-8 review remains NEEDS REVISION on parent-forgeable shared marker
metadata and legacy/generic sink APIs bypassing the expanded typed graph.
S7 rev-9 marker objection was withdrawn after threat-model adjudication; the
remaining blocker is fail-open non-module-aware type loading that loses
`provider.Provider` interface dispatch. Rev-10 is active.
S7 AM–AO rev-10 is APPROVED: 54/173 exact rows (AM15/AN23/AO16;
I16/C20/G13/U4/S1), checkpointed by `cf324c0` + `fdf86cb`. AP is active;
AQ–AX follow sequentially.
Post-checkpoint CI 32476007227 exposed an invalid Linux-only condition and the
macOS default timeout; correction `9456a52` is green on all three platforms in
32476989232. AP is active.
AP rev-0 is NOT APPROVED: abandon CP9/CP4, dangling regeneration, partial
purge/retry, total exit-3, D9 holder, Git/privacy, descriptor authority,
reference truth and blocking Windows surfaces require exact rev-1 coverage.
AP rev-1 review remains NEEDS REVISION on complete feature-subtree snapshots,
sole-route surface closure, multi-generation regeneration, completion hashes,
all output populations, descriptor dataflow, exact deny-table closure and
semantic origin-claim detection.
AP rev-2 review remains NEEDS REVISION on whole-declaration dangling scans,
the PIB-468 pending divergence window, complete/dataflow-aware output
populations, GOOS-correct exact `Fstatfs` identity, Darwin constant closure and
grammatically bound extraction-claim allowances.
AP rev-3 review remains NEEDS REVISION on closed command acceptance, real
evidence-driven divergence, exact holder/rescap clause allowlists, fail-closed
path/key analysis, target-GOOS `Fstatfs` identity and total classifier shape.
AP rev-4 review remains NEEDS REVISION on concatenated holder clauses,
value-proven field safety, every controlled schema producer, exact builtin fd
conversion/outer-classifier passthrough and the full rescap lock-clause domain.
AP rev-5 review remains NEEDS REVISION on helper-body value proof, every
string-key controlled map, exact `Fstatfs` destination flow and exclusion of
pre-classifier denial returns.
AP rev-6 review remains NEEDS REVISION on both-operand relative proofs,
map-bearing selector assignments, exact GOOS stat transformation and canonical
non-nil error guards.
AP rev-7 review remains NEEDS REVISION on named-enum value proof,
argument/parameter map mutation edges, exact `Fstatfs` error assignment and
canonical syscall-bound error provenance.
AP rev-8 review remains NEEDS REVISION on enum-selector assignment provenance,
standalone unresolved map mutators and indirect/aliased error lvalue writes.
AP rev-9 review remains NEEDS REVISION on map index/range alias propagation
and one-to-one immediate guards for every canonical syscall error definition.
AP rev-10 review remains NEEDS REVISION on sole-write provenance for the
sampled `stat` object between `Fstatfs` and classification.
AP rev-11 review remains NEEDS REVISION on assignment-form range targets that
can overwrite the sampled `stat`.
AP rev-12 is APPROVED: 34 exact rows (PIB-449…482, I9/C12/G9/U2/S2).
S7 cumulative coverage is 88/173; AP is checkpointed at `bf9424f`; AQ awaits
green blocking CI.
Post-checkpoint CI 32520986445 reopened AP: the native Windows dry-run target
is PIB-463 planned/no-mutation, not PIB-461 unsupported refusal, and the
120-second observer inner budget is too short on hosted runners. AQ is blocked.
The bounded mapping/budget correction is APPROVED; AP arithmetic remains
34 rows and AQ now waits only on corrected blocking CI.
Correction `3fd778b` is green on all three platforms in CI 32523687156,
including native PIB-463. AP is durably closed at 88/173 cumulative rows; AQ
(`PIB-483…505`, I13/C3/G7) is active.
AQ rev-0 review remains NEEDS REVISION on exact post-recovery retry state,
multi-hash pending effects, sole recovery returns, undo-CAS preservation,
typed privacy flow, indirect Cobra mutexes, package-wide syscalls and
sentence/cell-bound step semantics.
AQ rev-1 review remains NEEDS REVISION on no-op retry acceptance, aliased
post-recovery gates, all writer sinks, every GOOS source and clause-level step
semantics within table cells.
AQ rev-2 review remains NEEDS REVISION on observed mode-specific retry gates,
recursive local-helper traversal, builtin output sinks, aliased syscalls/Fd and
step-predicate polarity.
AQ rev-3 review remains NEEDS REVISION on production assignments to the
recovery hook and unresolved local-interface dispatch in output renderers.
AQ rev-4 review remains NEEDS REVISION on receiver-flow resolution for
interface implementations outside the fixed loaded-package subset.
AQ rev-5 is APPROVED: 23 exact rows (`PIB-483…505`, I13/C3/G7). Cumulative S7
coverage is 111/173; AR awaits the AQ checkpoint and green CI.
Staged-source validation reopened the checkpoint: AQ's accepted production
change shifted AP's exact `runPreparePublish` inventory hash, and PIB-485
regenerate/CP3 needs deterministic provider success on the unchanged tree.
The correction is APPROVED and AQ is checkpointed at `dc789f6`; cumulative S7
remains 111/173 and AR waits on green blocking CI.
CI 32539554233 is green on Ubuntu/Windows; macOS exhausted the blocking
20-minute package deadline while the AQ observer was progressing. AR remains
blocked on a guarded cumulative-S7 timeout correction.
The exact 40-minute non-Windows / 20-minute Windows timeout correction is
APPROVED; AR waits only on corrected blocking CI.
CI 32540987009 proved the budget but failed PIB-391 because frozen
`avp_guards_test.go` changed. Restore it byte-identical and relocate timeout
guards to S7-owned tests before AR.
Frozen source is restored; review found one S7 parser bypass for
flag-before-`./...` full-suite argv. AR remains blocked on that correction.
Argv parsing is fixed; review found one remaining `bash`/`sh -c` nested-suite
bypass. AR remains blocked on recursive/fail-closed shell handling.
Nested payloads are covered; review found one dynamic shell-executable bypass
(`"$BASH" -c ...`). AR remains blocked on fail-closed executable handling.
Dynamic executables now fail closed and the complete frozen-source timeout
correction is APPROVED; AR waits on the final CI rerun.
CI 32543144792 passed Ubuntu/Windows; macOS exceeded only AQ's 4m inner
observer cap at ~4m10s. AR remains blocked on a bounded AQ-only 8m/12m budget.
AQ/AP tuple checks exist, but review found they are disconnected from actual
call sites. AR remains blocked on a single-source category budget binding.
Category-keyed call sites now derive from the validated AP/AQ budget table and
the correction is APPROVED; AR waits on the final CI rerun.
Correction `e6cabb0` is green on all three platforms in CI 32544950471. AQ is
durably closed at 111/173 cumulative rows; AR (`PIB-506…520`, I7/C4/G4) is
active.
passes isolated full/race/vet and supported/unsupported cross-build gates;
`971da91` and CI
[32328091360](https://github.com/tesseracode/tesserapatch/actions/runs/32328091360)
are green on all three platforms. S6 resumes its final catalog fixture/review.

## Cluster H′ implementation — v0.15.0 typed feature resources + capture adapters ✅ SHIPPED

**Dispatch**: 2026-08-11.
**WAVE_BASE**: `46c984b`.
**Release**: v0.15.0.
**Implementation range**: `46c984b..e0771bf` (12 pre-consolidation
commits).
**Authorities**:
`PRD-feature-resource-claims-and-capture-adapters` +
`ADR-033-resource-capture-boundary` (both Accepted).

One sequential implementer owns the complete shared surface. Rev-0 scope:

- deterministic `resources.json`, immutable capture batches and atomic
  `current.json`;
- ignored-file, logical Git-metadata and Dolt `diff-summary` capture;
- ADR-027 redaction extraction and bounded in-memory scanning;
- Linux/macOS `flock`, filesystem/path gates and trusted private-copy process
  execution;
- `feature resource add|list|remove|clear|trust-dolt|capture|diff`;
- `record --resources`, docs, version notes and acceptance-matrix tests.

Review requires the full 120-clause / 189-row contract, dual independent
verdicts, unchanged guarded WIP and an 8/8 release close. No tag before final
acceptance.

**Rev-0** (`bff5ef5`, `c66845a`): dual **NEEDS REVISION**. Rev-1 is
dispatched for declaration redaction, true bounded output memory, timer
cleanup, strict batch integrity, capability canonicalization, and
mutation-resistant coverage of safety-critical matrix rows.

**Rev-1** (`d82a367`): original findings closed; internal NEEDS REVISION,
external APPROVED WITH NOTES. Rev-2 is dispatched for end-to-end batch
corruption taxonomy and direct noexec/copy-fault/CLI drain/SameFile ledger
coverage.

**Rev-2** (`86f93b7`): internal APPROVED WITH NOTES, external APPROVED.
All production findings closed. Pre-close fold adds Setpgid ledger
attribution and a native double-`WNOWAIT` non-reaping test before v0.15.0
consolidation.

**Close-note confirmation** (`e0771bf`): internal APPROVED, external
APPROVED; production diff empty. Full/race/cross validation clean, 120/120
acceptance clauses and 189/189 matrix rows auditable. Tagged v0.15.0 at
release consolidation.

**Post-release claim review**: APPROVED WITH NOTES. Golden IDs, published
batch identity, privacy, determinism, idempotency, path safety, redaction
refactor parity and 8/8 close were independently reproduced. The sole LOW
finding—duplicated primary reason text in aggregated batch failures—was
folded on `main` without moving the v0.15.0 tag.

**Next**: no successor dispatched; choose from the post-v0.15.0 backlog.

## Cluster H planning — typed feature resources + capture adapters ✅ ACCEPTED

**Registered**: 2026-08-10.
**Accepted**: 2026-08-11.
**WAVE_BASE**: `f04dec7`.
**Reviewed implementation contract**: `f04dec7..650b44f` (32 commits).
**Deliverables**:
`PRD-feature-resource-claims-and-capture-adapters` +
`ADR-033-resource-capture-boundary`.

The accepted v1 boundary adds a separate typed `resources.json` domain for
explicit ignored-file resources, allowlisted logical Git metadata, and a
Dolt `diff-summary` adapter. Resource captures are deterministic structural
sidecar artifacts; they never become canonical patch, lifecycle, reconcile,
land, or verify authority. Raw resource bodies are never persisted, raw
`.git/**` remains forbidden, and Git remains the only replay substrate.

**Accepted command family**:

```text
tpatch feature resource add|list|remove|clear|capture|diff|trust-dolt
tpatch record <slug> --resources
```

**Final contract**: 120 acceptance clauses, 189-row ADR matrix, four
resource-ID vectors, one full batch-ID vector, one directory combined-hash
vector, and six byte-identical PRD/ADR JSON wire blocks. Linux and macOS are
the supported lock/adapter platforms; unsupported hosts fail closed.

**Review close**: rev-13 internal APPROVED WITH NOTES and external APPROVED.
The internal note is an implementation clarification only: after the signal
phase, `cmd.Wait()` may reap before the non-reaping observer reports, so a
post-reap observer `ECHILD` is an expected secondary completion and must not
alter the already-final classification.

**Post-close claim review**: APPROVED WITH NOTES; tracking dates and one
off-by-one citation anchor were corrected before Cluster H′ dispatch.

**Next**: Cluster H′ implementation from these Accepted papers. Record a new
`WAVE_BASE` from `origin/main` immediately before dispatch. No release tag was
created for this planning-only cluster.

## Cluster F planning — 2026-08-05 — v0.13.0 GH #6 first-class rejected feature state (PRD + ADR pair) ✅ SHIPPED

Planning-phase cluster for v0.13.0 GH #6. Data-model extension (not just CLI addition), so planning phase separated from implementation phase. Docs-only. Dual review at rev-0/1/2/3; internal-only confirmation at rev-4. **4 review revs, 8 review turns, 10 commits**.

**Deliverables**:
- `docs/prds/PRD-rejected-feature-state.md` (~1000 lines).
- `docs/adrs/ADR-031-rejected-feature-state-data-model.md` (~1050 lines).

**Two-opinion protocol scoreboard**:
- rev-0 dual: internal BLOCKED (8 findings — architectural traversal caught append-only-audit + confirm-upstreamed-escape-hatch design flaws), external APPROVED WITH NOTES (2 doc-accuracy). Supervisor: NEEDS REVISION.
- rev-1 dual: internal BLOCKED (5), external NEEDS REVISION (3 — empirical convergence with internal on F-INT-1 `post-apply.patch` overwrite + rules-count 5→6). Convergent architectural finding → supervisor reversed rev-0 adjudication, adopted content-hash mechanism.
- rev-2 dual: external APPROVED WITH NOTES (2 LOW "not required for ship", explicit clearance), internal BLOCKED (3 — split-adjudicated: sided with external on F-INT-R2-1 LOW convention, folded 2 completeness gaps).
- rev-3 dual: external APPROVED WITH NOTES (1 LOW cosmetic, reaffirmed clearance), internal NEEDS REVISION (1 MEDIUM — test 26 note-only reopen wording).
- rev-4 internal-only confirmation: APPROVED (test 26/26b locked orthogonal integrity paths; ADR test-count fixed).

**Finding-count convergence**: internal 8→5→3→1→0; external 2→3→2→1→carry.

**Key architectural decisions locked-in**: content-hash evidence (`{path, sha256}` lowercase-hex); post-implementation reject OUT OF SCOPE (deferred to future `PRD-feature-unapply`); exit-code envelope 0/1/2/3 (principle: 2 = pre-mutation validation, 3 = state-machine refusal); CLI shape with mandatory `--note` + optional `--evidence` + explicit `--actor`; actor precedence `--actor` > `TPATCH_ACTOR` > `git config user.email` > `"unknown"`; symmetric dependency invariant (both reject-blocks-if-dependents AND edge-creation-blocks-if-parent-rejected); reopen unbounded append-only with historical-evidence verification on every reopen.

**Range**: `8574ff3..377d103` (planning baseline). Amended `c6aaeb2` (rev-5 verb-collision fold, 2026-08-05, external-only APPROVED WITH NOTES).

**Rev-5 amendment (2026-08-05, docs-only, `c6aaeb2`)**: post-Cluster-F external review flagged `tpatch reject` verb-collision with pre-existing `tpatch reconcile --reject <slug>` flag (`cobra.go:2093`). Alternative 3 disposition: kept bare verbs, added ADR-031 D10 (naming disposition, 3 alternatives with rationale) + PRD §4.1 (intentional non-relationship, 4-point rationale, 5 mitigations) + PRD test 27 (`--help` cross-reference golden-string assertion). External rev-5 confirmation: APPROVED WITH NOTES, 1 LOW F2 residual (§4.1 point 2 imprecise precondition wording for `runReconcileReject`; reviewer explicitly deferred to Cluster F' pickup; non-overlap conclusion unaffected). Decision-point count D1-D10.

**Next**: Cluster F' implementation cluster from planning baseline `c6aaeb2` (includes rev-5 amendment). Touches state enum, status fields, validation (Rule 7), CLI (reject/reopen/status filtering + confirm-upstreamed guard), assets, SPEC.md, 27 tests. Does NOT touch `internal/workflow/reconcile.go` (orthogonal per ADR D6). Cluster F rev-5 F2 residual (§4.1 point 2 wording fix) picked up during Cluster F'.

## v0.13.0 — 2026-08-05 — first-class rejected feature lifecycle state (GH #6) ✅ SHIPPED

Feature release implementing planning baseline from Cluster F. **Cluster F' — 4 review revs (rev-0 → rev-3), 27 commits, range `c6aaeb2..70764a3`.**

**Deliverables** (implementation):
- `internal/store/`: `StateRejected` (11th `FeatureState`); `RejectionStatus` + `RejectionHistoryEntry` + `EvidenceRef` + `DivergenceDetail`; closed 7-value `RejectionReason` enum; `ResolveActor` 4-tier precedence; Rule 7 (`ErrRejectedParent` — rejects edges onto rejected parents); `RefreshFeaturesIndex` renders trailing `## Rejected` table.
- `internal/cli/`: `tpatch reject <slug> --reason --note [--evidence...] [--actor]`, `tpatch reopen <slug> --note [--evidence...] [--actor]` with historical-evidence verification; `status --include-rejected` opt-in filter + rejection-aware DTO; `next` rejection-aware output; `apply`/`reconcile`/`confirm-upstreamed` refuse on rejected; `amend --depends-on`/`feature deps add` refuse rejected-parent edges (exit 3).
- `SPEC.md` + all 6 shipped skill formats + `assets_test.go` parity anchors.
- Tests: PRD §9 27-item matrix (rejection_test.go, reject_test.go); +10 rev-1 regressions; +1 rev-2 dangling-symlink guard. **971 top-level PASS / 0 FAIL** at ship.

**Two-opinion protocol scoreboard**:
| Rev | Internal | External | Adjudication |
|---|---|---|---|
| rev-0 | BLOCKED — 6 findings (1 BLOCKING wire-schema, 3 HIGH, 1 MEDIUM, 1 LOW) | APPROVED WITH NOTES — 3 (1 MEDIUM convergent, 2 LOW) | NEEDS REVISION → rev-1 (internal-strict precedent invoked) |
| rev-1 | APPROVED WITH NOTES — 1 MEDIUM dangling-symlink | APPROVED — 0 findings | NEEDS REVISION → rev-2 |
| rev-2 | APPROVED — clean | APPROVED WITH NOTES — 1 LOW audit-label | NEEDS REVISION → rev-3 (0-residual discipline) |
| rev-3 | APPROVED — clean | APPROVED WITH NOTES — 1 INFORMATIONAL only | **SHIPPED** |

**Finding-count convergence**: internal 6→1→0→0; external 3→0→1→0 (INFO). Every rev closed strictly more than it opened.

**Cross-reviewer catch coverage**: internal caught the wire-schema BLOCKING (`FeatureStatus.RejectionHistory` schema divergence — action discriminator vs completed-cycle pattern, generic `actor` field vs PRD §6 `rejected_by`/`reopened_by`) that external's example-reading missed. External caught the exit-3 golden-string alignment (F-EXT-1), the Oxford comma (F-EXT-2), the audit-label taxonomy (F-EXT-Rev2-1), and the shared-helper reach note (F-EXT-Rev3-1) that internal's specification-focused reads did not surface. Two-opinion protocol continued to pull disjoint findings.

**Key implementation decisions preserved**:
- History schema (rev-1 F-INT-1 fold): `RejectionHistoryEntry` = completed cycle (reject half + reopen half), appended on reopen only. Live `Rejection` set by reject, cleared by reopen. `PriorState` retained as legitimate audit field (not the reopen target).
- Validation ordering (rev-1 F-INT-3 fold): evidence (path resolve + safety + hash) precedes state-machine check → exit 2 wins over exit 3 for combined invalidity.
- Exit 3 mapping (rev-1 F-INT-4/F-EXT-1 fold): `mapDependencyValidationError` at `feature_deps.go` + `amend --depends-on` boundary; PRD §8 golden string byte-for-byte.
- Dangling-symlink guard (rev-2 F-INT-Rev1-1): `os.Lstat` disambiguation on `EvalSymlinks` ENOENT; `DivergentReasonMissing` (rev-3 F-EXT-Rev2-1 refinement).

**Range**: `c6aaeb2..70764a3` (27 commits: 10 rev-0 impl + 8 rev-1 fold + 2 rev-2 fold + 1 rev-3 fold + 6 tracking).

**Rule 18 trailer**: verified on all 27 commits. Side Research md5 preserved: `b385fe622db9926f48861105239f113e`. `make wave-close-check WAVE_BASE=c6aaeb2`: 8/8 PASS at consolidation.

**Tag**: `v0.13.0` at `70764a3` (consolidation commit).

**Backlog for future clusters** (registered 2026-08-05 external user report):
- `prd-verify-post-commit-mode` MEDIUM — V8 `post_apply_patch_replay_clean` misleading remediation on already-committed features.
- `prd-no-upstream-mode` MEDIUM — local-only tpatch mode (sibling PRD).

## Cluster E-prime — 2026-08-05 — post-Cluster-E hygiene follow-up (Obs 1 doc + Obs 2 ALLOWLIST) ✅ SHIPPED

Tiny process-hygiene follow-up cluster closing two LOW observations from external's post-Cluster-E review. Single implementer, external-only rev-0 confirmation (proportionate protocol for cross-wave doc/config refinement on already-shipped mechanism).

- **Obs 1 LOW** (`4ac4743`) — `internal/testutil/gitpin.go` doc comment clarifies that `PinGitAutoGCOff` unconditionally sets `GIT_CONFIG_COUNT=1`, silently discarding any pre-existing `GIT_CONFIG_KEY_N/VALUE_N` env entries. Forward-compat guidance included. Mechanism unchanged.
- **Obs 2 LOW** (`aa34f3c`) — `.wave-close-allowlist` at repo root, 16 initial-seed entries covering current WIP whitepapers/PRDs/case studies. Makefile `[2/8]` gate step subtracts allowlisted entries from WARN list; prints `OK (N entries allowlisted)` when residual empty, `WARN: M untracked files not in allowlist` otherwise. AGENTS.md Wave-Close Checklist synced.

**Deferrals** (backlog, no fold): **E'-N1 LOW** — allowlist stale-entry bitrot is silent. Reviewer explicitly framed as "not required for this rev to ship". Two mitigation options recorded (active sub-check OR AGENTS.md pruning discipline). Fold when allowlist grows beyond initial 16-entry seed.

**Range**: `2281309..aa34f3c` (3 commits).

**Precedent extensions**: (a) external-only rev-0 confirmation validated for **cross-wave** hygiene-scope clusters (prior precedent was intra-wave only); (b) reviewer's "not required to ship" self-classification is now a recognized supervisor deferral signal, preventing the Cluster D "3-iteration on same clause" pattern from being re-invoked by LOW-severity docs on already-shipped mechanisms.

**Structural upshot**: `[2/8]` sentinel is no longer background noise; combined with Cluster E's `[8/8] go test` + cross-package `gc.auto=0` pin, wave-close gate signal is now genuinely actionable — every WARN or FAIL means something. Sets clean floor for Cluster F.

## Cluster E — 2026-08-04 — process housekeeping (F1 gate go test + F2 gc.auto pin) ✅ SHIPPED

Process housekeeping wave before Cluster F (v0.13.0 GH #6) generates high-throughput feature-close cycles. Two findings from external's post-Cluster-D review + 1 rev-1 fold. Single implementer, sequential. **1 review rev + rev-1 fold**.

- **F1 MEDIUM** (`6496d27`) — `Makefile` `wave-close-check` gains `[8/8] go test -count=1 ./...`; renumbered `[N/7]` → `[N/8]`; AGENTS.md Wave-Close Checklist synced. Fixes the "gate PASSes with red suite" blind spot empirically demonstrated at Cluster D HEAD `1bc2a25`.
- **F2 LOW** (`d8c8bb4`) — `gc.auto=0` env pin in `internal/cli/TestMain` via `GIT_CONFIG_COUNT/KEY_0/VALUE_0`. Root cause: unpinned `git commit` forks `git maintenance --auto --detach` background writer that races `t.TempDir()` teardown on `.git/{info,objects}` under `-p 8 -parallel 8` load. Verified via `GIT_TRACE2_EVENT=1`.
- **E-EXT-1 MEDIUM rev-1 fold** (`c1d86e9` + `b294d8c`) — extracted `internal/testutil.PinGitAutoGCOff()` shared helper; `TestMain` added to `internal/gitutil`, `internal/workflow`, `internal/store` (F1's `[8/8]` gates on these packages, so per-package pins are required — each `go test` runs a separate process). `internal/cli` refactored to use the helper. Same canonical-helper pattern as Cluster D rev-1 R1.

**Two-opinion protocol**: rev-0 dual (internal APPROVED, external APPROVED WITH NOTES 1 MEDIUM); rev-1 external-only confirmation (APPROVED WITH NOTES, 2 non-functional commit-message accuracy notes only). Range `1bc2a25..b294d8c` (6 commits: 2 rev-0 impl + 2 rev-1 impl + 2 tracking).

**Structural upshot**: The wave-close gate is now correctness-aware — dogfoods the full suite on every close. Combined with the cross-package `gc.auto=0` pin, gate signal is finally reliable: green means the suite is green.

**Precedent extensions**: (a) small process housekeeping wave dispatched **before** feature cluster is now a recognized cluster shape (Cluster C precursor to v0.12.1; Cluster E precursor to Cluster F). (b) Shared testutil helper pattern for cross-package test-infra fixes (extends Cluster D rev-1 R1 canonical-helper pattern to test infrastructure).

## Cluster D — 2026-08-03 — correctness housekeeping (8 items) ✅ SHIPPED

Correctness-and-docs cluster clearing the v0.12.1 backlog + two external-review folds. Single implementer, sequential per Cluster C rule 5 same-file-overlap discipline. Four review revs.

- **PRD-#3 N2 D10 fallback**: when `patch-generations.json` is absent (pre-ADR-024 features), derive touched_paths from `post-apply.patch` via canonical `gitutil.FilesInPatch` (aligns with manifest generator; rev-1 fix eliminated bespoke-parser divergence class on rename headers).
- **PRD-#3 N3**: dedupe multi-slug D10 migration hint via `migrationHintFired` gate — prints exactly once per run.
- **PRD-#3 S1**: legacy-mode stderr note when patch-id detector is silenced by `--cumulative-legacy`.
- **PRD-#4 F-4**: crash-recovery idempotency guard for `applyConfirmUpstreamedTransition` (append-then-save asymmetry). External Rule 20 verified: reverting the fix produces 3-chain regression; fix produces clean 2-revision repair.
- **GH #5 fast-path follow-up**: docs corrected across three revisions (rev-0 doc addition → rev-1 "present-empty not omitted" → rev-3 verbatim wording on retirement audit emitted by both `reconcile` and `reconcile confirm-upstreamed` payloads, both paths, absent via `omitempty`, not in `status --json`).
- **Wave γ LOW-γr15-N1**: `session summarize --json --write` D6 refusal now emits JSON error envelope to stdout with exit 1; plaintext path preserved for non-`--json`.
- **F1 fold** (post-Cluster-C external): `make wave-close-check` untracked-source glob extended to `docs/whitepapers/*.md` and `docs/state-of-the-art/**`. Surfaced 12 WIP files at close for operator disposition.
- **F2 fold** (post-Cluster-C external): navigational pointer added at `docs/supervisor/LOG.md` cite of rewritten SHAs `2934521`/`6facb68` to the mapping subsection so fresh-clone readers can resolve dangling references.

**Deferrals** (documented, no fold): D-INT-2 (`--from-revision <original>` post-crash "superseded" — PRD-#4 lines 180/259 document flag as CI/test override not recovery path); F-EXT-2 (concurrent invocation of same slug — not supported CLI scenario).

**Commit range**: `4868f68..42f85d7` (13 commits).

**Review protocol**: four revs — rev-0 (dual, split verdict, sided with internal), rev-1 (dual, internal caught first Rule 17 recurrence), rev-2 (external-only, caught second Rule 17 recurrence), rev-3 (external-only, verbatim prescriptive wording broke the pattern, APPROVED).

**Pattern documented**: three consecutive iterations each introduced a fresh Rule 17 residual on the same fast-path help clause. Broken at rev-3 by supervisor-prescribed **verbatim wording** rather than allowing implementer tweaks — precedent for "when a clause misfires ≥ 2 times, ship prescriptive text". Cluster C rev-3 shell fix + v0.12.1 F-INT-3-1 trailer template were the earlier precedents.

**Two-opinion protocol scoreboard**: Internal-only catches: D-INT-1 rename semantics divergence (real correctness gap), D-INT-3-R1 rev-1 audit-via-status false claim. External-only catches: Rule 20 verification of Item 4 idempotency, D-EXT-1 rev-2 "typically review path" false claim. Convergence: rev-0 "OMITS/ABSENT" totality on fast-path JSON. Protocol pulled its weight on both sides.

**Test count**: 907 → 916 (+9 regression tests). Full suite green. Wave-Close Checklist gate PASS at close commit.

## Cluster C — 2026-08-02 — process housekeeping (AGENTS.md + Makefile) ✅ SHIPPED

Docs-and-tooling-only cluster codifying two Cluster A follow-ups and the v0.12.1 parallel-implementer entanglement postmortem:

- **AGENTS.md Parallel-Implementer Discipline addendum**: five rules (glob-shaped stagers forbidden; cluster lead declares shared-surface set; reviewers scope by function name; post-hoc entanglement fixed via `git rebase -i`; **same-file overlap = hard trigger for sequential execution**, not parallel).
- **AGENTS.md Cluster State canonical field**: `**Cluster state**: <TOKEN>` in `docs/handoff/CURRENT.md` with terminal allowlist (SHIPPED / APPROVED / ACCEPTED / IDLE) and mid-cycle denylist. Enables mechanical parseability of the wave-close state.
- **AGENTS.md WAVE_BASE selection recipe**: `git fetch origin && git rev-parse origin/main` before dispatch, record SHA in dispatch brief and CURRENT.md.
- **Makefile `make wave-close-check`**: seven-check mechanical gate (working tree clean, untracked-source WARN, HEAD pushed with fatal fetch-fail, per-commit trailer walk across `WAVE_BASE..HEAD` with invalid-range/empty-range detection, canonical Cluster state field with exactly-one enforcement, gofmt, vet+build). Prints manual-items reminder banner.

Also shipped inline outside cluster (2026-08-02): **CI hygiene fix at `4619b55`** — `gitInitTestRepo` pinned to `-b main`, restoring CI green on `main` after five weeks of failing runs.

**Commit range**: `bb31872..870182d` (5 commits + CI-hygiene commit `4619b55`).

**Review protocol**: four revs — rev-0 (dispatch), rev-1 (external found 3 HIGH + 2 MEDIUM empirical false-passes), rev-2 (external found 1 HIGH duplicate-field parser bug), rev-3 (external found 1 BLOCKING grep-c shell bug), rev-4 (external APPROVED WITH NOTES + wave-close authorized). Internal APPROVED at rev-1 and rev-2; rev-3 and rev-4 were external-only cycles because each addressed a single external-caught empirical bug not benefiting from architectural re-review. Two-opinion protocol scoreboard: **external-only catches on every rev**, validating the adversarial pass earning rent even on a "small" docs/tooling cluster.

## v0.12.1 — 2026-07-31 — correctness fix pass (GH #3 + #4 + #5) ✅ SHIPPED

Post-v0.12.0 bug-fix cluster bundling three independent correctness findings:

- **GH #5 — `tpatch record` transactional round-trip validation.** Pre-fix, a round-trip failure printed a warning but exited 0 and mutated feature metadata. Fixed by hoisting `gitutil.ValidatePatchReverse` above the first `s.WriteArtifact`; on failure without `--lenient`, exit non-zero with a range-mode hint and no mutation. `--lenient` semantics unchanged. Regression: `TestRecordRoundTripFailure_Transactional` + `TestRecordRoundTripFailure_LenientPreserved` + `TestRecordRoundTripFailure_DefaultWorkingTree_Transactional` (rev-1 fold added the default-WT coverage).

- **GH #3 — multi-slug reconcile canonical safety.** Pre-fix, `reconcile A B C` derived incremental patches by subtracting previous cumulative canonicals, which cross-contaminated later features when canonicals were independent (scoped/claims/`tpatch land` era). Fixed per [`PRD-multi-slug-reconcile-canonical-safety`](prds/PRD-multi-slug-reconcile-canonical-safety.md) + [`ADR-030-multi-slug-reconcile-derivation-mode`](adrs/ADR-030-multi-slug-reconcile-derivation-mode.md): default OFF cumulative derivation (each slug uses its canonical `post-apply.patch` as-is); `--cumulative-legacy` opt-in re-enables the legacy path with `--exclude=.git` + residual-stanza filter + store-boundary refusal. `.git/**` never enters a feature patch (D4 diff-boundary + D5 store-boundary, defense-in-depth). D6/D7 flag propagation: DAG reorder (ADR-011 D9) and phase 1.5 canonical-reload (M17 Wave D) both skipped under `--cumulative-legacy`. D10 migration diagnostic emits an actionable hint when the default path hits phase-1 failure with prior-slug `touched_paths` intersection (rev-1 extended to the `reconcileFeature` err-return branch).

- **GH #4 — `tpatch reconcile confirm-upstreamed` gains a human-review path** per [`PRD-confirm-upstreamed-human-review-path`](prds/PRD-confirm-upstreamed-human-review-path.md). See CHANGELOG for the full contract; two-tier reachability (preferred `Reconcile.UpstreamRef` / `@{upstream}`, HEAD-ancestry fall-back with residual-risk warning), 5-row supersession safety matrix, superseding transition revision, retirement audit invocation preserved.

**Cluster arc**: rev-0 dispatch → three parallel implementers → six-reviewer dual pass (2 per PRD/ticket) → seven rev-1 findings folded (1 HIGH paperwork, 1 MEDIUM byte-identity, 1 near-blocking wording, 1 real tie-break correctness bug, 3 LOW hardening) → rev-1 dual confirmation three-way APPROVED.

**Two-opinion protocol scoreboard**: 30/32 across this cluster. Rev-0 external reviewers caught 4 findings internal missed (PRD-#4 F1 warning wording, PRD-#4 F2 tie-break bug, PRD-#3 external N1 D10 gap, GH #5 NB-1 hint mislabel). Internal caught PRD-#3 F-INT-3-1 HIGH trailer parse failure that external didn't. Protocol pulled its weight.

**Cross-implementer entanglement postmortem**: parallel implementers on shared `internal/cli/cobra.go` produced commit `d930963` (labeled PRD-#3 Slice 1+2) that also captured PRD-#4's `reconcileConfirmUpstreamedCmd` changes via a `git commit -a` sweep. Reviewers were briefed to scope by function/helper name, not commit boundary. Deferred to Cluster A's follow-up: parallel implementers should stage via `git add <path>` per-PRD and never `git commit -a` when a worktree hosts multiple concurrent implementers.

- **GH #3 — Wave A — canonical safety** ✅ ACCEPTED (three-way APPROVED rev-1 2026-07-31). Range: `d930963` (Slice 1+2 — cross-contaminated with PRD-#4) → `2bb3532` (Slice 3 `.git/**` guards) → `ba3b3b3` (Slice 4 D10, rewritten from `2934521` for trailer fix) → `84485c9` (Slice 5 status flips, rewritten from `6facb68`) → `9ea680a` (rev-1 N1).
- **GH #4 — Wave B — confirm-upstreamed human review** ✅ ACCEPTED (three-way APPROVED rev-1 2026-07-31). Range: `d930963` production + `52f0f70` tests → `061acea` (rev-1 F-1 + F1 + F2 + F-2).
- **GH #5 — Wave C — record transactional** ✅ ACCEPTED (three-way APPROVED rev-1 2026-07-31). Range: `cebc6b6` → `adb6ba3` (rev-1 NB-1 + NB-2).
- Test count 907 top-level PASS (v0.12.0 baseline 877 + 30 across cluster).

## v0.12.0 — 2026-07-31 — feature supersession + write-file safety + active-feature-session ✅ SHIPPED

Post-planning implementation cluster for GH #1 + ADR-027 F3, executed
as three sequential waves through v0.12.0. Wave γ produced two real
BLOCK-caliber external catches where internal reviewers APPROVED
(rev-0 D6 writer-scope, rev-1 SaveContextSummary ordering) — the
two-opinion protocol did load-bearing work on this cluster.

- **Wave α — schema + labels + reconcile suppression** ✅ (three-way
  APPROVED rev-1 2026-07-29; rev-0 required rev-1 for 4 findings —
  F-SEXT-1 HIGH missing `<slug>` on `superseded-by`, F-SEXT-2 HIGH
  alphabetical vs PRD-locked severity order, F-SEXT-3 MEDIUM multi-
  active-superseder not rejected, Internal F1 MEDIUM stale docs↔runtime
  contradiction)
  - Added `store.DependencyKindSupersedes = "supersedes"` as a third
    valid `depends_on[].kind` literal alongside `hard` and `soft`
    (ADR-011 D1 preserved). Validation, CLI parser, and all six
    shipped skill assets updated in the same commit (Slice 1 anti-drift).
  - Confirmed ADR-011 D2 `DetectCycles` is edge-kind-agnostic by
    construction; added regression tests covering mixed
    hard/soft/supersedes cycles, self-supersession, and reciprocal
    supersession (Slice 2).
  - Added four composable derived labels via ADR-011 D3 pattern:
    `superseded-by <slug>`, `active-superseder`, `stale-superseder`,
    `orphan-superseder`. Labels render in PRD §4.3 severity order in
    `tpatch status` DAG output (text + JSON) and are stripped from
    persisted `Reconcile.Labels` via the shared `stripDerivedLabels`
    helper (Slices 3 + R1 + R2).
  - Reconcile suppression (Slice 4 + R4): `RunReconcile` filters
    superseded-by-healthy-or-stale features from the default effective
    replay set (stale runtime flip per PRD §4.5.3); explicit slug
    reconcile emits a historical-feature warning note. V7
    (`runClosureReplay`) skips superseded hard parents from the closure.
    Orphan supersession does NOT mask the historical target.
  - Multi-active-superseder rejection (Slice R3): `store.ErrMultipleActiveSuperseders`
    fires from `ValidateDependencies` + `ValidateAllFeatures` with
    actionable ADR-020-style messages naming all peer slugs. Real
    production callers at `feature_deps.go` (add/remove), `cobra.go`
    (bulk surface), `verify.go` (V4).
  - Status flipped `Proposed` → `Accepted` on both
    [`PRD-feature-supersession`](prds/PRD-feature-supersession.md) and
    [`ADR-028-supersession-edge-model`](adrs/ADR-028-supersession-edge-model.md).
  - Range: dispatch `7081c62` → rev-0 `48399f4..480f90a` → rev-1 brief
    `d21b4b4` → rev-1 `5e6515d..e5e0091` → rev-1 dual review `763b926`.
    Final code HEAD: `e5e0091`. Test count 129 (baseline 99 at v0.11.3).

- **Wave β — `write-file` recipe safety** ✅ ACCEPTED (three-way concurrent
  APPROVED rev-1 2026-07-30; rev-0 dual review split — internal BLOCKED
  on F-B1/F-B2 vs supervisor-external APPROVED-with-INFO; supervisor
  adjudicated with verbatim ADR-029 D6 + PRD §7.2 text and sided with
  internal reading; rev-1 folded 2 BLOCKING + 1 MEDIUM + 2 LOW plus the
  user-external Wave β verdict 3 additional doc corrections at
  consolidation — F1 MEDIUM `tpatch verify` help stale V0-V9 claim,
  F2 LOW stale "remaining nine" comments, F-INT-β-r1-1 LOW ROADMAP:615
  Slice 3 description)
  - `preimage_hash` + later-touch per
    [`PRD-write-file-recipe-safety`](prds/PRD-write-file-recipe-safety.md)
    and [`ADR-029`](adrs/ADR-029-write-file-recipe-safety.md).
  - Slice 1 — schema field `PreimageHash *string` on `RecipeOperation`
    + `preimage_hash` documented across all 6 shipped skill surfaces +
    new parity anchor in `assets/assets_test.go` (schema + docs in one
    commit; anti-drift lesson from Wave α rev-0 F-SEXT-2).
  - Slice 2 — apply-time preimage precondition precheck with ADR-029
    D3 all-or-nothing semantics; sentinel errors
    `ErrWriteFilePreimageMismatch` + `ErrWriteFileLaterTouch`;
    ADR-029 D1/D2 canonical `sha256:<64 lowercase hex>` byte hash;
    ADR-029 D4 legacy nil-preimage warn-and-proceed; ADR-029 D8
    no-source-body diagnostics; apply CLI surfaces warnings on
    stderr with `⚠` prefix.
  - Slice 3 — path-level later-touch detection using PRD §4.2's
    preferred deterministic artifact (`patch-generations.json.
    touched_paths`) with recipe-op-path fallback; **apply-time
    warn-class per ADR-029 D6 and PRD §7.2** (rev-1 R1 reverted the
    rev-0 refusal-class tightening after supervisor adjudication);
    deterministic slug ordering (PRD §5 note 4).
  - Slice 4 — supersession coupling per PRD-feature-supersession §4.5 /
    ADR-029 D7: superseded features downgrade drift severity from
    Error to Warning-with-note while STILL reporting drift; inherits
    Wave α R4 runtime flip (stale-superseder still downgrades);
    active superseder itself remains hard-reject; path-safety never
    downgraded.
  - Slice 5 — CHANGELOG amendment, PRD-write-file-recipe-safety
    Proposed→Accepted, ADR-029 Proposed→Accepted, ADR README
    status column update.

- **Wave γ — active-feature-session** ✅ ACCEPTED (three-way concurrent APPROVED rev-1.5 2026-07-31; rev-0 dual review split — external BLOCK ×5 Critical+HIGH+MEDIUM vs internal APPROVED-WITH-NOTES ×1 HIGH+3 LOW, zero overlap, supervisor sided external; rev-1 folded all 10 findings; rev-1 dual review split again with external Critical residual F-EXT-γ-1 on SaveContextSummary ordering; rev-1.5 preflighted `EnsureLocalIgnoreContract` at top of `runSessionSummarize` when `opts.Write == true`; user-external parallel verdict APPROVED with F1 LOW unpushed backlog)
  - `tpatch session {start,stop,list,summarize,purge}` command group
    + `.tpatch/local/capture/` local buffer lane per
    [`PRD-active-feature-session`](prds/PRD-active-feature-session.md)
    and [`ADR-027`](adrs/ADR-027-capture-context-privacy-boundary.md).
  - Slice 1 — storage lane + six-mandate refusal contract (PRD §4 D6)
    with detached-worktree fixtures + `git check-ignore` effective
    verification (mandate 5) + `LocalIgnoreRefusal` typed error.
    `tpatch init` amendment adds `.tpatch/local/` to `.gitignore`
    (mandate 1) or refuses with mandate 2 message.
  - Slice 2 — session cobra group + lifecycle (D1/D2/D4/D13/D14) +
    idempotent start (D1.5) + single-active invariant + deterministic
    `session list --json` (D14 + §8.15) + all 6 shipped skill assets
    updated in the same commit (Rule 15 parity guard).
  - Slice 3 — D11 redaction contract (§5 D11 + §8.13): ten forbidden
    content classes, boundary invariant proof (raw bodies never cross
    local→committed), refusal on all-scrubbed (§8.12: prior committed
    summary byte-identical).
  - Slice 4 — record `--with-session` + `--from-session` opt-in
    promotion (§6 D15 + §8.7 + §8.8) + cross-feature isolation
    boundary via `store.LoadSession` slug/manifest agreement check
    (§7 D18).
  - Slice 5 — CHANGELOG amendment,
    `PRD-active-feature-session.md` `Proposed`→`Accepted`, ROADMAP
    status flip to Rev-0 landed.
  - Range: dispatch `561e6de` → rev-0 `7c77723..d842697` → rev-1 brief
    `0cb5382` → rev-1 `3936e99..441428f` → rev-1 internal LOG `8eced6c`
    → rev-1 adjudication `c8b1b0c` → rev-1.5 `274fbb6` → rev-1.5 dual
    LOG `87648a6`. Test count 877 top-level PASS (rev-1.5, +12 net
    over rev-0's 865; +2 net over rev-1's 875).

## v0.12.0 planning artifacts — paper-only PRD/ADR pair landed ✅

Post-v0.11.3 parallel dispatch of three PRDs + two ADRs closing GH
issues #1 and #2's follow-up planning gaps. All at `Proposed` status
awaiting implementation cluster kickoff.

- **Stream A — `PRD-active-feature-session`** ✅ (three-way APPROVED
  2026-07-29, zero adversarial findings)
  - Draft at [`docs/prds/PRD-active-feature-session.md`](prds/PRD-active-feature-session.md).
  - Locks ADR-027 F3 (D1 local-buffer path softness) to Option A
    `.tpatch/local/capture/` with six-mandate ignore-before-write
    refusal contract in §D6 — user-external verified as "correct and
    unusually rigorous reading of the ADR precondition."
  - Defines session lifecycle, storage lane, promotion boundary with
    D3 redaction gate, and a new `tpatch session {start,stop,list,
    summarize,purge}` command group (explicitly declared NEW).
  - 500 lines, 5 clusters as D1-D19, 25 acceptance criteria.
  - Blocks: PRD-agent-event-log, PRD-record-context-summary,
    PRD-ide-capture-hooks, PRD-git-hook-capture-guards,
    ADR-capture-metadata-branch.

- **Stream B — Issue #1 PRD-pair + ADR-pair** ✅ (three-way APPROVED
  2026-07-29, zero adversarial findings)
  - PRD 1: [`PRD-feature-supersession`](prds/PRD-feature-supersession.md)
    (259 lines, 5 clusters, 12 acceptance criteria) — extends ADR-011
    dependency graph with `depends_on[].kind: "supersedes"` third edge
    kind. Preserves ADR-011 D1 storage, D2 cycle detection algorithm,
    D3 composable label pattern, D4 hard/soft semantics.
  - PRD 2: [`PRD-write-file-recipe-safety`](prds/PRD-write-file-recipe-safety.md)
    (233 lines, 4 clusters, 13 acceptance criteria) — adds
    `preimage_hash: <sha256>` field to `write-file` operations
    (v1 mandatory) + later-touch detection (v1 mandatory). Safeguards
    1/4/5 from GH #1 deferred to v1+.
  - ADR-028: [`supersession-edge-model`](adrs/ADR-028-supersession-edge-model.md)
    (D1-D8 lock).
  - ADR-029: [`write-file-recipe-safety`](adrs/ADR-029-write-file-recipe-safety.md)
    (D1-D8 lock; raw `sha256:<hex>` deliberately distinguished from
    record-identity `pg_/re_/rr_<12hex>`).
  - Cross-PRD bidirectional references close the main coherence risk
    of splitting one GH issue into two PRDs.

**Cluster process artifacts** (non-shipping):
- Parallel dispatch of paper-only PRDs across disjoint files worked
  cleanly — no collisions in `docs/handoff/CURRENT.md`, no reorder in
  `docs/adrs/README.md`, no gate regressions. 1,584 insertions, zero
  production code.
- 20 dispatch-brief carry-forward rules still binding.
- 17 consecutive rev cycles at three-way concurrence at final
  acceptance.
- Full per-stream snapshots archived to
  [`docs/handoff/HISTORY.md`](handoff/HISTORY.md).

## M18+ — Future

- Cost tracking and token budgeting
- Multi-repo orchestration
- Web dashboard
- Recipe modernization (`feat-recipe-schema-expansion`, `feat-record-autogen-recipe`)
- Parallel feature workflows

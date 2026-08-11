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

# Dependency Analysis - t3code-upstream-v0.0.23-2026-05

**Status**: Historical qualitative addendum to the tracked t3code study
**Date**: 2026-05 baseline; intake rebaselined 2026-09-22
**Owner**: Core
**Refresh trigger**: Reopen WP-004 planning or correct the underlying t3code
study records.
**Limitations**: Suggestion-oriented analysis only. This file does not prove
automatic hard edges, does not weaken manual confirmation, and does not add new
commutation/runtime evidence beyond the tracked study.

**Study type**: upstream-transition study with post-review dependency and runtime
lessons. For WP-004, this file uses feature/path metadata and local notes as
dependency evidence. For WP-003, the authoritative reconcile-safety findings
remain in `local-notes.md` and `summary.md`.

## Executive summary

The t3code study reinforces WP-004's bias toward explainable suggestions and
against automatic hard edges. It contains several obvious parent/child shapes,
but also shows high false-positive risk from broad domain overlap: many
`copilot-*` features are conceptually related, yet not every shared prefix or
same-area edit should become a hard dependency.

The strongest reusable finding is boundary-related: infrastructure-wide changes
should be separate from feature-specific behavior when their blast radius differs.
The `copilot-skill-controls-ws-rpc` split is the clearest example.

The tables below are qualitative candidate signals for later review. They are
not an accepted dependency graph and do not authorize automatic edge creation.

## Feature boundary quality

| Observed feature or slice | Boundary assessment | Evidence |
|---|---|---|
| `copilot-cli-provider` | Broad foundation feature | Adds provider adapter, driver, settings, contracts, UI registration, and related files. Good root parent, but broad enough to over-attract false hard edges. |
| Zero-file Copilot subfeatures | Boundary smell unless declared as children | Several features were documented as "baked into" `CopilotAdapter` or provider code. A registered feature with no separate patch should either be folded into the parent or carry explicit dependency/state evidence. |
| `copilot-skill-controls` | Mixed feature-specific and infrastructure behavior | The local notes say server-side skill controls touched adapter, registry, WS RPC endpoint, and contracts schema. |
| `copilot-skill-controls-ws-rpc` | Correct split after runtime incident | The WS registration bug affected all WebSocket clients, not only skill toggling, so infrastructure-scoped code deserved its own feature. |
| `effort-theming` | Mostly clean UI slice | It touched UI state and CSS. Same app area is not enough to imply Copilot dependency. |
| `readme-copilot-notice` | Documentation child or soft dependent | Reflects provider availability but should not gate provider behavior. |

## Observed dependency relationships

| Child | Parent candidate | Suggested kind | Evidence |
|---|---|---:|---|
| `copilot-text-generation` | `copilot-cli-provider` | hard | Text generation is a Copilot provider capability; the hunk metadata includes `apps/server/src/textGeneration/CopilotTextGeneration.ts` under the Copilot provider patch. |
| `copilot-turn-timing` | `copilot-cli-provider` | hard | Timing helper path `apps/server/src/provider/Layers/copilotTurnTracking.ts` appears in the Copilot provider patch. |
| `copilot-command-events` | `copilot-cli-provider` | hard or fold-in | Local notes say the behavior was baked into the Copilot adapter event handler. |
| `copilot-resource-events` | `copilot-cli-provider` | hard or fold-in | Same adapter-event evidence as command events. |
| `copilot-skill-discovery` | `copilot-cli-provider` | hard or fold-in | Local notes say it was baked into the adapter `rpc.skills` handler. |
| `copilot-plan-compaction` | `copilot-cli-provider` | hard or fold-in | Local notes say it was baked into system prompt mode handling. |
| `copilot-skill-controls` | `copilot-cli-provider` | hard | Uses adapter, registry, and Copilot SDK skill enable/disable capability. |
| `copilot-skill-controls-ws-rpc` | `copilot-skill-controls` | hard | Contract/WS endpoint exposes skill-control behavior. |
| `copilot-skill-controls-ws-rpc` | WS/contracts infrastructure | hard multi-parent candidate | `WsRpcGroup` registration is infrastructure-wide and affected all WS clients. |
| `copilot-cross-platform-build` / `copilot-dependents` | `copilot-cli-provider` | soft-to-hard | Build artifact changes support Copilot distribution, but path-only evidence is weaker than direct API evidence. |
| `readme-copilot-notice` | `copilot-cli-provider` | soft | Documentation reflects provider availability. |
| `windows-wsl-support` | `desktop-managed-environments-connections` | hard | Feature notes explicitly say it depends on desktop-managed-environments. |

## Missed or questionable dependencies

| Candidate | Assessment | Why |
|---|---|---|
| Zero-file Copilot subfeatures -> `copilot-cli-provider` | likely missed hard edges or bad boundaries | Features with no separate file claims can still carry behavior, but tpatch needs evidence that they are folded into a parent. |
| `copilot-skill-controls-ws-rpc` -> `copilot-skill-controls` | missed hard edge | The WS RPC exposes the skill-control adapter capability. |
| `copilot-skill-controls-ws-rpc` -> WS/contracts infrastructure | legitimate second parent or boundary split | A bug in registration broke global WS setup, proving infrastructure blast radius. |
| `copilot-cross-platform-build` -> `copilot-cli-provider` | questionable hard vs soft | Packaging support may be required for releases, but Tier 0-2 evidence likely supports only a soft suggestion unless created_by or direct path evidence exists. |
| Broad `copilot-cli-provider` parent for every `copilot-*` feature | false-positive risk | Slug/domain locality ranks candidates but should not automatically hard-gate all Copilot-named work. |

## Multi-parent cases

| Child candidate | Parent A evidence | Parent B evidence | Classification |
|---|---|---|---|
| `copilot-skill-controls-ws-rpc` | `copilot-skill-controls`: RPC exists to enable/disable Copilot skills. | WS/contracts infrastructure: missing `WsRpcGroup` registration crashed all WS connections. | Legitimate merge-point shape; emit one suggestion record per parent edge. |
| UI skill toggle work, if added later | Backend API parent: server-side skill-control RPC. | Design/UI parent: settings or component system path. | Legitimate UI-over-backend multi-parent candidate, but not yet present in this study. |
| `copilot-dependents` build support | Provider parent: packages Copilot-related runtime support. | Build-system parent: desktop artifact config path. | Possibly legitimate, but weak from imported metadata alone. |

## Hard, soft, and none classification

| Signal | Recommended kind | False-positive risk |
|---|---|---|
| Parent-created file later edited by child | hard | Low when `created_by`, generation snapshot, or patch path evidence exists. |
| Child exposes a parent capability through a new adapter/RPC path | hard | Low-to-medium; confirm the capability cannot exist without the parent. |
| Infrastructure registration required for a feature endpoint | hard, but as a separate parent edge | Medium; classify per path/op so infrastructure and feature behavior stay distinct. |
| Documentation mentioning a feature | soft | Low; ordering hint only. |
| Same `copilot-*` prefix | soft or none | High if promoted to hard. |
| Same file or same app area without direct API/path creation evidence | none or soft | Medium-to-high; same-area overlap was common. |

## WP-004 impact

| WP-004 point | Finding |
|---|---|
| Tier 0 explicit links | Reinforced. Explicit `depends_on`, `created_by`, and generation snapshots are the only safe automatic hard evidence. |
| Tier 1 deterministic artifacts | Reinforced. Parent-created paths and same claim IDs would have clarified zero-file subfeatures and WS split boundaries. |
| Tier 2 heuristic locality | Useful but risky. Slug/domain/path locality should rank candidates, not hard-gate them. |
| Multi-parent evidence | Strongly reinforced. The WS RPC split needs per-parent evidence, not vague "depends on both" language. |
| New evidence kind | Add `infrastructure-blast-radius` for feature-specific work that changes global runtime infrastructure. |
| New evidence kind | Add `registered-feature-no-patch` for features whose behavior is folded into another patch. |
| First PRD | `PRD-feature-dependency-suggestions` remains the right first PRD; include zero-patch and infrastructure-boundary warnings. |
| Tpatch-Depends-On trailers | Reinforced as a Git projection, not primary truth. |
| Persisted evidence | Strengthened. Later review needed to know raw evidence, review decision, and final state separately. |
| LLM-assisted classification | Strengthened for classification only. It can distinguish "same Copilot area" from direct parent capability use. |
| Static/symbol index | Strengthened for later tiers, especially RPC/adapter/contract references. |

## WP-003 impact

This study remains a reconcile case for WP-003, but the dependency-specific
lesson is narrower: validation evidence should include runtime checks and
feature-boundary repair notes.

| WP-003 point | Finding |
|---|---|
| `validation_refs` | Should include dynamic checks such as dev-server start, WS connect, browser smoke, and environment/config-mode checks. |
| Raw machine evidence vs review decision vs final state | Strongly reinforced by the corrected post-review state and runtime incident. |
| Evidence style | Reinforces `reconcile-evidence.jsonl` and `reconcile-revisions.jsonl`: enum evidence kinds, paths, operation IDs, validation refs, confidence, and follow-up actions. |
| Missing evidence kind | Add runtime validation and infrastructure-boundary notes as validation refs or revision evidence, without storing raw source bodies or transcripts. |

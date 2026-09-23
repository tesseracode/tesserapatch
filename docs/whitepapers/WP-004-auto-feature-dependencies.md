# WP-004 - Auto Feature Dependency Suggestions

**Status**: Approved paper research (2026-06-25; no implementation authorized)
**Authors**: T57
**Started**: 2026-05-26
**Turn log**: [WP-004-auto-feature-dependencies.turns.md](./WP-004-auto-feature-dependencies.turns.md)
**Related**:
- [Feature dependencies](../dependencies.md)
- [ADR-011 Feature Dependency DAG](../adrs/ADR-011-feature-dependencies.md)
- [WP-001 Feature-slice gap](./WP-001-feature-slice-gap.md)
- [WP-002 Capture and metadata foundation](./WP-002-capture-and-metadata-foundation.md)
- [WP-003 Reconcile safety and middle-pass](./WP-003-reconcile-safety-and-middle-pass.md)
- [Active clusters](../CLUSTERS.md)
- [Patch generation manifest boundary](../adrs/ADR-024-patch-generation-manifest-boundary.md)
- [Reconcile evidence and revision schema](../adrs/ADR-025-reconcile-evidence-and-revision-schema.md)

## Intake rebaseline *(O54, 2026-09-22)*

This whitepaper remains **Approved paper research** per Turn 4. That approval
did **not** authorize implementation work or automatic graduation into
`PRD-feature-dependency-suggestions`.

The original May/July research text is preserved as the historical baseline,
but its live-status references have moved on:

- `ADR-025` is now accepted, and `docs/ROADMAP.md` records the full WP-003
  cluster as shipped.
- The sections below still capture useful planning arguments, but not every
  May/July source-line cite was revalidated during this intake.
- If dependency-suggestion planning is reopened, WP-004 is the preferred
  research track to rebase first, but it remains planning-only until a later
  dispatch explicitly opens a PRD.

## 1. Context  *(T57)*

tpatch already has a manual dependency DAG: a child feature declares
`depends_on` edges to parent features, with `hard` and `soft` semantics. The
current user docs are explicit that the graph is "small, hand-curated" and that
"Nothing is auto-inferred" (`docs/dependencies.md:15-17`). That is a safe v1
boundary, but it creates a usability gap when adding new features:

- a new feature may edit files last touched by an existing feature;
- a new feature may edit a hunk near a prior feature without touching the exact
  same lines;
- a new feature may use symbols, paths, or config introduced by another feature;
- an agent may know the relationship from intent, but tpatch only records the
  edge if someone remembers to declare it.

The question for WP-004 is not "can tpatch infer dependencies perfectly?" It
cannot. The tractable question is whether tpatch can suggest likely parent
features using tiered evidence, then let a human or LLM-assisted reviewer
confirm `hard`, `soft`, or `none`.

This paper treats auto-deps as a suggestion problem, not an automatic mutation
problem. Existing `depends_on` remains the source of truth.

## 2. Current architecture surface  *(T57)*

Current tpatch already has most of the primitives needed for safe suggestions:

| Surface | Current role | Why it helps auto-deps |
|---|---|---|
| `status.json:depends_on` | Canonical child-to-parent edge storage (`internal/store/types.go:148-163`, `:216-237`). | Suggestions can target the existing edge model; no new dependency object is needed. |
| `created_by` recipe hints | Apply-time provenance gate for parent-created files (`docs/dependencies.md:114-139`). | Strong per-operation evidence when present; coverage depends on how the feature was implemented or recorded. |
| Topological reconcile/apply | DAG traversal and labels (`docs/dependencies.md:160-200`). | Suggested edges become useful immediately after confirmation. |
| File claims | Advisory feature/path scope metadata (`internal/store/claims.go:1-12`, `:65-83`). | Candidate parents can be filtered by overlapping claimed paths before reading patches. |
| Patch generations | Per-feature patch identity, touched paths, claim IDs, dependency snapshots (`internal/store/patch_generations.go:37-84`). | Candidate parents can be scored by generation-scoped evidence rather than stale historical patches. |
| Parent generation stale overlay | Detects a child captured against an older parent generation (`internal/workflow/parent_generation_stale.go:17-62`). | Proves the system already reasons over parent generation identity after edges exist. |
| Land trailers | Git commit to feature binding via `Tpatch-Feature` (`docs/feature-layout.md:90-94`). | Git history can be mined for feature ownership without inventing a second identity model. |
| Reconcile evidence | Planned `reconcile-evidence.jsonl` with `evidence_kind`, confidence, paths, and refs (`docs/prds/PRD-reconcile-verdict-evidence.md:100-148`). | Auto-dep suggestions should mirror this evidence style for auditability. |

Feasibility finding: auto-deps is feasible as a read-only suggestion layer that
consumes existing artifacts. It should not directly write `depends_on` until a
user or explicitly configured agent confirms the edge.

`created_by` coverage is not universal. Recipes generated through
`tpatch implement` can carry these hints, and manually authored recipes can add
them, but Path-B recorded features may have no `created_by` data at all. The
Tier 1 `parent-created-path` signal remains valid when it exists; absence of
`created_by` is not evidence that no dependency exists.

## 3. State of the art  *(T57)*

### 3.1 Explicit identity and trailers

Git trailers are standardized enough to parse and generate machine-readable
footers in commit messages. `git interpret-trailers` parses footer-style lines
such as `Signed-off-by`, supports custom keys, and can add or reposition
trailers. Gerrit uses a `Change-Id` footer to keep a stable review identity
across amend, rebase, and cherry-pick: the Change-Id is independent of the
commit ID and must stay in the commit-message footer.

Implication for tpatch: explicit links are still the best evidence. tpatch
already has `Tpatch-Feature` trailers and stable feature slugs. A future
`Tpatch-Depends-On: <slug>` trailer could help Git-projected workflows, but it
should be a projection of `status.json:depends_on`, not a new source of truth.

### 3.2 Build graph and affected-set tools

Bazel query treats build targets as a graph/DAG and supports graph questions
such as "why does this tree depend on that target?" with `somepath(...)`.
Nx `affected` combines Git file changes with a project graph: Git tells it which
files changed, the project graph maps files to projects, and Nx then finds
projects that depend on those changed projects.

Implication for tpatch: the directly analogous operation is:

```text
changed feature paths -> owning/claiming features -> dependency candidates
```

This favors a graph query/report first, not an LLM first.

### 3.3 Task systems and declared dependencies

Build systems such as Gradle model independent units of work as tasks and use
task dependencies to order execution. Their lesson is boring but useful:
dependency declarations remain explicit even when tools can inspect inputs and
outputs. Inferred relationships help with planning and diagnostics; declared
edges drive execution.

Implication for tpatch: `depends_on` should remain explicit. Suggestions should
explain "why this edge looks likely" and require confirmation.

### 3.4 Mining software repositories

Mining-software-repository tools such as PyDriller extract commits,
modifications, diffs, and source-code metadata from Git repositories. This is
the state-of-the-art family behind co-change and logical-coupling analysis:
files that change together repeatedly often share a hidden dependency even if no
static import edge exists.

Implication for tpatch: Git history can provide a weak but useful signal:
features that frequently touch the same files, adjacent hunks, or the same
landed commits may be candidates for `soft` edges.

Privacy boundary: co-change scoring should store only paths, feature slugs,
commit SHAs, patch/generation IDs, hashes, counts, and derived scores. It must
not persist commit message bodies, raw hunk text, raw source bodies, prompt
text, transcripts, vectors, or embeddings. This mirrors the no-source-bodies
boundary used by WP-002 and WP-003.

### 3.5 Static and code-property graphs

Code property graph systems such as Joern combine syntax, control-flow, and
data-flow representations into queryable graphs. This style of analysis can
find "B calls symbol introduced by A" more precisely than file overlap.

Implication for tpatch: symbol/static dependency inference is valuable, but it
is a later tier. It would require language/tool adapters or external indexers.
The v1 architecture should reserve evidence kinds for it without making it a
dependency of the core Go binary.

### 3.6 LLM-assisted classification

LLMs are useful when deterministic and heuristic signals disagree: for example,
two features touch the same file but different conceptual concerns, or a child
uses a parent-introduced concept without matching exact hunks. The LLM should
receive summaries and evidence, not raw source by default, matching the privacy
boundary already used by WP-002 and WP-003.

Implication for tpatch: LLM assistance is a classifier over candidate edges, not
the candidate generator of first resort.

## 4. Proposed tiered suggestion model  *(T57)*

WP-004 proposes a four-tier suggestion ladder. Each tier emits candidate edges
with evidence, confidence, and a recommended kind (`hard`, `soft`, or `none`).

| Tier | Name | Evidence | Typical recommendation |
|---|---|---|---|
| 0 | Explicit links | Existing `depends_on`, `created_by`, generation dependency snapshots, future Git trailers. | Confirm existing edge, or repair stale metadata. |
| 1 | Deterministic artifact overlap | Same claim ID, same touched path, child patch modifies a path parent creates, child generation captured against parent generation. | `hard` when child depends on parent-created path; otherwise review. |
| 2 | Heuristic locality | Last feature to touch path, nearby hunk overlap, path-prefix/domain overlap, co-change history, shared landed commit window. | Usually `soft`; `hard` only with strong path-created-by-parent evidence. |
| 3 | Static graph | Imports, calls, config references, generated API contracts, build graph edges, code property graph paths. | `hard` for direct symbol use; `soft` for broad project-level dependency. |
| 4 | LLM-assisted review | Feature request/spec summaries, evidence bundle, path/hunk/symbol facts, no raw source by default. | Classify `hard`/`soft`/`none` with rationale and confidence. |

Tier 0 and Tier 1 are feasible with current or shipped tpatch artifacts. Tier 2
is mostly Git/path math and should stay provider-free. Tier 3 needs optional
language/build adapters. Tier 4 is an opt-in assistant over the evidence from
Tiers 0-3.

### 4.1 Multi-parent suggestions  *(T57)*

Multi-parent dependencies are supported by today's graph, but they are a
modeling-clarity problem, not just a graph-support problem. A child with two or
more hard parents is a merge point. Apply and reconcile can linearize the order
deterministically, but the graph does not semantically resolve parent-parent
conflicts for the user.

Suggestion output must therefore keep each candidate parent edge explainable on
its own. `created_by` is per-operation and names exactly one parent; one recipe
operation can point at `parent-A` or `parent-B`, not both. When a candidate child
has strong evidence for multiple parents from different files or operations, the
suggestion should say which path/op points at which parent instead of collapsing
the evidence into a vague "depends on both" claim.

Legitimate multi-parent shapes include:

1. integration features that require two independent bases;
2. adapters between two feature slices;
3. UI work that depends on both a backend API and a design-system parent;
4. patch-theory-style sequencing where a child should reconcile only after all
   hard parents settle.

The default suggestion bias should still avoid false hard edges. Multi-parent
hard suggestions need per-edge evidence strong enough that each parent would be
reasonable even if reviewed independently.

## 5. Candidate scoring shape  *(T57)*

A future suggestion artifact could look like:

```json
{
  "schema_version": 1,
  "child": "copilot-skill-controls-ws-rpc",
  "candidate_parent": "copilot-skill-controls",
  "co_suggested_with": ["copilot-skill-controls-ws-contracts"],
  "suggested_kind": "hard",
  "confidence": "high",
  "tiers": ["deterministic-artifact-overlap", "llm-assisted-review"],
  "evidence": [
    {
      "kind": "parent-created-path",
      "path": "packages/contracts/src/rpc.ts",
      "parent_generation_id": "pg_abc123...",
      "linked_op_id": "op-7"
    },
    {
      "kind": "shared-domain",
      "value": "ws-rpc"
    }
  ],
  "action": "suggest-add-dep"
}
```

This is intentionally close to WP-003's evidence vocabulary. It should be
append-only or reproducible from current artifacts, privacy-preserving, and
safe to ignore. The write path should be:

```text
suggest -> explain -> confirm -> write status.json:depends_on
```

Never:

```text
suggest -> silently mutate depends_on
```

Multi-parent extension decision: use one suggestion record per
`(child, candidate_parent)` edge, not one combined record for a parent set. When
the engine surfaces several parents together, include `co_suggested_with` so the
UI can present the merge-point shape. Evidence entries may include
`linked_op_id`, `path`, or `claim_id` to show which operation or file supports
that specific parent. This keeps review and confirmation aligned with the
existing `depends_on[]` edge model.

## 6. Tag and ID system analysis  *(T57)*

The architecture does not need a new primary tag system before using what now
exists:

- stable feature identity is the slug;
- moving patch identity is the patch generation (`generation_id`,
  `patch_sha256`, `git_patch_id`);
- path/scope identity is the claim ID;
- Git-projected identity is `Tpatch-Feature`.

A new tag system would be useful only for two narrow cases:

1. **Git projection of dependencies**: a `Tpatch-Depends-On: <slug>` trailer can
   help reviewers who only inspect Git commits. It should mirror `depends_on`.
2. **Semantic feature roles**: optional labels like `provides: ws-rpc` or
   `requires: ws-rpc` could improve suggestions, but they need a privacy and
   taxonomy PRD. They are not required for v1 auto-dep suggestions.

## 7. Integration with active clusters *(historical baseline; see intake note above)*  *(T57)*

This section preserves the original planning-time snapshot from the paper's
May/July research pass. Use `docs/ROADMAP.md`, `docs/CLUSTERS.md`, and
`ADR-025` for current live status.

### WP-002 dependency

WP-002 is shipped. That means auto-deps can rely on file claims and patch
generations as first-party artifacts. The most important field is the generation
dependency snapshot in `patch-generations.json`: it records which parent
generation and parent patch hash a child was captured against. Auto-deps should
reuse that concept instead of inventing `since` syntax.

### WP-003 dependency

WP-003 is accepted and gated on ADR-025. Auto-deps should reuse WP-003's
evidence style: enum evidence kinds, confidence, matched paths/operations, no
raw source bodies, and append-friendly review records. If WP-003 ships first,
auto-dep suggestions can use the same evidence/revision machinery.

### Existing dependency DAG

ADR-011 already decides that dependencies live in `status.json`, that hard deps
gate apply, and that labels are derived overlays. Auto-dep suggestions should
not add new dependency kinds. They should suggest existing `hard` or `soft`
edges, plus `none` when the evidence is weak.

### Cluster dashboard

`docs/CLUSTERS.md` currently lists WP-002 as shipped and WP-003 as accepted,
blocked only on ADR-025. WP-004 should not be added to `CLUSTERS.md` until the
broker/supervisor opens it as an active cluster; the file says it is maintained
by the broker/supervisor.

## 8. Candidate graduation path *(not currently opened)*  *(T57)*

This remains a candidate path only. The 2026-09-22 intake explicitly keeps
WP-004 at paper-research status and does not open any PRD from this list.

If reviewers agree with this framing, WP-004 could graduate into these PRDs:

1. `PRD-feature-dependency-suggestions`
   - Read-only command: `tpatch feature deps suggest <slug> [--json]`.
   - Tiers 0-2 only.
   - Outputs candidates, confidence, evidence, and recommended kind.
2. `PRD-feature-dependency-suggestion-evidence`
   - Optional artifact for persisted suggestions if recomputation is too slow.
   - Aligns with ADR-025 evidence style.
3. `PRD-feature-dependency-confirmation-flow`
   - Confirm/reject suggestions and write `status.json:depends_on`.
   - Keeps mutation separate from scoring.
4. `PRD-git-dependency-trailers`
   - Optional `Tpatch-Depends-On` projection in landed commits.
   - Mirrors status, never primary truth.
5. `PRD-static-feature-dependency-index`
   - Optional language/build adapters for imports, symbols, and generated API
     contracts.
6. `PRD-llm-feature-dependency-classifier`
   - Opt-in LLM review over summarized evidence, after privacy boundary review.

Recommended first slice: PRD 1 only. It can be provider-free, deterministic
where possible, and immediately useful as a warning/report.

## 9. Case-study findings: TWS new-project build  *(T57)*

The TWS new-project case study
(`docs/state-of-the-art/case-studies/tws-dev-2026-05/`) reinforces WP-004
rather than opening a new whitepaper. It is not a reconcile study, so it should
not change WP-003's
`upstreamed` or `blocked` analysis. Its value is earlier in the lifecycle:
dependency signals appeared during define/explore, before implementation and
record.

Findings to carry into a future `PRD-feature-dependency-suggestions` draft:

1. Suggestions should run during `define` and `explore`, not only at `record`.
   The case study found useful parent candidates before patch bytes existed.
2. Same-patch or same-commit overlap is caution evidence, not hard-dependency
   evidence. Bundled commits can make unrelated features look coupled.
3. Boundary-warning evidence kinds are useful: `infrastructure-blast-radius` for
   changes that affect shared runtime surfaces, and `registered-feature-no-patch`
   for features present in metadata but not yet represented by patch bytes.
4. Per-edge multi-parent evidence remains the right shape. Multi-parent cases
   should preserve which path/op supports each parent.
5. The hard-edge threshold should stay high: explicit links, parent-created
   paths, direct capability/API use, or phase-confirmed prerequisites.
6. Runtime smoke checks belong as validation refs shared with WP-003 evidence
   machinery. They do not justify a separate whitepaper yet.

This does not change the candidate graduation path in section 8. It narrows the
first PRD: `tpatch feature deps suggest <slug>` should be usable before record
when only request/spec/exploration, claims, and known feature metadata exist.

## 10. Case-study findings: orphaned feature commit binding  *(T57)*

The `divergent-stack-sync` repair case adds a later-lifecycle provenance failure
that complements the earlier TWS "pre-record suggestion" case. The feature's
`status.json:apply.base_commit` pointed at unreachable SHA `6ccb5da` because the
original feature commit was created on a temporary smoke-test branch,
cherry-picked to main as `5c11e12`, and then the temporary branch was deleted.

tpatch correctly surfaced the broken binding:

```text
dependent-broken: feature "divergent-stack-sync" references SHA(s) 6ccb5da which are no longer reachable
```

The manual repair path was:

```bash
tpatch feature deps divergent-stack-sync add archive-worktree:hard

tpatch record divergent-stack-sync \
  --from 5c11e12^ \
  --to 5c11e12 \
  --files internal/stack_test.go \
  --regenerate-recipe

tpatch feature deps --validate-all
tpatch verify divergent-stack-sync
```

Final verify passed. The important finding is not a new dependency data-model
gap: existing `depends_on`, scoped `record`, recipe regeneration, DAG validation,
and verify were sufficient to repair the feature. The gap is provenance
assistance and boundary repair guidance.

Findings to carry into future diagnostics:

1. Normal Git recovery can orphan tpatch commit bindings. A cherry-pick from a
   temporary branch to main preserves the source change but not the original
   commit reachability. When the temporary branch is deleted,
   `status.apply.base_commit` can point at an unreachable SHA even though an
   equivalent reachable commit exists.
2. tpatch can detect unreachable SHAs today, but it cannot yet suggest an
   equivalent reachable commit. In this case, `5c11e12` was the reachable
   replacement for unreachable `6ccb5da`.
3. Git-parent replay and dependency-closure replay are different checks. A
   record roundtrip against the captured Git parent can succeed while
   `tpatch verify` fails against the declared dependency closure if a parent edge
   is missing.
4. Boundary repair and dependency repair can be coupled. Some files in a broad
   patch may be outside the true feature boundary, while other hunks require
   adding missing parent context before closure replay can pass.
5. Existing commands repaired the feature, but the diagnostic path was too
   manual: the operator had to infer the equivalent commit, discover the missing
   parent, narrow the file boundary, regenerate the recipe, validate the DAG, and
   rerun verify.

Candidate evidence kinds for future suggestion/diagnostic artifacts:

| Evidence kind | Meaning |
|---|---|
| `unreachable-binding-equivalent` | Unreachable SHA appears equivalent to a reachable commit by patch-id, touched paths, subject, or trailers. |
| `closure-replay-mismatch` | Patch applies to the captured Git parent but fails against the declared dependency closure. |
| `missing-parent-context` | Failed context first appears in another tpatch feature. |
| `feature-boundary-overreach` | Some files in the patch likely belong to unrelated features. |
| `candidate-rebind-commit` | Candidate reachable commit for metadata repair. |

This case does not create PRDs yet, but it suggests a future graduation path
adjacent to `PRD-feature-dependency-suggestions`:

1. `PRD-feature-boundary-diagnostics`
   - Read-only diagnostic.
   - Compare Git-parent replay vs dependency-closure replay.
   - Suggest parent edges and `--files` narrowing when evidence supports them.
2. `PRD-feature-rebind`
   - Metadata repair for unreachable commit bindings when a high-confidence
     equivalent reachable commit is found.
   - Should require patch-id/touched-path/trailer evidence and should not rewrite
     source files.
3. `PRD-record-closure-verify-hint`
   - Record output warns that Git-base roundtrip was checked but
     dependency-closure replay was not.
   - Points users to `tpatch verify <slug>` and dependency diagnostics when a
     feature has declared or likely parents.

## 11. Experiment plan  *(T57)*

Before implementation, collect at least two studies:

1. **Historical dependency study**: take a repo with known `depends_on` edges and
   ask whether tiers 0-2 would have suggested the same parents before the edges
   were declared. Seed corpus: the t3code upstream-transition study at
   `docs/state-of-the-art/case-studies/t3code-upstream-v0.0.23-2026-05/`,
   because it already contains known feature relationships, false-positive
   upstreamed verdicts, and post-review dependency cleanup notes.
2. **New-feature registration study**: during feature creation/recording, log
   suggested parents and ask a reviewer to label `hard`, `soft`, or `none`.
   This needs a separate corpus once auto-dep suggestions are running anywhere.

Metrics:

- top-1 and top-3 parent recall;
- false-positive rate for `hard` suggestions;
- hard vs soft classification accuracy;
- evidence coverage by tier;
- number of suggestions rejected as same-file-but-unrelated.

The highest-risk failure is a false `hard` dependency that gates unrelated work.
Therefore v1 should bias toward `soft` unless parent-created-path or symbol-use
evidence is strong.

## 12. Open questions  *(T57)*

1. Should suggestions run at `tpatch add`, `tpatch record`, or both?
2. Should suggestions ever inspect request/spec prose by default, or only with a
   privacy opt-in?
3. Should rejected suggestions be remembered to avoid nagging, and if so where?
4. Should `Tpatch-Depends-On` trailers be generated by `land`, or left out until
   users prove Git-only review needs them?
5. How should auto-deps treat broad foundation features that many later features
   touch but do not logically depend on?

## 13. Disputes  *(T57)*

None yet. This is the first research turn.

## 14. References  *(T57)*

- Git `interpret-trailers`: machine-readable commit footer parsing and
  generation.
- Gerrit Change-Id documentation: stable review identity across amend, rebase,
  and cherry-pick.
- Bazel query language: dependency graph queries such as `deps(...)` and
  `somepath(...)`.
- Nx affected docs: Git-file deltas plus project graph to find impacted
  projects.
- Gradle task docs: declared task dependencies as execution-order authority.
- PyDriller: Git repository mining for commits, modifications, diffs, and
  source metadata.
- Joern: code property graph analysis across syntax/control/data-flow graphs.

# WP-004 Auto Feature Dependency Suggestions - Turns

## Turn 1 - T57 - 2026-05-26

**Responding to**: human prompt
**Type**: proposal

Initial paper pass. I read the current dependency docs, ADR-011, WP-002,
WP-003, CLUSTERS.md, feature layout, and the shipped store surfaces for claims,
patch generations, dependency snapshots, and parent-generation-stale labels.

Finding: auto-deps is feasible as a suggestion layer over existing artifacts,
not as automatic dependency mutation. The existing architecture already has the
write target (`status.json:depends_on`), scope evidence (`claims.json`), patch
identity (`patch-generations.json`), Git projection (`Tpatch-Feature`), and
future evidence style (`reconcile-evidence.jsonl`). The proposed graduation path
therefore starts with `tpatch feature deps suggest <slug>` and keeps
confirmation separate.

External prior art anchors reviewed:

- Git trailers and Gerrit Change-Id for stable explicit identities.
- Bazel query and Nx affected for graph + changed-file impact analysis.
- Gradle task dependencies for explicit execution authority.
- PyDriller/MSR for Git-history and co-change mining.
- Joern/code-property graphs for later static/symbol tiers.

**Asks of next agent**:
- Challenge whether Tier 1/Tier 2 can be implemented without storing a new
  suggestion artifact.
- Check whether `Tpatch-Depends-On` trailers are worth a PRD or should stay
  deferred until Git-only reviewers ask for them.
- Stress-test false-positive hard dependency risk against one known tpatch repo.

## Turn 2 - T57 - 2026-05-26

**Responding to**: broker review
**Type**: revision

Applied the broker's small findings while keeping WP-004 in Exploring status.
F1 softened the `created_by` framing: it is strong evidence only when present,
and coverage depends on implementation/capture path. F2 added the co-change
privacy boundary: store paths, SHAs, IDs, hashes, counts, and scores, never raw
commit messages, hunk text, source bodies, transcripts, vectors, or embeddings.
F3 named the t3code v0.0.23 case study as the seed corpus for the historical
dependency experiment and noted that new-feature registration needs a separate
future corpus.

Added section 4.1 on multi-parent suggestions. The main finding is that multi-parent
is not a graph-support problem; the current DAG can hold multiple parents. It is
a modeling-clarity problem because a multi-parent child is a merge point and the
graph's deterministic order does not semantically resolve parent-parent
conflicts. The section also records the `created_by` constraint: per-operation
provenance names one parent, so multi-parent suggestions must explain which
path/op supports each parent.

Decision on section 5 extension: added a lightweight extension rather than deferring.
The model stays one suggestion record per `(child, candidate_parent)` edge, with
optional `co_suggested_with` for grouped presentation and `evidence[].linked_op_id`
for per-operation provenance. This keeps confirmation aligned with the existing
`depends_on[]` edge model and avoids a new parent-set object.

New risk surfaced: false multi-parent hard suggestions can over-constrain the
feature graph even when each individual same-file/locality signal is plausible.
Future experiments should measure false-positive hard edges separately for
single-parent and multi-parent suggestions.

## Turn 3 - T57 - 2026-05-26

**Responding to**: TWS case-study summary
**Type**: proposal

Added a short case-study findings section for the TWS new-project build. The
study is not a reconcile case, so it should not change WP-003's `upstreamed` or
`blocked` findings. It does sharpen WP-004's first-PRD scope: suggestions should
be useful during `define` and `explore`, before patch bytes exist, because TWS
surfaced dependencies before implementation.

Recorded six findings: run suggestions before record; treat bundled
same-patch/same-commit overlap as caution evidence; add boundary-warning
evidence kinds (`infrastructure-blast-radius`, `registered-feature-no-patch`);
preserve per-edge multi-parent evidence; keep hard-edge thresholds high; and
model runtime smoke checks as validation refs shared with WP-003 rather than a
new whitepaper.

Decision: no new WP and no PRDs now. This stays in WP-004 as evidence for a
future `PRD-feature-dependency-suggestions` draft.

## Turn 4 - human broker - 2026-06-25

**Responding to**: CO47 review of Turn 3
**Type**: decision

Approved WP-004 as paper research after the TWS case-study citation was added
to §9. This approval does not authorize implementation, CLUSTERS.md routing, or
PRD drafting yet. The paper remains a research artifact until the broker or
supervisor explicitly graduates it into `PRD-feature-dependency-suggestions` or
another follow-up.

## Turn 5 - T57 - 2026-07-10

**Responding to**: orphaned feature commit binding retrospective
**Type**: case-study-addendum

Added the `divergent-stack-sync` case-study findings. Key conclusion: this is
not a new dependency data model issue; it is provenance assistance and boundary
repair. Existing commands repaired the feature, but tpatch could guide the
operator better by suggesting equivalent reachable commits, missing parents, and
over-broad file boundaries.

## Turn 6 - O54 - 2026-09-22

**Responding to**: documentation-only research intake rebaseline
**Type**: review

Rebased the paper after the v0.17.0 research intake without graduating it.
Turn 4's broker approval still stands as **paper research only**: no
implementation dispatch and no automatic PRD creation were authorized here.

The main current-state correction is external to the original May/July text:
`ADR-025` is now accepted and `docs/ROADMAP.md` records the full WP-003
cluster as shipped, so WP-004's cluster-integration section must be read as a
historical planning baseline rather than a live blocker report. The paper still
supports a later dependency-suggestions planning pass, but only as a preferred
research seed. The recommended first slice remains provider-free / deterministic
Tiers 0-2 with a high bar for `hard` suggestions and no automatic graph
mutation.

This turn does **not** open `PRD-feature-dependency-suggestions` or any other
follow-up. If planning is reopened later, revalidate the May/July source-line
anchors against the then-current contracts first.

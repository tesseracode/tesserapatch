# PRD - Overlap verification and consolidation for independently landed features

**Status**: Draft (not accepted; planning only — no implementation is authorized by this document)
**Date**: 2026-11-05
**Owner**: Core
**Issue**: [GH #25](https://github.com/tesseracode/tesserapatch/issues/25)
**Related**: [ADR-013](../adrs/ADR-013-verify-freshness-overlay.md) (Amendment 1, D11–D14), [ADR-011](../adrs/ADR-011-feature-dependencies.md) (D4), [ADR-019](../adrs/ADR-019-tpatch-land-trailer-block-schema.md), [ADR-024](../adrs/ADR-024-patch-generation-manifest-boundary.md) (D3–D5), [ADR-028](../adrs/ADR-028-supersession-edge-model.md) (D6–D8), [ADR-029](../adrs/ADR-029-write-file-recipe-safety.md) (D5),
[SPEC.md](../../SPEC.md) §"Landed-feature verification", [CLAUDE.md](../../CLAUDE.md) (Key Design Decisions), [docs/land.md](../land.md), [docs/dependencies.md](../dependencies.md), [docs/ROADMAP.md](../ROADMAP.md) (GH #25/#26 rows), [docs/handoff/HISTORY.md](../handoff/HISTORY.md) (GH9-D13); GH #8, #14, #18, #19, #22, #24, #26.

---

## 1. Summary

A parent feature creates a file. A hard-dependent child feature later
rewrites that file. Both land with exact landing evidence. After the child
lands, `tpatch verify <parent>` exits 2 with
`failed_at: landed-content-absent`. The parent's historical replay passes.
Only the current-anchor half of `post_apply_patch_replay_clean` fails,
because the parent's exact postimage is no longer at `HEAD`.

Verify is telling the truth: the parent's postimage is gone. But it cannot
say why. It does not look at descendants, so it cannot tell "a recorded
child changed this" from "someone deleted this". Its remediation (R1) asks
the operator to re-record and re-land the parent. The documented
consolidation alternative does not work either: a metadata-only landing
under a new slug never gets a same-slug replay anchor.

This PRD sets out three options and their combinations: **A**
ordered-stack-aware verification, **B** a supported consolidation workflow
and **C** a read-only overlap assessment. It gives a drafter's
recommendation and the questions the operator must answer. It does not
authorize implementation.

## 2. Problem statement

### 2.1 Minimal reproduction (from GH #25)

Parent `linux-filter` creates `src/runtime.js` with an explicit-empty
creation preimage. Child `version-filter` has a hard edge to
`linux-filter` and rewrites `src/runtime.js`; its preimage is the SHA-256
of the parent's bytes. Both follow the same lifecycle: add, manual
analyze/define/explore/implement, apply done,
`record --staged --regenerate-recipe`, then `land --no-record`. Before the
child lands, parent verify exits 0. After the child lands, parent verify
exits 2 at `landed-content-absent`. Landing evidence is `exact`;
`recipe_replay_clean` and `write_file_preimage_fresh` pass;
`post_apply_patch_replay_clean` fails. The child exits 0, and a behavior
check on the product passes.

### 2.2 Why the parent fails (verified code path)

1. Landed features take `landedPhase`
   (`internal/workflow/verify_anchored.go:122-124`).
2. `landedPhase` runs the anchor-C ladder on the parent's canonical
   `post-apply.patch` at `HEAD` (`internal/workflow/verify_anchored.go:205`).
3. Step 1 is a plain reverse `git apply --check` in a temporary index
   (`internal/workflow/verify_landed.go:1089-1093`). The child replaced the
   parent's bytes, so the reverse check fails.
4. Step 2 is `-C0 --verbose` (`internal/workflow/verify_landed.go:1102-1110`).
   If it fails, or if it passes only with `(0/0)` hunks, the result is
   `absent` and `Blocked` (`internal/workflow/verify_landed.go:1122-1138`).
5. The historical half runs separately at the parent's same-slug replay
   anchor (`internal/workflow/verify_anchored.go:264`). The issue reports
   that the historical replay remains valid.
6. `foldAnchorC` sees `ladder.Blocked`, sets V8 `passed=false`, attaches R1
   (or R2 for `(0/0)`), and sets `failed_at` to `landed-content-absent`
   (`internal/workflow/verify_anchored.go:299-315`).
7. V8 has block severity, so the verdict is `failed` with exit 2
   (`internal/workflow/verify.go:630-643`).

Nothing in steps 1–6 looks at the child. The only dependency traversal on
this path is the hard-parent closure BFS, which walks from the target
toward its ancestors (`internal/workflow/verify_anchored.go:376-401`).

### 2.3 Downstream case and the Fusion stack

GH #25 reports two larger cases.

- **Dependency closure.** A dependency-closure feature reused a package
  resolver that an earlier pruning feature had created. The pruning
  feature's new-file postimage became absent. Readiness verification hit a
  match that would require all context to be discarded, which is the
  `(0/0)` block (`docs/adrs/ADR-013-verify-freshness-overlay.md:623-629`).
  The team chose a packaging superseder rather than relaxing checks.
- **Fusion stack** (issue comment): provider → packaging → SDK → Fusion.
  The descendant's historical closure fails at `parent-landing-drift`.
  Each landed hard ancestor is ladder-checked at the target's fixed anchor
  tree (`internal/workflow/verify_anchored.go:684`). A blocked ladder fails
  fast with R14 (`internal/workflow/verify_anchored.go:694-696`). The SDK
  feature had legitimately evolved the provider's source, so the provider's
  postimage was absent at that tree. A manual isolated-index proof
  reversed the four patches and then forward-applied them in dependency
  order, and the result matched the landing tree exactly. That proves the
  composition. It is not a native pass.

### 2.4 Consolidation migration complication

The issue reports that capturing the complete existing source range under
a new slug and landing only its metadata is not enough. (The documented
composable form of a range capture is `record --commit-range` then
`land --no-record`, `docs/land.md:19-25`.) Landing evidence becomes
`exact` and anchor C is clean, but historical replay fails. This matches
the code: the anchor resolver collects only single-parent commits whose
single `Tpatch-Feature` equals the slug
(`internal/workflow/verify_landed.go:1222-1236`) and requires the
candidate's parent tree to accept a forward apply
(`internal/workflow/verify_landed.go:1242-1258`). A metadata-only landing
already has the product in its parent tree, so it cannot qualify. No
qualifier is terminal: `historical-anchor-unavailable` with R11
(`internal/workflow/verify_anchored.go:232-253`), and the run never passes
on anchor C alone (`docs/adrs/ADR-013-verify-freshness-overlay.md:765-769`).

The recovery that did work was a genuine replay under the new slug:
(1) retain the original artifacts and landings; (2) create an isolated
worktree at the true pre-stack baseline; (3) execute the consolidated
recipe there and check exact product-tree equality; (4) record and land
the real replay under the new slug; (5) merge that landing into the
maintained branch without changing its tree or rewriting history;
(6) verify, then enable supersession. This is correct, but today's
remediation text does not lead an operator to it.

### 2.5 Status on v0.18.0

GH #25 still reproduces on v0.18.0 (ROADMAP row `docs/ROADMAP.md:46`).
The recorded v0.18.0 run (`docs/handoff/HISTORY.md:17761-17767`) matches
the issue: before the child lands, the parent exits 0. After the child
lands, the parent exits 2 with `failed_at: landed-content-absent`. Landing
evidence is `exact`; `recipe_replay_clean` and `write_file_preimage_fresh`
pass; `post_apply_patch_replay_clean` fails. The child exits 0, and the
behavior check passes. Both features now record with
`recipe coverage: complete`, and that does not change the parent result
(`docs/handoff/HISTORY.md:17768-17769`). The Fusion fixture still exits 2
with `parent-landing-drift` on the provider feature
(`docs/handoff/HISTORY.md:17770-17772`).

## 3. Current behavior

### 3.1 What each landed-path check proves

| Check | Where | Proves | Does not prove |
|---|---|---|---|
| Landing evidence | `classifyEvidence` (`internal/workflow/verify_landed.go:712`, digests `:843-858`, `exact` `:869-870`) | Exactly one reachable single-parent commit carries this slug and matches the current patch SHA, recipe SHA and `status.apply.base_commit`. | That the content is still in the tree. A revert keeps the landing reachable (`docs/adrs/ADR-013-verify-freshness-overlay.md:439`). |
| V7 `recipe_replay_clean` (historical) | `internal/workflow/verify_anchored.go:516` | The recipe replays at the replay anchor's parent tree, after closure arbitration. | Anything about `HEAD`. |
| V8 historical half | `internal/workflow/verify_anchored.go:547-556` | The canonical patch applies forward at the closure baseline. | Anything about `HEAD`. |
| V8 current half (anchor C) | `internal/workflow/verify_anchored.go:205`, ladder `internal/workflow/verify_landed.go:1078-1145` | The canonical patch's postimage is present at `HEAD`, either clean or with context drift that keeps at least one context line. | Why it is absent when it is absent. |
| V10 `write_file_preimage_fresh` | `evaluateV10` (`internal/workflow/verify_anchored.go:788-870`) | Each `write-file` preimage matches at the reference tree. | Overwrite safety at `HEAD`. |

Anchor C reads a temporary index seeded from `HEAD`. It does not read the
working tree or the real index (`SPEC.md:345-348`;
`docs/adrs/ADR-013-verify-freshness-overlay.md:597`). The ladder is
hardened: a `(0/0)` match blocks. This was measured at 0 false greens in 69
absent trees, at a cost of 26 false reds in 151 present trees
(`docs/adrs/ADR-013-verify-freshness-overlay.md:623-635`).

### 3.2 The same-slug landing anchor

`Tpatch-Feature` is the sole feature-to-commit binding (`docs/land.md:151`;
`docs/adrs/ADR-019-tpatch-land-trailer-block-schema.md:21-24`), and the
four trailer keys are fixed (`internal/gitutil/trailers.go:244-249`).
Replay-anchor candidates are collected exhaustively from exact-slug,
single-parent trailer commits (`internal/workflow/verify_landed.go:1222-1240`).
A candidate qualifies only if its parent tree accepts a forward `-C1` apply
of the current canonical patch (`internal/workflow/verify_landed.go:1242-1258`,
`:1322-1345`). If several qualify, their normalized identities must agree
before one is selected (`internal/workflow/verify_landed.go:1260-1295`).
**No broadening:** the resolver never falls back to "any commit that
introduced these paths" (`docs/adrs/ADR-013-verify-freshness-overlay.md:753-754`).
Enumeration never uses `--first-parent`, so a landing reachable only
through a merge's non-first parent is still found
(`internal/gitutil/trailers.go:284-289`).

### 3.3 Hard dependencies in verify

Hard edges gate `apply` and `created_by`; soft edges gate neither
(`docs/adrs/ADR-011-feature-dependencies.md:47-55`). The apply gate checks
parent **state** only (applied, active or upstream-merged;
`internal/workflow/dependency_gate.go:84-109`), and it is a no-op when the
DAG is disabled (`internal/workflow/dependency_gate.go:64-66`). In verify,
hard edges are read in one direction only: they build the target's
ancestor closure for V7/V8 (`internal/workflow/verify_anchored.go:380-401`;
`docs/dependencies.md:189-199`). Landed closure members are ladder-checked
at the target's anchor tree and never replayed
(`internal/workflow/verify_anchored.go:680-721`). No verify step uses a hard
edge to find **descendants** of the target.

### 3.4 Supersession in verify

`supersedes` preserves history and excludes the superseded feature from
default replay (`docs/adrs/ADR-028-supersession-edge-model.md:77-79`). It
does not change hard/soft semantics
(`docs/adrs/ADR-028-supersession-edge-model.md:81-83`). A superseded member
is skipped when another feature's closure is built
(`internal/workflow/verify_anchored.go:389-391`). For the superseded target
itself, only V10's severity drops to warn
(`internal/workflow/verify_anchored.go:796-800`); `foldAnchorC` does not
consult supersession (`internal/workflow/verify_anchored.go:274-321`). So
superseding the parent does not turn the parent's own verify green.
`isFeatureSupersededIn` returns true for healthy **and** stale superseders
(`internal/workflow/labels.go:436-472`).

### 3.5 Later-touch advisory

The later-touch index orders features by `RequestedAt` and indexes their
touched paths (`internal/workflow/verify_landed.go:1411-1440`). It is a
timestamp-and-path signal, not a transformation. The advisory is
warn-class (R13, `internal/workflow/verify_landed.go:1499-1501`; emitted at
`internal/workflow/verify_anchored.go:864-867`). ADR-029 D5 calls path-level
detection "not a total stack-safety proof"
(`docs/adrs/ADR-029-write-file-recipe-safety.md:60-66`).

### 3.6 Remediation text today

| Code | Text summary | Cite |
|---|---|---|
| R1 | Postimage absent at `HEAD`; re-record and re-land; do not reconcile. | `internal/workflow/verify_landed.go:1451-1453` |
| R2 | Matched only with all context discarded; re-record and re-land. | `internal/workflow/verify_landed.go:1455-1457` |
| R11 | No usable landing baseline; re-record and re-land to create a fresh single-parent landing. | `internal/workflow/verify_landed.go:1491-1493` |
| R14 | Hard parent landed but absent at the verification baseline; verify the parent first. | `internal/workflow/verify_landed.go:1503-1505` |

For GH #25, R1 asks the parent to re-record and re-land. The bytes now at
`HEAD` were produced by the child's recorded patch. Following R1 would make
the parent's patch describe the child's change. R14 sends the operator to
the parent, which already fails for the same reason.

## 4. Could existing primitives do this?

| Primitive | What it carries | Why it cannot resolve GH #25 alone |
|---|---|---|
| Hard edge (`depends_on`, kind `hard`) | Child needs parent applied. Gates apply and `created_by` (`docs/adrs/ADR-011-feature-dependencies.md:47-55`). | It records intent, not a transformation. It lives in mutable `status.json`, not in landing evidence. GH #22 and GH #25 both state that a hard edge does not prove overwrite safety. |
| Soft edge | Ordering hint (`docs/dependencies.md:37-40`). | Not load-bearing by definition. |
| `supersedes` edge | Exclusion from default replay (`docs/adrs/ADR-028-supersession-edge-model.md:77-79`). | Does not change the superseded target's own anchor-C result (§3.4). Returns true for stale superseders too. |
| Four landing-attestation trailers | Slug ↔ commit binding with patch, recipe and base digests (`docs/adrs/ADR-019-tpatch-land-trailer-block-schema.md:7-24`), written by `land` (`internal/cli/land.go:443-447`). | They bind one feature to one commit. They say nothing about later commits. The block is locked; `Tpatch-CVE` is the only reserved additive trailer (`docs/adrs/ADR-019-tpatch-land-trailer-block-schema.md:50-54`). |
| Generation manifest dependency snapshot | Child's `parent_generation`, `parent_patch_sha256` and `parent_recipe_sha256` at record time (`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:157-187`; `internal/store/patch_generations.go:69-77`). | Binds the child to a parent version. It is not consulted by verify for descendant explanation. Absent for legacy features, and no backfill is allowed (`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:129-134`). |
| Recipe provenance / coverage | Recipe base and coverage completeness (inventory snapshot, `internal/workflow/verify_landed.go:414-418`). | Describes the feature's own recipe. "Complete" coverage did not change the result on v0.18.0 (§2.5). |
| Later-touch advisory | Later feature touched a path (§3.5). | `RequestedAt` and path only. Warn-class. Not proof. |
| Re-record / re-land (R1, R11) | New attestation and, when the parent tree qualifies, a new replay anchor (`docs/adrs/ADR-013-verify-freshness-overlay.md:771-775`). | For the parent it would misattribute the child's bytes. For a metadata-only consolidation the new landing's parent does not qualify (§2.4). |
| `record --commit-range` + `land --no-record` | Capture a range, then land existing artifacts (`docs/land.md:19-25`). | Produces the metadata-only landing in §2.4. Metadata-only landing onto `main` is out of scope in v1 (`docs/land.md:258`). |
| `feature patch refresh` / `fixup` | Append a patch generation from a fresh capture (`internal/cli/feature_patch.go:89-99`). | The capture is index-to-worktree `git diff` (`internal/gitutil/gitutil.go:341-347`). After landing with a clean tree it is empty, so refresh skips and fixup errors. It cannot create a landing anchor. |
| `reconcile confirm-upstreamed` / `upstream_merged` | Upstream absorption; closure skips the member (`internal/cli/cobra.go:2718-2743`; `internal/workflow/verify_anchored.go:654-655`). | Using it for local descendant evolution would record a false upstream claim. |
| Context-drift ladder outcome | Postimage present with drifted context; passes with an advisory (`internal/workflow/verify_landed.go:1140-1144`; `internal/workflow/verify_anchored.go:316-320`). | Only applies when the postimage bytes survive. Here they do not. |

Conclusion: no existing primitive proves that a recorded, ordered
descendant transformation explains the missing postimage. The closest
ingredients are exact landing evidence for each feature, the generation
dependency snapshot and the temp-index ladder. None of them is wired to
look forward from the target.

## 5. Goals and non-goals

Goals: (1) decide whether overlapping, independently landed features are
supported as an ordered stack or must be consolidated; (2) give the
operator a truthful, read-only explanation of why a landed postimage is
absent; (3) keep every negative case in §9.2 red; (4) preserve D12 `(0/0)`
blocking, D14 exact-slug anchoring, the ADR-019 trailer schema and
ADR-011/ADR-028 edge semantics, unless an ADR amendment changes them on
purpose.

Non-goals: behavioral or semantic verification; hunk-level or path
ownership models; automatic consolidation or any history rewrite; changes
to reconcile.

## 6. Options

### 6.1 Option A — Ordered-stack-aware verification

**Idea.** When the target's own anchor-C ladder blocks, try to explain the
absence by undoing the target's recorded descendants, in their real landing
order, inside a temporary index.

**Proposed procedure (sketch, for decision only).**

1. Trigger only on the `ladder.Blocked` branch of `foldAnchorC`
   (`internal/workflow/verify_anchored.go:299`). A terminal evidence state
   is never rescued, and neither are absent artifacts or an unavailable
   historical anchor.
2. Build the candidate set E of explaining descendants. Every member must
   meet all of these eligibility rules:
   - **E1.** Its landing evidence is `exact`.
   - **E2.** It touches at least one path the target's patch touches.
   - **E3.** It reaches the target through a chain of hard edges.
   - **E4.** It has a generation dependency snapshot for its direct parent
     in that chain that matches that parent's current canonical patch SHA
     (§4).
   - **E5.** Its attestation commit is reachable from `HEAD` and descends
     from the target's attestation commit.
   - **E6.** Every block-severity check in its own verify passes in the
     **same run** as the target: landing evidence, V7, both V8 halves,
     V10 and its hard-parent closure. "Same run" means the same `HEAD`
     commit (A resolves it once and uses that SHA everywhere), the same
     single immutable inventory and the same single `git log` enumeration
     (`internal/workflow/verify_landed.go:18-26`). The run's instability
     re-statement (`internal/workflow/verify_landed.go:1652-1672`) must
     cover the descendant's artifacts too. A persisted or earlier verify
     result is never used. In A1 the descendant must pass natively. From
     A2 on, a descendant's own anchor-C result may be explained only by
     strictly later descendants, so the recursion follows ancestry order
     and never loops back to the target.
3. Order E by commit ancestry of the attestation commits. Do not use
   `RequestedAt`. If ancestry does not give a total order on the
   overlapping paths, stop (unexplained).
4. In a temp index seeded from `HEAD`, reverse-apply each member of E in
   reverse landing order. Each step must pass ladder step 1 (plain
   reverse, full context). Step 2 or `(0/0)` is not accepted.
5. At the resulting tree, the target's own ladder step 1 must pass.
6. Forward-apply E in landing order from that tree. The touched-path
   entries of the result must be byte-equal to `HEAD`.
7. On success, the V8 current half passes with a new anchor result (name
   to be decided). A mandatory warn advisory lists the explaining slugs and
   attestation commits in order. The historical half must still pass on
   its own.

**What it proves.** Two things. The target's recorded postimage existed.
And every change to it since then is a recorded, exact-evidence,
generation-bound transformation by a named descendant, applied in real
landing order, whose composition equals the current tree on the touched
paths.

**Trust boundary.** Read-only. It uses only the temp index (D11), the
immutable inventory and Git objects. It takes no operator input. It writes
nothing except what verify already persists.

**Must never be sufficient on its own:** a hard edge; exact descendant
evidence without the composition; a descendant verify result that was
persisted earlier or computed against a different `HEAD` or inventory; a
historical-anchor pass; a later-touch advisory; a `supersedes` edge; any
operator acknowledgement, bound or unbound; a `(0/0)` match at any step.

**Effect on locked decisions.** ADR-013 needs an amendment: anchor C's
meaning widens from "present at `HEAD`" to "present at `HEAD`, or
explained by recorded descendants", and the advisory vocabulary is a
closed set (`docs/adrs/ADR-013-verify-freshness-overlay.md:637-640`). D12
is kept, because the composition uses ladder step 1 only. D14 is kept,
because anchor collection does not change. ADR-019 is kept: no new trailer
key and no new writer of the landing-attestation trailers. ADR-011 D4 and
ADR-028 D7 are kept, because the hard edge is
only a candidate filter. ADR-029 D5 is kept: later-touch stays
warning-class and is not evidence. No CLAUDE.md key design decision is
affected (`CLAUDE.md:81-91`), but skill assets that describe verify output
must stay in parity (`CLAUDE.md:91`).

**Slice plan.** A0: ADR-013 Amendment 2 draft (semantics, vocabulary,
JSON shape). A1: target-level, single child, linear ancestry (the minimal
case). A2: chains of descendants. A3: member arbitration at historical
anchors (Fusion); this changes the D13 table
(`docs/adrs/ADR-013-verify-freshness-overlay.md:645-670`) and must be
coordinated with GH #26.

**Risks.**

- A composition bug could produce a false green. This is the most serious
  risk. Mitigation: the §9.2 negative matrix, plus a measured corpus like
  D12's, before acceptance.
- Merge topologies may not give a total order. v1 refuses them.
- If a descendant is re-recorded but not re-landed, the binding breaks and
  the target turns red. This is expected, but it is a new coupling.
- Extra temp-index work runs on blocked runs only. Verify complexity grows.

### 6.2 Option B — Supported consolidation workflow

**Idea.** Make the issue's six-step recovery (§2.4) the documented, audited
route. A new slug S gets a genuine source landing whose parent is the true
pre-stack baseline. Verify needs no change.

**What tpatch would check (read-only pre-flight, proposal):** the
originals' artifacts and landing commits are unchanged; the baseline
commit is an ancestor of `HEAD`; S's recipe was regenerated from the
combined state rather than reused from snapshot-specific parent recipes
(the Fusion comment warns that old whole-file recipes could overwrite
later exports); executing S's recipe in an isolated worktree at the
baseline gives a product tree equal to `HEAD`'s; and `supersedes` edges
from S are added only after `tpatch verify S` passes.

**What it proves.** S's canonical patch genuinely transforms the baseline
into the integrated product tree. S's landing is a real source landing. Its
parent accepts a forward apply, so D14 qualifies it without change
(§3.2). A merge whose tree equals the maintained branch makes it reachable
(`internal/gitutil/trailers.go:288-289`).

**Trust boundary.** All existing checks are unchanged. The pre-flight is
read-only. S's artifacts, generation entry, recipe sidecars and landing
commit come only from the normal `record` and `land` paths on S (§9.3 M1,
M2). The merge is an ordinary Git merge with no history rewrite. The
originals are retained.

**Must never be sufficient:** a metadata-only landing (it stays
`historical-anchor-unavailable`); a reachable base that accepts the patch
but has no same-slug landing; a `supersedes` edge alone; an operator claim
of tree equality without the executed replay.

**Effect on locked decisions.** None. It is consistent with Pattern A,
where `land` is feature-branch only (`docs/land.md:252-258`).

**Open consequence.** After consolidation, `tpatch verify <original>` still
exits 2. Supersession does not touch anchor C (§3.4). Either that is
accepted, or a superseded target's anchor-C absence is downgraded to warn
when a healthy, verified superseder exists. That downgrade would **extend**
ADR-028 D8, not apply it. D8 today makes only superseded historical recipe
drift warning-class, including `write-file` preimage drift
(`docs/adrs/ADR-028-supersession-edge-model.md:85-87`). It says nothing
about V8 anchor-C content absence. The extension would need amendments to
both ADR-028 and ADR-013.

**Slice plan.** B1: a runbook in `docs/land.md` (or a sibling doc), with
skill-asset parity. B2: a read-only pre-flight command. B3 (optional,
separate ADR): a guided execution verb.

**Risks.** High operator cost: even the two-feature minimal case would
need a third slug. A wrong baseline choice fails safely (red) but wastes
effort. Some branches forbid merge commits. Originals stay red unless the
open consequence above is decided.

### 6.3 Option C — Read-only overlap assessment

**Idea.** When anchor C blocks, compute and report a classification.
Verdict, exit code, `failed_at` and persisted fields do not change.

**Classes:**

| Class | Condition |
|---|---|
| `recorded-descendant-evolution` | The Option A procedure (§6.1 steps 2–6) succeeds. |
| `context-only-drift` | Ladder step 2 passes with zero `(0/0)` hunks. This is already a pass with an advisory today (`internal/workflow/verify_landed.go:1140-1144`). Reported for completeness. |
| `unexplained-content-loss` | The ladder blocks, and no valid candidate composition restores the postimage. |
| `invalid-or-missing-evidence` | A candidate descendant exists by edge and path, but it fails an eligibility rule (§6.1 E1–E6): its evidence is not `exact`, its artifacts changed, its generation binding is missing or mismatched, its landing is unreachable or out of order, its own same-run verify fails, or a needed object is missing. The report names which one. |

**What it proves.** Nothing that changes the verdict. It explains the red.

**Trust boundary.** Read-only. Temp index only. It is not persisted as a
verification result: the persisted set stays minimal
(`internal/workflow/verify.go:134-137`). No other command may consume it as
authority.

**Must never:** flip `passed`; change the exit code or `failed_at`; be
written to `status.json` as a success; or be read by apply, land or
reconcile.

**Effect on locked decisions.** It adds an ADR-013 vocabulary amendment
(the advisory set is closed) and possibly a `schema_version` bump
(`internal/workflow/verify.go:139`). It also needs skill-asset parity if the
skills describe verify JSON.

**Slice plan.** C1: target-level classifier, JSON only. C2: human output,
plus remediation text that branches on class instead of always printing
R1/R2. C3: member-level classification for `parent-landing-drift`
(Fusion), coordinated with GH #26.

**Risks.** Readers may treat a class as authority, so labeling must be
explicit. Extra work runs only on blocked runs.

### 6.4 Combinations

| Combination | Outcome for the minimal case | Notes |
|---|---|---|
| C only | Red, but explained. | Truthful. The operator still needs B to go green. |
| C + B | Red until consolidated; green after a genuine replay. | Consolidation is the supported route. |
| C + A | Green with a mandatory advisory once A1 ships. | C is A's classifier. Promotion needs the ADR. |
| C + A + B | A for bound native stacks; B for everything A refuses. | Broadest. Most work. |

## 7. Recommendation (drafter's; pending operator decision)

The drafter recommends **C + A, with B documented**, in this order:

1. **C1–C2 first.** They are read-only, change no verdict, and every other
   path needs the classifier. They also replace misleading R1 advice for
   this case.
2. **A1 next**, gated by an ADR-013 amendment. Before acceptance, the §9.2
   negative matrix and a measured absent/present corpus must show 0 false
   greens. Scope: hard-edge-bound, generation-bound, linearly ordered
   descendants only.
3. **B1 as documentation.** It is the supported route for cases A refuses:
   legacy features without manifests, non-tpatch commits, merge
   topologies, or an operator who wants a single slug.
4. **Defer A2, A3, C3, B2 and B3.** A3 and C3 wait for the GH #26
   anchor-selection decision.

Rationale: the minimal case is the normal dependency workflow. Requiring a
third slug for every parent/child overlap is a heavy default. A, as
sketched, binds the green to real transformations and tree equality. It
does not trust edges, history or acknowledgements. If the operator prefers
consolidation-only, the plan collapses to C + B with no verify semantic
change.

## 8. Open questions for the operator

1. **Product decision.** Ordered stacks (A), required consolidation (B),
   or both?
2. **Ordering source.** Is commit ancestry of attestation commits the only
   valid order? Should parallel landings joined by a merge be refused in
   v1?
3. **Binding.** Is a matching generation dependency snapshot required?
   What happens to legacy features without `patch-generations.json`:
   refuse, or classify only?
4. **Hard edge as a filter.** Must an explaining descendant reach the
   target through hard edges? Or may an edge-less feature with exact
   evidence also explain an absence? The issue title says "independently
   landed".
5. **Composition strictness.** Ladder step 1 only, or also step 2 without
   `(0/0)`?
6. **Superseded originals.** After a verified consolidation, should the
   original's own anchor-C absence stay block-class, or become warn-class
   through an extension of ADR-028 D8 (which today covers only recipe and
   `write-file` preimage drift) plus an ADR-013 amendment?
7. **Where C lives.** A field in `verify --json`, or a separate read-only
   command? Should any part of it be persisted?
8. **Remediation text.** Should R1/R2 branch on the C class (for example,
   pointing at A's advisory or the B runbook)?
9. **Fusion scope.** Is member-level explanation (A3/C3) part of this PRD's
   later slices, or folded into the GH #26 work?
10. **B tooling.** Documentation only, a read-only pre-flight, or a guided
    execution verb?
11. **Evidence states.** May a `duplicate-equivalent` descendant explain?
12. **Aggregates.** What should `verify --all` report for an explained
    target? The current aggregate handling of this case was not audited
    for this draft.

## 9. Acceptance criteria

### 9.1 Positive

- **AC-1 (C).** In the minimal case, parent verify still exits 2 at
  `landed-content-absent`. The report classifies
  `recorded-descendant-evolution` and names `version-filter` and its
  attestation commit.
- **AC-2 (A, if accepted).** In the minimal case, parent verify exits 0.
  The V8 current half reports the new anchor result. A mandatory advisory
  names the chain. The historical half still passes on its own.
- **AC-3.** Child verify is unchanged (exit 0).
- **AC-4 (B).** A metadata-only consolidation stays at
  `historical-anchor-unavailable`. A genuine replay landing under the new
  slug passes both anchors. The originals' artifacts and landing commits
  are byte-identical before and after.
- **AC-5.** The same repository state gives byte-identical JSON. Nothing
  depends on wall-clock time.
- **AC-6.** The assessment and A do not modify the working tree, the real
  index or `.tpatch/` beyond what verify persists today.

### 9.2 Negative-case matrix (must not turn green, must not classify as explained)

| # | Case (issue category) | Expected |
|---|---|---|
| N1 | Unrecorded revert of the parent's content after the child (unrecorded revert) | Red; `unexplained-content-loss`. |
| N2 | Non-tpatch commit edits the overlapping file after the child (unrecorded revert/edit) | Red; `unexplained-content-loss`. |
| N3 | Unrecorded revert of the child's landing | Parent passes by the plain ladder (content present). Child stays red. No explanation is used. |
| N4 | Child `post-apply.patch` changed after landing (stale/changed child artifacts) | Child evidence `stale`; parent red; `invalid-or-missing-evidence`. |
| N5 | Child `apply-recipe.json` changed after landing (stale/changed child artifacts) | Same as N4. |
| N6 | Child re-recorded (new generation) but not re-landed (stale/changed child artifacts) | Binding mismatch; parent red. |
| N7 | Child landing not reachable from `HEAD`, for example a cherry-pick without trailers (missing ancestry) | Parent red; `invalid-or-missing-evidence`. |
| N8 | Shallow or partial history hides a needed object (missing ancestry) | Existing terminal states (`shallow-history` / `history-incomplete`); red. |
| N9 | Child landing is an ancestor of the parent landing (wrong order) | Red. |
| N10 | Chain whose reverse order does not apply (wrong order) | Red. No permutation search. |
| N11 | Hard edge present, but the child's patch does not explain the bytes | Red (hard edge alone). |
| N12 | Edge-less feature with exact evidence rewrote the file | Red in v1, pending Q4. |
| N13 | Historical half passes, no explanation exists | Red (historical replay alone). |
| N14 | Operator acknowledgement with no slug/path/generation/hash binding (unbound acknowledgement) | Red. A bound acknowledgement alone is also red. |
| N15 | Only a later-touch advisory relates the features | Red. |
| N16 | Parent superseded by a stale or unverified superseder | Parent red. |
| N17 | A composition step matches only with `(0/0)` | Red. |
| N18 | Metadata-only consolidation, for example `record --commit-range` + `land --no-record` | `historical-anchor-unavailable`; red. |
| N19 | An explaining descendant fails a block-severity check in its own same-run verify, or only a persisted/earlier descendant result is available (§6.1 E6) | Red; `invalid-or-missing-evidence`. |

### 9.3 Never-manufacture rules

- **M1.** A, B and C add no new trailer key and no new writer of the four
  landing-attestation trailers (`Tpatch-Feature`, `Tpatch-Patch-SHA`,
  `Tpatch-Recipe-SHA`, `Tpatch-Base-Commit`;
  `internal/gitutil/trailers.go:240-249`;
  `docs/adrs/ADR-019-tpatch-land-trailer-block-schema.md:12-15`). Today
  `land` writes that block (`internal/cli/land.go:443-447`), and the
  schema stays locked
  (`docs/adrs/ADR-019-tpatch-land-trailer-block-schema.md:50-54`). B's new
  slug gets its attestation trailers only from a normal `land`. Other
  `Tpatch-*` trailers are outside this PRD: reconcile auto-drop writes
  `Tpatch-Slug` and an optional `Tpatch-CVE` on its removal commit
  (`internal/cli/reconcile_auto_drop.go:136-147`), and ADR-019 reserves
  `Tpatch-CVE` as the only additive trailer.
- **M2.** No synthetic, copied, backfilled or hand-written evidence. A, C
  and any B pre-flight are read-only: they create no
  `patch-generations.json` entry and no provenance, coverage or capture
  sidecar. B's execution creates such artifacts only for the new slug and
  only through the existing producers: the normal `record` path (canonical
  patch at `internal/cli/cobra.go:1886-1889`; provenance at
  `internal/workflow/recipe_autogen.go:224-238`; generation, capture event
  and coverage at `internal/workflow/recipe_coverage_publish.go:229-252`)
  and the normal `land` path.
  Nothing is written for the original slugs. No backfill
  (`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:129-134`); no
  wall-clock timestamps
  (`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:107`).
- **M3.** No persisted verify record claims a historical-anchor pass that
  was not computed in the same run. A descendant explanation is never
  recorded as a historical success.
- **M4.** B never amends, rebases or force-pushes. Original landings stay
  reachable, and the originals' artifacts stay byte-identical.
- **M5.** No acknowledgement is persisted as, or treated as, replay or
  verification authority (consistent with GH #22).

## 10. Out of scope

- **GH #18** — cumulative-recorded legacy stacks. This fixture uses
  independent deltas with genuine landings.
- **GH #22** — durable later-touch acknowledgement. GH #25 is a blocking
  V8 result, not a warning. The #22 binding model may be reused, but an
  acknowledgement never becomes authority here (M5).
- **GH #14** — verified reorder/reparent. No reordering is needed; its
  commutation primitive may be reused later.
- **GH #8** — closed. Dual anchoring works in this fixture.
- **GH #19, GH #24** — recipe provenance and coverage. They do not define
  descendant materialization (§2.5).
- **GH #26** — obsolete hard-parent historical-anchor selection after a
  genuine re-landing (`docs/ROADMAP.md:47`). **Adjacent, not identical:**
  the Fusion `parent-landing-drift` path shares the fixed-anchor member
  check (`internal/workflow/verify_anchored.go:684-696`), so A3 and C3 must
  be coordinated with it. The root causes differ.
- The documented-recipe `version` field finding (F-V18-1,
  `docs/handoff/HISTORY.md:17773-17790`) is unrelated.

## Appendix A — Claims audit

All cites were re-opened while drafting. Every cite lands within ±5 lines.

| # | Claim | Cite | Verified |
|---|---|---|---|
| 1 | Landed features route to `landedPhase`. | `internal/workflow/verify_anchored.go:122-124` | yes |
| 2 | Anchor C ladder runs at `HEAD` on the target's canonical patch. | `internal/workflow/verify_anchored.go:205` | yes |
| 3 | Ladder step 1 is a plain reverse check; pass ⇒ materialized-clean. | `internal/workflow/verify_landed.go:1089-1093` | yes |
| 4 | Ladder step 2 is `-C0 --verbose`. | `internal/workflow/verify_landed.go:1102-1110` | yes |
| 5 | Step-2 failure or `(0/0)` ⇒ `absent`, `Blocked`. | `internal/workflow/verify_landed.go:1122-1138` | yes |
| 6 | Otherwise ⇒ context drift. | `internal/workflow/verify_landed.go:1140-1144` | yes |
| 7 | Historical closure runs at the replay anchor; then `foldAnchorC`. | `internal/workflow/verify_anchored.go:264-268` | yes |
| 8 | Blocked ladder ⇒ V8 fails, R1/R2, `landed-content-absent`. | `internal/workflow/verify_anchored.go:299-315` | yes |
| 9 | Context drift ⇒ warn advisory only. | `internal/workflow/verify_anchored.go:316-320` | yes |
| 10 | Block-severity failure ⇒ exit 2. | `internal/workflow/verify.go:630-643` | yes |
| 11 | `failed_at` constants. | `internal/workflow/verify_landed.go:74-79` | yes |
| 12 | Closure BFS follows target's hard parents only; superseded skipped. | `internal/workflow/verify_anchored.go:376-401` | yes |
| 13 | Landed member ladder at the target's anchor tree; blocked ⇒ `parent-landing-drift` (R14); landed member not replayed. | `internal/workflow/verify_anchored.go:680-721` | yes |
| 14 | `upstream_merged` member skipped. | `internal/workflow/verify_anchored.go:654-655` | yes |
| 15 | Exact evidence requires patch, recipe and base digests to match; one match ⇒ `exact`. | `internal/workflow/verify_landed.go:843-870` | yes |
| 16 | Anchor collection: exact single slug, single parent. | `internal/workflow/verify_landed.go:1222-1240` | yes |
| 17 | Forward `-C1` qualification at `C^`; none ⇒ reason string. | `internal/workflow/verify_landed.go:1242-1258` | yes |
| 18 | Identity comparison and selection. | `internal/workflow/verify_landed.go:1260-1295` | yes |
| 19 | `forwardQualifies` uses context 1. | `internal/workflow/verify_landed.go:1322-1345` | yes |
| 20 | No qualifying anchor ⇒ `historical-anchor-unavailable`, R11. | `internal/workflow/verify_anchored.go:232-253` | yes |
| 21 | Enumeration never uses `--first-parent`. | `internal/gitutil/trailers.go:284-289` | yes |
| 22 | Four landing-attestation trailer key constants; the canonical spellings `land` emits. | `internal/gitutil/trailers.go:240-249` | yes |
| 23 | V10 supersession downgrade to warn only. | `internal/workflow/verify_anchored.go:796-800` | yes |
| 24 | Supersession true for healthy and stale superseders. | `internal/workflow/labels.go:436-472` | yes |
| 25 | Later-touch index uses `RequestedAt` and touched paths. | `internal/workflow/verify_landed.go:1411-1440` | yes |
| 26 | Later-touch advisory emitted in V10 (R13). | `internal/workflow/verify_anchored.go:864-867`; `internal/workflow/verify_landed.go:1499-1501` | yes |
| 27 | R1, R2, R11, R14 texts. | `internal/workflow/verify_landed.go:1451-1457`, `:1491-1493`, `:1503-1505` | yes |
| 28 | Apply gate checks parent state only; no-op without DAG. | `internal/workflow/dependency_gate.go:64-66`, `:84-109` | yes |
| 29 | `feature patch` capture is `CapturePatchScoped`; empty ⇒ refresh skip / fixup error. | `internal/cli/feature_patch.go:89-99` | yes |
| 30 | Capture is `git diff --` with excludes. | `internal/gitutil/gitutil.go:341-347` | yes |
| 31 | `confirm-upstreamed` is the upstream-confirmation verb. | `internal/cli/cobra.go:2718-2743` | yes |
| 32 | Persisted verify fields are a minimal set; `schema_version` field. | `internal/workflow/verify.go:134-139` | yes |
| 33 | Inventory snapshots provenance and coverage sidecars. | `internal/workflow/verify_landed.go:414-418` | yes |
| 34 | Generation dependency snapshot fields. | `internal/store/patch_generations.go:69-77` | yes |
| 35 | ADR-013 D12 ladder, measurements, closed advisory set. | `docs/adrs/ADR-013-verify-freshness-overlay.md:623-640` | yes |
| 36 | ADR-013 D13 arbitration table. | `docs/adrs/ADR-013-verify-freshness-overlay.md:645-670` | yes |
| 37 | ADR-013 D14 no broadening; unavailability terminal; re-land regains anchor H. | `docs/adrs/ADR-013-verify-freshness-overlay.md:753-775` | yes |
| 38 | ADR-013 E7: revert keeps landing reachable, content gone. | `docs/adrs/ADR-013-verify-freshness-overlay.md:439` | yes |
| 39 | ADR-013 D11 anchor C index-isolated. | `docs/adrs/ADR-013-verify-freshness-overlay.md:597` | yes |
| 40 | ADR-011 D4 hard/soft gating. | `docs/adrs/ADR-011-feature-dependencies.md:47-55` | yes |
| 41 | ADR-019 locks the four-trailer block emitted by `land`; `Tpatch-CVE` is the only reserved additive trailer; `Tpatch-Feature` is the sole binding. | `docs/adrs/ADR-019-tpatch-land-trailer-block-schema.md:7-24`, `:50-54` | yes |
| 42 | ADR-024 D3 no timestamps; D4 no backfill; D5 dependency snapshots. | `docs/adrs/ADR-024-patch-generation-manifest-boundary.md:107`, `:129-134`, `:157-187` | yes |
| 43 | ADR-028 D6 exclusion from default replay; D7 hard/soft unchanged; D8 makes only superseded historical recipe drift (including `write-file` preimage drift) warning-class, and says nothing about anchor-C content absence. | `docs/adrs/ADR-028-supersession-edge-model.md:77-87` (D8 at `:85-87`) | yes |
| 44 | ADR-029 D5 later-touch not a stack-safety proof. | `docs/adrs/ADR-029-write-file-recipe-safety.md:60-66` | yes |
| 45 | SPEC anchor C reads a temp index, not the worktree. | `SPEC.md:345-348` | yes |
| 46 | `land` flags; `--commit-range` via `record` then `land --no-record`. | `docs/land.md:19-25` | yes |
| 47 | `Tpatch-Feature` sole binding (docs). | `docs/land.md:151` | yes |
| 48 | Metadata-only landing onto `main` out of scope in v1. | `docs/land.md:252-258` | yes |
| 49 | Hard/soft table; verify walks hard-parent closure only. | `docs/dependencies.md:37-40`, `:189-199` | yes |
| 50 | GH #25 still reproduces on v0.18.0; complete recipe coverage does not change it; Fusion still `parent-landing-drift`. | `docs/handoff/HISTORY.md:17761-17772` | yes |
| 51 | ROADMAP GH #25 and GH #26 rows. | `docs/ROADMAP.md:46-47` | yes |
| 52 | Key design decisions and parity guard. | `CLAUDE.md:81-91` | yes |
| 53 | v0.18.0 minimal case: parent exits 0 before the child lands and 2 after (`landed-content-absent`); evidence `exact`; V7 and V10 pass; V8 fails; child exits 0; complete coverage does not change it. | `docs/handoff/HISTORY.md:17761-17769` | yes |
| 54 | Minimal case, historical replay remains valid, downstream case, consolidation complication, six-step recovery, Fusion comment. | GH #25 issue body and comment | yes |
| 55 | `land` writes the four-trailer attestation block. | `internal/cli/land.go:443-447` | yes |
| 56 | Reconcile auto-drop writes `Tpatch-Slug` and an optional `Tpatch-CVE` on its removal commit. | `internal/cli/reconcile_auto_drop.go:136-147` | yes |
| 57 | `record` writes `patch-generations.json`, the canonical patch and the recipe capture-event, coverage and (when planned) provenance artifacts. | `internal/cli/cobra.go:1886-1889`; `internal/workflow/recipe_autogen.go:224-238`; `internal/workflow/recipe_coverage_publish.go:229-252` | yes |
| 58 | A verify run uses one immutable inventory and one `git log` enumeration, then re-states the inventory before finalizing. | `internal/workflow/verify_landed.go:18-26` | yes |
| 59 | An unstable inventory fails V7, V8 and V10 with `failed_at: snapshot-unstable`. | `internal/workflow/verify_landed.go:1652-1672` | yes |

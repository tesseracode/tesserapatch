# Patch Theory and Commutation

**Status**: Snapshot research (paper-only; no implementation authorized)
**Date**: 2026-05-10
**Owner**: Core
**Related**: [SPEC §7 reconciliation](../../SPEC.md#7-reconciliation--4-phase-decision-tree),
[Reconcile Workflow](../reconcile.md),
[ADR-015 prior-art identity mapping](../adrs/ADR-015-prior-art-identity-mapping.md),
[Competitive Landscape §Lane D](../market-research/competitive-landscape.md#lane-d--patch-theory-dvcses-out-of-band-reference)

## Why this doc exists

tpatch currently records feature intent plus a canonical `post-apply.patch`,
then tries to replay or reconcile that patch when upstream moves. The open
research question is whether patch-theory vocabulary can help tpatch reason
about:

- when two feature patches commute,
- when order is constrained,
- when a conflict is real versus an artifact of patch context,
- and how much of that can be decided before asking a provider.

This doc is not a proposal to adopt Darcs or Pijul. It extracts the useful
math and translates it into tpatch's Git-patch substrate.

## Refresh triggers

- A future PRD proposes a commutation graph, patch-order planner, or structural
  reconcile phase.
- Darcs/Pijul-style patch theory enters a production tool tpatch wants to map.
- tpatch changes the canonical replay authority away from
  `artifacts/post-apply.patch`.

## 1. Executive findings

1. **Patch theory's core abstraction is useful even if the substrate is not.**
   tpatch cannot get Darcs/Pijul guarantees from unified diffs, but it can use
   the same words: identity, inverse, composition, commutation, dependency, and
   conflict.
2. **Commutation is the key middle concept.** If patches `A` and `B` commute,
   `A;B` and `B;A` reach the same tree. If they do not commute, tpatch needs
   either an ordering constraint, a resolution, or a provider/human pass.
3. **Pijul's graph model suggests better anchors.** Pijul uniquely identifies
   lines/vertices by the change that introduced them and their position in that
   change, so a line keeps identity across context changes. tpatch can imitate
   this partially with structural anchors and patch-generation hashes.
4. **The practical tpatch primitive is a commutation matrix.** For a cluster of
   features, tpatch can test or estimate pairwise relationships:
   `commutes`, `depends_on`, `conflicts`, `unknown`. The matrix can drive
   topological ordering, search, or provider escalation.
5. **Empirical commutation is possible.** In an isolated worktree, apply
   `A;B` and `B;A`; if both succeed and the resulting tree hashes match after
   canonical filtering, treat the pair as commuting for that base.

## 2. Patch-theory vocabulary

| Concept | Patch-theory meaning | tpatch translation |
|---|---|---|
| Patch | A first-class change that transforms one repository state into another. | A feature's canonical `artifacts/post-apply.patch`, optionally plus its apply recipe and intent docs. |
| Identity | The stable identity of the change, distinct from its current bytes. | Feature slug, with future moving identity represented by patch hash/trailer per ADR-015. |
| Moving identity | The current version of the change's bytes. | SHA-256 / patch-id / future patch generation for `post-apply.patch`. |
| Inverse | A patch that undoes another patch. | `git apply -R` or reverse-apply check in reconcile phase 1. |
| Composition | Applying one patch after another. | Applying feature patches in a candidate order. |
| Commute | `A;B` and `B;A` are both valid and have the same effect, possibly with transformed patch representations. | Patches can be reordered without changing the resulting tree. |
| Dependence | Patch `B` needs state introduced by patch `A`. | A hard dependency edge, a hunk context dependency, or an anchor dependency. |
| Conflict | No unambiguous merged state exists, or patch order changes the result. | 3-way conflict, failed apply, incompatible operation-level edit, or conflicting structural anchors. |
| Resolution patch | A new patch that records the chosen resolution. | A reconciled patch generation or provider/human conflict resolution artifact. |

## 3. Prior art

### 3.1 Darcs

Darcs describes itself as a VCS that focuses on changes rather than snapshots.
Its patch-theory docs define patches, identities, inverses, merge, conflict,
and commute. The important takeaways for tpatch are:

- patches have identities separate from their concrete position in history;
- patch sequences can sometimes be reordered while preserving identity and end
  state;
- conflicts are cases where no unambiguous merge/commutation exists;
- conflict resolution is itself recorded, rather than being an invisible
  side-effect.

Darcs is valuable as vocabulary. It is less directly portable as architecture:
tpatch sits on Git and stores unified diffs in `.tpatch/`, so it cannot make
all patch operations algebraically total or invertible.

### 3.2 Pijul

Pijul is the stronger implementation reference for modern patch theory. Its
manual models a repository as a directed graph of lines/blocks. Vertices are
identified by the hash of the change that introduced them plus position within
that change, so identical text introduced by different changes is not the same
line. Deletions are represented through edge labels rather than destructive
removal. Dependencies arise from the contexts required to make a change
meaningful.

The useful tpatch lessons:

- **Identity is not text equality.** Two identical lines can be different if
  they were introduced by different patches.
- **Context should be identity-rich.** A hunk's surrounding lines are weak
  anchors; structural anchors and patch-generation IDs are stronger.
- **Conflicts can be modeled explicitly.** Pijul names conflict shapes in its
  graph. tpatch can name conflict/unknown shapes in artifacts instead of
  collapsing everything to "blocked".
- **Diff granularity is flexible.** Pijul notes that splitting content could be
  driven by language structure rather than only lines. That maps directly to
  AST/keypoint research in the structural fingerprint doc.

### 3.3 Snapshot VCS contrast

Git stores snapshots and derives diffs. Patch theory stores changes as the
primary object. tpatch is a hybrid:

- Git remains the substrate.
- `.tpatch/features/<slug>/artifacts/post-apply.patch` is the replay authority.
- Intent docs add semantic context.
- Provider-backed reconcile can answer questions that byte-level replay cannot.

The middle path is not "become Pijul"; it is "add enough patch-theory metadata
to decide more cases before provider escalation."

## 4. A tpatch commutation model

For a feature patch `P`, define a research-only model:

```text
P = {
  id: stable slug,
  version: patch hash / generation,
  preconditions: anchors that must be present,
  reads: context needed to apply safely,
  writes: files/ranges/AST nodes modified,
  effects: normalized added/deleted text or structural edits
}
```

Two patches `A` and `B` likely commute when:

1. both apply to the same base,
2. `writes(A)` does not overlap `reads(B)` or `writes(B)`,
3. `writes(B)` does not overlap `reads(A)` or `writes(A)`,
4. structural anchors for each survive the other patch,
5. empirical `A;B` and `B;A` reach the same normalized tree.

They likely do not commute when:

- either patch deletes or rewrites an anchor the other patch needs;
- both edit the same hunk, AST node, exported symbol, config key, or generated
  block;
- applying them in different orders yields different tree hashes;
- one patch only applies after the other creates a file/symbol/path.

## 5. Static versus empirical tests

| Test | Cost | Strength | Failure mode |
|---|---|---|---|
| File-set disjointness | Very low | Good first pass. | Misses shared generated files, imports, config keys, semantic dependencies. |
| Hunk-range overlap | Low | Detects classic textual conflicts. | Line numbers drift; moved blocks can look unrelated. |
| Anchor survival | Medium | Better under upstream drift. | Requires good anchor design. |
| AST/CFG write-set overlap | Medium/high | Good for code moves/refactors. | Language support and parser errors. |
| Empirical `A;B` vs `B;A` | Medium/high | Direct evidence on current base. | Only proves commutation for one base and one normalization. |
| Test-suite equivalence | High | Catches behavioral breakage. | Slow and incomplete; not a proof. |

## 6. Ordering implications

Patch theory suggests a planner should separate three problems:

1. **Known constraints.** Hard dependencies and proven non-commuting pairs
   become ordering edges.
2. **Independent components.** Commuting/disjoint patches can be applied in any
   order or in parallel candidate worktrees.
3. **Unknown clusters.** Small uncertain components are where search-based
   ordering earns its keep. Large known-independent sets should not be sent
   through expensive search.

A future artifact could look like:

```json
{
  "base": "<upstream-sha>",
  "features": ["a", "b", "c"],
  "relations": [
    {"left": "a", "right": "b", "kind": "commutes", "evidence": "empirical-tree-hash"},
    {"left": "b", "right": "c", "kind": "depends_on", "evidence": "creates-file"},
    {"left": "a", "right": "c", "kind": "unknown", "evidence": "shared-anchor"}
  ]
}
```

This is not a schema proposal. It illustrates the information a middle pass
would need.

## 7. Fit with current tpatch

Current reconcile phases are:

1. reverse-apply,
2. operation-level evaluation,
3. provider-semantic,
4. forward-apply.

Patch-theory research most naturally fits between phase 2 and phase 3:

- after simple deterministic checks,
- before provider-semantic analysis,
- only for features/patch clusters that remain uncertain.

It also informs record-time collision prevention: a byte-identical collision is
one trivial identity case; patch-id and structural identity extend the ladder.

## 8. Non-goals and cautions

- Do not claim formal patch-theory guarantees for unified diffs.
- Do not auto-drop or auto-rewrite features based on fuzzy commutation alone.
- Do not make search nondeterministic without storing seed, candidate list, and
  scoring evidence.
- Do not collapse "dependency" and "conflict"; they are different relations.
- Do not use global feature ordering when only a small uncertain connected
  component needs ordering.

## 9. References

- Darcs, "Patch Theory, Take N", patch identity, inverse, merge, conflict, and
  commute: https://darcs.net/Theory/PekkaPatchTheory
- Darcs home page, "focus on changes rather than snapshots":
  https://www.darcs.net/
- Pijul manual, "Theory", graph-of-lines model, dependencies, conflicts, and
  CRDT framing: https://pijul.org/manual/theory.html
- ADR-015, "Prior-Art Mapping for Identity Duality, Operation Log, and Stack
  Primitives": ../adrs/ADR-015-prior-art-identity-mapping.md
- [Adjacent CLI argument conflict case study](case-studies/adjacent-cli-args-conflict-2026-08/)
- [GH #14 — verified feature reparenting/reorder](https://github.com/tesseracode/tesserapatch/issues/14)

## Open questions

- What is the smallest commutation artifact that would be useful without
  becoming a new VCS layer?
- Should empirical commutation require exact tree equality, patch-id equality
  of the combined diff, or a language-aware normalization?
- Can tpatch infer dependency edges from apply recipes strongly enough to avoid
  search in most cases?
- Should conflict resolutions become first-class patch generations before a
  search planner exists?
- When an applicable operation succeeds but the canonical unified diff
  conflicts, should reconcile stage the operation result as a deterministic
  candidate before provider escalation (GH #12)?

## Disputes

None logged.

# Search-based Patch Application

**Status**: Snapshot research (paper-only; no implementation authorized)
**Date**: 2026-05-10
**Owner**: Core
**Related**: [Reconcile Workflow](../reconcile.md),
[PRD-provider-conflict-resolver](../prds/PRD-provider-conflict-resolver.md),
[PRD-record-auto-base](../prds/PRD-record-auto-base.md),
[PRD-record-collision-detection](../prds/PRD-record-collision-detection.md),
[Patch theory research](patch-theory-and-commutation.md),
[Structural fingerprint research](patch-identity-and-structural-fingerprints.md)

## Why this doc exists

If patch order were always deterministic, tpatch could topologically sort
features and replay them. In practice, upstream drift creates uncertainty:
some patches commute, some depend on others, some only apply after relocation,
and some conflict in ways a provider can fix but a deterministic heuristic
cannot.

This doc researches a middle path: use search and optimization algorithms to
try multiple plausible orders and relocations in isolated worktrees, score the
results, and only escalate unresolved cases to a provider or human.

## Refresh triggers

- A PRD proposes a non-provider reconcile planner.
- tpatch adds structural fingerprints, commutation artifacts, or shadow
  worktree search.
- Real-world reconcile data shows clusters large enough that greedy ordering is
  insufficient.

## 1. Executive findings

1. **The problem is optimization, not adversarial game play.** Chess-style
   minimax is usually the wrong default because upstream is not making moves
   against us. MCTS, beam search, evolutionary search, and constraint solving
   fit better.
2. **Search belongs only in uncertain clusters.** Known hard dependencies and
   independent components should be handled deterministically. Search should
   focus on small connected components with unknown/conflicting relations.
3. **Fitness is multi-objective.** "Patch applies" is only one score. Better
   fitness combines clean apply, fewer conflicts, structural-anchor survival,
   patch identity preservation, test results, and minimal churn.
4. **Nondeterministic algorithms must be reproducible.** Store seed, candidate
   orderings, scores, winning order, failed attempts, and caps.
5. **Evaluation must be isolated.** Every candidate order belongs in a scratch
   worktree or equivalent sandbox. The real tree changes only after explicit
   acceptance.

## 2. Problem model

Given:

- a base tree `T0`,
- a set of feature patches `P = {p1, p2, ... pn}`,
- known dependency edges,
- known/estimated commutation and conflict relations,
- optional relocation candidates per patch,
- optional test command and validation checks,

find:

- an order of patches,
- one relocation choice per patch when needed,
- and a conflict resolution strategy if available,

that maximizes a score while respecting hard constraints.

Candidate state:

```text
state = {
  applied: ordered list of patch+relocation choices,
  remaining: patches not yet applied,
  tree: scratch tree after applied choices,
  score: cumulative fitness vector,
  diagnostics: failures/conflicts/anchor drift
}
```

## 3. Deterministic pre-pass

Search should not start from an unconstrained `n!` permutation space. First:

1. Load explicit tpatch hard dependencies.
2. Add obvious dependency edges: patch creates file/symbol that another patch
   edits; patch changes config key another patch reads; recipe `created_by`
   gates if present.
3. Add non-overlap facts: disjoint file sets, disjoint structural write sets,
   empirical commuting pairs.
4. Partition into connected uncertain components.
5. Sort trivial components deterministically.
6. Search only the remaining components.

This keeps the expensive planner small and auditable.

## 4. Candidate algorithms

### 4.1 Exhaustive enumeration

For very small uncertain clusters, enumerate every topologically valid order
and score each. This gives the strongest answer but grows factorially.

Use when:

- `n <= 7` or another tight cap;
- candidate relocation choices are few;
- tests are cheap or disabled for preliminary scoring.

### 4.2 Greedy and beam search

Greedy search picks the next patch with the best immediate apply score. Beam
search keeps the top `k` partial orders at each depth.

Use when:

- clusters are medium sized;
- early apply quality predicts final success;
- tpatch needs predictable runtime.

Beam search is a good first practical planner because it is simple, bounded,
and deterministic when sorting ties by stable IDs.

### 4.3 A* search

A* expands the partial order with the lowest estimated total cost:

```text
f(state) = cost_so_far(state) + lower_bound_cost_remaining(state)
```

It is attractive if tpatch can define an admissible lower bound on remaining
conflict cost. That may be hard. Without a good bound, A* degenerates toward
expensive best-first search.

Use when:

- the objective is simple enough to bound;
- optimality matters more than wall-clock;
- cluster size is small/medium.

### 4.4 Monte Carlo Tree Search

MCTS samples many possible continuations from a partial order and uses a bandit
policy (commonly UCT) to balance exploration and exploitation. It is closer to
Go-style search than minimax because the branching factor can be large and
terminal evaluation is expensive.

Use when:

- many plausible orderings exist;
- a full exhaustive search is impossible;
- evaluation can be sampled cheaply before running full tests.

For tpatch, an MCTS rollout can:

1. choose a valid next patch,
2. choose a relocation candidate if needed,
3. attempt apply in the scratch tree,
4. accumulate conflict/anchor/test score,
5. stop at full order or failure cap.

### 4.5 Evolutionary / genetic algorithms

Evolutionary search treats an ordering as a chromosome. A population of
candidate orderings evolves through selection, crossover, and mutation.

Permutation-specific tools:

- **Order crossover** preserves relative order from parents.
- **Swap/insert mutation** moves one patch.
- **Precedence repair** fixes candidates that violate hard dependencies.
- **Elitism** preserves the best candidates across generations.

Fitness can be multi-objective:

```text
fitness = [
  applied_patch_count,
  -conflict_count,
  -reject_count,
  anchor_survival_score,
  patch_identity_preservation_score,
  test_pass_score,
  -total_changed_lines_after_relocation,
  -runtime_cost
]
```

Use when:

- clusters are large enough that beam search is too narrow;
- evaluation can be parallelized;
- near-optimal is acceptable and reproducible seeds are stored.

### 4.6 Simulated annealing and tabu search

These are local-search methods over permutations. Start with an order, make
small changes, accept improvements, and sometimes accept worse moves to escape
local optima.

Use when:

- an initial order exists;
- the search space is smooth enough that local swaps matter;
- tpatch needs simpler runtime control than a full genetic algorithm.

### 4.7 Constraint solving / MaxSAT / ILP

Some parts of the problem are constraint satisfaction:

- `A before B`
- `not(A and B both edit anchor X without resolver)`
- "choose one relocation candidate per patch"
- "maximize satisfied soft constraints"

MaxSAT or ILP can encode this cleanly, but would likely add dependencies and
complexity outside tpatch's current zero-dep posture. It remains useful as a
research reference and possible external/offline experiment.

## 5. Fitness function design

A good score must be more nuanced than "did `git apply` exit 0?"

| Signal | Direction | Notes |
|---|---|---|
| Strict apply success | Maximize | Strongest cheap signal. |
| 3-way clean apply | Positive but weaker | Better than conflict, weaker than strict. |
| Conflict markers / `.rej` files | Minimize to zero | Hard failure for automatic accept. |
| Patch-id preservation | Maximize | Did the intended line-level change survive? |
| Structural-anchor survival | Maximize | Did keypoints remain mapped after each apply? |
| Test command pass | Maximize | Expensive; maybe only for finalists. |
| Feature-specific validation | Maximize | Stronger than generic tests when available. |
| Churn introduced by relocation | Minimize | Avoid wild rewrites to make apply pass. |
| Runtime / attempts | Minimize | Planner must respect caps. |
| Provider calls avoided | Maximize | The business goal of the middle pass. |

The score should be recorded as a vector first and collapsed to a scalar only
for algorithm internals. Humans and future agents need to inspect why a winning
order won.

## 6. Suggested planner ladder

| Cluster shape | Recommended strategy |
|---|---|
| All relations known and acyclic | Deterministic topological order. |
| All pairs commute | Stable lexical/slug order for reproducibility. |
| Small uncertain cluster | Exhaustive enumeration. |
| Medium uncertain cluster | Beam search with deterministic tie-breaking. |
| Large uncertain cluster, cheap partial scoring | MCTS. |
| Large uncertain cluster, parallel evaluation available | Evolutionary search. |
| Many crisp soft/hard constraints | Constraint solver prototype, likely out-of-process. |
| No candidate reaches safety threshold | Provider/human escalation. |

## 7. Worktree and artifact contract

Any future search planner should be constrained by tpatch's safety posture:

- Run candidates in isolated Git worktrees or equivalent temporary copies.
- Refuse dirty real trees before starting, same spirit as reconcile preflight.
- Never mutate `.tpatch/features/*` until a candidate is accepted.
- Store:
  - base SHA,
  - feature set,
  - hard constraints,
  - random seed,
  - algorithm and parameters,
  - attempted candidates,
  - score vector per candidate,
  - winning order,
  - failure reasons,
  - test command results if run.
- Cap by attempts, time, cluster size, and disk usage.
- Make the default behavior "preview/report", not "silently apply".

## 8. Where minimax fits

Minimax assumes an adversary choosing moves to make the outcome worse. Patch
application usually has no adversary; upstream history is fixed at reconcile
time. Therefore:

- minimax is not the default;
- MCTS is a better fit for large branching under expensive evaluation;
- robust planning across multiple possible upstream targets could use a
  minimax-like "optimize worst case" objective, but that is a different problem
  from applying a known feature set to one upstream ref.

## 9. References

- Mark Harman and Bryan F. Jones, "Search-based software engineering",
  Information and Software Technology, 2001.
- Westley Weimer et al., "Automatically finding patches using genetic
  programming", ICSE 2009.
- Claire Le Goues et al., "GenProg: A Generic Method for Automatic Software
  Repair Using Program Synthesis", IEEE TSE, 2012.
- Levente Kocsis and Csaba Szepesvari, "Bandit Based Monte-Carlo Planning",
  ECML 2006.
- Judea Pearl, "Heuristics: Intelligent Search Strategies for Computer Problem
  Solving", 1984, for A* and heuristic search background.
- Scott Kirkpatrick, C. Daniel Gelatt, and Mario P. Vecchi, "Optimization by
  Simulated Annealing", Science, 1983.

## Open questions

- What is the maximum uncertain-cluster size in real tpatch repositories?
- Which fitness signals can be computed cheaply enough to run per candidate?
- Should tests run inside the search loop, only for finalists, or only after
  human preview?
- What is the minimum artifact needed to make stochastic search reproducible?
- Should the first planner be beam search rather than evolutionary search to
  reduce implementation risk?

## Disputes

None logged.

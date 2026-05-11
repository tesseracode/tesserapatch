# Competitive Landscape — Prior-Art Positioning for tpatch

**Status**: Living analysis (refresh when a new system enters the
picture, when a major release of a listed system shifts a column, or
when a tpatch milestone closes a gap)
**Date**: 2026-05-02 (initial); **Last refresh**: 2026-05-09
(Cursor+Graphite acquisition added across §1 Lane B/C, §3 PESTEL
Political, §4 Five Forces Rivalry/Substitutes, §5 SWOT)
**Owner**: Core
**Related**: [ADR-015](../adrs/ADR-015-prior-art-identity-mapping.md),
[WP-001 §3.3 / §5.2](../whitepapers/WP-001-feature-slice-gap.md),
[PRD-tpatch-land](../prds/PRD-tpatch-land.md),
[PRD-record-collision-detection](../prds/PRD-record-collision-detection.md),
[PRD-record-auto-base](../prds/PRD-record-auto-base.md),
[`docs/commits.md`](../commits.md),
[`docs/handoff/CURRENT.md`](../handoff/CURRENT.md)

## Why this doc exists

This is a **strategic-positioning artifact**. It is not a decision
(use ADRs), not a proposal (use PRDs), not a multi-agent debate (use
whitepapers). It exists to:

1. Map tpatch against every prior-art system in this lane that has
   been researched.
2. Articulate, axis by axis, **what we take, what we leave, and what
   is genuinely ours**.
3. Provide the strategy-language artifacts (PESTEL, SWOT, SMART,
   Strategy Canvas) that future product / roadmap discussions can
   reference without re-deriving them.

This doc is not load-bearing for any milestone. It is a shared map
the team can return to when a roadmap question turns into a
positioning question.

## Refresh triggers

- A new system enters the lane → add a column / row.
- A major release of a listed system shifts a score → update.
- A tpatch milestone closes that materially closes a gap → update
  the tpatch column.
- A scoring is disputed → open a `## Disputes` block, resolve via
  discussion, fold the resolution back in.

---

## 1. The competitor set

Three lanes. tpatch lives in Lane A. Lanes B and C are listed for
context but scored only where overlap is real.

### Lane A — Patch management (our lane)

| System | Era / status | One-liner |
|---|---|---|
| **Quilt** | 2003, mature | Stack-of-patches manager (filename-as-id, `series` file). The original; still ubiquitous in Debian source format `3.0 (quilt)`. |
| **StGit** | 2005 → modern Rust rewrite (2020+) | "Quilt on Git" — patches are Git commit objects on a stack. |
| **Mercurial Queues (MQ)** | 2005, formally deprecated 2019 | Direct port of Quilt to Hg. Deprecated because filename-as-id doesn't survive rebases gracefully. |
| **Mercurial-evolve** | 2014–present, experimental | Stable change-id, mutable commit-hash, **obsolescence markers** exchanged via push/pull. The deepest model in the set. |
| **jj** (Jujutsu) | 2020 (Google) | Two-layer identity (change-id stable, commit-id moves), repo-wide **operation log**, `op restore`. The cleanest modern synthesis. |
| **gbp-pq** | ~2007, mature (part of git-buildpackage) | Two-branch round-trip: `import` files → commits, edit, `export` commits → files. Has `--time-machine N` for upstream reapply. |
| **tpatch** | 2026, v0.6.1 | Patch-as-artifact in `.tpatch/`, deterministic apply-recipe, AI-assisted 4-phase reconcile, multi-format skill emission. |

### Lane B — Stacked-PR workflow (different problem, scored only on overlap)

| System | Era / status | One-liner |
|---|---|---|
| **git-spr** | OSS | Linear, commit-centric stacks; tracking implicit in commit order. |
| **ghstack** | OSS (Meta) | Linear, commit-centric stacks; tracking via `gh/<user>/N/{base,head}` refs. |
| **git-gud** | OSS | Linear, branch-centric stacks; tracking via commit trailers (`GG-ID`, `GG-Parent`) + `.git/gg/config.json`. |
| **stk** | OSS | Linear, branch-centric stacks; tracking via `.git/stacks/<name>.yaml`. |
| **Graphite** | 2020+, **acquired by Cursor (Dec 2025)** | Closed-source commercial. Stacked PRs on Git/GitHub with **hg-evolve-style auto-rebase** (`gt modify` recursively updates downstream commits when an upstream commit changes). The most-funded, most-polished system in the lane. CLI + VS Code + web app + AI reviewer. Cross-listed in Lane C. |

These manage **stacks of pull requests against your own work**, not
**stacks of patches against an upstream you don't control**. Their
identity primitives (especially git-gud's commit trailers) are still
useful prior art for our trailer block — see §9. **Graphite's
auto-rebase semantics are the closest production-validated
implementation of hg-evolve-style commit obsolescence on Git** —
prior art for ADR-015 D3 (op log) at the substrate level, even
though the problem they solve is upstream-of-trunk, not
fork-vs-upstream.

### Lane C — Commercial / closed-source (most direct competitive threat)

| System | Posture | One-liner |
|---|---|---|
| **entire.io** | Independent commercial | Closed-source platform pitched at "intent-aware change management." Roadmap and primitives are not public. Primary commercial lookalike. |
| **Graphite** *(via Cursor)* | **Acquired by Cursor — Dec 2025** | Stacked-PR + AI reviewer + merge queue. Now part of an AI-coding-tool ecosystem with the explicit roadmap goal of *"self-driving PRs"* — agent-authored code reviewed by an AI reviewer through Graphite's stacking UX. The acquisition makes Cursor (~$300M+ deal) a candidate buyer for adjacent fork-management primitives. Cross-listed from Lane B because the *Cursor-owned* version of Graphite is no longer just a stacked-PR tool — it is a developer-platform play. |

### Lane D — Patch-theory DVCSes (out-of-band reference)

| System | Era / status | One-liner |
|---|---|---|
| **darcs** | 2003, mature but slow at scale | Pioneered VCS based on a *theory of patches*. Independent patches commute (apply in any order, same result and identifier). Conflict resolutions are first-class objects that don't reappear. Famously slow at large repos / merges. |
| **Pijul** | 2018+, active, GPL2 | "darcs done right" — same patch-theory foundation, modern performance. Has `channels` instead of branches; in many workflows, "feature branches" are just changes. First-class conflicts; partial clones. |

These are **VCSes**, not patch-management tools. They are listed
because their substrate-level approach (patches as the
substrate, not as a layer above a snapshot graph) sits one
level deeper than every Lane A column. We do not adopt patch
theory — SPEC commits to a Git / optionally-jj substrate. They
are reference frame only. See §2 for a brief commentary and
ADR-015 §Amendments 2026-05-03 for the D1–D7 mapping.

---

## 2. Concept map (extends ADR-015 §Working framework)

This is ADR-015's table with gbp-pq added and two new rows
(`Upstream re-apply intelligence`, `Standard header / trailer`).

| Concept | tpatch (today) | tpatch (post ADR-015 D2+D3) | Quilt | StGit | hg MQ | hg evolve | jj | gbp-pq |
|---|---|---|---|---|---|---|---|---|
| Stable identity | `slug` | `slug` | filename | patch name | filename | change-id | change-id | filename + `Gbp-Pq: Name` |
| Moving identity | `post-apply.patch` bytes (overwritten) | versioned `patches/<slug>/v<n>.patch` | none | git commit hash | none | obsolescence successor hash | commit-id | dropped on each round-trip |
| Cross-feature op log | none | `.tpatch/oplog/` (append-only) | none | git reflog | none | obsolescence DAG | `jj op log` | none |
| Refresh primitive | Path B sequence | Path B sequence (alias deferred) | `quilt refresh` | `stg refresh` | `hg qrefresh` | `hg amend` | `jj squash` into `@-` | `gbp pq export` |
| **Upstream re-apply intelligence** | 4-phase reconcile + provider | 4-phase reconcile + provider | manual `quilt push` | `stg rebase` | manual | `hg evolve` auto-resolve | `jj rebase` | **`--time-machine N`** + auto-detect-applied |
| Recovery / undo | git reflog only | (not yet authorized) | manual | git reflog | manual | `hg evolve` | `jj op restore` | none |
| Standard header / trailer | `Tpatch-Feature` (interim) | 4-trailer block (per PRD-tpatch-land §3.4) | none | none | none | none | none | **DEP-3** + `Gbp-Pq:` |

The new finding from gbp-pq is the **`--time-machine`** primitive:
when re-applying patches to a rebased upstream fails, walk back the
upstream history N commits and try each. That is a single-repo
upstream-search heuristic that none of jj/evolve/StGit names as a
first-class flag.

### A note on patch-theory DVCSes (darcs, Pijul)

darcs (2003) and Pijul (2018+) sit one level deeper than every
column in the table. They are *patch-theory* DVCSes: the
substrate is a commuting patch graph, not a snapshot history.
Independent patches apply in any order with the same result and
identifier. Conflicts are first-class objects resolved by a
single change that doesn't reappear.

For our concept map this means three rows already in the table
become *natively trivial* in patch theory:

- **Stable identity** — patch IDs are content-derived;
  commutation guarantees they survive context changes.
- **Cross-feature op log** — the patch graph IS the log.
- **Recovery / undo** — drop a patch; the math handles it.

We are not adopting patch theory. SPEC commits to a Git / jj
substrate, where commutation is not free. ADR-015 §Amendments
2026-05-03 records the full D1–D7 mapping. The takeaway for
tpatch is inspirational only: patch-theory primitives validate
the *direction* of D1 / D2 / D3 / D5 but cannot be ported onto
a snapshot substrate.

---

## 3. PESTEL (refreshed 2026-05-02)

Six external forces. Each cell lists the most-relevant signals for
tpatch's planning horizon.

### Political

- **GitHub Copilot proxy policy.** ADR-014 / ADR-005 already depend
  on Copilot's proxy being available for `localhost:4141`-style
  routing. A policy change there would force a provider rewrite.
- **Open-source funding climate.** OSS infrastructure projects in
  this lane (jj at Google, gbp-pq via Debian) survive because they
  are second-order to a parent ecosystem. tpatch has no parent.
- **Fork-friendliness of the upstreams we patch.** A maintainer
  hostile to forks reduces the addressable population for a
  reconciliation tool. Currently neutral; trend uncertain.
- **AI-coding-tool consolidation (NEW signal — 2026-05).** Cursor
  acquired Graphite in Dec 2025, Supermaven before that, plus
  Koala talent. Pattern: AI-coding-tool incumbents are buying
  adjacent developer-workflow tooling rather than building. If
  the pattern holds, an established AI-coding tool may eventually
  acquire or build a fork-management primitive (entire.io is the
  obvious candidate target). Also: the corporate-development
  posture of these acquirers tilts the lane toward closed-source
  bundles — ADR-002's provider abstraction and the OSS positioning
  are both moats against this trend.

### Economic

- **LLM token cost.** Our 4-phase reconcile depends on a provider
  for phase 3. Token-price volatility (or a 10x reduction) directly
  changes the cost-benefit of provider-assisted reconcile vs
  heuristic fallback.
- **Commodification of foundation models.** Helpful — the provider
  abstraction (ADR-002) means we ride this. Negative for any
  competitor (entire.io) whose moat depends on a single provider.
- **Fork economy.** The volume of "fork + customize OSS" is rising
  with agentic coding. That's the demand curve we ride.

### Social

- **Developer comfort with AI in the loop.** Trending up. Within
  the next 18 months "agent-coding" is likely to be the default for
  greenfield work. Forks lag — tpatch's audience is the lag cohort,
  who want structure and replay over speed.
- **Upstream-maintainer culture.** Maintainers prefer clean PRs
  with intent. tpatch's lifecycle phases (analyze / define /
  explore) produce intent that survives upstreaming. That's a
  social-fit lever, not just a tool feature.
- **Aversion to closed-source LLM use in OSS pipelines.** Mild but
  real. Provider abstraction protects us. entire.io is exposed.

### Technological

- **Foundation-model context windows.** 1M-token context (Anthropic
  beta, GPT-5.x, Gemini) makes whole-feature reconcile cheap and
  changes what "smart" reconciliation can do. The case-study repos
  fit comfortably in 1M tokens.
- **Alternative VCS adoption (jj).** jj's Git-compat means tpatch
  can ride on top of jj-managed repos as well as Git. We should
  validate this works.
- **GitHub Copilot proxy stability.** ADR-014 just shipped routing
  to handle the latest proxy regressions. The proxy is the most
  fragile single point of dependency in our stack.
- **Coding-agent skill formats.** 6 today. Will plateau or
  consolidate. Parity guard (`assets/assets_test.go`) is the lever
  that lets us track without burning out.

### Environmental

- **Inference energy cost.** Real but small at our scale.
  Heuristic fallback (offline mode) is the lever — keeps the tool
  usable without a model spin-up.

### Legal

- **AI-generated-code copyright.** Open question. Provider-assisted
  reconcile produces code transformations, not de novo code; risk is
  lower than full-line generation but not zero.
- **OSS license boundaries when AI assists with derivatives.** Same
  vector. Mitigated by transparent recipes — every byte produced by
  the provider is in `apply-recipe.json`, attributable.
- **EU AI Act / regional regulations.** Watch but no current
  blocker. Provider abstraction means we adapt by swapping
  providers, not by changing the tool.

**Synthesis.** Three signals are worth instrumenting in the
roadmap:

1. **GitHub Copilot proxy posture** — affects ADR-014 and the user's
   default provider choice. Dependency we don't control.
2. **Foundation-model context-window growth** — unlocks
   whole-feature reconcile and reduces the value of fine-grained
   slicing.
3. **entire.io public roadmap** — direct competitor signal. No
   public roadmap yet; watch.

---

## 4. Porter's Five Forces (refreshed 2026-05-03; Cursor+Graphite update 2026-05-09)

PESTEL maps macro forces; Five Forces maps competitive dynamics.
Verdicts: H = high, M = medium, L = low.

### Threat of new entrants — **HIGH**

The agentic-coding ecosystem is hot. The barrier to publishing a
"smart-rebase" CLI is small once you have a provider abstraction
and a patch model. New entrants would not need to match our
lifecycle phases or skill emission to capture mindshare.
*Mitigants:* ADR-015's identity / op-log discipline raises the
*correctness* bar; PRD-tpatch-land's four-trailer block raises
the *forwardable* bar. Neither is reproducible without similar
foundational work.

### Bargaining power of suppliers — **HIGH**

The "supplier" is foundation-model providers (Anthropic / OpenAI
/ GitHub Copilot proxy / Google). They can change pricing,
deprecate endpoints, change auth posture, or revoke proxy access.
ADR-014's Copilot-proxy regression is a concrete recent example.
*Mitigants:* ADR-002 provider abstraction (we can swap),
heuristic offline fallback (we don't *require* a provider for
the basic flow), and the multi-provider testing that flowed from
ADR-014.

### Bargaining power of buyers — **N/A today; M at monetization**

We are pre-revenue. There are no buyers to negotiate with. If
monetization happens (BMC §8 hypothetical paths), buyer power
will be moderate: fork maintainers can self-host, run heuristic
fallback, or roll their own — switching cost is real but not
prohibitive. Buyers in the platform / DevX team tier (BMC §8
tertiary segment) will have higher leverage because they
evaluate vs internal builds and against entire.io.

### Threat of substitutes — **HIGH**

The substitutes ladder, weakest to strongest:

1. **Manual rebase / cherry-pick.** Always available. Works fine
   for low-volume forks. Fails at the t3code scale (51 commits
   behind upstream).
2. **Quilt / StGit / hg-MQ / gbp-pq for adjacent users.**
   Mature, battle-tested, but no AI in the conflict path. Lane-A
   substitutes only matter for the cohort that doesn't want AI
   reconcile.
3. **Regenerate-from-scratch with an agent (vanilla).** "Just
   ask the agent to re-implement the feature against the new
   upstream." Real user move that erodes the structured-intent
   pitch. *Counter:* replay determinism, recipe, and audit
   trail are forward-looking benefits the regenerate path does
   not produce.
4. **Cursor + Graphite "self-driving PR" (NEW — 2026-05).**
   Substantially stronger than tier 3: Cursor authors the change,
   Graphite stacks it, Graphite Chat reviews it, the merge queue
   ships it. For users with a *forward* PR workflow, this
   compresses the regenerate-with-agent loop into a polished
   commercial product. *Counter:* this stack solves
   feature-against-trunk (Lane B), not fork-against-upstream
   (Lane A). The fork-maintenance JTBD (see `personas.md`) is
   not what they're targeting today. But it is plausibly one
   roadmap pivot away — Cursor has the cash and the pattern of
   acquiring adjacent tooling. Watch.
5. **Abandon the fork** (upstream the changes, accept upstream
   defaults, or switch to a vendor-supported alternative).
   Strategically the most concerning long-term substitute: if
   upstream eats the customization need, we lose our user.

### Competitive rivalry — **MEDIUM-HIGH (was MEDIUM until 2026-05)**

Two-tier rivalry picture:

- **Direct rivals (our seam).** Lane C entire.io remains the
  only commercial system pitched directly at the
  fork-vs-upstream problem. Their roadmap is private. The
  dominant rivalry signal here is **time-to-market against
  entire.io** for the AI-assisted-reapply primitive — feature-
  by-feature competition with the open-source set is not the
  watch.
- **Adjacent rivals (their seam, encroaching).** Cursor +
  Graphite is not a direct rival today. They're a *very-well-
  funded adjacent platform with the pattern of acquiring
  workflow tooling.* The Dec 2025 Cursor-Graphite acquisition
  raised the rivalry verdict from MEDIUM to MEDIUM-HIGH because
  it created a credible buyer / builder for the next adjacent
  primitive — and fork-management is a one-acquisition pivot
  away.

The dominant rivalry signal is now two-pronged: time-to-market
vs entire.io *and* the corporate-development posture of the
AI-coding-tool incumbents (Cursor especially).

### Synthesis

| Force | Verdict | Top mitigant |
|---|---|---|
| New entrants | H | ADR-015 + PRD-tpatch-land raise the correctness / forwardable bar |
| Suppliers | H | ADR-002 provider abstraction + heuristic fallback |
| Buyers | N/A → M | Monetization is a deferred decision (BMC §8 Block 5) |
| Substitutes | H | Replay determinism + audit trail (our edge — §11) + OSS positioning vs the Cursor+Graphite bundle |
| Rivalry | **M-H (was M)** | Time-to-market vs entire.io + watch Cursor's M&A posture |

Two forces dominate planning: **supplier power** (LLM provider
lock-in / outages) and **substitute pressure** (regenerate-with-
agent eroding the structured-intent pitch — *especially* the new
Cursor+Graphite bundle compressing that loop into a polished
product). Strategy-Canvas moat axes 6–9 in §7 address substitute
pressure; ADR-002 + ADR-014 address supplier power. The two HIGH
forces are also the two with concrete, named tpatch decisions
countering them — which is how it should read.

---

## 5. SWOT (refreshed 2026-05-02)

Four-quadrant; internal × external × ours × theirs.

### Internal — Strengths (ours)

- **Provider-assisted 4-phase reconcile** (ADR-010). No prior-art
  system has AI in the conflict path.
- **Multi-format skill emission with parity guard.** 6 formats
  emitted from one source-of-truth. Validated by
  `assets/assets_test.go`.
- **Deterministic apply-recipe.** Validated operations + checksums
  + `base_commit` + `recipe-SHA`. Strictly richer than
  git-format-patch / DEP-3.
- **Lifecycle phases (analyze / define / explore / implement /
  apply / record / reconcile).** Structured intent — nobody else
  ships this.
- **Zero-deps Go binary.** Single executable, embedded assets, no
  runtime dependencies.
- **`Tpatch-Feature` interim trailer documented** (`docs/commits.md`)
  — forward-compatible with PRD-tpatch-land §3.4.

### Internal — Weaknesses (ours)

- **WP-001 §5.2 collision shape is shipping.** Re-verified
  2026-05-01: copilot-api 9/21 (43%), t3code 11/17 (65%). The
  collision is a present correctness bug, not a future risk.
- **No op log** (WP-001 §3.3). ADR-015 D3 names the direction but
  hasn't shipped.
- **No versioned patch history** (ADR-015 D2). `post-apply.patch`
  is overwritten in place on every reconcile-driven rewrite.
- **No upstream-lock validation.** copilot-api has an empty
  scaffold lock and `tpatch reconcile` does not yell about it.
- **Single-author tooling.** No documented multi-collaborator
  story. gbp-pq has been used by hundreds of Debian maintainers
  for ~18 years — that's a battle-test we haven't received.
- **No primary-public release yet.** v0.6.1 is internal milestones.

### External — Opportunities (theirs / market)

- **gbp-pq's `--time-machine N`** is a stealable primitive for
  `tpatch reconcile`. New PRD candidate; not in roadmap yet.
- **gbp-pq's DEP-3 header standard** is precedent we can cite for
  PRD-tpatch-land §3.4's four-trailer block — DEP-3 has been
  forwarded upstream by Debian for ~15 years. Validates trailer-
  based identity.
- **jj / hg-evolve concept maturity.** ADR-015 D1 + D3 imported
  the vocabulary; future PRDs can reuse it without re-deriving.
- **Coding-agent adoption rising.** Demand-side tailwind for the
  skill-emission moat.
- **PRD-record-collision-detection drafted.** Closes the WP-001
  §5.2 gap when shipped.
- **entire.io closed-source.** OSS positioning is a moat for the
  cohort that won't adopt a closed tool in their build pipeline.
- **Cursor+Graphite is also closed-source / commercial.** The
  OSS-positioning seam now widens to a two-against-many
  posture: tpatch is the only OSS option for any user / org with
  procurement objections to closed-source bundles in their
  build pipeline.
- **Graphite's auto-rebase as production-validated prior art.**
  Their `gt modify` recursive-rebase implementation is the
  most-deployed hg-evolve-on-Git in existence. Cite as
  prior art when ADR-015 D3 (op log) graduates to a PRD.

### External — Threats (theirs / market)

- **entire.io shipping a competing primitive first.** Their roadmap
  is private. They could land "AI-assisted patch reapply" before
  `tpatch land`.
- **Cursor or another AI-coding-tool incumbent acquiring
  fork-management capability.** Cursor acquired Graphite in
  Dec 2025 (~$300M+) and Supermaven before that. Pattern is
  established. The most direct path for them to enter our lane
  is acquisition, not in-house build — entire.io is the obvious
  target. *Counter:* acquisition triggers a 12–24-month
  integration period during which an OSS alternative has a
  market window.
- **GitHub Copilot proxy policy change.** Would force a smart-
  routing rewrite (ADR-014 work).
- **jj's Git compat improving.** If jj absorbs the
  "stable-id-on-Git" story natively, our identity-duality story
  weakens — though our reconcile / skill story remains.
- **Foundation-model price hikes.** Make reconcile expensive.
- **"Vibe coding" trend.** Erodes demand for structured intent.
  Counter: structured intent is what makes vibe-coded changes
  reviewable and forwardable upstream.
- **"Self-driving PR" framing taking mindshare.** Cursor +
  Graphite's pitch is *"agent writes the change, agent reviews
  it, merge queue ships it"* — a compelling frame for the
  forward-PR workflow that may bleed into how users frame the
  fork-maintenance workflow ("why can't an agent just do this
  for me too?"). *Counter:* lifecycle phases + structured
  intent are the answer for users who care about
  *forwardability* and *audit*, not just speed.
- **gbp-pq existing user base.** Debian-package maintainers have
  18 years of muscle memory in `gbp pq`. We're not pitching them
  yet, but if we ever do, the switching cost is real.

---

## 6. SMART (refreshed 2026-05-02)

Earlier formulation: "ship `tpatch land` by v0.7." With the WP-001
§5.2 re-verification numbers and the gbp-pq `--time-machine`
direction in hand, this refines:

> **By v0.7**, ship (a) `tpatch land`, (b) `tpatch record`
> collision-detection, (c) `tpatch reconcile` upstream-lock
> validation guard, such that **zero new collisions** are introduced
> in either case-study repo for any feature recorded post-v0.7
> (audit at v0.7 + 30 days), and **at least 50% of v0.6 collision-
> group features** are re-recorded with auto-base.

- **S** — Specific: three concrete shipping deliverables (`land`,
  `record`-with-collision-detection, `reconcile`-lock-guard).
- **M** — Measurable: zero new collisions; ≥50% legacy collision-
  group remediation. Both auditable on the case-study trees.
- **A** — Achievable: PRD-tpatch-land + PRD-record-collision-
  detection + PRD-record-auto-base are all drafted. Land is gated
  on the other two.
- **R** — Relevant: closes the largest known correctness gap
  (WP-001 §5.2, re-verified 2026-05-01).
- **T** — Time-bound: v0.7 release.

**Stretch (v0.8, gated on v0.7 ship):** add `--time-machine N` to
`tpatch reconcile` (gbp-pq import); land ADR-015 D2 (versioned
patches); land ADR-015 D3 minimal op log.

---

## 7. Strategy Canvas

Blue Ocean style. Each axis is a "factor of competition" relevant
in this lane. Scores 0–5 (0 = absent; 5 = best-in-class today).
Scoring is **directional**, not survey data. Disputes welcome.

### Axes (10)

1. **Patch identity stability** — survives rewrites.
2. **Versioned patch history** — prior generations recoverable.
3. **Cross-feature operation log** — repo-wide audit, undo-able.
4. **Refresh primitive named** — single-verb fold-edits-into-patch.
5. **Upstream re-apply intelligence** — survives upstream rebase.
6. **Provider / AI-assisted reconciliation** — model in the conflict path.
7. **Replay recipe (richer than format-patch)** — validated ops + checksums.
8. **Skill / agent emission** — multi-IDE / multi-agent surfaces.
9. **Lifecycle phases (intent)** — analyze / define / explore / etc.
10. **Cross-format breadth** — works outside one VCS / packaging system.

### Scoring

| Axis | Quilt | StGit | hg MQ | hg evolve | jj | gbp-pq | **tpatch (today)** | **tpatch (post ADR-015 D2+D3)** |
|---|---|---|---|---|---|---|---|---|
| 1. Patch identity stability | 3 | 4 | 3 | 5 | 5 | 4 | 4 | 5 |
| 2. Versioned patch history | 0 | 2 | 0 | 5 | 5 | 0 | 1 | 5 |
| 3. Cross-feature op log | 0 | 1 | 0 | 4 | 5 | 0 | 0 | 5 |
| 4. Refresh primitive named | 5 | 5 | 5 | 4 | 5 | 5 | 4 | 4 |
| 5. Upstream re-apply intelligence | 1 | 2 | 2 | 4 | 3 | 4 | 5 | 5 |
| 6. AI-assisted reconciliation | 0 | 0 | 0 | 0 | 0 | 0 | 5 | 5 |
| 7. Replay recipe | 0 | 0 | 0 | 0 | 0 | 0 | 5 | 5 |
| 8. Skill / agent emission | 0 | 0 | 0 | 0 | 0 | 0 | 5 | 5 |
| 9. Lifecycle phases | 0 | 0 | 0 | 0 | 0 | 0 | 5 | 5 |
| 10. Cross-format breadth | 4 | 1 | 1 | 1 | 1 | 1 | 4 | 4 |
| **Mean** | **1.3** | **1.5** | **1.1** | **2.3** | **2.4** | **1.4** | **3.8** | **4.8** |

### Reading the canvas

- **Axes 6, 7, 8, 9 are the value-curve divergence.** Every
  prior-art system scores 0; tpatch scores 5. These are the four
  factors where we're not just better — we're in a different
  category. They are what justify a separate tool existing.
- **Axes 2, 3 are the value-curve gap.** Today we score 1, 0.
  Post ADR-015 D2 + D3 we score 5, 5. These two ADR-deferred
  shipments are the highest-leverage closures available — they
  flip a "behind" axis to "ahead" cheaply.
- **Axis 5** (upstream re-apply intelligence) is currently 5 for
  us, 4 for both gbp-pq and hg-evolve. Stealing gbp-pq's
  `--time-machine` solidifies the lead but doesn't change our
  category.
- **Axis 4** (refresh) — gbp-pq, Quilt, StGit, MQ, jj all score 5;
  we score 4 because we have the sequence (Path B) but no single
  verb. Lowest-cost axis to flip: an ADR-015 D4 follow-up that
  ships `tpatch refresh` as sugar over `apply --mode done` +
  `record`.
- **Axis 10** (cross-format breadth) — Quilt scores 4 because it
  predates Git and works on any tarball. We score 4 because we
  work on any Git repo. jj/StGit/MQ/evolve/gbp-pq all score 1.
  This is a quiet strength worth marketing.

### Visual shape (ASCII fallback)

```
       Q  StG  MQ  Evo  jj  gbp  tpat  tpat+D2D3
1.       . .   .   *   *   .    .     *
2.       .  o  .   *   *   .    -     *
3.       .  -  .   o   *   .    .     *
4.       *  *  *   o   *   *    o     o
5.       -  -  -   o   -   o    *     *
6.       .  .  .   .   .   .    *     *      ← we own this row
7.       .  .  .   .   .   .    *     *      ← we own this row
8.       .  .  .   .   .   .    *     *      ← we own this row
9.       .  .  .   .   .   .    *     *      ← we own this row
10.      o  -  -   -   -   -    o     o
                                                Legend: . = 0, - = 1-2,
                                                        o = 3-4, * = 5
```

The four `*`-only rows (6–9) are the **moat**. They are also the
rows least likely to be replicated by Lane A competitors, because
they require simultaneous investment in (a) a provider abstraction
and (b) multi-format asset discipline — neither of which the patch-
management tradition has cared about.

---

## 8. Business Model Canvas

Osterwalder 9-block model. The Strategy Canvas in §7 answers
"where do we differentiate?"; the Business Model Canvas answers
"how do we operate?" They are complements — pairing them is the
point of having both.

### Spatial overview

| Block | One-line summary |
|---|---|
| **Key Partners** | Foundation-model providers, coding-agent ecosystems, Git/jj substrates, case-study upstreams. |
| **Key Activities** | Spec → PRD → ADR → implementation cycle; reconcile-correctness research; skill-emission upkeep; provider integration. |
| **Key Resources** | Codebase (zero-deps Go); 6-format skill assets w/ parity guard; doc corpus; provider abstraction; two real case-study repos. |
| **Value Propositions** | Survive upstream rebases; AI in conflict path; replay determinism; multi-agent skill emission; structured intent; single-binary distribution. |
| **Customer Relationships** | Self-service CLI; documentation-driven; agent-mediated (skill files); community-driven (when public). |
| **Channels** | GitHub source / releases; homebrew (planned); docs site (TBD); skill files inside coding agents (passive distribution). |
| **Customer Segments** | Fork maintainers (primary); AI-coding-tool ecosystem participants (secondary); platform / DevX teams w/ multi-fork portfolios (tertiary, latent). |
| **Cost Structure** | Engineering time today (no infra burn); future: inference / hosting / DevRel if monetized. |
| **Revenue Streams** | None today (OSS / pre-revenue). Hypothetical paths surfaced in detail below as deferred decisions. |

### Block-by-block

#### 1. Customer Segments

- **Primary — fork maintainers.** Devs / small teams maintaining
  forks of fast-moving OSS. Empirically validated by the two case-
  study repos (`tesseracode/copilot-api`, `tesseracode/t3code`).
  Pain point: every upstream rebase risks losing custom work.
- **Secondary — AI-coding-tool users with custom OSS forks.**
  Cursor / Windsurf / Copilot users whose tool needs to handle
  fork lifecycle, not just feature branches.
- **Tertiary (latent) — platform / DevX teams.** Larger orgs
  running multi-fork portfolios for internal builds. Not pursuing
  yet; mentioned because monetization paths in Block 5 target
  them.
- **Adjacent (out of focus) — Debian-style package maintainers.**
  gbp-pq's audience. Could be a future segment but explicitly not
  pursuing now (distribution-coupling assumptions diverge — see
  §10).

#### 2. Value Propositions

- **Survive upstream rebases without redoing manual work.**
  Provider-assisted 4-phase reconcile (ADR-010). Direct response
  to the t3code-style "we are 51 commits behind" problem.
- **AI in the conflict path.** Unique among Lane A competitors.
  Foundation-model leverage is the single biggest differentiator.
- **Replay determinism.** Apply-recipe + base_commit + per-op
  checksums. Stronger than `git format-patch` + DEP-3.
- **Multi-IDE / multi-agent skill emission.** 6 formats from one
  source-of-truth. As coding-agent adoption rises, this is the UX
  surface that scales for free.
- **Structured intent (lifecycle phases).** Analyze / define /
  explore / implement / apply / record / reconcile. Captures the
  *why*, not just the diff.
- **Zero-deps single binary.** No daemon, no servers, no managed
  service required.

#### 3. Channels

- **GitHub source distribution** (`tesseracode/tesserapatch`).
- **Direct binary download / homebrew** (planned).
- **Documentation as primary teaching surface** (`docs/` is
  load-bearing; skill files inside `assets/`).
- **Coding-agent integration as a passive distribution channel.**
  Skill files installed into Claude / Copilot / Copilot-prompt /
  Cursor / Windsurf / Generic mean the agent itself becomes the
  user's tpatch interface. This is unusual and worth tracking
  explicitly — see Block 4.
- **(Future) word-of-mouth in the fork-maintainer community.**

#### 4. Customer Relationships

- **Self-service.** CLI tool. No mandatory onboarding flow.
- **Documentation-driven.** SPEC, PRDs, ADRs, agent-as-provider
  docs and skill files teach by example.
- **Agent-mediated.** The most-novel relationship surface. Most
  users will never type `tpatch` directly — they'll tell their
  coding agent what they want, and the agent invokes us through
  the skill-file contract. The skill files *are* the UX for that
  cohort.
- **Community-driven** (when the project goes public). GitHub
  issues / discussions; no commercial support tier yet.

#### 5. Revenue Streams

- **Currently: none.** tpatch is pre-revenue / OSS. The roadmap
  has not committed to monetization.
- **Hypothetical paths** (surfaced to make the option-space
  explicit; not committed):
  - **Hosted reconcile service.** Managed inference for users
    without their own provider quota. Provider abstraction makes
    this offerable without architectural change.
  - **Organization-tier features.** Multi-collaborator op log
    (ADR-015 D7 territory if ever revisited), audit / compliance
    reporting, role-based access to reconcile decisions.
  - **Support / consulting** for enterprises maintaining large
    fork portfolios.
  - **"Repo-health monitoring as a service."** Periodic upstream-
    drift checks across N repos using the user's own provider key
    — we host the cron + dashboard, never the inference.
- **Strategic note:** the closed-source Lane C competitor
  (entire.io) demonstrates demand for a paid product in this
  space. Whether tpatch competes on monetization or stays free /
  OSS is an explicit deferred decision, not an assumed answer.
  The asymmetric cost model in Block 9 means we can defer the
  decision indefinitely without burning runway.

#### 6. Key Resources

- The `tpatch` codebase (zero-deps Go, single binary).
- Skill assets across 6 formats, parity-guarded by
  `assets/assets_test.go`.
- Documentation corpus: SPEC, ROADMAP, PRDs, ADRs, whitepapers,
  agent-as-provider docs.
- Two real-world case-study repos providing empirical validation
  (and exposing real bugs — see WP-001 §5.2).
- Provider abstraction (vendor-agnostic; protects against any
  single LLM provider's policy / pricing change).
- Brand / namespace (`tesseracode/tesserapatch`).

#### 7. Key Activities

- Spec → PRD → ADR → implementation cycle (the cadence in
  AGENTS.md). This *is* the development model; not just process.
- Reconcile-correctness research and validation against real
  case studies (e.g. WP-001 §5.2 audit, ADR-015 prior-art
  mapping).
- Skill-emission maintenance across 6 surfaces, validated each
  release by the parity guard.
- Provider integration upkeep (Copilot proxy routing — ADR-014;
  OpenAI-compat; Anthropic; future Responses-API).
- (Future) open-source community engagement: issue triage,
  contributor onboarding, release management.

#### 8. Key Partnerships

- **Foundation-model providers** (Anthropic, OpenAI, Google,
  GitHub Copilot) — through the provider abstraction (ADR-002).
  Vendor-agnostic by design; no single-vendor commitment.
- **Coding-agent ecosystems** (Claude, Copilot, Copilot-prompt,
  Cursor, Windsurf) — passive partnership through skill-format
  support. We don't have business relationships with these
  vendors; we ship into their open extension surfaces.
- **Underlying VCS substrate** (Git today; future jj compat —
  see PESTEL §Technological).
- **Case-study upstreams** (`ericc-ch/copilot-api`,
  `pingdotgg/t3code`) — one-directional partnership: we patch
  them, they don't know we exist. Healthy baseline; we contribute
  upstream when patches mature.
- **Distribution partners** (homebrew, GitHub releases) — TBD.

#### 9. Cost Structure

- **Today:** dominated by engineering time. **Zero infrastructure
  cost** — single-binary distribution, no servers, no managed
  inference, no telemetry.
- **Cost categories if monetized:**
  - Inference cost (managed reconcile service).
  - Hosting (docs site, dashboard if any).
  - DevRel / docs (when the project goes public).
  - Telemetry / analytics (currently none — privacy-by-default).
- **Strategic note:** the dominant-cost-is-engineering-time
  profile is a moat against entire.io's commercial model. Their
  cost base (servers + inference + sales) requires recurring
  revenue to sustain; ours doesn't. We can choose to monetize;
  they can't choose not to.

### Synthesis (BMC)

Three takeaways the BMC surfaces that the Strategy Canvas alone
doesn't:

1. **Asymmetric cost model.** No infra burn. tpatch can subsist
   on contributor time alone, indefinitely. The closed-source
   competitor cannot. This is a structural advantage that the
   Strategy Canvas's competitor-axis view doesn't make visible.
2. **Agent-mediated relationship surface is the dominant UX.**
   Most "customers" never type `tpatch` directly. They tell
   their coding agent what they want; the agent invokes us via
   skill files. This makes Strategy-Canvas axis 8 (skill /
   agent emission) not just a competitive lane but the primary
   product surface — different from typical CLI-tool BMC
   assumptions.
3. **Monetization is a deferred decision, not an absent one.**
   The four hypothetical paths in Block 5 are real and
   architecturally enabled by today's design (provider
   abstraction, multi-collaborator op log on the roadmap, etc.).
   Documenting them here preserves option-value without
   committing roadmap scope. The user / supervisor should
   revisit this block annually.

---

## 9. What we take

| From | What | Where it lands |
|---|---|---|
| Quilt | "Stack of patches with a series file" mental model — proven to scale to hundreds of patches. | Already implicit in our feature DAG (ADR-011). Confirms the model scales. No code change. |
| StGit | `commit` / `uncommit` as graduate-from-stack primitives. | ADR-015 D5 (unstack), gated on D2 + D3. |
| hg-MQ | **Negative lesson** — filename-as-id doesn't survive upstream rebases gracefully. | Confirms ADR-015 D1 / D2 design choice. |
| hg-evolve | Obsolescence-DAG concept. | ADR-015 D3 names the op-log direction; D7 explicitly defers distributed exchange. |
| jj | change-id / commit-id duality, op log, op restore. | ADR-015 D1, D3. Vocabulary lock. |
| **gbp-pq** | **`--time-machine N` for upstream reapply.** | **NEW PRD candidate** for v0.8: `tpatch reconcile --time-machine N` walks upstream history N commits backward to find a base where the patch applies. Single-repo heuristic, not distributed-obsolescence (D7 still defers that). |
| **gbp-pq** | **DEP-3 header precedent.** | Confirms the trailer-block direction in PRD-tpatch-land §3.4. DEP-3 has been forwarded upstream by Debian for ~15 years; this validates trailer-based identity as a forwardable convention. |
| **gbp-pq** | **Auto-detect "patches already applied upstream" via `git rebase`.** | Reinforces: tpatch reconcile phase-3 obsolescence check should leverage `git patch-id` more aggressively. Possible follow-up to PRD-record-collision-detection. |
| **gbp-pq** | **`Gbp-Pq: Name` per-patch tag survives renumbering.** | Per-patch precedent for ADR-015 D1 (slug stability). Cited as prior art in the next ADR-015 amendment. |
| **gbp-pq** | **`import` ↔ `export` round-trip as first-class commands.** | Validates ADR-015 D4 (Path B is the canonical refresh primitive). May motivate the `tpatch refresh` alias sooner rather than later. |
| git-gud / stk | Commit-trailer-based identity (`GG-ID`, `GG-Parent`, `.git/stacks/<name>.yaml`). | Prior art for PRD-tpatch-land §3.4's four-trailer block. No code change; cite when defending the trailer choice. |

## 10. What we don't take

| From | What | Why not |
|---|---|---|
| StGit | Patches-are-Git-commits architecture. | ADR-015 D4 explicitly rejects. Incompatible with `.tpatch/` artifact model — we keep patches as files outside main history so they can be inspected, reviewed, and committed by the user with full visibility. |
| hg-MQ | Filename-as-id. | Documented failure mode (deprecated 2019). ADR-015 D1 / D2 chose duality. |
| hg-evolve | Distributed obsolescence-marker exchange via push/pull. | ADR-015 D7 explicitly defers — single-repo op log is the floor. Distributed semantics are their own ADR if ever revisited. |
| jj | Working-copy-as-commit / no index. | ADR-015 D6 explicitly rejects. Incompatible with our reliance on Git's index + the user's existing `git stage` / `git commit` muscle memory in Path B. |
| **gbp-pq** | **Drop-and-recreate the patch-queue branch on every round-trip.** | ADR-015 D2 chose the opposite. gbp-pq makes this trade because the **integration branch is the truth** (Debian package source); tpatch makes the opposite trade because **the patches are the truth** (we don't own the user's host repo). |
| **gbp-pq** | **DEP-3 specifically** (Debian-format-bound fields like `Origin: upstream`, `Forwarded: <bug-url>`). | We need the **trailer concept**, not the Debian-specific fields. Our four-trailer block (PRD-tpatch-land §3.4) names different concerns. |
| **gbp-pq** | **Built on Quilt's `series` file.** | We use a feature DAG (ADR-011), not a linear series. Quilt's series can't express our hard / soft dependency model. |
| **gbp-pq** | **Coupled to Debian source-format `3.0 (quilt)`.** | We are format-agnostic — any Git repo. |
| Lane B (git-spr / ghstack / git-gud / stk) | Stack-of-PRs orchestration. | Different problem (your own work, going upstream). We solve fork-vs-upstream, not feature-branch-vs-trunk. |

## 11. Our edge — three things nobody else has

1. **Provider-assisted reconciliation.** No prior-art system in
   Lane A has a model in the conflict path. ADR-010 + ADR-002
   (provider abstraction) mean we ride the foundation-model market
   without commitment to any one vendor. Closed-source Lane C
   competitors — entire.io, and (post-Dec-2025 acquisition)
   Cursor+Graphite if it ever pivots into our seam — cannot match
   this *and* be OSS. The OSS posture is the moat against the
   commercial-bundle threat.

2. **Multi-format skill / agent emission with a parity guard.** Six
   formats (Claude / Copilot / Copilot-prompt / Cursor / Windsurf /
   Generic) emitted from one source-of-truth, validated by
   `assets/assets_test.go`. Nobody else even has one. As coding-
   agent adoption rises (PESTEL §Social), this is the moat that
   widens passively.

3. **Deterministic apply-recipe richer than `git format-patch`.**
   Validated operations + per-op checksums + `base_commit` +
   `recipe-SHA`. The four-trailer block in PRD-tpatch-land §3.4 is
   the surface artifact of this richer model. gbp-pq's
   `git format-patch` + DEP-3 is the closest analog from Lane A;
   it is strictly weaker because it carries diffs only, not
   replay semantics.

## 12. Where we are weakest right now

Honest accounting of axes where we score behind a Lane A
competitor today:

- **Versioned patch history (axis 2):** today we score 1; jj /
  hg-evolve / StGit reflog all score 5 / 5 / 2. ADR-015 D2 names
  the direction but hasn't shipped. **Highest-leverage gap.**
- **Cross-feature op log (axis 3):** today we score 0; jj scores 5,
  hg-evolve scores 4. ADR-015 D3 names the direction. Second-
  highest-leverage gap.
- **Refresh primitive named (axis 4):** today we score 4 (sequence,
  not single verb); Quilt / StGit / MQ / jj / gbp-pq all score 5.
  Cheapest fix: a `tpatch refresh` alias as sugar over `apply
  --mode done` + `record`.

All three gaps are already mapped in ADR-015 (D2, D3, D4). None
require new architectural decisions to close.

## 13. Cross-references

- **ADR-015** — locks the framework that this doc populates with
  scores. Conceptual model → strategy view.
- **WP-001 §5.2** — collision-shape claims, re-verified 2026-05-01.
  Justifies SMART §5's measurable outcome.
- **PRD-tpatch-land** — §3.4 four-trailer block validated by gbp-pq
  + git-gud trailer precedents (§9).
- **PRD-record-collision-detection** — gates SMART §5.
- **PRD-record-auto-base** — gates SMART §5.
- **`docs/commits.md`** — interim `Tpatch-Feature` trailer
  convention + Path B workflow (the refresh primitive in §7 axis 4).

## 14. Disputes

(empty — open a `### <axis> — <date>` block here when a scoring is
contested. Resolve via discussion, fold the resolution into the
canvas, mark closed.)

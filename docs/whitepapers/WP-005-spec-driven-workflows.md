# WP-005 — Spec-driven Workflows: OpenSpec, GitHub Spec, and tpatch

**Status**: Graduated (2026-08-13) — first graduating PRD:
[PRD-artifact-validation-and-provenance](../prds/PRD-artifact-validation-and-provenance.md)
**Authors**: R58, CO76
**Started**: 2026-06-25
**Turn log**: [WP-005-spec-driven-workflows.turns.md](./WP-005-spec-driven-workflows.turns.md)
**Related**:
- [PRD artifact validation and provenance](../prds/PRD-artifact-validation-and-provenance.md) — graduates Agreed items 4–9; `PRD-prepare-intent-bundle` stays blocked until it is accepted
- [WP-001 Feature-slice gap](./WP-001-feature-slice-gap.md)
- [WP-002 Capture and metadata foundation](./WP-002-capture-and-metadata-foundation.md)
- [WP-003 Reconcile safety and middle-pass](./WP-003-reconcile-safety-and-middle-pass.md)
- [Active clusters](../CLUSTERS.md)
- [PRD tpatch land](../prds/PRD-tpatch-land.md)
- [PRD skill doc strategy](../prds/PRD-skill-doc-strategy.md)
- [PRD feature patch identity metadata](../prds/PRD-feature-patch-identity-metadata.md)
- [Competitive landscape](../market-research/competitive-landscape.md)
- [Personas](../market-research/personas.md)

## 1. Context *(R58)*

This research exists because OpenSpec and GitHub Spec Kit both frame the spec as
the durable human-agent contract. That sounds close to tpatch's own direction:
capture why a patch exists, preserve intent in versioned artifacts, make agent
work reviewable, and survive upstream change without losing the original
reasoning. The product question is narrower than "should tpatch adopt
spec-driven development?": which product/process primitives improve tpatch's
existing workflow without adding duplicate ceremony?

The product framing is the same Common JTBD tpatch already uses: when
customizations live on top of an upstream the user does not control, the tool
should capture why each patch exists, survive rebases, prove correctness to
reviewers, and stay compatible with `git log` / `git review` habits
(`docs/market-research/personas.md:239-247`). OpenSpec and Spec Kit are useful
comparison points because they also try to turn vague chat into durable,
reviewable artifacts before implementation.

This is exploratory paper research only. It does not create PRDs, approve a new
lifecycle, or authorize implementation.

This paper primarily evaluates spec-driven workflow ideas for the development of
tpatch itself. Productizing these ideas for downstream tpatch users is considered
only in §6.2. Turn 2 pressure-tests one bounded downstream primitive,
`tpatch prepare`, but authorizes planning only — not implementation.

## Agreed — Turns 2–3

The second council pass reached the following durable conclusions:

1. **Do not adopt OpenSpec or Spec Kit wholesale.** tpatch's core remains patch
   identity, replay and reconciliation against upstream. Spec-driven artifacts
   support that product; they do not replace it.
2. **Internal SDD stays optional and lightweight.** Behavior-delta appendices,
   cross-artifact consistency passes, task companions and parallel labels may
   help complex PRDs, but none becomes mandatory ceremony for small changes.
3. **Downstream SDD is encouraged, never enforced.** Durable feature intent is
   valuable, but tpatch users are not required to adopt a prescribed
   development methodology in order to maintain patches.
4. **`prepare` belongs in the product, but validation precedes
   orchestration.** The current manual Markdown phases check presence but can
   accept empty content (`internal/store/manual.go:25-31,45-79`), and Path A
   analysis writes `artifacts/analysis.json` while Path B does not
   (`internal/workflow/workflow.go:88-121`). A mutating bundle must not amplify
   those asymmetries.
5. **`prepare --check` is the first product slice.** It reports each canonical
   intent artifact independently and must say explicitly that structural
   presence does not certify semantic quality. Existing `defined` features are
   reported, not retroactively invalidated. Provenance is `unknown` unless an
   accepted persistent representation proves it; the check must never infer
   provenance from the overwrite-prone free-text `status.json.notes`.
6. **No new lifecycle state.** A successful preparation ends at the existing
   `defined` state; completeness remains an artifact-level fact. Invoking the
   optional full-bundle command must not make exploration mandatory for every
   trivial feature.
7. **Mutating preparation is gated.** `--manual` reuses validation extracted
   from the existing phase primitives; it must not naively invoke their
   incremental writers. `--regenerate` waits for non-destructive overwrite and
   provenance rules. The all-or-nothing publication unit includes the three
   canonical Markdown files, structured sidecars and the final `status.json`
   transition. A provider failure must expose either the complete prior set or
   the complete new set, never a half-generated bundle.
8. **Two PRDs graduate from this paper, in order.**
   `PRD-artifact-validation-and-provenance` defines truthful inspection,
   parity, migration and provenance. `PRD-prepare-intent-bundle` is blocked on
   that contract and defines orchestration. The first PRD may evaluate and
   propose a persistent representation; no pre-emptive ADR is required, but an
   ADR becomes mandatory when that representation is selected, before the
   contract is accepted for implementation.
9. **Existing routing remains compatible by default.** Slice 1 is advisory:
   richer classifications do not reroute `next`, fail `cycle`, or invalidate
   existing `defined` features. A later PRD may reuse the shared classifier
   only after enumerating every intended routing delta.

Items 1–8 originate in Turn 2's five independent advisors, five anonymous peer
reviews and chairman synthesis, as clarified by Turn 3 review. Item 9 is the
Turn 3 routing-compatibility correction. The council split on "ship now"
versus "reject as sugar"; peer review converged on the validation-first middle
position. See the append-only turn log for the disagreement and disposition.

## 2. Current tpatch process baseline *(R58)*

tpatch already has a layered process that separates exploration, proposal,
decision, implementation, review, and shipped state:

| Surface | Current role | Evidence |
|---|---|---|
| Whitepapers | Exploratory, multi-agent problem restatements between PRDs and ADRs; they either graduate into PRDs/ADRs or close as no action. | `docs/whitepapers/README.md:3-19`, `docs/whitepapers/README.md:104-112` |
| Turn logs | Append-only cross-agent collaboration logs with stable agent IDs and repo-file cites for factual claims. | `docs/whitepapers/README.md:66-102` |
| PRDs | Specific feature proposals; PRD status is the document source of truth, while roadmap and supervisor log own implementation status. | `docs/prds/README.md:1-8` |
| Claims audits | Recent PRDs often enumerate load-bearing current-behavior claims with file/line evidence before proposing change. | `docs/prds/PRD-tpatch-land.md:36-43`, `docs/prds/PRD-skill-doc-strategy.md:25-55` |
| ADRs | Locked decisions after design work; whitepapers and PRDs can graduate into ADR-backed implementation. | `docs/whitepapers/README.md:6-10` |
| CLUSTERS.md | Live dashboard for in-flight PRD clusters: Exploring, Drafting, Accepted, Implementing, Shipped, Closed. It is not the historical log or decision audit trail. | `docs/CLUSTERS.md:1-24` |
| Supervisor LOG | Review/decision audit trail with high-signal findings, severity, PRD section references, validation gate results, and action taken. | `docs/supervisor/LOG.md:1-18`, `docs/supervisor/LOG.md:50-70` |

The strongest current tpatch pattern is not "big design up front." It is:

```text
whitepaper gap study -> PRD cluster -> ADR where needed -> implementation wave
-> review against PRD acceptance contracts -> LOG decision -> cluster state
```

That resembles spec-driven workflows in spirit, but tpatch's artifacts are
project-process artifacts, not per-feature generated implementation folders. The
current process is strong on review evidence and historical traceability; it is
weaker on lightweight per-feature task plans that remain next to the spec during
implementation.

## 3. OpenSpec brief *(R58)*

### 3.1 What problem it solves

OpenSpec is an AI-coding workflow for agreeing with the agent before code is
written. Its README says AI assistants become unpredictable when requirements
live only in chat, and OpenSpec adds a lightweight spec layer so human and AI
align on what to build first. Official source: OpenSpec README, accessed
2026-06-25, <https://github.com/Fission-AI/OpenSpec>.

Its distinctive emphasis is brownfield-friendly change accumulation: official
docs say users should not document the whole existing codebase first; specs grow
one change at a time. Official source: OpenSpec "Using OpenSpec in an Existing
Project", accessed 2026-06-25,
<https://raw.githubusercontent.com/Fission-AI/OpenSpec/main/docs/existing-projects.md>.

### 3.2 Core concepts / primitives

OpenSpec's five primitives are:

1. `openspec/specs/` as the current behavioral source of truth.
2. `openspec/changes/<name>/` as one unit of work.
3. Delta specs using `ADDED`, `MODIFIED`, and `REMOVED` requirements instead of
   full-system rewrites.
4. Layered artifacts: proposal -> specs -> design -> tasks -> implement.
5. Archive as the operation that folds deltas into durable specs.

Official source: OpenSpec "Core Concepts at a Glance", accessed 2026-06-25,
<https://raw.githubusercontent.com/Fission-AI/OpenSpec/main/docs/overview.md>.

### 3.3 Files or artifacts it creates

After `openspec init`, the documented project shape is:

```text
openspec/
├── specs/
├── changes/<change-name>/
│   ├── proposal.md
│   ├── design.md
│   ├── tasks.md
│   └── specs/<domain>/spec.md
└── config.yaml
```

`proposal.md` captures intent, scope, and approach; delta `specs/` capture
requirements and scenarios; `design.md` captures technical approach; `tasks.md`
is the implementation checklist. Official source: OpenSpec "Getting Started",
accessed 2026-06-25,
<https://raw.githubusercontent.com/Fission-AI/OpenSpec/main/docs/getting-started.md>.

OPSX also externalizes workflow instructions through schemas and templates
rather than hardcoded TypeScript. Official source: OpenSpec "OPSX Workflow",
accessed 2026-06-25,
<https://raw.githubusercontent.com/Fission-AI/OpenSpec/main/docs/opsx.md>.

### 3.4 Lifecycle

The default loop is:

```text
/opsx:explore -> /opsx:propose -> /opsx:apply -> /opsx:sync -> /opsx:archive
```

`/opsx:explore` is optional and creates no artifacts; `/opsx:propose` creates a
change folder and planning artifacts; `/opsx:apply` works through tasks and
checks them off; `/opsx:sync` merges delta specs; `/opsx:archive` files the
completed change. Expanded commands add `/opsx:new`, `/opsx:continue`,
`/opsx:ff`, `/opsx:verify`, `/opsx:onboard`, and `/opsx:bulk-archive`.
Official sources: OpenSpec "Getting Started", "Commands", and "OPSX Workflow",
accessed 2026-06-25,
<https://raw.githubusercontent.com/Fission-AI/OpenSpec/main/docs/getting-started.md>,
<https://raw.githubusercontent.com/Fission-AI/OpenSpec/main/docs/commands.md>,
<https://raw.githubusercontent.com/Fission-AI/OpenSpec/main/docs/opsx.md>.

The important nuance is that OPSX now says these are actions, not rigid phases:
artifacts can be updated during implementation, and dependencies are enablers
rather than gates. Official source: OpenSpec "OPSX Workflow", accessed
2026-06-25,
<https://raw.githubusercontent.com/Fission-AI/OpenSpec/main/docs/opsx.md>.

### 3.5 How it preserves intent and handles change over time

Intent is preserved in two layers:

- active change folders preserve the "why/what/how/tasks" bundle while work is
  in flight;
- archived deltas merge into `openspec/specs/`, so behavior accumulates as
  durable specs rather than remaining in chat history.

Change over time is modeled as deltas. `ADDED` requirements append, `MODIFIED`
requirements replace prior behavior, and `REMOVED` requirements delete behavior
from the main spec at archive. Official source: OpenSpec "Getting Started",
accessed 2026-06-25,
<https://raw.githubusercontent.com/Fission-AI/OpenSpec/main/docs/getting-started.md>.

This is the part most relevant to tpatch: current behavior is not backfilled in
one sweep; the durable knowledge grows only where real changes occur. That
matches tpatch's preference for evidence-backed, incremental artifacts over
schema expansion.

### 3.6 How it helps agents

OpenSpec helps agents by keeping the agent on a shared artifact bundle instead
of free-floating chat. `tasks.md` gives a resumable checklist, delta specs give
behavioral acceptance context, and `/opsx:verify` checks completeness,
correctness, and coherence against artifacts. Official source: OpenSpec
"Commands", accessed 2026-06-25,
<https://raw.githubusercontent.com/Fission-AI/OpenSpec/main/docs/commands.md>.

It also ships skill/command files for many AI tools. The GitHub Copilot note is
important for this repo: OpenSpec says GitHub Copilot prompt files are recognized
as custom slash commands in IDE extensions, but Copilot CLI does not currently
consume `.github/prompts/*.prompt.md` directly. Official source: OpenSpec
"Supported Tools", accessed 2026-06-25,
<https://raw.githubusercontent.com/Fission-AI/OpenSpec/main/docs/supported-tools.md>.

### 3.7 Intentionally out of scope / process vs enforcement

OpenSpec intentionally avoids a mandatory full-codebase import and avoids rigid
phase gates. Artifacts are Markdown and can be edited by hand or by the AI at
any time. Official source: OpenSpec "Editing & Iterating on a Change", accessed
2026-06-25,
<https://raw.githubusercontent.com/Fission-AI/OpenSpec/main/docs/editing-changes.md>.

Technical enforcement appears strongest around file layout, CLI validation,
workflow schema/dependency awareness, and archive/sync mechanics. Human review
discipline is still a convention: the tool can create artifacts and validate
format/coherence, but it cannot prove the human actually reviewed the spec
line-by-line.

### 3.8 Visible failure modes / tradeoffs

- Lightweight artifacts can sprawl if "actions not phases" becomes "no review
  discipline."
- `tasks.md` checkboxes are useful for agents but can become a second source of
  truth if not archived or reconciled with specs.
- Brownfield delta specs are excellent for changed slices but may leave critical
  unchanged behavior undocumented for a long time.
- Copilot CLI users cannot assume OpenSpec's generated GitHub Copilot prompt
  files will be loaded directly.
- OpenSpec's own README positions Spec Kit as heavier and OpenSpec as lighter;
  that is an official project comparison, not a neutral benchmark. Official
  source: OpenSpec README, accessed 2026-06-25,
  <https://github.com/Fission-AI/OpenSpec>.

## 4. GitHub Spec brief *(R58)*

### 4.1 Disambiguation

"GitHub Spec" can refer informally to several things: GitHub-flavored
specifications, GitHub Issues as specs, or GitHub's official open-source Spec
Kit. For this research, I reviewed **GitHub Spec Kit** because the official
GitHub blog names "Spec Kit" as the open-source toolkit for spec-driven
development with GitHub Copilot, Claude Code, and Gemini CLI. Official source:
GitHub Blog, accessed 2026-06-25,
<https://github.blog/ai-and-ml/generative-ai/spec-driven-development-with-ai-get-started-with-a-new-open-source-toolkit/>.

### 4.2 What problem it solves

Spec Kit addresses the same root problem as OpenSpec: vague prompts cause agents
to guess. GitHub's blog says specs become the shared source of truth, and each
phase should be validated before moving on. Official source: GitHub Blog,
accessed 2026-06-25,
<https://github.blog/ai-and-ml/generative-ai/spec-driven-development-with-ai-get-started-with-a-new-open-source-toolkit/>.

Its repo describes Spec-Driven Development as making specifications executable:
specifications become the source that generates implementation rather than
discarded scaffolding. Official source: GitHub Spec Kit README and methodology,
accessed 2026-06-25,
<https://github.com/github/spec-kit>,
<https://raw.githubusercontent.com/github/spec-kit/main/spec-driven.md>.

### 4.3 Core concepts / primitives

Spec Kit's workflow centers on:

1. A project constitution (`.specify/memory/constitution.md`) that encodes
   project principles and governance.
2. `/speckit.specify` for user-centered "what/why" specs.
3. `/speckit.clarify`, `/speckit.checklist`, and `/speckit.analyze` as quality
   gates for ambiguity and cross-artifact consistency.
4. `/speckit.plan` for technical translation and constitutional compliance.
5. `/speckit.tasks` for user-story-oriented task breakdown.
6. `/speckit.implement` for execution.

Official sources: GitHub Spec Kit README and Quick Start, accessed 2026-06-25,
<https://github.com/github/spec-kit>,
<https://raw.githubusercontent.com/github/spec-kit/main/docs/quickstart.md>.

### 4.4 Files or artifacts it creates

Spec Kit initialization creates `.specify/` templates, scripts, and memory, and
feature work creates `specs/<numbered-feature>/` folders. The documented
feature directory can include:

```text
specs/<feature>/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
└── tasks.md
```

Official sources: GitHub Spec Kit README, plan template, and Quick Start,
accessed 2026-06-25,
<https://github.com/github/spec-kit>,
<https://raw.githubusercontent.com/github/spec-kit/main/templates/plan-template.md>,
<https://raw.githubusercontent.com/github/spec-kit/main/docs/quickstart.md>.

The spec template requires prioritized, independently testable user stories,
acceptance scenarios, edge cases, functional requirements, success criteria, and
assumptions. Official source: Spec Kit spec template, accessed 2026-06-25,
<https://raw.githubusercontent.com/github/spec-kit/main/templates/spec-template.md>.

The tasks template groups work by setup, foundational prerequisites, user
stories, dependencies, parallel opportunities, and MVP-first implementation
strategy. Official source: Spec Kit tasks template, accessed 2026-06-25,
<https://raw.githubusercontent.com/github/spec-kit/main/templates/tasks-template.md>.

### 4.5 Lifecycle

The lean path is:

```text
/speckit.specify -> /speckit.plan -> /speckit.tasks -> /speckit.implement
```

For production features or meaningful ambiguity, the recommended path is:

```text
/speckit.constitution -> /speckit.specify -> /speckit.clarify
-> /speckit.plan -> /speckit.checklist -> /speckit.tasks
-> /speckit.analyze -> /speckit.implement
```

Official source: GitHub Spec Kit Quick Start, accessed 2026-06-25,
<https://raw.githubusercontent.com/github/spec-kit/main/docs/quickstart.md>.

The GitHub blog summarizes the public process as specify, plan, tasks, and
implement, with explicit checkpoints where the human verifies and refines the
AI-generated artifacts. Official source: GitHub Blog, accessed 2026-06-25,
<https://github.blog/ai-and-ml/generative-ai/spec-driven-development-with-ai-get-started-with-a-new-open-source-toolkit/>.

### 4.6 How it preserves intent and handles change over time

Spec Kit preserves intent by splitting stable product intent (`spec.md`) from
technical implementation choices (`plan.md`) and executable work breakdown
(`tasks.md`). Its methodology says maintaining software means evolving
specifications, and requirement changes should update specs and regenerate
affected plans. Official source: Spec Kit methodology, accessed 2026-06-25,
<https://raw.githubusercontent.com/github/spec-kit/main/spec-driven.md>.

It also uses numbered feature branches/directories, so feature intent is tied to
a branch and `specs/<feature>/` folder. Official source: Spec Kit README,
accessed 2026-06-25, <https://github.com/github/spec-kit>.

### 4.7 How it helps agents

Spec Kit constrains agent output through templates, explicit uncertainty
markers, constitution checks, and task derivation. The methodology says
templates prevent premature implementation details, force `[NEEDS
CLARIFICATION]` markers, use checklists as "unit tests" for specs, and enforce
architectural gates through the plan. Official source: Spec Kit methodology,
accessed 2026-06-25,
<https://raw.githubusercontent.com/github/spec-kit/main/spec-driven.md>.

This is useful for agents because every command narrows the next command's input:
spec -> plan -> tasks -> implementation. The tasks template also marks
parallelizable tasks with `[P]`, which is directly relevant to multi-agent work.
Official source: Spec Kit tasks template, accessed 2026-06-25,
<https://raw.githubusercontent.com/github/spec-kit/main/templates/tasks-template.md>.

### 4.8 Intentionally out of scope / process vs enforcement

Spec Kit is a toolkit and methodology, not a guarantee that generated code is
correct. The GitHub blog emphasizes the human role as verification at every
phase: the AI generates artifacts, but the developer ensures they are right.
Official source: GitHub Blog, accessed 2026-06-25,
<https://github.blog/ai-and-ml/generative-ai/spec-driven-development-with-ai-get-started-with-a-new-open-source-toolkit/>.

The technical enforcement is heavier than OpenSpec: templates, scripts,
constitution files, CLI initialization, branch/feature structure, and analyze
commands. But the decisive review still remains human/process enforcement.

### 4.9 Visible failure modes / tradeoffs

- The artifact set can be too heavy for small tpatch changes.
- Generated plans may create a second planning hierarchy alongside existing
  PRDs, ADRs, handoffs, and supervisor LOG.
- Constitution gates are powerful, but tpatch already has repo-level
  instructions, ADRs, and review checklists; duplicating them could create
  drift.
- Spec Kit's examples and templates emphasize building feature implementations
  from specs, plans, contracts, and tasks; that is adjacent but not identical to
  tpatch's patch-stack and upstream-reconcile problem. Spec Kit does document
  iterative enhancement and brownfield modernization as target phases, so this is
  an orientation mismatch rather than a strict limitation. Official sources:
  Spec Kit README and Quick Start, accessed 2026-06-25,
  <https://github.com/github/spec-kit>,
  <https://raw.githubusercontent.com/github/spec-kit/main/docs/quickstart.md>.

## 5. Comparison table *(R58)*

| Axis | OpenSpec | GitHub Spec Kit | Current tpatch |
|---|---|---|---|
| Primary unit | `openspec/changes/<name>/` | `specs/<numbered-feature>/` + branch | feature slug, PRD cluster, whitepaper, ADR |
| Durable truth | `openspec/specs/` after archive | specification as source; code serves spec | `.tpatch/` feature artifacts + docs/PRDs/ADRs + Git trailers |
| Planning artifacts | proposal, delta specs, design, tasks | spec, plan, research, data model, contracts, quickstart, tasks | whitepaper, PRD, ADR, handoff, LOG, per-feature request/spec/exploration in tpatch state |
| Lifecycle style | Fluid actions, dependencies as enablers | More phase-gated; clarify/checklist/analyze gates | Cyclic implementation/review with supervisor decision |
| Brownfield stance | Do not backfill whole repo; grow specs one change at a time | Supports iterative enhancement and modernization, but templates are broader | Built for fork-vs-upstream patch maintenance |
| Intent preservation | Archive merges behavior deltas into main specs | Spec/plan/tasks remain in feature folder and branch | Feature slug, canonical patch, manifests, PRDs, ADRs, LOG, trailers |
| Agent aid | Slash commands, skills, task checklist, verify | Templates, constitution, checks, task parallelization | Skills, deterministic recipes, handoff, review checklist, evidence manifests |
| Validation | `openspec validate`, `/opsx:verify`, human review | checklist/analyze, constitution checks, human review | tests/builds, PRD acceptance sweeps, supervisor LOG |
| Main risk | Too little gate discipline | Too much artifact/process weight | Process artifacts are strong, but implementation task traceability is uneven |

## 6. What tpatch could borrow *(R58)*

No code yet. These are candidate primitives for paper or PRD exploration only.

### 6.1 Internal tpatch development process ideas

These ideas target how this repository plans, reviews, and implements tpatch
itself. They are the immediate focus of WP-005.

#### Small ideas

1. **Optional PRD companion tasks file.** For implementation PRDs with many
   acceptance criteria, add `docs/prds/<slug>.tasks.md` as a generated/reviewed
   checklist. Borrow from both OpenSpec `tasks.md` and Spec Kit's task template,
   but keep it optional.
2. **Explore/propose wording for whitepaper intake.** tpatch already uses
   whitepapers, but WP intake could explicitly say "explore current behavior
   before proposing deltas," matching OpenSpec's best brownfield habit.
3. **Archive discipline for paper deltas.** Borrow OpenSpec's idea that deltas
   fold into durable truth, but map it to tpatch docs: when a whitepaper
   graduates, explicitly list which current docs/specs/PRDs receive the durable
   knowledge and which are historical only.

#### Medium ideas

1. **Spec delta appendix for PRDs.** Add an optional section that says:
   `ADDED / MODIFIED / REMOVED` behavior against current tpatch docs. This may
   reduce PRD ambiguity without adopting OpenSpec's full folder model.
2. **Cross-artifact consistency pass.** Before review, run a paper-only
   checklist modeled on Spec Kit `/speckit.analyze`: do PRD goals, non-goals,
   acceptance criteria, tests, and ADR gates agree?
3. **Task parallelization labels.** For implementation waves, mark tasks as
   parallel-safe or blocking using a small convention like Spec Kit's `[P]`.

### 6.2 Product-facing downstream tpatch ideas

These ideas would affect users who run tpatch in their own target repositories.
Turn 2 concludes that this paper is sufficient prior art; a second whitepaper
is not needed for the bounded preparation seam.

1. **Truthful intent-artifact inspection.** Before orchestration, define
   deterministic structural states for the existing canonical artifacts:
   `analysis.md`, `spec.md`, `exploration.md`, and Path A's structured analysis
   sidecar. The check must distinguish absence and structural thinness without
   claiming semantic quality. Source provenance remains `unknown` until an
   accepted representation proves it.
2. **Optional intent-bundle preparation.** After the inspection contract is
   accepted, a future `tpatch prepare` may compose the existing Path A/Path B
   phases and stop at `defined`. It is an opt-in convenience, not a gate on all
   feature work.
3. **Earlier proposal/tasks bundle idea — deferred.** Do not add new
   `proposal.md` or `tasks.md` artifacts for downstream users before the
   existing intent artifacts are trustworthy and Path A/B are coherent.
4. **Project constitution mapping — deferred.** Spec Kit's constitution
   resembles tpatch's `SPEC.md`, `CLAUDE.md`, `AGENTS.md`, ADRs and skill
   assets. Duplicating them remains unjustified.

**Existing-primitives pre-flight.**

- Individual `analyze|define|explore --manual` commands adopt one canonical
  artifact at a time, but provide no bundle-wide read-only report or
  all-or-nothing publication (`internal/store/manual.go:25-31,45-79`).
- `cycle` runs analyze → define → explore → implement → apply → record. Its
  `--skip-execute` stops only after recipe generation, and interactive refusal
  after explore is not a deterministic batch or Path B completion contract
  (`internal/cli/phase2.go:26-145`).
- `next` emits one logical action and currently uses raw `exploration.md`
  presence to distinguish two `defined` substates; it neither validates the
  whole intent set nor publishes it (`internal/cli/phase2.go:409-466`).

Therefore `prepare` is not merely an alias for an existing stop point. The
PRD must still reuse shared validation rather than duplicate phase semantics.

## 7. What tpatch should not borrow *(R58)*

1. **Do not rewrite tpatch around OpenSpec or Spec Kit.** tpatch's core problem
   is patch identity, replay, and reconcile against upstream. Spec-driven
   artifacts are supporting process, not the product substrate.
2. **Do not add mandatory feature folders for every tiny change.** tpatch already
   has PRDs, ADRs, whitepapers, handoff, LOG, and `.tpatch/` feature artifacts.
   Mandatory extra Markdown would slow trivial fixes.
3. **Do not duplicate constitution/governance.** Spec Kit's constitution is
   valuable, but tpatch already has load-bearing governance in `SPEC.md`,
   `CLAUDE.md`, `AGENTS.md`, ADRs, and review checklists.
4. **Do not treat generated tasks as authority over PRD acceptance criteria.**
   Tasks are execution aids. In tpatch, PRDs/ADRs and tests must remain the
   contract.
5. **Do not assume Copilot CLI can consume IDE prompt files.** OpenSpec's own
   tool docs say GitHub Copilot prompt files work in IDE extensions, not
   Copilot CLI.
6. **Do not mutate broker-owned cluster state from experiments.** `CLUSTERS.md`
   remains broker/supervisor-owned; task-companion experiments must not edit it
   unless the broker explicitly asks.

## 8. Candidate experiments *(R58)*

1. **Paper-only task companion experiment.** Choose one future implementation
   PRD with more than eight acceptance criteria and draft
   `docs/prds/<slug>.tasks.md` without changing code. Reviewer checks whether it
   made implementation/review easier. Trigger/owner: the broker or supervisor
   can request the companion when a PRD has more than eight acceptance criteria,
   multiple waves, or cross-ADR gates.
2. **PRD delta appendix experiment.** Add an optional `Behavior delta` appendix
   to one exploratory PRD: `ADDED`, `MODIFIED`, `REMOVED`, with repo-file cites.
3. **WP graduation map.** For the next whitepaper that graduates, require a
   small table: "durable knowledge moves to PRD/ADR/doc X; historical evidence
   remains in WP Y." This borrows OpenSpec archive semantics without new tooling.
4. **Analyze checklist.** Before the next PRD review, ask a review agent to run a
   Spec-Kit-inspired cross-artifact pass: claims audit present, non-goals aligned
   with acceptance criteria, test obligations enumerate all display/JSON/privacy
   contracts. This mirrors recent supervisor findings in `docs/supervisor/LOG.md:54-67`.

All four experiments are internal paper/process experiments. They must not
mutate `CLUSTERS.md`; cluster-state updates remain broker/supervisor-owned.

Turn 2 adds a separate **planning sequence**, not an implementation
experiment:

1. draft and review `PRD-artifact-validation-and-provenance`;
2. only after that contract is accepted, draft/review the blocked
   `PRD-prepare-intent-bundle`;
3. create an ADR when a PRD selects a persistent provenance/publication
   representation, before accepting that representation for implementation;
4. add prepare to the implementation roadmap only after its prerequisite PRD
   is accepted.

## 9. Open questions *(R58)*

1. Should tpatch add optional tasks companions only to PRDs, or also to
   whitepaper graduations?
2. Is OpenSpec's `ADDED/MODIFIED/REMOVED` delta vocabulary useful for PRDs, or
   would it conflict with tpatch's current goals/non-goals/acceptance style?
3. Would a "constitution" artifact clarify tpatch's agent instructions, or would
   it duplicate `SPEC.md`, `CLAUDE.md`, and `AGENTS.md`?
4. Where should implementation task traceability live: tracked docs, `.tpatch/`
   feature artifacts, or transient handoff files?
5. What threshold justifies additional process? Candidate: only PRDs with more
   than N acceptance criteria, multiple implementation waves, or cross-artifact
   ADR gates.
6. Which deterministic structural states can `prepare --check` report without
   implying semantic document quality?
7. Should per-artifact Path A/Path B provenance use an existing field or a new
   manifest, and which choice requires an ADR? Until accepted, how is
   `provenance: unknown` rendered?
8. How should a provider-generated bundle stage and atomically publish
   Markdown, sidecars and `status.json` so a mid-sequence failure cannot expose
   a half-generated intent set?
9. Does slice 1 change the mutating `analyze|define|explore --manual` gate or
   remain report-only? How do existing `defined` and trivial features remain
   valid, and which individual-command / `next` / `cycle` outcomes are
   explicitly unchanged?

## 10. References *(R58)*

### Repo references

| Claim | Citation |
|---|---|
| Whitepapers are exploratory docs between PRDs and ADRs and do not authorize implementation. | `docs/whitepapers/README.md:3-19` |
| Whitepapers use append-only turn logs with stable agent IDs and file/line cites. | `docs/whitepapers/README.md:66-102` |
| Whitepapers graduate to PRDs/ADRs or close as no action. | `docs/whitepapers/README.md:104-112` |
| PRD document status is source of truth; roadmap and supervisor LOG own implementation status. | `docs/prds/README.md:1-8` |
| CLUSTERS.md is the live dashboard and defines cluster states. | `docs/CLUSTERS.md:1-24` |
| Recent supervisor reviews use PRD-line-specific findings and consolidated action lists. | `docs/supervisor/LOG.md:1-18`, `docs/supervisor/LOG.md:50-70` |
| Recent PRDs use claims-audit tables for current-behavior claims. | `docs/prds/PRD-tpatch-land.md:36-43`, `docs/prds/PRD-skill-doc-strategy.md:25-55` |
| tpatch personas require intent preservation, upstream survival, review proof, and Git compatibility. | `docs/market-research/personas.md:239-247` |
| Path B already advances analyze/define/explore from hand-authored artifacts. | `internal/store/manual.go:25-31,45-79`, `docs/agent-as-provider.md:7-47` |
| Path A analysis writes both prose and a structured sidecar; define reads the sidecar when present. | `internal/workflow/workflow.go:88-121` |
| `next` currently distinguishes pre/post-explore `defined` features by raw `exploration.md` presence. | `internal/cli/phase2.go:409-466` |

### External references

| Source type | URL | Accessed | Claim used |
|---|---|---:|---|
| Official OpenSpec README | <https://github.com/Fission-AI/OpenSpec> | 2026-06-25 | OpenSpec is a lightweight spec layer for human/AI agreement; default examples create `proposal.md`, `specs/`, `design.md`, and `tasks.md`; README compares itself with Spec Kit. |
| Official OpenSpec docs | <https://raw.githubusercontent.com/Fission-AI/OpenSpec/main/docs/getting-started.md> | 2026-06-25 | Default loop, created folder structure, artifact purposes, delta specs, and archive merge behavior. |
| Official OpenSpec docs | <https://raw.githubusercontent.com/Fission-AI/OpenSpec/main/docs/overview.md> | 2026-06-25 | Five core concepts: specs as truth, changes as units, delta specs, artifact chain, archive. |
| Official OpenSpec docs | <https://raw.githubusercontent.com/Fission-AI/OpenSpec/main/docs/existing-projects.md> | 2026-06-25 | Brownfield guidance: do not document the whole codebase first; grow specs from real changes. |
| Official OpenSpec docs | <https://raw.githubusercontent.com/Fission-AI/OpenSpec/main/docs/editing-changes.md> | 2026-06-25 | Artifacts are Markdown and editable anytime; no locked planning phase. |
| Official OpenSpec docs | <https://raw.githubusercontent.com/Fission-AI/OpenSpec/main/docs/commands.md> | 2026-06-25 | Command semantics for propose, explore, apply, verify, sync, archive. |
| Official OpenSpec docs | <https://raw.githubusercontent.com/Fission-AI/OpenSpec/main/docs/opsx.md> | 2026-06-25 | OPSX is action-oriented, schema/template-driven, and dependency-aware rather than phase-locked. |
| Official OpenSpec docs | <https://raw.githubusercontent.com/Fission-AI/OpenSpec/main/docs/supported-tools.md> | 2026-06-25 | GitHub Copilot prompt files are recognized in IDE extensions; Copilot CLI does not consume `.github/prompts/*.prompt.md` directly. |
| Official GitHub Blog | <https://github.blog/ai-and-ml/generative-ai/spec-driven-development-with-ai-get-started-with-a-new-open-source-toolkit/> | 2026-06-25 | Spec Kit workflow is specify, plan, tasks, implement; human verifies/refines each phase; specs are shared source of truth. |
| Official Spec Kit repository | <https://github.com/github/spec-kit> | 2026-06-25 | Spec Kit is GitHub's open-source toolkit; commands include constitution/specify/plan/tasks/implement and optional clarify/analyze/checklist. |
| Official Spec Kit methodology | <https://raw.githubusercontent.com/github/spec-kit/main/spec-driven.md> | 2026-06-25 | SDD treats specs as executable/source of truth; templates constrain LLM behavior; constitution and gates guide architecture. |
| Official Spec Kit quickstart | <https://raw.githubusercontent.com/github/spec-kit/main/docs/quickstart.md> | 2026-06-25 | Recommended production workflow includes constitution, specify, clarify, plan, checklist, tasks, analyze, implement. |
| Official Spec Kit template | <https://raw.githubusercontent.com/github/spec-kit/main/templates/spec-template.md> | 2026-06-25 | Spec template requires prioritized independently testable user stories, acceptance scenarios, requirements, success criteria, assumptions. |
| Official Spec Kit template | <https://raw.githubusercontent.com/github/spec-kit/main/templates/plan-template.md> | 2026-06-25 | Plan template includes technical context, constitution check, project structure, and generated supporting docs. |
| Official Spec Kit template | <https://raw.githubusercontent.com/github/spec-kit/main/templates/tasks-template.md> | 2026-06-25 | Tasks template groups setup/foundation/user stories, marks parallel tasks `[P]`, and defines MVP/incremental execution. |
| Official Spec Kit template | <https://raw.githubusercontent.com/github/spec-kit/main/templates/constitution-template.md> | 2026-06-25 | Constitution template captures core principles, constraints, workflow/review process, governance, and amendment metadata. |

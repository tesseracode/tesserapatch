# Personas — Who We Are Building For

**Status**: Living
**Date**: 2026-05-03 (initial)
**Owner**: Core
**Related**: [competitive-landscape.md §8 BMC](competitive-landscape.md),
[competitive-landscape.md §6 SMART](competitive-landscape.md),
[PRD-tpatch-land](../prds/PRD-tpatch-land.md),
[WP-001 §5.2](../whitepapers/WP-001-feature-slice-gap.md)

## Why this doc exists

The Business Model Canvas in `competitive-landscape.md §8` lists
four customer segments. This doc gives those segments faces and
JTBD framing so future PRD audience-fit arguments have a concrete
target. It commits to a primary audience for the v0.7 horizon
without closing off later expansion.

## Refresh triggers

- A new persona becomes empirically distinguishable in case-study
  data → add.
- A persona's pain point changes materially (e.g., agent
  ergonomics shift) → revise.
- A roadmap decision targets a non-listed persona → either add
  the persona or argue why it's adjacent.

---

## Primary audience for v0.7: fork maintainers — any kind

The thesis is one common pain shared across diverse contexts:
**you have customizations on top of an upstream you do not
control, and upstream keeps moving.** Three personas concretize
this. They are not segmentation marketing — they are the same
JTBD with different urgency and risk profiles.

> **Later expansion target (post-v0.7, not committed):** all
> software developers in any project — soft forks, hard forks,
> ground-up projects. The product matures by deepening the
> structured-intent story enough that "patch-as-artifact"
> becomes useful even outside fork-vs-upstream workflows. That
> is post-v0.7 strategy work.

---

## Persona 1 — Platform Pat *(Platform / DevX engineer)*

### Context

Pat works on the platform team at a 50–500 person SaaS. They
maintain forks of Argo / Backstage / Copilot-proxy / vendor
packages because the upstream defaults don't fit their build /
auth / observability stack. They typically maintain 3–10 such
forks at once. Their team reviews each other's reconciliation
work; correctness and audit are part of their job description.

### Job-to-be-done

> "When upstream rebases, I want my customizations to survive
> without me redoing manual conflict resolution work for every
> fork."

### Pain points

- **Upstream-rebase tax.** N forks × M upstream rebases per
  quarter = N × M reconcile-work. Linear in both axes.
- **Audit pressure.** Security / compliance asks "what did you
  change vs upstream?" — `git format-patch` answers shape but
  not *why*.
- **Onboarding cost.** New platform-team members must learn each
  fork's customization story; tribal knowledge is brittle.
- **Cross-fork consistency.** Custom auth that lives in three
  forks should evolve together; today there's no shared
  reasoning surface.

### How tpatch fits

- **Provider-assisted reconcile** survives the rebase (validated
  by `tesseracode/copilot-api` running 47 commits ahead of
  upstream).
- **Lifecycle-phase artefacts** (`analysis.md` / `definition.md` /
  `exploration.md`) capture *why*, addressable for audit.
- **Skill emission** means Pat's coding agent can reason about
  every fork without re-learning each one — onboarding cost
  collapses.
- **Deterministic apply-recipe** answers compliance's "prove this
  patch matches what you reviewed" question.

### Counter-evidence (where tpatch fits poorly today)

- **WP-001 §5.2 collisions.** 9 of 21 features in copilot-api have
  the same patch byte-for-byte. Pat would notice immediately.
  `record` UX must close this before Pat is a comfortable user.
  (PRD-record-collision-detection is the gating fix.)
- **Multi-collaborator op log doesn't exist** (ADR-015 D7
  deferred). Pat's team would need it. Single-user today.
- **No "this patch is already upstream" detector.** Pat would
  re-discover it manually for every PR.

---

## Persona 2 — Security Sam *(Security / appsec engineer)*

### Context

Sam is responsible for keeping vendored OSS up-to-date and free
of known CVEs. When a CVE drops on a dependency, Sam needs to
patch it in the vendored copy *before* upstream merges and
publishes a release. Sam may run pre-release patches in
production for days or weeks before upstream catches up.

Sam's deployment surface is wider than Pat's: the same patch
gets applied to many internal copies of the same upstream.
Determinism across applies is non-negotiable.

### Job-to-be-done

> "When a CVE drops, I want to apply the fix to my fork
> immediately, prove the fix is what we reviewed, and drop the
> patch cleanly when upstream's official fix lands."

### Pain points

- **Time-to-patch is the SLA.** Manual fork-and-patch is the
  current path; days of engineering for each.
- **"Did upstream merge their fix?"** has to be checked manually
  for every CVE-patch in flight.
- **Replay confidence.** Sam's patches are applied to many
  copies; bytes diverging across applies is a security finding
  in its own right.
- **Forwardability.** Sam often wants the fix in upstream
  eventually; the patch needs to be reviewable as a PR, not
  just usable internally.

### How tpatch fits

- **Patch-as-artifact + recipe** = deterministic replay across
  internal copies. Apply once, store, replay everywhere.
- **`Tpatch-Feature` trailer + audit log** = "we patched
  CVE-YYYY-NNNN on date X with patch checksum Y" — answer for
  the next compliance review.
- **Future: `reconcile` patch-already-upstream detection**
  (gbp-pq's auto-detect-applied; logged as a follow-up in
  `competitive-landscape.md §9`) directly answers "did the
  official fix land?"
- **Lifecycle phases** + the four-trailer block (PRD-tpatch-land
  §3.4) make Sam's patches forwardable as PRs without restating
  context for upstream maintainers.

### Counter-evidence

- **Lifecycle phases are heavyweight for "fix this CVE in 30
  minutes."** Sam may want a `tpatch hotfix` shortcut. Not on
  the roadmap; logged as a personas-driven backlog candidate
  (cross-link to `tesseracode/tesserapatch` issue tracker when
  the project goes public).
- **No "patch-already-upstream" detector ships today.** Highest
  pain point not closed in v0.7 scope.
- **No CVE-id field in the patch metadata.** Sam would want
  `CVE: CVE-YYYY-NNNN` in the trailer block.

---

## Persona 3 — Maintainer Mira *(Solo OSS dev maintaining a long-running fork)*

### Context

Mira maintains a personal or indie-product fork — a CLI tool, a
language server, a static-site generator they forked from a
larger upstream because the upstream maintainer rejected their
feature, or because they want a leaner alternative. The fork has
lived 1–5 years; upstream has moved on. Mira has no team. The
bus factor is one.

Mira increasingly works with a coding agent. "Explain this fork"
is a routine prompt with no good answer in `git log`.

### Job-to-be-done

> "I want my fork's reasoning preserved as upstream evolves so
> my future self (and my coding agent) can keep maintaining it
> without me holding everything in my head."

### Pain points

- **Reasoning rot.** Mira knew why every patch existed two
  years ago. Half of that knowledge is gone now.
- **Solo bus factor.** No teammates to share fork knowledge with.
- **AI ergonomics.** Mira works with an agent; the agent needs
  context the `git log` doesn't carry.
- **Onboarding their own future self.** Coming back to a fork
  after six months away is the worst-case version of every
  pain above.

### How tpatch fits

- **Lifecycle-phase artefacts are the biggest single value
  prop** for Mira. Structured intent solves reasoning rot
  directly.
- **Skill emission** means Mira's agent always knows how to
  reason about the fork — the agent file *is* the team handoff
  Mira doesn't have.
- **Versioned patch history** (post-ADR-015 D2 ship) gives
  Mira recovery without trusting only `git reflog`.
- **Single-binary, zero-deps Go** makes setup match Mira's
  appetite for tooling overhead (low).

### Counter-evidence

- **tpatch's setup overhead is non-trivial for a solo
  maintainer.** Onboarding flow matters more for Mira than for
  any other persona.
- **The product surface implies an enterprise audience.** SPEC,
  PRDs, ADRs, whitepapers, supervisor logs — all good engineering
  hygiene, all intimidating for a solo dev. Documentation
  written *for Mira* is a future need.
- **`tpatch reconcile` token cost.** A solo fork against a slow-
  moving upstream may not need provider-assisted reconcile
  often. Heuristic fallback (offline mode) is the lever; making
  it the default for Mira's persona is worth a roadmap thought.

---

## Adjacent personas — out of focus today

Calling these out so future PRDs that propose expansion know
what's at stake.

| Persona | Why they're adjacent | Why we're not targeting today |
|---|---|---|
| **General developer (any project, no fork)** | Lifecycle phases + structured intent could improve any repo's discipline. | Post-v0.7 expansion. Product is still focused on the fork-vs-upstream story. |
| **Distro packager (Debian / Fedora / NixOS)** | Their workflow IS gbp-pq — direct overlap. | Distribution-format coupling (`3.0 (quilt)`) is gbp-pq's wedge; we are format-agnostic. We don't pitch this seam (see `competitive-landscape.md §10`). |
| **Open-source maintainer of upstream** | Could use tpatch to prepare incoming PRs. | Upstream-flow is Lane B's seam (stacked-PR tools). We solve the inverse: fork-against-upstream, not feature-against-trunk. |
| **Multi-fork enterprise team (>10 forks)** | BMC §1 tertiary segment. High potential value. | Multi-collaborator op log not built (ADR-015 D7 deferred). Logged for revisit when D2 + D3 land. |

---

## Common JTBD across the three primary personas

Written generically:

> When my customizations live on top of an upstream I don't
> control, I want a tool that captures *why* each patch exists,
> survives upstream rebases, proves correctness to whoever
> reviews my work, and stays out of the way of `git log` /
> `git review` muscle memory.

Every primary-persona variant of this JTBD is in scope. Adjacent
personas have a different shape (e.g., distro packagers don't
need "stays out of the way of `git log`" — they live in a
different substrate; they need "stays out of the way of
`debian/source/format`").

---

## How this doc is used

- **PRDs** can cite a persona by name (e.g., "Security Sam needs
  upstream-applied detection") instead of re-deriving the
  audience case.
- **Roadmap discussions** can ask "which personas does this
  unblock?" as a prioritization heuristic.
- **Skill files / docs** can be written with a persona in mind —
  Pat needs different onboarding text than Mira.
- **Refresh trigger:** add a persona when case-study data
  produces an empirically distinguishable user; revise when an
  existing persona's pain shifts.

## Cross-references

- **`competitive-landscape.md §8` BMC** — flat segment list this
  doc concretizes.
- **`competitive-landscape.md §6` SMART** — the v0.7 outcome is
  measured on Pat-style fork repos.
- **`competitive-landscape.md §4` Porter's Five Forces** — the
  "regenerate-with-agent" substitute pressure (HIGH) is
  felt most by Mira.
- **WP-001 §5.2** — collision shape Pat would feel first.
- **PRD-tpatch-land** — the deliverable that makes Pat's audit
  story complete.
- **`docs/commits.md`** — the interim trailer convention Pat
  relies on for `git log` archaeology.

## Disputes

(empty — open a `### <persona> — <date>` block here when a
persona's framing or pain ranking is contested. Resolve via
discussion, fold the resolution back in.)

# PRD — `tpatch hotfix` (fast-path lifecycle for CVEs and time-boxed patches)

**Status**: Draft
**Date**: 2026-05-09
**Owner**: Core (research-driven; no implementation owner yet)
**Milestone**: Personas-driven UX shortcuts (post-WP-001, post-PRD-tpatch-land). Not yet on the roadmap.
**Depends on**:
  - [PRD-patch-already-upstream-detector](PRD-patch-already-upstream-detector.md) — hotfix's auto-drop-when-upstream-merges pitch is gated on a deterministic detector being available.
  - [PRD-tpatch-land](PRD-tpatch-land.md) §3.4 four-trailer block — `Tpatch-CVE` is an additive trailer field; this PRD must not collide with the four committed trailers.
  - [ADR-011 D5](../adrs/ADR-011-feature-dependencies.md) — `upstream_merged` already satisfies hard parents; hotfixes inherit that semantics for free.

**Related**:
  - [`docs/market-research/personas.md`](../market-research/personas.md) — Persona 2 "Security Sam" and the *fix-this-CVE-in-30-minutes* JTBD that motivates this PRD.
  - [`docs/market-research/competitive-landscape.md`](../market-research/competitive-landscape.md) §11 follow-ups — `tpatch hotfix` is listed as one of the five concrete backlog items surfaced by the persona counter-evidence.
  - [`docs/commits.md`](../commits.md) — interim `Tpatch-Feature` trailer convention; `Tpatch-CVE` extends it.
  - [`internal/store/types.go`](../../internal/store/types.go) — `FeatureStatus` is the canonical machine state. There is **no** `feature.yaml` (confirmed via grep — see ADR-011 / PRD-feature-dependencies).
  - [`docs/feature-layout.md`](../feature-layout.md) — feature-directory layout. PRD-1's stub-artifact set is a subset of this layout.

---

## 0. Claims Audit

This PRD is a **proposal**. It changes nothing. The audit below verifies the
factual claims it relies on, against the codebase as of 2026-05-09.

| Claim | Verified? | Evidence |
|---|---|---|
| `FeatureStatus` is the canonical machine state; there is no `feature.yaml`. | ✅ | `internal/store/types.go:121-167`; `ADR-011-feature-dependencies.md:24` "No new `feature.yaml` schema field." |
| Lifecycle states are exactly the 10 in `internal/store/types.go:9-19`. | ✅ | grepped `internal/store/types.go:9-19` (StateRequested … StateUpstreamMerged). |
| `upstream_merged` already satisfies hard dependencies. | ✅ | ADR-011 D5; `internal/store/types.go:209-213` (`SatisfiedBy` doc). |
| The current lifecycle has no fast path: every feature transits requested → analyzed → defined → (implementing →) applied → active. | ✅ | `internal/cli/cobra.go` analyze/define/explore/apply/record subcommands; `docs/commits.md:107` phase-ordering box. |
| Persona 2 "Security Sam" has a documented JTBD calling for a 30-minute fast path with audit retroactively attached. | ✅ | `docs/market-research/personas.md` "Persona 2 — Security Sam" + "Counter-evidence" subsection (entry: `tpatch hotfix`). |
| The interim trailer convention is `Tpatch-Feature: <slug>` over a two-commit pair. | ✅ | `docs/commits.md` end-to-end. |

**No code has been changed by this PRD.** Implementation lands behind a
flag (see §6) gated on supervisor approval and an explicit milestone
assignment.

---

## Summary

Add a **single-shot** subcommand `tpatch hotfix <slug>` that compresses
`analyze → define → explore` into one step, leaves `apply` / `record`
untouched, and adds two pieces of new metadata:

1. A new `FeatureStatus.Kind` field with values `feature` (default,
   back-compatible) and `hotfix`.
2. An optional `FeatureStatus.CVE` string field (e.g. `CVE-2026-12345`)
   that surfaces in the trailer block as `Tpatch-CVE: <id>` when present.

A hotfix feature reaches `defined` directly after `tpatch hotfix`,
skipping the LLM-heavy analyze / define / explore artifacts. Optional
`tpatch hotfix promote <slug>` retroactively backfills the lifecycle
artifacts from the now-applied state — turning a hotfix into a normal
feature for forwarding upstream, audit, or planning.

The pitch is shaped entirely by the **Security Sam** persona JTBD: *fix
this CVE in 30 minutes; prove it later; drop it cleanly when upstream's
fix lands.*

---

## 1. Problem statement

The current lifecycle (`requested → analyzed → defined → implementing →
applied → active`) is **correct for the Pat / Mira personas**
(maintenance + dual-track). It is **wrong for Security Sam**:

- Sam's request is single-line ("CVE-2026-12345 — bump dep + add
  validator").
- Sam's analysis is *the CVE advisory*, already written by someone else.
- Sam's exploration is *empty* — the upstream codebase already says
  where the vulnerable function lives.
- Sam needs **the patch shipped to production within his SLA**, with
  the lifecycle artifacts attached *after the fact*, not before.

Forcing Sam through analyze / define / explore costs him 20–40 minutes
of LLM-mediated phases that produce no information — it's a tax on the
critical path of a security-incident response.

The current `Tpatch-Feature: <slug>` trailer also carries no CVE
identifier. Pat (audit) cannot run `git log --grep='Tpatch-CVE:'` to
list every CVE-driven patch on the fork without scraping commit
messages — losing a primitive Pat's persona explicitly needs.

## 2. Goals / Non-goals

### Goals

1. A user can `tpatch hotfix <slug>` and reach `defined` in one
   invocation, with no LLM call required.
2. A `--cve <id>` flag attaches the CVE to status.json and to the
   trailer block on every commit produced by the hotfix lifecycle
   (started / record / chore-tpatch).
3. Hotfix features inter-operate with the existing apply / record /
   reconcile machinery without modification — they are *normal*
   features in `applied` / `active` / `upstream_merged` after Sam
   completes the workflow.
4. `tpatch hotfix promote <slug>` retroactively backfills
   `analysis.md` / `spec.md` / `exploration.md` from the
   already-applied patch (provider-assisted, optional, opt-in).
5. The DAG, dependency gate, verify, and reconcile primitives apply
   to hotfix features unchanged.
6. Backwards compatible: existing `status.json` files round-trip
   byte-identical (no `Kind` field → `KindFeature` default).

### Non-goals

1. **Bypassing apply / record.** The whole point of tpatch is the
   audit trail; hotfix shortens analyze / define / explore but
   leaves apply / record / reconcile *exactly* where they are.
2. **Auto-fetching CVE metadata.** No NVD / GHSA scraping, no
   curl-on-launch. The user supplies the CVE id; tpatch persists
   it.
3. **Bypassing the dependency gate.** A hotfix with a hard
   dependency is gated identically to a feature.
4. **A new lifecycle vocabulary.** Hotfixes ride the existing 10-
   state machine (§0). No `StateHotfixed`, no parallel state graph.
5. **Auto-drop on upstream merge.** Auto-drop is gated on
   PRD-patch-already-upstream-detector and is opt-in (see §5.4).

## 3. User-facing contract

### 3.1 New verb

```
tpatch hotfix <slug> [--cve <id>] [--title <line>] [--from <ref>] [--no-promote-stub]
```

| Flag | Purpose | Default |
|---|---|---|
| `--cve <id>` | CVE identifier, e.g. `CVE-2026-12345`. Persisted to `FeatureStatus.CVE`. Surfaces as `Tpatch-CVE:` trailer. | empty |
| `--title <line>` | One-line title for `record.md` and trailer-block subject lines. | derived from slug |
| `--from <ref>` | Treat the existing diff at `<ref>` as the hotfix payload (Path B shortcut: hotfix already coded; just register it). | `--from HEAD~1` if dirty tree heuristic detects it; otherwise prompt |
| `--no-promote-stub` | Skip writing the `analysis.md` / `spec.md` / `exploration.md` stubs entirely. status.json still tracks the hotfix; promote backfills later. | stubs are written |

### 3.2 New verb (forward-promotion)

```
tpatch hotfix promote <slug> [--provider <id>] [--model <id>]
```

Walks the slug's `post-apply.patch` + `record.md` + (if available)
`Tpatch-CVE` trailer and **provider-assists** the backfill of
`analysis.md` / `spec.md` / `exploration.md`. The state machine
remains unchanged — promote does not move the feature out of `applied`
/ `active`. It only enriches the artifact set.

Output:

- `analysis.md` — generated by the provider from the diff plus the
  CVE advisory hint.
- `spec.md` — generated as a one-paragraph rationale.
- `exploration.md` — generated as the file-by-file impact map.

If `--provider` is unset and no provider is configured, the command
errors out with a clear `--no-provider` hint and **does not write
anything** (deterministic, no half-promoted state).

### 3.3 Lifecycle path (compared to feature path)

```
            FEATURE                              HOTFIX
            requested  → analyze  → analyzed     requested  ┐
            analyzed   → define   → defined                 │  tpatch hotfix
            defined    → explore  → defined                 ┘  (single command)
                       │                         defined        ← reach this state directly
                       ↓
            defined    → apply    → applied      defined    → apply    → applied
            applied    → record   → active       applied    → record   → active
                       (reconcile inheritance is identical)
```

Hotfix advances `requested → defined` in **one** invocation. Apply,
record, reconcile, verify are unchanged.

### 3.4 Trailer-block extension

Augments PRD-tpatch-land §3.4. The locked four-trailer block (per
PRD-tpatch-land §3.4) is preserved verbatim; hotfix-kind features
get **one additive fifth trailer** when `FeatureStatus.CVE` is set:

```
Tpatch-Feature: cve-2026-12345-validate-input
Tpatch-Patch-SHA: <sha256 of post-apply.patch bytes>
Tpatch-Recipe-SHA: <sha256 of apply-recipe.json bytes, or "none">
Tpatch-Base-Commit: <status.json apply.base_commit>
Tpatch-CVE: CVE-2026-12345                              ← additive (hotfix-kind, --cve set)
```

Order: `Tpatch-CVE` is appended **after** the locked four, before the
repo-level `Co-authored-by:` trailer (per `CLAUDE.md` working rule 8 +
PRD-tpatch-land §3.4 "Repo-level trailers"). It is omitted entirely
when `FeatureStatus.CVE` is empty — so non-hotfix commits and
no-CVE hotfixes round-trip unchanged.

Pat (audit persona) can `git log --grep='^Tpatch-CVE:'` to enumerate
every CVE-driven patch on the fork. The grep target is anchored on
the four-trailer block: every CVE-bearing commit also carries the
canonical four, so `Tpatch-CVE` lookups can be cross-referenced
against `Tpatch-Feature` for slug-level audit without a second grep.

**Authoritative emitter.** Per PRD-tpatch-land §3.4 "Coordination
with PRD-tpatch-hotfix", `tpatch land` itself does **not** emit
`Tpatch-CVE`; the hotfix verb (or its delegate to `land`) is
responsible for the additive line. This keeps `land`'s emitter
single-purpose.

### 3.5 Status output

`tpatch status <slug>` renders an extra line for hotfix-kind
features:

```
slug:        cve-2026-12345-validate-input
kind:        hotfix                          ← NEW
cve:         CVE-2026-12345                  ← NEW (only when set)
state:       active
…
```

`tpatch list --kind hotfix` filters to hotfixes; `--kind feature`
filters out hotfixes; default shows both with the `kind` column.

## 4. Schema additions

In `internal/store/types.go::FeatureStatus`:

```go
// Kind discriminates the lifecycle path the feature took. Default is
// "feature" — the full analyze / define / explore lifecycle. "hotfix"
// is the fast-path lifecycle introduced by PRD-tpatch-hotfix; it
// reaches `defined` in one invocation and skips the LLM-heavy
// pre-apply phases.
//
// `omitempty` is load-bearing — pre-PRD-tpatch-hotfix fixtures must
// round-trip byte-identical. An empty Kind is treated as
// KindFeature at read time.
Kind FeatureKind `json:"kind,omitempty"`

// CVE is the optional CVE identifier for hotfix-kind features.
// Surfaces in the trailer block as `Tpatch-CVE:` when set. Empty
// for non-CVE hotfixes (e.g. internal-only urgent fixes) and for
// all KindFeature features.
CVE string `json:"cve,omitempty"`
```

```go
type FeatureKind string

const (
    KindFeature FeatureKind = "feature" // default, lifecycle goes through analyze/define/explore
    KindHotfix  FeatureKind = "hotfix"  // fast-path; promote backfills retroactively
)
```

Validation rule: `Kind` must be one of `{"", "feature", "hotfix"}`.
Empty is treated as `KindFeature` (back-compat). Validation rejects
any other value.

## 5. Behaviour

### 5.1 Stub artifacts (default, no `--no-promote-stub`)

`tpatch hotfix <slug>` writes:

- `.tpatch/features/<slug>/request.md` — verbatim `tpatch hotfix
  <slug> --cve <id> --title <line>` invocation, plus the CVE link
  if known.
- `.tpatch/features/<slug>/analysis.md` — one-line stub:
  `Hotfix — see CVE-2026-12345. Full analysis deferred to tpatch
  hotfix promote.`
- `.tpatch/features/<slug>/spec.md` — same pattern.
- `.tpatch/features/<slug>/exploration.md` — same pattern.
- `.tpatch/features/<slug>/status.json` — `Kind: KindHotfix`, `CVE:
  <id>`, `State: StateDefined`.

### 5.2 With `--no-promote-stub`

Skips the stub `.md` files entirely; only `request.md` and
`status.json` are written. `tpatch hotfix promote` later writes the
`.md` files for the first time.

### 5.3 Reconcile interaction

- A `hotfix`-kind feature that reconciles to `upstream_merged`
  triggers, by default, a one-line console hint:
  *"this hotfix was absorbed upstream as `<commit-sha>`; consider
  `tpatch feature remove <slug>`."*
- Auto-removal is **not** done by reconcile (conservative default
  — Sam may want the audit trail). Removal is a separate verb.
- The patch-id detector (PRD-patch-already-upstream-detector) is
  the deterministic mechanism that makes this hint trustworthy at
  scale. Without that PRD shipped, this PRD ships *unchanged* — the
  console hint just fires less often, gated on the existing
  Phase-3 provider verdict.

### 5.4 `--auto-drop-on-merged` (deferred to a follow-up PRD)

A future PRD may add `tpatch hotfix <slug> --auto-drop-on-merged`
that flips a `FeatureStatus` flag instructing reconcile to remove
the feature from the DAG when patch-id detects upstream
absorption. **Not in scope for this PRD.** The conservative
default — reconcile hints, user removes — is sufficient for the
Sam JTBD.

### 5.5 Interaction with the dependency gate

- Hotfix features participate in the DAG identically to features.
- A hotfix may declare hard / soft dependencies via
  `tpatch feature deps <slug> add <parent>` after creation
  (no `--depends-on` shortcut on `tpatch hotfix` itself — Sam
  rarely declares deps in the 30-min flow; this can be added
  in a follow-up if data shows otherwise).

### 5.6 Interaction with verify

- `tpatch verify <slug>` works identically.
- Verify checks that read `analysis.md` / `spec.md` /
  `exploration.md` for "evidence of intent" must tolerate the
  one-line hotfix stubs. New verify-check rule:
  `KindHotfix → relax-evidence-of-intent-checks`. Documented in
  the verify rule catalog (one rule edit).

## 6. Migration / rollout

- Behind the existing config flag pattern. Suggest:
  `Config.HotfixEnabled bool` (default false until v0.7.x).
- When the flag is OFF: `tpatch hotfix` errors out with the
  config-flag hint; `Kind` and `CVE` round-trip but are written
  only when the flag is ON.
- Pre-flag fixtures: zero churn — `omitempty` on both new fields
  guarantees byte-identical round-trip.
- **No SPEC change required at v0; no ROADMAP change at v0.** This
  PRD is research-driven; the supervisor assigns a milestone when
  scope locks.

## 7. Out of scope

- **Hotfix-specific metrics / dashboards** — no `tpatch metrics
  --kind hotfix` in this PRD.
- **GitHub Security Advisory integration** — no auto-population of
  `--cve` from a GHSA URL. Manual entry only.
- **Auto-drop on upstream merge** — flagged as a follow-up PRD
  (see §5.4).
- **Multi-CVE hotfixes** — `CVE` is a single string. A future PRD
  can promote it to a `[]string` if data warrants.
- **`Tpatch-CVE` written by `tpatch land` from non-hotfix features**
  — out of scope; PRD-tpatch-land owns trailer composition for
  feature-kind features.

## 8. Open questions

1. Should `tpatch hotfix promote` be **provider-required** (no
   heuristic fallback) or should it allow heuristic backfill from
   the diff alone? Default-position: provider-required (heuristic
   backfill is low-quality and may produce misleading
   `analysis.md`).
2. Should `--cve` be **validated** against the
   `CVE-YYYY-NNNN+` regex? Suggest yes, with a `--force-cve` escape
   hatch for `GHSA-…` ids.
3. Does `tpatch hotfix` need a `--depends-on <slug>[:hard|soft]`
   shortcut? Punted to v0.7.1 unless persona research promotes it
   sooner.
4. Should `tpatch list` show the `kind` column unconditionally or
   gate it on `--show-kind`? Default-position: unconditional once
   the flag is ON; back-compat fixtures don't run `tpatch list`
   for assertion.

## 9. Validation plan

1. Unit tests on `FeatureStatus` round-trip with `Kind` /
   `CVE` set + unset.
2. Unit tests on the validation rule (`Kind ∈ {"", "feature",
   "hotfix"}`).
3. Integration test: `tpatch hotfix <slug>` produces a feature in
   `defined` with the expected stub artifact set; `tpatch apply`
   + `tpatch record` reach `active` unchanged.
4. Integration test: `Tpatch-CVE` trailer appears on every commit
   the hotfix flow writes (when `--cve` is set), and is absent
   when not.
5. Integration test: `tpatch hotfix promote <slug>` (provider
   required) backfills the three `.md` files from the recorded
   diff.
6. Integration test: `tpatch reconcile` correctly classifies a
   hotfix as `upstream_merged` and emits the console hint.
7. Backwards-compat test: pre-PRD fixtures round-trip
   byte-identical with the new fields hidden by `omitempty`.

## 10. Cross-references

- **`docs/market-research/personas.md`** — Persona 2 Security Sam
  is the named user.
- **`docs/market-research/competitive-landscape.md` §5 SWOT
  Opportunities** — `tpatch hotfix` is one of the named follow-ups
  unblocking the Sam-driven backlog.
- **PRD-tpatch-land §3.4** — `Tpatch-CVE` is additive to the
  four-trailer block.
- **PRD-patch-already-upstream-detector** — gates the §5.3
  trustworthy-at-scale claim.
- **ADR-011 D5** — `upstream_merged` satisfaction story carries
  over to hotfixes for free.

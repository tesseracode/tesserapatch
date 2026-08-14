# PRD — Prepare Intent Bundle — `tpatch prepare <slug>` (mutating modes)

**Status**: Draft — Awaiting Review (rev-0)
**Date**: 2026-08-13
**Owner**: Core (planning lane)
**Byline**: writer sub-agent, rev-0 based on dispatch HEAD `20e8bbe`, WAVE_BASE `d060ff4`
**Milestone**: TBD — this document ships no code
**Issue**: [GH #11 — define the mutating prepare intent-bundle contract](https://github.com/tesseracode/tesserapatch/issues/11)
**Graduates from**: [WP-005 Spec-driven workflows](../whitepapers/WP-005-spec-driven-workflows.md), Turns 2–4
**Prerequisite (accepted)**: [PRD-artifact-validation-and-provenance](./PRD-artifact-validation-and-provenance.md)
rev-5 (Accepted 2026-08-13) and [ADR-034](../adrs/ADR-034-rooted-filesystem-inspection-boundary.md)
rev-2 (Accepted 2026-08-13). This PRD builds **on top of** that read-only
contract and does not reopen it.
**Architecture**: [ADR-035 — Intent bundle publication and history](../adrs/ADR-035-intent-bundle-publication-and-history.md)
(**Proposed**, rev-0). **This PRD and ADR-035 must be reviewed together.**
ADR-035 locks the publication/history decisions; this PRD states the product
contract that depends on them. Where they overlap, **ADR-035 is normative**.

> **Implementation is not authorized by this document.** No Go file, test,
> asset or CLI surface may change until **both** this PRD and ADR-035 are
> accepted. §19 states the gate; §17 states the slices that become dispatchable
> afterwards.

## Related

- [PRD-artifact-validation-and-provenance](./PRD-artifact-validation-and-provenance.md) — the accepted read-only `prepare <slug> --check` contract this PRD extends without modifying (its §20 lists exactly what this document must answer)
- [ADR-034 rooted filesystem inspection boundary](../adrs/ADR-034-rooted-filesystem-inspection-boundary.md) — D1–D18; reused verbatim for every canonical **read** this command performs
- [ADR-035 intent bundle publication and history](../adrs/ADR-035-intent-bundle-publication-and-history.md) — **companion, Proposed rev-0**; the transaction, the archive and the honesty limits
- [WP-005 Spec-driven workflows](../whitepapers/WP-005-spec-driven-workflows.md) — `## Agreed — Turns 2–3` items 4–9
- [WP-005 turn log](../whitepapers/WP-005-spec-driven-workflows.turns.md) — Turns 2, 3 and 4
- [Agent as Provider — Path B workflow](../agent-as-provider.md) — the phase → artifact → state contract
- [Path B Operator Guide](../path-b-operator-guide.md) — the hand-authored artifact flow this PRD's `--manual` mode adopts
- [Feature Layout](../feature-layout.md) — canonical vs audit-trail files under `.tpatch/features/<slug>/`
- [PRD-tpatch-land](./PRD-tpatch-land.md) — the shipped journal + crash-recovery precedent this PRD reuses and deliberately diverges from (redo vs undo)
- [PRD-feature-resource-claims-and-capture-adapters](./PRD-feature-resource-claims-and-capture-adapters.md) — the shipped content-addressed immutable-set + pointer precedent the archive reuses
- [ADR-033 resource capture boundary](../adrs/ADR-033-resource-capture-boundary.md) — D10 (no tracked timestamps), D11 (no Go map in a wire schema)
- [ADR-027 capture context privacy boundary](../adrs/ADR-027-capture-context-privacy-boundary.md) — D2 (no raw context), D6 (no wall-clock in determinism)
- [ADR-031 rejected feature state data model](../adrs/ADR-031-rejected-feature-state-data-model.md) — the per-command exit envelope and closed-enum precedent
- [PRD-tpatch-doctor](./PRD-tpatch-doctor.md) — the diagnostic surface that gains the pending-transaction check (§12.5)

## Revision history

| Rev | Disposition | What changed |
|---|---|---|
| rev-0 | Draft — Awaiting Review | First draft. Defines the four modes, the publication transaction, the undo journal, the content-addressed intent archive (**architecture gate fired → ADR-035**), the staged generator extraction, the lifecycle/compatibility deltas and a 234-row acceptance matrix. |

## Summary

The accepted prerequisite PRD gave `tpatch` a truthful **answer**: which intent
artifacts a feature has, and that their provenance is `unknown`. It
deliberately shipped no way to **act** on that answer, and refused plain
`tpatch prepare <slug>` with a reserved-surface exit 4
(`docs/prds/PRD-artifact-validation-and-provenance.md:356-382`).

This PRD defines that action. It adds three mutating modes to the same verb:

```text
tpatch prepare <slug>                 # Path A: generate ONLY the missing required artifacts
tpatch prepare <slug> --manual        # Path B: adopt an already-complete hand-authored bundle
tpatch prepare <slug> --regenerate    # explicit, archived, coherent overwrite of the whole bundle
```

Four properties are load-bearing, and each is stated as a limit rather than a
slogan:

1. **Default preserves.** Every canonical artifact that is already
   `present-nonempty` is never opened for writing, never staged and never
   renamed over. Only genuinely missing required artifacts are generated. An
   existing required artifact that is `present-empty`, `invalid-structured`,
   `symlink-refused`, `not-regular`, `unreadable`, `oversize` or `unstable` is
   **refused**, not silently overwritten (§6.1, §7.2).
2. **`--regenerate` cannot lose bytes.** Before the first canonical mutation,
   every artifact it is about to replace is copied into a durable,
   content-addressed, immutable **intent archive** inside the feature
   directory. That archive is a *byte-recovery* mechanism and is explicitly
   **not** a provenance representation (§9, ADR-035 D9). This selection is what
   fires the architecture gate and requires ADR-035 (§8.4).
3. **The transaction guarantee is command-boundary, not instantaneous.** An
   ordinary rename sequence over five files cannot make them appear
   simultaneously to a concurrent reader, and this PRD never claims it does.
   What is guaranteed is (a) at the command boundary the tree is all-old or
   all-new, (b) a crash inside the publication window is recoverable to
   all-old, and (c) the exposure window is bounded and enumerated (§7.1).
4. **No new lifecycle state.** A successful mutation ends at the existing
   `defined` (`internal/store/types.go:11`). `--check` mode is byte-identical
   to the accepted contract. Nothing calls `prepare`; `next`, `cycle` and the
   individual phase commands keep their current routing except for the four
   enumerated deltas in §12.6.

The publication unit — the thing that is all-old or all-new at the command
boundary — is exactly WP-005 Agreed item 7's set
(`docs/whitepapers/WP-005-spec-driven-workflows.md:75-81`): `analysis.md`,
`spec.md`, `exploration.md`, the structured sidecar `artifacts/analysis.json`,
the archive index, and the final `status.json` transition.

### What this PRD does not claim

- It does not claim the generated artifacts are *good*. The accepted
  disclaimer ("Structural presence only. This report does not certify semantic
  quality.", `docs/prds/PRD-artifact-validation-and-provenance.md:1796-1798`)
  applies verbatim to everything this command writes.
- It does not claim durable per-artifact provenance. `prepare --check` keeps
  reporting `provenance: unknown` for every artifact after this PRD ships, for
  every artifact this command writes, and §9.6 states exactly why the archive
  does not change that.
- It does not claim multi-file atomicity (§7.1).
- It does not claim bounded runtime. ADR-034 D16 withdrew every such claim for
  the read half; the write half adds a provider call, so the honest statement
  is a **deadline** (§11.5), not a guarantee of termination without one.
- It does not mandate any spec-driven methodology on downstream users. WP-005
  Agreed item 3 (`docs/whitepapers/WP-005-spec-driven-workflows.md:56-58`) is
  asserted mechanically, not assumed (§14.2, PIB-215).

## 1. Problem statement

### 1.1 There is no way to complete an intent bundle in one step

Today an operator completes a bundle by running up to three separate commands
(`analyze`, `define`, `explore`), each of which writes its artifact and
advances state independently (`internal/workflow/workflow.go:90-105`,
`:151-155`, `:196-200`). If the second fails, the tree keeps the first
artifact and a `analyzed` state — a half-bundle that no command reports as
such. WP-005 Agreed item 7 names this the thing a mutating `prepare` must fix
(`docs/whitepapers/WP-005-spec-driven-workflows.md:75-81`).

### 1.2 `cycle` is not that command

`cycle` runs analyze → define → explore → implement → apply → record in one
process (`internal/cli/phase2.go:26-32`). It calls the same incremental phase
functions in sequence (`internal/cli/phase2.go:62-96`), asserts state after
each (`internal/cli/phase2.go:69-71`), and continues past `defined` into
`implementing` and beyond unless `--skip-execute` is passed
(`internal/cli/phase2.go:122-126`). It is a **pipeline**, not a transaction:
every intermediate write is published to the canonical tree as it happens, and
a failure at step 3 leaves steps 1 and 2 on disk. §4 develops this as the
required existing-primitives pre-flight.

### 1.3 The phase functions cannot be reused as-is

`RunAnalysis` writes `artifacts/analysis.json`
(`internal/workflow/workflow.go:90`), then `analysis.md`
(`internal/workflow/workflow.go:96`), then mutates state
(`internal/workflow/workflow.go:103`) — three canonical mutations inside one
call, in that order, with a live `*store.Store`. `RunDefine` and `RunExplore`
have the same shape (`internal/workflow/workflow.go:151-155`, `:196-200`).
Worse, the retry helper writes raw provider responses into the canonical
artifacts directory *during* generation
(`internal/workflow/retry.go:105-109`), as does `RunAnalysis`'s provider-error
fallback (`internal/workflow/workflow.go:72`, `:80`). A `prepare` that called
these functions would have published four to twelve canonical files before it
knew whether the bundle would complete. WP-005 Turn 3 states exactly this
(`docs/whitepapers/WP-005-spec-driven-workflows.turns.md:112-117`).

### 1.4 Overwriting is currently unconditional and unrecoverable

`WriteFeatureFile` → `writeFile` → `os.WriteFile`
(`internal/store/store.go:443-449`, `:918-923`) truncates in place. There is no
preimage capture anywhere in the intent path. `tpatch define` run twice
destroys a hand-authored `spec.md` with no recovery route inside `tpatch`, and
`.tpatch/` is frequently untracked in real repositories — the CLI has a
dedicated helper to detect that (`internal/cli/cobra.go:3405-3407`). Any
`--regenerate` flag built on today's primitives would be a byte-shredder.

### 1.5 Path B has no completion verb

The documented Path B flow authors three files by hand and then runs three
`--manual` commands (`docs/path-b-operator-guide.md:61-73`). Each of those
commands validates only that the file *exists*
(`internal/store/manual.go:56-66`) — a zero-byte `spec.md` advances the feature
to `defined`. There is no single "I have authored the whole bundle, adopt it"
step, and no step that checks the bundle as a set before committing the
transition.

## 2. Goals / Non-goals

### 2.1 Goals

1. One command that takes a feature from any allowed pre-`defined` state to a
   **complete** intent bundle at `defined`, or leaves the tree exactly as it
   found it.
2. Preservation by default: never overwrite an existing non-empty canonical
   intent artifact unless the operator explicitly asked for it.
3. A `--regenerate` route whose destructiveness is bounded by a durable,
   in-repository byte archive, so prior hand-authored content is always
   recoverable without Git.
4. A truthful transaction contract that distinguishes command-boundary
   all-or-nothing, crash recoverability, and instantaneous multi-file
   visibility — and claims only the first two.
5. Crash recovery that decides from **evidence**, never from a recorded phase
   label, and refuses rather than guesses when the evidence is divergent.
6. Behavior compatibility: `--check`, `next`, `cycle`, the individual phase
   commands, `status`, `verify`, `doctor`, `land`, `record` and `reconcile` are
   unchanged except for the enumerated deltas in §12.6.
7. Reuse of the accepted ADR-034 rooted-inspection boundary for every canonical
   read, with no second path-safety model.
8. Determinism in tracked artifacts: no wall-clock field, content-addressed
   identifiers, canonical JSON, stable key order.

### 2.2 Non-goals

1. **No new `FeatureState`.** WP-005 Agreed item 6
   (`docs/whitepapers/WP-005-spec-driven-workflows.md:71-74`).
2. **No semantic quality judgement.** No thinness score, no lint, no heading
   requirement, no "stub detector".
3. **No durable per-artifact provenance.** The WP-005 provenance ADR trigger
   (`docs/prds/PRD-artifact-validation-and-provenance.md:2951-2985`) stays
   **unfired**; §9.6 and ADR-035 D9 state why the archive does not fire it.
4. **No change to `--check`'s report schema, exit envelope or bytes.**
5. **No mandatory SDD methodology** for downstream users.
6. **No restore verb.** Recovering an archived blob in v1 is an ordinary file
   copy (§9.5, Q4).
7. **No `--all` sweep**, no multi-slug form, no `--fix`.
8. **No interactive confirmation prompt** (§5.4).
9. **No implement/apply/record orchestration.** `prepare` stops at `defined`.
10. **No archive pruning verb** in v1 (§16 R6, Q5).

## 3. Terminology

| Term | Meaning in this document |
|---|---|
| **Intent bundle** | `analysis.md`, `spec.md`, `exploration.md` — the three canonical Markdown artifacts, per the accepted PRD §6.2 (`docs/prds/PRD-artifact-validation-and-provenance.md:432-436`). |
| **Publication set** | The ordered list of canonical files this invocation will create or replace, plus `status.json`. Computed at preflight, frozen before the journal is written (§7.2). |
| **Publication unit** | The set that is all-old or all-new at the command boundary (§7.1 T1). Identical to the publication set. |
| **Preimage** | The identity (SHA-256, size, mode, existence) of a canonical file as captured under the transaction lock immediately before publication. |
| **New-image** | The identity of the staged bytes that will replace it. |
| **Staging tree** | `.tpatch/local/intent-prepare/<slug>/stage-<12hex>/` — gitignored scratch where generated bytes live before publication. |
| **Journal** | `.tpatch/local/intent-prepare/<slug>/journal.json` — the durable **undo** record (§7.5). |
| **Intent archive** | `.tpatch/features/<slug>/artifacts/intent-archive/` — the tracked, content-addressed, immutable store of replaced bytes (§9). |
| **Generation** | One archive record naming the set of artifacts replaced by a single `--regenerate` publication (§9.3). Content-addressed; not a timestamped event. |
| **Mode** | Exactly one of `check`, `generate` (default), `manual`, `regenerate` (§5.2). |
| **Inspector** | The accepted read-only classifier from the prerequisite PRD §7, reused unchanged. |

## 4. Existing-primitives preflight

WP-005 Turn 3 requires this section by name
(`docs/whitepapers/WP-005-spec-driven-workflows.turns.md:118-123`). The
question is not "is a new command nicer" but "can a shipped primitive already
carry this responsibility".

| Primitive | What it does today | Why it cannot carry this |
|---|---|---|
| `analyze\|define\|explore` (Path A) | One provider call, one artifact write, one state transition each (`internal/workflow/workflow.go:90-105,151-155,196-200`) | Each publishes to the canonical tree before the next runs. Three invocations cannot be all-or-nothing, and a failure in the middle is not observable as a half-bundle. |
| `analyze\|define\|explore --manual` (Path B) | Validates the artifact **exists**, then advances state (`internal/store/manual.go:56-81`) | Presence-only; a zero-byte file advances state. One artifact per invocation. No set-level view. Accepted PRD §12 deliberately leaves these loose (`docs/prds/PRD-artifact-validation-and-provenance.md:2986-3015`), and this PRD does not tighten them (§12.4). |
| `cycle` | Sequential pipeline through `record` (`internal/cli/phase2.go:26-32,62-121`) | Not a transaction (§1.2); overshoots `defined`; regenerates unconditionally, so it is exactly the byte-shredder §1.4 describes. |
| `next` | Emits one action, inferring the `defined` sub-state from `exploration.md` presence (`internal/cli/phase2.go:437-446`) | Advisory only; writes nothing; cannot generate. |
| `prepare --check` (accepted) | Read-only classification and readiness verdict | Writes nothing by contract (`docs/prds/PRD-artifact-validation-and-provenance.md:2840-2866`). It is the **input** to this PRD, reused wholesale. |
| `store.writeFileAtomic` | Temp file + `Chmod` + `Write` + `Sync` + `Close` + `Rename` + parent-dir `Sync` (`internal/store/store.go:878-917`) | Correct for **one** file. Five sequential calls are five independent atoms, which is precisely the gap §7.1 exists to describe honestly. Reused as the single-file primitive, not as the transaction. |
| `land`'s journal | Durable journal + evidence-based recovery under the index lock (`internal/cli/land_journal.go:229-275,445-482`) | The right **shape**, wrong direction: land must roll *forward* because `git commit` already advanced HEAD and cannot be undone (`internal/cli/land_journal.go:16-23`). Nothing in a prepare publication is irreversible, so this PRD's journal is undo-only (§7.5, ADR-035 D5). |
| `store.PublishBatch` | Content-addressed immutable batch + atomically rewritten pointer (`internal/store/resource_publish.go:230-282`) | The right **shape** for the archive (§9), reused as a design precedent. It publishes one pointer, not a five-file set, so it is not the transaction either. |
| `MkdirAllAndSyncChain` / `SyncDir` / `RandomHex12` | Crash-safe directory and scratch primitives (`internal/store/fsdurable.go:22-33,41-52,96-103`) | Reused verbatim. They are ingredients, not the recipe. |
| `rescap` scratch + local-ignore gate | `.tpatch/local/<...>` scratch with a mandatory gitignore contract (`internal/rescap/scratch.go:35-62`) | Reused verbatim for the staging tree and journal location (§7.3, §13.5). |

**Conclusion.** Every ingredient exists; the composition does not. The new
surface is the composition (a transaction over a publication set) plus one new
durable object (the archive). Nothing else is invented.

## 5. CLI grammar and mode selection

### 5.1 Authorized grammar (v1, complete)

```text
tpatch prepare <slug> --check      [--json] [--quiet] [--path <dir>]
tpatch prepare <slug>              [--json] [--quiet] [--path <dir>] [--timeout <d>] [--no-retry] [--dry-run]
tpatch prepare <slug> --manual     [--json] [--quiet] [--path <dir>] [--dry-run]
tpatch prepare <slug> --regenerate [--json] [--quiet] [--path <dir>] [--timeout <d>] [--no-retry] [--dry-run]
```

- `<slug>` — exactly one, required, validated by the accepted canonical-slug
  grammar before any path is composed
  (`docs/prds/PRD-artifact-validation-and-provenance.md:696-772`). No new slug
  rule is introduced.
- `--check` — the accepted read-only mode. **Unchanged in every respect**
  (§12.1).
- `--manual` — Path B adoption. No provider call, no artifact write (§6.2).
- `--regenerate` — explicit archived overwrite of the whole bundle (§6.3).
- `--dry-run` — report the plan; write nothing at all, including no lock, no
  journal and no staging tree; make no provider call (§6.4).
- `--timeout` — a single deadline for the whole invocation. Default `180s`
  (§11.5, Q2). Rejected in `--check` and `--manual`, which make no provider
  call.
- `--no-retry` — disables validator-driven retry, exactly as the phase commands
  do today (`internal/cli/cobra.go:630`, `internal/workflow/retry.go:83-85`).
  Rejected in `--check` and `--manual`.
- `--json`, `--quiet` — the report routing shipped by the accepted contract
  (`docs/prds/PRD-artifact-validation-and-provenance.md:2367-2419`), extended
  to the mutating report schema of §10.
- `--path` — the inherited root persistent flag (`internal/cli/cobra.go:66`).

No other flag is registered. There is no `--all`, no `--fix`, no `--yes`, no
`--force`, no `--restore`, no `--format`, no `--interactive`.

### 5.2 Mode selection is total and mutually exclusive

`--check`, `--manual` and `--regenerate` are declared mutually exclusive with
cobra's `MarkFlagsMutuallyExclusive`, so any two of them is a **parse-time**
error and exits `1` before `RunE` runs. Exactly one mode is selected:

| `--check` | `--manual` | `--regenerate` | Mode |
|---|---|---|---|
| set | — | — | `check` |
| — | set | — | `manual` |
| — | — | set | `regenerate` |
| — | — | — | `generate` (default) |

`--timeout` and `--no-retry` are additionally declared mutually exclusive with
`--check` and with `--manual`. `--dry-run` is mutually exclusive with `--check`
only (a read-only mode has no plan to preview).

### 5.3 Behavior deltas this grammar creates — enumerated, not silent

The accepted contract reserved plain `prepare` and both mutating flags. Shipping
this PRD changes three observable behaviors, and the accepted PRD anticipated
exactly this (`docs/prds/PRD-artifact-validation-and-provenance.md:396-403`):

| Input | Before this PRD | After this PRD |
|---|---|---|
| `tpatch prepare <slug>` | exit `4`, frozen refusal line, no report (`docs/prds/PRD-artifact-validation-and-provenance.md:356-382`) | a real Path A run (§6.1) |
| `tpatch prepare <slug> --check --manual` | exit `1`, cobra `unknown flag` | exit `1`, cobra mutually-exclusive error |
| `tpatch prepare <slug> --check --regenerate` | exit `1`, cobra `unknown flag` | exit `1`, cobra mutually-exclusive error |

Consequences that must be carried through, each with an acceptance row:

1. Exit `4`'s reserved-surface population **disappears** and the frozen refusal
   string is deleted. Exit `4` is left **retired with no population** rather
   than rebound to a new meaning, so a harness that hard-coded "4 means
   `prepare` needs `--check`" can never silently misread a different condition
   as that one (§10.4, PIB-013, PIB-014).
2. The accepted PRD's `--help` mitigation for the `apply --mode prepare` name
   collision (`docs/prds/PRD-artifact-validation-and-provenance.md:344-355`)
   stays mandatory and must be updated to describe four modes, not one
   (PIB-016, PIB-017).
3. Both `--check`-plus-mutating-flag rows still exit `1` and still write
   nothing, so the *observable class* is unchanged even though the message text
   changes (PIB-011, PIB-012).

### 5.4 No confirmation prompt — decision and justification

`--regenerate` performs no interactive confirmation, and there is no `--yes`.

- The repository's only interactive precedent is `cycle --interactive`
  (`internal/cli/phase2.go:48,74-76`), which is opt-in and off by default.
- A prompt is not a safety mechanism here: the archive (§9) is. A prompt would
  be theater on top of a mechanism that already makes the operation
  recoverable.
- Prompts break harnesses and non-TTY invocation, and this command is
  explicitly harness-facing.
- The preview route is `--dry-run`, which is deterministic and scriptable.

`--regenerate` is itself the explicit act of consent: it is a flag the operator
must type, it is never implied by any other flag, and it is never the default.

## 6. Mode contracts

### 6.1 `tpatch prepare <slug>` — Path A, missing-only

**Intent**: complete the bundle without touching anything that already exists.

**Step 1 — inspect.** Run the accepted inspector over the four artifacts and
`status.json` (`docs/prds/PRD-artifact-validation-and-provenance.md:1653-1782`),
through one held `*os.Root` per ADR-034 D1. No second path model.

**Step 1a — the generation input.** All three shipped generators read
`request.md` and fail hard when it cannot be read
(`internal/workflow/workflow.go:37-40,112,160`). `prepare` therefore captures
`request.md` through the same bounded rooted read and **refuses before any
generation** when it is absent, empty, unsafe or unreadable: exit 3,
`request-unreadable`, zero mutation, no provider call (PIB-233). `--manual`
generates nothing and therefore does **not** require it (PIB-234).

**Step 2 — admissibility.** For each of the three **required** Markdown
artifacts, the inspector state decides:

| Inspector state | Default-mode disposition |
|---|---|
| `present-nonempty` | **Preserve.** Not staged, not opened for writing, not renamed over. Its bytes are read only as generation context (§11.2). |
| `absent` | **Generate.** Enters the publication set as a `create` entry. |
| `present-empty` | **Refuse** (exit 2, `artifact-empty-not-overwritten`). An empty file is *present*; overwriting it is the `--regenerate` route. |
| `invalid-structured` | Not reachable for Markdown (sidecar only). |
| `symlink-refused` | **Refuse** (exit 3, `artifact-unsafe`). Never followed, target never named. |
| `not-regular` | **Refuse** (exit 3, `artifact-unsafe`). |
| `unreadable` | **Refuse** (exit 3, `artifact-unsafe`). |
| `oversize` | **Refuse** (exit 3, `artifact-unsafe`). |
| `unstable` | **Refuse** (exit 3, `artifact-unstable`). |

The table is **total** over the accepted nine-value enum
(`docs/prds/PRD-artifact-validation-and-provenance.md:1765-1773`), and PIB-224
asserts totality mechanically.

The sidecar `artifacts/analysis.json` never causes a refusal and never affects
admissibility, exactly as in the accepted contract
(`docs/prds/PRD-artifact-validation-and-provenance.md:437-441`).

**Step 3 — the sidecar rule.** The sidecar enters the publication set **iff
this invocation generates `analysis.md`**. If `analysis.md` is preserved, the
sidecar is left exactly as found — including left absent. Synthesizing a
sidecar for a preserved (possibly hand-authored) `analysis.md` would fabricate
structured data that does not derive from the preserved bytes, and its presence
would then be a false Path A signal, which
`docs/prds/PRD-artifact-validation-and-provenance.md:2904-2920` forbids.

**Step 4 — nothing to do.** If all three required artifacts are
`present-nonempty` and the feature is already `defined`, the command is a
**no-op success**: exit 0, `action: "none"`, zero bytes written anywhere,
including `status.json` (PIB-035). If they are all present but the state is
below `defined`, the command publishes **only** the `status.json` transition
(§6.2's single-file path) and reports `action: "adopt"` (PIB-036).

**Step 5 — generate, stage, validate, publish.** §11 then §7.

**Step 6 — final state.** `defined`, `last_command: "prepare"`, notes per
§12.3.

### 6.2 `tpatch prepare <slug> --manual` — Path B adoption

**Intent**: "I authored the bundle by hand; adopt it."

- **No provider is constructed, no network call is made, no artifact byte is
  written.** PIB-045 asserts a provider spy records zero calls; PIB-046 asserts
  all four artifact files are byte-identical afterwards.
- The accepted inspector runs unchanged. Adoption proceeds **only** when the
  verdict is the accepted `ready` — all three required Markdown artifacts
  `present-nonempty` (`docs/prds/PRD-artifact-validation-and-provenance.md:432-436`).
- Not ready → refuse with the accepted `not_ready` semantics, **exit 2**, the
  full per-artifact report, and zero mutation (PIB-047, PIB-048).
- Ready → commit the transition: `state = defined`, `last_command = "prepare"`,
  notes per §12.3.
- **Single-file publication.** `status.json` is the only file written, through
  the shipped `writeFileAtomic` (`internal/store/store.go:871-917`). A single
  rename **is** atomic in the ordinary POSIX sense, so this path takes **no
  journal and no archive** — there is no multi-file window to protect
  (ADR-035 D3). It does take the transaction lock, so it cannot interleave with
  a sibling mutating `prepare` (§7.4).
- `SaveFeatureStatus` also refreshes `FEATURES.md` best-effort
  (`internal/store/store.go:368-377`); that is existing behavior of every state
  transition and is not part of the publication unit. PIB-049 pins that a
  `FEATURES.md` refresh failure does not fail the command, matching today.

### 6.3 `tpatch prepare <slug> --regenerate` — archived coherent overwrite

**Intent**: "Throw away the current bundle and produce a coherent new one."

**Scope decision: all three Markdown files plus the sidecar, always.**

Partial regeneration is not offered, and the reason is coherence, not
convenience: `spec.md` is generated from `analysis.md`
(`internal/workflow/workflow.go:120-129`) and `exploration.md` from both
(`internal/workflow/workflow.go:165-172`). Regenerating `spec.md` against an
old `analysis.md` and calling the result a bundle would produce a set whose
parts describe different intents, and nothing on disk would record the
mismatch. Q1 records a future `--only <ids>` as an additive, enumerated
extension.

**Preservation.** Before the first canonical mutation, every artifact whose
state is `present-nonempty`, `present-empty` or `invalid-structured` is copied
into the intent archive (§9). Artifacts that are `absent` archive nothing.
Artifacts that are `symlink-refused`, `not-regular`, `unreadable`, `oversize`
or `unstable` **refuse the whole invocation** (exit 3) — `--regenerate` will
not overwrite something it could not first safely read and archive. This is the
single most important refusal in the document: an overwrite route whose
preservation step can silently fail is not a preservation guarantee (PIB-060 …
PIB-066).

**Lifecycle.** Identical to §6.1 step 6.

### 6.4 `--dry-run` — plan, not outcome

`--dry-run` is available in `generate`, `manual` and `regenerate` modes.

- It performs the preflight inspection and computes the publication set.
- It writes **nothing**: no lock file, no journal, no staging tree, no
  directory, no `status.json`. PIB-072 asserts a filesystem spy records zero
  create/write/rename/mkdir calls of any kind.
- It makes **no provider call** (PIB-073).
- It exits `0` when the plan is admissible, and with the same refusal code the
  real run would produce when it is not (PIB-074 … PIB-076).
- Its report carries `dry_run: true` and the planned `actions` array, and it
  states verbatim: `Plan only. Generation was not attempted and may still
  fail.` Because it cannot run the generators, it is a statement about
  admissibility, never about outcome (PIB-077).

### 6.5 `--check` — unchanged

See §12.1. The accepted report bytes, schema, exit codes and zero-mutation
contract are preserved exactly, and PIB-198 … PIB-205 are the regressions that
prove it.

## 7. The publication transaction

### 7.1 What is guaranteed, and what a filesystem cannot give

Three distinct properties are routinely conflated under the word "atomic".
This PRD separates them and claims exactly two.

| | Property | Claimed? | Mechanism / why not |
|---|---|---|---|
| **T0** | **Instantaneous multi-file visibility.** A concurrent reader observes either the complete old set or the complete new set, at every instant. | **NO** | POSIX offers no multi-file atomic rename. Publishing five files is five independent `rename(2)` calls; between call *k* and *k+1* a concurrent reader observes a mixed set. No journal, lock or fsync changes this. Any claim otherwise would be false, so this PRD makes none. |
| **T1** | **Command-boundary all-or-old / all-new.** When the process returns, the publication set is entirely the new bytes (exit 0) or entirely the old bytes (any refusal or in-command failure). | **YES** | Stage → validate → journal → archive → publish, with rollback from the archive and the journal-held preimages on any failure inside the publication window (§7.8). |
| **T2** | **Crash recoverability.** After a kill, panic or power loss anywhere in the window, the *next* mutating `prepare` restores the complete old set, or refuses and preserves everything. | **YES** | The durable undo journal plus evidence-based recovery (§7.5, §7.9, §7.10). |

**The T0 exposure window, stated exactly.** It opens at the first canonical
rename and closes at the last. Inside it a concurrent reader — `tpatch status`,
`tpatch next`, `prepare --check` in another process, an editor, `git status` —
may observe any prefix of the publication order (§7.7) applied and the rest not.
The window contains no provider call, no network I/O, no subprocess and no
user-visible prompt: it is *N* renames and *M* directory fsyncs, where
*N ≤ 6*. That is the honest bound — a small number of syscalls, not zero time.

**What the accepted read-only check does inside the window.** It reports what is
actually on disk, which is the truthful answer. Its own instability probes
(`docs/prds/PRD-artifact-validation-and-provenance.md:1827-1917`) may
legitimately classify a file being renamed over as `unstable`. That is correct
behavior and is not a defect of either contract (PIB-206).

### 7.2 The publication set

Computed at preflight, frozen before the journal is written, and never
recomputed. Each entry is `(artifact_id, canonical relative path, action,
preimage, new-image)` with `action ∈ {create, replace}`.

| Order | Entry | Present when |
|---|---|---|
| 1 | `analysis.md` | it is generated this run |
| 2 | `spec.md` | it is generated this run |
| 3 | `exploration.md` | it is generated this run |
| 4 | `artifacts/analysis.json` | `analysis.md` is generated this run (§6.1 step 3) |
| 5 | `artifacts/intent-archive/index.json` | at least one entry has `action = replace` |
| 6 | `status.json` | always, in every mutating mode that reaches publication |

Archive **blob** files are not publication-set entries: they are additive,
immutable, content-addressed and written before the window opens (§9.2). An
orphaned blob left by a crash is a normal, permanent artifact and is never
garbage-collected — the same disposition the shipped resource-capture design
records for orphaned batches (`docs/feature-layout.md:103-106`).

`status.json` is **last**, deliberately. If a crash lands between entry 5 and
entry 6, the tree holds new artifacts and the old state — a strictly recoverable
combination, because recovery's undo has every preimage it needs and the state
has not yet claimed the bundle is complete. The reverse order would publish a
`defined` claim over artifacts that might never land.

### 7.3 Staging

- Root: `.tpatch/local/intent-prepare/<slug>/stage-<12hex>/`, mode `0700`,
  suffix from the shipped `RandomHex12` (`internal/store/fsdurable.go:96-103`).
- Created with `MkdirAllAndSyncChain` so the whole chain is durable even when
  intermediate directories already existed
  (`internal/store/fsdurable.go:41-52`, and the rationale at
  `internal/store/fsdurable.go:1-10`).
- Before any byte is written under `.tpatch/local/`, the shipped
  local-ignore contract gate runs (`internal/rescap/scratch.go:46-62`): the
  path must be covered by the `.tpatch/local/` ignore rule and nothing under it
  may be tracked. Failure refuses the command before staging (exit 3,
  `local-lane-not-ignored`) — PIB-186, PIB-187.
- Each staged file is written, `Sync`ed and closed; then the stage directory is
  `Sync`ed. Staged bytes are therefore durable before the journal names them.
- **Raw provider responses stay here.** They are written into the staging tree,
  never into `artifacts/` (§11.4). This is an enumerated delta from
  `analyze`/`define`/`explore` (§12.6 D3).
- **Cleanup.** On success the staging tree is removed and its parent `Sync`ed.
  On failure it is **retained** and its repo-relative path is named in the
  failure report, so a failed generation is inspectable. Retained trees are
  removed by the next successful mutating `prepare` for the same slug, and by
  nothing else (§7.11, PIB-098, PIB-099).

### 7.4 Lock: authority, scope and honest limits

- **Path**: `.tpatch/local/intent-prepare/<slug>/prepare.lock`.
- **Acquisition**: `O_CREATE|O_EXCL|O_WRONLY`, mode `0600`, carrying a random
  nonce, `Sync`ed — the shipped pattern at
  `internal/cli/land_journal.go:629-648`.
- **Held by**: mutating `prepare` only, for the whole invocation from before
  recovery until after journal clear.
- **Not held by**: `--check`, `--dry-run`, and every other tpatch command.
- **Stale-lock policy**: a lock whose nonce matches the pending journal's
  recorded nonce is removed by recovery; a lock with any other nonce, or an
  unreadable lock, is **left untouched** and refuses the command — the shipped
  owned-lock discipline at `internal/cli/land_journal.go:675-698`.

**The honest limit.** This lock excludes only another mutating `prepare` on the
same slug. It does **not** exclude `tpatch define`, `tpatch cycle`, an editor,
or a script, because none of them take it, and this PRD does not add lock
acquisition to any existing command (that would be a behavior delta on shipped
verbs with its own deadlock and compatibility surface). What protects against
those writers is not exclusion but **detection**:

> **Publish-time revalidation.** Immediately before the first rename, under the
> lock, every publication-set entry's canonical file is re-inspected and its
> identity compared to the preimage captured at preflight. Any mismatch —
> including an artifact that appeared where preflight saw `absent` — aborts the
> invocation **before** the window opens, exit 5, zero canonical mutations
> (PIB-100, PIB-101, PIB-102).

The residual window is between revalidation and each individual rename. A write
that lands there is overwritten and its bytes are **not** archived, because the
archive captured the revalidated preimage. This PRD states that limit rather
than papering over it (ADR-035 D6, PIB-103).

### 7.5 Journal: location, schema, and why it is undo-only

**Location**: `.tpatch/local/intent-prepare/<slug>/journal.json`, mode `0600`,
written with the shipped durable writer `gitutil.DurableWriteFile`
(`internal/gitutil/index_snapshot.go:455-500`) and read with a **strict**
decoder that rejects unknown fields and trailing content — the discipline at
`internal/cli/land_journal.go:348-380` and
`internal/store/resource_publish.go:305-320`. A journal whose `version` is not
this build's is refused, never guessed
(`internal/cli/land_journal.go:56-58` is the version precedent).

**Location rationale**: gitignored `.tpatch/local/`, because the journal is
transient control state, is machine-local, and must never enter a commit. It
carries **no artifact content** except two small machine-written preimages
(§7.6).

```json
{
  "version": 1,
  "slug": "fix-model-id-translation",
  "mode": "regenerate",
  "lock_nonce": "9f13…",
  "stage_rel": ".tpatch/local/intent-prepare/fix-model-id-translation/stage-a1b2c3d4e5f6",
  "entries": [
    {
      "artifact_id": "analysis",
      "rel": ".tpatch/features/fix-model-id-translation/analysis.md",
      "action": "replace",
      "preimage": { "exists": true, "sha256": "…", "size": 4211, "mode": 420 },
      "preimage_blob": "…",
      "new_image": { "exists": true, "sha256": "…", "size": 5017, "mode": 420 },
      "staged_rel": ".tpatch/local/intent-prepare/…/stage-…/analysis.md"
    }
  ],
  "index_preimage_rel": ".tpatch/local/intent-prepare/…/index.preimage.json",
  "status_preimage_rel": ".tpatch/local/intent-prepare/…/status.preimage.json"
}
```

- **No wall-clock field.** ADR-027 D6 and ADR-033 D10 forbid it in tracked
  artifacts; this PRD applies the same rule to the journal so that two
  identical operations produce identical journals, which is what makes
  PIB-160's determinism test possible. `land`'s journal does carry
  `created_at` (`internal/cli/land_journal.go:109`); this design deliberately
  does not, and ADR-035 D7 records the divergence and its reason (recovery
  decides from evidence, never from time).
- **No content**, with two enumerated exceptions (§7.6).
- **`Phase` is deliberately absent.** `land` keeps one and marks it advisory
  (`internal/cli/land_journal.go:108-109`); this design omits it entirely so
  no reader can be tempted to decide from it.

**Undo-only, and why.** `land` must roll *forward* because `git commit` already
advanced HEAD, an irreversible act (`internal/cli/land_journal.go:11-23`).
Nothing in a prepare publication is irreversible: the new artifacts did not
exist before, and regenerating them is a re-runnable operation. So recovery
never completes a partial publication. It restores the old set and lets the
operator re-run. This is simpler, has strictly fewer failure modes, and cannot
publish a bundle whose staged half was pruned (ADR-035 D5).

### 7.6 Preimage and new-image identity

Identity is `(exists, sha256, size, mode)`; equality requires all four (or both
non-existent). This is the shipped `landJournalFileState` shape and comparison
(`internal/cli/land_journal.go:65-79`), reused rather than reinvented.

- **Canonical artifact preimages** are recovered from the intent archive
  (§9.2): the blob is written and fsynced *before* the journal is finalized, so
  by the time the window can open, every replaceable byte already has a durable
  copy at a content-addressed path. The journal stores the blob's hash, not its
  bytes.
- **Two exceptions** are stored as raw bytes inside the journal directory,
  because neither belongs in a tracked, content-addressed *intent* archive and
  both are small machine-written files:
  - `index.preimage.json` — the archive index's own prior bytes. Storing it in
    the archive would be circular.
  - `status.preimage.json` — `status.json`'s prior bytes. It is lifecycle
    metadata, not intent content.
  Both are `0600`, gitignored, and removed with the journal.
- New-image identities are computed from the staged files after they are
  fsynced, so the journal cannot describe bytes that are not durable.

### 7.7 Publication order, fsync and durability

Ordered algorithm. Every step is durable before the next begins.

1. Acquire lock (§7.4). Run recovery (§7.10) **before** anything else.
2. Inspect (accepted inspector, ADR-034 boundaries). Compute the publication
   set. Refuse here for every §6 admissibility failure — nothing has been
   written except the lock.
3. Stage and validate generated bytes (§7.3, §11.6). Any failure here aborts
   with zero canonical mutations.
4. **Revalidate** every entry's canonical identity against preflight (§7.4).
   Mismatch → abort, exit 5.
5. Write archive blobs for every `replace` entry; fsync each; fsync
   `blobs/`. Capture `index.preimage.json` and `status.preimage.json` into the
   journal directory; fsync them.
6. Write the journal; fsync it; fsync the journal directory. **The window is
   now armed.**
7. Rename staged → canonical in the fixed order of §7.2, fsyncing each entry's
   parent directory after its rename. Each individual rename uses the shipped
   single-file atomic writer semantics
   (`internal/store/store.go:878-917`): the staged file is already written,
   chmod'ed, synced and closed, so publication is exactly the rename plus the
   parent-directory sync.
8. Clear the journal (remove `journal.json`, both preimage files and the
   staging tree; fsync the journal directory).
9. Release the lock; fsync the directory (`internal/cli/land_journal.go:650-662`
   is the shipped release shape).

Step 8 is the point after which the transaction is invisible. Steps 6→7 are the
armed window; steps 7→8 are the T0 exposure window.

**File modes.** Created files use `0644` (the shipped default at
`internal/store/store.go:918-923`); replaced files preserve the existing
file's permission bits, exactly as `writeFileAtomic` already does
(`internal/store/store.go:871-876`). Directories use `0755` in the tracked tree
and `0700` in `.tpatch/local/`.

### 7.8 In-command rollback

If any rename in step 7 fails, the command rolls back **immediately**, in
reverse publication order:

| Entry action | Undo |
|---|---|
| `create` | Remove the canonical file, but **only** if its identity equals the new-image. Otherwise refuse (something else wrote it). |
| `replace` | Restore from the archive blob named in the journal, via the same stage-and-rename primitive, but **only** if the canonical identity equals the new-image. Otherwise refuse. |
| `index.json` | Restore from `index.preimage.json`. |
| `status.json` | Restore from `status.preimage.json`. |

Then clear the journal and release the lock. Exit `5`, report `outcome:
"rolled-back"`, and state in one line that the tree is exactly as it was.

If the rollback itself fails, the journal is **kept**, the lock is released,
and the command exits `6` with a report naming the journal path, the archive
directory and the specific entry that could not be restored. No further
automatic action is attempted (PIB-110, PIB-111).

### 7.9 Crash phases — enumerated, each with its recovery outcome

| Phase | Crash point | On-disk evidence | Recovery outcome |
|---|---|---|---|
| CP0 | before the lock exists | nothing | nothing to do |
| CP1 | lock held, before journal | lock, maybe a staging tree | remove owned lock; remove staging tree; proceed |
| CP2 | blobs written, before journal | lock, blobs, no journal | blobs are additive and orphaned; remove owned lock; proceed |
| CP3 | journal durable, before first rename | journal; all entries == preimage | clear journal; nothing to restore |
| CP4 | after rename *k* of *n* (`0 < k < n`) | journal; first *k* == new-image, rest == preimage | undo the *k* published entries; clear |
| CP5 | after the last artifact rename, before `index.json` | journal; artifacts new, index old | undo all published entries; clear |
| CP6 | after `index.json`, before `status.json` | journal; artifacts + index new, status old | undo all published entries; clear |
| CP7 | after `status.json` rename, before journal clear | journal; **every** entry == new-image | **complete**: clear journal only; publish nothing, undo nothing |
| CP8 | after journal clear, before lock release | stale owned lock only | remove owned lock; proceed |
| CP9 | any of the above, plus a third party wrote one of the entries | at least one entry matches neither preimage nor new-image | **refuse** (exit 6), preserve every file, the journal and the archive; name them |

CP7 is why recovery decides from **evidence, not phase**: the process died after
the semantically final act, and no marker write could have made that
distinguishable without introducing its own crash point. This is the same
reasoning the shipped land journal records
(`internal/cli/land_journal.go:11-23`) and applies to the opposite direction.

### 7.10 Recovery: entry points, idempotency, cleanup

**Entry points.**

- **Automatic**: every mutating `prepare` for that slug runs recovery under the
  lock before any other work, exactly as `land` does
  (`internal/cli/land_journal.go:445-482`).
- **Diagnostic**: `tpatch doctor` gains a check that *reports* a pending
  journal and never acts on it (§12.5).
- **Nothing else.** `--check`, `--dry-run`, `next`, `cycle`, `status`,
  `verify`, `record`, `land`, `reconcile` and the phase commands neither
  recover nor refuse on a pending journal (§7.12, PIB-118 … PIB-123).

**Why not "every relevant command must recover or refuse".** That alternative
was evaluated and rejected (§8.2 alternative J2): it would put a write path
into read-only commands, add a lock acquisition to eleven shipped verbs, and
make a stale journal able to wedge `tpatch status`. The chosen model is
**pointer-free and generation-free**: a pending journal is a fact about one
slug's `.tpatch/local/` lane, it never changes the meaning of any canonical
file, and the canonical tree remains self-describing without it. Because
recovery is undo-only, a never-recovered journal leaves the tree in a state
that is *already* one of the enumerated CP-phases — mixed, but classified
truthfully by the accepted read-only check.

**Idempotency.** Recovery is a function of evidence, so running it twice is a
no-op the second time: the journal is gone. A recovery interrupted mid-undo
leaves a state that is itself a valid CP-phase and is re-recoverable (PIB-113).

**Cleanup.** Recovery removes: the journal, both preimage files, the staging
tree named in the journal, and any other `stage-*` directories under that
slug's lane. It never removes archive blobs, never removes the archive index,
and never touches another slug's lane (PIB-114, PIB-115).

### 7.11 Concurrency matrix

| Concurrent actor | Behavior |
|---|---|
| A second mutating `prepare`, same slug | Refused by the `O_EXCL` lock, exit 6, `transaction-locked`; the first is unaffected (PIB-124). |
| A second mutating `prepare`, different slug | Independent lanes, no interaction (PIB-125). |
| `prepare --check`, any slug | Never blocked; reports what is on disk; may report `unstable` inside the window (PIB-206). |
| An editor writing `spec.md` between preflight and revalidation | Detected at revalidation → abort before the window, exit 5, nothing written (PIB-100). |
| An editor writing `spec.md` inside the window | Overwritten; bytes not archived. Stated limit (§7.4), ADR-035 D6, PIB-103. |
| `tpatch define <slug>` concurrently | Not excluded (it takes no lock). Its write is caught by revalidation if it lands before the window, and lost if it lands inside it. Same limit, same disclosure (PIB-104). |
| `git checkout` / `git stash` moving `.tpatch/` under the command | Same class as an editor: detected before the window, lost inside it. `prepare` runs no Git command and holds no Git lock (PIB-105). |
| A Git worktree or a nested repository | `prepare` performs no Git operation at all, so worktree and index state are untouched by construction (PIB-106, source-scanned by PIB-107). |

### 7.12 What the read-only check reports while a journal is pending

**Nothing about the journal.** This is a decision, not an omission:

1. The accepted `--check` report schema is frozen and its field set is closed
   (`docs/prds/PRD-artifact-validation-and-provenance.md:2420-2576`). Adding a
   field would be a `schema_version` change to an accepted contract that this
   PRD is explicitly forbidden to modify.
2. `--check`'s subject is canonical artifacts. A journal is transient control
   state in a gitignored lane; it is not an artifact and has no state in the
   accepted nine-value enum.
3. What the operator actually needs — "is a prepare half-done here?" — is
   answered truthfully by `--check` anyway, because a mixed set reports as
   mixed, and by the new `doctor` check by name (§12.5).

So: `prepare --check` is byte-identical whether or not a journal exists, given
identical canonical bytes (PIB-207). The pending transaction is surfaced by
`doctor` and resolved by the next mutating `prepare`.

## 8. Non-destructive overwrite — the architecture gate

### 8.1 The question

`--regenerate` overwrites files a human may have hand-authored. The dispatch
constraint is absolute: **it must not lose prior hand-authored bytes.** The
question this section answers is *by what mechanism*, and whether that
mechanism is a persistent representation — because selecting one fires the
architecture gate and requires an ADR before acceptance.

### 8.2 Alternatives evaluated

| # | Option | For | Against | Verdict |
|---|---|---|---|---|
| **H1** | **Refuse overwrite entirely.** No `--regenerate`; the operator deletes files by hand first. | Zero new machinery; no new object; trivially safe. | Does not satisfy the dispatch, which requires an explicit coherent overwrite route. Pushes the destructive act onto `rm`, which has no archive at all — strictly *less* safe in practice. | **Rejected.** |
| **H2** | **Rely on Git.** Refuse `--regenerate` unless `.tpatch/features/<slug>/` is tracked and clean; recovery is `git checkout`. | No new object; uses a tool the user already has; zero storage growth. | `.tpatch/` is frequently untracked — the CLI ships a dedicated detector for exactly that condition (`internal/cli/cobra.go:3405-3407`) and warns about it. A Path B agent that authored three files five minutes ago has committed none of them. The guarantee would evaporate in the single most common case it is needed. It also imports Git availability into a command that otherwise runs none. | **Rejected as the sole mechanism.** Retained as a *reported* advisory (§10.3). |
| **H3** | **Ephemeral rollback-only journal.** Keep preimages only for the duration of the transaction; delete them at commit. | Satisfies T1/T2 with no durable growth; no new tracked object; no ADR. | Satisfies crash-safety and *not* the actual requirement: after a **successful** `--regenerate`, the prior bytes are gone forever. The dispatch's requirement is about the success path, not the crash path. | **Rejected for `--regenerate`.** Adopted for the transaction half (§7.5) — the journal *is* this, and it is sufficient there precisely because the archive covers the success path. |
| **H4** | **Durable immutable, content-addressed intent-generation snapshots; canonical files remain the sole authority.** Replaced bytes are copied to `artifacts/intent-archive/blobs/<sha256>.blob`; an `index.json` names which artifacts each generation replaced. No reader consults the archive to determine current state. | Prior bytes survive success, crash, clone and machine change. Content addressing dedupes: regenerating to identical content writes zero new bytes. Directly reuses the shipped, reviewed resource-capture shape — immutable content-addressed set plus one atomically-rewritten pointer (`internal/store/resource_publish.go:1-9,219-278`). Canonical readers are entirely unaffected. | Grows the tracked tree without bound (mitigated: content-addressed, so only *distinct* content costs bytes; §16 R6 and Q5 track pruning). Adds one tracked object and therefore **fires the architecture gate**. | **SELECTED.** |
| **H5** | **Pointer-based generation directories.** Canonical paths become pointers/symlinks into `generations/<id>/`. | Cheap "switch the whole bundle" semantics; history is a first-class directory. | Breaks every existing reader: `os.ReadFile` on `spec.md` now depends on link resolution, and the accepted inspector **refuses symlinks by design** (`docs/prds/PRD-artifact-validation-and-provenance.md:1768`), so the bundle would classify as `symlink-refused` — the check would call every prepared feature unsafe. Also breaks Git checkout on Windows without developer mode. | **Rejected outright.** |
| **H6** | **Sub-record on `FeatureStatus` / a dedicated manifest carrying prior content or hashes.** | Written atomically with state by the one writer (`internal/store/store.go:368-377`); the ADR-031 D1 argument. | A hash is not the bytes: it proves change, it does not recover content. Inlining content into `status.json` bloats the hottest file in the feature directory and makes every state transition rewrite it. And a *hash of an artifact* is exactly the shape §9.6 must avoid being read as a provenance claim. | **Rejected.** |

### 8.3 Selection

**H4 is selected**, with the transaction half of H3 (§7.5) retained for
crash-safety and H2 demoted to an advisory (§10.3).

### 8.4 The gate fires — ADR-035 is required

H4 is a **persistent history representation**: a new durable, tracked object
with its own wire format, its own identifiers and its own lifecycle. Per the
dispatch and per AGENTS.md's "ADR on every architecture decision" rule, this
PRD therefore creates
[`ADR-035-intent-bundle-publication-and-history.md`](../adrs/ADR-035-intent-bundle-publication-and-history.md)
as **Proposed rev-0**, adds it to the ADR index, and requires it to be reviewed
together with this PRD. **Neither may be accepted alone.**

### 8.5 What this selection is NOT

Two things must not be inferred from it, and both are asserted mechanically:

1. **It is not a provenance representation, and it does not fire the WP-005
   provenance trigger.** That trigger fires on "selection of a persistent
   provenance representation" and on nothing else
   (`docs/prds/PRD-artifact-validation-and-provenance.md:2966-2972`). The
   archive records *bytes that existed at some point at a path*. It records no
   author, no agent, no model, no Path A/Path B tag, and no source claim. It
   cannot answer "who wrote this", and nothing in this PRD's output claims it
   can. `prepare --check` keeps emitting the constant `provenance: unknown` for
   every artifact, unchanged (§9.6, PIB-140 … PIB-144).
2. **It is not a licence to cite ADR-034 as precedent for persistence.**
   ADR-034 D14 forbids exactly that
   (`docs/prds/PRD-artifact-validation-and-provenance.md:2957-2961`). ADR-034 is
   reused in this PRD for one thing only: the rooted, race-safe **read**
   boundary. Every write path in this document is governed by ADR-035
   (PIB-145).

## 9. The intent archive

Normative source: ADR-035 D8–D13. This section states the product-visible
contract.

### 9.1 Layout

```text
.tpatch/features/<slug>/artifacts/intent-archive/
├── index.json                     ← tracked manifest, canonical JSON, atomically rewritten
└── blobs/
    └── <64 lowercase hex>.blob    ← immutable, content-addressed prior bytes
```

Tracked, not gitignored. The bytes archived are the bytes of files that live at
tracked paths in the same directory tree, so archiving them introduces **no new
exposure class** — the identical content was already committable at
`analysis.md`. Putting the archive in `.tpatch/local/` instead was rejected
because a recovery guarantee that vanishes on a fresh clone is not a recovery
guarantee (ADR-035 D8).

### 9.2 Blobs

- Name: the lowercase-hex SHA-256 of the file's exact bytes, plus `.blob`. The
  hash is over the **raw bytes**, not a canonicalization.
- Immutable: an existing blob whose bytes already equal the content is **never
  rewritten**; the publication simply reuses it. Content-addressed dedupe means
  regenerating to previously-seen content costs zero new bytes — the shipped
  idempotency argument at `internal/store/resource_publish.go:240-246`.
- An existing blob file whose bytes **differ** from the content that hashes to
  its name is a corruption and refuses the invocation
  (`archive-blob-corrupt`, exit 3) rather than being overwritten — the shipped
  `batch-file-corrupt` disposition (`internal/store/resource_publish.go:199-203`).
- Written and fsynced before the journal is finalized, so a preimage is durable
  before it can be needed (§7.7 step 5).
- Never garbage-collected in v1. An orphaned blob is normal and permanent.

### 9.3 `index.json`

```json
{
  "schema_version": 1,
  "feature": "fix-model-id-translation",
  "generations": [
    {
      "generation_id": "3f0c…",
      "mode": "regenerate",
      "replaced": [
        { "artifact_id": "analysis",         "path": "analysis.md",             "blob": "a91e…", "size_bytes": 4211 },
        { "artifact_id": "spec",             "path": "spec.md",                 "blob": "77bd…", "size_bytes": 1902 },
        { "artifact_id": "analysis_sidecar", "path": "artifacts/analysis.json", "blob": "c40a…", "size_bytes": 733 }
      ]
    }
  ]
}
```

- `schema_version: 1`. Fixed key order by Go struct declaration order — the
  same mechanism the shipped report schemas use, and **no Go map anywhere in
  the wire format** (ADR-033 D11, restated at
  `internal/store/canonjson.go:11-17`).
- `generation_id` = lowercase-hex SHA-256 over the **canonical** encoding of
  `{feature, mode, replaced[]}` with `replaced` sorted by `artifact_id`. It is
  content-addressed and deterministic — the shipped `ComputeBatchID` shape
  (`internal/store/resource_publish.go:153-161`), whose derivation lives at
  exactly one point so every consumer agrees by construction
  (`internal/store/resource_publish.go:131-142`).
- `replaced` is sorted by `artifact_id`; `generations` is in publication order.
- **No wall-clock field anywhere.** ADR-027 D6, ADR-033 D10.
- **Idempotent append**: if a computed `generation_id` already exists in the
  index, the entry is **not** appended again. The archive is therefore a *set*
  of distinct generations, not a chronology — the same honest disposition the
  shipped capture design records (`docs/feature-layout.md:98-100`).
- `generations` ordering is publication order and carries **no** claim of
  wall-clock ordering, because no clock is recorded.
- A `generation_id` collision between two different canonical bodies refuses
  the invocation (`archive-generation-id-collision`, exit 3) — the shipped
  `batch-id-collision` disposition (`internal/store/resource_publish.go:199-203`).

### 9.4 Determinism

Two invocations that replace the same artifacts with the same prior bytes
produce **byte-identical** blobs and a byte-identical `index.json` entry. The
whole archive is a pure function of the sequence of replaced content — no
clock, no PID, no hostname, no random suffix reaches it (the random suffix is
confined to the gitignored staging tree). PIB-160, PIB-161, PIB-162.

### 9.5 Recovery UX in v1

The success report names, for each replaced artifact, the repo-relative blob
path. Recovery is an ordinary file copy:

```text
cp .tpatch/features/<slug>/artifacts/intent-archive/blobs/<hash>.blob \
   .tpatch/features/<slug>/spec.md
```

No `--restore` verb ships in v1 (non-goal 6, Q4). The report must state the
copy form explicitly and must not imply a verb that does not exist (PIB-163).

### 9.6 Why the archive is not provenance — stated so it cannot drift

| Question a provenance record answers | Does the archive answer it? |
|---|---|
| Who or what authored these bytes? | **No.** Nothing is recorded about the author. |
| Was this artifact produced by Path A or Path B? | **No.** `mode` names the *tpatch invocation mode that replaced it*, not the origin of the replaced bytes. A hand-authored file replaced by `--regenerate` records `mode: "regenerate"` — a fact about the replacing act. |
| Which model/provider produced them? | **No.** No provider, model, endpoint or configuration is recorded. |
| Is the current `spec.md` provider-generated? | **No.** The archive says nothing about the *current* file. |
| Have these exact bytes existed at this path before? | **Yes** — and that is the only question it answers. |

Consequently:

- `prepare --check` continues to report the constant `provenance: unknown` for
  every artifact, and **must not** consult the archive (PIB-140, source-scanned
  by PIB-141).
- No mutating report field may assert Path A vs Path B for a feature (PIB-142).
- The forbidden-inference list of the accepted contract
  (`docs/prds/PRD-artifact-validation-and-provenance.md:2904-2920`) is extended
  by exactly one entry — **the intent archive** — and PIB-143 asserts that
  entry exists in the shipped source.
- Any future PRD that wants to answer the first four rows must write the
  provenance ADR first. This PRD does not, and ADR-035 D9 says so normatively.

## 10. CLI output, exit codes and precedence

### 10.1 Stream routing

Composed with the **real** root error printer, which emits `error: %v` for
every non-nil `RunE` error (`internal/cli/cobra.go:33-39`) before
`exitCodeFor` maps the code (`internal/cli/cobra.go:43-52`). This PRD inherits
that composition from the accepted contract
(`docs/prds/PRD-artifact-validation-and-provenance.md:2367-2419`) and does not
re-litigate it.

| Flags | stdout | stderr |
|---|---|---|
| (none) | human report + progress lines | one `error:` line on every nonzero exit |
| `--json` | the JSON report, one document, trailing newline | one `error:` line on every nonzero exit |
| `--quiet` | one summary line only | one `error:` line on every nonzero exit |
| `--json --quiet` | the JSON report only | one `error:` line on every nonzero exit; **empty only on exit 0** |

**Progress vs final report.** Progress lines ("[1/3] Generating analysis…")
are written to **stderr**, never stdout, and are suppressed entirely under
`--json` or `--quiet`. This keeps stdout a single parseable document under
`--json`, which the shipped `next --format harness-json` convention already
assumes (`internal/cli/phase2.go:381-384`). The existing phase commands print
progress to stdout (`internal/cli/cobra.go:621-626`); this command deliberately
does not, and §12.6 D5 enumerates that as a *new-surface* choice, not a change
to those commands. PIB-170, PIB-171.

### 10.2 JSON report (v1)

`schema_version: 1`. Fixed key order by struct declaration order. No Go map.
This is a **different schema** from `--check`'s: `command` distinguishes them
and consumers must switch on it (PIB-172).

```json
{
  "schema_version": 1,
  "command": "prepare",
  "mode": "regenerate",
  "dry_run": false,
  "slug": "fix-model-id-translation",
  "outcome": "published",
  "action": "regenerate",
  "feature_state": "defined",
  "disclaimer": "Structural presence only. This report does not certify semantic quality.",
  "artifacts": [
    {
      "id": "analysis",
      "path": ".tpatch/features/fix-model-id-translation/analysis.md",
      "role": "required",
      "disposition": "regenerated",
      "generator": "provider",
      "archived_blob": "a91e…"
    },
    {
      "id": "spec",
      "path": ".tpatch/features/fix-model-id-translation/spec.md",
      "role": "required",
      "disposition": "regenerated",
      "generator": "heuristic",
      "archived_blob": "77bd…"
    },
    {
      "id": "exploration",
      "path": ".tpatch/features/fix-model-id-translation/exploration.md",
      "role": "required",
      "disposition": "regenerated",
      "generator": "heuristic",
      "archived_blob": ""
    },
    {
      "id": "analysis_sidecar",
      "path": ".tpatch/features/fix-model-id-translation/artifacts/analysis.json",
      "role": "optional",
      "disposition": "regenerated",
      "generator": "provider",
      "archived_blob": "c40a…"
    }
  ],
  "archive": {
    "generation_id": "3f0c…",
    "blobs_dir": ".tpatch/features/fix-model-id-translation/artifacts/intent-archive/blobs"
  },
  "advisories": [
    {
      "code": "provider-fallback-heuristic",
      "artifact_id": "spec",
      "message": "The provider call failed and the heuristic generator was used instead. Re-run after fixing the provider to replace it."
    }
  ]
}
```

Closed vocabularies:

| Field | Closed set |
|---|---|
| `mode` | `generate`, `manual`, `regenerate` (`check` never emits this schema) |
| `outcome` | `published`, `no-op`, `planned`, `refused`, `rolled-back`, `recovery-refused` |
| `action` | `none`, `adopt`, `complete`, `regenerate` |
| `disposition` (per artifact) | `preserved`, `generated`, `regenerated`, `untouched`, `absent-optional` |
| `generator` | `provider`, `heuristic`, `` (empty for anything not generated this run) |
| `advisories[].code` | the ten codes of §10.3 |

**`generator` is a statement about this process, not about the file's
history.** It is emitted transiently, is never persisted to any tracked
artifact by this field name, and §9.6 governs why it is not provenance
(PIB-144).

**Refusal shape.** On any refusal, `outcome` is `refused`, `artifacts` carries
the per-artifact dispositions the plan *would* have had (or `[]` when the
refusal happens before inspection, e.g. workspace or slug), `archive` is
omitted, and a `refusal` object carries a closed `code`, a message and a
self-contained remediation (PIB-173, PIB-174).

### 10.3 Advisory catalog (closed, ten codes)

| Code | Fires when | Says |
|---|---|---|
| `provider-not-configured` | no provider configured or reachable | the heuristic generator was used; how to configure one |
| `provider-fallback-heuristic` | a provider call failed and the heuristic generator produced the artifact | which artifact; that re-running replaces it |
| `analysis-preserved-sidecar-untouched` | `analysis.md` preserved, sidecar absent | the sidecar is not synthesized for a preserved analysis, and its absence is not a defect |
| `bundle-untracked-in-git` | `.tpatch/` is not tracked (`internal/cli/cobra.go:3405-3407`) | the archive is the only recovery route here; committing `.tpatch/` adds a second |
| `archive-blob-reused` | a replaced artifact's content already existed as a blob | zero new bytes were written for it |
| `archive-generation-duplicate` | the computed `generation_id` already exists in the index | no index entry was appended; the archive is a set |
| `staging-retained` | a failure left a staging tree | its repo-relative path; that the next successful run removes it |
| `recovered-prior-transaction` | recovery undid a pending transaction before this run | which entries were restored |
| `feature-state-below-defined` | the source state was `requested` or `analyzed` | the transition that was performed |
| `heuristic-mode-recorded-in-sidecar` | the sidecar was generated in heuristic mode | that `heuristic_mode: true` is set in the sidecar, exactly as `RunAnalysis` does today (`internal/workflow/workflow.go:25,208`) |

Advisory selection is a **total function of observed state**, never of artifact
id: every advisory's precondition is a predicate over the inspection result and
the transaction outcome, and PIB-175 asserts no advisory can contradict its own
artifact row.

### 10.4 Exit codes

Per-command contract, per `SPEC.md:137`.

| Code | Meaning | Report emitted? | Wrote anything? |
|---|---|---|---|
| `0` | success: published, no-op, plan OK, or `--check` ready | yes | only on `published` |
| `1` | generic CLI/parse error (arity, unknown flag, mutually exclusive flags), or an unexpected internal error | no (parse) / yes (internal) | no |
| `2` | **not-ready refusal**: `--manual` on an incomplete bundle; default mode on a `present-empty` required artifact; staged-output validation failure | yes | no |
| `3` | **cannot-act refusal**, two documented populations distinguished by `refusal.code`: (a) *indeterminate* — workspace not initialized, feature not found, unsafe slug, `status.json` malformed/unreadable, an artifact in an unsafe/unstable state, archive corruption, local-lane ignore-contract failure; (b) *lifecycle-state* — the source state does not permit preparation (§12.2) | yes | no |
| `4` | **retired.** The reserved-surface population (`prepare` without `--check`) no longer exists, and no new population is bound to it. `prepare` never exits 4. | — | — |
| `5` | **transaction aborted**: revalidation mismatch, generation failure after staging, or a rename failure that was **successfully rolled back**. The tree is exactly as it was. | yes | no net change |
| `6` | **manual intervention required**: rollback failed, recovery found divergent evidence, or the transaction lock is held by another process. Everything is preserved. | yes | possibly a partial publication, fully described |

`0`/`2`/`3` keep the meanings the accepted `--check` contract already binds
(`docs/prds/PRD-artifact-validation-and-provenance.md:1981-1987`), so the same
condition yields the same code in both halves of the verb. Population (b) of
exit `3` additionally matches the repository's shipped state-refusal
convention: `reject`, `reopen` and `refuseIfUnappliedState` all bind
state-transition refusal to `3` (`internal/cli/reject.go:45-48,68-70`;
`internal/cli/feature_unapply.go:464-473`; `SPEC.md:148`). Collapsing lifecycle
refusal into `3` rather than inventing a fifth code is what lets `prepare`
reuse the shipped `refuseIfUnappliedState` helper unchanged. `5` and `6` are
new and have no read-mode population.

### 10.5 Precedence (first match wins)

1. Cobra/pflag parse, arity or mutual-exclusion error → `1`. Nothing else
   runs; the text is pflag's and is outside this command's schema, exactly as
   ADR-034 D17 scopes it.
2. `--check` selected → the **entire accepted contract** takes over, including
   its own precedence, codes and bytes. Nothing below applies (PIB-198).
3. Canonical slug validation → `3` (`slug-unsafe`), reusing the accepted
   grammar and its no-echo rule
   (`docs/prds/PRD-artifact-validation-and-provenance.md:696-772`).
4. Workspace discovery → `3` (`workspace-not-initialized`).
5. Platform confinement support → `3`
   (`workspace-unsupported-platform`), reusing ADR-034 D5's fail-closed
   allowlist unchanged.
6. Local-lane ignore contract → `3` (`local-lane-not-ignored`).
7. Feature directory / `status.json` population → `3`
   (`feature-not-found`, `status-malformed`, `status-unreadable`).
8. `request.md` capture (generating modes only) → `3`
   (`request-unreadable`).
9. Pending-transaction recovery → `6` if divergent or the lock is foreign;
   otherwise recovery completes silently and evaluation continues.
10. Lifecycle-state gate (§12.2) → `3` (`state-refused`).
11. Artifact admissibility (§6.1 / §6.3 tables) → `3` for unsafe/unstable, `2`
    for `present-empty` in default mode, `2` for `--manual` not-ready.
12. Generation, staging, staged-output validation → `2` on validation failure,
    `5` on an unrecoverable generation failure after staging began.
13. Revalidation mismatch → `5`.
14. Publication failure → `5` if rolled back, `6` if rollback failed.
15. Otherwise → `0`.

The order is load-bearing in two places, and both have rows: the slug is
validated before any path is composed (PIB-176), and recovery runs before the
lifecycle gate so a pending transaction is never left behind by a refusal that
happens to come first (PIB-177).

### 10.6 Human output

Deterministic layout, fixed section order, no color, no wall-clock, no
duration, no absolute path, no symlink target, no artifact content:

```text
Feature: fix-model-id-translation   (state: defined)
Mode:    regenerate

  analysis.md              regenerated   (provider)   archived a91e…
  spec.md                  regenerated   (heuristic)  archived 77bd…
  exploration.md           regenerated   (heuristic)
  artifacts/analysis.json  regenerated   (provider)   archived c40a…

Archive: .tpatch/features/fix-model-id-translation/artifacts/intent-archive/blobs
  Restore a prior file with: cp <blobs-dir>/<hash>.blob <path>

Advisory: The provider call failed for spec.md and the heuristic generator was
          used instead. Re-run after fixing the provider to replace it.

Structural presence only. This report does not certify semantic quality.
```

The disclaimer is the **frozen** accepted string
(`docs/prds/PRD-artifact-validation-and-provenance.md:1796-1798`) and is
asserted byte-for-byte in both surfaces (PIB-178).

`--quiet` prints exactly one line per invocation, ending in the outcome token:

```text
prepare fix-model-id-translation: regenerate published (4 artifacts, 3 archived)
```

### 10.7 Refusal remediation is self-contained

Every refusal names only shipped commands, shipped flags and repo-relative
paths that exist. It must not cite a PRD path, an ADR path, an issue URL or any
`docs/` file — a shipped binary's diagnostic cannot depend on a document the
user does not have. This is the accepted contract's rule
(`docs/prds/PRD-artifact-validation-and-provenance.md:374-381`), applied to
every refusal code in §10.4 (PIB-179, mechanically guarded by PIB-180).

## 11. Path A generation

### 11.1 Pure, staged generators — the extraction

The three phase functions cannot be called (§1.3). The implementation extracts
their **generation** half from their **publication** half:

| New (pure) | Extracted from | Returns |
|---|---|---|
| `GenerateAnalysis(ctx, in AnalysisInput) (AnalysisResult, GenNote, error)` | `internal/workflow/workflow.go:35-88` (everything above the first `WriteArtifact`) | the struct, plus which generator ran and any raw response |
| `RenderAnalysisMD(result, slug) string` | already pure (`internal/workflow/workflow.go:351-401`) | markdown bytes |
| `GenerateSpec(ctx, in DefineInput) (string, GenNote, error)` | `internal/workflow/workflow.go:111-150` | markdown bytes |
| `GenerateExploration(ctx, in ExploreInput) (string, GenNote, error)` | `internal/workflow/workflow.go:159-195` | markdown bytes |

Hard constraints on the extracted functions, each source-scanned:

1. They take **no `*store.Store`** and hold no writer (PIB-184).
2. They perform **no filesystem write** of any kind (PIB-185).
3. Their inputs are plain values: request text, prior-artifact text, file tree,
   guidance text, provider handle and config.
4. `RunAnalysis`, `RunDefine` and `RunExplore` are **refactored to call them**
   and keep their current observable behavior byte-for-byte, so `analyze`,
   `define`, `explore` and `cycle` are unchanged (PIB-186, proved against
   pre-change goldens per §17).

`captureFileTree` and `readGuidanceFiles`
(`internal/workflow/workflow.go:249-253,290-301`) are already pure reads and
are reused unchanged.

### 11.2 Context flow and dependency order

Generation order is fixed: **analysis → spec → exploration**, matching the
shipped data dependencies (`internal/workflow/workflow.go:113-129,161-172`).

Each generator's context is assembled from the **effective** bundle — that is,
staged output from this run where it exists, and the **existing canonical
bytes** where an artifact was preserved:

| Generator | Consumes |
|---|---|
| analysis | `request.md` |
| spec | `request.md`, effective `analysis.md`, effective `artifacts/analysis.json` (when present) |
| exploration | `request.md`, effective `analysis.md`, effective `spec.md`, file tree |

Two rules make "effective" safe:

1. **Existing artifacts are read through the accepted inspector's bounded,
   rooted capture** — same cap, same refusals, same instability handling
   (ADR-034 D1, D8, D9). A preserved artifact that cannot be safely read has
   already refused the invocation at §6.1 step 2, so no generator ever receives
   partially-read or unsafely-resolved bytes (PIB-187).
2. **Consuming a preserved artifact claims nothing about it.** The generated
   `spec.md` does not record that its analysis was hand-authored, and the
   report does not either (PIB-142). Today's `RunDefine` already reads whatever
   `analysis.md` is on disk without asking where it came from
   (`internal/workflow/workflow.go:118-121`); this preserves that behavior
   rather than inventing a claim.

### 11.3 Provider vs heuristic — semantics and observable output

Preserved from the shipped behavior, deliberately:

| Situation | Behavior | Observable |
|---|---|---|
| No provider configured | heuristic generators run (`internal/workflow/workflow.go:82-84,144-146,189-191`) | `generator: "heuristic"`, advisory `provider-not-configured`, exit 0 |
| Provider configured, call fails or validation fails after retries | heuristic generator for that artifact | `generator: "heuristic"`, advisory `provider-fallback-heuristic`, exit 0 |
| Provider succeeds | provider output | `generator: "provider"`, no advisory |

- The provider is loaded with the **non-probing** loader `cycle` uses
  (`internal/cli/phase2.go:55`), not the probing loader the single-phase
  commands use (`internal/cli/cobra.go:609-612`). A missing or unreachable
  provider is a fallback condition, not a command failure — which is what makes
  `prepare` usable offline, per locked-in decision 7 of `CLAUDE.md`.
- Fallback still yields a **complete** new set, so WP-005 Agreed item 7's
  "complete prior set or complete new set"
  (`docs/whitepapers/WP-005-spec-driven-workflows.md:80-81`) holds: a provider
  failure never produces a half-bundle.
- The heuristic sidecar keeps `heuristic_mode: true`
  (`internal/workflow/workflow.go:25,208`), byte-compatible with what
  `analyze` writes today. That field is pre-existing persisted data, not a new
  provenance representation (PIB-146).
- **There is no `--require-provider` in v1.** Q3 records it; the default is
  fallback, matching every existing phase command.

### 11.4 Retry and raw responses

- Retry uses the shipped `GenerateWithRetry` loop and its corrective-prompt
  behavior (`internal/workflow/retry.go:77-131`), with `MaxRetries` from config
  and `--no-retry` mapping to the shipped `WithDisableRetry`
  (`internal/workflow/retry.go:47-58,83-85`).
- **`RetryOptions.Store` is left nil.** As shipped, a non-nil `Store` makes the
  retry loop write `artifacts/raw-<prefix>-response-<n>.txt` into the canonical
  tree mid-generation (`internal/workflow/retry.go:105-109`) — a canonical
  write before the transaction has decided anything. `prepare` instead
  captures raw responses through a sink and writes them into the **staging
  tree** (§7.3), where they are gitignored and never published (PIB-188,
  PIB-189).
- Consequence, enumerated as delta §12.6 D3: after `prepare`, no
  `raw-*-response-*.txt` appears under `artifacts/`, unlike after `analyze`.
  Failed runs keep them in the retained staging tree and the report names that
  path (PIB-190).

### 11.5 Timeout budget

- One `context.WithTimeout` for the whole invocation, default `180s`,
  overridable with `--timeout`. The single-phase commands each use `60s`
  (`internal/cli/cobra.go:629`); `prepare` may make three calls, so a 60s total
  would fail routinely, and three independent 60s budgets would make the
  command's worst case unbounded-by-flag.
- The deadline covers generation only. It **cannot** interrupt the filesystem
  reads of the inspection half — ADR-034 D16 withdrew every bounded-runtime
  claim, and this PRD does not resurrect one (PIB-191).
- A deadline expiry during generation is a generation failure: it falls back to
  the heuristic generator for that artifact, exactly like any other provider
  error, so a slow provider yields a complete bundle rather than a refusal
  (PIB-192).
- If the deadline expires **after** staging and before publication, the command
  publishes anyway: the deadline governs generation, not the transaction, and
  aborting a validated publication because a clock expired would be strictly
  worse (PIB-193).

### 11.6 Staged-output validation — before any canonical write

Every staged byte passes all of these before the window can open. Failure of
V1–V5 is exit `2`; failure of V6 is exit `5`, because a staged file whose
identity moved is a transaction-integrity failure rather than an output-quality
one. In both cases: zero canonical mutations, staging retained.

| # | Check | Rationale |
|---|---|---|
| V1 | non-empty after whitespace trim | the accepted `present-empty` rule (`docs/prds/PRD-artifact-validation-and-provenance.md:1765`) — publishing an artifact the check would call empty is incoherent |
| V2 | size ≤ the accepted `MaxArtifactBytes` | publishing something the check would call `oversize` is incoherent |
| V3 | no NUL byte | these are Markdown/JSON text artifacts; a NUL indicates a truncated or binary provider response |
| V4 | valid UTF-8 | same |
| V5 | sidecar parses as a JSON **object** | the accepted `invalid-structured` rule (`docs/prds/PRD-artifact-validation-and-provenance.md:1772`); the shipped validator already enforces object-ness at generation time (`internal/workflow/workflow.go:62`) |
| V6 | the staged file's post-fsync identity equals the identity recorded for it | nothing may be published that was not durably staged |

**V1–V6 are structural only.** No check inspects headings, length,
plausibility, or whether the text is on-topic. §14.1's disclaimer is what the
report says instead (PIB-194).

**The self-consistency invariant.** After a successful `published` outcome,
running the accepted `prepare <slug> --check` in the same tree must report
`ready` and exit 0. This is the single strongest end-to-end assertion in the
matrix (PIB-195) and is what V1/V2/V5 exist to guarantee.

## 12. Lifecycle and compatibility

### 12.1 `--check` is byte-identical

The accepted mode keeps its grammar, its precedence, its report schema, its
exact bytes, its exit envelope and its zero-mutation contract. The only
permitted change to the shipped `prepare` command file is the addition of the
new flags and the mode dispatch **above** the `--check` path. Regressions:
PIB-198 (bytes, human), PIB-199 (bytes, JSON), PIB-200 (exit codes across all
populations), PIB-201 (zero mutation), PIB-202 (`provenance: unknown`
constant), PIB-203 (disclaimer string), PIB-204 (abort shapes), PIB-205
(`artifacts` ⇔ `abort` invariant), PIB-207 (byte-identical with a pending
journal).

### 12.2 Allowed source states

| State | Mutating `prepare` | Why |
|---|---|---|
| `requested` | **allowed** | The ordinary entry point. Generates the whole bundle; ends `defined`. |
| `analyzed` | **allowed** | Completes the bundle from an existing analysis. |
| `defined` | **allowed** | Idempotent: completes a missing `exploration.md`, adopts under `--manual`, or regenerates. May be a no-op success (§6.1 step 4). |
| `implementing` | refused, exit 3 | A recipe already derives from the current intent (`internal/workflow/workflow.go` implement phase; `docs/feature-layout.md:85`). Rewriting intent underneath it desynchronizes the pair with nothing recording the mismatch. |
| `applied`, `active` | refused, exit 3 | A canonical patch exists (`docs/feature-layout.md:19,34`). Same argument, plus `verify`'s later `intent_files_present` check would compare against changed intent. |
| `reconciling`, `reconciling-shadow` | refused, exit 3 | A reconcile session is live; its inputs must not move under it. |
| `blocked` | refused, exit 3 | Post-implementation; resolve the blocker first. |
| `upstream_merged` | refused, exit 3 | Terminal, and asserts the opposite verdict from "prepare this feature" (`internal/store/types.go:26-31`). |
| `rejected` | refused, exit 3 | Terminal pre-implementation verdict; the shipped exit route is `tpatch reopen` (`internal/store/status.go:139-152`). |
| `unapplied` | refused, exit 3 | Delegated verbatim to the shipped `refuseIfUnappliedState` helper (`internal/cli/feature_unapply.go:464-473`), which already returns exit 3 and already names `tpatch apply` as the remedy. |

The table is **total** over the twelve shipped states
(`internal/store/types.go:9-38`); PIB-225 asserts totality mechanically against
`ValidFeatureState` (`internal/store/types.go:40-47`), so a thirteenth state
added later cannot silently default to "allowed".

**Interaction with reject/reopen.** `prepare` never rejects and never reopens.
A rejected feature refuses with a remediation naming `tpatch reopen <slug>`; a
reopened feature returns to its `PriorState`
(`internal/store/status.go:130-133`) and is then subject to this same table
(PIB-127, PIB-128).

### 12.3 Exact status metadata written

On a successful mutating publication, and only then:

| Field | Value |
|---|---|
| `state` | `defined` (`internal/store/types.go:11`) |
| `last_command` | `"prepare"` |
| `updated_at` | refreshed by the shipped writer (`internal/store/store.go:388`) |
| `notes` | one of exactly three frozen strings (below) |
| everything else | **untouched**, including `verify`, `rejection`, `rejection_history`, `depends_on`, `apply`, `reconcile`, `compatibility`, `id`, `title`, `requested_at` |

Frozen notes strings, one per mode:

```text
Intent bundle prepared (prepare); generated: analysis.md, spec.md
Intent bundle adopted (prepare --manual); artifacts authored by hand
Intent bundle regenerated (prepare --regenerate); prior bytes archived
```

The generated-file list in the first form is the publication set's Markdown
entries in canonical order, so the string is deterministic for a given plan.

**`Verify` is not touched.** ADR-013 makes `verify` and `amend` the only
writers of that record (`internal/store/types.go:236-251`), and `prepare` is
neither. This is asserted, not assumed (PIB-129). Note the honest consequence:
a feature that was `verified-fresh` before a `--regenerate` keeps that label
even though its intent changed. That is existing `verify` freshness semantics —
`define` has exactly the same effect today — and this PRD does not widen
`verify`'s writer set to fix it. Q6 records it.

**`Notes` is overwritten**, as it is by every phase transition
(`internal/store/store.go:380-393`). That is why the accepted contract forbids
inferring provenance from it
(`docs/prds/PRD-artifact-validation-and-provenance.md:2904-2920`), and this PRD
does not change the field's meaning or reliability.

### 12.4 The mutating `--manual` gates are unchanged

`analyze|define|explore|implement --manual` keep their current loose,
presence-only behavior (`internal/store/manual.go:56-81`). The accepted PRD
made that decision deliberately
(`docs/prds/PRD-artifact-validation-and-provenance.md:2986-3015`) and this PRD
does not reopen it. In particular:

- `tpatch define <slug> --manual` on a zero-byte `spec.md` still advances to
  `defined` and still exits 0 (PIB-130).
- `tpatch prepare <slug> --manual` on the same tree refuses with exit 2
  (PIB-131).
- The differential is intentional and is a composite acceptance row, not a
  footnote: the bundle verb is strict, the per-artifact verbs stay loose
  (PIB-132).

### 12.5 `doctor` gains one check

A new diagnostic check reports — and never acts on — a pending prepare
transaction. It is the ninth check, following the shipped D1…D8 registry
(`internal/workflow/doctor.go:228-235`):

- **ID**: `D9`. **Severity**: warning, never blocking.
- **Fires when**: `.tpatch/local/intent-prepare/<slug>/journal.json` exists.
- **Reports**: the slug, that a mutating `prepare` was interrupted, and that
  running `tpatch prepare <slug>` (any mutating mode) will recover it first.
- **Never**: acquires the lock, reads artifact content, mutates anything, or
  changes `doctor`'s exit code from what the other checks decide.
- Acceptance: PIB-133 (fires), PIB-134 (silent when absent), PIB-135 (zero
  mutation), PIB-136 (D1…D8 outputs byte-identical against pre-change
  goldens).

### 12.6 Enumerated behavior deltas — the complete list

Nothing outside this table changes.

| # | Delta | Surface |
|---|---|---|
| D1 | `tpatch prepare <slug>` stops refusing with exit 4 and performs a Path A run | `prepare` |
| D2 | `--manual` / `--regenerate` become registered flags; supplying them with `--check` changes the exit-1 message text from `unknown flag` to cobra's mutual-exclusion text | `prepare` |
| D3 | `prepare` writes no `raw-*-response-*.txt` under `artifacts/`, unlike `analyze`/`define`/`explore` | `prepare` (new surface only) |
| D4 | A new tracked directory `artifacts/intent-archive/` can appear under a feature | `.tpatch/features/<slug>/` |
| D5 | `prepare` prints progress to stderr, not stdout | `prepare` (new surface only) |
| D6 | `doctor` gains check `D9` | `doctor` |
| D7 | `RunAnalysis`/`RunDefine`/`RunExplore` are refactored to call the extracted pure generators; their observable behavior is unchanged and golden-pinned | `analyze`, `define`, `explore`, `cycle` |

**Non-invalidation obligations**, each with a row:

- `next`'s routing is byte-identical for every state, including the
  `exploration.md`-presence branch (`internal/cli/phase2.go:437-446`) — PIB-208
  against pre-change goldens.
- `cycle` is byte-identical end to end — PIB-209.
- `analyze`, `define`, `explore`, `implement`, with and without `--manual`, are
  byte-identical — PIB-210, PIB-211.
- `status`, `verify`, `record`, `land`, `reconcile`, `doctor` D1…D8 are
  byte-identical for a feature that never runs `prepare` — PIB-212, PIB-136.
- No command gains a `prepare` precondition, and nothing calls `prepare`;
  asserted in both directions by a reverse call-graph guard — PIB-213.
- Git worktrees, the Git index and `.git/**` are untouched: `prepare` executes
  no Git subprocess at all — PIB-106, source-scanned by PIB-107.

## 13. Security, privacy and determinism

### 13.1 Reads reuse the accepted boundary, unchanged

Every canonical read — inspection, generation context, revalidation — goes
through the accepted rooted capture and its refusals: one held `*os.Root`,
`fs.ValidPath` root-relative names, observed-symlink refusal, non-regular
refusal, the bounded `Max+1` read, the identity checks and the fail-closed
`unix || windows` platform allowlist (ADR-034 D1–D9, D15). No second path model
is introduced, and `filepath.Join(repoRoot, …)`, `os.Stat`, `os.Lstat`,
`os.Open`, `os.ReadFile` remain forbidden in the read path (PIB-181,
source-scanned).

### 13.2 Writes: safe paths, no symlink following, non-regular refusal

Writes are a **new** surface and ADR-035 D2 governs them:

1. Every write target is one of a **closed, compile-time list** of
   root-relative names derived from the validated slug: the four artifacts,
   `status.json`, `artifacts/intent-archive/index.json`,
   `artifacts/intent-archive/blobs/<hash>.blob`, and the `.tpatch/local/` lane.
   No write path is ever composed from provider output, from a report field, or
   from any file's content (PIB-182).
2. **Publication targets are never followed.** Immediately before each rename,
   the target's `Lstat` through the held root must show either non-existence or
   a **regular file**. A symlink, junction, directory, FIFO, socket or device
   at a publication target aborts the transaction (exit 5) — it is never
   replaced and never written through (PIB-183, PIB-196, PIB-197).
3. Temp files are created **inside the destination directory** so the rename is
   same-filesystem, matching the shipped writer
   (`internal/store/store.go:884-886`).
4. Modes: `0644` for new tracked files, preserved permissions on replacement
   (`internal/store/store.go:871-876`), `0700`/`0600` in `.tpatch/local/`.

### 13.3 Bounded provider outputs

Provider responses are bounded twice: by the request's `MaxTokens`
(`internal/workflow/workflow.go:58`, `:134`, `:179`) and, decisively, by V2's
byte cap on the staged file (§11.6). A response that exceeds the cap fails
validation and is never published (PIB-157).

### 13.4 No raw secret or provider-response leakage

- Config stores an env-var **name**, never a secret
  (`internal/store/store.go:72`), and `prepare` never reads, echoes or
  persists the value.
- Raw provider responses never reach the tracked tree (§11.4). They live in the
  gitignored staging lane and are deleted on success. This is strictly less
  exposure than `analyze` today, which writes them to `artifacts/`
  (`internal/workflow/retry.go:105-109`).
- No report field carries artifact content, a symlink target, an absolute path,
  a hostname, a PID, a duration or a wall-clock timestamp. The forbidden-field
  guard scopes to **key names and human labels**, not raw substrings, so a
  legitimate value like `archived_blob` cannot be made unspellable — the
  scoping lesson from the accepted contract
  (`docs/prds/PRD-artifact-validation-and-provenance.md:2554-2556,3626`) — PIB-158,
  PIB-159.
- Blob **content** is by definition the artifact's own bytes, at a path in the
  same tracked directory as the artifact. No new exposure class (§9.1).

### 13.5 Tracked vs local artifacts — the complete split

| Path | Tracked? | Lifetime |
|---|---|---|
| `.tpatch/features/<slug>/{analysis,spec,exploration}.md` | tracked | canonical |
| `.tpatch/features/<slug>/artifacts/analysis.json` | tracked | canonical |
| `.tpatch/features/<slug>/artifacts/intent-archive/index.json` | tracked | permanent |
| `.tpatch/features/<slug>/artifacts/intent-archive/blobs/*.blob` | tracked | permanent, immutable |
| `.tpatch/features/<slug>/status.json` | tracked | canonical |
| `.tpatch/local/intent-prepare/<slug>/prepare.lock` | **gitignored** | one invocation |
| `.tpatch/local/intent-prepare/<slug>/journal.json` | **gitignored** | one transaction |
| `.tpatch/local/intent-prepare/<slug>/{index,status}.preimage.json` | **gitignored** | one transaction |
| `.tpatch/local/intent-prepare/<slug>/stage-*/**` | **gitignored** | one invocation (retained on failure) |

The `.tpatch/local/` ignore contract is enforced by the shipped gate before any
byte is written there (`internal/rescap/scratch.go:46-62`), and a tracked file
anywhere under `.tpatch/local/` refuses the command (PIB-186, PIB-187).

### 13.6 Determinism

- No wall-clock field in any tracked artifact this command writes, and none in
  the journal either (§7.5). The only clock that moves is `status.json`'s
  pre-existing `updated_at`, written by the shipped `SaveFeatureStatus`
  (`internal/store/store.go:369-371`) — pre-existing behavior of every state
  transition, not a new field.
- Canonical JSON with fixed key order and **no Go map** in any wire format
  (ADR-033 D11, `internal/store/canonjson.go:11-17`) — PIB-165.
- The archive is a pure function of replaced content (§9.4).
- The JSON report's key order is fixed and its arrays are ordered by the
  canonical artifact order, never by map iteration — PIB-166.
- Two identical runs against identical trees with the heuristic generator
  produce byte-identical artifacts and byte-identical reports except for
  `status.json`'s `updated_at` — PIB-167.

## 14. Docs, assets and SPEC parity

### 14.1 Documents the implementation wave must update

| Document | Required change |
|---|---|
| `SPEC.md` | The `prepare` surface: four modes, the flag mutex table, the seven-code exit envelope with exit 4 recorded as retired, and the publication-unit statement. |
| `docs/feature-layout.md` | A new subsection for `artifacts/intent-archive/` — what it is, that it is tracked, immutable and content-addressed, that it is never canonical truth, that orphaned blobs are normal, and the `cp` restore form. |
| `docs/agent-as-provider.md` | A `prepare --manual` row alongside the per-phase `--manual` table (`docs/agent-as-provider.md:40-45`), stating that it adopts the **whole** bundle and is strict where the per-phase gates are loose. The existing sentence presenting `status.json.notes` as what "distinguishes Path B transitions from provider output" (`docs/agent-as-provider.md:47-54`) must additionally be corrected to a last-transition hint, **not** durable per-artifact provenance — a correction the accepted PRD already requires (`docs/prds/PRD-artifact-validation-and-provenance.md:3372-3435`) and which this PRD must not contradict. |
| `docs/path-b-operator-guide.md` | The three-`--manual`-commands flow (`docs/path-b-operator-guide.md:61-73`) gains `tpatch prepare <slug> --manual` as the one-step adoption alternative. |
| `CHANGELOG.md` | The deltas of §12.6. |
| `docs/adrs/README.md` | The ADR-035 index row (created with this PRD). |

### 14.2 Skill asset parity

All six shipped skill surfaces must name the command, and the parity guard must
be extended:

1. `requiredCommands` (`assets/assets_test.go:14-53`) gains `tpatch prepare`.
2. `requiredAnchors` (`assets/assets_test.go:62-...`) gains two anchors: one for
   the preservation default and one for the archive, so removing or paraphrasing
   either fires the guard.
3. **Hard constraint — the non-mandate assertion.** `prepare` must **not** be
   added to the skills' phase-ordering table or preflight sequence
   (`assets/assets_test.go:66-73`). WP-005 Agreed items 3 and 6
   (`docs/whitepapers/WP-005-spec-driven-workflows.md:56-59,73-76`) forbid
   making a bundle methodology mandatory; a skill that lists `prepare` as a
   required step would do exactly that in the surface agents actually read.
   PIB-216 asserts the command appears in all six files; PIB-217 asserts it
   appears in **no** phase-ordering or preflight block in any of them.
4. Skill text must not claim the command certifies quality, and must not claim
   provenance (PIB-218).

## 15. Migration

No migration step, no backfill, no rewrite of any existing file.

| Existing situation | Behavior on first `prepare` |
|---|---|
| Feature with all three Markdown files, `defined` (Path A or Path B) | default mode: no-op success, zero bytes written (PIB-035) |
| Feature with `analysis.md` + `spec.md`, no `exploration.md`, `defined` | default mode: generates `exploration.md` only; `analysis.md`, `spec.md` and the sidecar untouched (PIB-030) |
| Path A feature with a sidecar | sidecar untouched unless `analysis.md` is generated this run (PIB-032) |
| Path B feature with no sidecar | no sidecar is synthesized; advisory `analysis-preserved-sidecar-untouched` (PIB-033) |
| Feature with a zero-byte `spec.md` that reached `defined` through the loose `--manual` gate | default mode refuses (exit 2); `--regenerate` archives the zero-byte file and replaces it (PIB-034, PIB-067) |
| Legacy feature whose `status.json` predates any field added since | untouched; `omitempty` round-trip preserved (PIB-137) |
| Feature with no `artifacts/` directory | created on demand with `MkdirAllAndSyncChain`; no other feature touched (PIB-138) |
| Feature whose `.tpatch/` is untracked in Git | works; advisory `bundle-untracked-in-git` (PIB-139) |
| A feature that never runs `prepare` | byte-identical in every command (PIB-212) |

The archive directory is created **lazily**, only by the first `--regenerate`
that actually replaces something. A repository that never regenerates never
grows one (PIB-068).

## 16. Risks

| # | Risk | Mitigation |
|---|---|---|
| R1 | An operator reads "transaction" as T0 and assumes concurrent readers are safe. | §7.1's three-way table is normative; the term "atomic" is never applied to the multi-file publication in any shipped string; PIB-155 is a mechanical over-claim guard over every shipped message and doc sentence this PRD owns. |
| R2 | `--regenerate` destroys work despite the archive, because the archive step silently failed. | Archive failure refuses the whole invocation before the window (§6.3); PIB-060…PIB-066 cover each failure shape. |
| R3 | The undo-only journal is mistaken for land's redo journal by a future implementer. | ADR-035 D5 states the direction and its reason; §4's table contrasts them; the journal carries no `phase` field to decide from. |
| R4 | Extraction of the generators regresses `analyze`/`define`/`explore`/`cycle`. | S2 lands pre-change goldens **before** the refactor; PIB-186, PIB-208…PIB-211 compare against them. A refactor with no golden is exactly the no-op-vs-no-op comparison the accepted PRD warns about (`docs/prds/PRD-artifact-validation-and-provenance.md:3515-3520`). |
| R5 | Exit-code confusion between `--check` and mutating modes. | 0/2/3 mean the same thing in both; 4 is retired rather than rebound; 5/6 are new; §10.4 and PIB-013…PIB-015 pin it. |
| R6 | Unbounded archive growth. | Content-addressed dedupe means only distinct content costs bytes; blobs are the artifacts' own sizes, capped by `MaxArtifactBytes`; §20 Q5 tracks a pruning verb; the advisory `archive-blob-reused` makes dedupe visible. |
| R7 | The lock's limited authority is read as full mutual exclusion. | §7.4 states it in the negative and names the unexcluded writers; PIB-104 exercises a concurrent `define`. |
| R8 | The archive is later cited as provenance. | §9.6's table, ADR-035 D9, the extension of the forbidden-inference list (PIB-143) and the over-claim guard (PIB-155). |
| R9 | A future reviewer assumes a test proves semantics because it exists. | §18.1's disqualifying assertion shapes; §18.23's sensitivity requirement over every guard row. |
| R10 | Blob files confuse `git status` / `land` staging for users. | They are ordinary files under `artifacts/`, swept by the shipped feature-path-set rule (`internal/cli/land.go:723-725`); PIB-152 asserts `land` stages them like any other artifact and PIB-153 asserts `record`'s canonical patch is unaffected. |

## 17. Implementation slices and file ownership

All slices are gated on §19. Each is independently reviewable.

| Slice | Scope | New/modified files |
|---|---|---|
| **S1** | Transaction core: journal schema + strict decoder, preimage/new-image identity, lock, staging, revalidation, publication order, rollback, recovery, cleanup. Pure package, no CLI. | new `internal/intentpub/**` |
| **S2** | Generator extraction: `GenerateAnalysis`/`GenerateSpec`/`GenerateExploration`, the raw-response sink, and the refactor of `RunAnalysis`/`RunDefine`/`RunExplore` to call them. **Lands the pre-change goldens for `analyze`/`define`/`explore`/`cycle`/`next` first.** | modified `internal/workflow/workflow.go`, `internal/workflow/retry.go`; new `internal/workflow/generate_*.go` |
| **S3** | The archive: blobs, index, canonical encoding, `generation_id` derivation, idempotent append, corruption/collision refusals. | new `internal/store/intent_archive.go` |
| **S4** | CLI wiring: modes, flag mutexes, precedence, report model, renderers, exit codes, advisories, `--dry-run`. | modified `internal/cli/prepare.go` (the file the accepted S3 creates), new `internal/cli/prepare_publish.go` |
| **S5** | `doctor` D9; compatibility, non-invalidation, concurrency and crash-injection proofs. | new `internal/workflow/doctor_d9.go`; modified `internal/workflow/doctor.go` (registry line only) |
| **S6** | Docs, six skill surfaces, parity-guard extension, over-claim and citation guards, sensitivity meta-check. | `SPEC.md`, `docs/**`, `assets/skills/**`, `assets/assets_test.go` |

**Ordering.** S1 → S3 → S4 is strict. S2 may run in parallel with S1 and S3
**only** under an explicit file partition; S5 and S6 follow S4.

**Parallel-implementer discipline.** `internal/cli/prepare.go` and
`internal/workflow/workflow.go` are the shared surfaces. Per AGENTS.md, same-file
overlap is a hard trigger for sequential execution: **no two implementers may
touch `internal/cli/prepare.go`, and no two may touch
`internal/workflow/workflow.go`.** The cluster lead must declare the partition
at dispatch, every implementer stages by explicit path, and `git commit -a`,
`git add .`, `git add -A` and directory-scope adds are forbidden for this
cluster.

**Golden prerequisite.** S2's pre-change goldens for `analyze`, `define`,
`explore`, `cycle`, `next` and `doctor` D1…D8 must be captured and committed
**before** any refactor lands, or PIB-186 and PIB-208…PIB-212 degrade into
comparing a changed binary against itself.

## 18. Acceptance matrix

### 18.1 How to read this matrix

IDs are stable (`PIB-NNN`) and are never renumbered; a retired row is struck
through and keeps its number. Each row names the **observable** the test must
assert.

**A test does not satisfy a row merely by existing.** A row is satisfied only
when its test asserts the named observable. Disqualifying assertion shapes,
inherited from the accepted contract
(`docs/prds/PRD-artifact-validation-and-provenance.md:3531-3548`):

- asserting "the command did not return an error" satisfies no row;
- asserting "some JSON was produced" satisfies no row that names a field,
  value, order or exit code;
- a row naming an exit code must assert the numeric **process** exit code, not
  the presence of a Go error;
- a row naming "byte-identical" must compare bytes, not lengths, sizes or
  mtimes;
- a row naming a state, code or enum must assert that exact literal, not a
  truthy proxy;
- a row naming a **guard** must assert the guard's semantics — that it fails on
  a wrong input — not that the guard function or its test file exists;
- a row naming "zero writes" must be asserted with a filesystem spy or a
  whole-tree byte snapshot, never by re-reading one file.

If a row cannot be placed as written, the PRD is amended — the row is not
silently re-tiered.

Legend for **Kind**: `U` unit, `I` integration (real CLI invocation over a real
temp workspace), `S` source scan (AST), `G` mechanical guard, `C` crash/fault
injection through the declared seams.

**Injection seams.** Crash and failure rows are driven through named,
production-inert seams: `beforeJournalWrite`, `afterJournalWrite`,
`beforeBlobWrite`, `afterBlobWrite`, `beforeRename(i)`, `afterRename(i)`,
`beforeStatusRename`, `afterStatusRename`, `beforeJournalClear`,
`beforeLockRelease`, `failFsync(path)`, `failRename(path)`. Each is a
function-valued package variable that is `nil` in production; PIB-232 asserts
every one is `nil` at init and that no production call path assigns one.

### 18.2 A — CLI grammar, modes and flag mutexes

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-001 | I | `prepare <slug>` on a feature missing `spec.md` | exit 0; `spec.md` created; report `outcome: published` |
| PIB-002 | I | `prepare` with no slug | exit 1; cobra arity text; zero writes |
| PIB-003 | I | `prepare a b` | exit 1; zero writes |
| PIB-004 | I | `prepare <slug> --manual --regenerate` | exit 1; cobra mutual-exclusion text; zero writes |
| PIB-005 | I | `prepare <slug> --check --manual` | exit 1; mutual-exclusion text; zero writes |
| PIB-006 | I | `prepare <slug> --check --regenerate` | exit 1; mutual-exclusion text; zero writes |
| PIB-007 | I | `prepare <slug> --check --timeout 5s` | exit 1; mutual-exclusion text |
| PIB-008 | I | `prepare <slug> --manual --timeout 5s` | exit 1; mutual-exclusion text |
| PIB-009 | I | `prepare <slug> --manual --no-retry` | exit 1; mutual-exclusion text |
| PIB-010 | I | `prepare <slug> --check --dry-run` | exit 1; mutual-exclusion text |
| PIB-011 | I | `prepare <slug> --check --manual` | zero mutation: whole `.tpatch/` tree byte-identical |
| PIB-012 | I | `prepare <slug> --check --regenerate` | zero mutation: whole `.tpatch/` tree byte-identical |
| PIB-013 | I | `prepare <slug>` (no `--check`) in a workspace | exit is **not** 4; a real run occurs |
| PIB-014 | G | source scan over `internal/cli/**` | the retired reserved-surface refusal string is absent, and no code path constructs `ExitCodeError{Code: 4}` in the `prepare` command file |
| PIB-015 | I | table over all seven codes | each of 0,1,2,3,5,6 is reachable by a named input; 4 is unreachable by every input in the table |
| PIB-016 | I | `prepare --help` | lists all four modes; states it is unrelated to `apply --mode prepare`; names `--regenerate` as the only overwrite route |
| PIB-017 | I | `apply --help` | `--mode` description still points at `prepare`; text updated for four modes |
| PIB-018 | I | `prepare <slug> --json --quiet` on success | stdout is exactly one JSON document; stderr empty |
| PIB-019 | I | `prepare <slug> --quiet` on success | stdout is exactly one line ending in the outcome token |
| PIB-020 | S | the `prepare` command's flag registration | exactly nine flags registered: `check`, `manual`, `regenerate`, `dry-run`, `json`, `quiet`, `timeout`, `no-retry` plus the inherited persistent `path`; no `--all`, `--fix`, `--yes`, `--force`, `--restore`, `--format`, `--interactive` |

### 18.3 B — Default mode: preservation and missing-only

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-021 | I | all three Markdown files missing | all three created; exit 0; state `defined` |
| PIB-022 | I | `analysis.md` present-nonempty, other two missing | `analysis.md` **byte-identical** afterwards; other two created |
| PIB-023 | C | same as PIB-022, with a write spy | zero `OpenFile`-for-write, zero rename targeting `analysis.md` |
| PIB-024 | I | all three present-nonempty, state `defined` | zero bytes written anywhere in `.tpatch/`, including `status.json` |
| PIB-025 | U | `spec.md` present-empty (zero bytes) | refusal `artifact-empty-not-overwritten`, exit 2 |
| PIB-026 | U | `spec.md` whitespace-only | same as PIB-025 (whitespace-only is empty per the accepted rule) |
| PIB-027 | U | `spec.md` is a symlink to an in-repo file | refusal `artifact-unsafe`, exit 3; symlink not followed; target string absent from all output |
| PIB-028 | U | `exploration.md` is a directory | refusal `artifact-unsafe`, exit 3 |
| PIB-029 | U | `analysis.md` is a FIFO | refusal `artifact-unsafe`, exit 3; no open of it |
| PIB-030 | I | `analysis.md` + `spec.md` present, `exploration.md` absent | only `exploration.md` is created; the other two and the sidecar byte-identical |
| PIB-031 | U | `spec.md` mode 0000 | refusal `artifact-unsafe`, exit 3 |
| PIB-032 | I | Path A feature: `analysis.md` preserved, sidecar present | sidecar byte-identical afterwards |
| PIB-033 | I | Path B feature: `analysis.md` preserved, sidecar absent | sidecar still absent; advisory `analysis-preserved-sidecar-untouched` present |
| PIB-034 | I | zero-byte `spec.md` reached `defined` via `define --manual` | `prepare` refuses exit 2; `spec.md` still zero bytes |
| PIB-035 | I | complete bundle, state `defined` | `outcome: no-op`, `action: none`, exit 0, `status.json` byte-identical (including `updated_at`) |
| PIB-036 | I | complete bundle, state `analyzed` | `action: adopt`; only `status.json` changes; the four artifacts byte-identical |
| PIB-037 | U | `analysis.md` oversize (cap + 1) | refusal `artifact-unsafe`, exit 3; the file is never opened |
| PIB-038 | C | `spec.md` grows during its capture | classified `unstable`; refusal `artifact-unstable`, exit 3 |
| PIB-039 | U | sidecar is `invalid-structured`, all Markdown present-nonempty | no refusal; sidecar untouched; exit 0 with `action: none` |
| PIB-040 | U | sidecar is a symlink, all Markdown present-nonempty | no refusal (optional artifact never gates admissibility); sidecar untouched |
| PIB-041 | I | `analysis.md` generated this run | sidecar is (re)written in the same publication |
| PIB-042 | I | `analysis.md` preserved, `spec.md` generated | sidecar **not** in the publication set; a rename spy records none targeting it |
| PIB-043 | I | generated `spec.md` | its provider prompt contains the preserved `analysis.md`'s bytes (context flow) |
| PIB-044 | G | the §6.1 disposition table | its Go representation covers all nine accepted enum values with no `default` fallthrough |

### 18.4 C — `--manual` adoption

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-045 | I | `--manual` on a ready bundle, provider spy installed | zero provider calls |
| PIB-046 | I | `--manual` on a ready bundle | all four artifact files byte-identical afterwards |
| PIB-047 | I | `--manual` with `exploration.md` absent | exit 2; full per-artifact report; zero mutation |
| PIB-048 | I | `--manual` with zero-byte `spec.md` | exit 2; zero mutation |
| PIB-049 | I | `--manual` success with `FEATURES.md` unwritable | exit 0; `status.json` written; the index-refresh failure does not fail the command |
| PIB-050 | I | `--manual` success | `state=defined`, `last_command=prepare`, notes equal the frozen adoption string |
| PIB-051 | C | `--manual` success, rename spy | exactly **one** rename, targeting `status.json` |
| PIB-052 | C | `--manual` success, journal spy | no journal is created; no archive directory is created |
| PIB-053 | I | `--manual` while the lock is held | exit 6, `transaction-locked` |
| PIB-054 | I | `--manual` with `analysis.md` symlinked | exit 3, `artifact-unsafe`; zero mutation |
| PIB-055 | I | `--manual` from state `requested` with a complete bundle | exit 0; advisory `feature-state-below-defined`; state `defined` |
| PIB-056 | I | `--manual` on a feature already `defined` with a complete bundle | exit 0; `status.json` rewritten only if a field actually changes, else `no-op` |
| PIB-057 | I | `--manual --json` refusal | report carries `mode: manual`, `outcome: refused`, and the per-artifact rows |
| PIB-058 | I | `--manual` with `status.json` malformed | exit 3, `status-malformed`; zero mutation |
| PIB-059 | S | the `--manual` code path | contains no provider construction and no artifact write call |

### 18.5 D — `--regenerate` and the archive

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-060 | I | `--regenerate` on a full hand-authored bundle | each prior file's exact bytes exist at `intent-archive/blobs/<sha256>.blob` |
| PIB-061 | I | `--regenerate` with `spec.md` symlinked | exit 3; **nothing** archived, nothing regenerated, nothing renamed |
| PIB-062 | C | `--regenerate` with the blob write failing (`failFsync`) | exit 5 or 3; zero canonical mutations |
| PIB-063 | I | `--regenerate` where a blob file already exists with equal bytes | it is not rewritten (mtime and inode unchanged); advisory `archive-blob-reused` |
| PIB-064 | I | `--regenerate` where a blob file exists with **different** bytes for its name | exit 3, `archive-blob-corrupt`; nothing overwritten |
| PIB-065 | U | `generation_id` collision, injected through the derivation seam | exit 3, `archive-generation-id-collision`; index unchanged |
| PIB-066 | I | `--regenerate` with an `unreadable` required artifact | exit 3; nothing archived, nothing published |
| PIB-067 | I | `--regenerate` over a zero-byte `spec.md` | the zero-byte file is archived; new `spec.md` non-empty; exit 0 |
| PIB-068 | I | a workspace that never regenerates | no `intent-archive/` directory exists anywhere |
| PIB-069 | I | `--regenerate` twice with identical prior content | the second appends **no** index entry; advisory `archive-generation-duplicate` |
| PIB-070 | I | `--regenerate` | all three Markdown files **and** the sidecar are replaced; no partial-scope option exists |
| PIB-071 | I | `--regenerate` success report | names each archived blob and prints the `cp` restore form verbatim |

### 18.6 E — `--dry-run`

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-072 | C | `--dry-run` in every mode, filesystem spy | zero `mkdir`, `create`, `write`, `rename`, `remove` calls of any kind |
| PIB-073 | I | `--dry-run`, provider spy | zero provider calls |
| PIB-074 | I | `--dry-run` on an admissible plan | exit 0; `outcome: planned`; `dry_run: true` |
| PIB-075 | I | `--dry-run` on a `present-empty` required artifact (default mode) | exit 2, same refusal code as the real run |
| PIB-076 | I | `--dry-run --manual` on an incomplete bundle | exit 2, same refusal code as the real run |
| PIB-077 | I | `--dry-run` report | contains the verbatim plan-only sentence; contains no `generator` value and no archive hash |
| PIB-078 | I | `--dry-run --regenerate` | lists every artifact it would archive; creates no archive directory |
| PIB-079 | C | `--dry-run` with a pending journal | recovery does **not** run; the journal is byte-identical afterwards |
| PIB-080 | C | `--dry-run`, lock spy | the lock file is never created |

### 18.7 F — Staging, validation and no partial canonical write

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-081 | C | generation of the 2nd of 3 artifacts fails hard | zero canonical files created or modified; whole `.tpatch/features/<slug>/` byte-identical |
| PIB-082 | C | staged `spec.md` is empty (V1) | exit 2; zero canonical mutations; staging retained |
| PIB-083 | C | staged `spec.md` is whitespace-only (V1) | exit 2; zero canonical mutations |
| PIB-084 | C | staged `analysis.md` exceeds the cap (V2) | exit 2; zero canonical mutations |
| PIB-085 | C | staged bytes contain a NUL (V3) | exit 2; zero canonical mutations |
| PIB-086 | C | staged bytes are invalid UTF-8 (V4) | exit 2; zero canonical mutations |
| PIB-087 | C | staged sidecar is `[1,2,3]` (V5) | exit 2; zero canonical mutations |
| PIB-088 | C | staged sidecar is `{` (V5) | exit 2; zero canonical mutations |
| PIB-089 | C | a staged file changes identity after fsync (V6) | exit 5; zero canonical mutations |
| PIB-090 | I | successful run | the staging tree no longer exists and its parent is empty of `stage-*` |
| PIB-091 | I | failed run | the staging tree still exists; the report names its repo-relative path; advisory `staging-retained` |
| PIB-092 | I | a second successful run after a failed one | the earlier retained staging tree is removed |
| PIB-093 | I | staging tree permissions | directory `0700`; files `0600` |
| PIB-094 | I | `.tpatch/local/` not covered by an ignore rule | exit 3, `local-lane-not-ignored`, before any staging byte |
| PIB-095 | I | a tracked file exists under `.tpatch/local/` | exit 3, `local-lane-not-ignored`; zero mutation |
| PIB-096 | C | staged files after generation | each is fsynced and its identity recorded before the journal is written |
| PIB-097 | S | the staging path builder | derives from the validated slug and `RandomHex12` only; never from provider output |
| PIB-098 | I | run that fails after staging | staging retained; canonical tree byte-identical |
| PIB-099 | I | run that succeeds | staging removed; parent directory fsynced |

### 18.8 G — Publication, revalidation and concurrency

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-100 | C | an external writer modifies `spec.md` between preflight and revalidation | exit 5 before the first rename; zero canonical mutations; staging retained |
| PIB-101 | C | an artifact appears where preflight saw `absent` | exit 5; the appeared file is byte-identical afterwards |
| PIB-102 | C | an artifact disappears between preflight and revalidation | exit 5; zero canonical mutations |
| PIB-103 | I | documented limit: a write landing inside the window | test asserts the shipped docs and messages state the limit; **no row claims the write is preserved** |
| PIB-104 | I | a concurrent `tpatch define` completing before revalidation | exit 5; `define`'s bytes preserved |
| PIB-105 | I | `.tpatch/` replaced wholesale between preflight and revalidation | exit 5 or 3; nothing published |
| PIB-106 | I | any successful `prepare` in a Git repo with a dirty index | `git status --porcelain` shows only the expected new/modified `.tpatch` paths; the index checksum is unchanged |
| PIB-107 | S | the whole `prepare` call graph | no `exec.Command("git", …)`, no `gitutil` write entry point |
| PIB-108 | C | rename order | the observed rename sequence equals §7.2's order, with `status.json` last |
| PIB-109 | C | fsync sequence | each entry's parent directory is fsynced after its rename; the journal directory is fsynced after clear |
| PIB-110 | C | rename 2 of 4 fails; rollback succeeds | exit 5; every canonical file byte-identical to the pre-run state; journal cleared |
| PIB-111 | C | rename 2 of 4 fails and rollback also fails | exit 6; journal **retained**; report names the journal, the archive and the failing entry |
| PIB-112 | C | rollback attempted when a published entry no longer matches its new-image | rollback refuses that entry; exit 6; nothing overwritten |

### 18.9 H — Crash injection and recovery

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-113 | C | recovery run twice | the second is a no-op; no journal; identical tree |
| PIB-114 | C | recovery with two `stage-*` trees for the slug | both removed; no blob removed; the index untouched |
| PIB-115 | C | recovery with a pending journal for another slug | that slug's lane is untouched |
| PIB-116 | C | crash phase CP0 (before lock) | next run proceeds normally |
| PIB-117 | C | crash phase CP1 (lock, no journal) | owned lock removed; staging removed; run proceeds |
| PIB-118 | C | crash phase CP2 (blobs, no journal) | blobs remain; run proceeds; no index entry was added |
| PIB-119 | C | crash phase CP3 (journal, no rename) | journal cleared; every canonical file byte-identical to pre-run |
| PIB-120 | C | crash phase CP4 (2 of 4 renamed) | the 2 published entries are restored to preimage; journal cleared |
| PIB-121 | C | crash phase CP5 (artifacts new, index old) | all restored; journal cleared |
| PIB-122 | C | crash phase CP6 (index new, status old) | all restored including `index.json`; journal cleared |
| PIB-123 | C | crash phase CP7 (everything new, journal not cleared) | recovery **undoes nothing**; journal cleared; tree stays all-new |
| PIB-124 | I | a second mutating `prepare` while the lock is held | exit 6, `transaction-locked`; the first run's outcome unaffected |
| PIB-125 | I | two mutating `prepare` runs on different slugs | both succeed; neither lane touched by the other |
| PIB-126 | C | crash phase CP9 (a third party rewrote a published entry) | exit 6; every file, the journal and the archive preserved and named |

### 18.10 I — Lifecycle, states and status metadata

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-127 | I | `prepare` on a `rejected` feature | exit 3, `state-refused`; remediation names `tpatch reopen <slug>`; zero mutation |
| PIB-128 | I | reopen, then `prepare` | allowed iff the restored `PriorState` is in the allowed set |
| PIB-129 | I | successful `prepare` on a feature with a `verify` record | `status.verify` is byte-identical afterwards |
| PIB-130 | I | `define --manual` on a zero-byte `spec.md` | exit 0; state `defined` — unchanged loose behavior |
| PIB-131 | I | `prepare --manual` on the same tree | exit 2 |
| PIB-132 | I | composite of PIB-130 then PIB-131 in one workspace | both observables in one run; zero mutation from the second |
| PIB-133 | I | `doctor` with a pending prepare journal | a `D9` warning naming the slug; exit code unchanged from the no-journal case |
| PIB-134 | I | `doctor` with no journal | no `D9` output at all |
| PIB-135 | C | `doctor` with a pending journal, filesystem spy | zero writes; the lock is never created |
| PIB-136 | G | `doctor` D1…D8 output vs pre-change goldens | byte-identical |
| PIB-137 | I | legacy `status.json` with no optional fields | after a successful `prepare`, only `state`/`last_command`/`updated_at`/`notes` differ; no field is added |
| PIB-138 | I | feature with no `artifacts/` directory | created 0755; no other feature's directory touched |
| PIB-139 | I | untracked `.tpatch/` | exit 0; advisory `bundle-untracked-in-git` present |

### 18.11 J — Provenance boundary

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-140 | I | `prepare --check` after any mutating `prepare` | every artifact still reports `provenance: "unknown"` |
| PIB-141 | S | the inspector's call graph | it never reads `intent-archive/**`; no import, no path constant |
| PIB-142 | G | every shipped string of both report schemas | no field, value or message asserts Path A vs Path B for a feature |
| PIB-143 | S | the shipped forbidden-inference list | it contains an `intent-archive` entry |
| PIB-144 | G | `generator` field | present only in the mutating report; never written to any tracked file; a guard asserts the token appears in no `.tpatch/` byte after a run |
| PIB-145 | G | source + docs scan | ADR-034 is never cited as precedent for persistence; ADR-035 governs every write |
| PIB-146 | G | heuristic-mode run | the sidecar carries `heuristic_mode: true`, byte-compatible with `analyze`'s output for the same input |
| PIB-147 | G | the archive index and blob layout | carry no author, agent, model, provider, endpoint, or Path-A/Path-B field; sensitivity fixture proves the guard fails when one is added |

### 18.12 K — Security, privacy and forbidden fields

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-148 | U | publication target is a symlink at rename time | transaction aborts (exit 5); the symlink and its target are byte-identical afterwards |
| PIB-149 | U | publication target is a directory at rename time | exit 5; nothing written |
| PIB-150 | U | publication target is a FIFO at rename time | exit 5; nothing written; no open of it |
| PIB-151 | U | `artifacts/` is a symlink at publication time | exit 5; not followed |
| PIB-152 | I | `land` after a `--regenerate` | archive blobs and `index.json` are staged like any other `artifacts/**` file; no new refusal |
| PIB-153 | I | `record` after a `--regenerate` | `artifacts/post-apply.patch` is byte-identical to the same run without an archive present |
| PIB-154 | S | every write target constructor | derives from the compile-time name list plus the validated slug; no write path derives from provider output or file content |
| PIB-155 | G | every shipped message, help string and PRD/ADR sentence this cluster owns | no occurrence of "atomic", "atomically" or "simultaneously" applied to the multi-file publication; the guard fails when a fixture sentence adds one |
| PIB-156 | I | any refusal | the raw `<slug>` argument is never echoed when it fails canonical validation |
| PIB-157 | C | a provider response larger than the cap | V2 rejects it; exit 2; zero canonical mutations; the oversized bytes never reach a tracked path |
| PIB-158 | G | JSON report key walk at every nesting level | no key from the forbidden set (absolute path, timestamp, duration, hostname, pid, content, symlink target, secret, token, env value) |
| PIB-159 | G | sensitivity fixture for PIB-158 | injecting a `created_at` key makes the guard fail |

### 18.13 L — Determinism and canonical encoding

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-160 | U | two archive publications of identical prior bytes | byte-identical blob files and byte-identical index entries |
| PIB-161 | G | `index.json` bytes | no wall-clock field; key order fixed; `replaced` sorted by `artifact_id` |
| PIB-162 | U | `generation_id` derivation | one derivation point; equal canonical bodies produce equal ids; a one-byte change produces a different id |
| PIB-163 | I | success report | the `cp` restore form is printed verbatim and names no verb that does not exist |
| PIB-164 | G | journal bytes | no wall-clock field; no `phase` field; strict decode rejects an unknown field and trailing content |
| PIB-165 | S | every wire struct this PRD adds | no Go `map` type in any of them |
| PIB-166 | U | JSON report key order | equals the declared struct order in both the ordinary and refusal shapes |
| PIB-167 | I | two identical heuristic runs on identical trees | artifacts and reports byte-identical; only `status.json`'s `updated_at` differs |
| PIB-168 | U | archive index round-trip | decode→encode is byte-identical for a fixture with three generations |
| PIB-169 | G | blob filenames | match `^[0-9a-f]{64}\.blob$`; never uppercase hex, never truncated |

### 18.14 M — Output, exits and precedence

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-170 | I | default output | progress lines appear on **stderr**, never stdout |
| PIB-171 | I | `--json` and `--quiet` | no progress line appears on either stream |
| PIB-172 | I | both report schemas | `command` is `"prepare --check"` vs `"prepare"`; a consumer can switch on it |
| PIB-173 | I | refusal before inspection (bad slug) | `artifacts` is `[]`; `archive` omitted; `refusal.code` from the closed set |
| PIB-174 | I | refusal after inspection | `artifacts` carries the planned dispositions; `refusal.code` set |
| PIB-175 | G | advisory selection | a total function over observed state; no advisory can contradict its own artifact row; sensitivity fixture proves it fails when one does |
| PIB-176 | I | `prepare ../../etc --check`-less form outside a workspace | slug refusal (exit 3) precedes workspace discovery; the raw argument is not echoed |
| PIB-177 | C | pending journal plus a `rejected` feature | recovery runs and clears the journal **before** the exit-3 lifecycle refusal is returned |
| PIB-178 | I | every emission, both surfaces | the frozen disclaimer string byte-for-byte |
| PIB-179 | I | every refusal code | remediation names only shipped commands/flags/paths |
| PIB-180 | G | every refusal string | contains no `docs/`, no `.md`, no `http` |
| PIB-181 | S | the read path | no `os.Stat`, `os.Lstat`, `os.Open`, `os.ReadFile`, `filepath.Join(repoRoot, …)` |
| PIB-182 | S | the write path | targets come from the closed compile-time name list only |
| PIB-183 | U | pre-rename `Lstat` gate | a non-regular, non-absent target aborts before the rename |

### 18.15 N — Generator extraction and generation semantics

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-184 | S | the three extracted generators | none takes a `*store.Store` and none holds a writer |
| PIB-185 | C | the three extracted generators, filesystem spy | zero writes of any kind |
| PIB-186 | G | `analyze`/`define`/`explore` after the refactor | stdout, stderr, exit code and every written byte identical to pre-change goldens |
| PIB-187 | U | preserved artifact read as generation context | goes through the accepted bounded rooted capture; an unsafe one already refused at admissibility |
| PIB-188 | C | provider retry with `--no-retry` unset | raw responses land in the staging tree, not `artifacts/` |
| PIB-189 | S | the `prepare` generation path | `RetryOptions.Store` is nil at every construction site |
| PIB-190 | I | failed generation with retries | the retained staging tree contains the raw responses; `artifacts/` contains none |
| PIB-191 | G | shipped strings and docs | no claim that the command cannot hang or has bounded runtime |
| PIB-192 | C | provider deadline expiry during generation | heuristic fallback for that artifact; exit 0; advisory `provider-fallback-heuristic` |
| PIB-193 | C | deadline expiry after staging, before publication | the publication proceeds; exit 0 |
| PIB-194 | G | the validation set | exactly V1…V6; no check inspects headings, length or topicality |
| PIB-195 | I | successful `prepare`, then `prepare --check` in the same tree | `structural_readiness: ready`, exit 0 |
| PIB-196 | I | no provider configured | all artifacts generated heuristically; exit 0; advisory `provider-not-configured` |
| PIB-197 | I | provider configured but unreachable | heuristic fallback, exit 0 — **not** a command failure |

### 18.16 O — `--check` compatibility

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-198 | G | `--check` human output vs pre-change goldens | byte-identical across the ready / not-ready / abort populations |
| PIB-199 | G | `--check --json` vs pre-change goldens | byte-identical |
| PIB-200 | I | `--check` exit codes | 0/2/3 unchanged for every accepted population |
| PIB-201 | C | `--check`, filesystem spy | zero writes |
| PIB-202 | I | `--check` | `provenance: "unknown"` for all four artifacts |
| PIB-203 | I | `--check` | the frozen disclaimer byte-for-byte |
| PIB-204 | I | `--check` abort populations | `artifacts` is `[]` iff `abort` is present |
| PIB-205 | I | `--check` | the accepted `overall` totals unchanged |
| PIB-206 | C | `--check` executed inside a publication window | reports what is on disk; a mid-rename artifact may be `unstable`; no crash, no lock, no write |
| PIB-207 | G | `--check` with and without a pending journal, identical canonical bytes | byte-identical output |

### 18.17 P — Non-invalidation of shipped commands

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-208 | G | `next` for all twelve states, incl. the `exploration.md` branch | byte-identical to pre-change goldens |
| PIB-209 | G | `cycle` end-to-end | byte-identical to pre-change goldens |
| PIB-210 | G | `analyze\|define\|explore\|implement` | byte-identical to pre-change goldens |
| PIB-211 | G | the same four with `--manual` | byte-identical to pre-change goldens |
| PIB-212 | G | `status`, `verify`, `record`, `land`, `reconcile` on a feature that never ran `prepare` | byte-identical to pre-change goldens |
| PIB-213 | S | reverse call graph | nothing imports or invokes the prepare publication package except the `prepare` command file; `prepare` is not an `OnComplete` in `next` and not a step in `cycle` |
| PIB-214 | S | `store.Store` writers | no new writer is added to `internal/store` beyond the archive functions named in S3 |

### 18.18 Q — Docs, skills and asset parity

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-215 | G | `SPEC.md` | documents four modes, the flag mutex table and all seven exit-code rows including retired `4` |
| PIB-216 | G | all six skill files | each names `tpatch prepare`; `requiredCommands` extended |
| PIB-217 | G | all six skill files | `prepare` appears in **no** phase-ordering table and **no** preflight block; sensitivity fixture proves the guard fails when it is added |
| PIB-218 | G | all six skill files | no sentence claims semantic certification or provenance |
| PIB-219 | G | `docs/feature-layout.md` | documents `intent-archive/`, its immutability, that it is never canonical truth, and the `cp` restore form |
| PIB-220 | G | `docs/agent-as-provider.md` | carries the `prepare --manual` row and the corrected `notes` sentence |

### 18.19 R — Platform and build

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-221 | G | `go vet` and cross-build | clean for linux/amd64, linux/arm64, darwin/arm64, windows/amd64 |
| PIB-222 | I | the full mutating flow on native `windows-latest` | publication, rollback and recovery all pass; junction fixtures **fail** rather than skip |
| PIB-223 | U | an unsupported `GOOS` | the accepted fail-closed allowlist aborts before any root is opened |

### 18.20 S — Totality, ledger and sensitivity guards

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-224 | G | §6.1 and §6.3 disposition tables | total over the accepted nine-value enum; adding a tenth value fails compilation or the guard |
| PIB-225 | G | §12.2 state table | total over `ValidFeatureState`; a thirteenth state fails the guard rather than defaulting to allowed |
| PIB-226 | G | the closed vocabularies of §10.2 | the shipped constant sets equal the tables exactly |
| PIB-227 | G | the advisory catalog | exactly ten codes; every one reachable by a named fixture |
| PIB-228 | G | the refusal-code catalog | closed; every code reachable; every code has a remediation |
| PIB-229 | G | every `PIB-NNN` cited in prose | resolves to a real matrix row |
| PIB-230 | G | the acceptance ledger | every row maps to a resolvable package, `func TestX(*testing.T)` declaration and optional literal subtest, via AST — not a byte scan |
| PIB-231 | G | sensitivity meta-check | every row whose Kind contains `G` has a fixture proving the guard fails on a wrong input |
| PIB-232 | G | the injection seams of §18.1 | all are `nil` at init; no production path assigns one; a sensitivity fixture proves the guard fails when one is assigned |

### 18.21 T — Feature-request input

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-233 | I | `request.md` absent, empty, symlinked or unreadable, in `generate` and `regenerate` modes | exit 3, `request-unreadable`; zero mutation; a provider spy records zero calls |
| PIB-234 | I | `request.md` absent, `--manual` mode on a complete bundle | exit 0; adoption succeeds; generation is not required |

### 18.22 Counts, kinds and slice partition

- **234 rows**, `PIB-001`…`PIB-234`, contiguous, zero duplicates, zero retired.
- **20 categories**: A 20, B 24, C 15, D 12, E 9, F 19, G 13, H 14, I 13, J 8,
  K 12, L 10, M 14, N 14, O 10, P 7, Q 6, R 3, S 9, T 2. Sum = 234.
- **Slice partition** (each row in exactly one slice): S1 → F, G, H (46 rows);
  S2 → N (14 rows); S3 → D, L (22 rows); S4 → A, B, C, E, I, M, T (97 rows);
  S5 → J, O, P, R (28 rows); S6 → K, Q, S (27 rows). Sum = 234.

### 18.23 Sensitivity requirement

Every row whose Kind contains `G` carries a **sensitivity fixture**: a
deliberately wrong input that the guard must reject. A byte-scanning or
name-matching guard can false-pass silently, and the repository has already
been burned by exactly that
(`docs/prds/PRD-artifact-validation-and-provenance.md:3960-3991`). PIB-231 is
the meta-check that derives the guard set mechanically from the Kind column
rather than from a hand-maintained list.

## 19. Implementation authorization gate

**No implementation is authorized by this document.**

1. This PRD must be accepted.
2. **ADR-035 must be accepted.** It is `Proposed` at rev-0, and a writer cannot
   accept its own ADR; the two are reviewed together and acceptance of both is
   the precondition for dispatching S1.
3. Only then may the cluster lead declare the §17 file partition and dispatch.

Until both are accepted, no file under `cmd/`, `internal/`, `assets/`,
`tests/`, and no change to `SPEC.md` or `CHANGELOG.md`, is authorized by this
PRD.

## 20. Open questions

Only genuinely unresolved items. Everything else is a decision. None blocks
review.

| # | Question | Why open | Default if unanswered |
|---|---|---|---|
| Q1 | Should `--regenerate --only <ids>` exist later? | Coherence argues for all-or-nothing (§6.3), but an operator who only wants a new `exploration.md` currently has no route short of deleting the file. | Not in v1; additive later with an enumerated coherence statement. |
| Q2 | Is `180s` the right whole-command deadline? | No distribution of three-call prepare durations has been measured; the per-phase `60s` (`internal/cli/cobra.go:629`) was not chosen empirically either. | `180s`, overridable; changing it moves only the fallback boundary. |
| Q3 | Should a `--require-provider` flag exist? | Silent heuristic fallback can surprise an operator who wanted real analysis; the advisory is the current mitigation. | No flag in v1; fallback matches every existing phase command. |
| Q4 | Should a `tpatch prepare <slug> --restore <hash>` verb exist? | `cp` is unambiguous and needs no new surface, but it is unguarded — nothing stops restoring a blob over the wrong path. | No verb in v1; the report prints the exact `cp` form. |
| Q5 | Should archive pruning exist? | Growth is bounded by distinct content, not invocations, but a long-lived feature regenerated hundreds of times accumulates. | No pruning in v1; blobs are ordinary files and `rm` works; a verb needs its own safety design. |
| Q6 | Should `--regenerate` invalidate a `verified-fresh` label? | Intent changed, so the freshness claim is arguably stale — but ADR-013 makes `verify`/`amend` the only writers of that record, and `define` has the same effect today without invalidating. | No change; widening `verify`'s writer set is a separate PRD with its own delta. |
| Q7 | Should the archive be per-feature (chosen) or per-workspace with cross-feature dedupe? | A workspace-level blob store would dedupe across features, but it couples feature directories, complicates `land`'s per-feature path set (`internal/cli/land.go:723-725`) and makes `tpatch remove` ambiguous. | Per-feature, as specified. |
| Q8 | Should `doctor` D9 be an error rather than a warning? | A pending journal is recoverable automatically, so failing `doctor` would over-alarm — but a journal that persists for weeks indicates an abandoned transaction. | Warning; a future PRD may add an age-based escalation, which would require a clock and therefore its own determinism argument. |

## 21. Alternatives considered (summary)

Beyond §8's six overwrite alternatives:

| Option | Verdict |
|---|---|
| A new `prepared` FeatureState | **Rejected.** WP-005 Agreed item 6 forbids it, and completeness is an artifact-level fact that `prepare --check` already reports. |
| A separate verb (`tpatch bundle`, `tpatch intent`) instead of extending `prepare` | **Rejected.** It would fork the vocabulary the accepted PRD, WP-005 and GH #10/#11 already share, and leave `prepare --check` orphaned from its own mutating half. |
| Redo-style recovery (complete the publication like `land`) | **Rejected.** Nothing in the publication is irreversible, so undo is strictly simpler and cannot publish from a pruned staging tree (§7.5, ADR-035 D5). |
| Taking the transaction lock in every command that writes a feature artifact | **Rejected.** Eleven shipped verbs would gain a lock, a deadlock surface and a new failure mode, to close a window that revalidation already detects (§7.10). |
| Copy-on-write of the whole feature directory, then swap | **Rejected.** Directory swap is not atomic either, breaks any open descriptor, and would move audit-trail files (`patches/**`, `record.md`) that this command has no business touching. |
| Writing the archive to `.tpatch/local/` | **Rejected.** A recovery guarantee that vanishes on clone is not a guarantee (§9.1, ADR-035 D8). |
| Storing preimage **content** in the journal instead of the archive | **Rejected** for artifacts (unbounded gitignored growth, lost on clone); **adopted** for the two small machine-written metadata preimages, where the archive would be circular or category-wrong (§7.6). |

## 22. Claims-audit appendix

Every load-bearing claim this PRD makes about **current** behavior, with a
`file:line` anchor. A reviewer should spot-check that each anchor lands within
±5 lines of the cited construct at HEAD `20e8bbe`.

| # | Claim | Anchor |
|---|---|---|
| C1 | `RunAnalysis` writes the sidecar `artifacts/analysis.json` before anything else | `internal/workflow/workflow.go:90` |
| C2 | `RunAnalysis` then writes `analysis.md` | `internal/workflow/workflow.go:96` |
| C3 | `RunAnalysis` then mutates feature state to `analyzed` | `internal/workflow/workflow.go:103` |
| C4 | `RunDefine` writes `spec.md` | `internal/workflow/workflow.go:151` |
| C5 | `RunDefine` marks `defined` with a fixed notes string | `internal/workflow/workflow.go:155` |
| C6 | `RunExplore` writes `exploration.md` | `internal/workflow/workflow.go:196` |
| C7 | `RunExplore` also marks `defined` | `internal/workflow/workflow.go:200` |
| C8 | `RunAnalysis` writes a raw provider response into `artifacts/` on the fallback path | `internal/workflow/workflow.go:72` |
| C9 | …and again on the parse-failure path | `internal/workflow/workflow.go:80` |
| C10 | `RunAnalysis` falls back to the heuristic generator on provider error | `internal/workflow/workflow.go:67-69` |
| C11 | `RunDefine` falls back to the heuristic generator | `internal/workflow/workflow.go:141-143` |
| C12 | `RunExplore` falls back to the heuristic generator | `internal/workflow/workflow.go:186-188` |
| C13 | Heuristic mode is recorded in the sidecar as `heuristic_mode` | `internal/workflow/workflow.go:25` |
| C14 | …and set by the heuristic analysis constructor | `internal/workflow/workflow.go:208` |
| C15 | `RunDefine` reads `analysis.json` and `analysis.md` as context | `internal/workflow/workflow.go:117-123` |
| C16 | `RunExplore` reads `analysis.md` and `spec.md` as context | `internal/workflow/workflow.go:165-166` |
| C17 | Analysis requests are capped at 4096 output tokens | `internal/workflow/workflow.go:58` |
| C18 | Define requests are capped at 4096 output tokens | `internal/workflow/workflow.go:132` |
| C19 | Explore requests are capped at 4096 output tokens | `internal/workflow/workflow.go:177` |
| C20 | The analysis validator requires a JSON **object** | `internal/workflow/workflow.go:62` |
| C21 | `captureFileTree` is a pure read used for generation context | `internal/workflow/workflow.go:262-266` |
| C22 | `readGuidanceFiles` is a pure read used for generation context | `internal/workflow/workflow.go:293-304` |
| C23 | `renderAnalysisMD` is already pure | `internal/workflow/workflow.go:351-401` |
| C24 | `GenerateWithRetry` writes `raw-<prefix>-response-<n>.txt` when a `Store` is supplied | `internal/workflow/retry.go:105-109` |
| C25 | `GenerateWithRetry` retries with a corrective prompt and returns the last response | `internal/workflow/retry.go:94-131` |
| C26 | `WithDisableRetry` forces a single attempt | `internal/workflow/retry.go:47-58` |
| C27 | …consumed as `attempts = 1` | `internal/workflow/retry.go:83-85` |
| C28 | `--manual` validates only that the artifact exists (JSON validation is implement-only) | `internal/store/manual.go:56-66` |
| C29 | The `--manual` phase map is the single source of truth for four phases | `internal/store/manual.go:26-32` |
| C30 | `AdvanceStateManually` writes a free-text note and marks state | `internal/store/manual.go:78-80` |
| C31 | `WriteFeatureFile` path-checks then calls `writeFile` | `internal/store/store.go:443-449` |
| C32 | `writeFile` is `os.WriteFile` — a truncating in-place write | `internal/store/store.go:918-923` |
| C33 | `writeFileAtomic` preserves the existing file's permission bits | `internal/store/store.go:871-876` |
| C34 | `writeFileAtomicWithRename` is temp→chmod→write→sync→close→rename→dir-sync | `internal/store/store.go:878-917` |
| C35 | The temp file is created in the destination directory | `internal/store/store.go:884-886` |
| C36 | `SaveFeatureStatus` writes `status.json` atomically and best-effort refreshes `FEATURES.md` | `internal/store/store.go:368-377` |
| C37 | `SaveFeatureStatus` stamps `updated_at` when empty | `internal/store/store.go:369-371` |
| C38 | `MarkFeatureState` overwrites `Notes` on every transition | `internal/store/store.go:380-393` |
| C39 | `MarkFeatureState` bumps `updated_at` | `internal/store/store.go:388` |
| C40 | Config stores an auth **env-var name**, not a secret | `internal/store/store.go:72` |
| C41 | `MkdirAllAndSyncChain` fsyncs the whole chain unconditionally | `internal/store/fsdurable.go:41-52` |
| C42 | …and the rationale is crash-durability of already-existing directories | `internal/store/fsdurable.go:1-10` |
| C43 | `SyncDir` opens and fsyncs a directory | `internal/store/fsdurable.go:22-33` |
| C44 | `RandomHex12` is the per-invocation scratch suffix convention | `internal/store/fsdurable.go:96-103` |
| C45 | The twelve `FeatureState` values | `internal/store/types.go:9-38` |
| C46 | `defined` is an existing state | `internal/store/types.go:11` |
| C47 | `ValidFeatureState` enumerates them in one switch | `internal/store/types.go:40-47` |
| C48 | `upstream_merged` asserts an upstream implementation exists | `internal/store/types.go:26-31` |
| C49 | `FeatureStatus` carries `state`, `last_command`, `updated_at`, `notes` | `internal/store/types.go:206-216` |
| C50 | `Verify` is written only by `verify` and `amend` | `internal/store/types.go:236-251` |
| C51 | `RejectableStates` is pre-implementation only | `internal/store/status.go:139-152` |
| C52 | A reopen restores `PriorState` | `internal/store/status.go:130-133` |
| C53 | The land journal exists because `git commit` is irreversible | `internal/cli/land_journal.go:11-23` |
| C54 | The land journal is versioned and refuses other versions | `internal/cli/land_journal.go:56-58` |
| C55 | It lives under the gitignored `.tpatch/local/` root | `internal/cli/land_journal.go:60-62` |
| C56 | `landJournalFileState` is `(exists, sha256, mode)` with a `matches` comparison | `internal/cli/land_journal.go:65-79` |
| C57 | Its `Phase` field is explicitly advisory | `internal/cli/land_journal.go:110-111` |
| C58 | The journal records a wall-clock `created_at` | `internal/cli/land_journal.go:109` |
| C59 | The journal is written through `gitutil.DurableWriteFile` | `internal/cli/land_journal.go:310-325` |
| C60 | The journal decoder is strict (unknown fields refused) | `internal/cli/land_journal.go:348-380` |
| C61 | Recovery classifies the live state as preimage / postimage / divergent | `internal/cli/land_journal.go:418-444` |
| C62 | Recovery runs before the caller mutates anything | `internal/cli/land_journal.go:445-482` |
| C63 | The lock is `O_CREATE\|O_EXCL` with a nonce, fsynced | `internal/cli/land_journal.go:629-648` |
| C64 | Lock release removes the file and fsyncs the directory | `internal/cli/land_journal.go:650-662` |
| C65 | A stale lock is removed only when nonce **and** inode match | `internal/cli/land_journal.go:675-698` |
| C66 | `DurableWriteFile` is temp→write→fsync→close→rename | `internal/gitutil/index_snapshot.go:455-500` |
| C67 | The resource capture tree is an unordered content-addressed set plus one pointer | `internal/store/resource_publish.go:1-9` |
| C68 | Batch IDs are derived at exactly one point | `internal/store/resource_publish.go:131-142` |
| C69 | `ComputeBatchID` hashes the canonical body | `internal/store/resource_publish.go:153-161` |
| C70 | An already-identical batch file is an idempotent re-publish, never rewritten | `internal/store/resource_publish.go:240-246` |
| C71 | Named publication refusals include collision and corruption | `internal/store/resource_publish.go:199-203` |
| C72 | Publication is "write immutable content, then rewrite the pointer" | `internal/store/resource_publish.go:230-282` |
| C73 | Batch decoding is strict: unknown fields, trailing content and null arrays refused | `internal/store/resource_publish.go:305-320` |
| C74 | No Go map may reach a wire format | `internal/store/canonjson.go:11-17` |
| C75 | The root printer emits `error: %v` for every non-nil `RunE` error | `internal/cli/cobra.go:33-39` |
| C76 | `exitCodeFor` maps typed errors to their code and everything else to 1 | `internal/cli/cobra.go:43-52` |
| C77 | The root sets `SilenceUsage` and `SilenceErrors` | `internal/cli/cobra.go:56-62` |
| C78 | `--path` is a root persistent string flag | `internal/cli/cobra.go:66` |
| C79 | `openStoreFromCmd` resolves `--path` then `FindProjectRoot` | `internal/cli/cobra.go:3782-3793` |
| C80 | The single-phase commands use the **probing** provider loader | `internal/cli/cobra.go:609-612` |
| C81 | They print progress to stdout | `internal/cli/cobra.go:621-626` |
| C82 | Their timeout default is 60s | `internal/cli/cobra.go:629` |
| C83 | `--no-retry` is a shipped phase flag | `internal/cli/cobra.go:630` |
| C84 | `--manual` and `--skip-llm` are installed by one shared helper | `internal/cli/cobra.go:3410-3414` |
| C85 | `runManualPhase` is the single manual entry point for four phases | `internal/cli/cobra.go:3429-3437` |
| C86 | `isTpatchUntracked` detects an untracked `.tpatch/` | `internal/cli/cobra.go:3405-3407` |
| C87 | `cycle` is a sequential pipeline through record | `internal/cli/phase2.go:26-32` |
| C88 | `cycle` calls the incremental phase functions in order | `internal/cli/phase2.go:62-96` |
| C89 | `cycle` asserts state after each phase | `internal/cli/phase2.go:69-71` |
| C90 | `cycle` uses the **non-probing** provider loader | `internal/cli/phase2.go:55` |
| C91 | `cycle --interactive` is the repo's only interactive prompt precedent | `internal/cli/phase2.go:50` |
| C92 | `cycle --skip-execute` stops before recipe execution | `internal/cli/phase2.go:122-126` |
| C93 | `next` emits a single harness task and supports `--format harness-json` | `internal/cli/phase2.go:381-384` |
| C94 | `next` infers the `defined` sub-state from `exploration.md` presence | `internal/cli/phase2.go:437-446` |
| C95 | `refuseIfUnappliedState` refuses with the shipped state-refusal code | `internal/cli/feature_unapply.go:464-473` |
| C96 | The repo binds exit 2 to validation and exit 3 to state refusal | `internal/cli/reject.go:45-48` |
| C97 | …surfaced through `ExitCodeError` | `internal/cli/reject.go:68-70` |
| C98 | `doctor` runs a registry of checks D1…D8 | `internal/workflow/doctor.go:228-235` |
| C99 | `land` sweeps everything dirty under the feature directory into its path set | `internal/cli/land.go:723-725` |
| C100 | The `.tpatch/local/` prefix is the gitignored scratch root | `internal/rescap/scratch.go:34-37` |
| C101 | A local-lane ignore contract gate runs before any scratch write | `internal/rescap/scratch.go:46-62` |
| C102 | Skill parity is enforced by a required-command list | `assets/assets_test.go:14-53` |
| C103 | …and by verbatim required anchors including phase ordering and preflight | `assets/assets_test.go:62-73` |
| C104 | `SPEC.md` makes exit codes per-command contracts | `SPEC.md:137` |
| C105 | …and binds exit 3 to state-transition refusal for the reject family | `SPEC.md:148` |
| C106 | The Path B guide teaches authoring all three Markdown files then three `--manual` runs | `docs/path-b-operator-guide.md:61-73` |
| C107 | The phase → artifact → state contract table | `docs/agent-as-provider.md:40-45` |
| C108 | `status.json.notes` is currently presented as distinguishing Path B from provider output | `docs/agent-as-provider.md:47-54` |
| C109 | The feature layout marks `post-apply.patch` canonical and `patches/**` audit-only | `docs/feature-layout.md:19,34` |
| C110 | The recipe is written by the implement phase | `docs/feature-layout.md:85` |
| C111 | Capture batches are an unordered content-addressed set, not a chronology | `docs/feature-layout.md:98-100` |
| C112 | An orphaned batch left by a crash is normal and permanent | `docs/feature-layout.md:103-106` |
| C113 | WP-005 Agreed item 3: downstream SDD is encouraged, never enforced | `docs/whitepapers/WP-005-spec-driven-workflows.md:56-58` |
| C114 | WP-005 Agreed item 6: no new lifecycle state; exploration must not become mandatory | `docs/whitepapers/WP-005-spec-driven-workflows.md:71-74` |
| C115 | WP-005 Agreed item 7: the all-or-nothing publication unit and the complete-prior-or-complete-new rule | `docs/whitepapers/WP-005-spec-driven-workflows.md:75-81` |
| C116 | WP-005 Turn 3: prepare cannot call the incremental writers and claim atomicity | `docs/whitepapers/WP-005-spec-driven-workflows.turns.md:112-117` |
| C117 | WP-005 Turn 3: the required existing-primitives pre-flight | `docs/whitepapers/WP-005-spec-driven-workflows.turns.md:118-123` |
| C118 | The accepted PRD reserves plain `prepare` with exit 4 and a frozen refusal | `docs/prds/PRD-artifact-validation-and-provenance.md:356-382` |
| C119 | …and anticipates a future PRD registering the flags as an enumerated delta | `docs/prds/PRD-artifact-validation-and-provenance.md:396-403` |
| C120 | The accepted required set is the three Markdown artifacts | `docs/prds/PRD-artifact-validation-and-provenance.md:432-436` |
| C121 | The sidecar is optional and never affects readiness | `docs/prds/PRD-artifact-validation-and-provenance.md:437-441` |
| C122 | The closed nine-value state enum | `docs/prds/PRD-artifact-validation-and-provenance.md:1765-1773` |
| C123 | The frozen non-certification disclaimer | `docs/prds/PRD-artifact-validation-and-provenance.md:1796-1798` |
| C124 | The accepted exit-code table (0/1/2/3/4) | `docs/prds/PRD-artifact-validation-and-provenance.md:1981-1987` |
| C125 | The accepted stream-routing contract | `docs/prds/PRD-artifact-validation-and-provenance.md:2367-2419` |
| C126 | The accepted JSON schema and its no-map rule | `docs/prds/PRD-artifact-validation-and-provenance.md:2420-2576` |
| C127 | The accepted zero-mutation contract | `docs/prds/PRD-artifact-validation-and-provenance.md:2840-2866` |
| C128 | The forbidden provenance-inference sources | `docs/prds/PRD-artifact-validation-and-provenance.md:2904-2920` |
| C129 | The provenance ADR trigger fires only on selection of a persistent **provenance** representation | `docs/prds/PRD-artifact-validation-and-provenance.md:2966-2972` |
| C130 | ADR-034 D14 forbids citing ADR-034 as provenance precedent | `docs/prds/PRD-artifact-validation-and-provenance.md:2957-2961` |
| C131 | The accepted decision that `--manual` gates stay loose | `docs/prds/PRD-artifact-validation-and-provenance.md:2986-3015` |
| C132 | The accepted disqualifying assertion shapes | `docs/prds/PRD-artifact-validation-and-provenance.md:3531-3548` |
| C133 | The pre-change-golden prerequisite lesson | `docs/prds/PRD-artifact-validation-and-provenance.md:3515-3520` |
| C134 | The key-and-label scoping rule for forbidden-field guards | `docs/prds/PRD-artifact-validation-and-provenance.md:2554-2556,3626` |
| C135 | The guard-sensitivity requirement | `docs/prds/PRD-artifact-validation-and-provenance.md:3960-3991` |
| C136 | The accepted §20 list of what this PRD must answer | `docs/prds/PRD-artifact-validation-and-provenance.md:4016-4047` |
| C137 | The accepted slug grammar and its no-echo rule | `docs/prds/PRD-artifact-validation-and-provenance.md:696-772` |
| C138 | The accepted per-artifact classification ladder | `docs/prds/PRD-artifact-validation-and-provenance.md:1653-1782` |
| C139 | The accepted snapshot/instability semantics and their stated limits | `docs/prds/PRD-artifact-validation-and-provenance.md:1827-1917` |
| C140 | The accepted `--help` mitigation for the `apply --mode prepare` collision | `docs/prds/PRD-artifact-validation-and-provenance.md:344-355` |
| C141 | The accepted self-contained-remediation rule | `docs/prds/PRD-artifact-validation-and-provenance.md:374-381` |
| C142 | The accepted docs-update obligations, including the `agent-as-provider` correction | `docs/prds/PRD-artifact-validation-and-provenance.md:3372-3435` |

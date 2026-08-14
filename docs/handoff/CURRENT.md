# Current Handoff

## Status

**Cluster state**: REV-2 DISPATCHED

Transactional prepare-intent-bundle PRD **rev-1** and proposed ADR-035
**rev-1** closed every rev-0 finding, but three-way review found eleven
follow-up defects in lock-path stability, rooted/manual writes, archive
tombstones and purge CAS, platform/Git detection, in-root redirection, doctor
residue semantics and document consistency. A bounded **rev-2** is dispatched.
No mutating command is implemented or authorized.

## Active Task

- **Task ID**: PRD-prepare-intent-bundle
- **Description**: Define Path A generation, Path B adoption and explicit
  regeneration of a complete intent bundle with truthful transaction and
  recovery semantics.
- **Status**: Rev-2 dispatched after rev-1 NEEDS REVISION
- **Assigned**: 2026-08-13 (rev-0), 2026-08-14 (rev-1 and rev-2)
- **WAVE_BASE**: `d060ff4fc1aacaa34c865c9e620a902007805f76`
- **Issue**: [GH #11](https://github.com/tesseracode/tesserapatch/issues/11)
- **Prerequisite**: accepted artifact-validation/provenance PRD rev-5 +
  ADR-034 rev-2 — and, new in rev-1, that PRD's **implementation** must land
  before any mutating slice dispatches (PRD §17.1)
- **Release tag**: v0.15.1 remains fixed at `15560af`

## Rev-1 Review and Rev-2 Adjudication (2026-08-14)

**Internal verdict**: NEEDS REVISION
**External verdict**: NEEDS REVISION
**Reviewed writer tip**: `91dea32`
**Tracking tip before adjudication**: `94fd055`

The rev-0 findings are closed substantively. Rev-2 is limited to the following
accepted findings and their executable acceptance rows:

1. **Lock unlink/recreate (CRITICAL).** The live `flock`/deny-share lock cannot
   live at a pathname `git clean -xfd` may unlink. A second process can create
   and lock a new inode while the first still owns the deleted inode. D4 must
   select one stable cross-process authority for Git and non-Git workspaces,
   state its Windows parity and cleanup boundary, and test unlink/recreate
   rather than merely process death.
2. **Manual rooted write/CAS (HIGH).** D2 says every mutating write is rooted,
   but D3 reuses path-based `writeFileAtomic` for `--manual`. Make its
   single-file publication handle-relative and preimage-CAS-gated; retaining
   the no-journal single-file scope is acceptable.
3. **Tombstone identity and rehydration (HIGH).** X10 cannot recompute a
   pre-purge generation body after the only blob digest is cleared. Preserve
   enough immutable identity to validate a tombstone and define what happens
   when a later regeneration reproduces a tombstoned generation; it must not
   become a duplicate-id no-op plus orphan blob.
4. **Purge index CAS (HIGH).** Purge must capture the strict-decoded index
   preimage and compare it immediately before the rooted index rename. A
   concurrent index edit must refuse before any blob removal.
5. **Platform envelope (HIGH).** The shipped lock and filesystem preflight are
   implemented only for Linux/Darwin. Narrow mutating support to
   Linux/Darwin/Windows unless BSD statfs implementations and CI are part of
   the contract; keep read-only `--check`'s accepted envelope distinct.
6. **Nested Git detection (HIGH).** `Lstat(".git")` at the `.tpatch` root is a
   false negative inside a parent worktree. Use `git rev-parse
   --is-inside-work-tree` from the workspace as the authoritative detector,
   distinguish "not a work tree" from exec failure, and keep the privacy gate
   fail-closed when Git presence is unverifiable.
7. **Root guarantee wording (MEDIUM-HIGH).** `os.Root` prevents escape from
   the root but follows relative symlinks that remain inside it. Replace the
   false "pathname race closed" claim with the exact guarantee and disclose
   in-root ancestor redirection; split the acceptance coverage for outside-root
   escape and in-root redirection.
8. **Doctor row contradiction (MEDIUM).** PIB-134's "no journal → no D9" cannot
   coexist with no-journal residue rows such as PIB-386. Scope silence to a
   completely clean lane/archive and make PIB-139 conditional on being inside
   a usable Git worktree.
9. **Journal-loss detectability (MEDIUM).** An ordinary partial lifecycle
   bundle is indistinguishable from some lost-journal states. D21/doctor may
   diagnose only evidence-bearing lane/archive residue; they must not label a
   canonical mixed/partial artifact set as an interrupted prepare. State the
   undetectable boundary explicitly and add a negative ordinary-workflow row.
10. **Cross-reference consistency (LOW).** Correct T1 step 9→10, residual
    §7.7.3→§7.7.2, revalidation/CAS steps 4/8→5/9, and `S0b`→`S1b`.
11. **Tracking/matrix consistency (LOW).** The PRD has AA/S1b = 15, not 12.
    Recompute every rev-2 total, kind, category, slice and citation claim.

Also name `email-pii` alongside `home-absolute-path` in the redaction
false-positive discussion: it is the broad class observed on repository
Markdown. The claimed 430 citation occurrences are valid under the writer's
scope: 426 path-prefixed anchors plus four repeated root-level `SPEC.md`
anchors; no citation correction is required for that reviewer note.

Rev-2 remains a **docs-only** revision of the PRD, ADR-035 and handoff. It must
not implement `prepare`, change accepted prerequisites, edit source/assets, or
touch guarded WIP.

## Prepare PRD Writer Result — rev-1 (2026-08-14)

### Files changed (exact, complete)

| File | Change |
|---|---|
| `docs/prds/PRD-prepare-intent-bundle.md` | rewritten in place: 2,301 → 3,808 lines; `Status: Draft — Awaiting Review (rev-1)`; a `Revision history` row records what changed vs rev-0 |
| `docs/adrs/ADR-035-intent-bundle-publication-and-history.md` | rewritten in place: 465 → 902 lines; `Status: Proposed — Awaiting Review (rev-1)`; D1–D14 revised, D15–D21 added |
| `docs/handoff/CURRENT.md` | `Cluster state` flip to `AWAITING REVIEW`, Active Task block, and this section |

Nothing else was touched. **No** edit to `docs/ROADMAP.md`,
`docs/supervisor/LOG.md`, `docs/adrs/README.md` (the ADR-035 index row already
exists from rev-0 and its title is unchanged), `SPEC.md`, `CHANGELOG.md`,
`docs/prds/PRD-artifact-validation-and-provenance.md`, `docs/adrs/ADR-034-*.md`,
`docs/adrs/ADR-027-*.md`, `docs/whitepapers/**`, `internal/**`, `cmd/**`,
`assets/**`, `tests/**`, `.github/**`, `.wave-close-allowlist`, or any guarded
untracked WIP (`WP-004`, `WP-006`, `WP-007`, `PRD-recurring-patches.md`, the
state-of-the-art case studies).

### Counts (mechanically verified at this commit)

- **394 acceptance rows** `PIB-001`…`PIB-394`, contiguous, zero duplicates,
  zero retired. **160 new** rows (`PIB-235`…`PIB-394`); **15 rev-0 rows amended
  in place** with their IDs preserved (`PIB-015`, `-016`, `-017`, `-020`,
  `-041`, `-053`, `-079`, `-110`, `-117`, `-118`, `-124`, `-196`, `-197`,
  `-215`, `-227`), listed in §18.1.
- **38 categories** (A–T from rev-0, U–AL new): A 20, B 24, C 15, D 12, E 9,
  F 19, G 13, H 14, I 13, J 8, K 12, L 10, M 14, N 14, O 10, P 7, Q 6, R 3,
  S 9, T 2, U 10, V 12, W 5, X 6, Y 7, Z 4, AA 15, AB 7, AC 10, AD 8, AE 10,
  AF 5, AG 14, AH 17, AI 6, AJ 10, AK 10, AL 4. Sum = 394.
- **By kind**: `I` 179, `C` 79, `G` 72, `U` 43, `S` 21. Sum = 394.
- **Slice partition** is total and disjoint over eight slices: S1 75, S1b 15,
  S2 24, S3 42, S4 142, S4b 17, S5 48, S6 31 = 394.
- **165 claims** `C1`…`C165` (rev-0: 142), contiguous and ascending in file
  order; 23 new, 9 corrected (marked †).
- **ADR-035: 21 decisions** `D1`…`D21`, contiguous; every one is cited by the
  PRD and every one has an acceptance-dependency row naming real PIB rows.
- **12 crash phases** `CP0`…`CP11` (rev-0: 10), each with a recovery outcome
  and a row; **no** phase can leave a slug permanently blocked.
- Closed catalogs: 7 exit codes (4 still retired), **17** advisories (rev-0:
  10), 6 staged-output checks `V1`…`V6`, **10 journal binds** `J1`…`J10`,
  **10 index binds** `X1`…`X10`, 19 injection seams (rev-0: 12).
- **430 `file:line` citations** across both documents; all mechanically
  verified in range at this commit; zero out-of-range, zero missing files.

### How each rev-0 finding was closed

**Transaction / recovery**

1. **Stale locks (CRITICAL)** — the `O_EXCL`-plus-nonce model is **gone**. The
   lock is the shipped kernel `flock(2)` process-lifetime lock
   (`internal/rescap/lock_unix.go:63-88`), extracted into a shared package,
   plus a Windows deny-share `CreateFile` sibling and a fail-closed stub.
   Ownership is released by the OS on process death; the lock **file** carries
   zero bytes and no authority, so CP1/CP2/CP8 and every `--manual` crash are
   ordinary. Contention is a transient **exit 3**, not exit 6. §7.4, ADR-035
   D4, PIB-284…PIB-295.
2. **Semantic CAS (CRITICAL)** — every publish, undo, remove, blob write,
   index rewrite and purge is compare-and-swap gated on the target's *current*
   identity (§7.6.3). The journal binds itself to its plan by `plan_digest` and
   passes ten strict binds (J1–J10) before any restore, so a forged or corrupt
   journal refuses and preserves evidence. Both metadata paths are now full
   entries with preimage and new-image. §7.5, §7.6, ADR-035 D5, PIB-275…PIB-307.
3. **T1 scope (HIGH)** — T1 is now "at a command-owned final verification point
   under the lock, every entry matches its intended new image". External writes
   after that point are explicitly outside the postcondition; writes during the
   window are detected where observable and produce **divergence, not
   destructive rollback**. §7.1, §7.8 step 10, ADR-035 D1/D6, PIB-314, PIB-315,
   PIB-324.
4. **Rooted writes (HIGH)** — `(*os.Root).Rename`/`OpenFile`/`Remove`/`Mkdir`
   are adopted for every publication write (Go 1.26.1 is pinned, `go.mod:3`).
   No path-based `os.Rename`/`os.CreateTemp` after a rooted check. The
   ancestor-pathname race is **closed**; the residual is narrowed to content
   identity. §7.7, ADR-035 D2, PIB-308…PIB-313.
5. **Exit 6 / doctor (HIGH)** — new `--abandon-transaction` mode: takes the
   lock, moves journal + preimages + staging into `abandoned-<12hex>/`, touches
   **no** canonical file, previews without `--yes`. Doctor D9 now reports the
   total residue set (pending journal, live transaction via a non-creating lock
   probe, inert lock file as informational, staging, abandoned evidence, orphan
   blobs, corrupt index, journal loss). §6.6, §12.5, ADR-035 D13, PIB-268…
   PIB-274, PIB-362, PIB-363, PIB-380…PIB-387.

**Archive / privacy**

6. **Default coherence + sidecar (CRITICAL)** — the default mode preserves
   every present artifact **including the sidecar**, generates only a
   dependency-coherent **suffix** of analysis → spec → exploration, and refuses
   incoherent gaps with `incoherent-bundle-gap` (exit 2) naming `--regenerate`
   and `--manual`. Consequence: the default mode has **no `replace` entry
   ever**, so §9.2.1's per-mode archive table is total and unambiguous. §6.1.2,
   §6.1.3, ADR-035 D19, PIB-245…PIB-256.
7. **ADR-027 D3 (CRITICAL)** — the redaction contract is a **write
   precondition**: `redact.Scan`'s closed six classes run on in-memory bytes
   before any blob exists, and a match **refuses the whole regeneration**
   (exit 3, `archive-content-refused-sensitive`) naming artifact ids and class
   codes only. Refusal rather than scrubbing is justified by the exact-recovery
   conflict. No override flag, no config key. §9.6.1, ADR-035 D15,
   PIB-262…PIB-267, PIB-388.
8. **Retention (HIGH)** — "no new exposure class" is **withdrawn** and replaced
   by an explicit disclosure that the archive converts a destructive overwrite
   into durable retention; tracking is conditional on `.tpatch/` being tracked
   and no clone-durability is claimed before a commit. A bounded surface ships:
   `tpatch feature intent-archive list|purge` with tombstones, reference
   checks, orphan collection, dry-run-by-default and `--yes`. "Immutable" now
   means *never modified in place*, not *undeletable*. §9.6.2, §9.7, ADR-035
   D8/D16, PIB-316, PIB-317, PIB-342…PIB-361.
9. **Index integrity (MEDIUM)** — ten strict binds `X1`…`X10` including
   version-ahead ("upgrade tpatch", not "corrupt"), foreign-feature, path
   escape, closed ids, tombstone consistency and `generation_id` recomputation.
   §9.3.1, ADR-035 D10, PIB-331…PIB-341.
10. **Orphan blobs (MEDIUM)** — reported by `list`, `doctor` and by every
    outcome that leaves one; removable with `purge --orphans`. The exit-5
    message no longer claims the whole tree is byte-identical. §7.9, §9.7.3.

**Product / compatibility**

11. **Regenerate fallback (HIGH)** — `--regenerate` now **requires a configured
    provider and provider success**; no provider refuses before any mutation
    (exit 3), a failure or deadline refuses before publication (exit 5).
    `--allow-heuristic` is the only opt-in, with full grammar, help text and no
    config key. Deadlines are **per-phase (`--timeout-phase`, 90s) and total
    (`--timeout`, 180s)**, and a total expiry that causes ≥2 fallbacks emits a
    distinct cascade advisory. §11.3, §11.5, ADR-035 D18, PIB-365,
    PIB-368…PIB-375.
12. **FEATURES.md (HIGH)** — named as derived, best-effort, **outside T1**,
    never journaled, never rolled back; `--manual`'s write set is
    `{status.json} ∪ {FEATURES.md best-effort}`; rollback and recovery re-run
    the refresh; a failure is reported as an advisory. §12.3.1, ADR-035 D20,
    PIB-257…PIB-261.
13. **Git (HIGH)** — the "no Git operation" claim is **corrected**. Four exact
    read-only invocations are enumerated with a spy row, and the lane gate is
    made **conditional**: no `.git` → skip with advisory `workspace-not-git`
    (Path B keeps working); `.git` + usable git → shipped gate; `.git` +
    unusable git → refuse `local-lane-unverifiable`, disclosed as delta D9.
    §7.13, ADR-035 D17, PIB-279…PIB-283, PIB-326…PIB-329.
14. **Prerequisite sequencing (MEDIUM)** — §17.1 makes the accepted `--check`
    **implementation** a hard dispatch precondition, and PIB-391 asserts its
    goldens come from that cluster's commit range, not this one. S1b adds
    pre-change `feature resource` goldens for the lock extraction.
15. **Dry-run (MEDIUM)** — with a pending journal it now refuses
    `recovery-pending` (exit 3) instead of printing a plan that recovery would
    invalidate. §6.4, PIB-268, PIB-269.
16. **Journal-loss boundary (MEDIUM)** — `git clean -xfd` / fresh clone /
    out-of-band delete is an explicit **T2 boundary**: doctor detects, never
    repairs. Relocating the journal under `.git` was evaluated and rejected
    with reasons. §7.11.1, ADR-035 D21, PIB-321…PIB-323.
17. **Flags / lifecycle / notes / citations (MEDIUM–LOW)** — a complete flag
    delta table covering every newly registered flag plus a completeness guard
    (§5.3, PIB-236…PIB-244); exit 4 stays retired; every exit-6 population
    names its escape; `notes` is stated as a last-transition hint, not
    provenance (§12.3.2); and the drifted citations are fixed —
    `landJournalFileState` has **no size** (C56 †, size is now declared as this
    design's deliberate extension), the `O_EXCL`/inode lock precedent is cited
    only as the rejected alternative (C63 †, C65 †), `PIB-215`→`PIB-217` for
    the WP-005 non-mandate assertion, `internal/cli/phase2.go:50` for `--interactive`,
    `CLAUDE.md` decision **8** (not 7) for offline fallback, and nine
    `resource_publish.go` / `doctor.go` / `scratch.go` ranges re-resolved.

### Validation performed (docs-only change)

- **Anchors**: 430 `file:line` citations parsed and checked in range; zero
  out-of-range, zero missing files. Every newly written anchor was
  content-verified by reading the cited line, and ten ranges were tightened
  after that read (`internal/rescap/lock_unix.go:43-52`, `internal/cli/feature_resource.go:101-116`,
  `internal/redact/redact.go:18-21`, `internal/cli/feature_deps.go:43-53`, `internal/cli/land_journal.go:383-415`,
  `internal/store/resource_publish.go:131-143` / `:305-328` / `:358-399`, `internal/workflow/doctor.go:226-237`,
  `internal/cli/session.go:353-419`). All shorthand `:NNN-NNN` anchors were expanded to full
  paths so the checker can see them.
- **Matrix mechanics**: 394 rows contiguous, zero duplicates; every `PIB-NNN`
  cited anywhere in either document resolves to a real row; category counts sum
  to 394; the slice partition is total and disjoint.
- **Claims mechanics**: `C1`…`C165` contiguous **and ascending in file order**.
- **PRD ↔ ADR parity**: ADR decisions `D1`…`D21` are contiguous; **every** one
  is cited by the PRD; every acceptance-dependency row names PIB rows that
  exist; every `PRD §N.N` reference in the ADR resolves to a real PRD heading.
- **Markdown hygiene**: fences balanced (24 PRD, 0 ADR); zero table
  column-count mismatches (escaped `\|` accounted for); zero trailing-whitespace
  lines; every relative link resolves on disk; all `json` blocks parse; every
  `§N.N` reference resolves to a real heading.
- **Over-claim audit**: every occurrence of "atomic"/"atomically"/
  "simultaneously" was re-read; each is either about a genuinely single-file
  rename, about a shipped primitive's name, or an explicit statement that the
  property is **not** claimed.
- No Go source changed, so `gofmt` / `go build` / `go test` do not apply to
  this change set. `internal/workflow/docs_totality_guard_test.go` reads three
  verify-family documents, none of them touched here.

### Implementation status

**None.** No Go file, test, asset or CLI surface was created or modified. §19
now states a **three-part** gate: this PRD accepted, ADR-035 accepted, and the
accepted `prepare --check` PRD **implemented and landed** with its own goldens.

### Reviewer focus for rev-1

1. **§7.4 is the load-bearing replacement.** Test whether the kernel-lock model
   really removes every "permanently blocked slug" path — including the
   Windows deny-share sibling, the narrowed platform envelope (delta D8) and
   the claim that a leftover lock file is inert.
2. **§7.6.3's CAS table must be total.** Look for a mutating step that is not
   in it, and for any path where an undo could overwrite third-party bytes.
3. **§7.1's T1 wording.** The question is whether any sentence anywhere still
   implies a world-state guarantee after the command returns. PIB-155 and
   PIB-324 are the guards.
4. **§9.6 is the privacy answer.** Challenge the refuse-don't-scrub choice, the
   false-positive cost (R12, Q9), and whether §9.6.2's disclosure is complete —
   especially the committed-history caveat in §9.7.4.
5. **§9.7's purge ordering.** Verify that the index-then-blob order really
   lands crashes in the recoverable direction and that reference counting
   cannot orphan a live reference.
6. **§11.3.2's provider authority.** Is refusing `--regenerate` without a
   provider the right default for offline users, or does `--allow-heuristic`
   need to be more discoverable?
7. **§7.13.2's third row.** The `local-lane-unverifiable` refusal is a real
   regression against `define --manual`. Challenge whether the privacy argument
   justifies it.
8. **§17.1 and PIB-391.** Confirm the prerequisite is stated strongly enough
   that no implementer can dispatch a mutating slice on top of an unimplemented
   read half.
9. **Matrix bite.** 160 new rows: check that each names an *observable*, that
   the `G` rows have sensitivity fixtures, and that §18.41's six semantic
   fixtures are the right six.

### Open decisions left to review (§20, thirteen)

Q1 `--regenerate --only <ids>`; Q2 the 180s total deadline; Q3 a default-mode
`--require-provider`; Q4 a `--restore` verb; Q5 automatic retention policy;
Q6 whether `--regenerate` invalidates `verified-fresh`; Q7 per-feature vs
per-workspace archive; Q8 whether `doctor` D9 should be an error; **Q9** a
scoped redaction override; **Q10** the 90s per-phase deadline; **Q11**
`feature intent-archive` vs a top-level `tpatch archive`; **Q12** whether
`solaris`/`aix` deserve a different lock primitive; **Q13** whether a
non-allowlisted filesystem can opt in to mutating `prepare`. All thirteen have
a stated default; none blocks review.

## Prepare PRD Writer Contract

- Default Path A generates only missing intent artifacts and preserves every
  existing non-empty canonical file.
- `--manual` adopts an already structurally complete bundle without provider
  calls.
- `--regenerate` is the only overwrite route and must preserve prior
  hand-authored bytes under an explicit durable policy.
- Successful mutation ends at existing `defined`; no new lifecycle state.
- Publication unit is all three Markdown files, structured sidecars and final
  `status.json`.
- Distinguish command-boundary rollback/crash recovery from instantaneous
  multi-file visibility; do not call a rename sequence atomic.
- Define provider/heuristic fallback, timeouts, locks, journals, preimages,
  failure/recovery/idempotency and concurrent-editor behavior.
- Define allowed lifecycle states and interaction with `prepare --check`,
  individual phases, `next`, `cycle`, reject/reopen and post-implementation
  states.
- Preserve accepted rooted inspection/privacy/provenance boundaries.
- Evaluate persistent history/pointer/provenance alternatives. If one is
  selected, create the required ADR before acceptance.
- Include claims audit and executable acceptance matrix; no implementation.

## Prepare PRD Writer Result — rev-0 (2026-08-13)

### Files changed (exact, complete)

| File | Change |
|---|---|
| `docs/prds/PRD-prepare-intent-bundle.md` | **new**, 2,301 lines, `Status: Draft — Awaiting Review (rev-0)` |
| `docs/adrs/ADR-035-intent-bundle-publication-and-history.md` | **new**, 465 lines, `Status: Proposed (rev-0)` — created because the architecture gate fired |
| `docs/adrs/README.md` | one index row added for ADR-035 |
| `docs/handoff/CURRENT.md` | `Cluster state` flip, Active Task `Status`, and this section |

Nothing else was touched. **No** edit to `docs/ROADMAP.md`,
`docs/supervisor/LOG.md`, `SPEC.md`, `CHANGELOG.md`,
`docs/prds/PRD-artifact-validation-and-provenance.md`,
`docs/adrs/ADR-034-*.md`, `docs/whitepapers/**`, `internal/**`, `cmd/**`,
`assets/**`, `tests/**`, `.github/**`, `.wave-close-allowlist`, or any guarded
untracked WIP (`WP-004`, `WP-006`, `WP-007`, `PRD-recurring-patches.md`, the
state-of-the-art case studies).

### Counts (mechanically verified)

- **234 acceptance rows** `PIB-001`…`PIB-234`, contiguous, zero duplicates,
  zero retired. **20 categories**: A 20, B 24, C 15, D 12, E 9, F 19, G 13,
  H 14, I 13, J 8, K 12, L 10, M 14, N 14, O 10, P 7, Q 6, R 3, S 9, T 2.
  Sum = 234. Every row is assigned to exactly one of six slices
  (S1 46, S2 14, S3 22, S4 97, S5 28, S6 27 = 234).
- **142 claims** `C1`…`C142`, contiguous, each with a `file:line` anchor.
- **ADR-035: 14 decisions** `D1`…`D14`, each mapped to named PIB rows in the
  ADR's acceptance-dependency table.
- **171 distinct `file:line` anchors** (355 citations) across both documents;
  all mechanically verified in range, zero out-of-range, zero missing files.
- **10 crash phases** `CP0`…`CP9`, each with a recovery outcome and an
  acceptance row.
- Closed catalogs: 7 exit codes (one deliberately retired), 10 advisories,
  6 validation checks `V1`…`V6`, 12 injection seams.

### Contract decisions made (not deferred)

1. **Four modes on one verb**: `--check` (accepted, frozen), default
   `generate`, `--manual`, `--regenerate`, plus `--dry-run`, declared mutually
   exclusive through cobra so any conflict is a parse-time exit 1.
2. **Default preserves.** Every `present-nonempty` canonical artifact is never
   staged and never renamed over. `present-empty` refuses (exit 2);
   `symlink-refused` / `not-regular` / `unreadable` / `oversize` / `unstable`
   refuse (exit 3). The disposition table is total over the accepted nine-value
   enum.
3. **The sidecar is written iff `analysis.md` is generated this run**, so a
   preserved (possibly hand-authored) analysis never gets a synthesized
   structured sidecar — which would be a false Path A signal.
4. **`--manual` writes exactly one file** (`status.json`) and therefore takes
   **no journal and no archive**: one rename genuinely is atomic. It still
   takes the lock.
5. **`--regenerate` is all-or-nothing over the whole bundle** (three Markdown
   files + sidecar), because `spec` derives from `analysis` and `exploration`
   from both; partial regeneration would publish an incoherent set.
6. **Transaction truth is a three-way split.** T0 instantaneous multi-file
   visibility is **NOT** claimed (a rename sequence cannot provide it, and the
   exposure window is bounded and named); T1 command-boundary all-old/all-new
   and T2 crash recoverability **are**. The words "atomic"/"simultaneously" are
   forbidden for the multi-file publication and mechanically guarded.
7. **The journal is undo-only** — the deliberate inverse of `land`, which must
   roll forward because `git commit` is irreversible. Recovery decides from
   evidence; the journal carries no `phase` field and no wall-clock.
8. **Publication order is fixed with `status.json` last**, which is what makes
   the 10-phase crash table enumerable and every partial state recoverable.
9. **Lock authority is narrow and the limit is disclosed**: it excludes only a
   sibling mutating `prepare`. Other writers are handled by publish-time
   identity revalidation, and the residual in-window race is stated as a limit
   with no row claiming otherwise.
10. **Only mutating `prepare` recovers**; `doctor` gains a report-only `D9`
    check. The evaluated "every command must recover or refuse" model was
    rejected, and so was any pointer/generation-indirection model — canonical
    files stay self-describing.
11. **Exit envelope**: 0/1/2/3/5/6 with `4` deliberately **retired, not
    rebound** (its reserved-surface population disappears). 0/2/3 keep the
    accepted `--check` meanings, and exit 3 also carries lifecycle refusal to
    match the shipped `reject`/`reopen`/`unapplied` convention.
12. **Generators are extracted as pure functions**; `RetryOptions.Store` is
    left nil so raw provider responses land in the gitignored staging lane
    instead of `artifacts/` — an enumerated delta, and a privacy improvement.
13. **Provider failure falls back to the heuristic generator**, matching every
    shipped phase command, and still yields a complete new set. No
    `--require-provider` in v1.
14. **No confirmation prompt and no `--yes`**: the archive is the safety
    mechanism, `--dry-run` is the preview, and prompts break harnesses.
15. **Seven enumerated behavior deltas (D1–D7)** and six non-invalidation
    obligations, each with golden-pinned rows.

### Architecture gate — trigger and disposition

- **Fired.** `--regenerate` cannot preserve prior hand-authored bytes without a
  persistent representation. Six alternatives were evaluated (refuse overwrite;
  rely on Git; ephemeral rollback-only journal; durable immutable
  content-addressed snapshots; pointer/symlink generation directories;
  `FeatureStatus` sub-record / hash manifest).
- **Selected**: durable, **tracked**, immutable, content-addressed **intent
  archive** at `.tpatch/features/<slug>/artifacts/intent-archive/` (blobs +
  `index.json`), canonical files remaining the sole authority. It reuses the
  shipped resource-capture shape (immutable content-addressed set + atomically
  rewritten manifest) rather than inventing one.
- **Therefore** `docs/adrs/ADR-035-intent-bundle-publication-and-history.md`
  was created as **Proposed rev-0** and the ADR index updated. **Neither
  document may be accepted alone.**
- **Provenance trigger NOT fired.** The archive records *bytes that existed at
  a path* — no author, agent, model, provider, endpoint or Path-A/B tag.
  `provenance: unknown` stays constant, the forbidden-inference list gains
  exactly one entry (the archive), and ADR-034 is explicitly not cited as a
  persistence precedent (ADR-034 D14 forbids it).

### Determinism / privacy invariants held

- No wall-clock field in any tracked artifact this command writes, **or in the
  journal**. The only clock that moves is the pre-existing `status.json`
  `updated_at`.
- Content-addressed blob names and `generation_id`; one derivation point;
  idempotent append (the archive is a set, not a chronology).
- Canonical JSON, fixed key order, no Go map in any wire format.
- Raw provider responses never reach the tracked tree; secrets remain
  by-reference; the forbidden-field guard scopes to keys and labels, with a
  sensitivity fixture.

### Validation performed (docs-only change)

- **Anchors**: 355 `file:line` citations, 171 distinct, parsed and checked in
  range; zero out-of-range, zero missing files. Every anchor that was written
  from memory was then **content-verified** by reading the cited line, and 40+
  drifted anchors were re-resolved (workflow.go generator/validator/heuristic
  sites, store.go temp-create and config, types.go/status.go structs,
  land_journal.go `phase`/`created_at`, resource_publish.go derivation and
  strict-decode sites, phase2.go interactive/loader/skip-execute, cobra.go
  probing loader and manual helpers, SPEC.md exit rows, feature-layout and
  agent-as-provider sections, WP-005 Agreed items 3/6/7 and Turn-3 items 2/3,
  and 61 accepted-PRD section anchors).
- **Matrix mechanics**: 234 rows contiguous; zero duplicates; every `PIB-NNN`
  cited in either document resolves to a real row; category counts sum to 234;
  slice assignment is a partition.
- **Claims mechanics**: `C1`…`C142` contiguous, zero duplicates. Crash phases
  were renamed `C*`→`CP*` specifically so they cannot be confused with claim
  IDs in a table scan.
- **Markdown hygiene**: fences balanced (20 markers PRD, 0 ADR); zero
  column-count mismatches across all tables in both files; zero
  trailing-whitespace lines; every relative link resolves on disk; all three
  `json` blocks parse; every `§N.N` reference resolves to a real heading in
  both documents; every `ADR-0NN D<k>` reference is inside that ADR's decision
  range.
- No Go source changed, so `gofmt` / `go build` / `go test` do not apply to
  this change set. `internal/workflow/docs_totality_guard_test.go` reads three
  verify-family documents, none of them touched here.

### Implementation status

**None.** No Go file, test, asset or CLI surface was created or modified. §19
of the PRD states the authorization gate: implementation is blocked until
**both** the PRD and ADR-035 are accepted.

### Reviewer focus for rev-0

1. **§7.1 is the load-bearing honesty claim.** The question to test is not "is
   this a transaction" but "does the T0/T1/T2 split hold everywhere" — i.e.
   whether any sentence, message, table or row anywhere in either document
   implies instantaneous multi-file visibility. PIB-155 is the guard; challenge
   its scope.
2. **§8's alternatives table is the architecture gate.** Challenge the
   *rejection* of H2 (Git) and H3 (ephemeral journal) specifically: the whole
   ADR-035 requirement rests on the success-path argument, not the crash path.
3. **§9.6 must survive adversarial reading.** If any reviewer can construct a
   provenance question the archive answers, the provenance trigger fires and
   ADR-035 is insufficient.
4. **§7.9's CP-table plus §7.10's decision function.** Check that no reachable
   crash point is missing a row, that CP7 (evidence beats phase) is right, and
   that undo-only really loses nothing.
5. **§7.4's lock limit.** The design deliberately does not lock shipped
   commands. Challenge whether revalidation plus the disclosed residual window
   is an acceptable trade, or whether a delta on `define`/`cycle` is required.
6. **§12.6's seven deltas.** Verify the list is complete — particularly that
   retiring exit 4 and changing the two `--check`-plus-flag message texts are
   the only observable changes to the accepted surface.
7. **§17's partition** must satisfy the AGENTS.md same-file rule:
   `internal/cli/prepare.go` and `internal/workflow/workflow.go` are shared
   surfaces, so any two implementers touching either must run sequentially.

### Open decisions left to review (§20, eight)

Q1 `--regenerate --only <ids>`; Q2 the 180s whole-command deadline; Q3 a
`--require-provider` flag; Q4 a `--restore` verb; Q5 archive pruning; Q6
whether `--regenerate` should invalidate a `verified-fresh` label; Q7
per-feature vs per-workspace archive; Q8 whether `doctor` D9 should be an
error. All eight have a stated default; none blocks review.


## Final Verdict

- **Internal**: APPROVED at `cd15165`.
- **External**: APPROVED at `0275067`; final byline/guard folds changed no
  decision or matrix shape.
- **Accepted contract**: PRD rev-5, 208 AVP rows.
- **Accepted architecture**: ADR-034 rev-2, D1–D18.
- **Claims**: 95 repository + 24 Go-stdlib claims.
- **Implementation**: none.
- **Provenance ADR trigger**: still unfired; `unknown` remains constant.
- **Prepare PRD**: may now enter planning, but mutating implementation remains
  unauthorized until that separate PRD is accepted.

## PRD Writer Contract

- Claims-audit current `manual.go`, Path A workflow, `FeatureStatus.Notes`,
  `next`, `cycle`, feature layout and shipped skill surfaces.
- Define deterministic structural artifact states without claiming semantic
  quality.
- Define read-only `prepare --check` human/JSON output, precedence and exits.
- Pin `provenance: unknown` until accepted metadata proves otherwise.
- Decide explicitly whether individual mutating `--manual` gates change.
- Preserve existing `defined`, `next` and `cycle` behavior unless a behavior
  delta is enumerated and accepted.
- Cover symlinks, non-regular files, unreadable paths, privacy, determinism,
  migration and Path A/B parity.
- Evaluate provenance representation alternatives. No ADR is pre-created; if
  the PRD selects persistent representation, flag the ADR gate before
  acceptance for implementation.
- Exclude mutating prepare, regenerate, provider execution, atomic bundle
  publication, new lifecycle state and downstream SDD mandates.
- Mark WP-005 Graduated when the PRD file is created.

## PRD Writer Result — rev-0 (2026-08-13)

### Files changed (exact, complete)

| File | Change |
|---|---|
| `docs/prds/PRD-artifact-validation-and-provenance.md` | **new**, 1,472 lines, `Status: Draft — Awaiting Review (rev-0)` |
| `docs/whitepapers/WP-005-spec-driven-workflows.md` | header only: `Exploring` → `Graduated (2026-08-13)` + one `Related` bullet to the PRD. Body and turn log untouched. |
| `docs/whitepapers/README.md` | index row: WP-005 `Exploring` → `Graduated (2026-08-13)` |
| `docs/handoff/CURRENT.md` | this section + `Cluster state` flip |

Nothing else was touched. No edit to `docs/ROADMAP.md`, `docs/supervisor/LOG.md`,
`SPEC.md`, `docs/adrs/**`, `internal/**`, `cmd/**`, `assets/**`, `tests/**`,
`.wave-close-allowlist`, or any guarded untracked WIP (`WP-004`, `WP-006`,
`WP-007`, `PRD-recurring-patches.md`, state-of-the-art case studies).

### Counts

- **64 claims** in the §23 claims-audit appendix, each with a `file:line`
  anchor.
- **95 acceptance rows** (`AVP-001`…`AVP-095`) across 12 categories: CLI
  grammar 10, classification 20, readiness/exit 8, output shape 14, zero
  mutation 6, provenance 5, compatibility 9, Path A/B parity 4,
  security/privacy 6, concurrency 4, source scans/parity 6, totality guards 3.
- **133 distinct `file:line` anchors** across the whole document; all
  mechanically verified in-range at HEAD `12980f2`, and the load-bearing ones
  content-verified by reading the cited lines.

### Contract decisions made (not deferred)

1. **Scope**: `tpatch prepare <slug> --check [--json] [--quiet] [--path]` only.
   Plain `prepare` refuses with exit 4 *before* the store is opened;
   `--manual` / `--regenerate` are deliberately **unregistered** (cobra
   unknown-flag → exit 1).
2. **Turn-4 question answered**: mutating `analyze|define|explore|implement
   --manual` gates are **unchanged** in this slice. The inspector is pure and
   wired to `prepare --check` only. Six acceptance rows (`AVP-064`…`AVP-069`)
   pin the *loose* current behavior — including "zero-byte `spec.md` still
   advances to `defined`" — as deliberately unchanged. §12.3 lists what a
   future tightening PRD must enumerate.
3. **Closed 9-value state enum** extending the shipped
   `absent`/`present-empty`/`present-nonempty` vocabulary with
   `symlink-refused`, `not-regular`, `unreadable`, `oversize`,
   `invalid-structured`, `unstable`; 17-row total precedence table.
   `unstable` deliberately outranks every content-derived state so a
   mid-truncation read is never reported as `present-empty`.
4. **Required = `analysis.md` + `spec.md`.** `exploration.md` and
   `artifacts/analysis.json` are optional and can never change readiness; the
   sidecar is reported separately, and no output field may assert Path A vs
   Path B for a feature (that would be provenance inference from sidecar
   presence).
5. **Provenance = constant `unknown`.** Seven forbidden inference sources
   enumerated with anchors. Four persistence alternatives evaluated (status
   sub-record / dedicated manifest / derivation-only / write-time attestation);
   **none selected in rev-0** — the PRD is coherent without one — and the ADR
   trigger is stated as a gate on any future PRD that selects one.
6. **Exit envelope**: 0 ready / 1 generic CLI / 2 not_ready / 3 indeterminate
   (abort or required-artifact instability) / 4 reserved-surface refusal, with
   a six-step first-match-wins precedence and five closed abort codes.
7. **Output**: `schema_version: 1`, fixed key order, fixed-length 4-element
   `artifacts` array (even on abort), closed reason/advisory/abort catalogs,
   frozen disclaimer string, verify-style `--json`/`--quiet` stream routing.
   No wall-clock, no size, no hash, no content, no absolute path, no symlink
   target — 17 forbidden field names asserted absent.
8. **Snapshot semantics**: one descriptor-scoped capture per artifact, three
   named instability probes, **explicitly stated limits** (a same-length
   in-place rewrite is not detectable), no retry, no lock, and no
   cross-artifact atomicity claim.
9. **Skill parity**: v1 ships the command in all six surfaces and extends
   `requiredCommands` + `requiredAnchors` (`AVP-090`…`AVP-092`).
10. **Blocked-PRD statement**: `PRD-prepare-intent-bundle.md` remains blocked
    until this PRD is accepted — stated in the header, the summary and §20.

### Open decisions left to review (§21, four)

Q1 a later `--all` sweep mode; Q2 whether exit `4` becomes a cross-command
convention or stays local to `prepare`; Q3 the `MaxArtifactBytes = 4 MiB`
value; Q4 whether `request.md` becomes a fifth optional reported row. All four
have a stated default and none blocks review.

### Implementation status

**None.** No Go file, test, asset or CLI surface was created or modified. The
PRD defines five ordered implementation slices (S1–S5) for a future wave and
explicitly requires the AGENTS.md parallel-implementer file partition to be
declared at dispatch.

### Validation performed (docs-only change)

- Mechanical anchor audit: 133 distinct `file:line` citations parsed and
  checked in-range against the working tree; **zero** out-of-range, zero
  missing files. Re-run after the WP-005 header edit shifted that file by +2
  lines, and the ten affected anchors were re-resolved and content-verified.
- Markdown hygiene: fence balance (16 markers, balanced), table column
  consistency (0 mismatched rows), zero trailing-whitespace lines, all 13
  relative links resolve on disk.
- The `json` example block parses as valid JSON.
- No Go source changed, so `gofmt` / `go build` / `go test` are not applicable
  to this change set; no existing docs test targets these files
  (`internal/workflow/docs_totality_guard_test.go` reads exactly three
  verify-family documents, none of them touched here).

### Notes for the reviewer

- The PRD deliberately does **not** claim a test proves semantics merely by
  existing: §18.1 states five disqualifying assertion shapes, and §18.14
  requires sensitivity regressions for the three mechanical guards
  (`AVP-093`/`094`/`095`) because a byte-scanning guard can false-pass — the
  lesson from the F2 finding recorded above in this file.
- §4 is the required WP-005 existing-primitives pre-flight and covers
  `--manual`, `cycle`, `next`, `verify`, `status`, `doctor` and the generic
  path/fs helpers, each with an anchor.
- The `apply --mode prepare` name collision is treated as a real risk with two
  mandated help-text mitigations and two acceptance rows, not waved away.

## Rev-0 Review Adjudication

- **Internal**: NEEDS REVISION (3 HIGH, 2 MEDIUM).
- **External**: APPROVED WITH NOTES; F1–F4 block implementation dispatch and
  F5–F13 are valid completeness notes.
- **Supervisor verdict**: NEEDS REVISION.

### Rev-1 required corrections

1. **CLI/output envelope**: compose quiet and all nonzero paths with the root
   error printer; define abort `feature_state`, overall/readiness and artifact
   shape; keep refusal help self-contained.
2. **Safe bounded capture**: canonical slug validation before path use;
   race-safe no-follow/nonblocking open + post-open fstat; size checks before
   and after open; bounded `Max+1` reads; total fstat/read failure rows.
3. **Readiness truth**: because `prepare` is optional, its `ready` verdict
   requires all three canonical Markdown artifacts. The analysis sidecar stays
   optional. Do not conflate optional command adoption with bundle
   completeness.
4. **Total diagnostics**: advisory selection must be state-accurate; a missing
   `status.json` continues with `feature_state: unknown`; unsafe slugs and
   every abort population have closed codes.
5. **Compatibility/acceptance bite**: add composite loose-`--manual` versus
   strict-check regressions, reverse call-graph routing guards, sidecar
   instability, growth/FIFO/path traversal and quiet-abort rows; scope
   forbidden-field guards to keys/labels rather than substrings.
6. **Provenance/docs**: define `unknown` as the stable "not provable" meaning,
   add the future `agent-as-provider` correction, and keep the ADR trigger
   bounded.

## PRD Writer Result — rev-1 (2026-08-13)

### Files changed (exact, complete)

| File | Change |
|---|---|
| `docs/prds/PRD-artifact-validation-and-provenance.md` | rewritten in place; `Status: Draft — Awaiting Review (rev-1)`; a `Revision history` table records what changed vs rev-0 |
| `docs/handoff/CURRENT.md` | `Cluster state` flip to `AWAITING REVIEW`, Active Task block, and this section |

Nothing else was touched. **No** edit to `docs/ROADMAP.md`,
`docs/supervisor/LOG.md`, `SPEC.md`, `docs/adrs/**`, `docs/whitepapers/**`
(WP-005 was already marked Graduated in rev-0 and needed no further change),
`docs/prds/PRD-prepare-intent-bundle.md` (still undrafted and still blocked),
`internal/**`, `cmd/**`, `assets/**`, `tests/**`, `.wave-close-allowlist`, or
any guarded untracked WIP (`WP-004`, `WP-006`, `WP-007`,
`PRD-recurring-patches.md`, the state-of-the-art case studies).

### Counts (mechanically verified, commit-independent)

- **2,233 lines** in the PRD (rev-0: 1,478).
- **75 claims** in the §23 claims-audit appendix (rev-0: 64), each with a
  `file:line` anchor; `C1`…`C75` contiguous with no gaps.
- **140 acceptance rows** `AVP-001`…`AVP-140` (rev-0: 95), contiguous, no
  duplicates, no retired/struck rows. Nineteen categories:
  A 10, B 20, C 8, D 14, E 6, F 5, G 9, H 4, I 6, J 4, K 6, L 3, M 6, N 5,
  O 12, P 10, Q 1, R 9, S 2.
- **By kind**: `U` 38, `I` 80, `S` 6, `G` 13, plus 3 combined-kind rows
  (`S+G`, `S+I`, `I+G`). 15 rows carry a guard component and are covered by
  the §18.21 sensitivity requirement.
- **161 distinct `file:line` anchors** across 33 files; all mechanically
  verified in-range against the working tree, zero out-of-range, zero missing
  files. Four rev-0 anchors were re-resolved after content verification
  (`phase2.go` `fileExistsAt`, `assets_test.go` skill-file/anchor blocks,
  `workflow.go` heuristic block, `verify.go` stream routing).
- **45 new rows** (`AVP-096`…`AVP-140`); every rev-0 ID kept its number, and
  the rows whose meaning changed (`AVP-031`…`AVP-036`, `AVP-042`, `AVP-048`,
  `AVP-051`, `AVP-069`, `AVP-071`, `AVP-073`, `AVP-075`, `AVP-092`,
  `AVP-095`) were amended in place per §18.1's no-renumbering rule.
- **All 140 rows are assigned to exactly one implementation slice** (S1–S5);
  zero unassigned, zero double-assigned.

### Contract decisions in rev-1 (what changed, and why)

1. **Readiness = the full intent bundle.** `ready` now requires all three
   canonical Markdown artifacts present-and-non-empty. `artifacts/analysis.json`
   stays optional and can never affect readiness. §6.2.1 gives four cited
   reasons (bundle unit per WP-005 Agreed item 7; the Path B operator guide
   already teaches authoring all three; alignment with `verify`'s later
   blocking `intent_files_present`; every gap still fully reported). §6.2.2
   answers the WP-005 "must not make exploration mandatory" constraint with a
   six-question table separating **optional command adoption** from **relaxed
   bundle semantics** — nothing calls the command, `defined` is still reachable
   without exploration, `next`/`cycle` are byte-identical.
2. **Output composed with the real root error printer.** `cobra.go:33-39`
   prints `error: %v` for every non-nil `RunE` error, so rev-0's "stderr empty
   on `--json --quiet`" was false for exits 2/3/4. §10.1 now tables all
   15 exit × flag combinations, §9.5 closes the `ExitCodeError.Message`
   catalog, and §10.5 gives the exact `--quiet` line for every outcome.
   Exit 0 is the only case with an empty stderr under `--json --quiet`.
3. **Abort shape rebuilt.** `artifacts` is `[]` **iff** `abort` is present —
   rev-0's four all-`absent` rows claimed an inspection that never happened.
   `feature_state` is `"unknown"` on every abort (never `""`), `overall` is
   fully specified with the schema-constant totals, and `advisories` is `[]`.
   §9.3 guarantees every abort is decided before the first per-artifact
   `Lstat`, which is what makes the two-shape rule total (AVP-127, AVP-128).
4. **Slug validated before any path is composed.** New §7.2 defines a canonical
   grammar (`^[a-z0-9]+(-[a-z0-9]+)*$`, ≤60 bytes) that provably accepts every
   `store.Slugify` output, plus a Windows reserved-device refusal. New abort
   code `slug-unsafe` (exit 3). **No stream ever echoes the raw argument**:
   JSON `slug` is `""`, the human header and the quiet line withhold it.
   Traversal, absolute, control-byte and non-ASCII arguments are all closed by
   one rule (AVP-102…AVP-106).
5. **Race-safe platform capture replaces `Lstat`→ordinary-open.** §7.4
   specifies `O_RDONLY|O_NOFOLLOW|O_NONBLOCK` on unix (no follow, no FIFO
   hang) and a real Windows equivalent (`FILE_FLAG_OPEN_REPARSE_POINT` +
   `GetFileType == FILE_TYPE_DISK`), then post-open `fstat`, `os.SameFile`
   identity, regular-file recheck, size cross-check, bounded read, and a
   post-read `fstat`. AVP-118 asserts the build-tagged sibling exists and that
   no target falls back to a bare `os.Open`.
6. **Bounded read is `io.LimitReader(f, MaxArtifactBytes+1)`.** A growth race
   can never allocate unbounded memory; growth past the cap during the read is
   `unstable`, not `oversize`, and the two are mutually exclusive by
   construction (row 8 pre-open vs row 17 post-open). AVP-112, AVP-116,
   AVP-140.
7. **Ladder rebuilt to 24 total rows.** Adds no-follow-refusal-at-open, fstat
   failure, descriptor kind change, Lstat-vs-fstat size mismatch, growth past
   the cap, byte-count disagreement, post-read fstat failure and post-read size
   change. Seven named instability probes, all with sidecar equivalents
   (AVP-107…AVP-117). §8.3 keeps the honest limits: same-length in-place
   rewrite is undetectable, and a second `fstat` on a held descriptor is a
   tautology.
8. **Advisory selection is a total state → advisory function.** Nine sidecar
   states → at most one `analysis-sidecar-*` advisory. `*-absent-*` fires only
   for `state == absent`; `present-empty` / `invalid-structured` /
   `unstable` / `symlink-refused` / `not-regular` / `unreadable` / `oversize`
   each get a truthful **neutral** message that never claims absence and never
   calls the condition a defect. `analysis-sidecar-unstable` now has real
   production coverage. rev-0's `exploration-absent-verify-requires-later` and
   `optional-artifact-unstable` are removed. AVP-119…AVP-122.
9. **`status.json` populations split three ways.** Absent → **continue** the
   full inspection with `feature_state: "unknown"` + advisory
   `feature-state-absent`, exit from readiness (justified by `ListFeatures`
   already treating such a directory as ordinary). Malformed and unreadable
   stay distinct aborts, because a present-but-broken metadata file is
   `doctor` D1's domain. AVP-123…AVP-126.
10. **Forbidden-field guard scoped to keys and labels.** AVP-051 walks JSON
    **key names** at every nesting level and compares the human surface against
    a closed label set — never a raw substring scan, which would have made
    `oversize`/`artifact-oversize` impossible. AVP-140 exercises an `oversize`
    artifact with the guard green, which is the proof the scoping is real.
11. **`unknown` provenance given a stable definition.** §11.1 fixes it as "no
    trustworthy provenance is available from an accepted source", with three
    consequences: it is not a placeholder, adding known values later is
    additive (no `schema_version: 2`), and legacy artifacts stay `unknown`
    forever rather than being backfilled by guess. The ADR trigger is kept
    **narrow** — it fires on selection of a persistent representation and on
    nothing else. No representation is selected.
12. **`agent-as-provider.md` correction made a required future edit.** §16.1
    row (b) requires the shipped sentence that presents `status.json.notes` as
    what "distinguishes Path B transitions from provider output" to be
    corrected to a *last-transition hint, not durable per-artifact provenance*.
    Claim C67 anchors the current wording.
13. **Refusal remediation made self-contained.** The exit-4 line names only
    `--check` and `tpatch prepare --help`; it cites no PRD, no `docs/` path and
    no URL (AVP-100). §5.3 also states *why* the guard is implementable before
    the store opens.
14. **Test bite added.** Composite differentials that first advance a real
    feature to `analyzed`/`defined` through the real `--manual` command with a
    zero-byte / whitespace-only / symlink artifact, then run the real
    `prepare --check` and assert the honest non-`ready` verdict with zero
    mutation (AVP-130…AVP-133). Routing non-invalidation now uses
    **pre-change-binary goldens** plus reverse call-graph and source guards
    (AVP-134…AVP-137) instead of a before/after no-op comparison. AVP-138
    composes all three headline §1 failure modes in one real CLI run.
15. **Guard sensitivity generalised.** §18.1 adds a disqualifying assertion
    shape for guards; §18.21 extends the sensitivity requirement from three
    hand-picked rows to every row carrying a guard component; AVP-139 is the
    meta-check.

### Unchanged from rev-0 (deliberately)

- Planning-only scope: no Go file, test, asset or CLI surface created or
  modified.
- `--manual` gates stay loose and unchanged (§12), now additionally pinned by
  the composite rows.
- Skill parity still ships the command in all six surfaces — with a new §16.2
  item 5 forbidding it from entering the phase-ordering or preflight sequence,
  so non-goal 8 (no downstream SDD mandate) is asserted, not assumed
  (AVP-092).
- `PRD-prepare-intent-bundle.md` remains blocked and undrafted (§20).
- No ADR created; no persistent provenance representation selected.

### Open decisions left to review (§21, six)

Q1 a later `--all` sweep mode; Q2 whether exit `4` becomes a cross-command
convention; Q3 the `MaxArtifactBytes = 4 MiB` value; Q4 whether `request.md`
becomes a fifth optional row; **Q5 (new)** whether the Windows reserved-device
refusal should be platform-conditional; **Q6 (new)** whether `--format` should
alias `--json` to match `next`. All six have a stated default; none blocks
review.

### Validation performed (docs-only change)

- **Anchor audit**: 161 distinct `file:line` citations across 33 files parsed
  and checked in-range; zero bad. Every newly added anchor was additionally
  content-verified by reading the cited lines (root printer, `exitCodeFor`,
  `readBounded`, `openNoFollow` unix/windows, `Slugify`, `AddFeature`,
  `LoadFeatureStatus` error split, `ListFeatures`, `snapshot-unstable`
  vocabulary, `pathgate` held-descriptor tautology, assets anchor blocks).
- **JSON examples**: both `json` fenced blocks parse; top-level key order
  matches §10.2's declared order in both (the abort example adds `abort`).
- **Matrix mechanics**: 140 rows, contiguous `AVP-001`…`AVP-140`, zero
  duplicates; every `AVP-NNN` referenced in prose resolves to a real row;
  category counts sum to 140; slice assignment is a partition.
- **Claims mechanics**: `C1`…`C75` contiguous, zero duplicates.
- **Markdown hygiene**: 26 fence markers (balanced), 50 tables with zero
  column-count mismatches, zero trailing-whitespace lines, all relative links
  resolve on disk.
- No Go source changed, so `gofmt` / `go build` / `go test` are not applicable
  to this change set; no existing docs test targets these files
  (`internal/workflow/docs_totality_guard_test.go` reads exactly three
  verify-family documents, none of them touched here).

### Reviewer focus for rev-1

1. **§6.2 is the load-bearing change.** The question to test is not "is
   full-bundle readiness nicer" but "does §6.2.2 actually discharge WP-005
   Agreed item 6". The claim is that item 6 constrains the **lifecycle**, and
   that this command is outside the lifecycle in six specific, separately
   asserted ways.
2. **§10.1 must be read against `cobra.go:33-39`, not against `verify.go`
   alone.** `verify` copies the report routing; the `error:` line is the root's
   and applies on top. Any residual claim of an empty stderr on a nonzero exit
   is a defect.
3. **§7.5's ladder is the totality surface.** Check that no reachable
   filesystem condition is missing a row and that rows 8 and 17 really cannot
   both apply.
4. **§10.4 must stay a function of state, not of artifact id.** The rev-0
   defect was an advisory that could contradict its own artifact row.
5. **§9.4's three-way `status.json` split** is a deliberate asymmetry; the
   justification (absent is ordinary per `ListFeatures`, broken is `doctor`'s)
   is the thing to challenge, not the mechanics.
6. **AVP-136/AVP-137 depend on S1 capturing pre-change goldens.** §17 states
   this as a prerequisite; if it is skipped the rows silently degrade back into
   the rev-0 no-op comparison.


## PRD Writer Result — rev-2 (2026-08-13)

### Files changed (exact, complete)

| File | Change |
|---|---|
| `docs/prds/PRD-artifact-validation-and-provenance.md` | rewritten in place; `Status: Draft — Awaiting Review (rev-2)`; byline `rev-2 at HEAD c590f17`; the `Revision history` table gains a rev-2 row and records rev-1's disposition as NEEDS REVISION |
| `docs/handoff/CURRENT.md` | `Cluster state` flip to `AWAITING REVIEW`, Active Task `Status`/`Rev-2 writer base`, and this section |

Nothing else was touched. **No** edit to `docs/ROADMAP.md`,
`docs/supervisor/LOG.md`, `SPEC.md`, `docs/adrs/**`, `docs/whitepapers/**`
(WP-005 was marked Graduated in rev-0 and needed no further change),
`docs/prds/PRD-prepare-intent-bundle.md` (still undrafted, still blocked),
`internal/**`, `cmd/**`, `assets/**`, `tests/**`, `.github/**`,
`.wave-close-allowlist`, or any guarded untracked WIP (`WP-004`, `WP-006`,
`WP-007`, `PRD-recurring-patches.md`, the state-of-the-art case studies).

### Counts (mechanically verified, commit-independent)

- **3,075 lines** in the PRD (rev-1: 2,233; rev-0: 1,478).
- **188 acceptance rows** `AVP-001`…`AVP-188` (rev-1: 140), contiguous, zero
  duplicates, zero retired rows. **48 new rows**; every rev-1 ID kept its
  number.
- **24 categories**: A 10, B 20, C 8, D 14, E 6, F 5, G 9, H 4, I 6, J 4, K 6,
  L 3, M 6, N 5, O 12, P 10, Q 1, R 9, S 2, **T 12, V 17, W 5, X 6, Y 8**.
  Sum = 188. The letter `U` is deliberately skipped as a category so it cannot
  be confused with the `U` (unit) kind.
- **By kind**: `U` 57, `I` 94, `S` 6, `G` 23, `S+G` 3, `U+G` 2, `I+G` 1,
  `S+I` 2. Sum = 188.
- **Guard arithmetic (the rev-1 defect)**: a row carries a guard component
  **iff** its Kind contains `G` → 23 + 3 + 2 + 1 = **29 rows**. AVP-128
  (`S+I`) is now excluded, and AVP-139 derives the set mechanically instead of
  from a hand-list.
- **All 188 rows are assigned to exactly one slice** (S1 69, S2 27, S3 47,
  S4 38, S5 7). Verified as a partition: zero unassigned, zero double-assigned.
- **Claims audit split in two**: **88 repository claims** `C1`…`C88`
  (rev-1: 75), contiguous, anchored `file:line`; plus a new **§23.2 with 12
  Go-standard-library claims** `G1`…`G12`, anchored **by symbol** in
  `$GOROOT/src/os/...` rather than by line, each naming the acceptance row that
  verifies it at runtime.
- **159 distinct repository `file:line` anchors** across 35 files; all
  mechanically verified in-range, zero out-of-range, zero missing files.

### What rev-2 changed, finding by finding

1. **Rooted namespace (§7.3, §7.4).** One `os.OpenRoot(repoRoot)` per run,
   held for the whole inspection, passed to `Inspect`, closed once by the CLI.
   Every `Lstat` is `Root.Lstat`, every open is `Root.OpenFile`, every
   descriptor stat is `(*os.File).Stat`. `filepath.Join(repoRoot, …)`,
   `os.Stat`, `os.Lstat`, `os.Open`, `os.OpenFile` and `os.ReadFile` are all
   forbidden and source-scanned (AVP-089, AVP-144, AVP-150).
2. **Platform scope stated honestly (§7.4.1).** A three-row table over the Go
   build-tag split: `unix||wasip1` and `windows` are confined; `(js&&wasm)||plan9`
   is **not**, per the `Root` doc comment. A build-tagged
   `rootConfinementSupported` constant makes the unsupported targets abort
   `workspace-unsupported-platform` before `os.OpenRoot` is called
   (AVP-177, AVP-179).
3. **Component policy widened to reparse points.** The refusal predicate is
   `Mode()&(ModeSymlink|ModeIrregular)`, because Go maps a Windows **junction**
   (`IO_REPARSE_TAG_MOUNT_POINT`) to `ModeIrregular`, not `ModeSymlink`. A
   sensitivity fixture that tests only `ModeSymlink` lets the junction through
   and fails (AVP-146).
4. **The no-follow claim is dropped, and the race behavior is stated exactly
   (§7.4.2, §7.4.4).** `os.Root` **follows** in-root symlinks; it is a
   confinement primitive, not a no-follow primitive. On Unix `Root.OpenFile`
   already ORs `O_NOFOLLOW` internally and converts the `ELOOP` into an in-root
   *resolution*, so a caller cannot obtain a final-leaf refusal from it — the
   PRD says so and **no acceptance row claims otherwise**. What is claimed is a
   three-row table: an out-of-root raced link → `Root` refuses → `unreadable`,
   zero bytes; an in-root raced link to a *different* object → identity
   mismatch → `unstable`, **zero bytes**; an in-root raced link to the *same*
   inode → read proceeds, and the same-identity alias is explicitly **not**
   claimed detectable. AVP-148, AVP-149, AVP-151, plus AVP-152, a mechanical
   over-claim guard over every shipped string.
5. **Windows contract rebuilt (§7.4.3).** rev-1's raw `syscall.CreateFile` with
   `FILE_FLAG_OPEN_REPARSE_POINT` was self-contradictory — the flag makes the
   open *succeed* while the ladder classified from an open *error*. rev-2 has
   no raw syscall at all: reparse points are refused pre-open by `Root.Lstat`
   (whose Windows implementation is itself an `OPEN_REPARSE_POINT` handle open
   plus a handle-derived stat), and pipes/char devices are refused by
   `statHandle`'s `GetFileType`-derived modes at ladder rows 7 and 14.
   `openFlags()` returns `0` on Windows and `O_NOFOLLOW|O_NONBLOCK` on Unix
   (AVP-118, AVP-180).
6. **`os.SameFile`-on-a-pathname removed.** Both sides of the identity check
   are handle-derived on both platform classes. On Windows,
   `newFileStatFromGetFileInformationByHandle` clears the struct's `path` field
   specifically so `os.SameFile` will not re-fetch by pathname, and
   `(*File).Stat` is `statHandle(name, handle)` — cited as G7/G8 and asserted
   natively by AVP-176.
7. **Native Windows CI is an acceptance obligation, not an assumption.** The CI
   matrix is Linux + macOS today (C84). §16.1 makes adding `windows-latest` a
   **required** file change, §17 lands it in **S1** alongside the Windows code,
   AVP-175 parses the workflow and fails if it is absent, and AVP-176 is the
   native behavioral row. R7's severity was raised to High.
8. **Fixed-buffer read (§7.4.5).** rev-1's `io.ReadAll(io.LimitReader(f,
   Max+1))` "exact allocation ceiling" claim was **false** — `io.ReadAll` grows
   by `append`. rev-2 uses one `make([]byte, MaxArtifactBytes+1)` plus
   `io.ReadFull`, with a total four-way EOF table (`io.EOF` → empty;
   `ErrUnexpectedEOF` → captured `buf[:n]`; `nil` → grew past the cap →
   `unstable`; anything else → `unreadable`). An allocation-counting fixture
   asserts exactly one allocation of exactly `Max+1` bytes per capture
   (AVP-170…AVP-174); `io.ReadAll`, `io.LimitReader` and `os.ReadFile` are
   source-forbidden with a sensitivity fixture that reintroduces the rev-1 form.
9. **`status.json` (§9.4).** `store.LoadFeatureStatus` — `os.ReadFile` on an
   absolute pathname, symlink-following, unbounded, no kind or identity check —
   is **forbidden** (AVP-150), and the rev-1 `StateReader` seam is deleted from
   §7.1. The status file gets the full rooted discipline, its **own**
   `MaxStatusBytes = 1 MiB` cap (justified, and separate so widening Q3 cannot
   silently widen it), a 19-row first-match ladder, and a **nine-value** outcome
   enum. Two outcomes continue (`ok`, `absent`); seven abort.
10. **Closed abort catalog grown to thirteen codes (§9.4.4)** with **exact
    message templates (§9.4.5)**: `slug-unsafe`,
    `workspace-unsupported-platform`, `workspace-not-initialized`,
    `workspace-root-unopenable`, `feature-dir-unsafe`, `feature-not-found`,
    `status-symlink-refused`, `status-not-regular`, `status-oversize`,
    `status-unreadable`, `status-unstable`, `status-malformed`,
    `status-invalid-state`. Bijection code↔message asserted; no template wraps
    an `os` error or carries an absolute path (AVP-181).
11. **`FeatureState` validated before echo (§9.4.2 row 18).** The inspector
    carries its own closed twelve-value list with a two-way AST parity guard
    against `store` (AVP-165); a parsed-but-unrecognised state aborts
    `status-invalid-state` and **the offending value is never echoed**
    (AVP-164, AVP-185).
12. **Lifecycle line made truthful per population (§10.5.1).** A fifteen-row
    table (status `ok`, status `absent`, thirteen aborts). rev-1 printed
    `(status.json was not read)` on every abort, which is false for
    `status-malformed`, `status-invalid-state`, `status-unreadable` and
    `status-unstable`. AVP-154 asserts those four do **not** contain the
    substring `was not read`; AVP-153 is the totality guard.
13. **`--path` exit ownership corrected (§9.2).** `--path` is a persistent
    **string** flag (`internal/cli/cobra.go:66`); pflag validates nothing, so
    the failure surfaces inside `RunE` at `store.FindProjectRoot`
    (`internal/store/store.go:23-40`) — the actual trigger — and is bound to
    abort `workspace-not-initialized`, **exit 3**, not cobra exit 1. The
    genuine exit-1 population is unknown flags, wrong arity, and `--path` with
    no value (AVP-183, AVP-184). `store.Open` is documented as **not** the
    trigger and is never called.
14. **Quiet abort distinguishes codes.** All thirteen `--quiet` abort lines are
    pairwise distinct and carry the same closed token as `abort.code` and the
    `error:` line; the bare `— indeterminate` form is reserved for the one
    non-abort indeterminate case (AVP-098, AVP-184).
15. **"Printable ASCII" replaced (§14.3).** That claim was wrong on the
    command's own happy path — the `—` in the quiet line and the `→`
    remediation marker are required non-ASCII house style. The rule is now: no
    ASCII control byte other than the renderer's own `0x0A` (so no `0x1B`, no
    tab, no CR), no attacker-argument bytes, valid UTF-8, house style preserved
    (AVP-187).
16. **`slug-unsafe` remediation de-looped (§7.2).** rev-1 said "Run `tpatch
    status` to list valid slugs" — but `ListFeatures` applies no canonicality
    filter (C16), so `status` prints the same refused name back. The message
    now names `tpatch add` and the rename path only. §13.3 splits the
    hand-assembled population into a canonically-named row (fully inspectable,
    the only claim made) and a non-canonically-named row (AVP-186).
17. **Skill wording for exit 2 (§16.2 item 6).** Verbatim required paragraph in
    all six surfaces stating exit 2 is an expected report outcome, not a
    workflow or system failure; it may not be called an error, a failure or a
    blocker. AVP-188, with a sensitivity fixture on "fails with exit 2". New
    risk R12.
18. **Source claims corrected.** C60 now records that
    `internal/rescap/pathopen_windows.go` is an **unsupported compile-only
    stub** (bare `os.OpenFile`, `isSymlinkLoopError` always false) — precedent
    for the problem, not a reusable implementation. C61/C82 record that
    `rescap.readBounded` uses a growable `append` buffer. C28's stale
    `types.go:207-215` anchor for the `DependsOn` doc comment is corrected to
    `:219-234`. §4's preflight gains rows for `store.LoadFeatureStatus` and
    `os.Root`.
19. **New ladder rows and probes.** The per-artifact ladder stays 24 rows but
    is rebuilt on `Root`; row 10 (out-of-root escape refusal → `unreadable`) is
    stated separately from row 11 for honesty even though both share a state,
    because `errPathEscapes` is unexported and undiscriminable (G11). The seven
    instability probes move to rows 9, 13, 14, 15, 17, 18, 20.
20. **New alternatives recorded (§22).** Seven new rejected-alternative rows,
    each naming a rev-1 behavior and why it is gone:
    `io.ReadAll(LimitReader)`, pathname-walk resolution, the raw `CreateFile`,
    `os.SameFile`-on-a-pathname, `store.LoadFeatureStatus`, grouping the seven
    status aborts into one code, and the printable-ASCII assertion.

### Unchanged from rev-1 (deliberately, as instructed)

- **No mutating `prepare`**; plain `prepare <slug>` still refuses with exit 4
  before anything runs.
- **No lifecycle state**, no new `FeatureState`, no transition, no write.
- **Individual `--manual` / `next` / `cycle` unchanged** (§12, §13.2), still
  pinned by AVP-064…AVP-069 and the composite rows AVP-130…AVP-133.
- **`provenance` is the constant `unknown`** with §11.1's stable "not provable"
  meaning; the seven forbidden inference sources are unchanged.
- **No persistent provenance representation selected; no ADR created.** The
  §11.4 trigger remains narrow and unexercised.
- **`PRD-prepare-intent-bundle.md` remains blocked and undrafted** (§20).
- Full-bundle readiness (§6.2), the advisory total function (§10.4), the
  two-shape `artifacts` iff-`abort` rule, the composite differentials, the
  reverse call-graph routing guards, the sidecar instability rows and the
  forbidden-field key-name scoping all carry over intact.

### Open decisions left to review (§21, eight)

Q1 `--all`; Q2 whether exit `4` becomes a cross-command convention; Q3
`MaxArtifactBytes = 4 MiB`; Q4 `request.md` as a fifth optional row; Q5
platform-conditional Windows reserved-device refusal; Q6 `--format` aliasing;
**Q7 (new)** `MaxStatusBytes = 1 MiB` and whether it stays separate from
`MaxArtifactBytes`; **Q8 (new)** whether `workspace-unsupported-platform`
should be a compile-time refusal instead of a runtime abort. All eight have a
stated default; none blocks review.

### Implementation status

**None.** Planning only. No Go file, test, asset, workflow or CLI surface was
created or modified. The PRD defines five ordered slices (S1–S5); S1 now also
carries the `windows-latest` CI matrix row and the pre-change routing goldens.

### Validation performed (docs-only change)

- **Repository anchors**: 159 distinct `file:line` citations across 35 files
  parsed and checked in-range; zero bad. Every new or changed anchor was
  additionally content-verified by reading the cited lines — `FindProjectRoot`,
  `ListFeatures`, `ValidFeatureState`, the `FeatureState` const block, the
  `--path` registration, `LoadFeatureStatus`, `SaveFeatureStatus`,
  `AddFeature`, `Notes`, `DependsOn`, the CI matrix and step block, `go.mod`'s
  Go version, the `rescap` Windows stub, `readBounded`, and
  `pathgate.SamePathIdentity`.
- **Go stdlib claims**: read directly from the pinned toolchain's `GOROOT`
  (`go1.26.5`) — `os/root.go`, `root_openat.go`, `root_unix.go`,
  `root_windows.go`, `root_noopenat.go`, `stat_windows.go`, `stat_unix.go`,
  `types_windows.go`, `types_unix.go`, `file.go`. Cited **by symbol**, never by
  line, and every one is additionally backed by a runtime acceptance row.
- **Matrix mechanics**: 188 rows, contiguous `AVP-001`…`AVP-188`, zero
  duplicates; every `AVP-NNN` referenced in prose resolves to a real row;
  category counts sum to 188; kind counts sum to 188; the slice assignment is a
  verified partition of 1…188.
- **Claims mechanics**: `C1`…`C88` and `G1`…`G12` both contiguous, zero
  duplicates.
- **Markdown hygiene**: 30 fence markers (balanced); every table's data rows
  match its header column count (checked with `\|`-escape awareness); zero
  trailing-whitespace lines; all 13 relative links resolve on disk; both `json`
  fenced blocks parse.
- **Cross-reference closure**: every `§N.N` reference in the document resolves
  to a heading that exists.
- No Go source changed, so `gofmt` / `go build` / `go test` are not applicable
  to this change set; no existing docs test targets these files
  (`internal/workflow/docs_totality_guard_test.go` reads exactly three
  verify-family documents, none of them touched here).

### Reviewer focus for rev-2

1. **§7.4.2 and §7.4.4 are the load-bearing honesty sections.** The question to
   test is not "is `os.Root` safer" but "does the PRD claim exactly what
   `os.Root` provides and nothing more". Specifically: it claims **confinement**
   and **no substitution**, and it explicitly disclaims final-leaf no-follow
   and same-identity-alias detection. AVP-152 is the mechanical guard against
   any shipped string drifting back into the stronger claim.
2. **The Unix `O_NOFOLLOW` flag is deliberately described as inert.**
   `rootOpenFileNolog` already sets it and `doInRoot` converts the signal into
   an in-root resolution. If a reviewer finds any row or sentence that treats
   it as a refusal mechanism, that is a defect.
3. **§9.4.2's status ladder must be checked for totality against §9.4.3's nine
   outcomes and §9.4.4's thirteen codes.** The three tables have to agree, and
   AVP-168 is the guard that says so.
4. **§10.5.1 is the fix for the falsest sentence in rev-1.** Check that no
   annotation says "was not read" for a population where a read happened.
5. **§16.1's CI row is a hard requirement, not advice.** If the reviewer thinks
   `GOOS=windows` cross-building is sufficient, that disagreement should be
   raised now — the whole Windows half of §7.4.3 rests on runtime behavior a
   cross-build cannot execute.
6. **§18.26's guard arithmetic is stated as a predicate.** Recount it: 23 pure
   `G` + 3 `S+G` + 2 `U+G` + 1 `I+G` = 29, and AVP-128 (`S+I`) is out. If the
   count does not reproduce, the section is wrong.
7. **The claims audit is now two tables with two anchoring conventions**
   (§23.1 `file:line`, §23.2 by symbol). The by-symbol choice is deliberate —
   toolchain line numbers drift across patch releases — and each G-claim names
   its runtime verifier so the citation is not the only evidence.

## PRD Writer Result — rev-3 + ADR-034 rev-0 (2026-08-13)

### Files changed (exact, complete)

| File | Change |
|---|---|
| `docs/prds/PRD-artifact-validation-and-provenance.md` | edited in place; `Status: Draft — Awaiting Review (rev-3)`; byline `rev-3 at HEAD 5a678b5`; new `Architecture` header line linking ADR-034; `Related` gains an ADR-034 bullet; `Revision history` gains a rev-3 row and records rev-2's disposition as NEEDS REVISION. **3,830 lines** (rev-2: 3,075). |
| `docs/adrs/ADR-034-rooted-filesystem-inspection-boundary.md` | **new**, 635 lines, `Status: Proposed — Awaiting Review (rev-0)`, fourteen decisions D1–D14 |
| `docs/adrs/README.md` | one index line added for ADR-034 (Proposed — Awaiting Review), nothing else touched |
| `docs/handoff/CURRENT.md` | `Cluster state` flip to `AWAITING REVIEW`, Active Task `Status`/`Rev-3 writer base`/`Scope`, and this section |

Nothing else was touched. **No** edit to `docs/ROADMAP.md`,
`docs/supervisor/LOG.md`, `SPEC.md`, `docs/whitepapers/**` (WP-005 was marked
Graduated in rev-0 and needed no further change),
`docs/prds/PRD-prepare-intent-bundle.md` (still undrafted, still blocked), any
other `docs/adrs/ADR-0*.md`, `internal/**`, `cmd/**`, `assets/**`, `tests/**`,
`.github/**`, `.wave-close-allowlist`, or any guarded untracked WIP (`WP-004`,
`WP-006`, `WP-007`, `PRD-recurring-patches.md`, the state-of-the-art case
studies).

### Counts (mechanically verified, commit-independent)

- **PRD 3,830 lines** (rev-2: 3,075; rev-1: 2,233; rev-0: 1,478);
  **ADR 635 lines**.
- **202 acceptance rows** `AVP-001`…`AVP-202` (rev-2: 188), contiguous, zero
  duplicates, zero retired rows. **14 new rows**; every rev-2 ID kept its
  number, and the rows whose meaning changed (`AVP-083`, `AVP-108`, `AVP-116`,
  `AVP-117`, `AVP-118`, `AVP-139`, `AVP-144`, `AVP-146`, `AVP-152`, `AVP-167`,
  `AVP-170`, `AVP-176`, `AVP-177`, `AVP-182`, `AVP-184`, `AVP-187`) were
  amended in place per §18.1's no-renumbering rule.
- **25 categories**: A 10, B 20, C 8, D 14, E 6, F 5, G 9, H 4, I 6, J 4, K 6,
  L 3, M 6, N 5, O 12, P 10, Q 1, R 9, S 2, T 12, V 17, W 5, X 6, Y 8,
  **Z 14**. Sum = 202, verified by parsing the section headings rather than by
  hand.
- **By kind**: `U` 59, `I` 96, `S` 6, `G` 30, `S+G` 4, `U+G` 4, `I+G` 1,
  `S+I` 2. Sum = 202.
- **Guard arithmetic**: Kind contains `G` → 30 + 4 + 4 + 1 = **39 rows**
  (rev-2: 29). Complement 59 + 96 + 6 + 2 = 163; 39 + 163 = 202.
- **Slice partition verified programmatically**: S1 78, S2 28, S3 49, S4 38,
  S5 9 = 202; zero unassigned, zero double-assigned.
- **Claims audit**: **92 repository claims** `C1`…`C92` (rev-2: 88) and
  **21 Go-stdlib claims** `G1`…`G19` plus `G5a`/`G9a` (rev-2: 12), now split
  into **8 `contract`** and **13 `tripwire`** rows.
- **164 distinct repository `file:line` anchors** across 36 files; all
  mechanically verified in-range, zero out-of-range, zero missing files.
- **ADR-034**: 14 decisions, 10 rejected alternatives, a 14-row
  decision→acceptance-row dependency table whose every `AVP` reference resolves
  in the PRD.

### What rev-3 changed, finding by finding

**os.Root scope / ADR-034**

1. **ADR-034 created** (`Proposed — Awaiting Review`, rev-0) and linked from the
   PRD header, `Related` list and eleven inline decision points. The ADR is
   declared normative where the two overlap. Its four alternatives are the ones
   the review named: keep the `rescap` pathname `GatePath`; hand-roll
   `openat`/`NtCreateFile`; `os.Root`; platform refusal — with `os.Root` chosen
   for this read-only inspector and the reasoning stated in full.
2. **Logical, not physical, confinement (§7.4.2, ADR D2).** The `Root` doc
   comment's own sentence — "Methods on `Root` do not prohibit traversal of
   filesystem boundaries, Linux bind mounts, /proc special files, or access to
   Unix device files" — is now quoted verbatim (G13) and is the basis of the
   scope statement. Three rev-2 sentences are **prohibited by name**, including
   "no path outside the repository is ever opened, read, or named". A table
   states what actually closes each leaf shape: the kind gate and the bounded
   read, not the root. The PRD explicitly does **not** claim the bytes it reads
   physically originate inside the repository's filesystem.
3. **Workspace discovery declared outside the rooted capture (§7.3 step 1,
   §9.3 step 5, §14.1, ADR D3).** rev-2's "every filesystem operation it
   performs goes through one `*os.Root`" is corrected to "every filesystem
   operation **after workspace discovery**". `FindProjectRoot` is an ordinary
   symlink-following pathname walk that runs before `os.OpenRoot` — it produces
   the directory the root is opened on — and `--path` is stated as trusted
   input.
4. **`fs.ValidPath` replaces `EnsureSafeRepoPath` entirely (§7.3 step 2, ADR
   D4).** The §4 preflight row, the §14.1 summary and a new §22 alternatives
   row all record *why*: it is a lexical test against an absolute repository
   prefix, applied to a root-relative name in a handle-based design, and under
   `--path` from an unrelated cwd there is no prefix for it to test. New claim
   C91 anchors the signature. AVP-144 asserts both the replacement and the
   removal.
5. **Fail-closed platform allowlist (§7.4.1, ADR D5).** `//go:build unix ||
   windows || wasip1` → `true`, its exact negation → `false`. The `true`
   expression is byte-identical to `$GOROOT/src/os/root_openat.go`'s own tag
   and AVP-191 compares the two texts. `js/wasm`, `plan9` **and any future
   unmatched `GOOS`** abort `workspace-unsupported-platform`. The rev-2
   denylist is recorded as a rejected alternative with the fail-open failure
   mode spelled out.
6. **Slug validated before the platform check (§7.2, §9.3, ADR D5).**
   Precedence is now 1 cobra, 2 reserved-surface, **3 slug**, **4 platform**,
   5 discovery, 6 root open, 7 feature walk, 8 status, 9 instability,
   10 shortfall, 11 zero — which is what keeps §10.2 rule 8's "`slug` is `""`
   **iff** `slug-unsafe`" biconditional total on an unsupported target.
   AVP-192.
7. **rescap migration locked OUT OF SCOPE (ADR D11).** New claim C92; a new §22
   alternatives row; the ADR states that a wave which "helpfully" migrates
   `rescap` has exceeded scope.

**Symlink / identity / Windows**

8. **Pre- *and* post-component walks per capture (§7.4.4 steps 1 and 11, ladder
   rows 20a/20b, ADR D8).** A component observed as changed after the read
   makes the artifact `unstable`, bytes discarded, no content state — even when
   the leaf identity matched. Rows are lettered, not renumbered, so every rev-2
   row-number citation still resolves. AVP-195.
9. **Identity promise weakened to "an object *observed as different* is never
   read" (§7.4.4, §8.3, ADR D8).** Five divergences between "different" and
   "observed as different" are enumerated: same-length rewrite, same-identity
   alias, **hard-link alias** (undetectable by construction), **inode/NTFS
   file-ID reuse**, and **swap-and-restore between probes** — plus the
   walk→`Lstat`→open window, stated as a genuine unclosed TOCTOU residue.
   §8.3 now lists six limits (rev-2: three). AVP-196 pins three of them as
   *limits*, not capabilities.
10. **Windows reparse mapping corrected (§7.4.3, G9/G9a, ADR D7).** rev-2's
    "every other reparse tag sets `ModeIrregular`" was false: `AF_UNIX` sets
    **`ModeSocket`** and `DEDUP` sets **no type bit at all** (Go deliberately
    treats it as regular, with an explanatory comment). A five-row tag table
    states symlink → `ModeSymlink`; junction/`MOUNT_POINT` → name surrogate,
    `ModeDir` and `GetFileType` bits suppressed, `ModeIrregular` via `default`;
    other tags → `ModeIrregular` **only** in the default branch. The
    `ModeSymlink|ModeIrregular` predicate is restated as a *refusal* predicate
    that is necessary and **not sufficient**, backed by the `!IsRegular()` kind
    gate. AVP-198.
11. **`//go:debug winsymlink=1` pinned in `package main`** (`cmd/tpatch/main.go`,
    new claim C89) as a required implementation and acceptance obligation, with
    both a source half and a native behavioral half in AVP-198, plus the honest
    limit that the `GODEBUG` env var still overrides a `//go:debug` default.
    G14 and G15 anchor the fallback path and the setting's history.
12. **Identity mechanism labelled a Go-version tripwire (§7.4.4, ADR D10).**
    `Root.Lstat` + opened `File.Stat` + `os.SameFile` under the Go 1.26 pin;
    G7/G8/G16 are **tripwire**-classified; AVP-176 is described as an
    upgrade tripwire, not an eternal public guarantee. No ordinary `os.Lstat`
    or pathname reopening appears anywhere in the design.
13. **`O_NOFOLLOW` removed from the caller's flags (§7.4.3, ADR D6).** rev-2
    passed it "belt-and-braces" while documenting that `Root` consumes it —
    a false affordance. `openFlags()` is now exactly `syscall.O_NONBLOCK` on
    non-Windows and exactly `0` on Windows; AVP-118 asserts the flag set and
    fails if `O_NOFOLLOW` returns. A stable FIFO is refused pre-open at ladder
    row 7; a raced FIFO returns without hanging under `O_NONBLOCK` and is
    caught by the row-14 kind recheck.
14. **`O_NONBLOCK` pass-through recorded as an implementation tripwire**
    (G17, ADR D10) with **AVP-200**, a real-FIFO-under-deadline Go-upgrade
    test whose stated job is to go red at upgrade time rather than let the
    command hang in the field.
15. **Native `windows-latest` mandatory; junctions must FAIL not SKIP
    (§16.1, ADR D13, AVP-199).** `cmd /c mklink /J` in test code (no privilege
    or Developer Mode needed, unlike `mklink /D`), `t.Fatal` on unavailability,
    and a source half forbidding `t.Skip`/`t.Skipf`/`t.SkipNow` in the fixture
    path — the only permitted guard is `runtime.GOOS != "windows"`. The CI
    change remains a future implementation obligation of S1.

**Fixed buffer / test seams**

16. **One reused scratch buffer per invocation (§7.4.5, ADR D9, AVP-197).**
    rev-2's one-buffer-*per-capture* had a ~20 MiB worst case it never
    totalled. rev-3 allocates exactly one `MaxArtifactBytes+1` array in the CLI
    layer and reuses it sequentially for the status capture and all four
    artifacts; the status capture takes `scratch[:MaxStatusBytes+1]`, a
    sub-slice of the same array, which makes `MaxStatusBytes <
    MaxArtifactBytes` a compile-time-asserted structural invariant. The cost is
    stated honestly as a flat **4,194,305 bytes for every invocation**,
    including aborts. `io.ReadFull` EOF semantics are unchanged and total.
    Q9 records lazy allocation as the revisable alternative.
17. **Cap↔message coupling (§7.4.5, §9.4.2, AVP-201).** `4 MiB` and `1 MiB`
    appear verbatim in frozen messages; a guard derives each unit string from
    its constant and fails in both directions.
18. **Injected operation seam defined (§7.1.1, ADR D12).** `RootOps` (2
    methods) + `FileOps` (3 methods) wrapping `Lstat`/`OpenFile` and
    `Stat`/`Read`/`Close`, with deterministic `before`/`after` hooks available
    to test implementations only. **Exactly one non-test implementation** — the
    `*os.Root`/`*os.File` adapter — asserted by AVP-194 with a sensitivity
    fixture. The source guards explicitly allow the seam and still ban
    alternate pathname readers across production *and* test files. This is what
    makes every injected `fstat`/read/race row implementable; rev-2 specified
    two dozen rows that could not be written against a concrete `*os.Root`.
    Because `RootOps` exposes no mutator and no pathname-taking method, "cannot
    mutate, cannot enumerate" becomes a type-level property.

**Status / output / CLI**

19. **Status stays under the same boundary** — same root, same pre/post
    component walks, same `openFlags()`, same identity/kind/size rechecks, same
    shared buffer, its own cap, same valid-state gate before any echo. Stated
    in §9.4.2 and locked by ADR D1 and the ADR's closing paragraph. All nine
    status populations, the thirteen abort codes, the fifteen lifecycle
    annotations and the closed abort-message templates remain total.
20. **Every drifted citation corrected**, including the message-catalog
    references that pointed at the read-primitive guard (`AVP-172` → `AVP-181`
    in four places), the status-echo reference (`AVP-161` → `AVP-166`), the
    reparse-component reference (`AVP-166` → `AVP-146`), the escape-landing
    reference (`AVP-157` → `AVP-149`), the runtime-spy reference (`AVP-160` →
    `AVP-089`/`AVP-149`), the cap references (`AVP-162`/`AVP-163` →
    `AVP-157`/`AVP-158`/`AVP-174`), the identity-soundness and same-identity
    references (`AVP-167`/`AVP-155…158`/`AVP-159` → `AVP-148`/`AVP-149`/
    `AVP-151`/`AVP-152`/`AVP-176`), the forbidden-primitive reference
    (`AVP-173` → `AVP-172`), and the `rescap` reference (`AVP-172` →
    `AVP-180`). `AVP-083`'s ladder row is corrected from 10 to 9. `AVP-167`,
    which duplicated `AVP-124`, is amended in place into the human-surface half.
21. **`--path` populations unchanged and still correctly anchored**: malformed
    or missing `--path` **value** is cobra exit 1; workspace discovery failure
    is `RunE` exit 3 with `workspace-not-initialized` (AVP-183, AVP-184).
22. **Attacker-byte guarantees scoped to command-owned output (§14.3.1,
    AVP-193).** rev-2's unqualified "no byte of stdout or stderr is a control
    character" swept in cobra's and pflag's own parse diagnostics, which are
    produced before `RunE` from raw `os.Args` and interpolate the offending
    argument verbatim. §14.3.1 enumerates what the command owns, states that
    unknown-flag and arity errors are **outside the schema and not claimed
    sanitized**, and records that a hostile slug with a well-formed flag set
    still reaches `slug-unsafe` and is still withheld. AVP-193's sensitivity
    fixture fails if a future implementation intercepts and re-renders cobra's
    error.
23. **Canonical hand-assembled scope, loop-free `slug-unsafe` remediation and
    the skill exit-2 paragraph** are carried forward unchanged from rev-2, with
    a new §16.2 item 7 forbidding any skill surface from restating an
    `os.Root` guarantee (AVP-189).

**Matrix / citations**

24. **AVP-202, the citation-and-arithmetic guard.** Asserts every prose
    `AVP-NNN` resolves to a declared row; contiguity `AVP-001`…`AVP-202`; that
    each category's stated count equals the rows under that heading; that each
    kind's count equals the rows with that exact Kind string; that the guard
    predicate reproduces the stated guard count; and that §17's slice
    assignment is a partition. Four sensitivity fixtures. **Its doc comment
    states what it cannot prove**: resolution and arithmetic are mechanical,
    semantic mapping of a citation to a sentence is not, and remains a reviewer
    obligation.
25. **Guard arithmetic restated as reproducible arithmetic**: 30 + 4 + 4 + 1 =
    39 guard rows; complement 59 + 96 + 6 + 2 = 163; 39 + 163 = 202.
26. **Nine new §22 rejected-alternative rows**, each naming a rev-2 behavior
    and why it is gone: keeping `EnsureSafeRepoPath`; the platform denylist;
    caller-side `O_NOFOLLOW`; process-wide byte assertions; per-capture
    buffers; "a different object is never read"; the single ancestor walk;
    concrete-`*os.Root` testing; and migrating `rescap`.

### Unchanged from rev-2 (deliberately, as instructed)

- **Read-only check with full three-doc readiness** (§6.2): `ready` requires
  `analysis.md` + `spec.md` + `exploration.md`; the sidecar stays optional and
  can never affect readiness.
- **No mutation, no new state**: no mutating `prepare` (exit 4 refusal), no new
  `FeatureState`, no transition, no write on any path.
- **Loose `--manual`, `next` and `cycle` unchanged** (§12, §13.2), still pinned
  by AVP-064…AVP-069 and the composite rows AVP-130…AVP-133.
- **`provenance` is the stable constant `unknown`** with §11.1's "not provable"
  meaning and the seven forbidden inference sources.
- **No provenance ADR trigger fired.** ADR-034 D14 states this explicitly and
  forbids citing ADR-034 as provenance precedent: it is a filesystem *access*
  boundary and says nothing about what may be persisted about authorship.
- **`PRD-prepare-intent-bundle.md` remains blocked and undrafted** (§20).

### Open decisions left to review (§21, ten)

Q1 `--all`; Q2 exit `4` as a cross-command convention; Q3
`MaxArtifactBytes = 4 MiB`; Q4 `request.md` as a fifth row; Q5
platform-conditional Windows reserved-device refusal; Q6 `--format` aliasing;
Q7 `MaxStatusBytes = 1 MiB` and its separateness; Q8 compile-time vs runtime
platform refusal (narrowed by rev-3's allowlist); **Q9 (new)** lazy vs
unconditional scratch allocation; **Q10 (new)** whether the `RootOps`/`FileOps`
seam should be unexported. All ten have a stated default; none blocks review.

### Implementation status

**None.** Planning only. No Go file, test, asset, workflow or CLI surface was
created or modified. `cmd/tpatch/main.go` and `.github/workflows/ci.yml` are
named as **required future** edits (§16.1) and were **not** touched.

### Validation performed (docs-only change)

- **AVP mechanics**: 202 declared rows, contiguous `AVP-001`…`AVP-202`, zero
  duplicates; **every** `AVP-NNN` token in prose resolves to a declared row
  (zero unresolved); category counts parsed from the section headings sum to
  202 and match the §18.27 table exactly; kind counts parsed from the Kind
  column sum to 202 and match; the guard predicate yields 39; **§17's slice
  assignment was expanded programmatically and verified as a partition of
  1…202** (S1 78, S2 28, S3 49, S4 38, S5 9 — zero duplicates, zero missing).
- **Citation semantics**: the full prose→row citation list was dumped with each
  cited row's actual subject and reviewed line by line; every mismatch found is
  listed in item 20 above and corrected.
- **Repository anchors**: 164 distinct `file:line` citations across 36 files
  parsed and checked in-range; zero bad. The four new/changed ones were
  content-verified by reading the cited lines — `safety.EnsureSafeRepoPath`'s
  signature and prefix test (C91), `cmd/tpatch/main.go`'s eleven directive-free
  lines (C89), the CI matrix block (C90), and the `rescap` gate/content
  citations (C92).
- **Go stdlib claims**: re-read from the pinned toolchain's `GOROOT`
  (`go1.26.5`) — `os/root.go` (incl. the filesystem-boundary paragraph and
  `rootMaxSymlinks`), `root_openat.go`/`root_noopenat.go`/`root_unix.go`/
  `root_windows.go` build tags, `root_unix.go` `rootOpenFileNolog`/
  `checkSymlink`, `root_windows.go` `rootStat`, `stat_windows.go` `statHandle`
  and `(*File).Stat`, `types_windows.go` `mode`/`Mode`/`modePreGo1_23`/
  `isReparseTagNameSurrogate`/`newFileStatFromGetFileInformationByHandle`/
  `sameFile`/`loadFileId`, `file.go` `errPathEscapes`, `io/fs` `ValidPath`,
  and `internal/godebugs/table.go`'s `winsymlink` row. Cited **by symbol**, and
  each row is now classified `contract` or `tripwire`.
- **Claims mechanics**: `C1`…`C92` contiguous, zero duplicates;
  `G1`…`G19` + `G5a` + `G9a` = 21 rows, 8 contract + 13 tripwire = 21.
- **Cross-reference closure**: 129 numbered headings; **every** `§N.N`
  reference in the PRD resolves to a heading that exists (zero unresolved).
- **ADR↔PRD parity**: every `AVP` reference in ADR-034's decision→verification
  table resolves to a declared PRD row; ADR-034 declares D1–D14 with no gaps;
  every decision the review required is present (decision/context, logical
  confinement, platform allowlist, observed-symlink refusal and race limits,
  identity limits and Go upgrade tripwires, one reused scratch buffer,
  status+artifacts same boundary, `winsymlink`/native-Windows policy, no
  `rescap` migration, no provenance persistence, consequences, rejected
  alternatives, acceptance dependencies).
- **Markdown hygiene**: PRD 40 fence markers (balanced), 68 tables with zero
  column-count mismatches; ADR 2 fence markers (balanced), 5 tables with zero
  mismatches; zero trailing-whitespace lines in either; every relative link in
  both resolves on disk; both `json` fenced blocks still parse.
- No Go source changed, so `gofmt` / `go build` / `go test` are not applicable
  to this change set; no existing docs test targets these files
  (`internal/workflow/docs_totality_guard_test.go` reads exactly three
  verify-family documents, none of them touched here).

### Reviewer focus for rev-3

1. **§7.4.2 and ADR-034 D2 are the load-bearing honesty pair.** The question is
   not "is `os.Root` safer" but "does the pair claim exactly logical pathname
   confinement and nothing more". The quoted doc-comment paragraph is the
   evidence; AVP-189 is the mechanical guard, and it deliberately scans the ADR
   and the PRD as well as the shipped strings, because the over-claim rev-2
   shipped was in the *documents* first.
2. **§7.4.1's allowlist should be checked as *text*, not as intent.** The
   `true` tag must be byte-identical to `$GOROOT/src/os/root_openat.go`'s tag
   and the `false` tag its exact negation. If a reviewer thinks the denylist
   form was adequate, the disagreement to raise is whether a future `GOOS`
   should default to "confined".
3. **§7.4.4's promise is the single most important sentence in rev-3**: "an
   object **observed as different** is never read". Any sentence anywhere —
   PRD, ADR, message, skill — that upgrades it back to "a different object" is
   a defect, and the hard-link case (limit 3) is undetectable *by
   construction*, so no implementation change could close it.
4. **§7.1.1's seam constraint 1 is what keeps the seam from being an escape
   hatch.** Exactly one production implementation, AST-asserted, with a
   sensitivity fixture. If a reviewer can construct a plausible second
   production implementation the constraint would not catch, that is a finding.
5. **§7.4.5's cost statement is deliberately unflattering** (a flat 4,194,305
   bytes on every invocation, including aborts). It should be challenged on
   the merits — Q9 is the escape hatch — not softened.
6. **§14.3.1's exclusion is a security-scope statement, not an omission.**
   Cobra's parse diagnostics are third-party bytes this PRD does not modify.
   The claim to test is that the *hostile-slug* case is still fully covered
   (it reaches `slug-unsafe` and is withheld) and that only the
   mistyped-flag-plus-hostile-slug case is excluded.
7. **AVP-202 proves resolution and arithmetic, not meaning.** Its own doc
   comment says so. Semantic citation review is still the reviewer's job, and
   item 20 above lists exactly what rev-3 found and fixed by hand — a reviewer
   who spot-checks a different sample and finds another mismatch has found a
   real defect.
8. **ADR-034 is `Proposed`, not `Accepted`.** The writer cannot accept its own
   ADR. It becomes Accepted only when it and PRD rev-3 both pass review.

## PRD Writer Result — rev-4 + ADR-034 rev-1 (2026-08-13)

### Files changed (exact, complete)

| File | Change |
|---|---|
| `docs/prds/PRD-artifact-validation-and-provenance.md` | edited in place; `Status: Draft — Awaiting Review (rev-4)`; byline `rev-4 at HEAD be33d2a`; `Architecture` header and `Related` bullet now point at ADR-034 **rev-1** and D1–D18; `Revision history` gains a rev-4 row and records rev-3's disposition as NEEDS REVISION. **4,289 lines** (rev-3: 3,830). |
| `docs/adrs/ADR-034-rooted-filesystem-inspection-boundary.md` | edited in place; `Status: Proposed — Awaiting Review (rev-1)`; byline `rev-1 at HEAD be33d2a`; companion pinned to PRD rev-4; a new `Revision history` table records rev-0's disposition. **945 lines** (rev-0: 635). Decisions D1–D18, in numeric order. |
| `docs/handoff/CURRENT.md` | `Cluster state` flip to `AWAITING REVIEW`, Active Task `Status`/`Rev-4 writer base`/`Scope`, and this section |

Nothing else was touched. **No** edit to `docs/adrs/README.md` (deliberate — see
below), `docs/ROADMAP.md`, `docs/supervisor/LOG.md`, `SPEC.md`,
`docs/whitepapers/**`, `docs/prds/PRD-prepare-intent-bundle.md` (still undrafted,
still blocked), any other `docs/adrs/ADR-0*.md`, `internal/**`, `cmd/**`,
`assets/**`, `tests/**`, `.github/**`, `.wave-close-allowlist`, or any guarded
untracked WIP (`WP-004`, `WP-006`, `WP-007`, `PRD-recurring-patches.md`, the
state-of-the-art case studies).

**Why `docs/adrs/README.md` is untouched.** The dispatch brief scoped rev-4 to
"PRD, ADR-034 and CURRENT" and stated the ADR index needs no status change,
which is correct: ADR-034's status is still `Proposed — Awaiting Review`. The
index line's parenthetical still reads `(rev-0)` and `rev-3`. That is a stale
*revision* label, not a stale *status*, and correcting it would have exceeded
the declared file set. **Flagged for the supervisor**: the index line should be
refreshed to `(rev-1)` / `rev-4` at acceptance, in the same commit that flips
the status to `Accepted`.

### Counts (mechanically verified, commit-independent)

- **PRD 4,289 lines** (rev-3: 3,830; rev-2: 3,075; rev-1: 2,233; rev-0: 1,478);
  **ADR 945 lines** (rev-0: 635).
- **208 acceptance rows** `AVP-001`…`AVP-208` (rev-3: 202), contiguous, zero
  duplicates, zero retired rows. **6 new rows** (`AVP-203`…`AVP-208`); every
  rev-3 ID kept its number, and the rows whose meaning changed (`AVP-084`,
  `AVP-118`, `AVP-139`, `AVP-189`, `AVP-191`, `AVP-193`, `AVP-194`, `AVP-202`)
  were amended in place per §18.1's no-renumbering rule.
- **25 categories**: A 10, B 20, C 8, D 14, E 6, F 5, G 9, H 4, I 6, J 4, K 6,
  L 3, M 6, N 5, O 12, P 10, Q 1, R 9, S 2, T 12, V 17, W 5, X 6, Y 8,
  **Z 20**. Sum = 208, verified by parsing the section headings.
- **By kind**: `U` 61, `I` 96, `S` 6, `G` 31, `S+G` 5, `U+G` 6, `I+G` 1,
  `S+I` 2. Sum = 208.
- **Guard arithmetic**: Kind contains `G` → 31 + 5 + 6 + 1 = **43 rows**
  (rev-3: 39). Complement 61 + 96 + 6 + 2 = 165; 43 + 165 = 208. Note
  `AVP-193` moved `G` → `S+G` (it gained an AST half), so pure `G` went
  30 → 31 rather than 30 → 32.
- **Slice partition verified programmatically**: S1 83, S2 28, S3 49, S4 38,
  S5 10 = 208; zero unassigned, zero double-assigned.
- **Claims audit**: **95 repository claims** `C1`…`C95` (rev-3: 92) and
  **24 Go-stdlib claims** `G1`…`G22` plus `G5a`/`G9a` (rev-3: 21), split into
  **10 `contract`** and **14 `tripwire`**.
- **ADR-034**: **18 decisions** D1–D18 (rev-0: 14), 18 rejected alternatives
  (rev-0: 10), an 18-row decision→acceptance-row dependency table whose every
  `AVP` reference resolves in the PRD.
- **Ladder sizes**: artifact ladder **27 rows** (1–20, 20a, 20b, **20c**,
  21–24); status ladder **20 rows** (1–16, **16a**, 17–19).
- **Catalogs unchanged and still total**: 13 abort codes, 10 reason codes, 10
  advisory codes, 15 lifecycle lines, 9 state-enum values, 9 status outcomes.

### What rev-4 changed, correction by correction

**(1) Platform allowlist → `unix || windows`; `wasip1` unsupported (§7.4.1,
ADR D5, AVP-191 amended, AVP-208 new)**

- `confine_supported.go` → `//go:build unix || windows`;
  `confine_unsupported.go` → `//go:build !(unix || windows)`. Exact negation.
- The platform table gains a **fifth column** (`Supported by this design?`) and
  a separate `wasip1` row so the stdlib's confined set and this design's
  supported set are visibly different objects.
- **rev-3's "byte-identical to `$GOROOT/src/os/root_openat.go`'s tag"
  justification is withdrawn by name.** It reasoned about the *stdlib's*
  implementation set when the question is *this design's*. Four stated reasons
  for excluding `wasip1`: `openFlags()` has two halves not three; the
  `O_NONBLOCK`/FIFO semantics AVP-107/AVP-200 rest on are not WASI preview-1
  semantics; no runner/fixture/cross-build is proposed (which would reproduce
  R7's unexecuted-platform defect); and **no split implementation is specified
  or authorized** — the target is simply refused.
- The asserted property is now a **proper subset** relation: every `GOOS` in
  our `true` set is matched by the stdlib tag, and `wasip1` is matched by the
  stdlib tag and not by ours. AVP-208 owns it, with a sensitivity fixture that
  fails when `wasip1` is re-added without a `wasip1` `openFlags()` half.
- G4 amended; Q8's default sentence updated; one new §22 rejected-alternative
  row; ADR D5 rewritten with the same four reasons.

**(2) Every universal bounded-runtime claim withdrawn (§2.1, §5.1, §7.4.2,
§7.4.3, §8.3, §14.1, §14.4, §15, §16.2, §19 R18, ADR D16 new, AVP-207 new)**

- Three rev-3 sentences are removed and **named** in the withdrawal so a
  reviewer can grep them: "the command has no unbounded wait anywhere" (§8.3),
  "so nothing hangs" (§15), "no leaf kind can hang it" (§7.4.2).
- The guarantee list is rebuilt everywhere it appears (Summary, §2.1 goal 7,
  §7.4.2, §7.4.4, §14.1) as **four** properties: bounded allocation, bounded
  bytes requested, a bounded operation count, and a **non-wedging open** on
  Unix. `O_NONBLOCK`'s scope is stated as "the open, not the read" in §7.4.3,
  AVP-118, AVP-200, G17 and ADR D6/D10.
- New explicit statement, in five places, that an ordinary `read(2)` (and even
  `Lstat`) on a regular file served by a stalled NFS/SMB mount, a wedged FUSE
  server, a `/proc`-style provider or an unresponsive driver **can block
  indefinitely**, and that v1 has **no timeout, no context and no watchdog**.
- §5.1's `--timeout` justification rewritten: the honest reason is "no provider,
  network or subprocess wait to bound, and no cancellation contract defined",
  not "nothing can time out".
- §14.1 gains a fourth threat-model boundary: **availability is out of scope**;
  the security property is confidentiality and integrity.
- §16.2 item 7 extended to forbid any skill surface from making a timing
  promise ("always terminates", "cannot hang", "safe in a blocking preflight").
- **R18 (High)** added; R15's "field hang" phrasing corrected to "wedged open".
- **Q11 (new)** records the additive alternative and states honestly why it is
  not free: Go's `os` file reads are not context-cancellable and
  `SetReadDeadline` does not apply to ordinary files, so a `--timeout` that
  could not interrupt the read would be the same false affordance as rev-2's
  inert `O_NOFOLLOW`.
- **AVP-207** is the mechanical guard. Its row explicitly specifies the
  **quotation-context** rule (a forbidden phrase inside a sentence carrying a
  withdrawal marker is a quotation, not an assertion) plus a fourth *inverse*
  sensitivity fixture asserting the guard is green on the documents as
  written — without which the guard would be permanently red on its own
  withdrawal sections and would be weakened until it proved nothing. **AVP-189
  was amended in place with the same rule**, which it needed already.

**(3) Injectable `SameFile`; exactly two production adapters (§7.1.1, §7.4.4,
ADR D12, AVP-194 amended, AVP-206 new)**

- `RootOps` gains `SameFile(a, b fs.FileInfo) bool`. The seam is now **three +
  three = six methods**; §7.1.1's "five methods wide" and ADR D12's identical
  sentence are corrected.
- Rationale stated: `os.SameFile` is only meaningful over unexported
  `*os.fileStat` values no test outside `os` can construct, so AVP-084,
  AVP-151, AVP-160 and AVP-196 (b) were unwritable — the same defect rev-3
  fixed for `Lstat`/`OpenFile` and left in place one line later.
- **`osFileOps` is now declared explicitly** and returned from `OpenFile`.
  rev-3's sample returned the bare `*os.File`, which made the production
  `FileOps` a type declared in `os` and left AVP-194's "declared outside a
  `_test.go` file" with nothing in `internal/intent` to point at. §7.1.1's code
  sample, its prose, ADR D12 and AVP-194 now all say **exactly two production
  adapters, one per interface, both declared in `internal/intent`** — one
  production implementation per interface, unchanged in spirit.
- Ladder row 13 and status row 9 now read `!ops.SameFile(pre, post)`; §7.4.4
  step 6 likewise; the §7.4.4 race table and AVP-084 updated. G7/G8/G16 and the
  Windows soundness argument are unchanged (they describe what the production
  body delegates to).
- AVP-206 asserts both halves: the injectable verdict (no `os.fileStat`
  construction) and the AST fact that `os.SameFile` appears at exactly **one**
  production call site. R16 updated.

**(4) `Close` in both ladders (§7.4.4 step 11a, §7.5 row 20c, §9.4.2 row 16a,
ADR D15 new, AVP-203/204/205 new)**

- New capture step **11a** (lettered, not renumbered, so every "step 11"
  citation still resolves): close exactly once, **after** the post-capture
  component walk. The ordering is justified, not asserted — holding the
  descriptor across the walk keeps the object pinned so it cannot be unlinked
  and its identity reclaimed while the ancestors are re-observed.
- **Artifact ladder row 20c** → `unreadable`; **status ladder row 16a** →
  `unreadable`. Both sit after the last descriptor-scoped probe and **before**
  any content classification or parse. First-match-wins is stated explicitly:
  20c never overwrites or suppresses an upstream `unstable`/`unreadable`.
- **No new code is minted, and the PRD says why.** Reason `artifact-unreadable`,
  advisory `analysis-sidecar-unreadable`, abort `status-unreadable` — all
  existing. §10.3 gains a paragraph stating the catalogs stay at 10/11/13 so
  AVP-095, AVP-119, AVP-181, AVP-101, AVP-153 keep their arithmetic, and giving
  the reason (a code earns its place by changing the remediation).
- **Truthfulness fix that falls out of this**: `status-unreadable`'s §9.4.5
  message and §10.5.1 lifecycle line are widened to "could not be read **and
  closed cleanly**", because with rev-3's wording a run that read every byte
  and then failed to close would have printed "could not be read". The widened
  form is true of all six of its rows (3, 7, 8, 12, 15, 16a). §10.5.1's
  truthfulness bullet records the change.
- **Zero leaks**: AVP-205 counts opens against closes over every post-open
  ladder row, every status post-open row and all thirteen aborts, asserts zero
  outstanding descriptors when `Inspect` returns, adds an AST half (the close
  is not inside a success-only branch), and ships two sensitivity fixtures
  (skipped close, double close). **R19 (Medium)** added.
- §7.5's ordering list grows from six to **seven** load-bearing orderings;
  the ladder is **27 rows**; §7.6's `unreadable` definition and §15's recovery
  table gain their close rows; §7.1's `Inspect` doc comment states the close
  obligation.

**(5) Cobra parse-error ownership rewritten (§9.5, §10.1, §14.3.1, §5.4,
ADR D17 new, AVP-193 rewritten, C38 amended, C93/C94 new)**

- The mechanism is now stated from source: the root command sets
  `SilenceUsage: true` **and** `SilenceErrors: true`
  (`internal/cli/cobra.go:56-62`), so **cobra prints neither a usage block nor
  the error**; it returns the parse error from `rootCmd.Execute()`, the
  repository's own printer writes `error: %v`
  (`internal/cli/cobra.go:33-39`), and `exitCodeFor` maps the untyped error to
  `1` (`internal/cli/cobra.go:43-52`).
- **rev-3's "and the usage block cobra prints with them" is removed by name**
  — this binary prints no usage block on error, for any command.
- The excluded bytes are correctly characterised as a **repository-emitted line
  wrapping third-party text**, and the PRD states plainly that carrying them
  through our own `Fprintf` does not make them sanitized: the printer is a
  shared pre-existing surface applying no filtering, and changing it would be a
  cross-command behavior change no single command's PRD may make. Those bytes
  are outside the report schema **and** outside the §14.3.3 byte rules.
- **AVP-193's sensitivity fixture is replaced with a real one.** rev-3's
  fixture — "intercepts and re-renders cobra's error inside `RunE`" — is
  **unconstructible**, because a parse error is raised before `RunE`; a guard
  whose negative fixture cannot exist proves nothing. The row is now `S+G`: an
  **AST half** (no `FlagErrorFunc`, no `SetFlagErrorFunc`, no `SetErr`/`SetOut`,
  no self-formatting `Args` validator, no local `Silence*` assignment;
  sensitivity = adding a `FlagErrorFunc`) plus a **behavior half** over the five
  exit-1 inputs (exit 1, empty stdout, exactly one `error:` line matching no
  §9.5 template and carrying no abort code, no report, `.tpatch/`
  byte-identical).
- **C38's description now includes parse errors** and cites C93. C93 (the
  `Silence*` settings) and C94 (`exitCodeFor`'s default-1 mapping) are new.
  §10.1's exit-1 routing row and §9.5's closing paragraph rewritten.

**(6) `eleven` → `twelve` (§9.5, §10.5.2)**

Both occurrences fixed; each cell already listed 5 + 7 = 12 codes. A new
sentence reconciles the arithmetic explicitly: twelve canonical-slug-known
codes plus `slug-unsafe` = the unchanged **thirteen** of §9.4.4, which is the
number AVP-098, AVP-101, AVP-127, AVP-153 and AVP-181 operate over.

**(7) G18 anchor corrected (§23.2, §7.3 step 2, ADR D4)**

The property list (UTF-8, unrooted, slash-separated, no `.`/`..`/empty element,
no leading/trailing slash) lives in the **`io/fs` package documentation's
`# Path Names` section**, not in `ValidPath`'s doc comment, which states only
"valid for use in a call to Open" and links to that section. G18 now anchors
**both** (section for the property, function for the predicate). The row also
records the **`"."` special case** — `fs.ValidPath(".")` is `true` — and notes
that no name this design composes is `"."`, so nothing changes today, but a
future refactor must not assume `ValidPath` refuses it. §7.3 step 2 and ADR D4
carry the same correction.

**(8) ADR D7 name-surrogate table corrected**

`isReparseTagNameSurrogate()` is a **bit test** —
`FILE_ATTRIBUTE_REPARSE_POINT` set **and** `ReparseTag & 0x20000000` — not the
two-tag list its source comment names. rev-0's table answered "no" for *any
other tag*, which is wrong. The corrected table gives each tag's numeric value,
marks AF_UNIX (`0x80000023`) and DEDUP (`0x80000013`) as **non**-surrogates
(so their `ModeDir`/`GetFileType` bits are **not** suppressed), and states that
"any other tag" **depends on that tag's own bit** — `ModeIrregular` either way,
but the suppression differs. A third consequence is added: the two exceptions
are exceptions to the *surrogate* rule as well as the *mode* rule, which is
precisely why the `!IsRegular()` kind gate — not the refusal predicate — must
be total. `winsymlink=1` is retained unchanged. The PRD's §7.4.3 table gets the
same numeric values and the same "depends on the bit" wording.

**(9) Exit-3 workspace divergence disclosed (§9.2, ADR D18 new, C95 new)**

`tpatch status` outside a workspace exits **1** (untyped
`FindProjectRoot` error via `openStoreFromCmd` → `exitCodeFor`'s default);
`tpatch prepare <slug> --check` exits **3** with a full abort report. §9.2 now
states the divergence, justifies it on three grounds (per-command exit
contracts per `SPEC.md:135-141`; this command's exit code is a *verdict* and
collapsing to 1 would make it the only nonzero path with no report, breaking
the `artifacts` ⇔ `abort` invariant; additive and reversible — no existing
command changes and this command never calls `openStoreFromCmd`), and states
what is **not** claimed: a harness grepping exit 1 for "no workspace" will not
get that answer here, and §16.1's `SPEC.md` row must document the envelope.

**(10) Fixed-buffer rationale corrected (§7.4.5, ADR D9, §22, G20/G21/G22 new)**

- **`io.ReadAll(io.LimitReader(f, Max+1))` is bounded.** The limit reader caps
  the result, so total allocation is `O(Max)`. rev-1 claimed a false *exact
  ceiling*; rev-2/rev-3 over-corrected into "the allocation is not bounded",
  which is equally false. rev-4 states both accurately and forbids the
  unboundedness claim.
- The real rejection is cost **shape**: `ReadAll` grows by `append`, so one
  capture performs a *sequence* of increasing allocations with copies, and the
  sequence is paid again on each of **five** sequential captures. The chosen
  reusable fixed buffer **caps and flattens** that cost.
- **The ~4 MiB zeroing cost is now stated**: `make([]byte, n)` yields a zeroed
  slice (G22), so the invocation pays one zeroing pass — once per invocation,
  never per capture. Q9's note updated (lazy allocation removes the cost only
  for runs that open nothing).
- **Cap↔message coupling is restated exactly and unchanged**: AVP-201 *derives*
  the unit string from the constant (never compares two literals), asserts
  appearance in exactly the enumerated messages and non-appearance of any other
  limit figure, and fails in **both** directions.
- G20 (LimitReader bounds → contract), G21 (ReadAll growth shape → tripwire)
  and G22 (zeroed `make` → contract) added; §22's `ReadAll` row rewritten;
  R17 updated.

**(11) Walk→`Lstat` race limit retained and sharpened (§7.4.4, §8.3, ADR D8)**

The residue is restated in the place most likely to be read as stronger than it
is: an object *observed as different* is never read, **but** an object never
observed to be different — most importantly a consistent in-root alias or hard
link that satisfies every probe — **is** read, and its bytes are then
**attributed to the canonical artifact name** in the report. The claim is "the
object I classified is the object I read", not "the canonical name designated
this object at every instant". Limits 1, 2 and 5 in §8.3, the §7.4.4 limit
list, ADR D8 and R5 all carry the attribution sentence.

**(12) Cap-message coupling and Windows junction mechanism unchanged**

`cmd /c mklink /J`, `t.Fatal` never `t.Skip`, the source half forbidding
`t.Skip`/`t.Skipf`/`t.SkipNow` in the fixture path, and `runtime.GOOS !=
"windows"` as the only permitted guard — all verbatim from rev-3 (§16.1,
AVP-199, ADR D13). AVP-201's mechanism likewise, now stated explicitly as
derive-not-compare and fail-in-both-directions.

**(13) IDs, counts, categories, kinds, slices, claims, citations, guards**

All updated and mechanically re-verified — see "Validation performed".

### Unchanged from rev-3 (deliberately, as instructed)

- **`os.Root` as the mechanism** and the whole D1/D3/D4 policy.
- **Logical (not physical) confinement**, §7.4.2's table, the quoted `Root`
  doc-comment paragraph, AVP-189, AVP-190, R14, ADR D2.
- **Full three-doc readiness** (§6.2): `ready` requires `analysis.md` +
  `spec.md` + `exploration.md`; the sidecar stays optional.
- **`status.json` safety and `FeatureState` validation**: same boundary, same
  caps, all nine populations, thirteen abort codes, fifteen lifecycle
  annotations, valid-state gate before any echo.
- **`provenance` is the stable constant `unknown`**; no ADR trigger fired
  (ADR D14).
- **Manual/routing compatibility**: loose `--manual`, `next` and `cycle`
  unchanged; AVP-064…AVP-069 and AVP-130…AVP-133 still pin the loose behavior.
- **No mutation, no new state**; **the `prepare` reserved-surface block**
  (exit 4) and the unregistered `--manual`/`--regenerate`.
- **`PRD-prepare-intent-bundle.md` remains blocked and undrafted** (§20).
- **ADR status**: `Proposed — Awaiting Review`. The writer cannot accept its
  own ADR.

### Open decisions left to review (§21, eleven)

Q1 `--all`; Q2 exit `4` as a cross-command convention; Q3
`MaxArtifactBytes = 4 MiB`; Q4 `request.md` as a fifth row; Q5
platform-conditional Windows reserved-device refusal; Q6 `--format` aliasing;
Q7 `MaxStatusBytes = 1 MiB` and its separateness; Q8 compile-time vs runtime
platform refusal; Q9 lazy vs unconditional scratch allocation; Q10 whether the
seam should be unexported; **Q11 (new)** whether v1 should bound wall-clock
with a `--timeout` or a cancellable context. All eleven have a stated default;
none blocks review.

### Implementation status

**None.** Planning only. No Go file, test, asset, workflow or CLI surface was
created or modified. `cmd/tpatch/main.go`, `.github/workflows/ci.yml` and
`SPEC.md` remain named as **required future** edits (§16.1) and were **not**
touched.

### Validation performed (docs-only change)

- **AVP mechanics**: 208 declared rows, contiguous `AVP-001`…`AVP-208`, zero
  duplicates; **every** `AVP-NNN` token in the PRD's prose **and in ADR-034**
  resolves to a declared row (zero unresolved); category counts parsed from the
  section headings sum to 208 and match §18.27; kind counts parsed from the
  Kind column sum to 208 and match; the guard predicate yields **43**; §17's
  slice assignment expanded programmatically and verified as a partition of
  1…208 (S1 83, S2 28, S3 49, S4 38, S5 10 — zero duplicates, zero missing,
  each slice's stated count equal to its expanded count).
- **Guard-count self-check caught a writer error**: the first draft of §18.27
  stated pure `G` = 32 and guard = 44. The parser returned 31 and 43 because
  `AVP-193` moved out of pure `G` into `S+G`. §18.27, §18.28 and AVP-139 were
  corrected to 31/43 and the narrative now explains the move.
- **Claims mechanics**: `C1`…`C95` contiguous, zero duplicates;
  `G1`…`G22` + `G5a` + `G9a` = 24 rows, 10 contract + 14 tripwire = 24.
- **Repository anchors**: every `file:line` citation in **both** documents
  parsed and checked in-range; zero out-of-range, zero missing files. The four
  new/changed ones were content-verified by reading the cited lines —
  `internal/cli/cobra.go:56-62` (`SilenceUsage`/`SilenceErrors` on the root
  command struct, C93), `:43-52` (`exitCodeFor`'s default `return 1`, C94),
  `:33-39` (the `Fprintf` printer, C38), `:3782-3793` +
  `internal/store/store.go:23-40` (`openStoreFromCmd` → `FindProjectRoot`
  returning a plain `errors.New`, C95).
- **Go stdlib claims re-read from the pinned toolchain's `GOROOT`**
  (`go1.26.5`): `os/types_windows.go` `(*fileStat).mode`,
  `isReparseTagNameSurrogate` (confirming the `0x20000000` bit test and the
  AF_UNIX/DEDUP branches verbatim, including the DEDUP explanatory comment),
  `Mode`/`modePreGo1_23`; `io/fs/fs.go` package doc `# Path Names` section and
  `ValidPath`'s body (confirming the `"."` special case returns `true`);
  `os/root_openat.go`'s build tag; `os/root_unix.go` `rootOpenFileNolog`.
- **Cross-reference closure**: every `§N.N` reference in the PRD **and** in
  ADR-034 resolves to a PRD heading that exists (zero unresolved).
- **ADR↔PRD parity**: ADR declares D1–D18 with no gaps, in numeric order (D13
  and D14 were moved back above D15 after the new decisions were appended);
  every `AVP` reference in the ADR's 18-row decision→verification table
  resolves to a declared PRD row; every ADR decision D1–D18 is cited by name
  somewhere in the PRD; the ADR's companion pointer, references block and
  consequences all name rev-4.
- **Markdown hygiene**: PRD 44 fence markers (balanced), 69 tables with **zero**
  column-count mismatches (counted with `\|` escapes stripped, which is what
  the earlier "mismatch" noise was); ADR 2 fence markers (balanced), 6 tables
  with zero mismatches; **zero** trailing-whitespace lines in either; every
  relative link in both resolves on disk; both `json` fenced blocks parse.
- No Go source changed, so `gofmt` / `go build` / `go test` are not applicable
  to this change set; no existing docs test targets these files
  (`internal/workflow/docs_totality_guard_test.go` reads exactly three
  verify-family documents, none of them touched here).

### Reviewer focus for rev-4

1. **The `wasip1` exclusion is a *narrowing*, and it costs a target.** rev-3
   would have run confined on `wasip1`; rev-4 refuses it. The argument is that
   confinement is necessary but not sufficient — the design also needs an
   `openFlags()` half and a runner, and has neither. If the reviewer thinks
   `wasip1` support is worth a third build-tagged half, that is a scope
   *addition* with its own slice, not a build-tag edit; **rev-4 deliberately
   does not split the implementation**.
2. **The bounded-runtime withdrawal is the largest semantic change.** Nothing
   about the *code* changes — the same `O_NONBLOCK`, the same kind gates — only
   what the documents promise. Check §7.4.2, §8.3 and §14.1 for any residual
   assertion, and check that AVP-207's quotation-context rule is specified well
   enough to be implementable: the guard must tolerate the withdrawal
   sections' own quotations of the removed sentences, which is why it ships a
   fourth, *inverse* fixture asserting green-on-current-text.
3. **`Close` mints no new code, deliberately.** The alternative (a fourteenth
   abort code) is argued down in §22 and ADR D15 item 5. The visible cost is
   that `status-unreadable`'s frozen message and lifecycle line changed text;
   if the reviewer prefers a new code, that reopens the thirteen-code
   arithmetic in five places (AVP-095, AVP-098, AVP-101, AVP-153, AVP-181).
4. **`SameFile` is one production method that exists only for testability.**
   Its production body is a one-line delegation. If the reviewer objects to
   widening the interface for tests, the counter-argument is in §7.1.1 and
   ADR D12 constraint 3: without it, four identity rows are unwritable, which
   is the exact defect class rev-3 was revised to fix.
5. **Two production adapters, one per interface** — this is a *clarification*
   of rev-3's "exactly one production implementation", not a relaxation. rev-3
   said one implementation of *each* interface but its code sample returned a
   bare `*os.File`, which put the `FileOps` implementation in package `os`.
   Check §7.1.1 rule 1, ADR D12 constraint 1 and AVP-194 agree on this.
6. **Ladder row lettering.** 20c and 16a are lettered so no rev-2/rev-3 row
   citation breaks. Verify the ordering claims: 20c after 20a/20b, before
   21–24; 16a after 16, before 17.
7. **AVP-193 is the one row whose *kind* changed** (`G` → `S+G`), which is why
   the guard arithmetic moved 39 → 43 rather than 39 → 45. §18.27 explains it.
8. **`docs/adrs/README.md` is untouched by design** and its parenthetical
   revision labels are now stale. That is flagged above for the supervisor to
   fix at acceptance, not silently left.

## WP-005 Turn 2 Scope

- Append one `agreement` turn introducing the new stable agent ID.
- Record the council method and the advisor/reviewer split honestly.
- Add `## Agreed` findings: validation before orchestration, `prepare --check`
  first, no new lifecycle state, optional internal SDD, downstream SDD
  encouraged but never enforced.
- Resolve WP-005's prior "separate whitepaper" uncertainty: this paper is
  sufficient prior art for two gated PRDs.
- Keep the paper `Exploring` until the first graduating PRD is drafted.
- Do not edit code, create an ADR, or add prepare to the implementation
  roadmap in this task.

## WP-005 Turn 2 Writer Result

- Added CO76 as the second stable whitepaper author and appended one
  `agreement` turn without editing Turn 1.
- Recorded the full council split (2 ship now / 2 reject / 1 specify first),
  five-reviewer convergence and chairman disposition.
- Added an unbylined `## Agreed` section covering validation before
  orchestration, check-first delivery, no new state, optional internal SDD and
  downstream encourage-never-enforce.
- Replaced the open-ended "separate whitepaper" product note with two ordered
  PRDs: artifact validation/provenance first; prepare bundle blocked second.
- Kept `Status: Exploring` until the first graduating PRD is drafted.
- Added current-code anchors for manual validation, Path A's structured
  analysis sidecar and `next`'s file-presence inference.

## Turn 2 Review Adjudication

- **Internal**: NEEDS REVISION — WP-004 is guarded/untracked, so WP-005's
  Related link breaks in a clean checkout.
- **External**: APPROVED WITH NOTES; five valid findings are folded:
  1. provenance must be `unknown` until a representation is chosen, and the
     prerequisite PRD may trigger an ADR at decision time;
  2. all-or-nothing publication includes intent artifacts, sidecars and
     `status.json`, not just staged Markdown;
  3. compare `prepare` explicitly with `cycle`, `next` and the individual
     manual phases;
  4. richer validation is advisory for existing routing unless a later PRD
     enumerates a deliberate behavior change;
  5. map each council lens to its position in the durable record.
- **Coupled observation accepted**: update the whitepaper index for all tracked
  WP-002/WP-003/WP-005 documents.
- **Protocol**: Turn 2 remains immutable; corrections append as Turn 3.

## Turn 3 Revision Result

- Removed the broken guarded-WP-004 link without staging unrelated WIP.
- Mapped every advisor lens to its position and explained the 3/2 peer-review
  selection that produced the validation-first verdict.
- Defined legacy/current provenance as `unknown` until accepted persistent
  metadata proves otherwise; PRD-1 may trigger an ADR when it makes that
  choice.
- Expanded atomic publication to Markdown, structured sidecars and
  `status.json`; distinguished shared validation from incremental phase
  writers.
- Added the required existing-primitives pre-flight for individual
  `--manual`, `cycle` and `next`.
- Made slice-1 validation advisory and routing-compatible by default.
- Refreshed the tracked whitepaper index for WP-002, WP-003 and WP-005.

## Turn 3 Review Verdict

- **Internal**: APPROVED.
- **External**: APPROVED WITH NOTES.
- Every rev-0 finding is closed.
- LOW N1 closed by attributing council versus review-derived Agreed items.
- LOW N2 is pinned as a required first-PRD acceptance decision: report-only
  versus stronger mutating `--manual` gates and their compatibility.
- Turn 4 records the disposition append-only.

## Post-Release Review Adjudication

- **External verdict**: APPROVED WITH NOTES.
- **F1 LOW — valid**: Wave C extraction left 16 accepted-contract
  `verify.go:<line>` anchors stale; re-resolve them against current symbols
  and refresh the validation marker.
- **F2 LOW — valid**: `TestAcceptanceLedger_TestsExist` can false-pass on a
  comment because it searches raw bytes. Replace it with package-aware AST
  resolution and a sensitivity regression.
- **Observation — deferred explicitly**: the wave-close untracked-source
  sentinel cannot enumerate source files inside an untracked nested Git
  repository. Registered as GH #9; no release invariant is invalidated.
- **Tag policy**: fold only on `main`; never move v0.15.1.
- **Next planning work, deferred one turn by the operator**: append WP-005
  council Turn 2, then draft artifact-validation/provenance and prepare-bundle
  PRDs through the normal writer/review loop.

## Post-Release Fold Result

- Every verify-family citation in ADR-013 and PRD-verify-freshness was
  re-resolved against the extracted source at tag `15560af`; a non-vacuous
  bounds guard now checks the full reference set.
- Stale present-tense `active`-parent prose was corrected to describe the
  shipped Wave C behavior and its current source sites.
- The 161-row ledger now resolves an exact package, runnable
  `func TestX(*testing.T)` declaration, and optional literal subtest through
  Go AST. Comments, wrong/external packages, invalid signatures, unused table
  names and unrelated literals all fail sensitivity fixtures.
- Three internal review passes closed package/signature/subtest and citation
  scope/semantics findings. The final WIP-file observation was dismissed:
  those files predate this task, are guarded, and have an empty diff.
- Full uncached suite, vet and build pass.
- GH #9 records the deferred nested-repository source-sentinel blind spot.
- v0.15.1 remains byte-fixed at `15560af`; the fold is main-only at `64010bf`.

## Rev-3 Review Verdict

- **Internal**: APPROVED.
- **External/original reproducer**: APPROVED.
- No high-confidence defect remains.
- The original pre-land → land → post-land sequence and `verify --all` pass.
- The C-locale, offline apply-classifier grammar resisted fatal, mixed,
  wrapper, signalled, translated and broad-phrase adversarial outcomes.
- All rev-0 through rev-2 findings remain closed.

## Session Summary — rev-3 fold

**P1 — deterministic locale.** `internal/gitutil/trailers.go` appends
`GIT_NO_LAZY_FETCH=1` and the new `CLocaleEnv` (`LC_ALL=C`) LAST to
every evidence invocation, so both win over the inherited environment
AND any caller-supplied extra. That covers `log`, `rev-parse`,
`read-tree`, `apply`, `diff`, `cat-file` and `merge-base` — rev-2
forced the locale only on the `-C0` ladder step, leaving every other
classified diagnostic at the mercy of the ambient locale. The
`ForceCLocale` option is removed rather than left as a second, weaker
path. The workflow gateway's `offlineEnv()` carries both entries too, so
the shadow worktree commands are equally deterministic. Unrelated
callers are untouched: `shadowEnv(nil)` still returns nil.

**P1 — narrow, safe `ApplyProbeAnswered`.** The broad substring list
(`already exists`, `new file`, `deleted file`, `patch does not apply`,
…) is gone. The rule is now:

- success → answer;
- exit 1 → answer, by exit code alone (measured: every ordinary
  conflict exits 1, so no text is consulted);
- any exit other than 0/1/128 → FAILURE regardless of stderr, which
  covers signalled and unstartable processes (-1), 2, 126, 127, 129+;
- exit 128 with a missing-object or network diagnostic → FAILURE;
- exit 128 → answer ONLY when every non-empty stderr line matches the
  anchored C-locale malformed-patch grammar, with at least one
  recognised diagnostic line.

The grammar is `IsMalformedPatchDiagnostic`: anchored regexps for
`No valid patches in input (allow with "--allow-empty")`, `corrupt patch
at <path>:<line>` and `at line N`, `patch fragment without header`,
`patch with only garbage at line N`, and `corrupt binary patch`
(including its paired no-valid-patches line), plus `Checking patch …`
as an informational line that can never satisfy the requirement alone.
A `fatal:` spoof, a wrapper line, an unknown line, an empty diagnostic
or any mixture is rejected.

Measured on git 2.55.0 under `LC_ALL=C` (probe repo removed):

| input | exit | stderr |
|---|---|---|
| empty / garbage / prose | 128 | `error: No valid patches in input (allow with "--allow-empty")` |
| truncated or garbage hunk | 128 | `error: corrupt patch at ../p.patch:5` |
| fragment without header | 128 | `error: patch fragment without header at ../p.patch:1: @@ …` |
| corrupt binary payload | 128 | `error: corrupt binary patch at …` + the no-valid-patches line |
| new file that already exists | **1** | `error: f.txt: already exists in index` |
| delete of a missing file | **1** | `error: gone.txt: does not exist in index` |
| context mismatch | **1** | `error: f.txt: patch does not apply` |

`unrecognized input` was NOT reproducible on this git and is therefore
excluded — the classifier fails closed to `unavailable` for anything it
cannot demonstrate.

## Current State

- GH #8 stays empirically closed with the rev-3 binary, including under
  a foreign ambient locale (`LC_ALL=fr_FR.UTF-8`): before land passes,
  after land `landing evidence: exact` with a dual-anchor baseline, and
  `verify --all` exits 0.
- AC-L68 / AC-L69 remain green against a real filtered remote.
- No blocker.

## Files Changed (rev-3)

New:

- `internal/gitutil/apply_classifier_test.go`
- `internal/workflow/verify_rev3_fold_test.go`

Modified:

- `internal/gitutil/trailers.go` (`CLocaleEnv`, unconditional locale,
  anchored grammar, exit-code-led `ApplyProbeAnswered`; `ForceCLocale`
  removed)
- `internal/workflow/verify_gitgate.go` (`offlineEnv` carries the
  locale)
- `internal/workflow/verify_landed.go` (ladder no longer passes a
  per-call locale flag)
- `internal/workflow/acceptance_ledger_test.go`
- `CHANGELOG.md`, `docs/handoff/CURRENT.md`

## Test Results

- `gofmt -l .` clean; `go vet ./...` clean; `GOOS=linux` and
  `GOOS=windows` vet clean.
- Cross-builds clean: linux/amd64, linux/arm64, darwin/arm64,
  windows/amd64.
- `go test -count=1 ./...` — all 12 packages pass.
- `go test -race -count=1 ./internal/workflow ./internal/gitutil
  ./internal/store ./internal/cli` — pass.
- **Regression proof.** Against the rev-2 production files, 6 rev-3
  workflow sub-tests fail (four broad-phrase historical-V8 exits, the
  ladder broad-phrase case, and the C-locale assertion). The classifier
  itself is proven in-tree by
  `TestRev3ClassifierFixesRev2Misclassifications`, which reproduces the
  rev-2 predicate verbatim and enumerates ten outcomes it promoted to a
  patch verdict and rev-3 rejects;
  `TestRev3ClassifierKeepsEveryRealAnswer` proves no real git answer was
  lost.
- Real-git goldens: `TestRealGit_MalformedPatchesExit128AndAreAdmitted`
  and `TestRealGit_OrdinaryConflictsExitOne` measure the actual
  diagnostics rather than mocking them.
- Acceptance ledger: 161/161 rows mapped; AC-L26/L58/L69/L117/L123/
  L129/L134 remapped to the rev-3 black-box and golden proofs.
- Docs totality guard (AC-L135 / G1–G10): zero hits; accepted documents
  untouched.
- GH #2 regression green and unmodified (AC-L121 diffs it against
  WAVE_BASE `b768602`).
- Side Research md5: `b385fe622db9926f48861105239f113e`.

## Next Steps

1. Complete rev-1 PRD + ADR-035 correction.
2. Re-review both documents together.
3. Continue bounded revisions until both documents are accepted.
4. Keep implementation blocked throughout.

## Blockers

Rev-0 is not acceptable until the findings below close.

## Rev-0 Review Adjudication

- **Internal**: NEEDS REVISION (2 CRITICAL, 3 HIGH, 1 MEDIUM).
- **External**: NEEDS REVISION (3 HIGH, 9 MEDIUM, 6 LOW).
- **Supervisor verdict**: NEEDS REVISION.

### Transaction/recovery blockers

1. Locks created before/after the journal (CP1/CP2/CP8 and all `--manual`
   crashes) cannot prove ownership, so stale recovery is impossible.
2. Canonical and metadata rollback need semantic new-image CAS checks; a
   forged journal or concurrent edit must never be overwritten.
3. T1 all-old/all-new is only a command-owned guarantee. Concurrent unlocked
   writers can change files after publication; wording and tests must scope it.
4. Rooted preflight paired with path-based rename leaves an ancestor-swap write
   race; publication writes need a rooted write boundary or an honest locked
   alternative.
5. Exit-6 divergent states need a safe, explicit operator recovery/abandon
   path; doctor must report orphan locks/staging as well as journals.

### Archive/privacy blockers

1. Default mode can overwrite an existing sidecar without `--regenerate`; the
   preservation/archive policy is contradictory.
2. The tracked never-pruned archive can permanently preserve removed secrets,
   grows without a retention policy, and is not clone-durable until committed.
3. Archive/index strict decoding, semantic journal binding and orphan-blob
   disclosure are incomplete.
4. Archive persistence is bytes-at-path history only; Notes and output must not
   become durable provenance claims.

### Product/compatibility blockers

1. `--regenerate` may replace a hand-authored bundle with heuristic boilerplate
   after provider failure; timeout cascade/fallback policy must be explicit.
2. `--manual`/multi-file status publication interacts with `FEATURES.md`;
   the claimed write set and refresh semantics are false/incomplete.
3. Local-lane gates and archive advisories perform Git operations and can
   refuse non-Git workspaces; disclose or redesign instead of claiming no Git.
4. The accepted `--check` prerequisite is planning-only, so implementation
   sequencing/goldens must require it to land first.
5. Flag delta table, dry-run/recovery ordering, default upstream-coherence
   cases, lifecycle states and provider behavior need complete rows.

### Matrix/architecture follow-up

- Revise ADR-035 D2/D4/D8/D13 with the PRD.
- Add real rows for every stale-lock, CAS, archive privacy/retention,
  concurrent-writer, rooted-write, provider fallback and prerequisite case.
- Keep T0 rejected, provenance `unknown`, no new state, and implementation
  unauthorized.

### Additional external concurrence

1. Evaluate and adopt `(*os.Root).Rename` for handle-relative publication;
   narrow the residual race to content identity rather than over-conceding the
   pathname half.
2. Engage ADR-027 D3 explicitly. The archive turns overwrite-removable mistakes
   into retained tracked bytes, so rev-1 must define retention and a safe
   removal/redaction route rather than claiming no new exposure class.

1. **ADR-034 is `Proposed`, not `Accepted`.** A writer cannot accept its own
   ADR. It and PRD rev-3 are reviewed together; acceptance of both is the
   precondition for dispatching implementation slice S1.
2. **`PRD-prepare-intent-bundle.md` remains blocked** and undrafted until this
   PRD is accepted (§20).

No implementation is authorized. `cmd/tpatch/main.go` and
`.github/workflows/ci.yml` are named as required *future* edits and were not
touched.

## Rev-2 Review Adjudication

- **Internal**: NEEDS REVISION (4 HIGH, 2 MEDIUM).
- **External**: NEEDS REVISION; all rev-1 findings closed, then bounded
  `os.Root`, seam, citation and platform-policy findings.
- **Supervisor verdict**: NEEDS REVISION → rev-3 + ADR-034.
- **Disposition**: every item below is **closed in rev-3** — see "PRD Writer
  Result — rev-3 + ADR-034 rev-0" for the point-by-point mapping.

### Architecture decision now requiring ADR

Adopting a held `*os.Root` as a new read-only rooted namespace is a non-obvious
repository architecture choice. It differs from the shipped
`rescap.GatePath` pathname model and sets platform/confinement/identity policy.
Rev-3 must create proposed
`ADR-034-rooted-filesystem-inspection-boundary.md` and review it with the PRD.
This is **not** the deferred provenance ADR; provenance remains constant
`unknown`.

### Rev-3 required corrections

1. Scope `os.Root` honestly: logical root confinement, not physical filesystem
   boundary confinement; bind mounts/filesystem boundaries remain reachable.
2. Use allowlist build tags (`unix || windows || wasip1`) and fail closed on
   other targets; validate slug before platform selection.
3. Remove `EnsureSafeRepoPath` misuse and use canonical `fs.ValidPath`
   root-relative names.
4. Correct Windows reparse mapping, pin `winsymlink=1`, and make native Windows
   junction tests fail rather than skip.
5. Define injectable rooted/file operation seams for deterministic race/error
   rows without weakening production call-graph guards.
6. Reuse one fixed cap-plus-one data buffer per inspection and document cost.
7. Weaken identity claims to “objects observed as different are never read”;
   document inode/file-ID reuse and same-identity aliases.
8. Pre/post-walk components per capture; describe the residual swap-back
   limitation.
9. Scope attacker-byte guarantees to command-owned output; Cobra parse errors
   remain generic CLI behavior.
10. Correct every drifted AVP citation and add a citation-resolution guard.
11. Couple cap values to frozen messages, define Windows test mechanism, and
    close the remaining status/output wording.

## Rev-3 Review Adjudication

- **Internal**: NEEDS REVISION (4 HIGH, 1 MEDIUM plus one LOW).
- **External**: APPROVED WITH NOTES; architecture accepted, four contract
  parity corrections required before S1.
- **Supervisor verdict**: NEEDS REVISION → narrow rev-4.

### Rev-4 required corrections

1. Remove `wasip1` from the supported allowlist (or define a separate
   compilable flag contract); this revision chooses fail-closed
   `unix || windows`.
2. Withdraw universal no-unbounded-wait claims. Allocation is bounded; ordinary
   OS reads can still block. Static/raced FIFO handling remains tested.
3. Add an injectable `SameFile` operation to the test seam so identity rows are
   implementable without constructing private `os.fileStat` values.
4. Add `Close` failures to status/artifact ladders and AVP coverage.
5. Correct Cobra parse-error ownership: the shared root printer emits pflag
   text unsanitized; those generic exit-1 bytes are outside this command's
   report schema.
6. Align `RootOps`/`FileOps` samples, ADR D12 and AVP-194 on two adapters.
7. Fix the twelve-code arithmetic, G18 `fs.ValidPath` anchor, Windows
   name-surrogate wording and workspace-exit divergence disclosure.
8. Preserve ADR decisions, update matrix/claims/counts, and re-review only the
   affected contract surfaces.

## Rev-1 Review Adjudication

- **Internal**: NEEDS REVISION (2 HIGH, 3 MEDIUM).
- **External**: NEEDS REVISION; every rev-0 finding closed, then three
  blocking status/output findings plus bounded completeness notes.
- **Supervisor verdict**: NEEDS REVISION → rev-2.
- **Disposition**: every item below is **closed in rev-2** — see "PRD Writer
  Result — rev-2" for the point-by-point mapping.

### Rev-2 architecture correction

Go 1.26's `os.Root` is the cross-platform rooted namespace primitive. Rev-2
must open one root for the repository and use root-relative `Lstat`/open for
`status.json` and all artifacts. This closes ancestor escape without custom
pathname re-resolution:

1. Refuse observed symlink components.
2. Before reading, compare rooted `Lstat` identity to the opened handle's
   `File.Stat` identity and recheck kind/size.
3. Use final no-follow/nonblocking flags on Unix. On Windows rely on
   `os.Root`'s handle-relative implementation and handle-derived stat identity,
   not ordinary `os.Lstat`/`os.SameFile`.
4. Read into one preallocated `MaxArtifactBytes+1` buffer/loop; do not claim
   `io.ReadAll(LimitReader)` has an exact allocation ceiling.

### Remaining corrections

- Apply the same safe bounded capture to `status.json`.
- Validate `FeatureState`; never echo unknown lifecycle bytes.
- Define human lifecycle text for status present/absent/unreadable/malformed/
  invalid-state populations.
- Close abort messages and correct `--path`/workspace error ownership.
- Correct quiet-abort guidance, control-byte wording, guard arithmetic,
  canonical hand-assembled feature scope and skill wording for ordinary exit 2.
- Rebuild AVP totals around all new status/root/race cases.

## Context for Next Agent

Reviewer focus for rev-3:

1. **Exit 1 is decided WITHOUT reading stderr.** That is what makes the
   grammar safe to narrow: every ordinary conflict — including the
   `already exists` / `does not exist` / `does not apply` forms rev-2
   listed — exits 1, so removing them from the grammar cannot cost a
   real answer. `TestRealGit_OrdinaryConflictsExitOne` measures it.
2. **Exit 128 is the only text-sensitive case**, and it is required:
   git reports a malformed or empty patch with exit 128, and two shipped
   GH #2-era rows depend on that staying a patch-level answer.
3. **The grammar fails closed.** A form that could not be demonstrated
   on this git (`unrecognized input`) is excluded; an unrecognised
   diagnostic yields `unavailable` (R10), which is the honest "the
   reader could not answer" outcome rather than a fabricated patch
   verdict.
4. **Locale is applied at the ONE env builder**, so there is no second
   path that can drift. `ForceCLocale` was deleted rather than kept as a
   no-op, to remove the ambiguity of two mechanisms for one guarantee.

Carried forward from rev-0/rev-1/rev-2 (still true):

- `landing_evidence.state` is OMITTED for the D10 artifact-presence
  short-circuit; `failed_at` and `reason` carry the outcome.
- Classification terminals run before the static short-circuit and
  allocate no shadow; unanswerable probes route through
  `terminalEvidencePhase` so remediations stay inside the closed
  R10/R22 set and `failed_at` inside the thirteen-value vocabulary.
- V2 parses the captured recipe bytes; the verify path has no live
  artifact read (AST-guarded).
- AC-L84 and AC-L111 are proven in two halves because V4 pre-empts the
  arbitration branch; AC-LD18a's refusal half is driven through the
  production validator.
- The isolated index lives under `<git-dir>/tpatch-verify`.
- Stray build artifacts in the repo root predate the session and are
  gitignored; they were left untouched.

## Side Research — State-of-the-art middle pass (2026-05-10)

Paper-only exploratory pass completed for a non-LLM middle layer between
deterministic reconcile heuristics and full provider/coding-agent workflows.
This does **not** change code, schema, CLI behavior, roadmap status, PRDs, or
ADRs.

### Research packet

Created `docs/state-of-the-art/` with docs modeled after the existing market
research / PRD conventions: header block, related links, refresh triggers,
references, open questions, and disputes.

Files:

- `docs/state-of-the-art/README.md`
- `docs/state-of-the-art/patch-theory-and-commutation.md`
- `docs/state-of-the-art/patch-identity-and-structural-fingerprints.md`
- `docs/state-of-the-art/search-based-patch-application.md`
- `docs/state-of-the-art/experiment-guide-structural-middle-pass.md`
- `docs/state-of-the-art/tpatch-metadata-for-patch-identity.md`
- `docs/state-of-the-art/patch-capture-context-research-brief.md`
- `docs/state-of-the-art/patch-capture-prior-art-and-hooks.md`
- `docs/state-of-the-art/research-roadmap.md`
- `docs/state-of-the-art/tpatch-middle-pass-synthesis.md`

### Findings

1. Patch theory is useful as vocabulary for identity, inverse, composition,
   commutation, dependency, and conflict, but tpatch should not claim
   Darcs/Pijul guarantees on top of unified diffs.
2. Patch identity should be treated as a ladder: exact bytes, `git patch-id`,
   token fingerprints, AST/CFG/PDG similarity, behavioral checks, and finally
   provider/human intent judgment.
3. Computer-vision feature matching maps to code relocation: detect salient
   code keypoints, compute local descriptors, match across old/new upstream,
   reject outliers, then attempt relocated apply in a shadow tree.
4. Search-based application should operate only on uncertain patch clusters,
   after deterministic dependency/commutation pre-passes shrink the search
   space.
5. Beam search is the likely first practical non-LLM planner; MCTS and
   evolutionary algorithms remain candidates for larger uncertain clusters.
6. Vector retrieval / RAG fits as a distinct middle layer: dense retrieval can
   rank likely patch/hunk/code-region matches below full provider reasoning,
   while generation over retrieved context still belongs to the provider tier.
7. The experiment guide defines collection formats for feature metadata, hunks,
   keypoints, fingerprints, retrieval results, commutation relations,
   candidate apply attempts, metrics, and ground-truth labels.
8. First-party tpatch metadata should be the happy path for tpatch-aware repos:
   current metadata is good for lifecycle/DAG reasoning, but future patch
   generations, dependency version snapshots, operation IDs/read-write sets,
   structural anchors, relation artifacts, and vector manifests would make
   identity and ordering easier before fuzzy fallback.
9. A new patch-capture research brief preserves this PRD/ADR queue and defines
   the next front: Quilt-style explicit file claims, Git index/hook boundaries,
   IDE hooks, coding-agent event logs, and privacy-safe agent context capture.
10. Entire is verified as a concrete prior-art target. Its model uses Git hooks,
    agent hooks, commit trailers, a separate `entire/checkpoints/v1` metadata
    branch, shadow checkpoints, full transcript/session storage, redaction, and
    optional checkpoint remotes. tpatch should borrow the Git-native linking
    pattern but default toward summaries/references over raw transcripts.
11. `docs/state-of-the-art/research-roadmap.md` is now the durable exploratory
    tracker so research can advance independently if `docs/handoff/CURRENT.md`
    is reassigned to implementation work.
12. Amendment models differ by tool: Quilt/StGit usually refresh the managed
    patch, Git supports both amend and fixup/squash-forward workflows, Aider
    favors small commits plus undo, and Entire preserves context links around
    rewrites. tpatch likely needs canonical-current patch plus append-only
    generations, with explicit amend/fixup/fold/fork semantics.

### PRD drafts promoted from research (2026-05-13)

The first capture/metadata foundation PRDs were drafted as paper-only planning
docs:

- `docs/prds/PRD-feature-file-claims.md`
- `docs/prds/PRD-record-capture-modes.md`
- `docs/prds/PRD-feature-patch-identity-metadata.md`
- `docs/prds/PRD-feature-patch-amend.md`

`docs/state-of-the-art/research-roadmap.md` is updated to point at these drafts.
The remaining gate before implementation is review/acceptance of the queued
capture privacy and amendment-policy ADRs plus PRD review.

### Candidate follow-up names

These are research outputs only, not queued roadmap work. Four items below now
have draft PRDs as noted above.

- `PRD-structural-patch-fingerprints`
- `PRD-feature-patch-identity-metadata`
- `PRD-dependency-version-snapshots`
- `PRD-recipe-operation-identity`
- `PRD-structural-anchor-manifest`
- `PRD-patch-vector-index`
- `PRD-reconcile-commutation-graph`
- `PRD-reconcile-search-planner`
- `ADR-structural-middle-pass-boundary`
- `PRD-reconcile-planner-audit-artifacts`
- `PRD-feature-file-claims`
- `PRD-record-capture-modes`
- `ADR-patch-amendment-policy`
- `PRD-feature-patch-amend`
- `PRD-active-feature-session`
- `PRD-agent-event-log`
- `PRD-ide-capture-hooks`
- `PRD-git-hook-capture-guards`
- `ADR-capture-context-privacy-boundary`
- `ADR-capture-metadata-branch`
- `PRD-record-context-summary`

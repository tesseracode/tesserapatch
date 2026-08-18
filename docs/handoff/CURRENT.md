# Current Handoff

## Status

**Cluster state**: IDLE

`implement-prepare-check` is **ACCEPTED** at implementation tip `cacaaf8`
(close/tracking commit on `main`), WAVE_BASE `9a8c1d0`, range
`9a8c1d0..cacaaf8`. Rev-4 internal review APPROVED; rev-4 external review
APPROVED with two LOW nonblocking AVP-175 parser notes and no product finding.
CI run
[32101270327](https://github.com/tesseracode/tesserapatch/actions/runs/32101270327)
is green on Ubuntu, macOS and Windows.
[GH #16](https://github.com/tesseracode/tesserapatch/issues/16) is closed. **No
release tag** — this prerequisite ships with the later mutating-prepare
release. Full archive: `docs/handoff/HISTORY.md` → 2026-08-17.

## Active Task

- **Task ID**: `implement-prepare-intent-bundle`
- **Issue**: none yet — must be opened before dispatch
- **Description**: Implement the mutating `tpatch prepare <slug>` intent-bundle
  contract from the accepted `PRD-prepare-intent-bundle` rev-14 +
  `ADR-035-intent-bundle-publication-and-history` rev-14 (ADR-035 normative
  where they overlap).
- **Status**: **Not Started — awaiting dispatch**
- **Assigned**: unassigned
- **WAVE_BASE**: none yet — record `git rev-parse origin/main` immediately
  before the first implementer is dispatched
- **Release tag**: TBD; the accepted `prepare --check` prerequisite will ship
  with this release

## Prerequisite Status

PRD §19's three acceptance conditions are now all satisfied:

1. `PRD-prepare-intent-bundle` Accepted at rev-14 (2026-08-14).
2. `ADR-035` Accepted at rev-14 (2026-08-14), reviewed jointly with the PRD.
3. §19(3) — the accepted `prepare --check` contract
   (`PRD-artifact-validation-and-provenance` rev-5 / rev-6 errata + `ADR-034`
   rev-2 / rev-3 errata) is **implemented, accepted and landed** as of
   `cacaaf8`.

Implementation is therefore unblocked. **The PRD's implementation slices and
their required sequence must be re-read in full before dispatch** — this
handoff deliberately does not restate them, and the slice partition, not this
file, is the dispatch authority.

## Backlog

- [GH #17](https://github.com/tesseracode/tesserapatch/issues/17) — open,
  **non-blocking**: the pre-existing `windows-latest` full-suite failures
  (200 top-level / 283 including subtests, six packages) stay visible behind
  one `continue-on-error` step that names the issue. AVP-175 pins the demotion
  to exactly one step, the exact literal `true`, and the Windows full-suite
  step; removing it when GH #17 lands is a deliberate edit, not a silent one.
- Two LOW AVP-175 parser follow-ups from the rev-4 external verdict: no
  flow-mapping step decoding, and a first-match (not uniqueness-proving)
  decoy-leaf floor. Address them whenever `.github/workflows/ci.yml` or
  AVP-175 is next edited.
- `GOOS=js GOARCH=wasm go build ./cmd/tpatch` fails in `internal/rescap` at
  `WAVE_BASE` unchanged; unticketed, out of scope of any prepare wave.
- [GH #12](https://github.com/tesseracode/tesserapatch/issues/12),
  [GH #13](https://github.com/tesseracode/tesserapatch/issues/13),
  [GH #14](https://github.com/tesseracode/tesserapatch/issues/14),
  [GH #15](https://github.com/tesseracode/tesserapatch/issues/15) — parked
  research backlog; no implementation or architecture decision authorized.

## Next Steps

1. Open the tracking issue for `implement-prepare-intent-bundle` and link it
   from the ROADMAP row and from this handoff.
2. Record a fresh `WAVE_BASE` (`git fetch origin && git rev-parse origin/main`)
   **before** the first implementer is dispatched, and put it in the dispatch
   brief so `make wave-close-check WAVE_BASE=<sha>` is scoped correctly.
3. Re-read `PRD-prepare-intent-bundle` rev-14 and `ADR-035` rev-14 and derive
   the slice partition and sequence from them.
4. Dispatch **sequentially**: the mutating prepare surface overlaps
   `internal/cli/cobra.go`, `internal/cli/prepare.go` and `internal/intent/`,
   and same-file overlap is a hard trigger for sequential execution
   (`AGENTS.md` → Parallel-Implementer Discipline). Explicit-path staging only.
5. Run the mutating implementation → joint internal/external review cycle to
   acceptance, then plan the release that carries both this wave and the
   accepted `prepare --check` prerequisite.

## Blockers

- None. No wave is in flight.

## Context for Next Agent

- `internal/intent` must not import `internal/store`; the status schema is
  mirrored locally on purpose and kept honest by the AST parity guard.
- The guard registry is the single source of truth for the 43 `G` rows. Adding
  a matrix row whose Kind contains `G` without registering a guard fails
  AVP-139 and the ledger automatically.
- Routing goldens must never be re-recorded from the current binary.
  Reconstruct the `WAVE_BASE` binary in a temporary detached worktree
  **outside** the repository, exactly as
  `internal/cli/testdata/routing-goldens/README.md` documents.
- `prepare --check` is read only by contract. The mutating wave adds new modes
  alongside it; it must not reopen the accepted read-only contract, and
  ADR-034's rooted boundary is a **read** boundary that ADR-035 explicitly does
  not extend to writes.
- The untracked research WIP in `git status` predates these waves and is
  covered by `.wave-close-allowlist`. Do not touch it.

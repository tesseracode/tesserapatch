# GH #23 prepare-intent-bundle pre-change goldens

These 51 deterministic `.txt` fixtures are the pre-change compatibility
baseline required by `PRD-prepare-intent-bundle` rev-14 section 17.2. They
cover PIB-186, PIB-198 through PIB-212, PIB-286, PIB-287, doctor D1 through
D8, and the PIB-391 provenance guard.

## Provenance

| Field | Value |
|---|---|
| GH issue | #23 |
| Recorded baseline | `95cab04c481201675bb42263110d4711111c8d6d` |
| Accepted `prepare --check` baseline | `cacaaf867ebde100b699e20d76010f92316afc72` |
| `WAVE_BASE` | `3b579fc7243bf0d1b21605d3c87562226f1fd936` |
| Accepted routing baseline | `9a8c1d049bb973ccf377bd9f0fa67d7080d2d773` |
| Build | `go build -buildvcs=true -trimpath -ldflags "-X github.com/tesseracode/tesserapatch/internal/buildinfo.Version=prepare-pib-golden" ./cmd/tpatch` |
| Recorder binary env | `TPATCH_PREPARE_PIB_GOLDEN_BIN` |
| Record switch env | `TPATCH_RECORD_PREPARE_PIB_GOLDENS=1` |
| Exact-SHA acknowledgement | `TPATCH_PREPARE_PIB_GOLDEN_SHA=95cab04c481201675bb42263110d4711111c8d6d` |
| Accepted-check binary env | `TPATCH_PREPARE_CHECK_GOLDEN_BIN` |
| Accepted-check acknowledgement | `TPATCH_PREPARE_CHECK_GOLDEN_SHA=cacaaf867ebde100b699e20d76010f92316afc72` |

`95cab04` is a valid pre-production baseline because its complete delta from
`WAVE_BASE` is limited to `docs/ROADMAP.md`, `docs/handoff/CURRENT.md`, and
`docs/supervisor/LOG.md`. No production file changed. The test checks that
three-path delta directly. Record mode requires both the exact-SHA
acknowledgement above and Go build information with non-empty
`vcs.revision=95cab04c481201675bb42263110d4711111c8d6d` and
`vcs.modified=false`. Comparison mode refuses `TPATCH_PREPARE_PIB_GOLDEN_BIN`
and `TPATCH_PREPARE_CHECK_GOLDEN_BIN`, and always builds current code itself.
PIB-198 through PIB-207 use a second binary whose VCS revision is the accepted
read-only implementation tip `cacaaf8`, not a binary built by GH #23.

## Recording procedure

Create a detached full clone outside this repository, build the baseline with
`-trimpath`, drive it from the new test code in the current worktree, then
remove it. A full clone is deliberate: Go 1.26 does not emit VCS settings for
this repository's linked-worktree checkout, while record mode requires the
binary itself to attest its exact clean revision.

```sh
set -euo pipefail
WORK="$(mktemp -d "${TMPDIR:-/tmp}/tpatch-gh23-goldens.XXXXXX")"
trap 'rm -rf -- "$WORK"' EXIT
BASELINE_DIR="$WORK/tpatch-gh23-baseline"
BASELINE_BIN="$WORK/tpatch-95cab04"
CHECK_DIR="$WORK/tpatch-gh16-check-baseline"
CHECK_BIN="$WORK/tpatch-cacaaf8"
test ! -e "$BASELINE_DIR" && test ! -e "$BASELINE_BIN"
test ! -e "$CHECK_DIR" && test ! -e "$CHECK_BIN"
git clone --no-hardlinks --quiet "$PWD" "$BASELINE_DIR"
git -C "$BASELINE_DIR" checkout --quiet --detach \
  95cab04c481201675bb42263110d4711111c8d6d
git clone --no-hardlinks --quiet "$PWD" "$CHECK_DIR"
git -C "$CHECK_DIR" checkout --quiet --detach \
  cacaaf867ebde100b699e20d76010f92316afc72

(cd "$BASELINE_DIR" && go build -buildvcs=true -trimpath \
  -ldflags "-X github.com/tesseracode/tesserapatch/internal/buildinfo.Version=prepare-pib-golden" \
  -o "$BASELINE_BIN" ./cmd/tpatch)
(cd "$CHECK_DIR" && go build -buildvcs=true -trimpath \
  -ldflags "-X github.com/tesseracode/tesserapatch/internal/buildinfo.Version=prepare-pib-golden" \
  -o "$CHECK_BIN" ./cmd/tpatch)
TPATCH_PREPARE_PIB_GOLDEN_BIN="$BASELINE_BIN" \
  TPATCH_PREPARE_CHECK_GOLDEN_BIN="$CHECK_BIN" \
  TPATCH_RECORD_PREPARE_PIB_GOLDENS=1 \
  TPATCH_PREPARE_PIB_GOLDEN_SHA=95cab04c481201675bb42263110d4711111c8d6d \
  TPATCH_PREPARE_CHECK_GOLDEN_SHA=cacaaf867ebde100b699e20d76010f92316afc72 \
  go test ./internal/cli -run '^TestPreparePIBPreChangeGoldens$'
```

A normal run leaves all record-only variables unset. It builds the current binary
with `-trimpath` and VCS metadata enabled, captures the same surfaces, and
compares every byte. Both builds inject the fixed presentation version
`prepare-pib-golden`; this keeps `added_by_tool_version` byte-stable without
disabling or weakening the VCS build information used for provenance.

## Normalization

The recorder uses a fresh repository, HOME, and XDG configuration for every
scenario; provider credentials are never inherited and
`TPATCH_NO_AUTO_DETECT=1` prevents localhost services from becoming fixture
inputs. Git author, committer, dates, branch, and content are fixed. After
`init` and `add`, the six init-managed skill assets are removed from the
fixture tree before the initialized tpatch workspace is committed. This keeps
the mandatory S6 asset edits from changing unrelated commit IDs while later
land/reconcile fixtures still run from a clean tracked baseline.
Feature identity is deterministic (`id == slug`), and `requested_at`,
`updated_at`, and request `**Created**` begin at `2000-01-02T03:04:05Z`.
It normalizes only:

1. the per-scenario absolute repository root to `<workspace>`;
2. named wall clocks legitimately refreshed by a command (`updated_at`,
   `generated_at`, `verified_at`, `captured_at`, `recorded_at`, and
   `**Recorded**`) to `<wall-clock>`;
3. the environment-owned `git_version` report value to `<git-version>`;
4. only D3's current `bundled sha256=` value to the literal
   `bundled sha256=<12hex>`, while retaining the real installed digest and
   finding.

Resource IDs, batch IDs, Git object IDs, semantic status fields, artifact
content (including `added_by_tool_version`), stdout, stderr, and exit codes are
otherwise preserved. Every runtime fixture snapshots the complete persistent
command-owned set: `.tpatch/FEATURES.md`, `.tpatch/upstream.lock`, the full
feature directory, and `.tpatch/local/resource-scratch/<slug>` (including its
persistent `.lock` when present), plus the future
`.tpatch/local/intent-prepare/<slug>` journal lane. The harness separately asserts immutable `id`/`requested_at`,
expected `updated_at` mutation for phases/manual/record, and byte-identical
status/write sets for read-only surfaces.

PIB-198 through PIB-207 use eight `check-*` fixtures from the accepted
`cacaaf8` binary: ready, not-ready and abort human/JSON populations plus ready
with a pending future journal. The pending/non-pending pairs are asserted
byte-identical. The pending fixture carries a nonempty, plan-digest-bound
J1–J10 journal with supporting stage/archive bytes and mode `0600`, rather
than an existence-only marker. Every human and JSON invocation receives its
own before/after snapshot of the complete workspace (including `.git`, modes,
directories and symlink targets) plus hermetic HOME.

PIB-209 is deliberately not re-recorded: it maps to the immutable accepted
`routing-goldens/cycle-*` fixtures. That cycle guard pins the SHA-256 of all twelve
accepted routing `.txt` files plus their README, requires that README to name
accepted WAVE_BASE `9a8c1d0`, and pins each path's complete Git history:
`2cbccf63529309bce17f181053816fadfdcb112a` for every fixture and README
creation, plus `36f23b38c6d80234ea2924ec3d7cf0d1d5087f29` only for the README,
all within `9a8c1d0..cacaaf8`.

PIB-391 requires record mode to prove the `check-*` producer binary's non-empty
`vcs.revision` is exactly `cacaaf8` and `vcs.modified` is false. Supplying the
GH #23 baseline or current binary in that slot refuses.
The accepted GH #16 executable check-test set is derived from every surviving
`*_test.go` path touched under `internal/intent`, `internal/cli`, and `assets`
in `9a8c1d0..cacaaf8`, then checked against a closed 21-path manifest with
accepted-tip SHA-256 values and accepted-range histories. Current bytes remain
frozen except `assets/assets_test.go`, which S6 must extend; its accepted-tip
blob and history remain pinned. This includes the PIB-206 race fixtures rather
than only the CLI examples. The new `check-*` files' first Git appearance must
precede every mutating production path touched by **any commit** from
WAVE_BASE; an edit-then-revert production-first sensitivity is mandatory. The
committed files carry no cryptographic producer attestation, so permanent
producer re-audit rebuilds `cacaaf8` and repeats the guarded record procedure.

`resource-unsupported-platform.txt` is a committed native runtime transcript,
not Darwin-generated output. Darwin validates the source/build-tag/manifest
guards, the closed `internal/rescap/lock_unsupported.go` Git history, and
Windows cross-compilation. The build-tagged
`prepare_pib_golden_windows_test.go` creates a valid resource declaration,
invokes the real CLI boundary on Windows, normalizes only the workspace path,
and byte-compares exit/stdout/stderr with that fixture.
The 50 supported-runtime fixtures skip on targets where resource capture is
unsupported; Linux and Darwin execute them, while the native Windows row owns
that platform's refusal.

## Manifest

| File | Surface | Rows |
|---|---|---|
| `check-ready-human.txt` | ready `prepare --check`, human | PIB-198, PIB-200, PIB-201, PIB-203 |
| `check-ready-json.txt` | ready `prepare --check --json` | PIB-199…PIB-205 |
| `check-not-ready-human.txt` | not-ready `prepare --check`, human | PIB-198, PIB-200, PIB-201, PIB-203 |
| `check-not-ready-json.txt` | not-ready `prepare --check --json` | PIB-199…PIB-205 |
| `check-abort-human.txt` | feature-not-found abort, human | PIB-198, PIB-200, PIB-201, PIB-204 |
| `check-abort-json.txt` | feature-not-found abort, JSON | PIB-199…PIB-205 |
| `check-ready-pending-human.txt` | ready check with pending journal, human | PIB-207 |
| `check-ready-pending-json.txt` | ready check with pending journal, JSON | PIB-207 |
| `next-requested.txt` | `next`, requested, text + harness-json | PIB-208 |
| `next-analyzed.txt` | `next`, analyzed, text + harness-json | PIB-208 |
| `next-defined.txt` | `next`, defined, text + harness-json | PIB-208 |
| `next-defined-exploration-present.txt` | `next`, defined with exploration, text + harness-json | PIB-208 |
| `next-implementing.txt` | `next`, implementing, text + harness-json | PIB-208 |
| `next-applied.txt` | `next`, applied, text + harness-json | PIB-208 |
| `next-active.txt` | `next`, active, text + harness-json | PIB-208 |
| `next-reconciling.txt` | `next`, reconciling, text + harness-json | PIB-208 |
| `next-reconciling-shadow.txt` | `next`, reconciling-shadow, text + harness-json | PIB-208 |
| `next-blocked.txt` | `next`, blocked, text + harness-json | PIB-208 |
| `next-upstream-merged.txt` | `next`, upstream_merged, text + harness-json | PIB-208 |
| `next-rejected.txt` | `next`, rejected, text + harness-json | PIB-208 |
| `next-unapplied.txt` | `next`, unapplied, text + harness-json | PIB-208 |
| `phase-auto-analyze.txt` | no-provider analyze output + feature tree | PIB-186, PIB-210 |
| `phase-auto-define.txt` | no-provider define output + feature tree | PIB-186, PIB-210 |
| `phase-auto-explore.txt` | no-provider explore output + feature tree | PIB-186, PIB-210 |
| `phase-auto-implement.txt` | no-provider implement output + feature tree | PIB-210 |
| `phase-manual-analyze.txt` | manual analyze + pre-authored artifact/tree | PIB-211 |
| `phase-manual-define.txt` | manual define + pre-authored artifact/tree | PIB-211 |
| `phase-manual-explore.txt` | manual explore + pre-authored artifact/tree | PIB-211 |
| `phase-manual-implement.txt` | manual implement + pre-authored recipe/tree | PIB-211 |
| `compat-status.txt` | status on a never-prepared feature | PIB-212 |
| `compat-verify.txt` | verify checks on a recorded applied, never-prepared feature | PIB-212 |
| `compat-record.txt` | record on a never-prepared feature | PIB-212 |
| `compat-land.txt` | land dry-run on a recorded, never-prepared feature | PIB-212 |
| `compat-reconcile.txt` | reconcile against deterministic `HEAD` on an eligible recorded, never-prepared feature | PIB-212 |
| `doctor-D1.txt` | deterministic D1 finding | section 17.2 |
| `doctor-D2.txt` | deterministic D2 finding | section 17.2 |
| `doctor-D3.txt` | deterministic D3 finding | section 17.2 |
| `doctor-D4.txt` | deterministic D4 finding | section 17.2 |
| `doctor-D5.txt` | deterministic D5 finding | section 17.2 |
| `doctor-D6.txt` | deterministic D6 finding | section 17.2 |
| `doctor-D7.txt` | deterministic D7 finding | section 17.2 |
| `doctor-D8.txt` | selected empty D8 check, checks_run=1 | section 17.2 |
| `resource-add.txt` | resource add | PIB-286 |
| `resource-list.txt` | resource list | PIB-286 |
| `resource-remove.txt` | exact-ID resource remove that proves removal | PIB-286 |
| `resource-clear.txt` | resource clear | PIB-286 |
| `resource-trust-dolt.txt` | exact git-metadata ID reaches `resource-not-dolt-adapter` | PIB-286 |
| `resource-capture.txt` | resource capture without Dolt | PIB-286 |
| `resource-diff.txt` | changed ignored-file resource diff | PIB-286 |
| `resource-contention.txt` | existing capture contention refusal | PIB-286 |
| `resource-unsupported-platform.txt` | native Windows exact unsupported lock refusal/code | PIB-287 |

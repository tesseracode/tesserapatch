# GH #13 S0 frozen baseline

Production baseline: `bbb7a4310b2ebea095890a284997fbbb42535bc7`.
Contract: accepted ADR-037 / ADR-043 and PRD rev-7, PRD section 8 S0.

The runtime goldens exercise the shipped reconciliation pipeline. They do not
implement or enable `operation_candidate_enabled`. Each framed transcript
contains complete session JSON, evidence JSONL, reconciliation Markdown,
ReconcileSummary JSON and lifecycle state. Exact byte comparison preserves
field presence, ordering, confidence, phases, verdicts and evidence identity.

## Determinism

Git author/committer identity, dates and object format are fixed in the fixture.
Git hashes and evidence/revision IDs are not normalized. The only normalized
runtime values are:

- the fixture repository root;
- the validated `.tpatch/shadow/<slug>-<timestamp>` path;
- the opaque resolver-session ID, whose shape and agreement between the
  reconcile result, status and resolver-owned session are checked first;
- named ReconcileSummary `attempted_at` and Markdown `Timestamp` wall clocks.

Semantic-field mutations must survive normalization and fail byte comparison.
Golden capture requires the explicit `TPATCH_UPDATE_ROC_S0_GOLDENS=1` test
environment and a passing test-only scope check against the baseline commit.
Capture is not a passing comparison: rerun without that variable afterward.
S0's scope guard deliberately rejects production/public-contract changes;
later slices must transition that guard explicitly when production work is
authorized rather than accidentally weakening this baseline.
The capture allowlist names only the five new S0 test files, this fixture
directory and tracking documents. Existing shared fixture builders cannot
change while new snapshots are captured.

Plain strict, applicable-only and mixed operation fixtures intentionally have
the same shipped output: applicable-only/mixed phase 2 emits no note today.
Their actual recipe presence and evaluator counts are asserted before pipeline
execution, with missing-recipe/dropped-operation controls, so identical output
does not let those input cohorts collapse silently.

## Future expectations, not shipped behavior

`candidate-expectations.json` separates current gate-off behavior from future
gate-on expectations. For the legacy all-present fixture, removing its legacy
recipe is a test-only oracle for the later phase's outcome; it is not execution
of the future config gate, eligibility checks or proof carrier.

The adjacent-argument cases preserve both motivating variants from
`docs/state-of-the-art/case-studies/adjacent-cli-args-conflict-2026-08/reproduce.sh`:

- `delete-all` reuses the existing GH #15 `testdata/rga-s0/` promoted bytes.
- `between-delete-first` adds feature arguments between the original arguments,
  removes upstream's `--old-a`, and retains `--old-b` after the feature arguments.

The fixture validator checks that each desired result is exactly one prescribed
insertion into the upstream bytes, and real Git still reports the original
textual conflict. This pins the future success expectation without implementing
the later slice's alignment algorithm or claiming a candidate can run today.

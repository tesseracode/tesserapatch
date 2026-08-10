# Review Playbook

How to review incoming work. `AGENTS.md` defines the review *process* (roles,
verdict vocabulary, wave-close gate). This file defines the review *technique* —
what to actually do to find problems.

Every rule below earned its place by catching a real defect in this repo. The
case that produced it is cited so you can judge whether it applies.

---

## 0. Core principle

**Verify claims; do not trust them.** A handoff saying "all §6 criteria MET", a
commit titled "fix X", or a summary saying "docs-only" is a *claim under review*,
not evidence. Almost every finding in this repo's history came from checking a
claim that everyone else accepted.

Corollary: when you cannot verify something, **say so explicitly** rather than
implying you did.

---

## 1. Standard sequence

Run these in order. Stop and report if step 1 or 2 fails.

### 1.1 Validate the range before reviewing it

```sh
git rev-parse --verify <base> && git rev-parse --verify <head>
git merge-base --is-ancestor <base> <head>; echo $?   # 0 = valid range
git log --oneline <base>..<head>
git diff --stat <base>..<head>
```

*Why*: ranges given in prose are often wrong or wider than stated. A "docs-only
cluster" once spanned two clusters, the earlier of which shipped `Makefile`,
`AGENTS.md`, and a new config file.

Verify scope claims mechanically:

```sh
git diff --stat <base>..<head> -- ':!docs/**'   # empty ⇒ docs-only confirmed
```

### 1.2 Check durability

```sh
git rev-list --count origin/main..HEAD   # 0 = pushed
git status --short | grep -vE '^\?\?'    # empty = nothing uncommitted
git tag --list 'vX.Y*'
git ls-remote --tags origin 'vX.Y*'      # annotated tag needs the ^{} entry
```

*Why*: three consecutive waves shipped "approved" work that existed only on one
machine. A release that is committed and pushed but **untagged**, or tagged
locally but not pushed, is the same failure class.

### 1.3 Read production code before tests

Ask "does this acceptance criterion have a code path at all?" *before* "is it
tested?"

*Why*: a `corrupt_entries` JSON envelope was reported as a LOW test-coverage gap.
Reading the production path first showed the envelope **did not exist** — a HIGH
blocking gap.

### 1.4 Check wiring, not just definition

A validator that exists but is never called enforces nothing.

```sh
grep -rn "FuncName" internal/ cmd/ | grep -v '_test.go'
```

*Why*: used to confirm `ErrMultipleActiveSuperseders` actually fires from the
`feature deps` write path and verify V4 — not just from tests.

### 1.5 Empirically reproduce user-facing behavior

Build the binary and run it in a scratch workspace that resembles a *user's*
repo, not this one.

```sh
go build -o /tmp/tp ./cmd/tpatch
cd /tmp && rm -rf probe && mkdir probe && cd probe
git init -q . && git commit -q --allow-empty -m init
/tmp/tp init >/dev/null 2>&1
# ... exercise the surface under review
```

*Why*: this is the single highest-yield technique. It caught D6 flagging every
upstream tag as drift in an ordinary user repo; confirmed session writers refuse
without a gitignore rule and leave no artifact behind; and proved label ordering
was severity-first rather than alphabetical.

### 1.6 Prove a regression test actually catches the bug

Apply the new test to pre-fix code. If it passes there, it guards nothing.

```sh
git worktree add -q --detach /tmp/prefix <pre-fix-sha>
cp path/to/new_test.go /tmp/prefix/path/to/
cd /tmp/prefix && go test ./pkg -run TestName    # must FAIL
git worktree remove --force /tmp/prefix
```

*Why*: used to confirm the verify V7/V8 double-apply test genuinely failed before
the shadow-reset fix.

### 1.7 Cross-check tracking docs against the artifacts they describe

When a cluster edits `ROADMAP.md`, `CURRENT.md`, `HISTORY.md`, or a CHANGELOG
entry, treat that prose as a claim about the artifacts the same cluster shipped —
and verify it. Symbol names in a dispatch brief are the highest-risk case: grep
each one against the accepted paper.

```sh
git diff --stat <base>..<head> -- 'docs/ROADMAP.md' 'docs/handoff/**' 'CHANGELOG.md'
grep -c '<SymbolFromSummary>' docs/adrs/ADR-0NN-*.md docs/prds/PRD-*.md   # 0 ⇒ drift
```

*Why*: a ROADMAP dispatch brief named `UnappliedStatus` and a Rule-7-parallel
`ErrUnappliedParent` — neither appears in the ADR it summarizes, and the ADR's
test matrix had affirmatively decided edge creation onto that parent is *allowed
(no Rule-7-analog)*. Papers converge over several revisions; the summary written
*about* them often is not re-verified against the final text.

### 1.8 Run the gates

```sh
gofmt -l .            # never piped — must be run directly
go vet ./...
go build ./cmd/tpatch
go test -count=1 ./...
make wave-close-check
```

---

## 2. Recurring defect classes

Check each on every review touching the relevant surface.

| Class | What to check | Case that produced it |
|---|---|---|
| **Totality claims** | Any "only X supported" / "V0–V9" / "the full list is Y" statement, re-verified against *all* layers | Help text claimed `V0-V9` after `V10` shipped; a flag table said "only the flags shown" while root persistent `--path` was inherited |
| **Display-string contracts** | Exact rendered output, incl. ordering and interpolated values | `superseded-by` shipped without its `<slug>`; labels sorted alphabetically instead of the PRD-locked severity order |
| **Fix completeness** | Did the fix cover *every* instance, or just the one observed? | A CI fix pinned one test helper; a gc-pin was applied to one package when three spawn git |
| **Undisclosed behavior change** | Diffs to shared loaders/helpers that alter a *shipped* command's behavior | A lenient loader change silently made `reconcile review list` exit non-zero on a file lacking a trailing newline — no criterion, no CHANGELOG line, no test |
| **Repo-relative references in runtime output** | Remediation strings naming files that exist only in this repo | D6 told users to "follow RELEASING.md", which `tpatch init` never installs — the defect class ADR-020 already banned for skill assets |
| **Ordering around guards** | Whether a write happens *before* the precondition that guards it | `SaveContextSummary` ran before the D6 ignore-contract check — a passing unit test did not reveal it |
| **Audit-trail integrity** | After any history rewrite, whether cited SHAs still resolve | The supervisor log cited two commits unreachable from `main` and never pushed |
| **Config artifacts as dependencies** | A gate reading a file that is untracked ⇒ behaves differently elsewhere | `.wave-close-allowlist` was untracked *and* invisible to the sentinel that consumed it |
| **Summary-vs-source drift** | Tracking prose (ROADMAP / handoff / CHANGELOG) contradicting the artifact it describes — especially symbol names in a dispatch brief | A ROADMAP brief prescribed `ErrUnappliedParent` / `UnappliedStatus` against an ADR that defines neither; a handoff called a *changed* lenient loader "existing" |

---

## 3. Traps that produce false findings

These are mistakes made *while reviewing*. Each one nearly produced a wrong
report.

1. **Pipeline exit codes.** `cmd | tail` returns `tail`'s status. Capture the
   real one:
   ```sh
   cmd >/dev/null 2>&1; echo "EXIT=$?"
   ```
   Nearly reported a passing exit-3 guard as broken.

2. **Shell semantics differ from `make`.** `make` recipes run under `sh`, which
   word-splits unquoted expansions; **zsh does not**. Replicate Makefile logic
   with `sh -c '...'`, never interactively in zsh. This produced a false
   "allowlist mechanism is broken" signal.

3. **Assertion failure vs teardown failure.** A `FAIL` may be a `t.TempDir()`
   cleanup race, not a broken assertion. Re-run the single test in isolation
   before calling it a regression:
   ```sh
   go test -count=1 -run 'TestName' ./pkg
   ```

4. **Environment-dependent tests.** Distinguish "fails here" from "fails
   everywhere" — check whether the test depends on `init.defaultBranch`, gc
   behavior, or filesystem timing before attributing it to the cluster.

5. **Grep scope.** A missing hit may mean the wrong include pattern, not absent
   code. Confirm with a second, broader search before declaring something
   nonexistent.

---

## 4. Verifying a revision

When a rev-N lands against prior findings:

1. Re-verify **each** finding independently — do not trust the closure summary.
2. Prefer the same empirical method that exposed it originally.
3. Confirm the fix did not introduce a new problem (a rev-2 once fixed a false
   help-text claim by introducing a new totality claim).
4. Check that behavior-locking tests were **updated, not deleted** — renamed with
   inverted assertions is correct; missing is not.
5. Check for a residual: the fix may be correct but incompletely applied
   (a D6 refusal was fixed at one writer, leaving another writing before the
   check).

---

## 5. Severity and verdicts

**Severity**

- **BLOCKING** — contract violated, data loss, or a shipped surface broken.
- **HIGH** — acceptance criterion unmet, or a user-visible contract wrong.
- **MEDIUM** — real defect with a workaround, or an undisclosed behavior change.
- **LOW** — accuracy/consistency issue with no functional impact.
- **Observation** — no defect; context worth recording.

Do not inflate. A precisely-scoped MEDIUM with evidence carries more weight than
a speculative HIGH.

**Verdicts**

- **APPROVED** — no findings above LOW, or LOWs that need no action.
- **APPROVED WITH NOTES** — sound work; findings should be folded into the next
  wave rather than blocking this one.
- **NEEDS REVISION** — at least one finding must land before close.

---

## 6. Report format

1. **What it is** — scope and commit span, stated plainly.
2. **Prior findings** — closed / partially closed / open, with evidence.
3. **Findings** — severity, `file:line`, the evidence, and the suggested fix.
4. **Validation** — gates, tests, invariants; what passed *and what you could
   not complete*.
5. **Verdict**, repeated as the final line.

Cite `file:line` for every claim. Quote real command output rather than
paraphrasing it. If verification was incomplete, state exactly which check did
not run and why.

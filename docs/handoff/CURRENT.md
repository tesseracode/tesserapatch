# Current Handoff

## Status

**Cluster state**: IN PROGRESS

`implement-prepare-intent-bundle` is dispatched from WAVE_BASE `3b579fc` under
[GH #23](https://github.com/tesseracode/tesserapatch/issues/23). The accepted
rev-15 PRD + ADR-035 are authoritative, with strict implementation order
`S1b → S1 → S3 → S4 → S4b`, pre-change goldens before producer refactors,
then S5/S6 and sequential S7 hardening. No release tag is authorized before
joint acceptance. Golden implementation review discovered that rev-14
PIB-391 required standalone `prepare --check` output files to have been
committed by GH #16, but GH #16 committed no such paths. Rev-15 corrects only
that impossible evidence predicate. Rev-15 review round 0 returned internal
NEEDS REVISION and external APPROVED WITH NOTES; round 1 aligned ADR-035
D14, the rev-15 amendment ledger, shipped-prerequisite prose, producer
auditability and fixture-before-production sensitivity. Joint re-review
returned internal APPROVED and external APPROVED WITH NOTES; the sole
`cacaaf8` terminology note is folded. Rev-15 is accepted and the
pre-production golden baseline landed at `f9208c7`, with its Git-maintenance
race pin at `977b9d5`. CI run
[32178723042](https://github.com/tesseracode/tesserapatch/actions/runs/32178723042)
is green on Ubuntu, macOS and Windows; the dedicated GH #23 native Windows
resource golden is blocking and passed.

S7 AV is durably accepted at `b6b1709`: CI
[33166330402](https://github.com/tesseracode/tesserapatch/actions/runs/33166330402)
passed all three platforms and both observer jobs. Cumulative S7 is 157/173.
S7 AW (`PIB-552`…`PIB-560`, nine rows) is now dispatched sequentially.
AW rev-2 closes both review rounds and is **APPROVED**: remaining-stage rendering no longer
mislabels future stages as immediate retries, multi-class repair procedures
are emitted as separate command blocks rather than shell pipelines, the
tokenizer derives commands from emitted physical lines, and the AP declaration
pin follows the reviewed function body. The exact AW suite, observer, vet,
build and full non-observer suite pass. Checkpoint and blocking CI are next.
AW production, exact tests and observer/CI guards are checkpointed at
`0d0cc25`.
AW is durably accepted at `31192a0`: CI
[33177631054](https://github.com/tesseracode/tesserapatch/actions/runs/33177631054)
passed all three platforms and both observer jobs. Cumulative S7 is 166/173.
S7 AX (`PIB-561`…`PIB-567`, seven rows) is now dispatched sequentially.
AX rev-0 is implemented in the worktree with no production change. Static
implementation exposed three contract-record inconsistencies requiring
adjudication before acceptance: PIB-562 assigns doctor exit 3 despite D9's
accepted warning-only exit 0; PIB-566 assigns doctor a retry_cwd field its
accepted carrier does not own; and §10.2 illustrates an admitted exit-0 report
with a rank-1 corrupt-object stage that rev-13 makes unreachable.
AX rev-2 closes the aggregate-target, JSON-carrier, sensitivity-anchor,
future-revision and quantification-scan defects found in execution/review.
Paired PRD/ADR rev-19 corrects the three record inconsistencies without a
product decision or behavior change. The exact AX/rev-19 guard suite passes;
joint final review is active.
Direct fallback review found no residual after the joint-review sub-agent
returned no usable verdict. Rev-19 is checkpointed at `0ecdcb1`; AX exact
tests and CI observer guards are checkpointed at `5e444d6`. The committed
rev-19 writer-diff guard passes. Blocking CI is next; final aggregate external
review remains before release authorization.
AX is durably accepted at `51382fc`: CI
[33198502426](https://github.com/tesseracode/tesserapatch/actions/runs/33198502426)
passed Ubuntu, macOS, Windows and both observer jobs. S7 is now **173/173**.
The aggregate body-sensitive 567-row ledger and complete G-row sensitivity
registry are the active pre-release task.
Aggregate rev-0 is **NEEDS REVISION**: it resolves only 490/567 ledger rows and
72/123 G sensitivities, its matrix parser misorders PIB-392 as row 291, and
two supposedly live sensitivity mappings fail. Declared-blocked rows are not
an acceptance mechanism; rev-1 must reach 567/567 and 123/123 with zero
blocked entries.
Aggregate rev-1 reaches zero declared blocks and its 123/123 G sensitivity
registry passes, but the 567-row body resolver rejects dynamic/nonexistent
subtest paths across grammar, intentpub journal/stage, AVP readiness, archive
decoder and refusal fixtures. Rev-2 must replace every failed path with a
literal resolvable body; the resolver stays unchanged.
Aggregate rev-2 passes the 567-row ledger, 123-row sensitivity registry and
all meta-gates, and all new non-CLI owning packages pass. Eleven CLI fixtures
still fail (`PIB-059`, `128`, `136`, `180`, `206`, `221`, `258`, `267`,
`294`, `295`, `356`), so the aggregate is not accepted.
Aggregate rev-3 passes every aggregate/owning fixture, but independent review
rejects the PIB-231 sensitivity half: ten golden fixtures compare recorded
bytes with themselves, seven rows borrow another validator's mutation, and the
shape gate lets sibling/arbitrary mutations qualify. The 567-row PIB-230
ledger is approved; rev-4 must correct all 17 G rows and make the same-validator
link mechanically biting.
Aggregate rev-4 is **APPROVED**: all 567 rows resolve to body-sensitive
runnable targets, all 123 G rows carry distinct same-validator wrong-input
proofs, blocked escapes are structurally forbidden, and the aggregate/new
owning suites pass. Final tracked-state validation is active.
Aggregate acceptance is checkpointed at `7d4981a`. The full uncached
non-observer suite passes (CLI 512.183s); race passes for the changed
aggregate/S5/golden shard (901.022s), the isolated shared S6 guard baseline
(307.001s), and all other touched packages. The monolithic CLI race attempt
hit its 40-minute cap in an older AP guard, so it was replaced by those
non-overlapping shards. Vet/build, Linux/Darwin/Windows/BSD CLI test-compiles,
Linux amd64/arm64, Darwin amd64 and Windows amd64 binary builds all pass.
Side Research remains `b385fe622db9926f48861105239f113e`.
Aggregate CI
[33230665925](https://github.com/tesseracode/tesserapatch/actions/runs/33230665925)
is green on all three platforms and both observer jobs at `6eb51c0`. The
complete GH #23 implementation range is now in final split review: production
correctness separately from contract/tests/docs/release readiness.
Production review found one latent `Pread` EINTR negative-offset panic; the
reviewed fix is committed at `1c5ad8b`. Release-readiness review then found
stale acceptance metadata, authorization/index/ROADMAP/handoff records and a
CHANGELOG graduation guard that would reject a versioned heading. The
non-tracking corrections are implemented and their targeted guards pass;
tracking reconciliation and post-fix CI remain.
Focused correction review is complete. The ROADMAP acceptance date is
2026-08-29, and the changelog guard now survives all three repository-standard
states: `Unreleased`, `vX.Y.Z (unreleased)`, and the final dated release
heading, including a later empty `Unreleased` section.
The paired readiness contract/index/guard corrections are checkpointed at
`223efb0`; production fix `1c5ad8b` immediately precedes them.
Final blocking CI
[33236724647](https://github.com/tesseracode/tesserapatch/actions/runs/33236724647)
is green on Ubuntu, macOS, Windows and both observer jobs at `30dbdba`.
GH #23 implementation and acceptance are complete; release authorization is
the only remaining boundary.
The first mechanical close attempt passed checks 1–7 but `internal/cli`
reached Go's 10-minute default during check 8 under mandated serial execution.
Blocking CI had passed by splitting the package into fresh processes; exact
CI-equivalent sharding is the sole active close correction.
The correction is reviewed: `scripts/wave-close-test-shards.sh` pins the Go
environment and runs the exact 22-command CI partition serially; Make invokes
it directly, and a sensitivity-guarded parity test covers command order,
environment, dry-run safety and the `scripts/**` forgotten-file sentinel.
`make wave-close-check
WAVE_BASE=3b579fc7243bf0d1b21605d3c87562226f1fd936` passes **8/8** at
`2df3346`: the tree is clean, HEAD pushed, all 96 wave commits carry the
trailer, terminal state parses, formatting/vet/build pass, and all 22 uncached
CI-equivalent test shards pass.
Post-close external review is APPROVED WITH NOTES and raises one valid LOW:
unknown or malformed archive selectors are correctly classified by the store
as `archive-selector-invalid`, but the CLI rewrites that code to
`archive-index-corrupt`, and unknown generations use the wrong store code.
A bounded rev-20 contract/code correction is active. The review's two carried
sensitivity notes are already closed by
`TestExecuteSemanticNoOpCASRejectsDriftBeforePublication` and
`TestPlanRejectsArtifactBoundToNoncanonicalPath`.
Rev-20 is now **accepted** after three correction reviews. The public code is
limited to well-formed unknown blob/generation IDs; malformed values retain
exit-1 usage behavior; typed refusal rendering no longer rewrites the code;
selector failures publish no lower-precedence archive observations. The
54-code catalog, PIB-431/PIB-465, PRD/ADR/SPEC/CHANGELOG and frozen guards are
aligned.
Focused store/CLI/catalog/frozen-document suites pass, including malformed
selector precedence and the archive-with-residue report-isolation fixture.
The full non-observer suite passed before that final isolation fold
(`internal/cli` 545.446s). A fresh local race/cross-build launch is deferred
because the authoritative resource gate fell to 69% free memory; blocking CI
will validate the final committed bytes without relaxing the 80% threshold.
Rev-20 implementation/tests are checkpointed at `fd8dd8b`; the accepted
contract/SPEC/CHANGELOG/ADR-index record is checkpointed at `9f7095c`.

S1b landed at `1f35605`; CI
[32185709105](https://github.com/tesseracode/tesserapatch/actions/runs/32185709105)
is green on Ubuntu, macOS and Windows. S1 landed at `f0ae54b`; CI
[32202082897](https://github.com/tesseracode/tesserapatch/actions/runs/32202082897)
is green on all three platforms. S3 landed at `4c3dbfe`; CI
[32220278819](https://github.com/tesseracode/tesserapatch/actions/runs/32220278819)
passed every Ubuntu, macOS and Windows test job (the release job was correctly
skipped). S2 landed at `16d614a`; CI
[32229096085](https://github.com/tesseracode/tesserapatch/actions/runs/32229096085)
is green on all three platforms.

S4 mutating prepare is complete at `49301eb`. Its primary implementation
landed at `5853ba7`; its first
blocking CI run
[32280073787](https://github.com/tesseracode/tesserapatch/actions/runs/32280073787)
failed deterministically on two pre-S4 source guards that only saw
`prepare_publish.go` after it became tracked. AVP-134 still admitted one
`internal/intent` importer instead of the accepted S4 pair, and AVP-141 still
expected one module-wide `os.OpenRoot` site instead of the frozen check site
plus S4's two mutating sites. The correction retains exact file/site
allowlists and sensitivity arms; no product behavior changes. Follow-up CI
[32281269945](https://github.com/tesseracode/tesserapatch/actions/runs/32281269945)
is green on Ubuntu, macOS and Windows.

S4b retention is complete at `e3099d5`; blocking CI
[32291924127](https://github.com/tesseracode/tesserapatch/actions/runs/32291924127)
is green on Ubuntu, macOS and Windows.

S5 doctor D9 rev-1 is implemented in the worktree and remains under revision.
Rev-0's confinement, non-destructive read-error and class-aggregation findings
are closed. Re-review found five MEDIUM residuals: a decoded pending hash is
not reported when an unrelated blob read fails; PIB-143/206/209 ledger targets
are static or unrelated rather than sensitive runtime/source coverage; the AST
resolver accepts non-`testing.T` `.Run` calls and unrelated table names; PIB-323
checks forbidden claims only in doctor text, not docs; and PIB-222 has no
native Windows runtime assertion.

S5 rev-2 closes all five rev-1 findings; production D9 now has no open review
finding. Re-review remains NEEDS REVISION on three ledger/test-quality
residuals: identifier shadowing can spoof receiver/table bindings in the AST
resolver; PIB-316…322 mappings omit required runtime observables; and
PIB-143/145 do not scan the exact authoritative forbidden-list/source/docs
surfaces their rows name.

S5 rev-3 folds those three residuals without changing D9 production: the
ledger binds lexical AST objects under receiver/table shadowing; PIB-316…322
use complete real CLI crash/recovery/orphan/fresh-state scenarios; and
PIB-143/145 scan their authoritative table/source/docs populations. The
implementation-discovered AVP PRD rev-7 erratum adds the one
`intent-archive/**` forbidden-inference row already required by accepted
ADR-035 and PIB-143; no decision, matrix row or guard count changes.

S5 rev-3 re-review found three MEDIUM test/ledger residuals: the lexical-object
resolver still reads only a table's declaration initializer and ignores a
later assignment before range; PIB-145's positive ADR-035 claims are
document-wide rather than bound to normative D2/current PRD sections; and
PIB-316/318/320…322 omit explicit byte-identity, post-rename, cleanup, doctor
and preceding-interruption observables. Production D9 and the rev-7 erratum
have no open finding.

S5 rev-4 closes the runtime-observable gaps. Re-review found two MEDIUM
resolver residuals: table mutation through an alias or inside the range body
can still make initializer names differ from executed subtests, and normative
section lookup can bind to a copied heading-delimited block in historical
prose. Both are test-only hardening; production D9 and the PRD erratum remain
clean.

S5 rev-5 closes alias/range-body table-object and normative hierarchy gaps.
Re-review found two precise MEDIUM parser residuals: direct string range
variables bypass the range-name stability check, and Markdown fence/ATX
parsing accepts false fence closers or headings indented as code. Both are
test-helper-only; production D9, runtime rows and PRD erratum remain clean.

S5 rev-6 closes the final direct-range and CommonMark parser edges. Focused
re-review returned APPROVED with all production, runtime, ledger and erratum
findings closed.

The first staged-state full suite found two pre-commit integration gaps:
AVP-134 did not yet include the accepted D9 `internal/intent` importer, and
PIB-146 inherited package-global provider configuration from earlier tests,
making its heuristic parity fixture non-hermetic. The bounded correction pins
the exact four-file importer set and isolates PIB-146's config home; product
behavior is unchanged.
Both corrections now pass the complete staged-state normal/race gate.

S5 doctor D9 is complete at `f7ccd61`; blocking CI
[32304087548](https://github.com/tesseracode/tesserapatch/actions/runs/32304087548)
is green on Ubuntu, macOS and Windows.

S6 public parity rev-0 is implemented in the worktree and under revision after
an independent NEEDS REVISION verdict. The main blocker is systemic: most of
the 31 acceptance-row IDs are shifted or mapped to unrelated observables, and
several totality/sensitivity guards cannot detect their required regressions.
Public prose also misstates bundle-vs-phase notes, abandon's Git exemption and
pending-purge terminal recovery, while the Unreleased changelog omits required
§12.6 deltas.

S6 rev-1 closes the systemic row-number drift and public prose/changelog
findings. Re-review remains NEEDS REVISION on eleven guard-quality residuals:
shadowed `testing.T` ledger binding; non-discriminating rename wrappers and
FIFO no-open proof; incomplete write-target/atomicity/JSON guards; incomplete
disposition/state/vocabulary totality; proxy advisory/refusal reachability;
non-biting seam assignment sensitivity; and synthetic rather than wired
golden provenance validation.

S6 rev-2 folds those eleven guard findings: exact rename/FIFO targets,
testing-parameter AST binding, mechanical write-target/schema/totality guards,
production-derived 18-advisory and 53-refusal catalogs, complete one-delta seam
sensitivities, and actual record/comparison golden wiring. Focused re-review is
pending.

S6 rev-2 re-review found four MEDIUM guard residuals: the write-sink inventory
omits descriptor-relative unlink cleanup; vocabulary extraction substitutes
expected order and scans a fixed function whitelist; refusal reachability
constructs desired reports instead of observing public fixtures; and broad
`not` handling can hide positive atomic-visibility overclaims.

S6 rev-3 closes the prior surface gaps but re-review found six deeper MEDIUM
issues: sink discovery remains filename/callee lexical rather than type/import
and dataflow aware; atomic negation is sentence-wide; vocabulary derivation
misses the real repair-order array and nonliteral assignments; catalog
fixtures inject expected classified results/errors; only JSON quiet rendering
is exercised; and emission discovery can miss struct literals/constant
propagation while counting declarations as emissions.

The type-aware rev-3 rebuild now passes every guard and real JSON/human catalog
fixture except deterministic public reachability of
`lock-filesystem-unsupported`. The real classifier runs before the existing
lock failure seam and this host has no denied filesystem. One micro-prerequisite
will expose an internal classifier-injection acquisition entry point, guarded
against non-test callers, so S6 can exercise the classifier and public mapping
without constructing a final typed error.

S6 rev-4 adds the real denied-filesystem fixture and closes catalog runtime
reachability. Re-review found four MEDIUM static-analysis residuals: leading
`Although ...,` concessive clauses can hide positive atomic claims; vocabulary
discovery omits Generator/Resume/Selector/Class/Kind/CWD wire fields; code
emission extraction still skips dynamic helper composites and uses
name/exclusion heuristics instead of emitter reachability; and sink taint loses
receiver type, bound OpenFile flags and syscall path arguments.

S6 rev-5 closes those four but re-review found four MEDIUM analyzer bypasses:
whole-function low-level adapter exemptions can hide a new sink; last-assignment
flow lets a later safe value mask earlier taint and misses compound flags/
method values; another concessive form (`Though`) hides positive visibility;
and tuple-result assignments or callable aliases can hide vocabulary/refusal/
advisory emissions.

S6 rev-6 closes those bypasses but re-review found five MEDIUM flow residuals:
delegated wrapper parameters can be reassigned before the exempted sink;
function-valued struct fields/parameters hide sink/emitter identities; generic
comma splitting breaks a single positive visibility claim; union-only flags
ignore unconditional assignment kills; and nested multi-result forwarding
leaves later tuple positions unresolved.

S6 rev-7 closes those flow cases but re-review found six MEDIUM analyzer edges:
closures, loop backedges and switch fallthrough are not in reaching state;
arbitrary string-return helpers and local helper aliases/method expressions
escape sink reachability; advisory callback factories are unresolved without
failure; named-result bare returns do not populate tuple flow; and quoted
prohibition examples are mistaken for asserted atomicity claims.

S6 rev-8 closes those but re-review found six MEDIUM soundness/performance
issues: deferred closures use declaration-time rather than exit-time state;
break/continue paths are treated as fallthrough; fixed-point exhaustion returns
partial state; method expressions preserve type aliases; fenced/escaped
prohibition examples lose directive context; and TestS6 regressed from ~358s
to ~1,661s without analyzer caching.

S6 rev-9 closes those with runtime restored to ~375s. Re-review found four
MEDIUM remaining constructs: named deferred functions/methods are ignored;
goroutine effects are applied synchronously; range/select produced variables
are unbound; and goto transfers never reach labels.

S6 rev-10 closes those but re-review found three MEDIUM semantic edges:
deferred selector/index/call arguments are not frozen at defer time; goroutine
pointer/container aliases taint only the pointer variable, not its pointee;
and named collection types or pointers to arrays are unresolved in otherwise
safe range domains.

S6 rev-11 closes those but re-review found two HIGH and one MEDIUM analyzer
residual: deferred argument side effects are discarded instead of applied
left-to-right; pointer aliases hidden by `any(...)`/anonymous-interface
conversions escape goroutine taint; and goroutine capture analysis taints
read-only or lexically shadowed variables by name.

S6 rev-12 closes those but re-review found two HIGH and one MEDIUM residual:
nested declared helper mutations inside goroutines are skipped; map/slice
backing aliases do not taint all aliases on element mutation; and deferred
boolean argument evaluation ignores `&&`/`||` short-circuit semantics.

S6 rev-13 closes those but re-review found one HIGH and two MEDIUM residuals:
reslices and channel-carried slices lose backing identity; append always
inherits old backing even when known len==cap guarantees allocation; and nested
ordinary short-circuit expressions still use the legacy eager call walk.

S6 rev-14 closes those but re-review found two HIGH and one MEDIUM residual:
the recursive effect cache treats selector/interface method and callback
mutations as pure; append growth assumes exact capacity although Go may
overallocate; and deterministic channel receives union all historical sends
instead of consuming FIFO state.

S6 rev-15 closes those but re-review found one HIGH and one MEDIUM residual:
ordinary resolved helper pointer/method mutations are ignored outside
goroutine/recursive-expression contexts, and consumed deterministic channel
values are retained in historical sends and reappear after a later transition
to nondeterministic concurrency.

S6 rev-16 closes those but re-review found one HIGH residual: transitive
helper/callback writes to package globals are recognized as mutations yet
skipped when no guarded argument is present because the global-write gate
examines only direct assignments.

S6 rev-17 closes that but re-review found three HIGH global-flow residuals:
writes through global pointer/interface/map/slice aliases do not taint their
pointees/backings; package `init` mutations are absent from seeded global
state; and imported package global selectors are not resolved.

S6 rev-18 closes those but re-review found two HIGH and one MEDIUM
package-initialization residual: global initializer values are resolved against
later state instead of frozen in dependency order and initializer call side
effects are omitted; synchronous callback APIs such as `sync.Once.Do` can hide
global writes; and impossible permutations of ordered init functions create
false unsafe final states.

S6 rev-19 closes those but re-review found two HIGH residuals: initializer
dependency ordering includes reads but not global writes/escapes performed by
initializer helpers; and repeated `sync.Once.Do` calls execute every callback
instead of tracking the Once object's done state.

S6 rev-20 closes those but re-review found four HIGH `sync.Once`/panic
residuals: method receiver identity is frozen after argument effects; global
pointer aliases receive independent Once identities; aggregate copies
containing a used Once are not rejected/propagated; and recovered panic paths
execute statements that are unreachable at runtime.

S6 rev-21 closes those but re-review found three HIGH and one MEDIUM residual:
recover state is shared across branches/panic episodes; used Once aggregates
can be copied through channel sends or deferred arguments; shadowed
`panic`/`recover` identifiers are treated as builtins; and deferred Once.Do
bypasses the Once state machine.

S6 rev-22 closes those but re-review found three HIGH residuals: a Once.Do
callback invoked by a deferred function incorrectly inherits direct-defer
recover privilege; builtin `copy` validates only source and can overwrite a
used Once destination; and range/map-key value copies omit Once copy/state
validation.

S6 rev-23 closes those but re-review found two HIGH and two MEDIUM
copy-boundary residuals: append reallocation copies existing used elements;
copy validation ignores zero/partial intervals; range validation ignores
whether only keys are bound; and generic union constraints can hide a nested
Once type.

S6 rev-24 closes those but re-review found three MEDIUM precision residuals:
full copy aliases destination Once identities to source instead of creating
independent value identities; zero/in-place append rejects existing used
elements even though no existing element is copied; and compatible generic
collection union terms lose their shared collection structure.

S6 rev-25 closes those but re-review found four MEDIUM copy/generic residuals:
pointer-element copy aliases destination slice backing to source; self-append
resolves through its post-assignment binding; generic instantiations discard
type arguments; and intersected embedded constraints lose compatible
collection shape.

S6 rev-26 closes those but re-review found three MEDIUM precision residuals:
pointer-element copy silently stops updating destination identity beyond 128
elements; selector/index self-appends are not frozen before assignment; and
generic function call results do not substitute inferred type arguments.

S6 rev-27 closes those but re-review found four MEDIUM generic type-system
residuals: generic receiver parameters are not substituted; partial explicit
type arguments cannot infer trailing parameters; variadic generic parameters
are unresolved; and method-only embedded constraints erase compatible
collection shape.

S6 rev-28 closes those but re-review found four MEDIUM generic-inference
residuals: untyped constants are prematurely defaulted; dependent constraints
are checked before substitution; exact terms are treated like `~` terms; and
method constraints compare names without full signatures. The next revision
will replace custom inference with authoritative `go/types` results.

S6 rev-29 replaces custom generic authority with `go/types`. Re-review found
one MEDIUM residual: the legacy partial-type-info gate accepts a package when
any one error is allowlisted, suppressing unrelated generic/constraint errors
in the same package.

S6 rev-30 closes that but re-review found one MEDIUM cache-key residual:
type-graph caching hashes sources but not the exact partial-error registration,
so a changed registration can reuse a previously accepted partial graph.

S6 rev-31 closes the final cache-key residual. Focused re-review returned
APPROVED with all 31 row mappings, 22 G sensitivities, type/dataflow/catalog
guards, SPEC/operator docs, Unreleased changelog and six-skill parity closed.

The first tracked-state full gate found test-integration rather than contract
failures: provider catalog fixtures inherited package-global config; repeated
catalog baselines pushed `internal/cli` past Go's 10-minute timeout; and the S5
AST ledger dereferenced a non-range selector from the new S6 test. S5
nil-safety is corrected; S6 must make provider fixtures hermetic and cache
immutable baseline catalog/analyzer evidence without weakening sensitivities.

Tracked-state integration is now closed. Provider-dependent fixtures use
isolated config homes and restore seams; immutable 18-advisory/53-refusal/
analyzer baseline evidence builds once with deep-copy returns. The older S5
ledger handles non-range selectors safely. Measured TestS6 is ~101s and the
complete `internal/cli` package ~281s, comfortably below Go's default timeout.

Final tracked-state validation passes: full uncached suite (`internal/cli`
261s), S6 race (`internal/cli` 395s), affected assets/intentlock/intentpub/
store/workflow race suites, gofmt, vet, host build and Linux amd64/arm64,
Darwin amd64 and Windows amd64 cross-builds.

S6 public docs/assets parity is complete at `65e876a`; blocking CI
[32447914694](https://github.com/tesseracode/tesserapatch/actions/runs/32447914694)
is green on Ubuntu, macOS and Windows.

S7 rev-0 is implemented in the worktree but independent review returned
NEEDS REVISION systemically. All 51 G sensitivities toggle a synthetic
`family-valid` fact instead of mutating guarded input; the full ledger accepts
unrelated aggregate wrappers; numerous CLI/publication rows omit their stated
modes/faults/reports; archive 531…566 use low-level plan proxies instead of
complete CLI/doctor repair ladders; authority/Git/D9 rows omit real process
state machines; Windows rows are incomplete/non-blocking; and PIB-567 compares
hard-coded revision surrogates rather than both real documents.

S7 rev-1 rebuilt AM exactly for PIB-395…401 and 403…409; PIB-402 exposed a
stale accepted-contract contradiction. Rev-5-era rows/prose say regeneration
rehydrates removal-pending references, while rev-8+ D10/D13/D16 make every
pending hash purge-owned and unreachable to regeneration. Current production
already obeys the later rule and rehydrates tombstones only. A bounded
PRD/ADR rev-16 erratum will amend §9.3 and PIB-402/403/425 without changing a
decision, row, kind or count; AM then completes before AN–AX continue.

S7 rev-2 completes AM–AO exactly: 54/173 rows (AM 15, AN 23, AO 16;
I16/C20/G13/U4/S1). The accepted PRD/ADR rev-16 erratum is present with real
PIB-402/403/425 tests. One production correction makes shared-blob remediation
emit executable narrow/`--all` preview and confirm commands. Independent
review of this block is pending; AP–AX (119 rows, 38 G guards) remain.

AM–AO review remains NEEDS REVISION on six findings: the ledger accepts
aggregate assertion bodies (PIB-443 exposed); PIB-437/438/445/447/448 guards
remain token/proxy based; privacy tests cannot observe cross-file raw sinks;
rev-16 does not derive actual changed rows/arithmetic; BSD/filesystem rows omit
full CLI/remediation truth; and PIB-424 remediation commands are not executed.

S7 rev-3 folds those six with body-aware targets, all twelve PIB-443 crash
cells, real semantic/privacy/revision validators, classifier/platform evidence
and executed PIB-424 routes. It reports 54 exact rows but an AN20/AO19 split
rather than the PRD's prior AN23/AO16 arithmetic; block re-review must resolve
that discrepancy before AP begins.

S7 rev-3 review remains NEEDS REVISION on five findings: unreachable or
parent-aggregate assertions can satisfy the ledger; rev-16 diff checks do not
reject unrelated normative edits; privacy subprocesses omit generator roots
and non-`os.WriteFile` transcript sinks; blocking CI cross-compiles FreeBSD
only rather than all four supported BSD targets; and G1–G4 runtime checks do
not reject the complete §7.13 Git-variable set. The category discrepancy was
reporting-only: the PRD and current manifest both correctly use AM15/AN23/AO16
with AN PIB-410…432 and AO PIB-433…448.

S7 rev-4 closes all five findings without beginning AP. The body resolver now
requires a reachable assertion in the exact selected leaf and rejects dead or
aggregate wrappers; all 54 AM–AO rows still resolve, including PIB-443's
twelve crash cells. Rev-16 compares the base documents against exact hashed
section allowlists and rejects unrelated PRD or ADR edits. PIB-419 executes
analysis/spec/exploration success, failure and retry in isolated subprocesses,
while PIB-420/446 follow the whole workflow package call graph and bite
cross-file stderr/stdout/file writers. Blocking CI cross-compiles all four BSD
targets without executing them. PIB-427/439 capture every G1–G4 environment,
derive the §7.13 scrub population, include `GIT_NAMESPACE`, and reject every
exact/indexed variable on every request. Arithmetic is AM15/AN23/AO16
(PIB-395…448; I16/C20/G13/U4/S1).

S7 rev-4 review remains NEEDS REVISION on three findings: lexical subtest
discovery accepts `t.Run` registrations inside dead/post-return parent paths;
privacy guards miss response writes through `os.CreateTemp` and `*os.File`
methods outside the subprocess cwd; and the production/test scrub added
`GIT_NAMESPACE` beyond §7.13's exact closed list. Rev-5 must make registration
reachability and temp-file observation bite, then remove the unauthorized
variable rather than amend the accepted contract.

S7 rev-5 closes those three findings without beginning AP. Exact leaf
resolution now applies reachability to the complete parent registration path,
so dead and post-return `t.Run` calls fail alongside the existing dead-callback
and aggregate-wrapper sensitivities; all 54 rows and PIB-443's twelve leaves
still resolve. PIB-419 confines cwd, temp and home roots to one observed
sandbox and its same validator bites an out-of-cwd temp leak. PIB-420/446 now
detect reachable `os.CreateTemp` plus receiver-typed `*os.File.Write` and
`WriteString`, including a cross-file helper-returned file fixture.
`GIT_NAMESPACE` is removed from production and PIB-427/439 authority sets;
the compatibility test instead proves it remains inherited as §7.13 requires.
Arithmetic remains AM15/AN23/AO16 (PIB-395…448; I16/C20/G13/U4/S1).

S7 rev-5 review remains NEEDS REVISION on four findings: unknown-condition
parent paths need dominance, not may-reach, and three leaves currently fail to
register when selected alone; the raw-temp guard still misses aliased method
expressions plus create/write/unlink; AO exact rows do not require nonempty
targets and PIB-446 omits the new temp sensitivity; and overflow-sized
digit-only indexed Git-variable suffixes survive production's integer parser
despite §7.13 requiring every `[0-9]+` index.

S7 rev-6 closes those four findings without beginning AP. The ledger resolver
now requires selected registration on every reachable parent exit and treats
return plus testing Fatal/Fatalf/FailNow/Skip terminators as no-fallthrough;
dead branches and uninvoked closures bite while all 54 targets and PIB-443's
twelve leaves resolve. PIB-401/402/404/424/445/448 setup was made independent
of sibling execution, and every affected literal leaf passes when selected
alone. PIB-419/420/446 use stdlib `go/types` identity plus local value/alias
propagation for `os.CreateTemp`, `*os.File.Write`/`WriteString`, method
expressions/values and create-write-unlink helpers. AO now rejects empty exact
targets and maps both temp leaves. Production Git scrubbing now recognizes a
nonempty ASCII-digit suffix lexically, including overflow-sized indexes on
every G1–G4 request, while malformed suffixes and `GIT_NAMESPACE` remain
inherited. Arithmetic remains AM15/AN23/AO16
(PIB-395…448; I16/C20/G13/U4/S1).

S7 rev-6 review remains NEEDS REVISION on two analyzer boundaries. Static
registration flow still accepts constant infinite-loop and short-circuit
unreachable `t.Run` calls and misses aliased test terminators. Raw-temp sink
provenance remains intraprocedural, so helper parameters/returns, interfaces,
package variables and selector dispatch can hide a create-write-unlink path.
Rev-7 will replace approximated registration semantics with observed exact
leaf execution and make sink-identity rejection interprocedurally complete (or
conservatively reject every forbidden identity in the reachable construction).

S7 rev-7 closes both analyzer findings without beginning AP. AST resolution is
now limited to exact package/test/subtest bodies and reachable assertions;
registration authority comes from one non-recursive `go test -json -p=1`
subprocess with an escaped exact top-level union. It observes one literal
target for each of the 54 AM–AO rows plus all twelve PIB-443 leaves, requiring
exactly one RUN and PASS event per target. The same validator rejects
compilable infinite-loop, short-circuit, aliased-SkipNow and uninvoked-closure
packages, with an enforced timeout and ledger/observer recursion exclusions.
PIB-446 now scans forbidden `go/types` identities in every expression of the
fixed-point reachable function/method graph and referenced package
initializers. Helper arguments/returns, interface boxing, package-var alias
chains, selector dispatch and multi-alias create-write-close-unlink fixtures
all bite while canonical final provider bytes remain allowed. Arithmetic
remains AM15/AN23/AO16 (PIB-395…448; I16/C20/G13/U4/S1).

S7 rev-7 review remains NEEDS REVISION on three boundaries: test2json events
can be forged by framed stdout from an unregistered test; cancellation kills
the `go` parent but does not reliably bound/terminate its test-binary
descendants; and raw-sink reachability omits package `init` roots plus local
implementations invoked through imported interfaces. Rev-8 will use
out-of-band instrumented leaf evidence, bounded inner/outer timeouts with
descendant cleanup, and conservative init/interface reachability.

S7 rev-8 closes those three findings without beginning AP. The validator now
builds an isolated AST overlay that injects an exact literal marker call into
each of the 54 selected row bodies plus all twelve PIB-443 leaves. The marker
helper is a normal no-op, but under a nonce-bearing validator environment it
creates one O_EXCL SHA-256-named file containing the nonce and target ID; JSON
RUN/PASS is diagnostic only. A framed test2json forgery shows clean JSON
diagnostics but fails for its missing marker. The child `go test` has an inner
timeout at one third of the outer deadline, WaitDelay, Unix/BSD process-group
kill and build-safe Windows/other child kill; the infinite-loop fixture records
its PID and proves bounded exit plus reap. Raw-sink reachability now treats
every package `init` as a root and uses `go/types` interface implementation
checks to enqueue compatible local concrete methods. Init-assigned aliases and
an `io.Writer` local implementation both bite while canonical final provider
bytes remain allowed. Arithmetic remains AM15/AN23/AO16
(PIB-395…448; I16/C20/G13/U4/S1).

S7 rev-8 review remains NEEDS REVISION on two trust boundaries. Marker
directory, nonce and deterministic IDs are shared with every selected test,
so a parent can forge both marker and test2json evidence while its leaf remains
unreachable. The expanded init/interface graph is used only for the newest
CreateTemp/file-method identities; legacy sinks such as `os.WriteFile` still
use the narrower graph, and generic receiver constraints are unresolved.
Rev-9 will inject per-target correlation tokens unavailable through shared
environment and run every forbidden sink through one typed full graph.

S7 rev-9 closes both review findings without beginning AP. Every selected leaf
receives an independent random per-run correlation path and token as literals
only in its AST overlay body; the child environment carries neither value, and
the marker helper uses O_EXCL.
The same validator rejects forged framed output plus the old shared marker
shape, a swapped target/path association and a mutated token, then removes
all marker, overlay and process scratch. PIB-446 now uses one fixed-point typed
graph for generator roots, all package init functions, reachable helpers,
referenced initializers, local methods, imported-interface implementations and
type-parameter constraints. The centralized classifier covers the legacy
retry/Store, filesystem, file-method, console, io, artifact/report/history and
CreateTemp identities. Init-assigned legacy aliases, `os.WriteFile` local
interface methods, generic `io.Writer` dispatch and the prior
create-write-unlink cases all bite while canonical final provider bytes remain
allowed. Arithmetic remains AM15/AN23/AO16
(PIB-395…448; I16/C20/G13/U4/S1).

S7 rev-9 review has one accepted finding and one withdrawn finding. The typed
graph still uses `importer.Default` plus empty packages for unresolved module
imports, so calls through `provider.Provider` can lose interface selection and
hide a sink. The marker objection is withdrawn: no accepted text requires
resistance to deliberately malicious same-UID test code, and no portable
same-process harness can provide it. Rev-10 will move overlay scratch outside
the repository, mode it 0700, describe markers as correlation evidence under
the non-adversarial test-source boundary, and add fail-closed module-aware type
loading plus a real `provider.Provider` sensitivity.

S7 rev-10 closes the one accepted defect and applies the bounded hardening for
the withdrawn marker finding without beginning AP. PIB-446 now obtains the
real module dependency/export graph through cached
`go list -deps -export -json` calls from the repository module and feeds those
exports to `importer.ForCompiler`; there is no empty-package fallback, and any
list/import/type-check error fails closed. A type-correct mutation of the real
local `countingProvider.Generate` implementation writes and unlinks provider
output and is reached through the actual module-resolved
`provider.Provider` interface before failing. Separate unresolved-module and
type-error fixtures prove fail-closed behavior while the stdlib
`io.Writer`/generic/init cases remain biting. Correlation overlay, marker and
process workspaces now use neutral random names outside the repository with
0700 mode and complete cleanup; no runtime scratch is created under
`internal/cli`. The marker is explicitly test-only semantic/proxy regression
evidence for PIB-230/§18.53, not a sandbox or security boundary: source under
validation is assumed not to inspect same-UID overlay files or its own
executable. Framed output, old-marker, wrong-association and wrong-token
sensitivities remain. Arithmetic remains AM15/AN23/AO16
(PIB-395…448; I16/C20/G13/U4/S1).

S7 rev-10 is APPROVED. AM–AO closes at 54/173 exact rows:
AM15/AN23/AO16, I16/C20/G13/U4/S1, with no residual findings. AP is now the
queue head. The accepted rev-16 contract is checkpointed at `cf324c0`; the
reviewed implementation/test block is checkpointed at `fdf86cb`.

Post-checkpoint CI [32476007227](https://github.com/tesseracode/tesserapatch/actions/runs/32476007227)
failed before AP dispatch. The Linux-only BSD step violated AVP-175's closed
step-condition grammar on all three legs, and macOS crossed Go's default
10-minute package timeout. The bounded correction runs BSD cross-compiles on
both non-Windows legs and restores the guard-pinned `-timeout 20m` full-suite
command. AP remains blocked until corrected CI is green.

The correction is committed at `9456a52`; blocking CI
[32476989232](https://github.com/tesseracode/tesserapatch/actions/runs/32476989232)
is green on Ubuntu, macOS and Windows. AM–AO is durably closed and AP is
unblocked.

S7 AP is implemented in the worktree and awaits review. The PRD-derived block
is exactly 34 rows (`PIB-449…482`) with I9/C12/G9/U2/S2. Every row has a
nonempty body-sensitive literal target, all 34 targets emit independent
observed-execution correlations, and each of the nine G rows has a biting
same-validator wrong input. Two production gaps proven by AP were corrected:
human dry-run output now prints the closed
`Execution preflight: not_evaluated` field as well as the verbatim plan
sentence, and `prepare --help` now discloses the host-local/non-cross-machine
limit of an accepted unknown-local filesystem flock. PIB-461 now derives and
executes every current evaluated dry-run refusal class: 22 native rows pair
real refusing and admissible runs with zero authority/provider/flock/Git
effects and whole-tree snapshots; the remaining
`workspace-unsupported-platform` row has a literal native-Windows runtime
target plus the blocking Windows compile lane. AQ–AX remain blocked pending AP
review.

S7 AP rev-0 review is NOT APPROVED. The exact ledger maps multiple rows to
adjacent operations rather than their normative workflows: CP9/CP4 abandon
evidence is incomplete; dangling rows do not drive regenerate/list/doctor;
partial-purge rows omit the public exit-5 report and emitted retry execution;
exit-3 coverage is not catalog-total; D9 lacks a real child holder and all
owned surfaces; Git/global-ignore/field coverage is partial; filesystem rows
do not bind the classified descriptor to the flock or perform post-release
Control; PIB-482 omits the forbidden extraction claim; and AP Windows dry-run
coverage is absent from the blocking selector. AQ remains blocked.

S7 AP rev-1 closes all eight findings without opening AQ–AX. PIB-450/451 now
use real two-entry CP4 publication evidence and a derived CP9 divergent state,
preserving the complete canonical subtree while abandon moves the journal
unparsed. PIB-457…460 drive ordinary public prepare, list, doctor and purge
through the one dangling-hash route, then prove zero missing-blob removal and
global one-CAS rehydration. PIB-466/467 execute the real partial-purge seam,
parse and run the emitted retry from the workspace root, strict-decode each
intermediate state and finish the selector; PIB-469 executes every PRD-derived
exit-3 population against exact whole-tree snapshots. PIB-470/471 hold the
real authority/flock in a child process while D9 proves zero authority,
Control, process and write capability, and every owned truth surface rejects
claims that no holder is running. PIB-473/475/476 bind every compatibility
caller, real main/worktree/submodule and global-excludes-only contexts, exact
G1–G4 requests and recursive path-safe reports. PIB-478/479/481 bind
open-root/open-dot, identity, fstatfs and flock to the same held descriptor,
pin Linux/Darwin deny tables plus sshfs unknown-local real-flock behavior, and
prove deterministic post-release Control/concurrent-close classification.
PIB-482 excludes only the uniquely bounded revision-history ledgers, resolves
all remaining §/PIB/D anchors and rejects both dangling anchors and a positive
rescap extraction claim. The blocking Windows GH #23 step now runs and
asserts the literal PIB-461 native leaf; omission, narrowed-condition and
advisory-step mutations fail while AVP-175 remains green. The AP34 ledger was
remapped to these normative leaves and all 34 observed correlations pass.
AQ–AX remain blocked pending rev-1 review.

S7 AP rev-1 review remains NEEDS REVISION on seven exactness findings:
PIB-450 snapshots only one slug rather than the complete features subtree;
PIB-459 does not close every owned surface to the sole purge command; PIB-460
does not run public regeneration over multiple generations; PIB-467 omits the
completion hash list; PIB-476 samples only two production output populations;
PIB-478 does not dataflow-bind `file.SyscallConn` through `Control` to
`Fstatfs`; PIB-479 ignores extra literal deny cases; and PIB-482 recognizes
only extract/reuse keywords rather than equivalent origin claims.

S7 AP rev-3 review remains NEEDS REVISION on eight guard/evidence gaps:
PIB-459 ignores detected non-purge `tpatch` commands; PIB-468 injects the
expected typed divergence error rather than corrupting evidence for production
to classify; PIB-471 remains a finite phrase blacklist; PIB-476 treats
unresolved path values as safe; PIB-477 ignores computed map keys; PIB-478's
foreign-platform `Fstatfs` identity is name-only; PIB-479 does not constrain
all denying control-flow paths; and PIB-482 still misses direct origin synonyms
such as “copied from”. AQ–AX remain blocked.

S7 AP rev-4 closes all eight findings without opening AQ–AX. PIB-459 now
checks every command in the exact owned declaration/prose inventory and admits
only the canonical dangling purge argv. PIB-468 injects hash-wrong evidence at
the storage probe immediately after pending CAS; normal production validation,
not a special typed-error branch, returns exit 6 while preserving the pending
index and every selected blob. PIB-471 and PIB-482 use exact, bounded
nonhistorical clause inventories, so any new holder-absence or
prepare-authority/rescap claim is unclassified and fails closed. PIB-476 has a
complete report-type/producer/runtime-population inventory, fail-closed typed
string flow, explicit non-path/relative classifications, authority-refusal and
panic-redaction runtime cases, and biting repo-root, join and unknown-helper
mutations. PIB-477 resolves controlled map keys with module-aware `go/types`
constant evaluation and rejects unresolved dynamic keys. PIB-478 type-checks
both Linux and Darwin source against target-GOOS export data and resolves the
exact imported `syscall.Fstatfs` object; PIB-479 requires the sole closed
five-case switch and rejects extra branch/helper denial routes. AP34 and the
independent observer remain exact and green. AQ–AX remain blocked pending
rev-4 review.

S7 AP rev-4 review remains NEEDS REVISION on five closure gaps: PIB-471 does
not resolve concatenated holder clauses; PIB-476 exempts unresolved values by
destination field name; PIB-477 omits controlled storage/intentpub schema
producers; PIB-478 accepts arbitrary one-argument fd conversion and PIB-479
does not constrain the outer held-filesystem classifier; and PIB-482's
inventory domain excludes a nonhistorical rescap lock clause without the word
“prepare”. AQ–AX remain blocked.

S7 AP rev-5 closes all five rev-4 findings without production changes or
opening AQ–AX. PIB-471 reconstructs maximal owned compile-time strings before
clause normalization, including concatenations and static helper returns.
PIB-476 removes destination-name exemptions: every controlled assignment is
proved from exact typed fields, enums, canonical parameters, result positions
or validated constructors, and repo-root/unknown parameter/helper values fail
even in `Code`. PIB-477 inventories every controlled report, `status.json`,
sidecar, `index.json` and journal schema/producer across CLI, store, intentpub
and the sidecar type; computed/unresolved generator-class keys and any new
unclassified schema/producer fail closed. PIB-478 requires the exact universe
`int` conversion object between the `Control` callback descriptor and imported
`syscall.Fstatfs`; PIB-479 requires direct inner-classifier result passthrough
and the unchanged denied/class gate before flock. PIB-482 hashes every bounded
nonhistorical rescap clause, including ADR D4's
`internal/rescap/lock_unix.go` precedent. AP34 and its independent observer
remain exact and green. AQ–AX remain blocked pending rev-5 review.

S7 AP rev-5 review remains NEEDS REVISION on four closure gaps: PIB-476
trusts helpers by `Rel` suffix/name instead of validating their returns;
PIB-477 checks only `map[string]any`, not every string-key controlled map;
PIB-478 does not bind the `Fstatfs` destination to the exact `stat` object
subsequently classified; and PIB-479 permits early denial returns before the
validated terminal passthrough. AQ–AX remain blocked.

S7 AP rev-6 closes all four rev-5 findings without production changes or
opening AQ–AX. PIB-476 recursively validates the actual body and every
reachable return of helper calls feeding controlled fields, with argument
binding, exact operation semantics and one AST-body-hash-bound sanitizer;
helper names and `*Rel` suffixes grant no authority. PIB-477 follows every
string-keyed map type, regardless of element type, through controlled encoder
and interface/call dataflow, resolving every key constant and failing closed
on dynamic keys or unresolved map-bearing values. PIB-478 requires
`syscall.Fstatfs` to receive the exact builtin-`int` callback descriptor and
unary `&stat`, then proves that same typed local object feeds the exact inner
classifier. PIB-479 validates the complete outer classifier body: only
Fstatfs error returns plus one unchanged inner-classifier success return are
permitted, with no extra boolean assignment or denial route. AP34 remains
exact at `I9/C12/G9/U2/S2`; its ledger, all AP selectors and independent
observed-correlation run are green. AQ–AX remain blocked pending rev-6 review.

S7 AP rev-6 review remains NEEDS REVISION on four narrow dataflow gaps:
PIB-476 allows an unresolved right operand after a relative prefix; PIB-477
does not follow map assignments through struct fields; PIB-478 only requires
the classifier input to mention `stat` rather than use the exact GOOS
transformation; and PIB-479 accepts a nil `err` return as an error-only path.
AQ–AX remain blocked.

S7 AP rev-7 closes all four rev-6 findings without production changes or
opening AQ–AX. PIB-476 proves every nonliteral concatenation operand
independently, permits only literal empty/separator composition, preserves
validated nonempty relative segments, and rechecks the normalized composite
for absolute/traversal content. PIB-477 follows map-bearing struct fields,
nested selectors, pointer/map aliases and later field-map writes to controlled
encoders, resolving every key against the exact field object/schema. PIB-478
requires the exact GOOS-specific sampled-input AST and typed object chain and
pins both transformation helper bodies. PIB-479 accepts an error return only
as the sole body of an exact `sameError != nil` guard and rejects nil checks,
unrelated guards and swapped error objects. AP34 remains exact at
`I9/C12/G9/U2/S2`; focused rows, ledger, all AP selectors and observed
correlation are green. AQ–AX remain blocked pending rev-7 review.

S7 AP rev-7 review remains NEEDS REVISION on four provenance edges:
PIB-476 trusts named string result types without proving enum values; PIB-477
does not propagate map mutations through helper arguments/parameters; PIB-478
does not require `Fstatfs` to assign the exact guarded `statErr`; and PIB-479
permits synthetic writes to an error variable before an otherwise canonical
non-nil guard.

S7 AP rev-8 closes all four rev-7 findings without production changes or
opening AQ–AX. PIB-476 proves named-string values from exact declared enum
constants or recursively validated assignments/constructors; dynamic
conversions from environment, repository-root or unresolved helper values
fail even when the destination type is a named string. PIB-477 propagates
controlled map aliases and mutations interprocedurally through parameters,
returns, wrappers, nested calls and pointer-to-map values, while unresolved
external mutators fail closed. PIB-478 requires the exact
`statErr = syscall.Fstatfs(int(descriptor), &stat)` assignment and the existing
exact descriptor, buffer and GOOS-specific classifier dataflow. PIB-479 binds
all three permitted error objects to their exact `SyscallConn`, `Control` and
`Fstatfs` definitions, requires a sole write plus immediate dominating
same-object non-nil guard, and rejects manual, reassigned, multiple-definition,
unresolved and swapped provenance. AP34 remains exact at I9/C12/G9/U2/S2;
focused rows, complete affected packages, independent observed correlation
and all requested platform compiles are green. AQ–AX remain blocked pending
rev-8 review.

S7 AP rev-8 review remains NEEDS REVISION on three alias edges: PIB-476 still
trusts enum-typed selector fields without tracing their assignments; PIB-477
checks unresolved external map mutators only when a direct indexed write also
exists; and PIB-479 ignores indirect/aliased lvalue writes that can replace a
canonical syscall error. AQ–AX remain blocked.

S7 AP rev-9 closes all three rev-8 findings without production changes or
opening AQ–AX. PIB-476 records every selector assignment and keyed/unkeyed
composite field value, records omitted named-enum defaults as deterministic
zero-value flow, and recursively proves local constructor/helper results;
direct, composite and helper-derived dynamic `prepareOptions.mode` values fail
while exact declared constants pass. PIB-477 evaluates unresolved mutators
independently of indexed writes, marks unresolved function-valued map calls
fail-closed, and retains
interprocedural acceptance of local mutators whose every key is proven safe.
PIB-479 resolves direct, parenthesized, address/dereference and pointer-alias
references to the three canonical error objects, rejects any address-taking,
and still permits only each exact `SyscallConn`, `Control` or `Fstatfs`
definition followed by its exact dominating guard. AP34 remains exact at
I9/C12/G9/U2/S2; focused rows, all affected AP selectors, independent
observed correlation and requested platform compiles are green. AQ–AX remain
blocked pending rev-9 review.

S7 AP rev-9 review remains NEEDS REVISION on two final graph invariants:
PIB-477 does not propagate controlled-map aliases through index/range-derived
values, and PIB-478/479 do not require exactly one immediate dominating guard
for every canonical syscall error definition. AQ–AX remain blocked.

S7 AP rev-10 closes both rev-9 findings without production changes or opening
AQ–AX. PIB-477 carries controlled-map ownership through index expressions,
type assertions, selector/index alias chains and map range values; nested
constant keys are validated, dynamic nested indexes fail closed, and
map-of-map/interface mutations remain connected to the encoded root. Both
index and range forbidden-key fixtures bite while resolved constant-key
mutations pass. PIB-478/479 explicitly map the three canonical
`SyscallConn`, `Control` and `Fstatfs` error definitions to exactly one
same-object non-nil guard each. The guards must dominate immediately in
syscall/control/fstatfs order before classifier use; the `Control` definition
is owned by its guard initializer and the callback-owned `Fstatfs` definition
is guarded in the immediately following outer scope. Missing, duplicate,
delayed and wrong-object guards all fail while reaching-definition and
no-mutation proofs remain active. AP34, affected AP selectors, independent
observed correlation and requested platform compiles are green. AQ–AX remain
blocked pending rev-10 review.

S7 AP rev-10 review remains NEEDS REVISION on one final mutation edge:
PIB-478/479 do not require `Fstatfs` to be the sole write to the sampled
`stat` object, so a guard initializer or intervening reassignment can erase the
sample before classification. AQ–AX remain blocked.

S7 AP rev-11 closes the sole rev-10 finding without production changes or
opening AQ–AX. PIB-478/479 now treats the exact local `stat` object as
single-writer sampled evidence: the only address use is the canonical `&stat`
second argument to the sole imported `syscall.Fstatfs` call. Direct,
compound/initializer, field, closure and pointer-alias mutations plus helper
address escapes all fail. Linux admits no sampled-stat slice; Darwin admits
only its one pinned `stat.Fstypename[:]` classifier input, whose helper body
and exact field remain guarded. The matching `statErr` guard must immediately
follow the callback-owning `Control` scope, and the exact platform classifier
must immediately consume the unchanged sample. All requested wrong-input
sensitivities bite; AP34, affected AP tests, independent observed correlation
and platform compiles are green. AQ–AX remain blocked pending rev-11 review.

S7 AP rev-11 review remains NEEDS REVISION on one AST edge: assignment-form
`range` key/value targets are not classified as writes and can overwrite the
sampled `stat` before classification. AQ–AX remain blocked.

S7 AP rev-12 closes the sole rev-11 finding without production changes or
opening AQ–AX. The sampled-stat write collector now classifies assignment-form
`RangeStmt` key/value targets, all assignment tokens (including compound and
guard initializers), select receive clauses, increment/decrement and
type-switch assignments by the exact `go/types` object reached through
parentheses, selectors, indexes, slices and dereferences. Address-taking
remains forbidden except for the one canonical `&stat` passed to
`syscall.Fstatfs`; pointer/helper/closure aliases
therefore cannot bypass the sole-write rule, while shadow declarations remain
distinct objects. `classifyHeldFilesystem` is additionally pinned to its exact
eight-statement shape, zero-valued stat/statErr declaration positions, ordered
error guards and immediate classifier consumption. Range key/value, initialized
declaration, compound field and select-receive sensitivities all bite. AP34,
affected AP tests, independent observed correlation and platform compiles are
green. AQ–AX remain blocked pending rev-12 review.

S7 AP rev-12 is APPROVED with no residual findings. AP closes at 34/173 rows:
PIB-449…482, I9/C12/G9/U2/S2. Combined with AM–AO, S7 now has 88/173 exact
rows. AQ remains blocked until the AP checkpoint is staged, validated, pushed
and green on blocking CI. The reviewed AP implementation/test block is
checkpointed at `bf9424f`.

Post-checkpoint CI [32520986445](https://github.com/tesseracode/tesserapatch/actions/runs/32520986445)
invalidated the close before AQ dispatch. The native Windows dry-run leaf was
misattributed to PIB-461 and expected exit-3 unsupported-platform, but PIB-463
requires exit-0 `planned`, `execution_preflight: not_evaluated`, no mutation
and no platform/filesystem/Git/lock/recovery refusal. Ubuntu/macOS also proved
the AP observer's 120-second inner deadline is below hosted-runner runtime
(about 138 seconds). AP requires a bounded post-acceptance correction and
fresh review/CI; AQ–AX remain blocked.

The bounded AP post-acceptance CI correction is complete without production
changes. The native Windows target is now
`TestS7APDryRunWindowsNotEvaluatedPlatform/PIB-463`; it and the host seam
exercise the public quiet JSON dry-run and directly assert exit 0, exact
planned/not-evaluated report shape and plan note, nil refusal, zero authority,
provider and lock activity, byte-identical workspace state, and absence of
every closed-catalog refusal code. PIB-461 now retains only evaluated step-7
coverage; PIB-463 owns both dry-run leaves and the Windows blocking guard. CI
and same-validator workflow sensitivities require the renamed PIB-463 target.
The AP observer now has an eight-minute outer budget, four-minute inner budget
and one-minute cleanup margin; wrong-input fixtures retain their explicit
short deadlines, including the three-second hanging-process reap proof.

The AP CI correction is APPROVED. AP's 34-row arithmetic is unchanged and its
corrected acceptance is restored; AQ remains blocked only on committing,
pushing and observing green blocking CI with the native PIB-463 leaf.

The correction is committed at `3fd778b`; blocking CI
[32523687156](https://github.com/tesseracode/tesserapatch/actions/runs/32523687156)
is green on Ubuntu, macOS and Windows, including the native PIB-463 leaf. AP is
durably closed at 34 rows and cumulative S7 coverage is 88/173. AQ is
unblocked and active.
Focused AP34, observer, workflow, vet/build/format and Windows cross-compile
validation is green. Fresh review and blocking CI are required before AQ.

S7 AQ is implemented in the worktree and awaits review. The manifest-derived
block is exactly 23 rows (`PIB-483…505`) with I13/C3/G7; all 23 rows have
nonempty exact body-sensitive targets and independent observed-execution
correlations without weakening the accepted AM–AP partitions. Real fixtures
cover CP3–CP7 terminal recovery for generate/manual/regenerate, J1–J10/CP9/
undo-CAS recovery refusal, prepare-journal and purge-pending precedence,
purge-owned terminal recovery, abandon gate/Git/residue behavior, flag
grammar, absolute-root privacy and every required retry population. The seven
G rows have biting same-validator mutations. One production gap proven by
PIB-488 is corrected: recovery failures now append sanitized repo-relative
journal, lane, canonical-feature and archive evidence locations while
retaining the existing classifier and exit class. AR–AX remain blocked
pending AQ review.

S7 AQ rev-0 review remains NEEDS REVISION on eight exactness findings:
PIB-485 mutates the recovered tree before retry; PIB-486/491 do not bound
multi-hash effects exactly; PIB-487 permits extra recovery-branch returns;
PIB-488 assumes undo-CAS preservation; PIB-497 trusts human-output identifiers
by name; PIB-503 misses indirect Cobra mutex calls; PIB-504 does not inventory
all package syscalls; and PIB-505 uses an ambiguous neighboring-text window.
AR–AX remain blocked.

S7 AQ rev-1 closes all eight rev-0 findings and is ready for review. PIB-485
executes the emitted retry from the byte-exact post-recovery tree, proves the
requested mode through argv/report/artifact evidence and rejects a second
recovered result. PIB-486/491 use two pending hashes, five generations, a
shared pending reference, a separate selector and an unrelated retained blob;
they compare exact indexes and blob identities before running the selector
retry separately. PIB-487 requires one recovery-success return after the
completion seam and release. PIB-488 compares the complete `.tpatch` tree
captured at the real undo-CAS injection seam. PIB-497 reuses AP's module-aware
recursive report proof through the complete reachable human-renderer call
graph. PIB-503 resolves Cobra method values, aliases, wrappers and callable
parameters. PIB-504 type-checks every Linux, Darwin and Windows production
file and closes the complete `Flock`/`Fstatfs`/`(*os.File).Fd` inventory.
PIB-505 binds all 95 nonhistorical references to exact Markdown sentences,
list items or table cells and to the mechanically parsed ordered step tables.
The AQ23 I13/C3/G7 partition is unchanged; AR–AX remain blocked pending
rev-1 review.

S7 AQ rev-1 review remains NEEDS REVISION on five residuals: PIB-485 accepts
an invalid exit-0 no-op retry; PIB-484/487 gate detection is alias-bypassable;
PIB-497 omits non-`fmt` writer sinks; PIB-504 inventories only Linux/Darwin/
Windows rather than every GOOS source; and PIB-505 still permits wrong
same-cell step citations. AR–AX remain blocked.

S7 AQ rev-2 closes all five rev-1 findings and is ready for review. PIB-485
rejects `no-op` absolutely and uses three normative untouched-tree retries —
generate/CP3, manual/CP4 and regenerate/CP7 — whose requested operation remains
real. PIB-484/487 use a module-aware callable graph with a closed baseline
inventory; direct gates, function and method-value aliases, wrappers,
parameters, returned callables and unresolved calls all fail while the sole
terminal return remains mandatory. PIB-497 starts at all three public emitters
and follows the reachable human, quiet, JSON, stdout and stderr graph through
fmt/io/writer methods, aliases and local helpers; uncontrolled JSON, global
print, `io.WriteString`, `io.Copy` and `Writer.Write` all bite through AP476's
recursive value proof. PIB-504 derives every GOOS from `go tool dist list`,
evaluates every production file's build partition, requires union coverage
with no skipped source and uses target exports wherever a syscall/Fd candidate
exists. PIB-505 binds all 95 citations to their immediate punctuation-bounded
semantic clause, validates the ordered-step predicate before the closed
normalized clause-inventory hash, and rejects both real-wrong PIB-494
citations despite `flock` elsewhere in the cell. AQ23 remains I13/C3/G7;
AR–AX remain blocked pending rev-2 review.

S7 AQ rev-2 review remains NEEDS REVISION on five semantic edges: PIB-485 can
claim an operation attempt from refusal text without a mode-specific boundary;
PIB-484/487 stops traversal at trusted local helper names; PIB-497 permits
builtin `print`/`println`; PIB-504 misses syscall/Fd aliases; and PIB-505
checks step keywords without required/forbidden polarity. AR–AX remain blocked.

S7 AQ rev-3 closes those five findings and is ready for review. PIB-485 now
uses real mode-specific retry fixtures and exact observed boundaries:
generate reaches provider/staging/set-revalidation, manual reaches the named
manual-status CAS seam, and regenerate reaches provider authority before its
precise refusal; the same result without the boundary fails. PIB-484/487
recursively traverse every reachable local helper and stop only at a closed
external/stdlib leaf inventory. PIB-497 treats the exact builtin `print` and
`println` objects as traced output sinks. PIB-504 follows typed syscall
callables through values, assignments, parameters, wrappers and returns across
every GOOS partition, while any `(*os.File).Fd` selection fails. PIB-505 binds
the mechanically parsed canonical step definitions and every immediate
citation clause by normalized exact hashes, so gate/action/order polarity
reversals fail. AQ23 remains I13/C3/G7; AR–AX remain blocked pending review.

S7 AQ rev-3 review remains NEEDS REVISION on two callgraph edges:
PIB-484/487 trusts `afterRecoveryComplete` before validating production
assignments/callback bodies, and PIB-497 silently accepts unresolved local
interface dispatch absent from its function universe. AR–AX remain blocked.

S7 AQ rev-4 closes both findings and is ready for review. PIB-484/487 now scan
every non-test CLI production source, require the one exact nil `func()` seam
declaration and reject every assignment, lvalue mutation or address escape;
conditional, aliased and helper callback assignments bite while test-only
assignments stay outside the production scan. PIB-497 now resolves typed local
interface dispatch through all implementing value, pointer, embedded and
generic method sets, recursively validates every concrete method body, rejects
missing internal targets, and keeps external interface calls fail-closed
unless they are exact classified leaves. Safe local implementations pass;
`io.WriteString`, `Writer.Write`, builtin `println` and method-value alias
sensitivities fail. AQ23 remains I13/C3/G7; AR–AX remain blocked pending
rev-4 review.

S7 AQ rev-4 review remains NEEDS REVISION on one interface edge: PIB-497
enumerates implementations only from a fixed loaded-package subset and can
miss imported module/external concrete receivers. AR–AX remain blocked.

S7 AQ rev-5 closes that final interface edge and is ready for review. PIB-497
now resolves the actual receiver value through assignments, composites,
conversions, local constructors/returns and context-specific interface
parameter bindings from each reachable callsite. Concrete methods come from a
module-aware source graph whose local dependency closure is checked against an
offline `go list -deps -mod=readonly` manifest; external exports use the
existing `GOPROXY=off`/`GOSUMDB=off` importer. Only the resolved receiver
implementations are traversed—never a guessed world subset—and every
module-local method must have a loaded body. External or unresolved
implementations fail closed except exact writer sinks whose output arguments
are independently validated. Imported workflow implementations bite through
direct, constructor/return and interface-parameter routes; an unavailable
external implementation fails, while a selected safe module implementation
passes. AQ23 remains I13/C3/G7; AR–AX remain blocked pending rev-5 review.

S7 AQ rev-5 is APPROVED with no residual findings. AQ closes at 23/173 rows:
PIB-483…505, I13/C3/G7. Cumulative S7 coverage is 111/173. AR remains blocked
until the AQ checkpoint is staged, validated, pushed and green on blocking CI.

Staged-source validation reopened the AQ checkpoint before commit. The
accepted `prepare_publish.go` evidence-path change intentionally changes the
AP PIB-459 exact declaration hash for `runPreparePublish`, so the accepted AP
inventory required an explicit update. The full CLI package also proved
PIB-485's regenerate/CP3 retry is provider-dependent and rolls back at exit 5;
the test required deterministic provider behavior outside the byte-exact
recovered tree and prove the unchanged retry publishes. AR–AX remain blocked.

The bounded staged-source correction is complete. PIB-459's exact declaration
inventory accepts only the reviewed `runPreparePublish` hash delta caused by
AQ's recovery-evidence-path routing. PIB-485 now isolates provider
configuration and process environment outside the recovered workspace, uses a
deterministic in-memory provider for generate/regenerate, restores every
global/environment value, and requires all three modes to publish through
their observed operation boundaries with exact artifact dispositions. The
complete uncached CLI package and intentlock package pass serially. No
additional production behavior changed; AR–AX remain blocked.

The staged-source correction is APPROVED. Full CLI and intentlock packages
pass with the exact AP inventory and hermetic PIB-485 retry. The reviewed AQ
implementation/test block is checkpointed at `dc789f6`; AR remains blocked on
tracking commit, push and green blocking CI.

Post-checkpoint CI [32539554233](https://github.com/tesseracode/tesserapatch/actions/runs/32539554233)
is green on Ubuntu and Windows but macOS hit the blocking suite's 20-minute
package timeout while `TestS7ObservedAQRegistrationAuthority` was still
progressing at 2m42s. No AQ assertion failed. The non-Windows blocking timeout
needs a guarded bounded increase for cumulative S7 runtime; AR–AX remain
blocked pending correction and fresh CI.

The bounded AQ CI timeout correction is complete. Only the blocking
non-Windows full-suite command now uses an exact finite 40-minute package
timeout; the GH #17 Windows allowed-failure full suite remains at 20 minutes,
and every targeted native gate is unchanged. AVP-175 pins both commands,
continues to prove the non-Windows step is blocking, and has biting
lowered/removed-timeout sensitivities with no-op detection for every mutation.
No production or AP/AQ behavior changed. AR–AX remain blocked pending review,
checkpoint, push and fresh blocking CI.

The AQ CI timeout correction is APPROVED. AR remains blocked only on commit,
push and a green three-platform rerun.

CI [32540987009](https://github.com/tesseracode/tesserapatch/actions/runs/32540987009)
proved the 40-minute budget sufficient (macOS CLI completed in about 21
minutes) but correctly failed PIB-391 because the correction modified frozen
accepted source `internal/intent/avp_guards_test.go`. That file must be
restored byte-for-byte; the exact 40m/20m timeout assertions and sensitivities
must move to an S7-owned guard. Windows remained green. AR–AX stay blocked.

The bounded frozen-source correction is complete.
`internal/intent/avp_guards_test.go` is byte-identical to baseline `cdcd665`
with SHA-256
`60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`
and an empty baseline diff. The workflow retains 40 minutes only for the
blocking non-Windows full suite and 20 minutes for the GH #17 Windows
allowed-failure suite. A new S7-owned structural workflow guard pins exact
step count, conditions, ownership and finite timeout commands; same-validator
lower/remove/default/unbounded/swap/demote/narrow sensitivities all bite.
No production or AQ behavior changed. AR–AX remain blocked pending review,
checkpoint, push and fresh blocking CI.

Review remains NEEDS REVISION on one S7 guard bypass: full-suite detection
requires literal `go test ./...`, so a second invocation with flags before the
package (`go test -count=1 -timeout 0 ./...`) is ignored. The guard must parse
argv independent of flag order and reject every additional `./...` suite.

The bounded S7 timeout-guard revision closes that bypass. Each workflow
step's shell is tokenized into command argv with line continuations, basic
quotes, assignment prefixes and `env` prefixes handled. Every `go test`
invocation is inspected, and exact `./...` package arguments are recognized
independently of flag order. The workflow must contain exactly the intended
two full suites; extra blocking, advisory, unconditioned, package-first,
package-last and env-prefixed forms all fail. Timeout parsing rejects absent,
zero/unbounded, duplicate and wrong values before canonical command-order
validation. Frozen AVP source remains byte-identical. No production or AQ
behavior changed; AR–AX remain blocked.

Review remains NEEDS REVISION on one nested-shell bypass:
`bash -c 'go test -count=1 -timeout 0 ./...'` is seen only as a `bash`
invocation. Literal `sh`/`bash -c` payloads must be recursively parsed, and
opaque shell execution must fail closed.

The bounded nested-shell revision closes that bypass. Explicit `sh`/`bash`
command-mode invocations require exactly one statically literal payload and
are recursively parsed under an eight-level depth limit plus active-payload
cycle guard. Dynamic variables, command substitutions, extra payload argv and
all opaque shell modes fail closed. Direct, env-prefixed and nested-nested
full suites are included in the workflow-global count regardless of condition
or advisory ownership. Frozen AVP source and the exact 40m/20m workflow
commands remain unchanged. No production or AQ behavior changed; AR–AX remain
blocked.

Review remains NEEDS REVISION on one dynamic-executable bypass:
`"$BASH" -c 'go test -timeout 0 ./...'` is ignored because the executable
token is compared literally. A dynamic shell executable paired with command
mode must fail closed.

The bounded dynamic-executable revision closes that bypass. Shell tokens now
retain dynamic construction provenance for parameter expansion, command
substitution and backticks. An unresolved effective executable fails closed;
the only admitted dynamic executable identities are the workflow's fixed
`tpatch --version` smoke-test suffixes. Literal executables with dynamic
arguments remain valid, and explicit GitHub step-level `shell:` configuration
is never treated as command argv. Variable, `${...}`, `$()`, backtick and
env-prefixed dynamic shell sensitivities all bite through the same validator.
Nested recursion, workflow-global suite counting, exact 40m/20m commands and
frozen AVP source remain unchanged. No production or AQ behavior changed;
AR–AX remain blocked.

The complete frozen-source/S7-timeout correction is APPROVED. AR remains
blocked only on commit, push and a green three-platform CI rerun.

CI [32543144792](https://github.com/tesseracode/tesserapatch/actions/runs/32543144792)
passed Ubuntu and Windows; macOS failed only because AQ's four-minute inner
observer cap expired at about 4m10s. The 40-minute suite budget worked and no
AQ assertion failed. Only AQ's hosted observer budget requires a bounded
8m-inner/12m-outer correction; AP and short wrong-input/reap deadlines stay
unchanged. AR–AX remain blocked.

The bounded AQ observer-budget correction is complete. AQ alone now uses a
12-minute outer context and eight-minute hosted inner limit with the existing
one-minute cleanup margin. AP remains exactly 8m/4m/1m, AM–AO and all
wrong-input fixture deadlines are unchanged, and the hanging-process fixture
remains three seconds. The budget guard pins both category tuples separately,
requires finite positive limits, inner below outer-minus-cleanup, and both
outers below CI's 40-minute package budget; global-replacement and
wrong-category sensitivities bite. No workflow, frozen AVP source, production
or AQ semantics changed. AR–AX remain blocked pending review and fresh CI.

Review remains NEEDS REVISION because tuple checks are disconnected from the
actual AP/AQ call sites; swapping their constants still passes. Observer calls
and validation must derive from one category budget table or be source-bound
to the exact category values.

The bounded callsite-binding revision closes that finding. One category table
now owns the AP and AQ outer, inner, cleanup and row-range values. Both public
observer tests call one helper with an explicit category key; the helper
validates the exact table and binds that key to the complete expected PIB row
set before constructing either timeout. Cloned-table sensitivities reject
swapped tuples, wrong values, missing/extra categories and the accidental
global eight-minute replacement, while cross-category target sensitivities
reject using the same key at both call sites. Wrong-input and three-second
reap behavior remain unchanged. No workflow, frozen AVP source, production or
AQ semantics changed; AR–AX remain blocked.

The category-bound AQ observer correction is APPROVED. AR remains blocked
only on commit, push and a green three-platform CI rerun.

Correction `e6cabb0` passed blocking CI
[32544950471](https://github.com/tesseracode/tesserapatch/actions/runs/32544950471)
on Ubuntu, macOS and Windows. AQ is durably closed at 23 rows; cumulative S7
coverage is 111/173. AR is unblocked and active.

S7 AR rev-0 is **NEEDS REVISION**. Independent review found material
false-pass paths in PIB-506/507 divergence and removal coverage, PIB-508
catalog derivation, PIB-510…513 abandon totality/read-order/no-Git evidence,
PIB-514/515 write-confinement and exact preview comparisons, PIB-516…518
branch-specific retry prose, PIB-519 shipped-surface route inventory, PIB-520
grammar parsing, and the AR-specific 12m/8m/1m budget sensitivities. Production
must add the required branch-specific partial-purge instructions; the remaining
corrections are bounded AR test/guard/observer/tracking work. No AR files are
safe to stage before rev-1 re-review. AS–AX remain blocked.

S7 AR rev-1 implementation is complete and ready for independent re-review.
PIB-506/507 now prove each divergence report and printed removal command
against real path inventories; PIB-508/511/518/519/520 use closed,
same-validator semantic guards for the normative route catalog, prebranch
abandon graph, purge-progress objects, eight shipped permanent-block claims
and all five grammar productions. PIB-509…517 now prove exact stop ordering,
zero Git attempts, whole-workspace confinement, pending-object identity and
branch-specific retry behavior. Production rendering now keeps the required
Git-history caveat for index divergence and emits exact pending-recovery,
completion-only and orphan-scan retry instructions. The AR ledger remains
exactly 15 rows (I6/C4/G4/S1), with independent observed correlation and
12m/8m/1m category-bound budget sensitivities. No AM–AQ semantics, workflow,
frozen AVP source or assets changed. No AR file is safe to stage before the
rev-1 verdict; AS–AX remain blocked.

S7 AR rev-1 is **NEEDS REVISION**. The authorized production-rendering and
observer-budget corrections are accepted, but seven test/guard false-pass
paths remain: PIB-506's non-owned reports are containment-only; PIB-508 does
not reject extra route alternatives; PIB-511 collapses duplicate stop sites
and does not fail closed over every reachable read/Git call; PIB-515 snapshots
only `.tpatch`; PIB-516…518 do not compare complete progress objects,
renderings and emitter sites; PIB-519 accepts unresolved route slogans and
uses an incomplete shipped-surface inventory; and PIB-520 can ignore a
leading-whitespace production. Rev-2 is bounded to AR evidence and tracking;
the accepted production rendering must not widen. No AR file is safe to stage.
AS–AX remain blocked.

S7 AR rev-2 implementation is complete and ready for independent re-review.
PIB-506 now deep-compares both non-owned tombstone reports, their exact JSON
shape, exclusive repair route, stderr and privacy envelope. PIB-508 requires
the exact two-row route partition and exact normative definitions. PIB-511
preserves ordered stop-site multiplicity, fails closed over reachable
module-local/dynamic read and Git sinks, and pairs source sensitivities with a
recording executable/process seam. PIB-515 snapshots the whole workspace and
proves that an outside-`.tpatch` write is observable. PIB-516…518 compare
complete partial reports, JSON keys, human/error rendering and the closed
`purge_progress` emitter inventory with valid-shape semantic mutations.
PIB-519 inventories tracked shipped production/help strings, assets and
applicable docs repository-wide and resolves references to validated route
authority; PIB-520 parses every logical production despite leading whitespace.
The accepted rev-1 production strings and AR 12m/8m/1m observer budget remain
unchanged. No AR file is safe to stage before the rev-2 verdict; AS–AX remain
blocked.

S7 AR rev-2 is **NEEDS REVISION**. Four rev-1 findings are closed, but three
semantic guard gaps remain. PIB-511 does not collect refusal sites from
reachable helper bodies, can exempt additional read/callable alias shapes and
does not compare every gate-table exit/code/route cell exactly. PIB-518 can
miss progress writes through pointer aliases or unkeyed composites. PIB-519
uses fixed claim substrings plus paragraph-wide route co-occurrence, so the
mandated interrupted-purge slogan and unrelated/negated routes can pass.
Rev-3 is bounded to those three guards and tracking. Production, runtime,
ledger, observer, workflow and frozen AVP surfaces must remain unchanged. No
AR file is safe to stage; AS–AX remain blocked.

S7 AR rev-3 implementation is complete and ready for independent re-review.
PIB-511 now parses every gate-table cell exactly and compares the ordered
seven implementation refusal sites by code, exit and route class. Its typed
reachable-call analysis follows helper bodies and parenthesized, local-variable
and function-field aliases; unresolved calls fail closed outside a small
type-checked allowlist. Same-validator typed mutations cover a helper-mediated
duplicate refusal, `os.Open`, aliased reads, raw/aliased Git, the wrong exit and
the wrong remediation. PIB-518 now uses typed field identity and alias flow to
inventory direct, keyed, unkeyed and indirect `PurgeProgress` stores in source
order; pointer-alias and valid unkeyed-composite writes fail. PIB-519 now
derives an exact eight-clause shipped inventory, evaluates each claim sentence
against only its affirmative route or resolved authority context, and rejects
the mandated interrupted-purge slogan, an unrelated-sentence route and a
negated route without changing claim cardinality. Production, runtime, ledger,
observer, workflow, frozen AVP and assets are byte-unchanged by rev-3. No AR
file is safe to stage before the rev-3 verdict; AS–AX remain blocked.

S7 AR rev-3 is **NEEDS REVISION**. The direct rev-2 sensitivities now fail,
but their validators remain incomplete. PIB-511 loses refusals returned by
reachable helpers, appends callee stops outside call-site execution order and
silently accepts call-graph cycles. PIB-518 traverses progress stores only in
`feature_intent_archive.go`, so a helper in another production file can write
through a passed field pointer. PIB-519 inspects individual string literals
through a fixed regex/negation list, so folded strings, equivalent
“permanently stranded” claims and unlisted prohibitions can escape. Rev-4 is
bounded to those three residual evidence paths and tracking; accepted
production, runtime, ledger, observer, workflow, frozen AVP and closed AR
surfaces remain frozen. No AR file is safe to stage; AS–AX remain blocked.

S7 AR rev-4 implementation is complete and ready for independent re-review.
PIB-511 now composes typed report/refusal values through helper returns,
parameters, local variables, parentheses and aliases, instantiates those
summaries per call, and records emitter stops at call-site execution order.
Every reachable emitter must resolve a refusal, repeated calls retain
multiplicity, and reachable direct or mutual recursion fails closed. Biting
mutations cover returned/aliased refusals, a callee stop placed between local
stops, mutual recursion and an unresolved report transformer while retaining
all rev-3 sink/table mutations. PIB-518 now scans every non-test CLI production
file, propagates field-pointer origin through chained declarations,
assignments and transitive helper parameters, and inventories the resolved
store site package-wide; unresolved pointer stores fail closed. Its cross-file
two-helper and chained-assignment mutations join the retained direct pointer
and unkeyed-composite cases. PIB-519 now folds maximal Go string constant
expressions, including binary literals and named constants, discovers both
permanent blocking and stranding claims, and requires affirmative
operator-directed command/manual grammar or a validated authority reference.
Split strings, named-constant stranding, `forbidden to run`, `must not`, exact
slogan and unrelated-sentence mutations all fail the same validator. The real
shipped inventory remains exactly eight claims. Accepted production, runtime,
ledger, observer/registration, workflow, frozen AVP, assets and closed AR
surfaces are unchanged by rev-4. No AR file is safe to stage before the rev-4
verdict; AS–AX remain blocked.

S7 AR rev-4 is **NEEDS REVISION**. PIB-510 is order-dependent: its Git spawn
spy is installed before the production source set performs `git ls-files`, so
the test fails cold with exit 93 and passes after another test warms the shared
cache. PIB-511 merges alternative report values with boolean OR and treats a
bare report as resolved, allowing a helper with one refused and one unrefused
return path to pass; local rebinding also loses to caller-parameter
substitution. PIB-518 derives field identity from a promoted selector's
receiver and misses a `PurgeProgress` store through an embedded report.
PIB-519 folds only file-level constants, requires the redundant word
`permanently` for unrecoverability claims, and accepts conditional
`if … run …` text as an affirmative route. Rev-5 is bounded to the cold
PIB-510 ordering fix plus these three validator corrections and tracking.
Production, ledger, observer/registration, workflow, frozen AVP and all other
closed surfaces remain frozen. No AR file is safe to stage; AS–AX remain
blocked.

S7 AR rev-5 implementation is complete and ready for independent re-review;
it is not approved. PIB-510 now resolves and validates the production source
graph before installing the exit-93 Git/PATH spawn spy, so a fresh isolated
process proves both the static order and the subsequent zero-Git runtime
contract without cache warming. PIB-511 replaces existential boolean
aggregation with explicit refusal alternatives: every reachable exit-3 report
alternative must resolve to exactly one typed refusal, local dominating
assignments precede caller substitution, unchanged parameter paths remain
visible, and summaries retain call-site order, multiplicity and cycle
rejection. New sensitivities cover mixed refused/unchanged returns,
parameter rebinding, a helper result stored locally, two helper layers reused
at multiple call sites, and a legitimate caller-refused pass-through. PIB-518
identifies `PurgeProgress` through the selected `types.Var` field object, so
promoted access through anonymous embedded report pointers is inventoried
alongside the prior package-wide helper, alias and composite stores.
PIB-519 obtains maximal string constants from the typed package, including
function-local and cross-file aliases/concatenations, avoids constant-use
subexpression duplicates, recognizes unqualified archive unrecoverability and
requires affirmative clause-local route language. Conditional,
hypothetical, quoted/example-only, unrelated, prohibited and absent/negative
reference routes all fail; the real shipped inventory remains exactly eight
claims. Only the AR runtime test, AR guard test and this handoff changed in
rev-5. Production, ledger, observer/registration, workflow, frozen AVP,
assets and every other closed surface remain unchanged. No AR file is safe to
stage before the rev-5 verdict; AS–AX remain blocked.

S7 AR rev-5 is **NEEDS REVISION**. PIB-510 is closed by a cold isolated pass,
but four guard defects remain. PIB-511 substitutes caller parameters before
dominating local refusal-code assignments, accepts one resolved code
alternative while discarding unresolved alternatives, and does not validate
retry/remediation exactly. PIB-518 matches `PurgeProgress` by package, name
and type instead of canonical field-object identity, so a same-shaped decoy
can replace the authorized store. PIB-519 still uses finite claim/route
blacklists, duplicates constant aliases, and excludes embedded `.mdc` and
extensionless textual assets. Rev-6 is bounded to the AR guard and tracking:
resolve every code/route alternative exactly, bind the canonical selected
field, use positive clause/reference authority, deduplicate constants by
origin and derive inventory from all embedded textual assets. The AR runtime
test is now frozen with PIB-510. Production, ledger, observer/registration,
workflow, frozen AVP and all other closed surfaces remain frozen. No AR file
is safe to stage; AS–AX remain blocked.

S7 AR rev-6 implementation is complete and ready for independent re-review;
it is not approved. PIB-511 now resolves every dominating local refusal-code
binding before caller substitution, carries every alternative without
existential merging, and compares ordered duplicate-preserving stops by exact
code, exit, constructor route, retry and source-derived remediation. The
concrete `code = "slug-unsafe"` rebinding, wrong retry and same-code/wrong
remediation mutations all fail while caller-refused pass-through remains
valid. PIB-518 resolves the canonical
`intentArchivePurgeReport.PurgeProgress` `*types.Var` and accepts a selector
only when `types.Selection.Obj()` is that exact object. Promoted anonymous
pointer stores remain visible; a same-shaped decoy cannot replace an
authorized store, while an unrelated decoy store adds no false emitter.
PIB-519 recognizes unqualified blocking, stranding and unrecoverability
claims, validates executable routes with positive operator-directed
clause/reference grammar, rejects conditional, hypothetical, prohibited and
quoted/attributed command contexts, and deduplicates typed constant aliases by
their declaration/value origin. The shipped-source scan now derives every
embedded asset from `assets/embed.go`'s `go:embed` authority, including the
Cursor `.mdc` and extensionless Windsurf file; route-less mutations in each
fail. The real shipped claim inventory remains exactly eight. Only the AR
guard test and this handoff changed in rev-6. PIB-510's runtime test,
production, ledger, observer/registration, workflow, frozen AVP, assets and
all other closed surfaces remain unchanged. No AR file is safe to stage before
the rev-6 verdict; AS–AX remain blocked.

S7 AR rev-6 is **NEEDS REVISION**. The corrected PIB-511, PIB-518 and PIB-519
guards pass independently, canonical field identity and complete embedded
textual-asset discovery are closed, and every protected hash remains stable.
Four material residuals remain. First, the cold AR observer exceeds its fixed
8-minute inner budget: it timed out after 483.925s, so the 583.44s focused run
inside the separate 12-minute outer budget was insufficient evidence.
Second, PIB-511 misses address-taken helper rebinding and nested remediation
return alternatives. Third, PIB-518 skips invoked function literals and loses
definite noncanonical origin through pointer aliases. Fourth, PIB-519 ignores
route suffix context, matches `D13` inside `D130`, and misses equivalent
positive recoverability claims. Rev-7 is bounded to the AR guard and tracking;
it must close those adversarial variants and reduce cold observer execution
below the unchanged 8-minute budget without weakening coverage. Production,
runtime, ledger, registration/observer, workflow, frozen AVP, assets and every
other closed surface remain frozen. No AR file is safe to stage; AS–AX remain
blocked.

S7 AR rev-7 implementation is complete and ready for independent re-review;
it is not approved. The pre-fix cold observer reproduced the fixed 8-minute
failure at 483.71s test / 484.64s real. The three named algorithm defects also
reproduced against the rev-6 validator: PIB-511 accepted helper-mediated
pointer rebinding, PIB-518 accepted an invoked canonical-store closure, and
PIB-519 accepted a command followed by a prohibition.

PIB-511 now treats address-taken refusal-code and report values as
side-effecting unless their flow is resolved: direct pointers, pointer aliases,
named wrappers, parenthesized aliases and invoked closure captures are covered.
Remediation extraction walks every nested return and resolves assignment
alternatives, so nested `if`/`switch` and local override branches cannot hide
behind a later correct return. Exact ordered code/exit/constructor/retry/
remediation comparison remains unchanged. PIB-518 now inventories invoked
direct, parenthesized and locally aliased function literals while leaving
uninvoked callbacks out of the population. Canonical and definite
noncanonical pointer origins propagate separately through aliases and helpers;
mixed conditional origins fail closed, while a definite decoy-pointer store is
ignored without poisoning the canonical inventory. PIB-519 validates the
whole command clause, including suffix polarity and conditions, uses exact
reference boundaries (`D130`, `D13a` and `pre-D13-post` do not satisfy D13),
and recognizes affirmative recoverability constructions such as “The archive
can always be recovered” without misclassifying nearby negative or quoted
examples.

Performance work is semantic-preserving: PIB-511 and PIB-518 type-check only
the complete `internal/cli` production package while imported dependencies
remain typed from the real module, reuse each exact typed mutation for its
same-validator assertion, and retain every prior sensitivity. PIB-519 reuses
immutable typed Go claim sections only when an exact SHA-256 key over every Go
source name and byte body matches; any mutated Go tree receives a fresh typed
state, while document/asset mutations reuse only unchanged Go evidence. The
final cold observer passes in 339.933s, over 140 seconds below its unchanged
480-second inner limit. Only the AR guard test and this handoff changed.
PIB-510 runtime, production, ledger, registration/observer, workflow, frozen
AVP, assets and all other closed surfaces remain unchanged. No AR file is safe
to stage before the rev-7 verdict; AS–AX remain blocked.

S7 AR rev-7 is **NEEDS REVISION**. The unchanged cold observer now passes in
337.88s test / 338.83s wall, about 141 seconds inside its fixed 480-second
inner budget, and the rev-6 direct probes are closed. Four guard-authority
residuals remain. PIB-511 does not derive §6.6's evidence-present manual
removal override for authority rows 5, 6 and 8, and address origin still
escapes through another pointer level, method receivers or captured callable
fields. PIB-518 skips reachable closures passed through invoked callable
parameters or fields and loses definite decoy origin through another pointer
level. PIB-519 still truncates route context at semicolons, accepts command
prefixes with illegal suffixes or missing required flags, admits negative
route/reference prose, and misses equivalent active recoverability claims.
Rev-8 is bounded to the AR guard and this handoff. Production, runtime,
ledger, registration/observer, workflow, frozen AVP, assets, PRD/ADR and every
other closed surface remain frozen. The fixed timing envelope is unchanged.
No AR file is safe to stage; AS–AX remain blocked.

S7 AR rev-8 is **NEEDS REVISION**. The unchanged cold observer passes in
395.35s test / 397.26s wall, about 85 seconds inside 480 seconds and below the
420-second target; focused guards/regressions, formatting, scoped vet, host
build and Linux/Darwin cross-compiles pass. Four authority residuals remain.
PIB-511 does not source-bind `prepareLaneHasPendingEvidence`, so predicate
polarity can reverse destructive-fallback eligibility without failing the
validator. PIB-518 omits an invoked callback returned by a local or named
helper. PIB-519 still accepts conditional or negative authority and
prefix-shaped manual targets, including traversal and canonical-path-child
forms, and does not inventory equivalent `restore`, always-possible recovery,
impossible-to-strand or `unblock` claims. Rev-9 is bounded to the AR guard and
this handoff. Production, runtime, ledger, registration/observer, workflow,
frozen AVP, assets, PRD/ADR and every other closed surface remain frozen. No
AR file is safe to stage; AS–AX remain blocked.

S7 AR rev-9 closes the four named rev-8 semantic gaps and its authorized
performance fold is implemented. Fresh PIB-511/518/519 package times are
181.220s, 59.099s and 16.555s: 256.874s total, below the 325-second observer
eligibility gate. The single newly authorized cold observer passes all 15
registered targets in 253.53s test / 253.958s package / 254.46s wall, below
the 420-second target without changing its 12m/8m/1m budgets. Narrow
regressions, formatting, scoped vet, host build and Linux/Darwin amd64
cross-compiles pass. The revision is ready for independent review but is not
approved or safe to stage; production and all other frozen surfaces remain
unchanged and AS–AX remain blocked.

S7 AR rev-9 independent review is **NEEDS REVISION**. Its performance fold is
accepted as credible: fresh reviewer package time is 263.288s across
PIB-511/518/519, and the unchanged cold observer passes all 15 targets in
264.985s package / 265.56s wall. Five authority residuals remain. PIB-511 does
not validate the canonical pending-evidence predicate body. PIB-518 misses a
factory callback returned through a named result and bare `return`. PIB-519's
lightweight constant evaluator loses valid Go conversions and mutually
exclusive build-specific candidates, wildcard targets and conditional
`then run` clauses still satisfy route authority, and `can never ... strand`
is misclassified as a negative rather than an active anti-stranding
guarantee. Rev-10 is bounded to those five corrections in the AR guard plus
this handoff. Production, runtime, ledger, registration/observer, workflow,
frozen AVP, assets, PRD/ADR and every other closed surface remain frozen. No
AR file is safe to stage; AS–AX remain blocked.

S7 AR rev-10 implements the five bounded guard corrections. PIB-511 now
validates the unique canonical `preparePendingEvidenceName` body against the
typed control/stage predicate objects and parameter, and compiled behavior
separately proves control evidence, stage evidence, unrelated entries and
contention's wait-only route. PIB-518's package-wide invoked-callable fixpoint
now resolves named function results at bare returns through assignments,
aliases, branches and nested factories, while unresolved invoked result
origins fail closed and uninvoked returned callbacks remain excluded.
PIB-519's bounded constant graph now evaluates Go integer/rune-to-string
conversion and propagates every viable build-variant candidate with lexical
scope, cross-product/deduplication, cycle/unresolved failure and full-package
cache keys. Route authority validates raw exact argv/target bytes before any
normalization, uses closed complete-clause suffixes and rejects wildcard,
conditional and extended routes. Claim polarity is verb-sensitive:
`never recover/restore/unblock` remains negative while active/passive
`never block/strand` guarantees enter the route-validated inventory. Fresh
PIB-511/518/519 package time is 269.803s, below the 325-second gate, and the
single authorized cold observer passes all 15 targets in 277.24s test /
277.670s package / 278.23s wall under the unchanged 12m/8m/1m budgets.
Rev-10 is ready for independent review but is not approved or safe to stage;
AS–AX remain blocked.

S7 AR rev-10 independent review is **NEEDS REVISION**. Review stopped before
focused Go tests and the observer after four Medium static false-pass paths
were established. PIB-518 misbinds method-expression argument zero as the
callback instead of consuming the explicit receiver, so the real invoked
callback can escape inventory. PIB-519 retains stale folded claims when an
unchanged file depends on a changed cross-file/build-specific constant,
accepts selector values from the wrong domain such as `--blob <slug>`, and
does not inventory the equivalent active guarantee `No operator can strand
the archive.` Rev-11 is bounded to those four corrections in the AR guard
plus this handoff. Production, runtime, ledger, registration/observer,
workflow, frozen AVP, assets, PRD/ADR and every other closed surface remain
frozen. No AR file is safe to stage; AS–AX remain blocked.

S7 AR rev-11 implements the four bounded guard corrections. PIB-518 now uses
the typed selector kind to consume a method expression's explicit receiver
before binding callable parameters; method values and ordinary functions keep
their zero offset. PIB-519 invalidates every section and folded claim in an
affected Go package when any declaration changes, distinguishes blob
`<hash>` and generation `<id>` selectors while requiring concrete values to
be 64 lowercase hex, and inventories singular/plural negative-quantifier
anti-stranding guarantees without admitting double negation or negative
recoverability. Fresh PIB-511/518/519 package time is 279.930s, below the
325-second gate. The single authorized unchanged cold observer passes all 15
targets in 309.27s test / 310.272s package / 310.87s wall under the immutable
12m/8m/1m budgets. Rev-11 is ready for independent review but is not approved
or safe to stage; AS–AX remain blocked.

S7 AR rev-11 independent review is **NEEDS REVISION**. Review stopped before
focused Go tests and the observer after three Medium static false-pass paths
were established. PIB-518 consumes a method expression's explicit receiver
but discards its callable origin, so a function field invoked through that
receiver can execute an extra canonical store outside the inventory. PIB-519
accepts manual-route slugs that production always rejects, including
Windows-reserved names and values longer than 60 bytes, and its
negative-quantifier grammar misses equivalent punctuated or reordered
guarantees such as `No operator, ever, can strand the archive.` Rev-12 is
bounded to those three corrections in the AR guard plus this handoff.
Production, runtime, ledger, registration/observer, workflow, frozen AVP,
assets, PRD/ADR and every other closed surface remain frozen. No AR file is
safe to stage; AS–AX remain blocked.

S7 AR rev-12 implements the three bounded guard corrections. PIB-518 now
binds typed method-expression and method-value receiver expressions to the
canonical receiver object before resolving invoked function fields, follows
receiver aliases/addressing/composites/promoted fields through the same
callable fixpoint, and fails closed only when an invoked receiver-derived
origin remains unresolved. PIB-519 now delegates every concrete route slug
to production `intent.CanonicalSlug` while preserving the normative
`<slug>` placeholder, and replaces the whitespace-sensitive
negative-quantifier alternatives with an exact bounded token/clause parser
covering benign punctuation and either `ever` placement. Fresh
PIB-511/518/519 package time is 295.121s, below the 325-second gate. The
single authorized unchanged cold observer passes all 15 targets in 312.07s
test / 312.614s package / 313.78s monotonic wall under the immutable
12m/8m/1m budgets. Rev-12 is ready for independent review but is not approved
or safe to stage; AS–AX remain blocked.

S7 AR rev-12 independent review is **NEEDS REVISION**. Review stopped before
focused Go tests and the observer after one Medium PIB-518 false-pass path was
established. Direct selector calls bind their receiver, but a bound method
value transported through a local alias loses both its selected method and
receiver. Invoking that alias can therefore execute a receiver-held canonical
store outside the inventory without failing closed. Rev-13 is bounded to
preserving method-value callable identity and bound receiver origins through
aliases, arguments and returns, with unsupported invoked interface dispatch
failing closed. PIB-511, PIB-519 and every other closed surface remain frozen.
No AR file is safe to stage; AS–AX remain blocked.

S7 AR rev-13 implements the bounded PIB-518 callable-selector transport
correction. A guard-only named-callable origin now preserves a concrete method
declaration together with either its bound receiver or unbound
method-expression semantics across assignments, function parameters and
function returns. Invocation—not materialization—activates the method, binds
the preserved receiver to its declared receiver object, and then binds
ordinary parameters at argument zero; transported method expressions retain
receiver-at-argument-zero and parameter offset one. Statically recoverable
interface method values resolve through typed concrete receiver origins;
invoked unresolved interface transport fails closed. Fresh PIB-511/518/519
package time is 287.673s, below the 325-second gate. The single authorized
unchanged cold observer passes all 15 targets in 306.83s test / 307.312s
package / 307.76s monotonic wall under the immutable 12m/8m/1m budgets.
Rev-13 is ready for independent review but is not approved or safe to stage;
AS–AX remain blocked.

S7 AR rev-13 independent review is **NEEDS REVISION**. Static review stopped
before all Go execution after one Medium PIB-518 false-pass was established:
direct interface method invocation reaches the named-callable resolver with
`transported == false`, so a known concrete interface receiver is not
recovered and an unknown dynamic receiver does not fail closed. Existing
interface sensitivities alias the method before invoking it and therefore
exercise only the transported path. Rev-14 is bounded to direct invocation
recovery/fail-closed behavior and complete-validator known/unknown direct
interface sensitivities in the AR guard plus this handoff. PIB-511, PIB-519
and every other closed surface remain frozen. No AR file is safe to stage;
AS–AX remain blocked.

S7 AR rev-14 test-first phase is complete. The exact direct-interface
reviewer repro parses and type-checks, mutates the production emitter with one
receiver-held canonical store, and invokes the complete PIB-518 validator.
Against the unchanged rev-13 resolver it failed as required at 10.27s test /
10.807s package / 15.02s wall because the validator returned nil and counted
zero writes instead of the expected four. Implementation is now confined to
separating invocation context from callable transport in the guard resolver;
no frozen surface has changed.

S7 AR rev-14 implements that bounded correction. Callable resolution now
carries invocation arguments/state separately from transport state. Direct or
transported local interface MethodVals with callable-bearing signatures
recover concrete value/pointer receiver origins when invoked; direct or
transported interface MethodExprs recover argument zero and preserve the
unbound receiver offset. Unknown invoked dispatch fails closed, while
materialized/stored/passed/returned-but-uninvoked interface method values
remain inert. Concrete receiver and receiver-field origin resolution now
follows explicit named-helper returns, closing returned direct dispatch
without broadening ordinary function-field authority. Fresh PIB-511/518/519
package time is 311.215s. The single authorized unchanged cold observer passes
all 15 targets in 395.37s test / 395.705s package / 396.16s monotonic wall.
Rev-14 is ready for independent review but is not approved or safe to stage;
AS–AX remain blocked.

S7 AR rev-14 independent review is **NEEDS REVISION**. Static review stopped
before all Go execution after one Medium PIB-518 false-pass was established:
`go/types` represents a variadic `invoke(...func())` parameter as `[]func()`,
but `s7ARSignatureCarriesCallable` recognizes only a tuple entry whose whole
type is function-valued. Known direct variadic MethodVal/MethodExpr dispatch
therefore skips receiver recovery, while unknown dispatch does not fail
closed. All rev-14 sensitivities use fixed-arity `invoke(func())`. Rev-15 is
bounded to recognizing only the final variadic slice element when it is
function-valued and adding complete-validator known/unknown MethodVal,
MethodExpr and inert-materialization sensitivities. PIB-511, PIB-519 and every
other closed surface remain frozen. No AR file is safe to stage; AS–AX remain
blocked.

S7 AP rev-2 review remains NEEDS REVISION on seven concrete gaps:
PIB-459 scans only dangling `case` bodies and misses alternatives in other
inventoried declarations; PIB-468 injects divergence after a completed removal
instead of preserving the pending record and every blob; PIB-476 still omits
divergence/remaining-repairs/planned-preview populations and checks only one
concatenation operand; PIB-478 host-type-checks the foreign GOOS source and
does not resolve the exact `syscall.Fstatfs` object; PIB-479 ignores Darwin
identifier-valued deny cases; and PIB-482's broad precedent allowance can mask
a positive extraction claim. AQ–AX remain blocked.

The guarded classifier entry point now passes isolated full/race/vet plus all
Linux, Darwin, Windows, FreeBSD, Plan9, JS and WASI compile gates. It is ready
for a separate micro commit before the final S6 fixture.

The classifier micro-prerequisite is complete at `971da91`; blocking CI
[32328091360](https://github.com/tesseracode/tesserapatch/actions/runs/32328091360)
is green on Ubuntu, macOS and Windows. S6 can now drive the real denied
filesystem classifier and finish its 53-entry public catalog.

The S6 rebuild is **blocked on implementation prerequisites**, not prose.
Contract/source verification confirms: §18.1 requires 37 named nil injection
seams that are mostly absent; rooted `DurableWrite` does not revalidate the
directory chain and final leaf after the rename seam/CAS, so PIB-148…151 cannot
be proved and a swapped symlink can be overwritten; and S4b emits
`archive-selector-invalid` although the closed refusal catalog contains no such
public code. S6 docs/assets stay uncommitted while one sequential prerequisite
hardening revision closes these production gaps.

Prerequisite rev-0 review returned NEEDS REVISION on four MEDIUM integration
findings: existing `FEATURES.md` refreshes now pass an implicit
expected-absent gate; status/control rename seams are classified by artifact ID
instead of exact write role and miss staging; `failPurgeAfterFirstMutation`
fires in orphan/unreferenced/absent branches that own other failure seams; and
`afterPurgeIndexRename` is skipped when the index rename committed but later
durability verification failed.

Prerequisite rev-1 folds all four: FEATURES refresh CASes its captured
identity; every rooted writer declares ordinary-canonical,
canonical-status or control role with exact seam sequences; the generic
first-mutation purge failure exists only after a pending-claim CAS; and the
post-index seam fires on every committed rename including later durability
error. Focused re-review is pending.

Prerequisite rev-1 re-review found one HIGH and two MEDIUM residuals. Temp
cleanup retains a parent descriptor opened independently from temp creation,
so an ancestor swap can make `unlinkat` delete the same basename in the wrong
directory or strand the owned temp. Write-role zero silently means ordinary
canonical instead of failing closed. The between-hashes seam can fire after an
already-complete hash that made no current-run mutation.

Prerequisite rev-2 closes those three. Re-review found one HIGH and one MEDIUM
residual: same-inode temp content can change after writing and still pass a
dev/inode-only final gate; and descriptor cleanup exists only on Linux
amd64/arm64 although mutation authority supports other non-Android Linux
architectures, causing those targets to create then strand a temp at exit 6.

Prerequisite rev-3 closes content hashing and all-Linux build coverage.
Re-review found two HIGH low-level residuals: same-size bytes can change after
`Pread` but before post-read `Fstat` unless mutation-sensitive mtime/ctime
nanoseconds are compared; and mips64/mips64le cannot pass Go's `Stat_t`
directly to raw `newfstatat` because the kernel layout needs the stdlib's
private conversion/wrapper.

Adjudication withdrew the coarse-timestamp prerequisite: accepted T1
post-rename verification guarantees tampered bytes cannot return success and
classifies the documented residual as exit 6. Review instead found two MEDIUM
exit-mapping bugs: committed raw-preimage divergence is hard-coded to
rolled-back exit 5, and archive append normalization overwrites an existing
exit 6 with resumable exit 5.

Prerequisite rev-5 preserves committed exit 6 across raw preimages, archive
blobs and manual status. Focused review returned APPROVED with every named
seam, rename-time gate, selector error, write role, descriptor-cleanup and
exit-mapping finding closed.

The approved prerequisite patch passes isolated tracked-state validation:
full uncached suite, full CLI/intentlock/intentpub/store race suites, vet, host
build, all 13 Linux architecture builds, Darwin amd64/arm64, Windows amd64 and
mips64/le intentpub test compilation.

The S6 prerequisite hardening is complete at `b37ba4c`; blocking CI
[32316113750](https://github.com/tesseracode/tesserapatch/actions/runs/32316113750)
is green on Ubuntu, macOS and Windows. S6 resumes against the now-complete
named seam, rename-time gate and closed selector behavior.

The current rev-1 implementation uses confined
`os.OpenInRoot` reads and exact-D9 selection bypasses the legacy feature
loader; unreadable/unstable regular files remain non-destructive unsafe
evidence; pending/repair classes aggregate to one finding and route; and the
48-row ledger is PRD-bound and AST-resolves runnable tests/literal subtests,
with strengthened PIB-221/323/380/381 observables.
Rev-0's pending-preview, predicted-remediation, terminal-recovery,
partial-class, divergence-shape and retry-heading defects are closed. Rev-1
findings are also closed. Rev-2's shell quoting and selector-precedence
findings are closed. Rev-3's final control-character path finding is closed by
one managed-path predicate at snapshot/plan report boundaries, with
non-echoing zero-write refusal and real newline/tab/ESC/DEL/C1 fixtures.
Focused rev-4 re-review returned APPROVED.

Pre-commit tracked-state validation found one stale S4 guard: AVP-134 admitted
the two prepare command files but not S4b's accepted
`feature_intent_archive.go` importer. The bounded correction pins the exact
three-file set and retains both extra-forbidden-importer and
missing-authorized-set sensitivities; no product behavior changes.
The corrected staged-state full and race gates are green.

Before that CI blocker, S4 was internally approved. Its first independent code review
returned NEEDS REVISION on exit-3 writes after stale cleanup/staging, use of a
pre-authority artifact snapshot, unsafe abandon rollback, lost deadline
classification and repeated human rollback sections. The revision re-inspects
under the held authority, moves every exit-2/3 gate ahead of cleanup/staging,
prevalidates V1–V5 without writing, preserves concurrent abandon evidence,
carries bounded deadline metadata and renders archive residue once. Serialized
normal and race suites pass across all five changed packages. Focused re-review
found one remaining exit-class demotion in staging failures; the final fold
preserves typed exit 6 for both base and archive-index post-rename durability
failures, and re-review returned APPROVED.

The `tesseracode/copilot-api` v0.15.1 feedback was independently triaged on
2026-08-18 and accepted at evidence commit `e6901a2` (range
`7206dab..e6901a2`). GH #18–#22 now track the confirmed migration gaps; the
diagnosis and evidence are archived in the cumulative-verification case study.
This backlog intake does not preempt the active prepare queue.

S7 AU implementation also surfaced **documentation** defects, not production
ones: three acceptance-matrix rows stated expectations the frozen rules and the
shipped product do not produce. `PIB-542`'s first fixture expected all four
selectors to refuse exit 3 over an archive holding one repair class (the mixed
hash `h₂`) beside a healthy `h₃`, which contradicts the frozen rev-12 /
ADR-035 D16 rule that `--all --yes` is admitted when the chosen class is the
archive's only class — a healthy hash is in no class and makes no second class.
`PIB-543` required exit 3 from **every** observer and one procedure
presentation from all five, while the product's `doctor` D9 is warning-only and
exits 0 (D16 already says so) and the same corrupt-object route is printed in
two presentations. `PIB-544` named a driver seam set that is not the one the
four §9.7.2 insertion windows are reached through.
`PRD-prepare-intent-bundle` **rev-17** and `ADR-035` **rev-17** are raised as a
paired **no-decision erratum**, dated 2026-08-27, **acceptance pending joint
review**. They correct those three rows only: for `PIB-542`, `--orphans`,
`--blob <h₃>` and `--generation <id₃>` keep refusing exit 3 zero-write while
`--all --yes` is admitted, prints its whole-archive blast-radius disclosure and
purges the archive at exit 0; for `PIB-543`, `prepare`, `--regenerate`,
`--manual` and `list` exit 3 while `doctor` reports the same observation as a
warning and exits 0, with `list` carrying the quoted procedure plus the
structured `retry`/`retry_cwd` pair, `doctor` the same quoted procedure lines
without that pair, and the three mutating readers the same route as refusal
`remediation` prose over the unquoted managed path; for `PIB-544`, the driver
list becomes the store fixture
`TestRecoverPendingPurgeInsertionWindowsPIB544`'s own seam mapping, with
`afterPurgeBlobRevalidate` excluded as PIB-550's fifth-window seam. The clean
control, the two-mixed-hash fixture, the two-class residue-plus-mixed fixture
and every window outcome are unchanged. No D16 normative text, decision, exit
code the product emits, state, row ID, kind, category or count changes; the
matrix stays at **567** rows with **thirty-six** semantic guards, and the AU
fixtures follow the corrected rows rather than the AU tests weakening their
expectations.

S7 AV is at **rev-2** in the worktree. Review of rev-1 returned five findings,
all folded here without touching production. The load-bearing one is `PIB-550`:
rev-1 recorded a PRD-row/implementation divergence rather than resolving it.
The adjudication is a paired **rev-18 no-decision erratum** in the PRD and
ADR-035, dated 2026-08-28 and **acceptance pending joint review**. The shipped
removal is conditioned on a fresh identity re-probe taken inside
`prepareArchiveStorage.RemoveBlob`, so an external replacement landing at
`beforePurgeBlobRemove` is refused with the replacement preserved (exit 5
`archive-purge-partial`, rerun exit 6 `archive-purge-evidence-divergent`), and
the genuinely open residual narrows to the unseamed gap between that re-probe
and `root.Remove`. The erratum amends `PIB-550`, §9.7.2's fourth window row,
its closing residual paragraph, §9.7.2's exit-6 atomicity note, §21's
alternatives row, the §18.53 sensitivity entries of `PIB-546` and `PIB-551`,
and ADR-035's paired D16 residual prose and alternatives row. **No decision,
normative rule, row ID, kind, category, count, command, exit code or emitted
state changes**; the matrix stays at **567** rows with **thirty-six** semantic
guards, and ADR-035's decisions D1–D21 stand exactly as accepted.

## Active Task

- **Task ID**: `implement-prepare-intent-bundle`
- **Issue**: [GH #23](https://github.com/tesseracode/tesserapatch/issues/23)
- **Description**: Implement the mutating `tpatch prepare <slug>` intent-bundle
  contract from the accepted `PRD-prepare-intent-bundle` rev-15 +
  `ADR-035-intent-bundle-publication-and-history` rev-15 (ADR-035 normative
  where they overlap).
- **Status**: **In progress — rev-20 checkpointed at `9f7095c`; CI next**
- **Assigned**: 2026-08-18
- **WAVE_BASE**: `3b579fc7243bf0d1b21605d3c87562226f1fd936`
- **Release tag**: TBD; the accepted `prepare --check` prerequisite will ship
  with this release

## Prerequisite Status

PRD §19's three acceptance conditions are now all satisfied:

1. `PRD-prepare-intent-bundle` Accepted at rev-15 (2026-08-18).
2. `ADR-035` Accepted at rev-15 (2026-08-18), reviewed jointly with the PRD.
3. §19(3) — the accepted `prepare --check` contract
   (`PRD-artifact-validation-and-provenance` rev-5 / rev-6 errata + `ADR-034`
   rev-2 / rev-3 errata) has frozen implementation content at `cacaaf8` and
   was formally accepted/closed at `7206dab`.

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
- [GH #18](https://github.com/tesseracode/tesserapatch/issues/18) — cumulative
  verify semantics and a truthful migration assessment.
- [GH #19](https://github.com/tesseracode/tesserapatch/issues/19) — Path B
  manual recipe provenance publication and safe adoption.
- [GH #20](https://github.com/tesseracode/tesserapatch/issues/20) — honest
  legacy patch-generation adoption without fabricated provenance.
- [GH #21](https://github.com/tesseracode/tesserapatch/issues/21) — guarded
  mechanical recipe-schema migrations through doctor.
- [GH #22](https://github.com/tesseracode/tesserapatch/issues/22) — durable
  later-touch acknowledgement that does not weaken preimage/replay safety.

## Downstream Feedback Assessment

- Exact installed `tpatch v0.15.1` result at `copilot-api@e2d7ce4`: 0 passed,
  53 failed, 3 skipped, 0 error; failing checks 38 V8 / 16 V7 / 6 V10 /
  1 intent.
- Downstream health remains green: typecheck, lint, 352 tests and build.
- 29 of 38 V8-failing patches apply at their own recorded base; nine do not.
  Own-base verification is therefore useful evidence, not sufficient proof.
- Four recent V10 failures are missing `recipe-provenance.json`, not measured
  stale hashes. All 11 non-empty preimages match their recorded base.
- Doctor confirms 24 D2 manifests cannot be produced by the recommended
  same-byte refresh. All 24 have reachable candidate bases in `status.json`,
  but only six patches apply to them; eight historical D7 recipes required a
  manual mechanical migration.
- Full evidence and limits:
  `docs/state-of-the-art/case-studies/copilot-api-cumulative-verify-2026-08/summary.md`.

## Files Changed

- `internal/cli/prepare.go`
- `internal/cli/prepare_publish.go`
- `internal/cli/prepare_publish_s4_test.go`
- `internal/cli/prepare_test.go`
- `internal/cli/prepare_avp_test.go`
- `internal/cli/prepare_avp2_test.go`
- `internal/cli/prepare_pib_golden_test.go`
- `internal/gitutil/ignore.go`
- `internal/gitutil/ignore_prepare_test.go`
- `internal/intentpub/stage.go`
- `internal/intentpub/plan_stage_hardening_test.go`
- `internal/intent/avp_source_scans_test.go`
- `internal/rescap/gitgate.go`
- `internal/rescap/scratch.go`
- `internal/workflow/session_ignore.go`
- `docs/handoff/CURRENT.md`
- `docs/ROADMAP.md`
- `docs/supervisor/LOG.md`
- S4b implementation:
  `internal/cli/feature_intent_archive.go`,
  `internal/cli/feature_intent_archive_test.go`, and the registration line in
  `internal/cli/feature_deps.go`.
- S4b tracked-source correction:
  `internal/intent/avp_source_scans_test.go`.
- S5 implementation:
  `internal/workflow/doctor_d9.go`,
  `internal/workflow/doctor_d9_test.go`,
  `internal/workflow/doctor_d9_s5_matrix_test.go`,
  `internal/cli/doctor_d9_test.go`,
  `internal/cli/prepare_s5_test.go`, plus the bounded doctor registry/help/test
  deltas.
- S5 contract/guard corrections:
  `docs/prds/PRD-artifact-validation-and-provenance.md`,
  `internal/intent/avp_source_scans_test.go`, and
  `internal/cli/prepare_pib_golden_windows_test.go`.
- S7 AM–AO rev-5:
  `.github/workflows/ci.yml`,
  `internal/cli/prepare_s7_am_ledger_test.go`,
  `internal/cli/prepare_s7_an_archive_test.go`,
  `internal/cli/prepare_s7_ao_guard_test.go`,
  `internal/cli/prepare_s7_ao_runtime_test.go`,
  `internal/cli/prepare_s7_bsd_guard_test.go`,
  `internal/cli/prepare_s7_rev16_test.go`,
  `internal/gitutil/ignore.go`,
  `internal/gitutil/ignore_prepare_test.go`,
  `internal/gitutil/s7_ao_guard_test.go`,
  `internal/intentlock/s7_am_source_guard_test.go`,
  `internal/store/s7_am_archive_test.go`,
  `internal/store/s7_ao_parity_test.go`, and
  `internal/workflow/s7_an_privacy_test.go`.
  Rev-5's bounded delta is in
  `internal/cli/prepare_s7_am_ledger_test.go`,
  `internal/cli/prepare_s7_ao_runtime_test.go`,
  `internal/gitutil/ignore_prepare_test.go`, and
  `internal/workflow/s7_an_privacy_test.go`; the unauthorized production
  `internal/gitutil/ignore.go` edit was fully reverted.
- Rev-6's bounded delta is in
  `internal/cli/feature_intent_archive_test.go`,
  `internal/cli/prepare_s7_am_ledger_test.go`,
  `internal/cli/prepare_s7_an_archive_test.go`,
  `internal/cli/prepare_s7_ao_guard_test.go`,
  `internal/cli/prepare_s7_ao_runtime_test.go`,
  `internal/cli/prepare_s7_rev16_test.go`,
  `internal/gitutil/ignore.go`,
  `internal/gitutil/ignore_prepare_test.go`,
  `internal/store/s7_am_archive_test.go`, and
  `internal/workflow/s7_an_privacy_test.go`.
- Rev-7's bounded delta is in
  `internal/cli/prepare_s7_am_ledger_test.go`,
  `internal/cli/prepare_s7_registration_test.go`,
  `internal/store/s7_ao_parity_test.go`, and
  `internal/workflow/s7_an_privacy_test.go`.
- Rev-8's bounded delta is in
  `internal/s7marker/marker.go`,
  `internal/cli/prepare_s7_am_ledger_test.go`,
  `internal/cli/prepare_s7_registration_test.go`,
  `internal/cli/prepare_s7_process_unix_test.go`,
  `internal/cli/prepare_s7_process_other_test.go`, and
  `internal/workflow/s7_an_privacy_test.go`.
- Rev-9's bounded delta is in
  `internal/s7marker/marker.go`,
  `internal/cli/prepare_s7_am_ledger_test.go`,
  `internal/cli/prepare_s7_registration_test.go`, and
  `internal/workflow/s7_an_privacy_test.go`.
- Rev-10's bounded delta is in
  `internal/s7marker/marker.go`,
  `internal/cli/prepare_s7_am_ledger_test.go`,
  `internal/cli/prepare_s7_registration_test.go`,
  `internal/cli/testdata/s7registration/fixture_test.go`, and
  `internal/workflow/s7_an_privacy_test.go`.
- S7 AP's bounded delta is in
  `internal/cli/prepare.go`,
  `internal/cli/prepare_publish.go`,
  `internal/cli/prepare_s7_registration_test.go`,
  `internal/cli/prepare_s6_contract_test.go`,
  `internal/cli/prepare_s7_ap_support_test.go`,
  `internal/cli/prepare_s7_ap_ledger_test.go`,
  `internal/cli/prepare_s7_ap_abandon_test.go`,
  `internal/cli/prepare_s7_ap_dryrun_test.go`,
  `internal/cli/prepare_s7_ap_dryrun_windows_test.go`,
  `internal/cli/prepare_s7_ap_dangling_test.go`,
  `internal/cli/prepare_s7_ap_git_test.go`,
  `internal/cli/prepare_s7_ap_purge_test.go`,
  `internal/cli/prepare_s7_ap_guards_test.go`,
  `internal/cli/prepare_s7_ap_filesystem_test.go`,
  `internal/cli/prepare_s7_platform_guard_test.go`,
  `internal/store/s7_ap_purge_test.go`,
  `internal/intentpub/s7_ap_writer_test.go`,
  `internal/intentlock/s7_ap_authority_test.go`,
  `internal/intentlock/s7_ap_authority_darwin_test.go`,
  `internal/intentlock/s7_ap_authority_linux_test.go`,
  `internal/gitutil/s7_ap_git_test.go`,
  `internal/rescap/s7_ap_git_compat_test.go`,
  and
  `internal/workflow/s7_ap_doctor_test.go`.
- S7 AP rev-2's bounded delta is in
  `internal/cli/prepare_s7_ap_abandon_test.go`,
  `internal/cli/prepare_s7_ap_dangling_test.go`,
  `internal/cli/prepare_s7_ap_purge_test.go`,
  `internal/cli/prepare_s7_ap_git_test.go`,
  `internal/cli/prepare_s7_ap_guards_test.go`,
  `internal/intentlock/s7_ap_authority_test.go`, and
  `internal/intentlock/s7_ap_authority_linux_test.go`.
- S7 AP rev-3's bounded delta is in
  `internal/store/intent_archive.go`,
  `internal/store/s7_ap_purge_test.go`,
  `internal/cli/prepare_s7_ap_dangling_test.go`,
  `internal/cli/prepare_s7_ap_purge_test.go`,
  `internal/cli/prepare_s7_ap_git_test.go`,
  `internal/cli/prepare_s7_ap_guards_test.go`,
  `internal/cli/prepare_s7_ap_ledger_test.go`, and
  `internal/intentlock/s7_ap_authority_test.go`.
- S7 AP rev-4's bounded delta is in
  `internal/cli/prepare_s7_ap_dangling_test.go`,
  `internal/cli/prepare_s7_ap_purge_test.go`,
  `internal/cli/prepare_s7_ap_git_test.go`,
  `internal/cli/prepare_s7_ap_guards_test.go`,
  `internal/cli/prepare_s7_ap_ledger_test.go`,
  `internal/store/s7_ap_purge_test.go`,
  `internal/intentlock/s7_ap_authority_test.go`,
  `internal/workflow/s7_ap_doctor_test.go`, and
  `internal/workflow/doctor.go`. The prior PIB-468 production-only typed-error
  exception in `internal/store/intent_archive.go` is absent from the final
  worktree.
- S7 AP rev-5's bounded delta is test/tracking-only:
  `internal/workflow/s7_ap_doctor_test.go`,
  `internal/cli/prepare_s7_ap_git_test.go`,
  `internal/cli/prepare_s7_ap_guards_test.go`,
  `internal/intentlock/s7_ap_authority_test.go`, and this handoff. No
  production file changed.
- S7 AP rev-6's bounded delta is test/tracking-only:
  `internal/cli/prepare_s7_ap_git_test.go`,
  `internal/cli/prepare_s7_ap_guards_test.go`,
  `internal/intentlock/s7_ap_authority_test.go`, and this handoff. No
  production file changed.
- S7 AP rev-7's bounded delta is test/tracking-only:
  `internal/cli/prepare_s7_ap_git_test.go`,
  `internal/cli/prepare_s7_ap_guards_test.go`,
  `internal/intentlock/s7_ap_authority_test.go`, and this handoff. No
  production file changed.
- S7 AP rev-8's bounded delta is test/tracking-only:
  `internal/cli/prepare_s7_ap_git_test.go`,
  `internal/cli/prepare_s7_ap_guards_test.go`,
  `internal/intentlock/s7_ap_authority_test.go`, and this handoff. No
  production file changed.
- S7 AP rev-9's bounded delta is test/tracking-only:
  `internal/cli/prepare_s7_ap_git_test.go`,
  `internal/cli/prepare_s7_ap_guards_test.go`,
  `internal/intentlock/s7_ap_authority_test.go`, and this handoff. No
  production file changed.
- S7 AP rev-10's bounded delta is test/tracking-only:
  `internal/cli/prepare_s7_ap_guards_test.go`,
  `internal/intentlock/s7_ap_authority_test.go`, and this handoff. No
  production file changed.
- S7 AP rev-11's bounded delta is test/tracking-only:
  `internal/intentlock/s7_ap_authority_test.go` and this handoff. No
  production file changed.
- S7 AP rev-12's bounded delta is test/tracking-only:
  `internal/intentlock/s7_ap_authority_test.go` and this handoff. No
  production file changed.
- The bounded AP post-acceptance CI correction is test/workflow/tracking-only:
  `.github/workflows/ci.yml`,
  `internal/cli/prepare_s7_ap_support_test.go`,
  `internal/cli/prepare_s7_ap_dryrun_test.go`,
  `internal/cli/prepare_s7_ap_dryrun_windows_test.go`,
  `internal/cli/prepare_s7_ap_ledger_test.go`,
  `internal/cli/prepare_s7_platform_guard_test.go`,
  `internal/cli/prepare_s7_registration_test.go`, and this handoff. No
  production file changed.
- S7 AQ's bounded delta is in
  `internal/cli/prepare_publish.go`,
  `internal/cli/prepare_s7_aq_abandon_test.go`,
  `internal/cli/prepare_s7_aq_docs_test.go`,
  `internal/cli/prepare_s7_aq_flags_test.go`,
  `internal/cli/prepare_s7_aq_ledger_test.go`,
  `internal/cli/prepare_s7_aq_privacy_test.go`,
  `internal/cli/prepare_s7_aq_purge_test.go`,
  `internal/cli/prepare_s7_aq_recovery_test.go`,
  `internal/cli/prepare_s7_aq_retry_test.go`,
  `internal/cli/prepare_s7_registration_test.go`,
  `internal/cli/prepare_s7_ap_git_test.go`,
  `internal/cli/prepare_s7_ap_purge_test.go`,
  `internal/intentlock/s7_aq_authority_test.go`, and this handoff. Production
  changes are limited to the recovery-error evidence formatter and its call
  site in `prepare_publish.go`.
- S7 AQ rev-1 is test/tracking-only:
  `internal/cli/prepare_s7_aq_docs_test.go`,
  `internal/cli/prepare_s7_aq_flags_test.go`,
  `internal/cli/prepare_s7_aq_privacy_test.go`,
  `internal/cli/prepare_s7_aq_purge_test.go`,
  `internal/cli/prepare_s7_aq_recovery_test.go`,
  `internal/cli/prepare_s7_ap_git_test.go`,
  `internal/intentlock/s7_aq_authority_test.go`, and this handoff. No
  production behavior changed in rev-1.
- S7 AQ rev-2 is test/tracking-only:
  `internal/cli/prepare_s7_aq_docs_test.go`,
  `internal/cli/prepare_s7_aq_privacy_test.go`,
  `internal/cli/prepare_s7_aq_recovery_test.go`,
  `internal/intentlock/s7_aq_authority_test.go`, and this handoff. No
  production behavior changed in rev-2.
- S7 AQ rev-3 is test/tracking-only:
  `internal/cli/prepare_s7_aq_docs_test.go`,
  `internal/cli/prepare_s7_aq_privacy_test.go`,
  `internal/cli/prepare_s7_aq_recovery_test.go`,
  `internal/intentlock/s7_aq_authority_test.go`, and this handoff. No
  production behavior changed in rev-3.
- S7 AQ rev-4 is test/tracking-only:
  `internal/cli/prepare_s7_aq_privacy_test.go`,
  `internal/cli/prepare_s7_aq_recovery_test.go`, and this handoff. No
  production behavior changed in rev-4.
- S7 AQ rev-5 is test/tracking-only:
  `internal/cli/prepare_s7_aq_privacy_test.go` and this handoff. No production
  behavior changed in rev-5.
- The bounded AQ staged-source correction is test/tracking-only:
  `internal/cli/prepare_s7_ap_dangling_test.go`,
  `internal/cli/prepare_s7_aq_recovery_test.go`, and this handoff. No production
  behavior changed beyond AQ's already accepted recovery-evidence-path delta.
- The bounded AQ CI timeout correction is workflow/test/tracking-only:
  `.github/workflows/ci.yml`,
  `internal/intent/avp_guards_test.go`, and this handoff. No production or
  AP/AQ behavior changed.
- The bounded frozen-source correction restores
  `internal/intent/avp_guards_test.go` exactly and moves timeout assertions to
  `internal/cli/prepare_s7_ci_timeout_guard_test.go`; the workflow and this
  handoff are the only other relevant files. No production or AQ behavior
  changed.
- The bounded frozen-source S7 guard revision is test/tracking-only:
  `internal/cli/prepare_s7_ci_timeout_guard_test.go` and this handoff. Frozen
  AVP source, workflow commands and production behavior are unchanged.
- The bounded nested-shell S7 guard revision is test/tracking-only:
  `internal/cli/prepare_s7_ci_timeout_guard_test.go` and this handoff. Frozen
  AVP source, workflow commands and production behavior are unchanged.
- The bounded dynamic-executable S7 guard revision is test/tracking-only:
  `internal/cli/prepare_s7_ci_timeout_guard_test.go` and this handoff. Frozen
  AVP source, workflow commands and production behavior are unchanged.
- The bounded AQ observer-budget correction is test/tracking-only:
  `internal/cli/prepare_s7_registration_test.go` and this handoff. Workflow,
  frozen AVP source and production behavior are unchanged.
- The bounded observer-budget callsite-binding revision is test/tracking-only:
  `internal/cli/prepare_s7_registration_test.go` and this handoff. Workflow,
  frozen AVP source and production behavior are unchanged.
- S7 AR rev-0 was test/tracking-only:
  `internal/cli/prepare_s7_ar_runtime_test.go`,
  `internal/cli/prepare_s7_ar_guard_test.go`,
  `internal/cli/prepare_s7_ar_ledger_test.go`,
  `internal/cli/prepare_s7_registration_test.go`, and this handoff. No
  production, workflow, frozen AVP or asset source changed.
- S7 AR rev-1 additionally changes
  `internal/cli/feature_intent_archive.go` for the two reviewer-authorized
  rendering corrections, and hardens the same four AR test/observer files plus
  this handoff. `docs/supervisor/LOG.md` remains reviewer-owned and unchanged
  by the implementation pass.
- S7 AR rev-2 is test/tracking-only:
  `internal/cli/prepare_s7_ar_runtime_test.go`,
  `internal/cli/prepare_s7_ar_guard_test.go`, and this handoff. The accepted
  `internal/cli/feature_intent_archive.go` rendering and
  `internal/cli/prepare_s7_registration_test.go` observer budget are unchanged.
  `docs/supervisor/LOG.md` remains reviewer-owned and was not edited by this
  revision.
- S7 AR rev-3 is test/tracking-only:
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. The accepted
  production renderer, runtime tests, ledger and observer/registration source
  remain byte-identical to the rev-2 worktree baseline.
- S7 AR rev-4 is test/tracking-only:
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. No production,
  runtime, ledger, observer/registration, workflow, frozen AVP or asset source
  changed in rev-4.
- S7 AR rev-5 is test/tracking-only:
  `internal/cli/prepare_s7_ar_runtime_test.go`,
  `internal/cli/prepare_s7_ar_guard_test.go`, and this handoff. The runtime
  delta is restricted to resolving PIB-510's source graph before installing
  its Git/PATH spy. No production, ledger, observer/registration, workflow,
  frozen AVP or asset source changed in rev-5.
- S7 AR rev-6 is test/tracking-only:
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. PIB-510 and the
  AR runtime test are closed. Production, ledger, observer/registration,
  workflow, frozen AVP, assets and every other closed surface remain frozen.
- S7 AR rev-7 is test/tracking-only:
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. Production,
  runtime, ledger, observer/registration, workflow, frozen AVP, assets,
  PRD/ADR and every other closed surface remain frozen.
- S7 AR rev-8 is test/tracking-only:
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. It may correct
  only the four rev-7 guard findings. PIB-511 now binds the evidence-present
  authority override and arbitrary address/callable indirection; PIB-518
  resolves forwarded invoked callables and deeper canonical/decoy pointer
  origins; PIB-519 validates exact argv, full suffix context, target-local
  references and active/passive recoverability grammar. Immutable claim
  candidates are cached only under exact source-name/body hashes, and the
  lighter PIB-511 program retains the complete typed package without S6's
  unrelated emission-domain fixed point. Production, runtime, ledger,
  observer/registration, workflow, frozen AVP, assets, PRD/ADR and every other
  closed surface remain frozen.
- S7 AR rev-9 is test/tracking-only:
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. It may correct
  only the four rev-8 guard findings: bind the pending-evidence helper's
  positive predicate and return flow; resolve invoked callbacks returned by
  local literals or named helpers; enforce positive exact route/reference and
  normalized manual-target semantics; and inventory the named equivalent
  recoverability constructions with polarity controls. Production, runtime,
  ledger, observer/registration, workflow, frozen AVP, assets, PRD/ADR and
  every other closed surface remain frozen. A semantic-preserving performance
  fold is authorized within the same guard/tracking boundary. It must retain
  every rev-0…rev-9 sensitivity and may run one new cold observer only after
  fresh PIB-511/518/519 package times total <=325 seconds.
- S7 AR rev-10 is test/tracking-only:
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. It closes the
  five rev-9 findings but remains rejected on method-expression callback
  binding, dependent-file cache invalidation, selector-specific route values
  and negative-quantifier anti-stranding inventory. Production, runtime,
  ledger, observer/registration, workflow, frozen AVP, assets, PRD/ADR and
  every other closed surface remain frozen.
- S7 AR rev-11 is restricted to
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. It may correct
  only the four rev-10 findings. The review transition also prepends
  `docs/supervisor/LOG.md`; no other surface is authorized.
- S7 AR rev-12 is restricted to
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. It may correct
  only the three rev-11 findings: bind method-expression receiver origins into
  callable receiver fields; apply production's complete slug predicate to
  every manual route; and inventory equivalent punctuated/reordered
  negative-quantifier anti-stranding guarantees while preserving all
  exclusions. The review transition also prepends `docs/supervisor/LOG.md`;
  no other surface is authorized. The implementation changed exactly those
  two authorized files; production and every frozen test surface remain
  untouched.
- S7 AR rev-13 is restricted to
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. It may correct
  only the rev-12 PIB-518 finding: retain a selected method and its bound
  receiver together as callable origins across local aliases, arguments and
  returns, then activate and bind them when invoked. Unsupported invoked
  interface dispatch must fail closed. Same-validator sensitivities must
  cover local, passed and returned value/pointer method values plus inert,
  ordinary-function and interface controls. PIB-511, PIB-519 and every other
  closed surface remain frozen. The review transition also prepends
  `docs/supervisor/LOG.md`; no other surface is authorized. Implementation
  changed exactly the guard and this handoff; every frozen source remains
  byte-identical.
- S7 AR rev-14 is restricted to
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. It may correct
  only the rev-13 PIB-518 finding: direct interface method invocation must
  recover a known concrete receiver regardless of transport and unresolved
  direct interface dispatch must fail closed. Direct interface method
  expressions must recover their argument-zero receiver or fail closed.
  Same-validator sensitivities must cover known and unknown direct interface
  invocation plus inert and ordinary function-field controls. PIB-511,
  PIB-519 and every other closed surface remain frozen. The review transition
  prepends `docs/supervisor/LOG.md`; no other surface is authorized.
  Implementation changed exactly the guard and this handoff; production and
  every frozen test/source surface remain byte-identical.
- S7 AR rev-15 is restricted to
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. It may correct
  only the rev-14 PIB-518 finding: a local interface method whose final
  variadic parameter has a function-valued slice element must enter the same
  concrete-receiver recovery/fail-closed path as the fixed-arity form.
  Unrelated slice/container parameters must not become callable authority.
  Complete-validator sensitivities must independently cover known and unknown
  direct variadic MethodVal and MethodExpr invocation plus inert
  materialization, without weakening any fixed-arity sensitivity. PIB-511,
  PIB-519 and every other closed surface remain frozen. The review transition
  prepends `docs/supervisor/LOG.md`; no other surface is authorized.
  Implementation changed exactly the guard and this handoff; production and
  every frozen test/source surface remain byte-identical.
- S7 AR rev-16 is restricted to
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. It corrects only
  the rev-15 PIB-518 finding: preserve every explicit variadic callback,
  retain fixed-prefix and MethodExpr receiver offsets, and recover known
  `slice...` callback origins or fail closed. Complete-validator sensitivities
  cover later MethodVal/MethodExpr callbacks, grouped fixed parameters,
  named helper/slice/element aliases, unresolved expansion, and all inherited
  inert/container/ordinary controls. Implementation changed exactly the guard
  and this handoff; production and every frozen test/source surface remain
  byte-identical.
- S7 AR rev-17 is restricted to
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. Guard lines
  5219–5700 add 20 complete-validator repro/sensitivity cases; 6083–6102 add
  ordered callable origin state; 7435–7838 preserve variadic binding and gate
  expansion evidence on invocation; 7839–8327 resolve ordered
  composite/index/dynamic origins; and 8328–8698 model bounded local sequence
  assignment, alias, index, copy, conditional and reslice state. No
  production, runtime, ledger, observer/registration, AVP, workflow, asset,
  PRD/ADR/SPEC or research surface changed.
- S7 AR rev-18 remains restricted to
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. Guard lines
  5613–6047 add 28 complete-validator sensitivities; 6547–6615 define the
  exact/all invocation-demand domain; 8130–8583 perform demand-scoped
  expansion and forwarding; 9154–9597 preserve ordered local sequence and
  backing identity; and 9598–9872 analyze append/call/helper mutation effects.
  Production, runtime, ledger, observer/registration, AVP, workflow, asset,
  PRD/ADR/SPEC, LOG, research and root-artifact surfaces remain frozen.
- S7 AR rev-19 remains restricted to
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. Guard lines
  6048–6732 add 41 complete-validator sensitivities; 8954–9331 define and
  clone the correlated reaching-state domain; 9332–10309 implement
  statement/control-flow transfer, backing overrides and exact call
  observations; 10310–10767 apply reaching state to target resolution,
  interprocedural binding and forwarding; and 11907–11958 replace the
  flow-insensitive helper-mutation summary. Production, runtime, ledger,
  observer/registration, AVP, workflow, asset, PRD/ADR/SPEC, LOG, research and
  root-artifact surfaces remain frozen.
- S7 AR rev-20 remains restricted to
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. Guard lines
  6733–7317 add 37 complete-validator sensitivities; 8194–8216 retain the
  fixed-slice direct-invocation exclusion; 9679–10492 define, clone, compare,
  schedule and publish correlated execution states; 10493–11369 apply
  program-order assignments plus exact/ranged backing mutations; and
  11697–12050 seed every active function and compare branch sequences by
  semantic object identity. Production, runtime, ledger,
  observer/registration, AVP, workflow, asset, PRD/ADR/SPEC, LOG, research and
  root-artifact surfaces remain frozen.
- S7 AR rev-21 remained restricted to
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. It corrected
  cross-helper async escape, scheduled direct-literal closure effects and LIFO
  defer replay, then added a late captured-sink helper correction after its
  observer had been consumed.
- S7 AR rev-22 changed only this handoff while validating the frozen rev-21
  final guard. The parent review transition prepends
  `docs/supervisor/LOG.md`; no code, test, contract, asset, workflow or
  research surface changed.
- S7 AR rev-23 is restricted to
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. It may correct
  only scheduled zero-argument closures invoked through reaching-state scalar
  aliases, including registration-time target capture and fail-closed
  ambiguity. The parent dispatch prepends `docs/supervisor/LOG.md`; all other
  production, test, contract, asset, workflow and research surfaces are
  frozen.
- S7 AR rev-29 validation-only changed `docs/handoff/CURRENT.md` only. No
  code, test, asset, workflow, contract, LOG, staging or Git ref changed.
- S7 AR final checkpoint:
  `internal/cli/feature_intent_archive.go`,
  `internal/cli/prepare_s6_contract_test.go`,
  `internal/cli/prepare_s7_ar_guard_test.go`,
  `internal/cli/prepare_s7_ar_ledger_test.go`,
  `internal/cli/prepare_s7_ar_runtime_test.go`, and
  `internal/cli/prepare_s7_registration_test.go`. The reviewed code/test block
  is committed at `db17262`.
- S7 AU (uncommitted worktree, rev-5): the three new AU files
  `internal/cli/prepare_s7_au_ledger_test.go`,
  `internal/cli/prepare_s7_au_runtime_test.go` and
  `internal/cli/prepare_s7_au_guard_test.go`; the registration/CI wiring in
  `internal/cli/prepare_s7_registration_test.go`,
  `internal/cli/prepare_s7_ci_timeout_guard_test.go` and
  `.github/workflows/ci.yml`; the contract documents
  `docs/prds/PRD-prepare-intent-bundle.md` and
  `docs/adrs/ADR-035-intent-bundle-publication-and-history.md` (rev-17
  erratum); `docs/supervisor/LOG.md`; and this handoff. The **only**
  uncommitted production change is the three-line uncommitted-CAS exit-class
  clamp in `internal/store/intent_archive.go` that the PIB-544 fifth fixture
  depends on. `internal/store/intent_archive_purge_test.go` and the rev-3
  `SelectorAll` admission guard were reverted and are no longer part of this
  slice.
- S7 AV (uncommitted worktree, rev-1): the three new AV files
  `internal/cli/prepare_s7_av_ledger_test.go`,
  `internal/cli/prepare_s7_av_guard_test.go` and
  `internal/cli/prepare_s7_av_runtime_test.go`; the observer/CI wiring in
  `internal/cli/prepare_s7_registration_test.go`,
  `internal/cli/prepare_s7_ci_timeout_guard_test.go` and
  `.github/workflows/ci.yml`; and this handoff. **No production source or asset
  is touched by AV** — every AV row was satisfiable against the shipped seams,
  so no production change was demonstrated as needed.
- S7 AV rev-2 (uncommitted worktree) additionally touches
  `docs/prds/PRD-prepare-intent-bundle.md` and
  `docs/adrs/ADR-035-intent-bundle-publication-and-history.md` (the paired
  rev-18 erratum) and `internal/cli/prepare_s7_rev16_test.go` (the erratum
  evidence guard, which pins both documents against base
  `2d9492cbf6fd9c69c5aa75d64d05983c05e1563f` and therefore had to learn the
  rev-18 regions, the seventh changed matrix row and the 16/17/18 revision
  tail). `internal/cli/prepare_s7_av_guard_test.go` and
  `internal/cli/prepare_s7_av_runtime_test.go` carry the four test-side folds.
  **Production, assets, ROADMAP and LOG remain untouched**, and the six AV
  ledger rows still map to the same six exact top-level targets.

## Test Results

- S7 AU rev-5 — full AU/store/CI suite **PASS** in 12.291s CLI / 0.251s
  store; final focused AU suite **PASS** in 12.081s; exact nine-target observer
  **PASS** in 15.398s package (15.12s test). `gofmt`, affected vet and
  `go build ./cmd/tpatch` pass.

- S7 AR final strict eligibility — PASS: PIB-518 121.776s, PIB-511 102.177s,
  PIB-519 31.983s; package sum 255.936s, 69.064s below the 325-second cap.
- S7 AR final observer — PASS on its sole authorized run: 268.44s test,
  268.835s package, 269.279s monotonic wall under the 419-second cutoff.
  Independent rev-31 static and rev-32 evidence reviews both returned
  APPROVED. Post-observer validation was intentionally read-only.
- `go test -p=1 -count=1 ./...` — PASS.
- Serialized race coverage for all five S4 packages — PASS; the final revision
  additionally reran the five transaction-order/fault regressions under
  `-race`.
- Exact 51-fixture `TestPreparePIBPreChangeGoldens` — PASS; no fixture was
  re-recorded.
- `gofmt -l .`, `go vet ./...`, host build and Linux amd64, Darwin amd64 and
  Windows amd64 cross-builds — PASS.
- S2 prerequisite CI
  [32229096085](https://github.com/tesseracode/tesserapatch/actions/runs/32229096085)
  — green on Ubuntu, macOS and Windows.
- Side Research EOF tail remains
  `b385fe622db9926f48861105239f113e`.
- S4 blocking CI
  [32280073787](https://github.com/tesseracode/tesserapatch/actions/runs/32280073787)
  — FAILED only AVP-134 and AVP-141 on stale pre-S4 tracked-source allowlists.
- Corrected AVP-134/AVP-141 targeted tests, the exact 51-fixture/provenance
  guards and `go test -p=1 -count=1 ./...` from the tracked S4 state — PASS.
- Corrected blocking CI
  [32281269945](https://github.com/tesseracode/tesserapatch/actions/runs/32281269945)
  — PASS on Ubuntu, macOS and Windows; release job correctly skipped.
- S4b rev-0 targeted `TestFeatureIntentArchive*`, affected feature-deps/AVP
  guards, gofmt and diff check — PASS before review; verdict NEEDS REVISION on
  seven substantive state/report/sensitivity findings.
- S4b rev-1 closes all seven rev-0 findings and its targeted tests pass;
  re-review remains NEEDS REVISION on six narrower retry/list/schema/spy
  findings.
- S4b rev-2 closes those six findings; re-review remains NEEDS REVISION on
  shell-safe corrupt-object repair paths and selector gate precedence.
- S4b rev-3 closes both rev-2 findings; final re-review remains NEEDS REVISION
  on non-echoing refusal of control-containing managed paths.
- S4b rev-4 closes the final path-safety finding; focused re-review APPROVED.
- First tracked-state `go test -p=1 -count=1 ./...` failed only AVP-134's stale
  exact importer set; correction active.
- Corrected staged-state `go test -p=1 -count=1 ./...`, full
  `go test -race -p=1 ./internal/cli`, exact 51-fixture/provenance guards,
  gofmt, vet, host build and Linux/Darwin/Windows amd64 cross-builds — PASS.
- S4b blocking CI
  [32291924127](https://github.com/tesseracode/tesserapatch/actions/runs/32291924127)
  — PASS on Ubuntu, macOS and Windows; release job correctly skipped.
- S5 rev-0 targeted doctor/S5/golden/AVP/platform checks, vet, build and
  cross-builds — PASS before review; verdict NEEDS REVISION on four
  path-safety/error-taxonomy/aggregation/ledger findings.
- S5 rev-1 targeted doctor/S5/51-golden/compatibility tests, vet and Linux
  amd64/arm64, Darwin arm64 and Windows amd64 builds — PASS; focused re-review
  verdict NEEDS REVISION on five pending/ledger/docs/Windows residuals.
- S5 rev-2 closes those five findings; re-review remains NEEDS REVISION on
  three AST/matrix/authoritative-surface coverage residuals.
- S5 rev-3 targeted S5/doctor/AVP/51-golden tests, vet, cross-build guards and
  Windows test-binary/PIB-222 symbol verification — PASS; focused re-review
  verdict NEEDS REVISION on three ledger/runtime-observable residuals.
- S5 rev-4 closes those runtime residuals; re-review remains NEEDS REVISION on
  two alias/range-body and copied-heading resolver gaps.
- S5 rev-5 closes those two; re-review remains NEEDS REVISION on direct
  range-name mutation and strict Markdown fence/indent parsing.
- S5 rev-6 closes the final parser findings; focused re-review APPROVED.
- First staged-state full suite failed only AVP-134's stale importer set and
  PIB-146's inherited provider config; bounded test corrections active.
- Corrected `go test -p=1 -count=1 ./...`, full
  `go test -race -p=1 ./internal/cli ./internal/workflow`, gofmt, vet, host
  build and Linux amd64/arm64, Darwin amd64 and Windows amd64 cross-builds —
  PASS.
- S5 blocking CI
  [32304087548](https://github.com/tesseracode/tesserapatch/actions/runs/32304087548)
  — PASS on Ubuntu, macOS and Windows; release job correctly skipped.
- S6 rev-0 assets/docs/targeted/golden/vet checks — PASS before review; verdict
  NEEDS REVISION on systemic row attribution/guard sensitivity plus bounded
  prose/changelog findings.
- S7 AM–AO rev-4 exact ledger/resolver and focused row tests — PASS.
- `go test ./internal/cli -run '^TestS7' -count=1 -timeout 5m` — PASS
  (50.867s); complete affected `internal/cli` package — PASS (294.910s).
- Complete affected `internal/gitutil`, `internal/intentlock`,
  `internal/store` and `internal/workflow` packages — PASS.
- FreeBSD/OpenBSD/NetBSD/DragonFly amd64 CLI test binaries compile and are not
  executed; workflow YAML parse, affected-package vet, host build, gofmt and
  whitespace checks — PASS.
- S7 AM–AO rev-5 focused resolver, PIB-419/420/446, PIB-427 and PIB-439
  regressions — PASS.
- Complete S7 selectors — CLI 51.281s, workflow 0.458s, gitutil 0.228s,
  intentlock 0.306s, intentpub 0.299s and store 0.395s.
- Complete affected packages — CLI 289.485s, workflow 89.775s, gitutil
  6.832s, intentlock 0.384s, intentpub 8.912s and store 2.657s.
- Affected-package vet, host build, gofmt and `git diff --check` — PASS.
  No full repository suite, race, CI or AP work was run.
- Staged-source close at `fdf86cb`: serial uncached tests for `internal/cli`,
  `gitutil`, `intentlock`, `intentpub`, `s7marker`, `store` and `workflow`;
  affected vet; host build; gofmt and diff checks — PASS.
- S7 AM–AO rev-6 focused resolver/ledger, typed privacy and indexed Git scrub
  tests — PASS. All PIB-401/402/404/424/445/448 affected leaves and both new
  PIB-446 temp leaves pass under exact individual `go test -run` selection.
- Complete S7 selectors — CLI 51.855s, workflow 1.263s, gitutil 0.292s,
  store 0.342s, intentlock 0.267s and intentpub 0.267s.
- Complete affected packages — CLI 268.741s, workflow 85.487s, gitutil
  6.680s and store 2.594s. Affected vet, host build, gofmt and whitespace
  checks pass. No full repository suite, race, CI, commit or AP work was run.
- S7 AM–AO rev-7 observed registration authority — PASS: 54 row targets plus
  all twelve PIB-443 leaves in one subprocess (48.718s). All four wrong
  registration packages and six new PIB-446 identity sensitivities bite.
- Complete S7 selectors — CLI 106.354s, workflow 1.600s, store 0.349s,
  gitutil 0.236s, intentlock 0.252s and intentpub 0.253s.
- Complete affected packages — CLI 327.279s, workflow 89.490s and store
  2.588s. Affected vet, host build, gofmt and whitespace checks pass. No full
  repository suite, race, CI, commit or AP work was run.
- S7 AM–AO rev-8 out-of-band marker authority — PASS: 54 row targets plus all
  twelve PIB-443 leaves in 57.838s. Framed forgery and bounded timeout/reap
  sensitivities pass in 4.007s; both new PIB-446 init/interface fixtures bite.
- Complete S7 selectors — CLI 115.227s, workflow 1.520s, store 0.344s,
  gitutil 0.246s, intentlock 0.268s and intentpub 0.256s.
- Complete affected packages — CLI 336.064s and workflow 89.176s; marker
  helper compile, affected vet, host build, gofmt and whitespace checks pass.
  CLI test binaries cross-compile for Windows, FreeBSD, OpenBSD, NetBSD and
  DragonFly. No full repository suite, race, CI, commit or AP work was run.
- S7 AM–AO rev-9 random-correlation authority — PASS: all 54 row targets plus
  all twelve PIB-443 leaves in 57.463s. Framed/old-marker forgery, timeout/reap,
  target-association and token mutations pass in 4.749s; the exact ledger
  passes in 4.553s.
- Every unified PIB-446 sink sensitivity bites, including init-assigned legacy
  aliases, local imported-interface `os.WriteFile`, generic `io.Writer`,
  legacy retry/Store, artifact and prior CreateTemp/file-method cases.
- Complete S7 selectors — CLI 116.067s, workflow 1.832s, intentlock 0.422s,
  store 0.354s, gitutil 0.268s and intentpub 0.257s.
- Complete affected packages — CLI 339.853s and workflow 86.256s; marker
  helper compile, affected-package vet, host build, gofmt and whitespace checks
  pass. CLI test binaries cross-compile for Windows, FreeBSD, OpenBSD, NetBSD
  and DragonFly. No full repository suite, race, CI, commit or AP work was run.
- S7 AM–AO rev-10 module-aware/fail-closed PIB-446 sensitivities — PASS
  (1.087s): the real local `provider.Provider` implementation, unresolved
  module import and independent type error all fail through the same validator;
  every non-fail-closed sink mutation is required to type-check first.
- Random external correlation workspace sensitivities — PASS (4.769s):
  outside-repository and 0700 assertions, framed/old-marker evidence,
  timeout/reap, wrong association and wrong token all bite with no repository
  scratch. All 54 row targets plus twelve PIB-443 leaves pass in 58.252s; the
  exact ledger passes.
- Complete S7 selectors — CLI 109.685s, workflow 1.137s, intentlock 0.282s,
  store 0.341s, gitutil 0.279s and intentpub 0.246s.
- Complete affected packages — CLI 351.392s and workflow 96.515s; marker
  helper compile, affected-package/testdata vet, host build, gofmt and
  whitespace checks pass. CLI test binaries cross-compile for Windows,
  FreeBSD, OpenBSD, NetBSD and DragonFly. No full repository suite, race, CI,
  commit or AP work was run.
- S7 AP exact ledger — PASS: 34 PRD-derived rows (`PIB-449…482`),
  I9/C12/G9/U2/S2, all targets nonempty/body-sensitive. Independent observed
  execution of all 34 targets passed in 93.909s.
- Complete AP selectors — CLI 167.977s, gitutil 0.541s, workflow 0.344s,
  intentlock 0.273s, store 0.276s and intentpub 0.312s. PIB-461 derives all
  23 evaluated refusal classes from §6.4, executes 22 native class pairs and
  binds the platform class to its literal Windows runtime leaf. All nine G
  same-validator sensitivities bite.
- Existing dry-run/help regressions, affected-package vet, host build, gofmt
  and whitespace checks — PASS. CLI and intentlock test binaries cross-compile
  for Windows, FreeBSD, OpenBSD, NetBSD and DragonFly; all six AP package test
  binaries cross-compile for Linux amd64. No full repository suite, race, CI,
  commit, push or AQ–AX work was run.
- S7 AP rev-1 complete AP/observer selectors — PASS in CLI 167.393s; all 34
  remapped row targets remain exact, nonempty and independently correlated,
  and all nine G same-validator mutations bite.
- Complete affected packages — CLI 476.664s, workflow 86.158s, rescap 17.583s,
  intentpub 8.888s, gitutil 6.602s, store 2.830s and intentlock 0.348s.
  Targeted AVP-175 workflow grammar, prior seam-registry/non-invalidation
  guards, affected-package vet, host build, gofmt and whitespace checks pass.
  CLI tests cross-compile for Windows and FreeBSD/OpenBSD/NetBSD/DragonFly;
  Linux intentlock tests cross-compile. No full repository suite, race, CI,
  commit, push or AQ–AX work was run.
- S7 AP rev-2 closes all seven rev-1 findings. Focused PIB-450/459/460/467/476/
  478/479/482 and their same-validator sensitivities pass; the final exact
  PIB-459/460 rerun passes in 1.022s.
- Final AP34 ledger plus independent observed-correlation run — PASS in
  89.283s. The complete AP/observer selector run passes in 177.839s, including
  all nine G rows and their wrong-input sensitivities.
- Complete affected packages — CLI 481.755s and intentlock 0.483s. Affected
  package vet, host build, changed-file gofmt and whitespace checks pass. CLI
  tests cross-compile for Windows and FreeBSD/OpenBSD/NetBSD/DragonFly; Linux
  intentlock tests cross-compile. No full repository suite, race, CI, commit,
  push or AQ–AX work was run.
- S7 AP rev-3 focused PIB-459/468/476/478/479/482 leaves and sensitivities —
  PASS. PIB-468 now exercises the public command at the exact post-pending-CAS,
  pre-removal seam and preserves every selected blob plus the strict pending
  index.
- Complete AP plus independent observer — PASS: CLI 168.099s, gitutil 0.501s,
  intentlock 0.383s, intentpub 0.304s, rescap 0.457s, store 0.259s and
  workflow 0.377s. The separate all-AP observed-correlation run passed in
  94.005s and the AP34 ledger passed in 1.650s.
- Complete affected packages — CLI 484.658s, store 3.060s and intentlock
  0.461s. Affected-package vet, host build, changed-file gofmt and whitespace
  checks pass. The exact PIB-478/479 leaves also execute successfully in a
  native Linux container; CLI tests cross-compile for Windows and
  FreeBSD/OpenBSD/NetBSD/DragonFly. No full repository suite, race, CI, commit,
  push or AQ–AX work was run.
- S7 AP rev-4 focused AP CLI targets — PASS in 81.325s. Store, workflow,
  intentlock, intentpub, gitutil and rescap AP targets pass; the AP34
  ledger/independent observer passes in 91.633s with the new PIB-476 runtime
  panic-redaction leaf.
- Complete affected packages — CLI 460.849s, store 3.193s, workflow 98.688s,
  intentlock 0.489s, intentpub 9.137s, gitutil 11.488s and rescap 16.967s.
  Affected-package vet, host build, changed-file gofmt and whitespace checks
  pass. The exact PIB-478/479 leaves pass natively in a Linux arm64 container;
  CLI tests cross-compile for Windows and FreeBSD/OpenBSD/NetBSD/DragonFly.
  No full repository suite, race, CI, commit, push or AQ–AX work was run.
- S7 AP rev-5 focused PIB-471/476/477/478/479/482 and every new
  same-validator sensitivity — PASS. All AP CLI targets pass in 85.722s;
  store, workflow, intentlock, intentpub, gitutil and rescap AP selectors pass.
  The AP34 ledger final rerun passes in 1.736s and the independent observed-correlation
  final rerun passes in 87.969s.
- Complete affected packages — CLI 486.604s, workflow 97.560s, store 3.356s,
  intentlock 0.497s, intentpub 9.107s, gitutil 7.141s and rescap 15.740s.
  Affected-package vet, host build, changed-file gofmt and whitespace checks
  pass. PIB-478/479 pass natively in a Linux arm64 Go 1.26 container; CLI test
  binaries cross-compile for Windows and FreeBSD/OpenBSD/NetBSD/DragonFly.
  No full repository suite, race, CI, commit, push or AQ–AX work was run.
- S7 AP rev-6 focused PIB-476/477/478/479 and all new helper/map/buffer/CFG
  same-validator sensitivities — PASS. AP34 ledger — PASS (1.889s);
  independent observed-correlation — PASS (98.753s). Complete AP selectors —
  CLI 92.976s, gitutil 0.612s, intentlock 0.481s, intentpub 0.267s, rescap
  0.455s, store 0.259s and workflow 1.119s. Affected-package vet, host build,
  gofmt and whitespace checks pass. PIB-478/479 pass natively in Linux arm64
  Go 1.26; CLI test binaries cross-compile for Windows and
  FreeBSD/OpenBSD/NetBSD. No full repository suite, race, CI, commit, push or
  AQ–AX work was run.
- S7 AP rev-7 focused PIB-476/477/478/479 and every new
  concatenation/field-alias/classifier-input/error-guard sensitivity — PASS.
  AP34 ledger — PASS (1.832s); independent observed-correlation — PASS
  (93.566s). Complete AP selectors — CLI 90.668s, gitutil 0.523s, intentlock
  0.460s, intentpub 0.275s, rescap 0.488s, store 0.251s and workflow 1.076s.
  Affected-package vet, host build, gofmt and whitespace checks pass.
  PIB-478/479 pass natively in Linux arm64 Go 1.26; CLI test binaries
  cross-compile for Windows and FreeBSD/OpenBSD/NetBSD. No full repository
  suite, race, CI, commit, push or AQ–AX work was run.
- S7 AP rev-8 focused PIB-476/477/478/479 and every new named-enum,
  interprocedural-map, exact-Fstatfs-result and canonical-error-provenance
  sensitivity — PASS (5.882s, 3.841s and 0.621s respectively). Final AP34
  ledger — PASS (1.748s); independent observed-correlation — PASS (90.239s).
  Complete AP selectors — CLI 96.868s, gitutil 0.530s, intentlock 0.506s,
  intentpub 0.327s, rescap 0.447s, store 0.260s and workflow 1.165s.
  Complete affected packages — CLI 510.060s and intentlock 0.557s. Affected
  vet, host build, gofmt and whitespace checks pass. PIB-478/479 pass natively
  in Linux arm64 Go 1.26; CLI and intentlock test binaries cross-compile for
  Windows and FreeBSD/OpenBSD/NetBSD/DragonFly. No full repository suite,
  race, CI, commit, push or AQ–AX work was run.
- S7 AP rev-9 focused PIB-476/477/478/479 and every new enum-selector,
  unresolved-map-mutator and error-address/alias sensitivity — PASS (6.469s,
  3.688s and 0.521s respectively). Final AP34 ledger — PASS (1.782s);
  independent observed-correlation — PASS (90.892s). Complete AP selectors —
  CLI 91.752s, gitutil 0.529s, intentlock 0.516s, intentpub 0.302s, rescap
  0.475s, store 0.264s and workflow 1.085s. Affected-package vet, host build,
  gofmt and whitespace checks pass. PIB-478/479 pass natively in Linux arm64
  Go 1.26 (2.433s); CLI and intentlock test binaries cross-compile for Windows
  and FreeBSD/OpenBSD/NetBSD/DragonFly. No full suite, race, CI, commit, push
  or AQ–AX work was run.
- S7 AP rev-10 focused PIB-477/478/479 and every new nested-index/range and
  one-to-one syscall-guard sensitivity — PASS (4.366s and 0.528s
  respectively). Final AP34 ledger — PASS (1.825s); independent
  observed-correlation — PASS (98.300s). Affected AP selectors — CLI 100.122s
  and intentlock 0.575s. Affected-package vet, host build, gofmt and
  whitespace checks pass. PIB-478/479 pass natively in Linux arm64 Go 1.26
  (2.370s); CLI and intentlock test binaries cross-compile for Windows and
  FreeBSD/OpenBSD/NetBSD/DragonFly. No full suite, race, CI, commit, push or
  AQ–AX work was run.
- S7 AP rev-11 focused PIB-478/479 sampled-stat provenance and every new
  reassignment/initializer/pointer/helper/closure/field sensitivity — PASS
  (0.602s). Final AP34 ledger — PASS (1.900s); independent
  observed-correlation — PASS (92.934s); affected intentlock AP selectors —
  PASS (0.552s). Affected-package vet, host build, gofmt and whitespace checks
  pass. PIB-478/479 pass natively in Linux arm64 Go 1.26 (2.448s); CLI and
  intentlock test binaries cross-compile for Windows and
  FreeBSD/OpenBSD/NetBSD/DragonFly. No full suite, race, CI, commit, push or
  AQ–AX work was run.
- S7 AP rev-12 focused PIB-478/479 complete sampled-stat write-context
  collection and all new range/declaration/compound/select sensitivities —
  PASS (0.780s). Final AP34 ledger — PASS (1.727s); independent
  observed-correlation — PASS (88.562s); affected intentlock AP selectors —
  PASS (0.650s). Affected-package vet, host build, gofmt and whitespace checks
  pass. PIB-478/479 pass natively in Linux arm64 Go 1.26 (2.364s); CLI and
  intentlock test binaries cross-compile for Windows and
  FreeBSD/OpenBSD/NetBSD/DragonFly. No full suite, race, CI, commit, push or
  AQ–AX work was run.
- Staged-source AP close at `bf9424f`: serial uncached full tests for CLI,
  gitutil, intentlock, intentpub, rescap, store and workflow; affected vet;
  host build; gofmt and diff checks — PASS.
- AP post-acceptance CI correction: PIB-463 host-seam semantics, AP34 ledger,
  renamed Windows workflow blocking guard and its sensitivities — PASS
  (1.981s focused; 42.293s final affected selectors). Observer wrong-input and
  explicit hanging-process cleanup fixtures — PASS (4.504s). AP34 independent
  observed correlation with the four-minute hosted inner budget — PASS
  (88.196s final). Windows amd64 CLI test binary cross-compile, `go vet
  ./internal/cli`, host `go build ./cmd/tpatch`, gofmt and diff checks — PASS.
  Native Windows execution remains for blocking CI. No full suite, race, CI,
  commit, push or AQ–AX work was run.
- S7 AQ focused tests — PASS: CLI 20.960s and intentlock 0.340s. AQ23 exact
  ledger resolution is included; independent observed correlation — PASS
  (23.655s). PIB-498's reused partial/dangling/corrupt/tombstone archive
  targets and the affected AP PIB-476 privacy guard pass. `go vet
  ./internal/cli ./internal/intentlock`, host `go build ./cmd/tpatch`,
  changed-file gofmt and diff checks pass. CLI, intentlock and the command
  cross-compile for Linux amd64, Windows amd64 and FreeBSD amd64. No full
  suite, race, CI, commit, push or AR–AX work was run.
- S7 AQ rev-1 focused CLI tests — PASS (22.317s), including AQ23 I13/C3/G7
  ledger resolution and all seven biting G-row sensitivities. Independent AQ
  observed correlation — PASS (24.949s). AQ intentlock guard — PASS (0.438s);
  affected AP PIB-476 recursive report proof — PASS (6.048s).
  `go vet ./internal/cli ./internal/intentlock`, host
  `go build ./cmd/tpatch`, changed-file gofmt and `git diff --check` pass.
  CLI, intentlock and `cmd/tpatch` cross-compile for Linux amd64, Windows
  amd64 and FreeBSD amd64. No full suite, race, CI, commit, push or AR–AX work
  was run.
- S7 AQ rev-2 focused CLI tests — PASS (45.649s), including the unchanged
  AQ23 I13/C3/G7 ledger and every G-row sensitivity. Independent AQ observed
  correlation — PASS (56.882s). AQ intentlock — PASS (1.015s); affected AP
  PIB-476 — PASS (11.600s). `go vet ./internal/cli ./internal/intentlock`,
  host `go build ./cmd/tpatch`, changed-file gofmt, diff and cleanup checks
  pass. CLI and intentlock test binaries cross-compile for Linux, Windows,
  FreeBSD, OpenBSD, NetBSD and DragonFly amd64; `cmd/tpatch` cross-builds for
  Linux, Windows and FreeBSD amd64. PIB-504 additionally type-checks every
  GOOS partition derived from the installed Go target matrix. No full suite,
  race, CI, commit, push or AR–AX work was run.
- S7 AQ rev-3 focused CLI suite — PASS (53.015s), including the unchanged AQ23
  I13/C3/G7 ledger; exact ledger re-audit — PASS (1.994s). Independent AQ
  observed correlation — PASS (58.151s). AQ intentlock — PASS (1.100s);
  affected AP PIB-476 — PASS (6.187s). `go vet
  ./internal/cli ./internal/intentlock`, host `go build ./cmd/tpatch`,
  changed-file gofmt, diff and cleanup checks pass. CLI and intentlock test
  binaries cross-compile for Linux, Windows, FreeBSD, OpenBSD, NetBSD and
  DragonFly amd64; `cmd/tpatch` cross-builds for Linux, Windows and FreeBSD
  amd64. PIB-504 type-checks every GOOS partition mechanically derived from
  the installed Go target matrix. No full suite, race, CI, commit, push or
  AR–AX work was run.
- S7 AQ rev-4 focused CLI suite — PASS (52.672s), including the unchanged AQ23
  I13/C3/G7 ledger and both new biting guard families. Independent AQ observed
  correlation — PASS (52.809s); affected AP PIB-476 — PASS (6.022s). `go vet
  ./internal/cli`, host `go build ./cmd/tpatch`, changed-file gofmt, diff and
  cleanup checks pass. CLI test binaries cross-compile for Linux, Windows,
  FreeBSD, OpenBSD, NetBSD and DragonFly amd64; `cmd/tpatch` cross-builds for
  Linux, Windows and FreeBSD amd64. No full suite, race, CI, commit, push or
  AR–AX work was run.
- S7 AQ rev-5 focused CLI suite — PASS (53.266s), including the unchanged AQ23
  I13/C3/G7 ledger and the receiver-flow sensitivities. Independent AQ observed
  correlation — PASS (56.691s); affected AP PIB-476 — PASS (6.163s). `go vet
  ./internal/cli`, host `go build ./cmd/tpatch`, changed-file gofmt, diff and
  cleanup checks pass. CLI test binaries cross-compile for Linux, Windows,
  FreeBSD, OpenBSD, NetBSD and DragonFly amd64; `cmd/tpatch` cross-builds for
  Linux, Windows and FreeBSD amd64. The renderer graph uses an offline
  `go list -deps -mod=readonly` manifest with `GOPROXY=off` and `GOSUMDB=off`.
  No full suite, race, CI, commit, push or AR–AX work was run.
- Bounded AQ staged-source correction — PASS. Exact AP PIB-459 and AQ PIB-485
  pass (0.725s and 1.528s); the complete AQ recovery suite passes (15.395s);
  independent AQ observed correlation passes (55.896s). Serial uncached
  package validation passes for `internal/cli` (605.805s with the required
  20-minute package timeout) and `internal/intentlock` (1.447s). `go vet
  ./internal/cli ./internal/intentlock`, host `go build ./cmd/tpatch`,
  changed-file gofmt and `git diff --check` pass. No race, CI, commit, push or
  AR–AX work was run.
- Bounded AQ CI timeout correction — PASS. Focused AVP-175 guard and
  cross-platform source half pass (0.433s and 0.238s); complete
  `TestAVPGuards` plus `TestAVPWindowsSourceGuards` pass (5.599s); all S7
  workflow guards pass (0.382s). The workflow parses as YAML. `go vet
  ./internal/intent ./internal/cli`, host `go build ./cmd/tpatch`,
  changed-file gofmt and `git diff --check` pass. No full suite, race, CI,
  commit, push or AR–AX work was run.
- Bounded AQ frozen-source correction — PASS. The complete
  `TestPreparePIBPreChangeGoldens` parent, including
  `PIB-391-routing-provenance-and-immutability`, passes (7.135s). Restored
  `TestAVPGuards` plus `TestAVPWindowsSourceGuards` pass against the 40m
  workflow (5.277s); the new structural S7 timeout guard and all sensitivities
  pass (0.347s). Workflow YAML parsing, `go vet ./internal/intent
  ./internal/cli`, host `go build ./cmd/tpatch`, changed-file gofmt and
  `git diff --check` pass. Frozen source hash is
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`,
  and `git diff cdcd665 -- internal/intent/avp_guards_test.go` is empty. No
  full suite, race, CI, commit, push or AR–AX work was run.
- Bounded frozen-source S7 guard revision — PASS. The argv-aware timeout guard
  and all same-validator sensitivities pass (0.355s). The complete
  `TestPreparePIBPreChangeGoldens` parent, including PIB-391, passes (7.225s);
  restored `TestAVPGuards` plus `TestAVPWindowsSourceGuards` pass (5.347s).
  Workflow YAML parsing, `go vet ./internal/intent ./internal/cli`, host
  `go build ./cmd/tpatch`, changed-file gofmt and `git diff --check` pass.
  Frozen source remains SHA-256
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`
  with an empty diff against `cdcd665`. No full suite, race, CI, commit, push
  or AR–AX work was run.
- Bounded nested-shell S7 guard revision — PASS. The recursive argv/shell
  timeout guard and direct, env-prefixed, nested, nested-nested and dynamic
  payload sensitivities pass (0.508s). The complete
  `TestPreparePIBPreChangeGoldens` parent, including PIB-391, passes (7.127s);
  restored `TestAVPGuards` plus `TestAVPWindowsSourceGuards` pass (5.141s).
  Workflow YAML parsing, `go vet ./internal/intent ./internal/cli`, host
  `go build ./cmd/tpatch`, changed-file gofmt and `git diff --check` pass.
  Frozen source remains SHA-256
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`
  with an empty diff against `cdcd665`. No full suite, race, CI, commit, push
  or AR–AX work was run.
- Bounded dynamic-executable S7 guard revision — PASS. Variable, parameter,
  command-substitution, backtick and env-prefixed dynamic executable
  sensitivities pass with the same validator (0.370s). The complete
  `TestPreparePIBPreChangeGoldens` parent, including PIB-391, passes (7.030s);
  restored `TestAVPGuards` plus `TestAVPWindowsSourceGuards` pass (5.141s).
  Workflow YAML parsing, `go vet ./internal/intent ./internal/cli`, host
  `go build ./cmd/tpatch`, changed-file gofmt and `git diff --check` pass.
  Frozen source remains SHA-256
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`
  with an empty diff against `cdcd665`. No full suite, race, CI, commit, push
  or AR–AX work was run.
- Bounded AQ observer-budget correction — PASS. AQ observed registration
  authority passes locally (53.218s). The complete wrong-input suite,
  including separate AP/AQ tuple assertions, category/global-replacement
  sensitivities and the unchanged three-second hanging-process reap fixture,
  passes (4.920s); the AP budget leaf passes independently (0.354s). `go vet
  ./internal/cli`, host `go build ./cmd/tpatch`, changed-file gofmt and
  `git diff --check` pass. Workflow diff is empty; frozen AVP source remains
  SHA-256
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`
  with an empty diff against `cdcd665`. No full suite, race, CI, commit, push
  or AR–AX work was run.
- Bounded observer-budget callsite-binding revision — PASS. Both AP/AQ budget
  leaves pass (0.342s); the complete wrong-input/budget/reap suite passes
  (4.807s), including cloned-table and cross-category binding sensitivities.
  AQ observed registration authority passes (54.152s). `go vet
  ./internal/cli`, host `go build ./cmd/tpatch`, changed-file gofmt and
  `git diff --check` pass. Workflow diff is empty; frozen AVP source remains
  SHA-256
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`
  with an empty diff against `cdcd665`. No full suite, race, CI, commit, push
  or AR–AX work was run.
- S7 AR implementation — PASS. Focused AR runtime, semantic guard, exact
  ledger and observed-registration tests pass serially (7.861s). The affected
  AM–AQ ledgers, AP/AQ observers, observer wrong-input suite, AQ
  abandon/pending-purge regressions and existing real partial/divergence
  archive tests pass serially (159.141s). `go vet ./internal/cli
  ./internal/store`, host `go build ./cmd/tpatch`, Linux amd64 and Darwin amd64
  CLI test cross-compiles, changed-file gofmt and `git diff --check` pass. No
  full suite, race, CI, commit or push was run.
- S7 AR rev-0 independent review — NEEDS REVISION. Focused AR tests
  (3.429s), observed registration (5.328s), scoped `gofmt -d` and
  `git diff --check` pass, but the reviewer demonstrated false-pass paths in
  PIB-506…520 runtime, semantic guard, grammar and observer-budget evidence.
  AR is not safe to stage.
- S7 AR rev-1 implementation — PASS, ready for independent review. Serial
  focused runtime/guard/ledger tests (`go test -count=1 -p=1 -timeout=12m
  ./internal/cli -run '^TestS7AR' -v`) pass in 49.045s. The AR observer and
  complete wrong-input/budget/reap suite pass in 57.047s (observer 51.74s;
  wrong inputs 4.85s). Existing real partial/divergence archive plus AP/AQ
  purge regressions pass in 0.892s; `internal/intentlock` passes in 1.287s.
  `go vet ./internal/cli ./internal/intentlock` passes in 0.19s, host
  `go build ./cmd/tpatch` in 0.11s, and Linux amd64 / Darwin amd64 CLI test
  cross-compiles in 0.50s / 0.69s. Scoped gofmt and `git diff --check` pass;
  no staged files or scratch binaries remain. Frozen AVP source is still
  SHA-256 `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`
  with an empty diff against `cdcd665`; workflow diff is empty. No full suite,
  race, CI, commit, push or AS–AX work was run.
- S7 AR rev-1 independent review — NEEDS REVISION. Focused AR tests pass in
  52.56s, observed registration in 56.39s and wrong-input coverage in 6.15s.
  Scoped formatting/diff checks, frozen AVP hash and empty workflow diff pass;
  authorized production rendering matches the PRD. Seven runtime/guard
  evidence paths remain false-passable, so AR is not safe to stage. Review was
  Darwin-only; the device-node population uses the declared injected seam.
- S7 AR rev-2 implementation — PASS, ready for independent review. The complete
  serial `^TestS7AR` runtime/guard/ledger suite passes in 109.080s; the
  category-bound AR observer plus wrong-input/budget/reap suite passes in
  117.571s. Narrow real partial/divergence and AP/AQ purge/abandon regressions
  pass in 3.032s; focused store purge tests pass in 0.233s and
  `internal/intentlock` in 1.662s. `go vet -p=1 ./internal/cli`, host build,
  Linux amd64 and Darwin amd64 CLI test cross-compiles, scoped gofmt and
  `git diff --check` pass. Frozen AVP source remains SHA-256
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`;
  its baseline diff and the workflow diff are empty. No staged files or
  scratch binaries remain. No full suite, race, CI, commit, push or AS–AX work
  was run.
- S7 AR rev-2 independent review — NEEDS REVISION. Focused AR tests pass in
  103.97s, observer/wrong-input coverage in 112.23s and the focused
  partial-branch regression in 1.37s. The exact 15-row `I6/C4/G4/S1` ledger,
  formatting, diff, frozen AVP and workflow checks pass; production rendering
  and observer budgets remain unchanged. PIB-511, PIB-518 and PIB-519 retain
  semantic false-pass paths, so AR is not safe to stage. Review was
  Darwin-only; device behavior used the declared seam.
- S7 AR rev-3 implementation — PASS, ready for independent review. The three
  corrected semantic guards pass serially in 244.223s and the complete serial
  `^TestS7AR` suite passes in 243.866s. Narrow real partial/divergence and AP/AQ
  purge/abandon regressions pass in 3.251s. Scoped gofmt, `go vet -p=1
  ./internal/cli` (0.65s), host build (0.37s), and Linux amd64 / Darwin amd64
  CLI test cross-compiles (3.67s / 3.54s) pass. The accepted production and
  runtime files retain baseline SHA-256
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8` and
  `77617b685b971da3f0a01bf199911608df4afdd8507cff756af03835ccc25569`.
  Frozen AVP remains
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`;
  workflow diff is empty. The guard-only revision cannot affect registration,
  so the unchanged AR observer/wrong-input suite was not rerun. No full suite,
  race, CI, staging, commit, push or AS–AX work was run.
- S7 AR rev-3 independent review — NEEDS REVISION. The three guards pass in
  229.671s; the ledger plus AR observer/wrong-input suite passes in 249.772s;
  the focused partial/divergence regression passes in 1.23s. Direct rev-3
  mutations, the exact 15-row `I6/C4/G4/S1` ledger, 12m/8m/1m budgets,
  formatting, diff, frozen AVP and workflow checks pass. PIB-511 still loses
  helper-return refusals and call-site order, PIB-518 misses cross-file
  pointer-helper writes, and PIB-519 misses folded/equivalent claims and
  unlisted negation. AR is not safe to stage.
- S7 AR rev-4 implementation — PASS, ready for independent review. The final
  serial PIB-511/518/519 guard trio passes in 350.601s and the complete serial
  `^TestS7AR` suite passes in 342.831s. The focused real partial/divergence and
  S4 abandon regressions pass in 0.900s. Scoped gofmt and diff checks pass;
  `go vet -p=1 ./internal/cli` passes in 0.91s, host build in 0.13s, and Linux
  amd64 / Darwin amd64 CLI test cross-compiles in 3.37s / 3.58s. The accepted
  production renderer, AR runtime, AR ledger and observer/registration files
  retain SHA-256
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `77617b685b971da3f0a01bf199911608df4afdd8507cff756af03835ccc25569`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`
  and
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`.
  Frozen AVP remains
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`;
  workflow diff is empty and HEAD/origin remain `c363ed7`. The guard-only
  revision cannot affect registration, so observer/wrong-input was not rerun.
  No full suite, race, CI, staging, commit, push or AS–AX work was run.
- S7 AR rev-4 independent review — NEEDS REVISION. The PIB-511/518/519 trio
  passes in 339.199s, but the remaining selected AR evidence fails because
  cold PIB-510 reaches the Git spy during `git ls-files`; isolated PIB-510
  reproduces exit 93 in 0.99s. The observer/budget suite passes in 366.778s.
  Exact ledger, budgets, formatting, diffs, frozen AVP, workflow and hashes
  pass. PIB-511 still accepts an unrefused helper alternative, PIB-518 misses
  promoted-field stores, and PIB-519 misses package/local constants and
  unrecoverability while accepting conditional routes. AR is not safe to
  stage.
- S7 AR rev-5 implementation — PASS, ready for independent review but not
  approved. The pre-fix cold PIB-510 reproduced exit 93 in 4.42s; after moving
  source-graph resolution ahead of the spy, fresh isolated runs pass in 26.22s
  and 22.95s. The corrected serial PIB-511, PIB-518 and PIB-519 guards pass in
  275.69s, 76.54s and 111.82s. An initial focused suite exposed repeated regexp
  compilation and hit its 10-minute timeout; hoisting immutable claim/route
  patterns removed that cost, and the final serial `^TestS7AR` suite passes in
  457.22s with more than two minutes of timeout margin. Narrow real
  partial/divergence and S4 abandon regressions pass in 2.19s. Scoped gofmt and
  diff checks pass; `go vet -p=1 ./internal/cli` passes in 0.62s, host build in
  0.12s, and Linux amd64 / Darwin amd64 CLI test cross-compiles in 3.49s /
  3.45s. Runtime and guard SHA-256 are
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`
  and `1e7524368bdd4040e47c4e9496dca16dedc6bf0963c5b0fe84eaea5f78c9bdf8`.
  Production, AR ledger and observer/registration retain
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`
  and
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`.
  Frozen AVP remains
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`;
  workflow/assets diffs are empty, HEAD/local/origin remain `c363ed7`, staging
  is empty and no scratch binaries remain. Observer/wrong-input was not rerun
  because registration did not change. No full suite, race, CI, staging,
  commit, push or AS–AX work was run.
- S7 AR rev-5 independent review — NEEDS REVISION. Cold isolated PIB-510
  passes in 24.742s; the focused AR suite passes in 459.850s and observer
  authority in 460.309s. The ledger remains exactly 15 rows
  (`I6/C4/G4/S1`), and formatting, scoped diffs, protected hashes and workflow
  checks pass. PIB-511 still loses refusal-code alternatives and local
  rebinding, PIB-518 accepts same-shaped decoy fields, and PIB-519 retains
  blacklist false passes, constant-alias duplication and an incomplete
  embedded-asset inventory. AR is not safe to stage.
- S7 AR rev-6 implementation — PASS, ready for independent review but not
  approved. Final serial PIB-511, PIB-518 and PIB-519 runs pass in 310.54s,
  88.99s and 131.78s. The complete focused `^TestS7AR` suite passed once in
  583.44s under the registered 12-minute outer budget; its final follow-up was
  the PIB-511-only pass after reusing the already-built typed source graph for
  remediation extraction, with no validator-semantic change. Narrow real
  partial/divergence and S4 abandon regressions pass in 1.68s. Changed-file
  gofmt and scoped `git diff --check` pass; `go vet -p=1 ./internal/cli`
  passes in 0.62s, host build in 0.13s, and Linux amd64 / Darwin amd64 CLI
  test cross-compiles in 3.51s / 3.36s. Guard SHA-256 is
  `38b0053707847163dfe074508c53a9921c80865a5f99945f98c1f0f74212d6da`.
  Frozen runtime, production, AR ledger, observer/registration and AVP retain
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`.
  Workflow/assets diffs are empty, HEAD/local/origin remain `c363ed7`,
  staging is empty and no scratch binaries remain. Registration did not
  change, so observer/wrong-input was not rerun. No full repository suite,
  race, CI, staging, commit, push or AS–AX work was run.
- S7 AR rev-6 independent review — NEEDS REVISION. PIB-511, PIB-518 and
  PIB-519 pass independently in 312.378s, 91.821s and 126.886s; ledger and
  wrong-input/budget guards pass in 6.514s, and PIB-508/520 regressions pass
  in 0.566s. The cold AR observer fails its mandatory 8-minute inner budget
  at 483.925s. Adversarial inspection also finds address-taken rebinding and
  nested-remediation gaps in PIB-511, invoked-closure and decoy-pointer-origin
  gaps in PIB-518, and suffix-context, exact-reference and positive
  recoverability gaps in PIB-519. Protected hashes, formatting, scoped vet
  and Linux/Darwin cross-compiles pass; HEAD/local/origin remain `c363ed7`,
  staging is empty, and AR is not safe to stage.
- S7 AR rev-7 implementation — PASS, ready for independent review but not
  approved. Before correction, the cold observer reproduced its timeout at
  483.71s test / 484.64s real; focused bite-proofs reproduced PIB-511 accepting
  indirect pointer rebinding in 97.80s, PIB-518 accepting an invoked closure
  store in 60.85s, and PIB-519 accepting a prohibited command suffix in
  60.15s. Final fresh-process PIB-511, PIB-518 and PIB-519 runs pass in
  146.065s / 149.03s real, 33.193s / 34.21s real and 148.153s / 150.99s real.
  The unchanged cold `TestS7ObservedARRegistrationAuthority` passes in
  339.933s / 340.34s real, over 140 seconds inside its fixed 480-second inner
  budget and below the 420-second target. Exact AR ledger, empty-target,
  wrong-input/budget, PIB-508/520, real PIB-506/507, partial/divergence and S4
  abandon regressions pass together in 6.966s. The observer executes every
  registered AR row target, and the separate ledger/meta run covers the
  remaining focused evidence, so a redundant complete `^TestS7AR` rerun was
  not performed. Changed-file gofmt and `git diff --check` pass;
  `go vet -p=1 ./internal/cli` passes in 0.70s, host build in 0.38s, and Linux
  amd64 / Darwin amd64 CLI test cross-compiles in 3.36s / 3.38s. Guard
  SHA-256 is
  `502d6aa1d7a0f2715bb0dd5ce03495aa7a92be2604497ff7cc53b718d4a2457c`.
  Frozen runtime, production, AR ledger, observer/registration and AVP retain
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`.
  Side Research remains `b385fe622db9926f48861105239f113e`;
  workflow/assets rev-7 diffs are empty, the exact ledger remains
  `I6/C4/G4/S1`, HEAD/local/origin remain `c363ed7`, staging is empty and no
  scratch artifacts remain. Remaining uncertainty is limited to independent
  rev-7 review. No full repository suite, race, CI, staging, commit, push or
  AS–AX work was run.
- S7 AR rev-7 independent review — NEEDS REVISION. The unchanged cold
  `TestS7ObservedARRegistrationAuthority` passes in 337.88s / 338.83s wall,
  about 141 seconds inside its fixed 480-second inner budget. Focused
  ledger/runtime/meta regressions pass in 17.354s; formatting, diff-check,
  scoped vet, host build and Linux/Darwin cross-compiles pass. Four material
  guard gaps remain: PIB-511 omits the evidence-present authority remediation
  population and loses deeper pointer/receiver/captured-callable flow;
  PIB-518 skips callable-parameter/field closure execution and loses deeper
  definite-decoy pointer origin; PIB-519 accepts semicolon-truncated or
  prefix-only invalid commands, negative route/reference prose and misses
  equivalent active recoverability claims. Guard and protected hashes match,
  workflow/assets diffs are empty, the ledger remains `I6/C4/G4/S1`,
  Side Research is stable, HEAD/local/origin remain `c363ed7`, and staging is
  empty. No full suite, race, CI or AS–AX review was run.
- S7 AR rev-8 implementation — PASS, ready for independent review but not
  approved and unsafe to stage. Test-first, same-validator probes reproduced
  all four rev-7 gaps before correction: the evidence-present authority
  remediation drift was accepted in 9.34s / 12.79s wall; a two-level
  refusal-code pointer mutation was accepted in 9.24s / 9.96s wall; a
  callable-parameter canonical `PurgeProgress` store was accepted in 9.06s /
  9.81s wall; and `--abandon-transaction=false` was accepted as the exact
  abandon route in 24.27s / 25.01s wall.

  PIB-511 now compares the complete typed `prepareAuthorityRefusal` body
  against the accepted ordered code set, evidence predicate, repo-relative
  lane, `rm -rf` route, untracked-state statement, destructive cost and
  canonical-preservation text. Real filesystem assertions cover empty and
  evidence-present rows 5/6/8 plus row 7's unconditional wait-only route.
  Address provenance recurses through arbitrary address/dereference,
  parentheses, conversions and aliases, and calls inspect arguments, method
  receivers/function values and invoked captured callable fields for both
  refusal-code and report objects. Definite unrelated pointer flow remains
  accepted.

  PIB-518 now computes invoked closures with a bounded monotone fixpoint over
  callable arguments, parameters, named helper forwarding, aliases and
  composite fields. It inventories forwarded canonical stores, preserves
  uninvoked callbacks, and recurses through arbitrary pointer depth while
  retaining canonical versus definite noncanonical field identity;
  conditional, reassigned or escaped relevant origins fail closed.

  PIB-519 tokenizes the complete command-bearing code span and accepts only the
  exact abandon argv, legal purge selector grammar with `--yes`, and exact
  manual forms. Route polarity covers the command's full qualifying suffix,
  including text after semicolons. References are exact-boundary and
  target-local, and controlled active/passive recoverability grammar includes
  `Operators can always recover the archive` without admitting negative,
  conditional, quoted, example-only, future or aspirational prose. The exact
  eight-claim shipped inventory and embed-derived `.mdc`/extensionless asset
  discovery remain intact. Claim-cache entries are immutable and keyed by
  exact source name and bytes; mutated Go still rebuilds typed state.

  Final fresh-process guards pass: PIB-511 172.84s / 174.50s wall, PIB-518
  45.87s / 46.87s wall, and PIB-519 120.45s / 124.55s wall. An initial
  post-correction observer run exposed excess repeated claim parsing and hit
  the fixed inner timeout at 483.95s / 484.90s wall; exact-key immutable
  per-source claim caching removed that duplication. The unchanged cold
  `TestS7ObservedARRegistrationAuthority` then passes in 360.32s / 361.29s
  wall, almost two minutes inside 480s and below the 420s target. Exact
  ledger/empty-target, wrong-input/budget, PIB-508/520, real PIB-506/507,
  partial/divergence and S4 abandon regressions pass in 7.478s / 7.91s wall.
  `go vet -p=1 ./internal/cli` passes in 0.68s, host build in 0.38s, and Linux
  amd64 / Darwin amd64 CLI test cross-compiles in 3.38s / 3.53s. Changed-file
  gofmt and diff checks pass.

  Guard SHA-256 is
  `4697a165ee4b24ee8f69f94d5af55df47211b07296c93904cf73a050c9a3a55d`.
  Frozen runtime, production, AR ledger, observer/registration and AVP retain
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`.
  Workflow/assets rev-8 diffs are empty, the exact ledger remains
  `I6/C4/G4/S1`, Side Research remains
  `b385fe622db9926f48861105239f113e`, HEAD/local/origin remain `c363ed7`,
  staging is empty and no scratch artifacts remain. Validation was
  Darwin-hosted and retains the declared device-node seam limitation. No
  complete `^TestS7AR`, full repository suite, race, CI, staging, commit, push
  or AS–AX work was performed.
- S7 AR rev-8 independent review — NEEDS REVISION. The unchanged cold observer
  passes in 395.35s test / 397.26s wall, about 85 seconds inside its fixed
  480-second inner budget and below the 420-second target. All 15 registered
  targets correlated. Focused guards and regressions, formatting, scoped vet,
  host build and Linux/Darwin cross-compiles pass; the exact ledger remains
  `I6/C4/G4/S1`; protected hashes, workflow/assets diff, Side Research hash,
  refs and empty staging state remain intact.

  Four same-authority gaps remain. PIB-511 accepts an in-memory polarity
  reversal in `prepareLaneHasPendingEvidence` because the validator binds
  `prepareAuthorityRefusal` but relies on the already-compiled helper for
  runtime assertions. PIB-518 accepts an invoked helper-returned callback
  containing an additional canonical `PurgeProgress` store. PIB-519 accepts
  `subject to` authority, a negative `not governed by D13` reference, an
  intent-lane traversal target and a child of a canonical blob target.
  Equivalent active claims using restore, always-possible recovery,
  impossible-to-strand and unblock vocabulary are not inventoried. Rev-9 may
  change only the AR guard and this handoff. No full suite, race, CI, staging,
  commit, push or AS–AX work was performed.
- S7 AR rev-9 bounded implementation — semantic corrections pass focused
  validation, but the mandatory observer target is BLOCKED and the revision is
  not ready for review or staging. Same-validator test-first probes reproduced
  each rev-8 gap before validator changes:
  - PIB-511 accepted a source mutation that changed
    `preparePendingEvidenceName(name)` to its negation, in 78.01s test /
    81.56s wall.
  - PIB-518 accepted a locally invoked factory-returned callback containing an
    additional canonical `PurgeProgress` store, in 28.16s / 32.36s wall.
  - PIB-519 accepted an abandon command qualified by `subject to approval`, in
    53.58s / 57.21s wall.
  - PIB-519 left `Operators can always restore the archive.` outside the exact
    eight-claim inventory, in 48.20s / 51.79s wall.

  PIB-511 now source-binds `prepareLaneHasPendingEvidence` itself. The
  canonical typed helper must call the canonical
  `preparePendingEvidenceName` object exactly once in the accepted
  entry-by-entry loop, return true only on the positive match and otherwise
  fall through to false; `prepareAuthorityRefusal` must call that exact helper.
  Same-validator fixtures reverse/remove/replace the predicate, reverse the
  return and add an unconditional early true, while the existing compiled
  empty/evidence-present rows 5/6/8 and row 7 wait-only assertions remain.

  PIB-518 now resolves function-valued results from invoked local literals and
  named helpers, recursively follows returned factory calls, carries results
  through declaration-later assignment and aliases, and computes one
  package-wide invoked-literal fixpoint for the per-function canonical-field
  inventory. Local, named, nested/forwarded and assignment-after-declaration
  factory stores fail; an uninvoked returned closure remains excluded. Prior
  parameter, field and arbitrary-pointer sensitivities remain.

  PIB-519 now validates code-block commands as their exact executable line and
  inline commands against their complete qualifying clause. Suffix authority
  is positive and closed rather than accepted by absence from a blacklist;
  `subject to`, `unless`, `provided that`, newline and second-clause conditions
  fail. Reference relations and suffixes are exact and target-local, so
  negative/conditional D13 prose cannot borrow authority. Manual targets are
  segment-normalized exact intent-lane or managed-blob forms: traversal,
  repeated separators, glob/shell syntax, siblings/children, missing `--` on a
  blob and extra tokens fail; the exact lane and blob placeholder forms pass.
  Controlled permanence grammar now inventories active/passive
  recover/restore/unblock, always-possible recovery and
  impossible-to-block/strand constructions, with negative, conditional,
  hypothetical, future, aspirational, historical, quoted/attributed and
  unrelated controls. The shipped inventory remains exactly eight and cache
  keys remain exact source-name/body hashes.

  Fresh focused passes:
  - PIB-511: 197.304s package / 200.50s wall.
  - PIB-518: 54.244s / 55.30s wall.
  - PIB-519: 138.414s / 141.70s wall.
  - Exact AR ledger/empty-target, observer wrong-input/budget, PIB-508/520,
    real PIB-506/507, partial/divergence and S4 abandon regressions: 7.361s /
    7.87s wall.

  The unchanged cold `TestS7ObservedARRegistrationAuthority` was then run once
  in a fresh process as required. It had not completed after the 430-second
  wait, already exceeding the required <=420-second target. The inner
  `go test -json` process was still active at approximately 7m53s elapsed and
  was terminated explicitly (PIDs 65434/65467) before the immutable 8-minute
  deadline; therefore there is no passing observer test/package time and the
  15 target markers were not accepted. Per the stop condition, scoped vet,
  host build and Linux/Darwin cross-compiles were not run after this miss, and
  no second observer attempt was made.

  Changed-file gofmt and diff checks pass. Guard SHA-256 is
  `856af3d8562a9e44822310c27e582e3d7e8ed599b0258d51198bf549f36264c3`.
  Frozen runtime, production, AR ledger, observer/registration and AVP retain
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`.
  The exact ledger remains 15 rows `I6/C4/G4/S1`; workflow/assets rev-9 diff is
  empty; Side Research remains `b385fe622db9926f48861105239f113e`;
  HEAD/local/origin remain `c363ed7`; staging is empty. Root `tpatch` and
  `cli.test` binaries predated this rev-9 run and were preserved under the
  explicit no-touch rule; rev-9 created no repository scratch file. No full
  suite, race, CI, commit, push or AS–AX work was performed.
- S7 AR rev-9 performance fold — PASS, ready for independent review but not
  approved and unsafe to stage. The fold removes repeated structural work
  without weakening the rev-0…rev-9 sensitivities:
  - PIB-511's top authoritative path still invokes the exact typed
    `s7ARValidateAuthorityRefusalOverride` primitive. Helper/predicate and
    authority-remediation mutations now build a minimal typed program
    containing the exact parsed production declarations for
    `prepareAuthorityRefusal`, `prepareLaneHasPendingEvidence` and
    `preparePendingEvidenceName`; typed object identity still binds the
    canonical positive predicate and helper call. Mutations requiring
    pre-abandon graph evidence continue to rebuild and validate the complete
    graph. Final PIB-511 timing is 180.63s test / 181.220s package / 185.45s
    wall, down from 197.304s package.
  - PIB-518 retains the complete package type model, invoked-callable fixpoint,
    factory-return alternatives, canonical/decoy pointer provenance and every
    prior full-path sensitivity. Final timing is 58.57s test / 59.099s package
    / 60.30s wall.
  - PIB-519 no longer type-checks the entire shipped production graph merely
    to derive constant strings. It parses every shipped Go source and builds a
    deterministic declaration graph for exact string literals,
    parenthesized/string-converted expressions, concatenation, local and
    package constants, cross-file aliases and declaration-root deduplication.
    Mutually exclusive build-file declarations remain separate candidates;
    exact source-name/body keys select immutable incremental states. The same
    complete claim/route validator consumes these sections. Route,
    reference, normalized manual-target and permanence fixtures—including
    all four rev-8 reviewer repros—still bite. Invalid → valid → invalid
    same-name/body mutations prove stale-cache rejection and order
    independence. The shipped inventory remains exactly eight. A first
    incremental-cache-only measurement remained bottlenecked at 142.392s
    package / 146.89s wall; removing the whole-production type check yields
    the final 16.08s test / 16.555s package / 17.07s wall.

  Fresh focused package total is 256.874s
  (`181.220 + 59.099 + 16.555`), below the non-negotiable 325-second gate.
  The unchanged cold `TestS7ObservedARRegistrationAuthority` was therefore
  run exactly once in a fresh process. It passes and correlates all 15
  registered targets in 253.53s test / 253.958s package / 254.46s wall,
  retaining the immutable 12-minute outer, 8-minute inner and 1-minute
  wrong-input budgets and finishing more than 165 seconds inside the
  420-second target.

  The exact AR ledger/meta/empty-target and observer wrong-input/budget guards,
  PIB-508/520, real PIB-506/507 divergence populations, partial archive
  controls and S4 abandon regressions pass together in 8.956s package / 10.11s
  wall. Changed-file gofmt and diff checks pass.
  `go vet -p=1 ./internal/cli` passes in 0.76s; host build in 0.44s; Linux
  amd64 and Darwin amd64 CLI test cross-compiles in 4.69s and 4.57s.

  Guard SHA-256 is
  `42ba27d06da62557172b319a8d3f0f0811bb3f0905969d7ba0f5ccd1fa3a4499`.
  Frozen runtime, production, AR ledger, observer/registration and AVP retain
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`.
  The exact ledger remains 15 rows `I6/C4/G4/S1`; workflow/assets rev-9 diff
  is empty; Side Research remains `b385fe622db9926f48861105239f113e`;
  HEAD/local/origin remain `c363ed7`; staging is empty. The pre-existing root
  `tpatch` and `cli.test` binaries retain their pre-task mtimes and were not
  touched; the only repository file newer than this authorization is the
  authorized guard before this handoff update. No Go process or
  rev-9-created scratch/cross-compile artifact remains. No complete
  `^TestS7AR`, full suite, race, CI, staging, commit, push or AS–AX work was
  performed.
- S7 AR rev-9 independent review — **NEEDS REVISION**. Fresh reviewer
  PIB-511/518/519 package times are 184.950s, 60.446s and 17.892s
  (263.288s total). The unchanged cold observer passes all 15 targets in
  264.985s package / 265.56s wall. Narrow regressions, formatting, diff
  checks, scoped vet, host build and Linux/Darwin amd64 cross-compiles pass;
  exact ledger, frozen hashes, refs, workflow/assets, Side Research, empty
  staging and cleanup remain intact. Review found five Medium gaps: the
  canonical pending-evidence predicate body is unvalidated; named-result
  callbacks are omitted; valid Go constant candidates can be lost; wildcard
  and conditional routes pass; and active `never strand` guarantees escape
  inventory.
- S7 AR rev-10 bounded correction — PASS, ready for independent review but
  not approved and unsafe to stage.
  - PIB-511's authoritative `s7ARValidateAuthorityRefusalOverride` path now
    extracts the exact unique production declarations for
    `preparePendingEvidenceName`, `prepareControlEvidenceName`,
    `prepareStageEvidenceName` and `validPrepareHex` into the small typed
    authority program. It requires the exact side-effect-free
    control-OR-stage return, one typed call to each canonical predicate with
    the real `name` parameter object, and the already-bound single
    `prepareLaneHasPendingEvidence` call. Same-validator fixtures bite the
    reviewer's control-only body, stage-only, conjunction, reversed polarity,
    wrong argument, shadowed/decoy predicate, discarded decoy call and
    duplicate call. Existing full-graph pre-abandon mutations remain on the
    complete graph path. Compiled behavior independently proves
    `journal.json`, `stage-0123456789ab`, unrelated false and row-7
    contention with stage evidence.
  - PIB-518 extends the existing package-wide callable/fixpoint primitive,
    not a side validator. Named function-valued result objects are emitted at
    bare returns; the shared assignment-origin graph resolves declaration-
    later assignment, parenthesized aliases, branch alternatives and nested
    named/local factories. Every invoked factory result is re-evaluated after
    fixpoint convergence and unresolved/cyclic origins fail closed. The exact
    named-result reviewer repro, later-assigned alias, two-branch alternatives
    and unresolved helper factory all bite the complete purge-progress
    inventory. An invoked factory whose returned callback is definitely not
    invoked remains excluded, as do all prior canonical/decoy pointer
    controls.
  - PIB-519 retains the rev-9 performance fold while upgrading the declaration
    graph to Go constant values. It handles quoted strings, rune/integer
    constants, `string('c')`, typed string constants, parentheses,
    concatenation, local/package constants and cross-file aliases. Identifier
    resolution propagates a deterministic set of every package/build
    candidate and computes/deduplicates the full Cartesian product without
    merging lexical objects or packages. Mutated package groups include every
    unchanged semantic input, and the all-source key still includes exact
    source names and bytes. Exact `string('c')`, typed/local-shadowed,
    two-prefix/two-suffix build cross-product, duplicate-candidate, cyclic and
    unresolved fixtures all exercise the complete production/asset inventory
    validator. Existing invalid → valid → invalid cache and order sensitivity
    remains passing.
  - Route validation now rejects noncanonical raw command bytes before
    splitting or target handling. It accepts only exact single-space argv,
    exact abandon/purge forms, the trailing-slash intent lane without `--`,
    or the exact managed blob with `rm -rf --`; raw glob/shell bytes,
    traversal, duplicate separators, missing/extra slash, siblings, children,
    extra tokens and extensions fail. Complete clauses reject conditional,
    prohibited, subject-to, unless, provided-that, second-clause and
    `if approved, then run` authority. Open-ended suffix recognition was
    replaced by the closed shipped suffix set. The exact wildcard and
    conditional reviewer repros plus wildcard/meta, suffix extension,
    punctuation/newline and valid exact controls bite the same route
    validator.
  - Permanence polarity now recognizes active/reordered/passive anti-stranding
    forms (`Operators can never strand the archive`, `Never can operators
    strand the archive`, `Operators will never leave the archive stranded`,
    `The archive can never be stranded`) as claims that require route
    authority. `never recover`, conditional, quoted, historical and unrelated
    subjects remain excluded.

  Fresh final focused results, each in its own serial `-p=1 -count=1`
  process: PIB-511 182.14s test / 182.688s package / 183.35s wall; PIB-518
  64.63s / 65.155s / 66.30s; PIB-519 21.46s / 21.960s / 22.44s. Package sum
  is 269.803s, below the required 325 seconds. The unchanged cold
  `TestS7ObservedARRegistrationAuthority` then ran exactly once in a fresh
  process and passed/correlated all 15 targets in 277.24s test / 277.670s
  package / 278.23s monotonic wall, retaining the immutable 12-minute outer,
  8-minute inner and 1-minute wrong-input budgets and finishing below 420s.

  The rev-9 narrow ledger/meta/empty-target, observer wrong-input/budget,
  PIB-508/520, real PIB-506/507, archive partial/divergence and S4 abandon
  regression set passes in 8.639s package / 9.92s wall. Changed-file gofmt
  and diff checks pass. `go vet -p=1 ./internal/cli` passes in 0.81s; host
  build in 0.46s; Linux amd64 and Darwin amd64 CLI test cross-compiles in
  4.93s and 4.85s.

  New guard SHA-256 is
  `096fc873744010d824579ebf7020ec7f0fbfad192aa01c35afad109e88eac6ff`.
  Frozen runtime, production renderer, AR ledger, observer/registration and
  AVP retain
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`.
  The exact ledger remains 15 rows `I6/C4/G4/S1`; Side Research remains
  `b385fe622db9926f48861105239f113e`; workflow/assets rev-10 diff is empty;
  HEAD/local/origin remain `c363ed7`; staging is empty. Pre-existing root
  `tpatch` and `cli.test` retain their pre-task mtimes and were not touched.
  Only the two authorized files are newer than rev-10 authorization; no Go
  process or rev-10 scratch/cross-compile artifact remains. No complete
  `^TestS7AR`, full suite, race, CI, staging, commit, push, tag or AS–AX work
  was performed.
- S7 AR rev-10 independent review — **NEEDS REVISION**. Static adversarial
  review found four Medium false-pass paths: method expressions misbind their
  explicit receiver as a callback argument; cross-file constant dependents
  retain stale folded claim-cache entries; blob/generation route values share
  a permissive grammar that admits wrong placeholders; and `No operator can
  strand the archive.` escapes the positive claim inventory. Per the
  early-stop rule, the reviewer did not run focused Go tests, the observer,
  regressions, vet/build or cross-compiles. Guard and frozen hashes, refs,
  exact 15-row `I6/C4/G4/S1` ledger, Side Research, workflow/assets, staging
  and process/scratch cleanup remain intact.
- S7 AR rev-11 bounded correction — PASS, ready for independent review but
  not approved and unsafe to stage.
  - Test-first reviewer repros failed through the shared complete validators
    before the fixes: method-expression callback omission at 12.75s test /
    13.262s package / 18.33s wall; unchanged dependent claim cache at 1.25s /
    1.585s / 2.24s; `--blob <slug>` route acceptance at 1.14s / 1.476s /
    1.98s; and omitted `No operator can strand the archive.` inventory at
    1.20s / 1.533s / 2.15s. The combined exact repro test passes after the
    corrections in 16.366s package / 20.67s wall.
  - PIB-518's package-wide invoked-callable fixpoint now derives the argument
    offset from `go/types.Info.Selections`: `types.MethodExpr` consumes the
    explicit receiver at argv zero, while `types.MethodVal` has an already
    bound receiver and ordinary functions retain argv zero. The reviewer's
    value-receiver repro and a pointer-receiver method expression now add the
    extra canonical store to the complete purge-progress inventory.
    Complete-validator controls also prove method values and ordinary
    functions remain inventoried. Nested-function ownership, invoked-only
    filtering, named-result/factory origins and unresolved fail-closed
    behavior remain unchanged.
  - PIB-519's incremental declaration resolver still rebuilds only affected
    package groups, but now regenerates sections and `sourceClaims` for every
    Go file in each affected package, not only the source whose bytes changed.
    The exact Linux/Darwin prefix plus unchanged shared-claim repro proves
    harmless → authoritative → harmless, then the Darwin-first order
    permutation, through one reused complete-inventory state. Both stale
    negative and stale positive results therefore bite without cross-package
    cache contamination.
  - PIB-519 route authority keeps raw-byte validation ahead of normalization
    and replaces the generic selector value predicate with exact selector
    domains: `--blob` accepts only `<hash>` or 64 lowercase hex and
    `--generation` accepts only `<id>` or 64 lowercase hex. The exact
    `--blob <slug>` reviewer repro, blob/generation wrong-placeholder
    cross-products, short/non-hex/uppercase values and valid placeholder/
    concrete boundaries all exercise the complete claim/route validator.
    Existing wildcard/metacharacter, traversal, sibling/child, conditional
    and closed-clause protections remain passing.
  - PIB-519's verb-sensitive active anti-stranding grammar now inventories
    `No operator/user[s] can/will/must [ever] block/strand` and
    `leave ... blocked/stranded` forms. The exact reviewer sentence plus
    singular/plural, `ever`, block, strand and leave-stranded controls each
    grow the authoritative inventory and fail without a route. Conditional,
    hypothetical, historical, quoted/attributed, unrelated-subject,
    unrelated-verb, double-negation and `never recover` controls remain
    excluded; an explicit double-negative predicate prevents a leading
    negative quantifier from being re-read as the old `operator cannot`
    positive form.

  Fresh final focused results, each in a separate serial
  `-p=1 -count=1` process: PIB-511 179.93s test / 180.500s package / 181.02s
  wall; PIB-518 70.48s / 71.023s / 72.21s; PIB-519 27.89s / 28.407s /
  31.92s. The package-time sum is 279.930s, below the mandatory 325-second
  observer gate. The unchanged
  `TestS7ObservedARRegistrationAuthority` then ran exactly once in a fresh
  process and passed/correlated all 15 targets in 309.27s test / 310.272s
  package / 310.87s wall, retaining the immutable 12-minute outer, 8-minute
  inner and 1-minute wrong-input budgets and finishing below 420 seconds.

  The rev-10 narrow ledger/meta/empty-target, observer wrong-input/budget,
  PIB-508/520, real PIB-506/507, archive partial/divergence and S4 abandon
  regression set passes in 7.755s package / 8.57s wall. Changed-file gofmt
  and diff checks pass. `go vet -p=1 ./internal/cli` passes in 0.81s; host
  `go build -p=1 ./cmd/tpatch` in 0.40s; Linux amd64 and Darwin amd64 CLI test
  cross-compiles pass in 4.54s and 4.55s.

  New guard SHA-256 is
  `2e7bce99a8c8ccf4df7897f73c6c105b12c149f4682cc5b92c866a69089f637b`.
  Frozen runtime, production renderer, AR ledger, observer/registration and
  AVP retain
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`.
  The executable ledger guard confirms exactly 15 rows `I6/C4/G4/S1`; Side
  Research remains `b385fe622db9926f48861105239f113e`; workflow/assets
  rev-11 diff is empty; HEAD/local/origin remain `c363ed7`; staging is empty.
  Pre-existing root `tpatch` and `cli.test` retain their pre-task mtimes and
  were not touched. No Go process or rev-11 scratch/cross-compile artifact
  remains. No complete `^TestS7AR`, full suite, race, Windows execution, CI,
  staging, commit, push, tag or AS–AX work was performed.
- S7 AR rev-11 independent review — **NEEDS REVISION**. Static adversarial
  review found three Medium false-pass paths: method-expression receivers are
  consumed for parameter alignment but not bound as callable origins;
  concrete route slugs omit production's 60-byte and Windows-reserved-name
  rules; and punctuated/reordered negative-quantifier anti-stranding claims
  escape inventory. The cross-file invalidation correction,
  selector-specific blob/generation domains and PIB-511 closure had no
  material static residual. Per the early-stop rule, the reviewer did not run
  focused Go tests, the observer, regressions, vet/build or cross-compiles.
  Guard and frozen hashes, refs, exact 15-row `I6/C4/G4/S1` ledger, Side
  Research, workflow/assets, staging and process/scratch cleanup remain
  intact.
- S7 AR rev-12 bounded correction — PASS, ready for independent review but
  not approved and unsafe to stage.
  - Test-first complete-validator repros failed against the rev-11 guard
    before correction. The exact receiver-held method-expression store failed
    at 11.79s test / 12.327s package / 16.61s wall because the inventory
    remained unchanged. The exact `con` and 61-byte route cases both
    false-passed at 1.37s / 1.730s / 5.14s. The two punctuated/order claim
    variants remained absent at 1.25s / 1.593s / 2.17s. After the shared
    fixes,
    `go test -v -p=1 -count=1 ./internal/cli -run '^TestS7ARRev12ReviewerRepros$'`
    passes in 12.75s test / 13.258s package / 16.66s wall.
  - PIB-518 derives receiver binding from `types.Selection.Kind()`.
    `types.MethodExpr` binds argument zero to the selected declaration's
    receiver object and declared parameters from argument one;
    `types.MethodVal` binds `SelectorExpr.X` to the receiver and parameters
    from argument zero; ordinary functions retain zero offset and no receiver.
    Receiver origins use the package-wide alias/origin fixpoint. Function
    fields resolve through object identities, reaching assignments,
    keyed/unkeyed composites, address/dereference/conversions and typed
    `Selection.Index()` paths for promoted fields. Receiver-derived
    unresolved invoked fields fail closed without converting unrelated
    production runtime function fields into false failures. Complete
    sensitivities cover value/pointer method expressions, method values,
    promoted fields, pointer aliases, ordinary callback mapping, unresolved
    receiver origins, an inert receiver-held canonical callback and an
    invoked receiver-held decoy callback.
  - PIB-519 removes the local kebab-only route approximation.
    `s7ARValidRouteSlug` now directly invokes production
    `intent.CanonicalSlug` and separately admits only normative `<slug>`.
    Abandon, purge, managed intent-lane removal and managed blob removal all
    call this helper. Complete claim/route sensitivities bite `con` and a
    61-byte slug across all four forms. A production-equivalence table covers
    empty, lowercase ASCII lexical forms, leading/trailing/double dashes,
    non-ASCII/uppercase characters, 60/61-byte boundaries,
    `CON`/`PRN`/`AUX`/`NUL`, `COM1`…`COM9`, `LPT1`…`LPT9`, and safe close
    controls `console`, `com0`, `com10`, `lpt10`; complete route controls
    preserve `<slug>`, safe concrete slugs and rev-11 blob/generation
    domains.
  - PIB-519 removes negative-quantifier forms from the substring regex and
    uses one bounded clause parser. It accepts only exact
    `no operator[s]/user[s]` subjects, accepted modals, optional `ever` before
    or after the modal, benign comma/parenthesis/dash separators, and the
    existing block/strand/leave-blocked/leave-stranded predicates over exact
    slug/archive/transaction subjects. Exact token exhaustion preserves
    exclusions for conditional/hypothetical/historical/quoted/attributed,
    exceptions and qualifiers, unrelated subjects/verbs, unsupported
    `there is no operator`, double negation and negative
    recover/restore/unblock claims.

  Fresh focused commands ran separately and serially with `-p=1 -count=1`:
  `^TestS7ARAbandonGateTableGuard$/^PIB-511$` passed at 180.82s test /
  181.223s package / 181.71s wall;
  `^TestS7ARPurgeProgressGuard$/^PIB-518$` passed at 81.06s / 81.463s /
  81.96s; and `^TestS7ARPermanentBlockClaimsGuard$/^PIB-519$` passed at
  31.93s / 32.435s / 33.53s. Package-time sum is 295.121s. One initial
  mistyped PIB-511 selector produced `no tests to run` in 0.481s package /
  0.97s wall and is not counted. The unchanged observer then ran exactly once
  through a 419-second process-group cutoff and passed/correlated all 15
  targets at 312.07s test / 312.614s package / 313.78s monotonic wall.

  The unchanged rev-11 narrow ledger/meta/empty-target, observer
  wrong-input/budget, PIB-508/520, real PIB-506/507, archive
  partial/divergence and S4 abandon regression set passes in 8.081s package /
  9.28s wall. Changed-file gofmt and diff checks pass.
  `go vet -p=1 ./internal/cli` passes in 0.67s; host
  `go build -p=1 -o /dev/null ./cmd/tpatch` in 0.47s; Linux amd64 and Darwin
  amd64 CLI test cross-compiles to `/dev/null` pass in 3.96s and 4.05s.
  Directing outputs to the null device created no build artifact or cleanup
  target and preserved all pre-existing root binaries.

  New guard SHA-256 is
  `73ff2b1b0bcbff901376baa5e4a787d000f3da7c373f4e03f3944e6db4279c9a`.
  Frozen runtime, production renderer, AR ledger, observer/registration and
  AVP retain
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`.
  The executable ledger remains exactly 15 rows `I6/C4/G4/S1`; Side Research
  remains `b385fe622db9926f48861105239f113e`; workflow/assets rev-12 diff is
  empty; HEAD/local/origin remain `c363ed7`; staging is empty. Root `tpatch`
  and `cli.test` retain their pre-task mtimes; pre-existing `tpatch.exe` and
  `cli.test.exe` were not touched. No Go process or rev-12 scratch/cross-
  compile artifact remains. No complete `^TestS7AR`, full suite, race,
  Windows execution, CI, staging, commit, push, tag or AS–AX work was
  performed.
- S7 AR rev-12 independent review — **NEEDS REVISION**. Static adversarial
  review found one Medium PIB-518 false-pass path. Receiver binding is
  attached only to a selector directly in `call.Fun`; assigning
  `receiver.invoke` to a variable and invoking that alias loses both the
  selected method and bound receiver. The selector origin is excluded by
  `s7ARCallableOriginMayReturnFunction`, while
  `s7ARFunctionLiteralExpressions` accepts selector origins only for
  `types.FieldVal`, not `types.MethodVal`. A receiver-held canonical store can
  therefore execute without entering the inventory or producing an
  unresolved-origin failure. Existing sensitivities invoke method values
  directly and miss local, argument and return transport. Per the early-stop
  rule, the reviewer ran no focused Go tests, observer, regressions,
  vet/build or cross-compiles and did not complete the PIB-511/519 re-audit.
  Guard and frozen hashes, refs, exact 15-row `I6/C4/G4/S1` ledger, Side
  Research, workflow/assets, empty staging and process/scratch cleanup remain
  intact.
- S7 AR rev-13 bounded correction — PASS, ready for independent review but
  not approved and unsafe to stage.
  - Test-first exact reviewer evidence was added before the resolver change.
    `go test -v -p=1 -count=1 ./internal/cli -run
    '^TestS7ARRev13ReviewerRepro$'` failed against rev-12 at 10.04s test /
    10.546s package / 14.12s wall with `same complete validator omitted an
    invoked aliased method value`: the extra canonical store did not enter the
    inventory. The first shared correction made that exact test pass at 9.73s
    / 10.256s / 13.71s. The final test also includes unsupported transported
    interface dispatch and passes at 12.28s / 12.778s / 13.87s, with the
    interface case requiring deterministic `unresolved callable origins`.
  - PIB-518 now resolves a typed `s7ARNamedCallableTarget` alongside existing
    function-literal origins. `types.MethodVal` targets retain the selected
    concrete declaration plus `SelectorExpr.X`; `types.MethodExpr` targets
    retain the declaration with an unbound receiver. Local assignment origins,
    `s7ARBindCallableArguments` parameter origins and exact direct/named return
    expressions all feed the same recursive, cycle-bounded resolver. At an
    invocation, a bound receiver is bound to `FuncDecl.Recv` before declared
    parameters start at argument zero; an unbound method expression takes its
    receiver from argument zero and begins declared parameters at argument
    one. Ordinary named functions retain zero offset/no receiver, and the
    existing literal and function-field resolver remains separate.
  - Interface method values transported to an invocation inspect typed
    assignment/parameter origins, conversions, address/dereference forms and
    composite/field origins to recover concrete receiver types. Exact
    `types.LookupFieldOrMethod` identity selects only a local concrete
    declaration. A transported interface method with no complete concrete
    origin fails closed; a merely materialized method value is never
    activated. Recursive origin cycles terminate through expression-identity
    guards and produce unresolved evidence rather than disappearing.
  - The complete PIB-518 matrix proves locally aliased value and pointer
    method values, nested receiver aliases/address/dereference, parameter
    transport, named-local return transport, statically recoverable interface
    dispatch and an aliased unbound method expression. It uses one typed
    production-source mutation and requires exactly eight actual canonical
    writes, so every one of the five matrix routes must enter the complete
    inventory rather than another route masking it. The exact reviewer test
    independently proves the local value-method alias. The existing direct
    method-value/expression, callback-alignment, receiver-held and promoted
    field cases remain unchanged and green. A consolidated zero-authority
    control proves never-invoked known and unresolved method values stay
    inert, while invoked decoy receiver, ordinary-function alias and
    function-field alias callbacks remain non-authoritative.
  - Iteration remained serial. The first PIB-518 pass exposed the new
    named-result return sensitivity as unresolved at 42.42s test / 42.942s
    package / 44.18s wall. Three bounded diagnostic reruns (40.80s/41.353s,
    40.85s/41.380s and 42.45s/42.991s test/package) showed the function-typed
    named result identifier lacked an expression `TypeAndValue`; object-type
    fallback closed it, and all temporary diagnostics were removed. The full
    unoptimized correction then passed at 101.24s / 101.868s. An initial
    fresh focused set passed individually at PIB-511 205.88s / 206.312s /
    206.79s wall, PIB-518 101.52s / 102.166s / 103.39s, and PIB-519 32.97s /
    33.497s / 34.62s, but its 341.975s package sum exceeded the 325-second
    gate. The observer was correctly skipped at that point.
  - Guard-only optimization removed redundant local binding rescans from the
    new origin resolver, limited package initializer searches to package-scope
    objects, stopped concrete-type traversal once exact type identity was
    known, and consolidated new complete-validator matrices without deleting
    or weakening any rev-0…rev-12 sensitivity. Intermediate PIB-518
    measurements were 107.02s / 107.637s / 111.80s wall before consolidation
    and 91.69s / 92.364s / 96.52s afterward. The final fresh, separate
    `-p=1 -count=1` runs pass at PIB-511 177.64s test / 178.069s package /
    178.65s wall, PIB-518 78.02s / 78.606s / 79.90s, and PIB-519 30.49s /
    30.998s / 32.13s. Final package sum is **287.673s**.
  - Eligibility established, the unchanged
    `TestS7ObservedARRegistrationAuthority` ran exactly once through the
    established 419-second process-group cutoff with `-p=1 -count=1
    -timeout=12m`. It passed/correlated all 15 targets at 306.83s test /
    307.312s package / 307.76s monotonic wall, preserving the immutable
    12-minute outer, 8-minute inner and 1-minute wrong-input budgets.
  - The unchanged rev-12 narrow ledger/meta/empty-target,
    wrong-input/budget, PIB-508/520, real PIB-506/507, archive
    partial/divergence and S4 abandon regressions pass at 8.067s package /
    9.30s wall. Changed-file gofmt and whitespace checks pass.
    `go vet -p=1 ./internal/cli` passes in 0.94s; host
    `go build -p=1 -o /dev/null ./cmd/tpatch` in 0.60s; Linux amd64 and Darwin
    amd64 CLI-test cross-compiles to `/dev/null` pass in 6.36s and 6.02s.
    Null-device outputs created no artifact or cleanup residue and all four
    pre-existing root binaries retained their pre-task sizes/mtimes.
  - New guard SHA-256 is
    `454afa8671e57edc5b0229826880d8bba7933be4bb3f2aaa7fe6adc5839dfd88`.
    Frozen runtime, production renderer, AR ledger, observer/registration and
    AVP retain
    `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
    `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
    `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
    `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
    and
    `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`.
    The executable ledger passes at exactly 15 rows `I6/C4/G4/S1`; Side
    Research remains `b385fe622db9926f48861105239f113e`; workflow/assets
    rev-13 diff and staging remain empty; HEAD/local/origin remain `c363ed7`.
    No Go process or rev-13 scratch/cross-compile residue remains. No complete
    `^TestS7AR`, full suite, race, Windows execution, CI, staging, commit,
    push, tag or AS–AX work was performed.
- S7 AR rev-13 independent review — **NEEDS REVISION**. Static review found
  one Medium PIB-518 false-pass at
  `internal/cli/prepare_s7_ar_guard_test.go:5462-5465`,
  `:5533-5536` and `:5614-5648`. Direct interface invocation passes
  `transported == false`, bypassing both concrete receiver recovery and
  unresolved-dispatch failure. Existing interface sensitivities first alias
  the selected method and therefore cover only transported invocation. The
  mandatory early-stop fired: no reviewer Go command, repro, focused PIB
  timing, observer, regression, vet/build or cross-compile ran, and PIB-511/
  PIB-519 re-audit did not resume. HEAD/local/origin remain `c363ed7`; staging
  and workflow/assets diff remain empty; all supplied hashes match; the
  ledger remains exactly 15 rows `I6/C4/G4/S1`; Side Research remains
  `b385fe622db9926f48861105239f113e`; no Go process, scratch residue or
  root-binary change was present.
- S7 AR rev-14 test-first reviewer repro — expected RED. Exact command
  `go test -v -p=1 -count=1 ./internal/cli -run
  '^TestS7ARRev14ReviewerRepro$'` failed at 10.27s test / 10.807s package /
  15.02s wall with `error=<nil> writes=0 counted=false, want four canonical
  writes`. This is the reviewed false-pass: the valid direct interface
  MethodVal invocation did not recover its concrete receiver, activate the
  concrete method or inventory its receiver-held canonical store. Resource
  preflight reported no Go/compiler/vet/link process and 61% system-wide free
  memory.
- S7 AR rev-14 bounded correction — PASS, ready for independent review but
  not approved and unsafe to stage.
  - `s7ARCallableResolutionContext` separates `invoked`, invocation arguments
    and `transported`. Every real call resolves with invocation context;
    assignment/argument/return origins retain transport context without
    activating a method. Interface MethodVal recovery therefore runs for
    direct and transported invocation alike. Interface MethodExpr recovery
    uses the actual invocation argument zero and returns an unbound target, so
    the existing receiver-at-zero/declared-parameters-at-one offset remains
    intact. Bound MethodVals still bind the selected receiver first and begin
    declared parameters at argument zero.
  - Interface recovery is limited to local callable-bearing interface methods
    in the accepted typed source universe. This captures the PIB-518 callback
    authority class while leaving unrelated production interfaces such as
    file metadata, errors and provider adapters outside callable dispatch
    authority. Known value/pointer concrete origins resolve by exact
    `types.LookupFieldOrMethod`; no concrete origin produces deterministic
    `unresolved callable origins`. Concrete receiver and strict callable-field
    traversal now follow explicit named-helper return expressions, so a
    returned interface receiver activates only when its selected method is
    actually called.
  - The complete PIB-518 matrix adds five independent canonical stores:
    direct value MethodVal, direct pointer MethodVal, direct dispatch in a
    passed-interface helper, direct invocation on a returned interface, and
    direct interface MethodExpr. It requires exactly eight actual writes
    (three production plus five mutations). Separate valid typed sensitivities
    require unresolved evidence for unknown direct MethodVal and MethodExpr
    dispatch. A zero-authority control materializes, passes, returns and stores
    a known interface method without invoking it, then exercises ordinary
    named-function and function-field calls; the complete validator remains
    green. Every rev-0…rev-13 sensitivity remains present and unchanged.
  - Post-red development exposed two intended fail-closed boundaries. The
    first corrected repro run failed at 10.55s test / 11.086s package / 15.43s
    wall because all unrelated external interface calls were treated as
    callable authority. Restricting to local interface methods reduced that
    to the unrelated local `Unwrap` method at 10.32s / 10.880s / 15.26s;
    requiring a callable-bearing signature closed the false positive. The
    exact final reviewer repro passes at 9.68s / 10.201s / 11.33s.
  - The first complete PIB-518 run then failed at 35.39s test / 35.789s
    package / 36.37s wall because strict `receiver.before` traversal did not
    follow the returned-interface helper. Named-helper return traversal closed
    that root and PIB-518 passed at 98.40s / 98.937s / 102.44s during
    development. No diagnostic instrumentation remains.
  - Fresh final serial `-p=1 -count=1` runs pass at PIB-511 181.93s test /
    182.348s package / 182.85s wall, PIB-518 95.93s / 96.600s / 97.69s, and
    PIB-519 31.76s / 32.267s / 33.43s. Package sum is **311.215s**, below the
    325-second eligibility gate. An initial mistyped PIB-511 selector matched
    the top test but no subtest (0.347s package / 0.83s wall); it is excluded
    from the sum and the exact `TestS7ARAbandonGateTableGuard/PIB-511` command
    supplied the recorded result. PIB-511 and PIB-519 therefore retain their
    complete semantic coverage after the resolver change.
  - Eligibility established, the unchanged cold
    `TestS7ObservedARRegistrationAuthority` ran exactly once through the
    established 419-second process-group cutoff with `-p=1 -count=1
    -timeout=12m`. It passed/correlated all 15 targets at 395.37s test /
    395.705s package / 396.16s monotonic wall, preserving the immutable
    12-minute outer, 8-minute inner and 1-minute wrong-input budgets. It was
    not rerun.
  - The established ledger/meta/empty-target, wrong-input/budget,
    PIB-508/520, real PIB-506/507, archive partial/divergence and S4 abandon
    regressions pass at 8.606s package / 9.90s wall. Changed-file gofmt and
    whitespace checks pass. An aggregate preflight/vet wrapper exited one
    without Go diagnostic output; decomposed format/diff/process/memory checks
    were green and standalone `go vet -p=1 ./internal/cli` passed in 0.79s.
    Host `go build -p=1 -o /dev/null ./cmd/tpatch` passed in 0.47s; Linux
    amd64 and Darwin amd64 CLI-test cross-compiles to `/dev/null` passed in
    4.14s and 4.74s. Null-device outputs left no artifact or cleanup residue.
    Every Go phase was serial and preflighted with no active Go/compiler/vet/
    link process and normal memory pressure.
  - New guard SHA-256 is
    `1a34805d303f5f3c69db6601517ed6058b36068237d345ae91f97c346ae696de`.
    Frozen runtime, production renderer, AR ledger, observer/registration and
    AVP retain
    `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
    `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
    `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
    `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
    and
    `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`.
    The executable ledger remains exactly 15 rows `I6/C4/G4/S1`; Side Research
    remains `b385fe622db9926f48861105239f113e`; HEAD/local/origin remain
    `c363ed7`; staging and workflow/assets diff remain empty; all four root
    binaries retain their pre-task sizes/mtimes. No Go process or rev-14
    scratch/cross-compile residue remains. No full/whole `^TestS7AR` suite,
    race, Windows execution, CI, staging, commit, push, tag or AS–AX work was
    performed.
- S7 AR rev-14 independent review — **NEEDS REVISION**, static early-stop.
    `s7ARSignatureCarriesCallable` examines the complete tuple-entry type, while
    `go/types` represents `...func()` as `[]func()`. Known variadic interface
    MethodVal/MethodExpr calls therefore bypass concrete receiver recovery and
    unknown calls bypass fail-closed handling. Existing rev-14 tests cover only
    fixed arity. No reviewer Go command ran. HEAD/local/origin remain
    `c363ed7`; staging and workflow/assets diff remain empty; guard SHA-256
    remains
    `1a34805d303f5f3c69db6601517ed6058b36068237d345ae91f97c346ae696de`;
    all frozen hashes, the exact `I6/C4/G4/S1` ledger and Side Research
    `b385fe622db9926f48861105239f113e` remain intact; process, residue and root
    binary checks are clean.
- S7 AR rev-15 test-first reviewer repro — expected RED. Before changing the
  classifier, a valid typed local `invoke(...func())` interface and concrete
  receiver with receiver-held canonical PurgeProgress authority were added
  through the complete PIB-518 validator. Exact serial command `go test -v
  -p=1 -count=1 ./internal/cli -run
  '^TestS7ARRev15ReviewerRepro$'` failed at 10.23s test / 10.763s package /
  14.94s wall with `error=<nil> writes=0 counted=false, want four canonical
  writes`. This proves rev-14 returned before concrete MethodVal receiver
  recovery and silently omitted the canonical store. Resource preflight
  reported no Go/compiler/vet/link process and 79% system-wide free memory.
- S7 AR rev-15 bounded implementation — PASS, ready for independent review
  but not approved and unsafe to stage.
  `s7ARSignatureCarriesCallable` retains its exact fixed tuple-entry scan and
  adds one variadic-only branch: for `Signature.Variadic()` it inspects only
  the final parameter's unaliased underlying slice element with the existing
  function-valued predicate. This admits `...func()`, aliases and named
  function element types while leaving fixed `[]func()`, maps, channels,
  structs and every other container non-callable. No invocation, transport,
  receiver, origin, package-boundary, cache or fixpoint path changed.
  Complete PIB-518 additions independently require the reviewer MethodVal
  store at exactly four writes and a pointer MethodExpr store added to the
  existing five-route fixed-interface matrix at exactly nine writes. Separate
  unknown MethodVal and MethodExpr cases require unresolved failures. The
  existing rev-14 inert/control validation now additionally spans variadic
  materialization, assignment, parameter/return/store transport, fixed
  containers, an ordinary variadic named function and a variadic
  function-valued field without an extra validator pass. The exact reviewer
  repro now passes at 11.39s test / 11.776s package / 15.86s wall with 79%
  free memory and no competing Go process. Static audit confirms only the
  variadic final slice element enters interface authority; MethodExpr still
  recovers receiver argument zero and inert materialization never reaches the
  invoked branch.
  Fresh final serial `-p=1 -count=1` focused runs pass at PIB-518 100.95s
  test / 101.439s package / 105.82s wall, PIB-511 175.08s / 175.655s /
  176.93s, and PIB-519 31.33s / 31.855s / 33.18s. Package-time sum is
  **308.949s**, below the immutable 325-second observer-eligibility gate.
  Preflights found no competing Go process and 79%, 61% and 63% free memory.
  Eligibility established, the unchanged
  `TestS7ObservedARRegistrationAuthority` ran exactly once through the
  established 419-second process-group cutoff with `-p=1 -count=1
  -timeout=12m`. It passed/correlated all 15 targets at 341.05s test /
  341.564s package / 342.04s monotonic wall, preserving the immutable
  12-minute outer, 8-minute inner and 1-minute wrong-input budgets and staying
  below 420 seconds. Observer validation is complete and will not be rerun.
  The established ledger/meta/empty-target, observer wrong-input/budget,
  PIB-508/520, real PIB-506/507, archive control/partial/divergence and S4
  abandon regression set passes at 18.785s package / 19.75s wall. It includes
  the exact ledger assertion of 15 rows `I6/C4/G4/S1`. Changed-file gofmt and
  whitespace checks pass. `go vet -p=1 ./internal/cli` passes in 0.75s; host
  `go build -p=1 -o /dev/null ./cmd/tpatch` in 0.45s; Linux amd64 and Darwin
  amd64 CLI-test cross-compiles to `/dev/null` pass in 4.27s and 4.74s.
  Preflights found no competing Go process and 74%–77% free memory. Null-device
  outputs created no artifact or cleanup target.
  Final guard SHA-256 is
  `21ceb16f7a430a5a0dd686375624745e685b02355291fc5a1f8355342191ddf3`.
  Frozen runtime, production renderer, AR ledger, observer/registration and
  AVP retain
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`.
  Side Research remains `b385fe622db9926f48861105239f113e`;
  HEAD/local/origin remain `c363ed7`; staging and workflow/assets diff remain
  empty. Root `tpatch`, `cli.test`, `tpatch.exe`, `cli.test.exe` and
  `gitutil.test.exe` retain their pre-task sizes/mtimes. The only files newer
  than dispatch are the two authorized files. No Go process, scratch or
  cross-compile residue remains. No full/whole `^TestS7AR` suite, race,
  Windows execution, CI, staging, commit, push, tag, LOG edit or AS–AX work
  was performed. Rev-15 is implemented for independent review but remains
  unapproved and unsafe to stage.
- S7 AR rev-15 independent review — **NEEDS REVISION**, static early-stop.
  `s7ARSignatureCarriesCallable` now correctly admits the final
  function-valued element of a variadic interface signature, but
  `s7ARBindCallableArguments` still flattens declared parameter identifiers and
  pairs them one-to-one with explicit call arguments. A variadic method has
  only one final AST parameter identifier, so only its first callback reaches
  the callee object; every later callback is dropped. Either direct MethodVal
  or MethodExpr dispatch can therefore recover the concrete target and
  receiver while missing canonical authority in a later callback. Existing
  rev-15 active sensitivities each pass exactly one callback and do not bite
  that route. The equivalent `slice...` origin must be recovered or rejected
  fail-closed.
  The mandatory static early-stop fired, so no reviewer Go command, focused
  PIB test, observer, regression, vet/build or cross-compile ran. PIB-511 and
  PIB-519 were not re-audited after the finding. HEAD/local/origin remain
  `c363ed7`; staging and workflow/assets diff remain empty; guard SHA-256
  remains
  `21ceb16f7a430a5a0dd686375624745e685b02355291fc5a1f8355342191ddf3`;
  every frozen hash, the exact 15-row `I6/C4/G4/S1` ledger and Side Research
  `b385fe622db9926f48861105239f113e` remain intact; process, residue and root
  binary checks are clean. Rev-15 is rejected and unsafe to stage.
- S7 AR rev-16 is dispatched to the existing implementation agent. Authorized
  mutation remains only
  `internal/cli/prepare_s7_ar_guard_test.go` plus this handoff. The correction
  is limited to variadic callable-argument binding: preserve every fixed
  prefix and MethodExpr receiver offset, bind every explicit tail callback to
  the final variadic parameter, and recover expanded-slice origins or reject
  unresolved expansion fail-closed. Complete-validator sensitivities must
  independently place canonical authority after the first variadic callback
  for MethodVal and MethodExpr routes and bite fixed-prefix and expansion
  behavior without broadening fixed-container or inert authority. All frozen
  surfaces, earlier sensitivities, budgets and staging restrictions remain
  unchanged.
- S7 AR rev-16 test-first reviewer repro — expected RED. Before changing the
  binder, `TestS7ARRev16ReviewerRepro` added a valid typed local
  `invoke(...func())` interface whose concrete method invokes
  `callbacks[1]()`. The direct MethodVal call passes a decoy first callback
  and places the canonical PurgeProgress store only in the second callback.
  Exact serial command `go test -v -p=1 -count=1 ./internal/cli -run
  '^TestS7ARRev16ReviewerRepro$'` failed at 10.79s test / 11.329s package /
  15.27s wall with `error=<nil> writes=0 counted=false, want four canonical
  writes`. This proves rev-15 resolves the method and receiver but discards
  explicit variadic arguments after the first. Resource preflight found no
  Go/compiler/vet/link process and 84% system-wide free memory.
- S7 AR rev-16 bounded implementation — PASS, ready for independent review
  but not approved and unsafe to stage.
  `s7ARBindCallableArguments` now derives semantic parameter positions while
  preserving unnamed and multi-name fixed fields, keeps the existing
  MethodExpr receiver slice offset, binds every explicit variadic tail
  expression to the one final parameter object, and expands `slice...` only
  when that final parameter's unaliased slice element is function-valued.
  Statically known composites, local/package aliases, named slice/element
  types, callable fields and direct/literal/named helper returns are expanded;
  origin cycles or missing/unsupported expansion evidence remain incomplete.
  The final invoked-call audit resolves every expanded callback to a literal
  or named callable target and deterministically reports unresolved origins.
  Fixed `[]func()` and non-callable variadics keep their prior one-argument
  binding and never enter the expansion authority path.
  The first post-change reviewer run exposed an over-broad final check against
  ordinary `...string` Git helpers and failed at 10.55s test / 11.066s
  package / 14.80s wall with deterministic `runGitCapture`/`runGitEnvOut`
  evidence. The shared predicate was then narrowed to the accepted
  function-valued variadic element only; no production fixture was special
  cased. The same exact reviewer command passed at 10.45s test / 10.978s
  package / 14.69s wall. Both preflights found no competing Go process and
  83% free memory. A PIB-518 implementation probe then passed at 100.47s
  test / 100.998s package / 101.54s wall with 83% free memory.
  Complete-validator sensitivities now independently bite direct MethodVal
  authority in callback two, direct MethodExpr callback-two binding, grouped
  fixed parameters plus receiver offset, a named helper returning an aliased
  named callback slice for `slice...`, and a known receiver with unresolved
  expanded slice. Existing rev-15 unknown MethodVal/MethodExpr, inert
  transport and fixed-container/ordinary-function controls remain unchanged.
  Final serial validation passes the exact reviewer repro at 11.30s test /
  11.876s package / 16.33s wall; PIB-518 at 98.17s / 98.615s / 99.11s;
  PIB-511 at 178.73s / 179.343s / 180.55s; and PIB-519 at 28.86s / 29.194s /
  29.67s. The three focused package times sum to **307.152s**, below the
  immutable 325-second observer gate. Preflights found no competing Go
  process and 71%, 72%, 66% and 71% free memory. An initial mistyped
  `TestS7ARRetryHeadingGuard/PIB-519` selector reported no tests in 0.499s
  package / 1.59s wall; it is excluded from evidence, was immediately
  corrected to the ledger-bound `TestS7ARPermanentBlockClaimsGuard/PIB-519`,
  and did not overlap any Go work. Observer eligibility is established.
  The unchanged `TestS7ObservedARRegistrationAuthority` then ran exactly once
  for rev-16 through the established 419-second process-group cutoff with
  `-p=1 -count=1 -timeout=12m`. All 15 targets passed/correlated at 322.02s
  test / 322.505s package / 322.90s monotonic wall, below 420 seconds while
  preserving the immutable 12-minute outer, 8-minute inner and 1-minute
  wrong-input budgets. Preflight found no competing Go process and 73% free
  memory. This observer run is final and will not be rerun.
  The established ledger/meta/empty-target, observer wrong-input/budget,
  PIB-508/520, real PIB-506/507, archive control/partial/divergence and S4
  abandon regression set passes at 18.560s package / 19.82s wall, including
  the exact 15-row `I6/C4/G4/S1` ledger assertion. Preflight found no competing
  Go process and 72% free memory. Changed-file gofmt and whitespace checks
  pass. `go vet -p=1 ./internal/cli` passes in 0.63s; host
  `go build -p=1 -o /dev/null ./cmd/tpatch` in 0.44s; Linux amd64 and Darwin
  amd64 CLI-test cross-compiles to `/dev/null` in 4.17s and 3.67s.
  Preflights found no competing Go process and 77% free memory.
  Final guard SHA-256 is
  `aa467f908123601e275450e68db226383ae70a922c4b4ff7e8e77a36ac5788a0`.
  Frozen runtime, production renderer, AR ledger, observer/registration and
  AVP remain
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`.
  Side Research remains `b385fe622db9926f48861105239f113e`;
  HEAD/local/origin remain `c363ed7`; staging and workflow/assets diff remain
  empty. Root `tpatch`, `cli.test`, `tpatch.exe`, `cli.test.exe` and
  `gitutil.test.exe` retain their dispatch sizes/mtimes. The only files newer
  than dispatch are the two authorized files; no scratch or cross-compile
  output was created. A separate `go test ./internal/ -count=1` process
  appeared after all rev-16 Go work had completed; no rev-16 command
  overlapped it, and it exited during a bounded 30-second wait before the
  final process check. No full/whole `^TestS7AR` suite, race, Windows
  execution, CI, staging, commit, push, tag, LOG edit or AS–AX work was
  performed. Rev-16 is ready for independent review but remains unapproved
  and unsafe to stage.
- S7 AR rev-16 independent review — **NEEDS REVISION**, static early-stop.
  The full variadic tail now reaches the callee, but three Medium PIB-518
  defects remain. First, all tail callbacks collapse into one parameter-object
  origin set and `IndexExpr` ignores its constant index, so authority in an
  uninvoked `callbacks[1]` can be attributed to a method that invokes only
  `callbacks[0]`. Second, expanded-slice origin collection ignores element
  writes, `copy`, and alias mutation; a known stale initializer can therefore
  suppress fail-closed evidence and omit the callback actually invoked.
  Third, unresolved expansion is rejected for every known callable-variadic
  target even when its parameter is never invoked or forwarded to an invoked
  sink, creating a false failure for inert `retain(callbacks...)`.
  The rev-16 positive sensitivities do not bite those inverse and mutation
  paths. The mandatory static early-stop fired, so no reviewer Go command,
  focused PIB test, observer, regression, vet/build, cross-compile or CI ran;
  PIB-511 and PIB-519 were not re-audited after the findings.
  HEAD/local/origin remain `c363ed7`; staging and workflow/assets diff remain
  empty; guard SHA-256 remains
  `aa467f908123601e275450e68db226383ae70a922c4b4ff7e8e77a36ac5788a0`;
  every frozen hash, the exact 15-row `I6/C4/G4/S1` ledger and Side Research
  `b385fe622db9926f48861105239f113e` remain intact. The canonical rev-16 RED
  evidence is this handoff's `error=<nil> writes=0 counted=false`; the review
  dispatch brief's `writes=3 counted=true` text was a transcription error.
  Rev-16 is rejected and unsafe to stage.
- S7 AR rev-17 is dispatched to the existing implementation agent. Authorized
  mutation remains only
  `internal/cli/prepare_s7_ar_guard_test.go` plus this handoff. The correction
  must preserve ordered variadic-element origins so a constant index resolves
  exactly while genuinely dynamic index/range traversal conservatively
  unions; model or fail closed on element writes, `copy`, and alias mutation;
  and gate unresolved expanded-slice rejection on actual or transitive
  invocation of the callee variadic parameter. Same-complete-validator
  sensitivities must cover inverse MethodVal and MethodExpr indexing,
  index-write, `copy`, alias mutation, dynamic traversal, and inert unresolved
  expansion. Frozen surfaces, prior sensitivities, budgets and staging
  restrictions remain unchanged.
- S7 AR rev-17 test-first reviewer repros — expected RED before helper
  changes. Resource preflight found no Go/compiler/vet/link process and 82%
  free memory. Exact
  `go test -p=1 -count=1 ./internal/cli -run '^TestS7ARRev17ReviewerRepros$'`
  failed at 19.11s test / 19.479s package / 22.79s wall. Direct MethodVal and
  MethodExpr inverse cases each invented the uninvoked index-1 store
  (`writes=4`); direct index write, `copy`, and alias index write each omitted
  the invoked canonical callback (`error=<nil> writes=0 counted=false`); the
  inert named expanded slice failed closed with unresolved-callable evidence.
  The initial unnamed function-field parameter correctly remained inert; it
  was strengthened to the reviewer's callable-bearing named-parameter form
  and independently RED under
  `-run '^TestS7ARRev17ReviewerRepros$/inert-field-expanded-slice$'` at 9.67s
  test / 10.185s package / 13.78s wall with unresolved-callable evidence.
  Every construction parsed/type-checked and invoked the complete PIB-518
  validator. No helper implementation had changed at RED.
- S7 AR rev-17 bounded implementation and narrow GREEN. Callable origins now
  carry a coherent ordered-sequence abstraction: alternatives retain element
  position and source function, constant indices select one position,
  dynamic indices/ranges conservatively union candidates, and only
  variadic-bound sequences enter this authority path. Fixed parameters,
  MethodExpr receiver offset and fixed `[]func()` exclusion remain intact.
  A bounded forward sequence interpreter applies exact direct index writes,
  `copy`, and shared direct aliases; exact overwrites replace stale
  initializers. Conditional, append, reslice and package-scope mutation paths
  retain candidates but mark the sequence incomplete so an invoked sink fails
  closed. Expanded-slice completeness is now gated by direct or transitive
  use of the variadic parameter; unresolved inert named functions and
  function-valued fields remain clean.
  The original seven repros first passed at 18.431s package / 21.40s wall.
  The expanded sensitivity pass initially exposed one fixpoint issue:
  package-root analysis visited a range body before its variadic parameter
  was bound, poisoning the element origin as incomplete (32.648s package /
  35.81s wall). Range binding was gated on a real variadic-bound origin; the
  exact range case then passed at 10.100s package / 14.02s wall. A subsequent
  0.56s compile-only check caught swapped literal/named fallback returns while
  adding the fixed-slice exclusion and was corrected before runtime
  validation. Final exact
  `go test -v -p=1 -count=1 ./internal/cli -run '^TestS7ARRev17ReviewerRepros$'`
  passes all 20 typed complete-validator subtests at 38.25s test / 38.646s
  package / 42.35s wall. Independent expectations cover inverse and positive
  MethodVal/MethodExpr indices, dynamic index, range, direct/copy/alias
  mutation and stale-overwrite inverses, inert named/field expansions,
  transitive invocation, conditional/append/reslice/package fail-closed
  routes, and fixed-slice exclusion. Each Go preflight found no competing
  Go/compiler/vet/link process and 82% free memory.
- S7 AR rev-17 fresh focused eligibility gate — PASS. Rev-14 through rev-17
  reviewer repros pass together at 44.202s package / 45.38s wall (rev-17
  30.42s). Fresh serial selectors pass: PIB-518 at 84.94s test / 85.350s
  package / 86.25s wall with 74% free memory; PIB-511 at 170.78s test /
  171.409s package / 172.43s wall with 67% free memory; PIB-519 at 30.18s
  test / 30.703s package / 31.75s wall with 73% free memory. The fresh package
  sum is **287.462s**, below the immutable 325-second observer eligibility
  gate. No competing Go/compiler/vet/link process was active at any preflight.
- S7 AR rev-17 unchanged observer — PASS on its single authorized
  implementation run. A fresh preflight found no competing Go process and
  73% free memory. `TestS7ObservedARRegistrationAuthority` ran through a new
  process group with a 419-second cutoff and
  `go test -v -p=1 -count=1 -timeout=12m`; all 15 registered AR targets
  passed at 307.87s test / 308.365s package / 308.81s outer wall. The cutoff
  did not fire, and the frozen observer retained its 12m/8m/1m budgets. This
  observer run is final and will not be rerun.
- S7 AR rev-17 established narrow regressions — PASS at 7.751s package /
  8.49s wall with 73% free memory and no competing Go process. The exact
  15-row `I6/C4/G4/S1` ledger and empty-target meta-guard, observer
  wrong-input/correlation/budget cases, PIB-508/520, all real PIB-506/507
  divergence classes, archive partial/divergence branches and the three S4
  abandon regressions passed under one bounded `-p=1 -count=1` selector.
- S7 AR rev-17 final validation and boundary — PASS. Changed-file gofmt,
  no-index whitespace and repository `git diff --check` are clean.
  `go vet -p=1 ./internal/cli` passes in 0.64s; host
  `go build -p=1 -o /dev/null ./cmd/tpatch` in 0.39s; Linux amd64 and Darwin
  amd64 CLI-test cross-compiles to `/dev/null` in 4.15s and 3.83s.
  Preflights found no competing Go/compiler/vet/link process and 74–76% free
  memory. Final guard SHA-256 is
  `01aa7379b2b23fbc6e844adb8b1db057c5ced8c3183dbc714387a0aa6434ea35`.
  Frozen runtime, production renderer, AR ledger, observer/registration and
  AVP remain
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`.
  Side Research remains `b385fe622db9926f48861105239f113e`;
  HEAD/local/origin remain `c363ed7`; staging and workflow/assets diff remain
  empty. Parent-owned `docs/supervisor/LOG.md` is untouched by rev-17. Root
  binaries retain dispatch sizes/mtimes; only the two authorized files are
  newer than dispatch; no scratch/cross-compile output or Go process remains.
  No full/whole `^TestS7AR` suite, race, Windows execution, CI, staging,
  commit, push, tag, LOG edit or AS–AX work was performed. Append, reslice and
  package-scope mutation are intentionally conservative fail-closed routes,
  not unresolved implementation gaps. Rev-17 is ready for independent review
  but remains unapproved and unsafe to stage.
- S7 AR rev-17 independent static review — **NEEDS REVISION**. Boundary
  verification passed: HEAD/local/origin remain `c363ed7`; staging and
  workflow/assets diff remain empty; guard SHA-256 remains
  `01aa7379b2b23fbc6e844adb8b1db057c5ced8c3183dbc714387a0aa6434ea35`;
  every frozen hash, the exact 15-row `I6/C4/G4/S1` ledger and Side Research
  `b385fe622db9926f48861105239f113e` match; no file or process residue was
  introduced. Static review found three Medium PIB-518 gaps. An invoked
  explicit variadic element can retain `transported=false` and resolve
  empty-but-complete when its callback identifier is unknown. Local helper
  mutation and guaranteed append backing-array reuse can leave stale sequence
  origins. Expansion completeness is gated by one parameter-level boolean, so
  an exact `callbacks[0]()` sink can reject an unknown but provably uninvoked
  index 1. Existing sensitivities do not bite those routes. The mandatory
  static early-stop fired; no Go, observer, vet, build, cross-compile or CI
  command ran, and PIB-511/519 were not completed. Rev-17's implementation
  claims above are historical evidence, not acceptance evidence.
- S7 AR rev-18 is dispatched to the existing implementation agent. Authorized
  mutation remains only `internal/cli/prepare_s7_ar_guard_test.go` plus this
  handoff. Rev-18 must preserve transported authority for every invoked
  sequence element, model or conservatively invalidate local-helper and
  shared-backing append/reslice mutation only along invoked routes, and
  propagate an element-sensitive invocation demand: exact constant indices
  demand only reachable elements, while dynamic index/range and unresolved
  forwarding demand every possible element. Independent complete-validator
  sensitivities and inverses must cover unknown explicit callbacks, local
  helper element mutation, guaranteed append backing reuse, exact-index
  expansion with an unknown uninvoked sibling, and dynamic/range counterparts.
  Frozen surfaces, prior sensitivities, budgets and staging restrictions
  remain unchanged.
- S7 AR rev-18 test-first baseline — **RED**. After a clean process preflight
  and 82% free memory,
  `go test -v -p=1 -count=1 ./internal/cli -run '^TestS7ARRev18ReviewerRepros$'`
  failed in 41.031s package / 44.63s wall with 12/22 controls already passing
  and ten intended bites. Both selected unknown explicit MethodVal/MethodExpr
  routes returned `error=<nil>`; exact expanded index 0, its MethodExpr form
  and the one-hop exact forward falsely rejected an unknown uninvoked index 1;
  local helper direct/alias mutation, unresolved helper mutation and
  guaranteed append reuse all returned `error=<nil>`. Exact known,
  unselected explicit, inert transport, out-of-range, selected/dynamic/range,
  transitive-selected, reslice mutation, post-sink mutation, read-only helper,
  disjoint append and forced-allocation inverses behaved independently.
  Assertions distinguish unresolved-origin diagnostics, exact four-write
  inventory and clean results; no syntax/type/model error or count overlap
  remains. The earlier bodyless-function fixture exposed an unrelated
  pre-existing S6 model panic and was replaced before this canonical RED with
  a valid function-valued variable, without changing any helper.
- S7 AR rev-18 bounded implementation/new sensitivity matrix — **GREEN**.
  `TestS7ARRev18ReviewerRepros` now passes 28/28 in 49.403s package /
  52.79s wall after an 83% free-memory and empty-toolchain preflight. The
  coherent correction adds one invocation-demand domain (exact source
  indices versus all), carries it through local sequence aliases, element
  aliases, direct/transitive variadic forwarding and MethodVal/MethodExpr
  dispatch, and applies it to both explicit selected elements and expanded
  slices. Sequence uncertainty is now index-addressed where effects are
  bounded; local helper direct/alias/nested effects, unknown calls, append
  overlap and reslice backing aliases mark only reachable backing positions,
  in source order. Read-only helpers, post-sink mutation, disjoint/reassigned
  aliases, forced append allocation, read-only reslices and uninvoked mutated
  indices stay clean. Empty transient forwarding states no longer poison the
  monotone origin fixpoint. The matrix independently covers explicit and
  expanded exact/dynamic/range demand, selected/unselected unknowns, direct
  MethodVal/MethodExpr, one-hop sequence and element aliases, helper and
  append/reslice mutation, and exact inverses. No production or frozen surface
  changed.
- S7 AR rev-18 inherited and focused eligibility — **PASS**. Rev-14 through
  rev-18 reviewer repros pass together in 84.025s package / 85.09s wall.
  Fresh serial `-p=1 -count=1` package times are PIB-518 89.259s
  (88.83s test / 90.00s wall), PIB-511 166.048s (165.47s / 167.18s), and
  PIB-519 31.644s (31.09s / 32.46s), totaling **286.951s**, below the
  immutable 325-second observer gate. Each phase had an empty-toolchain
  preflight and 71–74% free memory. Rev-18 therefore earns exactly one
  unchanged observer run recorded below.
- S7 AR rev-18 unchanged observer — **PASS** on its single authorized
  implementation run. A fresh preflight found no competing toolchain process
  and 75% free memory. The observer ran in a new process group with the
  immutable 419-second cutoff and
  `go test -v -p=1 -count=1 -timeout=12m ./internal/cli -run
  '^TestS7ObservedARRegistrationAuthority$'`; all 15 registered AR targets
  passed at 320.53s test / 321.047s package / 321.48s outer monotonic wall.
  The cutoff did not fire, and the frozen 12m/8m/1m budgets remain in force.
  This rev-18 observer run is consumed and must not be rerun.
- S7 AR rev-18 established narrow regressions and final validation — **PASS**.
  The exact ledger/empty-target, observer wrong-input/budget, PIB-508/520,
  real PIB-506/507, archive partial/divergence and three S4 abandon selectors
  pass in 7.433s package / 8.28s wall. Changed-file gofmt, no-index whitespace
  and repository `git diff --check` are clean. `go vet -p=1 ./internal/cli`
  passes in 0.62s; host `go build -p=1 -o /dev/null ./cmd/tpatch` in 0.41s;
  Linux and Darwin amd64 CLI-test cross-compiles to `/dev/null` in 3.73s and
  3.71s. Preflights found no competing toolchain process and 73–76% free
  memory. Final guard SHA-256 is
  `9e6641df2a5ead424bd721e3c7c10c51ed423adea49e88bbaa356bd6d6719093`.
  Frozen runtime, renderer, ledger, observer/registration and AVP hashes remain
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`.
  Side Research remains `b385fe622db9926f48861105239f113e`;
  HEAD/local/origin remain `c363ed7`; staging and workflow/assets diff remain
  empty. Only the two authorized files are newer than dispatch; parent-owned
  LOG and all root binaries retain their boundary state; no process, scratch
  or cross-compile residue remains. No full/whole `^TestS7AR`, race, Windows,
  CI, staging, commit, push, tag, LOG edit or AS–AX work occurred.
- S7 AR rev-18 independent static review — **NEEDS REVISION**. Boundary
  verification passed: HEAD/local/origin remain `c363ed7`; staging and
  workflow/assets diff remain empty; guard SHA-256 remains
  `9e6641df2a5ead424bd721e3c7c10c51ed423adea49e88bbaa356bd6d6719093`;
  every frozen hash, the exact 15-row `I6/C4/G4/S1` ledger and Side Research
  `b385fe622db9926f48861105239f113e` match; no file or process residue was
  introduced. Static review found one Medium PIB-518 false positive.
  `s7ARCallableVariadicParameterDemand` computes sequence views and element
  demands with whole-body monotonic unions, so a definite reassignment to a
  disjoint slice cannot kill a stale parameter-derived view and a later
  assignment can contaminate an earlier call. An unknown callback passed by
  the caller is therefore treated as invoked even when the callee first
  replaces its variadic parameter with an independent resolved slice. The
  same defect affects sequence and element aliases. Existing sensitivities do
  not overwrite a transitive forwarding value. The mandatory static
  early-stop fired; no Go, observer, vet, build, cross-compile or CI command
  ran, and remaining backing-model/PIB-511/PIB-519 review did not complete.
  Rev-18's implementation claims above are historical evidence, not
  acceptance evidence.
- S7 AR rev-19 is dispatched to the existing implementation agent. Authorized
  mutation remains only `internal/cli/prepare_s7_ar_guard_test.go` plus this
  handoff. Rev-19 must replace whole-body monotonic demand propagation with
  program-point-aware reaching state: definite straight-line reassignment
  strongly replaces a sequence or element view, genuine control-flow joins
  conservatively merge alternatives, and statements after a call cannot
  affect that call. Independent complete-validator sensitivities and inverses
  must cover variadic-parameter, sequence-alias and element-alias replacement
  before invocation, post-call assignment, conditional/loop replacement and
  transitive forwarding. Frozen surfaces, earlier sensitivities, budgets and
  staging restrictions remain unchanged.
- S7 AR rev-19 implementation started from the exact rejected rev-18 guard
  baseline
  `9e6641df2a5ead424bd721e3c7c10c51ed423adea49e88bbaa356bd6d6719093`.
  Boundary preflight confirms HEAD/local/origin `c363ed7`, empty staging,
  every frozen hash and Side Research invariant, no active Go toolchain
  executable and 83% free memory. Test-first complete-validator sensitivities
  are being added before any demand-helper edit.
- S7 AR rev-19 complete-validator RED — **11 intended bites, 11 disclosed
  controls PASS** in 40.02s test / 40.553s package / 43.70s wall with `-p=1
  -count=1`. Every case parsed and type-checked. The canonical failing results
  were unresolved-callable evidence with `writes=0 counted=false` rather than
  syntax/type or inventory-count overlap:
  `variadic-parameter-replaced-known-before-forward`
  `[applyIntentArchivePurgeResult:s7ARRev19ReplaceKnown]`;
  `sequence-alias-replaced-known-before-forward`
  `[applyIntentArchivePurgeResult:s7ARRev19ReplaceSequenceAlias]`;
  `element-alias-overwritten-known-before-invocation`
  `[applyIntentArchivePurgeResult:s7ARRev19ReplaceElementKnown
  s7ARRev19ReplaceElementKnown:selected]`;
  `assignment-after-forward-cannot-contaminate-earlier-call`
  `[applyIntentArchivePurgeResult:s7ARRev19AssignAfterForward]`;
  `conditional-all-branch-replacement-kills-entry`
  `[applyIntentArchivePurgeResult:s7ARRev19ReplaceAllBranches
  s7ARRev19ReplaceAllBranches:s7ARRev19Sink
  s7ARRev19Sink:callbacks[0]]`;
  `loop-body-replacement-before-only-invocation`
  `[applyIntentArchivePurgeResult:s7ARRev19ReplaceInsideInvokingLoop]`;
  `multiple-calls-use-state-at-each-program-point`
  `[applyIntentArchivePurgeResult:s7ARRev19TwoResolvedCalls]`;
  `two-hop-replacement-preserves-program-order`
  `[applyIntentArchivePurgeResult:s7ARRev19ReplaceBeforeMiddle]`;
  `reslice-alias-replacement-kills-alias-route`
  `[applyIntentArchivePurgeResult:s7ARRev19ReplaceResliceAlias]`;
  and MethodVal/MethodExpr receiver-offset cases
  `[applyIntentArchivePurgeResult:s7ARRev19Invoker{}.replace]` /
  `[applyIntentArchivePurgeResult:s7ARRev19Invoker.replace]`.
  The 11 pre-fix controls that already passed were unknown strong replacement,
  alias/original separation, unknown element overwrite, before-call
  contamination, one-branch and zero-iteration joins, later unknown call,
  original-after-reslice, exact unselected sibling, dynamic demand and range
  demand. No demand-helper implementation had changed when RED was recorded.
- S7 AR rev-19 primary GREEN — **22/22 PASS** in 42.04s test / 42.548s
  package / 45.86s wall. The correction replaces whole-body demand unions
  with bounded program-point states, strong RHS-before-LHS updates,
  alternative-preserving branch/loop flow and exact call-site observations.
  The same reaching states now constrain scalar invocation resolution,
  expanded-sequence completeness and interprocedural callable binding, which
  closes stale element-alias and all-branch sequence origins without weakening
  the 11 unknown/partial/dynamic controls. Preflight found no active Go
  toolchain executable and 85% free memory. Static self-audit and challenge
  sensitivities remain pending before focused eligibility.
- S7 AR rev-19 static self-audit challenge RED — four additional intended
  bites in 21.07s test / 21.630s package / 24.97s wall, with four controls
  already passing. Direct and shared-backing alias element overwrite both
  produced outer unresolved evidence
  `[applyIntentArchivePurgeResult:s7ARRev19OverwriteElement]` /
  `[applyIntentArchivePurgeResult:s7ARRev19OverwriteThroughAlias]` instead of
  four writes. Disjoint-alias and post-return helper writes both incorrectly
  poisoned the unchanged source sequence and produced
  `[applyIntentArchivePurgeResult:s7ARRev19Sink
  s7ARRev19Sink:callbacks[0]]` instead of clean. Alias reassignment preserving
  the original, terminating-branch join, all-case switch replacement and
  RHS-before-LHS multi-assignment controls passed. These bites prove two
  bounded sibling gaps: exact/shared element replacement was not reflected in
  invocation demand, and `s7ARCallableParameterMutationDemand` retained the
  rejected whole-body alias union. Eligibility has not started.
- S7 AR rev-19 static self-audit correction and final matrix — **41/41 PASS**
  in 74.76s test / 75.165s package / 75.69s wall. The unified state now clones,
  compares and caps whole correlated alternatives; applies RHS-before-LHS
  strong updates; tracks exact backing identity/offset and per-index
  overrides; walks return, if, switch, type-switch, select, loop and range
  flow; records each call's reaching state; and keys recursive demand by the
  callee body while failing closed at 64 states or 32 loop iterations.
  State-aware target activation, binding and final audit prevent a later
  callable alias from activating or poisoning an earlier inert call. The same
  walker replaces the old whole-body mutation helper, so disjoint alias writes
  and unreachable post-return writes stay inert while shared-backing writes
  remain visible. Additional bite-proofs cover direct/shared element writes,
  all/one-branch overwrite, unknown overwrite, terminating branches,
  switch/select/type-switch joins, multi-assignment, scalar forwarder
  reassignment, shadowed objects and inert closures. A competing external Go
  workload was observed once and allowed to finish before the next command;
  no commands overlapped. Focused inherited repros and eligibility remain
  pending.
- S7 AR rev-19 inherited repros and focused eligibility — **PASS**.
  Rev-14 through rev-19 reviewer repros pass together in 154.781s package /
  155.58s wall. Fresh serial package times are PIB-518 **89.470s**
  (90.55s wall), PIB-511 **167.192s** (167.70s wall) and PIB-519 **31.332s**
  (31.89s wall), totaling **287.994s**, below the immutable 325-second gate.
  Preflights found no active Go toolchain executable and 71–82% free memory.
  Two unrelated broad Go workloads were observed between phases and allowed
  to finish before commands resumed; no workload overlapped. Rev-19 has earned
  exactly one unchanged observer run recorded below.
- S7 AR rev-19 unchanged observer — **PASS** on its single authorized
  implementation run. A fresh preflight found no active Go toolchain
  executable and 77% free memory. The observer ran in a new process group with
  the immutable 419-second cutoff and
  `go test -v -p=1 -count=1 -timeout=12m ./internal/cli -run
  '^TestS7ObservedARRegistrationAuthority$'`; all 15 registered AR targets
  passed at 304.71s test / 305.229s package / 305.740s outer monotonic wall.
  The cutoff did not fire, and the frozen 12m/8m/1m budgets remain unchanged.
  This rev-19 observer run is consumed and must not be rerun.
- S7 AR rev-19 established narrow regressions and final validation — **PASS**.
  The exact ledger/empty-target, observer wrong-input/budget, PIB-508/520,
  real PIB-506/507, archive partial/divergence and three S4 abandon selectors
  pass in 7.944s package / 9.09s wall. Changed-file gofmt, no-index whitespace
  and repository `git diff --check` are clean. `go vet -p=1 ./internal/cli`
  passes in 0.80s; host `go build -p=1 -o /dev/null ./cmd/tpatch` in 0.47s;
  Linux and Darwin amd64 CLI-test cross-compiles to `/dev/null` in 3.84s and
  3.86s. Preflights found no competing Go toolchain executable and 74–75%
  free memory. Final guard SHA-256 is
  `b752aa3f8c00862d00967907fdb8d3030c12deb7ba5af646bcad83b861511800`.
  Frozen runtime, renderer, ledger, observer/registration and AVP hashes remain
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`.
  Side Research remains `b385fe622db9926f48861105239f113e`;
  HEAD/local/origin remain `c363ed7`; staging and workflow/assets diff remain
  empty. Only the two authorized files are newer than dispatch; parent-owned
  LOG and all root binaries retain their boundary state; no toolchain,
  scratch or cross-compile residue remains. No full/whole `^TestS7AR`, race,
  Windows, CI, staging, commit, push, tag, LOG edit or AS–AX work occurred.
- S7 AR rev-19 independent re-review — **NEEDS REVISION**. Boundary evidence
  remained intact: HEAD/local/origin `c363ed7`, empty staging, clean
  whitespace, exact `I6/C4/G4/S1` ledger, every frozen hash, Side Research and
  no workflow/assets or residue drift. The reviewer found two Medium PIB-518
  gaps and fired the mandatory static early-stop before any Go command:
  1. `defer`/`go` expanded calls are permanently observed at their statement
     state. A deferred call captures the expanded slice descriptor, so a later
     shared-backing element write can change the callback invoked at function
     return; a post-`go` mutation is timing-dependent. Rev-19 can false-clean
     both because none of its 41 cases covers either statement shape.
  2. `s7ARCallableReachingStatesAtCalls` returns no reaching states when the
     containing function lacks a callable variadic parameter. Non-variadic
     scalar aliases and fixed slices therefore fall back to the rejected
     whole-body union, allowing a later replacement to contaminate an earlier
     synchronous call. The existing post-call scalar control lives in a
     variadic owner and cannot bite this route.
  The 41 rev-19 sensitivities otherwise use substantive typed outcomes, but
  the backing-model remainder, PIB-511/519, overflow behavior, timing and
  observer were not independently reverified after the findings. Guard hash
  remains
  `b752aa3f8c00862d00967907fdb8d3030c12deb7ba5af646bcad83b861511800`.
- S7 AR rev-20 is dispatched to the existing implementation agent. Authorized
  mutation remains only `internal/cli/prepare_s7_ar_guard_test.go` plus this
  handoff. Test-first complete-validator sensitivities and true inverses must
  cover deferred expanded calls before/after shared-backing mutation,
  descriptor-only reassignment, relevant asynchronous alternatives, and
  non-variadic scalar/fixed-slice assignments before and after invocation.
  Deferred calls must use every reachable execution/return state or fail
  closed when their captured backing can change; asynchronous calls must
  conservatively retain every statement-time/later target. The correlated
  walker must run for every active function rather than falling back to a
  whole-body union when no callable variadic parameter exists. All rev-14–19
  sensitivities, frozen surfaces, budgets and staging restrictions remain
  unchanged.
- S7 AR rev-20 implementation started from exact guard baseline
  `b752aa3f8c00862d00967907fdb8d3030c12deb7ba5af646bcad83b861511800`.
  Boundary preflight confirms HEAD/local/origin `c363ed7`, empty staging,
  every frozen hash and Side Research invariant, no active Go toolchain
  executable and no boundary drift. The parent-owned LOG and pre-existing
  runtime/registration/research worktree entries remain untouched. Test-first
  complete-validator defer/go and non-variadic sensitivities are being added
  before any reaching-state helper edit.
- S7 AR rev-20 primary complete-validator RED — **9 intended bites, 10
  disclosed controls PASS** in 36.20s test / 36.754s package / 39.92s wall
  with `-p=1 -count=1`; preflight found no active Go toolchain executable and
  86% free memory. Every fixture parsed and type-checked. Deferred direct,
  shared-alias and one-branch unknown backing overwrites plus the post-`go`
  concurrent overwrite incorrectly returned the four-write inventory
  (`applyIntentArchivePurgeResult:direct-builder` duplicated) instead of
  unresolved `s7ARRev20Sink` evidence. Ordinary scalar assignment after its
  call, explicit deferred scalar capture and two ordinary scalar calls before
  a later unknown assignment incorrectly returned unresolved
  `s7ARRev20ScalarAfter:callback`, `s7ARRev20DeferScalar:callback` and two
  `s7ARRev20ScalarCalls:callback` routes with `writes=0 counted=false`.
  An ordinary fixed-slice unknown write before the sink incorrectly returned
  four writes, while an all-path known overwrite after a defer incorrectly
  returned clean (`err=nil`, `writes=0 counted=false`) rather than four
  writes. Passing pre-fix controls were ordinary fixed-slice post-call write,
  unmutated defer/go, defer/go descriptor reassignment, unknown-before-defer,
  unknown-at-go statement, ordinary scalar pre-call assignment, unreachable
  post-return mutation and disjoint alias write. No reaching-state helper had
  changed when this canonical RED was recorded.
- S7 AR rev-20 primary correction — **19/19 GREEN** in 38.66s test / 39.218s
  package / 42.49s wall; preflight found no active Go toolchain executable and
  87% free memory. Every active function now receives a correlated empty/
  parameter-seeded reaching state rather than the old non-variadic fallback.
  Call-site bindings are distinct from whole-body assignment origins, and a
  callable slot's transported role survives replacement while its value
  origin strongly updates. Deferred/asynchronous callable-slice calls retain
  statement-time descriptors and scalar targets in scheduled state; direct
  shared-backing overrides are replayed at reachable deferred returns, while
  asynchronous observations include statement-time and each later state.
  Descriptor replacement remains disjoint. Primary defer/go and ordinary
  scalar/fixed-slice bites and their controls all use exact four-write or
  route-specific unresolved evidence. Static mutation/control-flow challenge
  and the skipped PIB-511/519 remainder remain pending before eligibility.
- S7 AR rev-20 static offset challenge RED — **2/2 intended bites** in 11.45s
  test / 11.840s package / 15.47s wall; preflight found no active Go
  toolchain executable and 86% free memory. A `copy` into `callbacks[1:]`
  after deferring `callbacks[:1]`, and an append beginning at the captured
  slice's length, both incorrectly returned unresolved
  `s7ARRev20Sink`/`callbacks[0]` evidence with `writes=0 counted=false`.
  Neither operation can change captured element zero. This independently
  proves that the first conservative mutation fold was backing-wide rather
  than offset/length sensitive; no fixture spelling, inventory count or
  generic error assertion is involved.
- S7 AR rev-20 static self-audit and final matrix — **37/37 PASS** in 67.14s
  test / 67.715s package / 70.88s wall with clean 87%-free-memory and
  no-toolchain preflights. The scheduled-call state is cloned/equality-checked
  with captured scalar origins, descriptor backing identities/offsets, exact
  origin overrides, uncertainty ranges and async/deferred mode. Direct/alias/
  reslice writes replay exactly; `copy` and append mutation ranges are
  offset/length sensitive; forced allocation, beyond-captured-length writes
  and read-only/disjoint helpers remain inert. Local helper and invoked
  closure writes are visible; inert closure declarations are not executed.
  Deferred mutator ordering is deliberately conservative for a shared
  captured backing, while async calls are observed before and after each
  reachable mutation, including RHS mutation before descriptor replacement.
  Ordinary scalar/fixed-slice flow additionally covers tuple RHS-before-LHS,
  shadowed objects, MethodVal/MethodExpr offsets and two-hop binding. State
  cap 64, loop limit 32 and unsupported labeled flow now install an explicit
  incomplete call state rather than dropping an alternative. During the
  preceding two-case offset GREEN, an external `go test` appeared between
  preflight and execution and overlapped that superseded 12.275s run; it was
  allowed to drain. The final 37-case run above started only after a fresh
  exact-executable preflight and had no overlap. Inherited repros and focused
  eligibility remain pending.
- S7 AR rev-20 inherited rev-14…20 repros — **PASS** in 266.873s package /
  267.33s wall after a static-first correction loop. The first inherited run
  exposed four genuine regressions plus one nil-cache panic: exact `copy`
  updates had been reduced to uncertainty, fixed `[]func()` direct invocation
  lost its exclusion, reslice writes became over-precise, scalar slot
  relevance was incorrectly conflated with variadic element dependency, and
  nested mutation analysis lacked initialized recursion maps. The correction
  stores exact copy origin overrides, keeps offset-sensitive disjoint copy/
  append controls, excludes direct fixed-slice element invocation while still
  tracking fixed slices forwarded to variadic sinks, conservatively marks
  derived reslice writes, separates persistent scalar-slot authority from
  input element demand, and initializes every mutation fixpoint. A
  semantic-object sequence equivalence check preserves all-path identical
  branch updates while branch-divergent invoked elements fail closed.
  Targeted rechecks and the complete rev-14…20 rerun pass. Fresh PIB-518,
  PIB-511 and PIB-519 timing eligibility now begins; no inherited timing is
  reused.
- S7 AR rev-20 fresh focused eligibility — **PASS**. Serial `-p=1 -count=1`
  package times are PIB-518 **97.603s** (98.39s wall), PIB-511 **175.146s**
  (175.62s wall) and PIB-519 **30.147s** (30.65s wall), totaling
  **302.896s**, below the immutable 325-second gate by 22.104s. Exact
  executable preflights found no competing Go toolchain workload and 79–80%
  free memory for every counted run. Several unrelated external Go workloads,
  including one broad suite, appeared before eligibility and were allowed to
  drain; no counted command overlapped. Rev-20 has earned exactly one
  unchanged observer execution under the frozen budgets.
- S7 AR rev-20 unchanged observer — **PASS** on its single authorized
  implementation execution. A fresh preflight found no active Go toolchain
  executable and 80% free memory. A new process group ran
  `go test -v -p=1 -count=1 -timeout=12m ./internal/cli -run
  '^TestS7ObservedARRegistrationAuthority$'` under the immutable external
  419-second cutoff; all 15 registered AR targets passed in 300.37s test /
  300.879s package / 301.350s outer monotonic wall. The cutoff did not fire,
  and the frozen 12m outer, 8m inner and 1m wrong-input budgets remain
  unchanged. Rev-20's observer allowance is consumed and must not be rerun.
- S7 AR rev-20 bounded regressions and final validation — **PASS**. The exact
  ledger/empty-target, observer wrong-input/budget, PIB-508/520, real
  PIB-506/507 divergence classes, real partial/divergence reports and three
  S4 abandon selectors pass in 7.558s package / 8.67s wall. Changed-file
  gofmt, no-index whitespace and repository `git diff --check` are clean.
  `go vet -p=1 ./internal/cli` passes in 0.74s; host
  `go build -p=1 -o /dev/null ./cmd/tpatch` in 0.35s; Linux and Darwin amd64
  CLI-test cross-compiles to `/dev/null` in 3.91s and 3.85s. Every preflight
  was serial, found no competing toolchain process and retained 76–78% free
  memory. Final guard SHA-256 is
  `125172b33728814009291ff9a044869a6c3afe8f0cdd9fb333c178e331669314`.
  Frozen runtime, renderer, exact 15-row `I6/C4/G4/S1` ledger,
  observer/registration and AVP hashes remain
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`.
  Side Research remains `b385fe622db9926f48861105239f113e`;
  HEAD/local/origin remain `c363ed7`; staging and workflow/assets diff remain
  empty. Only the two authorized files are newer than dispatch; parent-owned
  LOG, pre-existing frozen worktree entries and all root binaries retain their
  boundary state; no scratch, test-binary or cross-compile residue remains.
  No full/whole `^TestS7AR`, race, Windows, CI, staging, commit, push, tag,
  LOG edit or AS–AX work occurred.
- S7 AR rev-20 independent static re-review — **NEEDS REVISION**. The
  mandatory early-stop fired before any Go command or observer. Three Medium
  PIB-518 defects remain. First, a helper's scheduled goroutine state does
  not propagate back to its caller, so `launch(callbacks)` followed by an
  unknown caller-side write to the same backing can certify only the
  helper-local statement-time callback. Second, deferred or asynchronous
  zero-argument closures that capture and mutate a callable slice are not
  scheduled because relevance is derived only from explicit arguments.
  Third, all deferred mutations are applied before all deferred observations,
  ignoring LIFO and falsely rejecting the order where the sink executes
  before a later-executing mutator. The reviewer confirmed all 37 rev-20
  fixtures are substantive complete-validator sensitivities with no
  fixture-name special casing, but correctly left PIB-511/519, inherited
  backing remainder, caps and timings unexecuted after the static rejection.
  HEAD/local/origin remain `c363ed7`; staging, workflow/assets diff, frozen
  hashes, exact `I6/C4/G4/S1` ledger, observer budgets and Side Research
  remain intact. Guard SHA-256 remains
  `125172b33728814009291ff9a044869a6c3afe8f0cdd9fb333c178e331669314`.
- S7 AR rev-21 is dispatched to the existing implementation agent. Authorized
  scope remains only `internal/cli/prepare_s7_ar_guard_test.go` plus this
  handoff. Rev-21 must propagate asynchronous callable-backing escape across
  helper boundaries, or fail closed exactly when a caller can subsequently
  mutate the escaped backing; include scheduled zero-argument closure capture
  effects; and replay defers in reverse registration order by observing each
  call before applying its effects to calls that execute later. Test-first
  complete-validator coverage must bite caller-after-return mutation while
  preserving disjoint/no-later-mutation controls, bite deferred and
  asynchronous captured-closure mutation, and distinguish both deterministic
  defer orders. All 37 rev-20 sensitivities, frozen boundaries, caps and
  budgets remain unchanged. Rev-20's observer is consumed; rev-21 may earn
  exactly one unchanged observer only after fresh PIB-511/518/519 package
  time remains at or below 325 seconds.
- S7 AR rev-21 implementation started from exact guard SHA-256
  `125172b33728814009291ff9a044869a6c3afe8f0cdd9fb333c178e331669314`.
  Boundary preflight confirms HEAD/local/origin `c363ed7`, empty staging,
  every frozen hash, exact 15-row `I6/C4/G4/S1` ledger, Side Research
  `b385fe622db9926f48861105239f113e`, and an empty workflow/assets diff.
  No exact Go toolchain/test executable was active and memory was 85% free.
  Parent-owned LOG and every pre-existing dirty/untracked/root-artifact
  surface remain untouched. Complete-validator caller-escape,
  scheduled-closure and LIFO fixtures are being added before any analyzer
  helper changes.
- S7 AR rev-21 canonical complete-validator RED — **7 intended bites, 7
  inverse controls PASS** before any analyzer helper change. The 14-case
  matrix ran with `-p=1 -count=1` in 30.63s test / 31.154s package /
  34.31s wall after exact-process preflights with 85% free memory. Direct
  caller-after-helper shared-backing mutation, deferred/go zero-argument
  captured closure mutation and deferred explicit-slice-parameter mutation
  incorrectly returned the canonical inventory (duplicated
  `applyIntentArchivePurgeResult:direct-builder`) instead of route-specific
  unresolved evidence. The safe LIFO order where the sink executes before
  the mutator and its three-defer/noop inverse incorrectly returned
  unresolved `s7ARRev21Sink:callbacks[0]` with `writes=0 counted=false`.
  The initial two-hop fixture was already conservative for an unrelated
  fixed-slice forwarding limitation, so it was disclosed rather than counted;
  a refined variadic launch + variadic middle wrapper then reproduced the
  intended clean false negative in 10.32s test / 10.844s package / 14.14s
  wall. Passing pre-fix controls were no caller mutation, disjoint caller
  backing mutation, descriptor-only reassignment, statement-time unknown
  target not laundered by a later known write, inert closure declaration,
  captured-variable descriptor replacement onto a disjoint backing, and the
  order where the mutator executes before the sink. Every fixture parsed and
  type-checked through the typed production source set and complete validator;
  bites use exact four-write or route-specific unresolved assertions and
  reject inventory-count substitution. Starting guard SHA remains
  `125172b33728814009291ff9a044869a6c3afe8f0cdd9fb333c178e331669314`;
  only sensitivity source has changed.
- S7 AR rev-21 primary correction — **14/14 GREEN** in 30.56s test /
  31.092s package / 34.28s wall after clean exact-process preflights with 85%
  free memory. Callable state now carries offset-sensitive escaped-backing
  demand through actual/formal bindings and recursive wrappers; a later
  overlapping backing write installs explicit incomplete evidence while
  disjoint writes and descriptor replacement remain inert. Scheduled function
  literals retain explicit argument bindings separately from captured
  callable objects, so zero-argument closures refresh captured descriptors and
  scalars from execution state while explicit slice parameters keep their
  registration-time descriptor. Deferred calls are processed in reverse
  registration order: each call is observed against its current execution
  state before only its backing/capture mutation effects are transferred to
  older defers. Async closures retain both pre-effect and post-effect
  alternatives. Direct/two-hop caller escape, both closure forms, both defer
  orders and the three-defer sequence now have their substantive expected
  outcomes. Static alias/reslice/copy/append, terminal-state, call-site,
  go-mutator and cap/loop challenge begins before inherited eligibility.
- S7 AR rev-21 static self-challenge found and closed one sibling bite. The
  first 13-case run took 28.73s test / 29.255s package / 32.55s wall with 85%
  free memory: a helper that launched a zero-argument goroutine mutator on an
  already escaped backing incorrectly returned the canonical four-write
  inventory. Direct function literals were unreachable in mutation-summary
  mode because that mode has no origin catalog; direct literals are now
  resolved before the catalog fallback, so captured async mutation composes
  through the helper and overlaps the escaped backing. Two additional first-
  run failures were disclosed fixture confounds rather than bites: a
  variadic-parameter reslice control crossed an unrelated origin limitation,
  and an unused launch helper polluted an explicit-scalar control. Local
  composite backing and per-fixture helper sources removed those confounds.
  The expanded **20/20 static challenge matrix PASS** took 41.28s test /
  41.805s package / 45.43s wall after clean preflights with 86% free memory.
  It covers caller alias/reslice/copy/append reuse, disjoint offset copy,
  forced allocation, nested/two-hop escape, branch and explicit-return
  terminals, unreachable writes, both go registration orders, captured versus
  explicit scalar parameters, nested helper/invoked/inert closures, all-
  terminal descriptor replacement, sequential LIFO, and deterministic
  fail-closed behavior at the immutable 32-iteration loop and 64-state caps.
  No path, fixture, function-name, source-text or inventory-count special case
  remains. Inherited rev-14…21 repro validation is now eligible to begin.
- S7 AR rev-21 inherited rev-14…21 repros — **PASS** in 393.151s package /
  393.60s wall after one correction loop. The first inherited run exposed
  three rev-20 go regressions: explicit non-expanded variadic callback
  arguments were treated as caller-owned slice backings, so a helper's async
  escape installed incomplete evidence even though Go allocates the variadic
  slice for that call. This falsely rejected an unmutated go call,
  descriptor-only replacement and the RHS-mutation route. Cross-function
  escape propagation now applies to fixed sequence arguments and expanded
  variadic slices only; explicit scalar variadic arguments retain rev-20
  value/temporary-slice semantics. The three focused regressions pass in
  13.68s test / 14.210s package / 17.29s wall, and the complete inherited
  selector subsequently passes after a clean 79%-free-memory preflight.
  No whole `^TestS7AR` selector or consumed observer ran. Fresh PIB-518,
  PIB-511 and PIB-519 timing eligibility begins now; inherited timing is not
  reused.
- S7 AR rev-21 fresh focused eligibility — **PASS**. Serial
  `-p=1 -count=1` package times are PIB-518 **99.357s** (100.23s wall),
  PIB-511 **170.233s** (171.19s wall) and PIB-519 **30.326s** (31.41s wall),
  totaling **299.916s**, below the immutable 325-second gate by 25.084s.
  Exact-process preflights retained 72–81% free memory. Two unrelated external
  Go test workloads appeared before PIB-518/511; each attempted command
  stopped at preflight without starting this repository's test, and execution
  waited until the external workload drained. No counted eligibility command
  overlapped. Rev-21 has earned exactly one unchanged observer execution
  under the frozen 12m/8m/1m budgets and 419-second external cutoff; rev-19
  and rev-20 observers remain consumed and were not rerun.
- S7 AR rev-21 unchanged observer — **PASS** on its single authorized
  execution. A fresh preflight found no active exact Go toolchain/test
  executable and 72% free memory. A new process group ran
  `go test -v -p=1 -count=1 -timeout=12m ./internal/cli -run
  '^TestS7ObservedARRegistrationAuthority$'` under an external monotonic
  419-second cutoff. All 15 registered AR targets passed in 325.70s test /
  326.141s package / 326.572s outer wall; the cutoff remained clear. The
  frozen 12m outer, 8m inner and 1m wrong-input budgets are unchanged.
  Rev-21's observer allowance is consumed and must not be rerun.
- S7 AR rev-21 late static challenge found one additional material PIB-518
  gap **after** the observer had been consumed. A helper that launched a
  zero-argument goroutine closure which captured a callable slice and invoked
  the sink from inside that closure did not transport the async invocation
  demand to its caller; caller mutation therefore returned the canonical
  inventory in 10.04s test / 10.558s package / 13.88s wall. Async escape
  summary now executes a direct scheduled literal against its bound/captured
  reaching snapshot and merges the closure body's invocation demand before
  transporting the parameter effect. The bite and no-mutation/descriptor-
  reassignment inverses pass together in 12.89s test / 13.354s package /
  16.56s wall with 79% free memory. This is a material change to the PIB-518
  guard target observed by `TestS7ObservedARRegistrationAuthority`; the prior
  15-target observer result is therefore superseded for final-code evidence.
  Rev-21's one observer execution remains consumed and the explicit no-rerun
  rule forbids obtaining a fresh final-code observer. The implementation will
  complete bounded post-fix static/build validation, but cannot transition to
  `AWAITING REVIEW` under the current dispatch.
- S7 AR rev-21 is procedurally **BLOCKED** because its final material guard
  change superseded its consumed observer evidence. The supervisor dispatched
  rev-22 as a final-code validation-only revision from guard SHA-256
  `e7ce411cc2d9241774597a830eef977b00c42e533cbf4ce390f29afa46091b3c`.
  The guard and every other code/test boundary are frozen: rev-22 may update
  only this handoff. It must finish bounded post-fix regressions, then obtain
  fresh serial PIB-511/518/519 timing totaling at most 325 seconds before one
  unchanged observer execution under the frozen 12m/8m/1m budgets and
  419-second cutoff. Any code change blocks rev-22 immediately and requires a
  new revision; rev-19, rev-20 and rev-21 observers remain consumed.
- S7 AR rev-22 final-code validation started without changing the frozen
  guard. HEAD/local/origin are exactly
  `c363ed7c1e1b43db2421c4c02bbd3c0e5d6b1430`, staging is empty, workflow and
  assets have an empty diff, and the frozen guard, runtime, renderer, ledger,
  observer/registration and AVP SHA-256 values are respectively
  `e7ce411cc2d9241774597a830eef977b00c42e533cbf4ce390f29afa46091b3c`,
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`.
  The exact ledger remains 15 rows `I6/C4/G4/S1`; the 64-state and 32-loop
  caps are frozen. The first exact-process check found an unrelated external
  `go`/`compile` workload while memory remained 83% free, so no rev-22 Go
  command started and validation is waiting for that workload to drain.
- S7 AR rev-22 late captured-sink final-code gate — **PASS**. After two
  unrelated external Go workloads drained, a clean exact-process preflight
  retained 83% free memory. The unchanged command
  `go test -v -p=1 -count=1 ./internal/cli -run
  '^TestS7ARRev21StaticChallenges$/(caller-go-captured-sink-helper-shares-escaped-backing|caller-go-captured-sink-helper-without-mutation|caller-go-captured-sink-helper-descriptor-reassignment)$'`
  passed all three complete-validator cases in 13.42s test / 13.933s package /
  14.86s wall. The shared-backing caller mutation remains unresolved, while
  no mutation and descriptor-only reassignment retain exact authority. The
  guard remains byte-frozen; complete rev-21 and inherited matrices begin
  next.
- S7 AR rev-22 complete rev-21 matrices — **PASS** unchanged. A clean
  exact-process preflight retained 83% free memory. The exact selector
  `^(TestS7ARRev21ReviewerRepros|TestS7ARRev21StaticChallenges)$` passed all
  14 canonical cases and all 23 static challenges in 74.509s package /
  74.94s wall (32.51s and 41.58s test bodies). The matrix continues to cover
  cross-function async escape, captured scheduled closures, both defer
  orders, three-defer LIFO, alias/reslice/copy/append, branch/return flow and
  deterministic 64-state/32-loop fail-closed behavior. The inherited exact
  rev-14…21 selector then passed from a clean 76%-free preflight after an
  external workload had drained; its verbose output exceeded the terminal
  capture limit, so a concise unchanged rerun will record exact package/wall
  timing before proceeding to non-observer regressions. No whole
  `^TestS7AR` selector or observer ran.
- S7 AR rev-22 inherited and non-observer regressions — **PASS**. After
  continuously arriving external Go workloads drained, the concise unchanged
  exact rev-14…21 repro selector passed in 371.906s package / 372.78s wall
  from a clean 78%-free preflight. The established non-observer set then
  passed in 18.934s package / 20.01s wall from a clean 75%-free preflight:
  exact ledger/empty-target; wrong-input, correlation and frozen budget
  guards; PIB-508/520; every real PIB-506/507 divergence class; PIB-509…517
  abandon/control contracts; real partial, completion-only, orphan and
  divergence reports; and all three S4 abandon regressions. The ledger
  resolver still proves exactly 15 rows `I6/C4/G4/S1`, the wrong-input suite
  pins AR's 12m/8m/1m tuple and PIB-506/507 exercise regular, symlink,
  directory, FIFO and device-seam routes. No observer ran. Read-only
  formatting, vet and build validation begins next.
- S7 AR rev-22 read-only format/build gate — **PASS**. `gofmt -l` reports
  no frozen-guard change, the expected status-1 no-index whitespace scan emits
  no warning, and repository `git diff --check` is clean. From separate clean
  exact-process preflights with 78–79% free memory,
  `go vet -p=1 ./internal/cli` passed in 0.80s wall, host
  `go build -p=1 -o /dev/null ./cmd/tpatch` in 0.54s, and
  `CGO_ENABLED=0 GOOS={linux,darwin} GOARCH=amd64 go test -c -p=1 -o
  /dev/null ./internal/cli` passed in 4.59s and 4.63s. Null-device outputs
  created no build artifact. The frozen guard needs no source edit, so fresh
  final-code PIB-518/511/519 timing eligibility begins.
- S7 AR rev-22 fresh final-code timing eligibility — **PASS**. Serial
  `-p=1 -count=1` runs from clean exact-process preflights passed at PIB-518
  **102.30s test / 102.880s package / 103.41s wall**, PIB-511 **168.43s /
  168.967s / 169.75s**, and PIB-519 **29.61s / 29.966s / 30.46s**. The exact
  package-time sum is **301.813s**, 23.187s below the immutable 325-second
  gate. Preflights retained 79–82% free memory. One unrelated Go workload
  appeared before the first PIB-519 attempt; the exact-process guard stopped
  before this repository's test began, execution waited for it to drain, and
  only the subsequent clean run is counted. The guard remains unchanged, so
  rev-22 has earned exactly one final-code observer execution under the frozen
  12m outer / 8m inner / 1m wrong-input budgets and 419-second external
  cutoff. Rev-19, rev-20 and rev-21 observer evidence remains consumed and is
  not relabeled.
- S7 AR rev-22 unchanged final-code observer — **PASS** on its sole
  authorized execution. A 60-second quiet-window check and immediate
  exact-process preflight found no competing Go/compiler/linker/vet/test
  executable and 82% free memory. A new process group ran
  `go test -v -p=1 -count=1 -timeout=12m ./internal/cli -run
  '^TestS7ObservedARRegistrationAuthority$'` with a monotonic 419-second
  external cutoff. All 15 registered AR targets passed in **327.17s test /
  327.672s package / 328.311s outer monotonic wall**; the cutoff remained
  clear. The frozen 12m/8m/1m budgets are unchanged. Rev-22's observer
  allowance is now consumed and will not be rerun; final validation is
  restricted to read-only non-Go boundary checks.
- S7 AR rev-22 final-code validation is complete and **AWAITING REVIEW**.
  Read-only final checks confirm HEAD/local/origin
  `c363ed7c1e1b43db2421c4c02bbd3c0e5d6b1430`, empty staging, empty
  workflow/assets diff and clean `git diff --check`. Final guard, runtime,
  renderer, ledger, observer/registration and AVP SHA-256 values remain
  `e7ce411cc2d9241774597a830eef977b00c42e533cbf4ce390f29afa46091b3c`,
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`.
  Side Research remains `b385fe622db9926f48861105239f113e`.
  The known root `tpatch`, `cli.test`, `tpatch.exe`, `cli.test.exe` and
  `gitutil.test.exe` artifacts retain their pre-existing boundary state;
  null-device builds created no new residue. One unrelated Go/test workload
  appeared during the first final non-Go process check after the observer and
  drained before close; the terminal exact-process check is clear with 77%
  free memory. Rev-22 wrote only this handoff; the guard and all other
  code/test/docs paths remained frozen. No full or whole `^TestS7AR` suite,
  race suite, Windows execution, CI, staging, commit, push, tag, LOG edit or
  AS–AX work occurred.
- S7 AR rev-22 independent static re-review — **NEEDS REVISION**. The reviewer
  confirmed all refs, frozen hashes, Side Research, empty staging, exact
  15-row ledger and rev-22 evidence, then found one Medium PIB-518 gap at
  `internal/cli/prepare_s7_ar_guard_test.go:11333-11416`. Scheduled
  zero-argument closure captures are discovered only when `call.Fun` is a
  direct function literal. A scalar alias such as
  `mutate := func() { callbacks[0] = replacement }; defer mutate()` is not
  scheduled, so LIFO execution-time backing effects are lost; the analogous
  aliased `go launch()` route also loses captured-sink async demand. Mandatory
  static early-stop fired before any Go or observer command. The worktree
  remained unchanged.
- S7 AR rev-23 is dispatched as a correction and challenge revision before
  observer authorization. It must add canonical defer/go alias bites and
  registration-time value-capture, reassignment, inert and disjoint controls;
  resolve literal targets from the reaching state at scheduling time; and fail
  closed on ambiguous or unresolved targets. It may run serial `-p=1`
  targeted and inherited non-observer validation after exact-process and
  memory preflights, but no observer, focused eligibility trilogy, full/race/
  Windows suite, CI, staging, commit, push or tag. After a static
  self-challenge it must freeze the guard hash and stop for independent
  pre-observer review.
- S7 AR rev-23 canonical-RED phase started from the exact accepted boundary.
  HEAD/local/origin are
  `c363ed7c1e1b43db2421c4c02bbd3c0e5d6b1430`, staging is empty, workflow/
  assets have an empty diff, and the guard baseline is
  `e7ce411cc2d9241774597a830eef977b00c42e533cbf4ce390f29afa46091b3c`.
  Runtime, renderer, ledger, observer/registration and AVP hashes remain
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`;
  Side Research remains `b385fe622db9926f48861105239f113e`.
  The exact-process preflight found no `go`, `compile`, `link`, `vet` or
  `cli.test` executable and 83% free memory. Thirteen typed complete-validator
  cases are added before analyzer edits: two reviewer bites, registration-time
  target reassignment in both directions, pre-registration reassignment,
  unresolved target fail-closed, inert/no-mutation/descriptor/disjoint
  controls, captured-callable by-reference behavior and async inverses.
  Only the guard and this handoff are writable; the next command is the narrow
  canonical RED selector.
- S7 AR rev-23 canonical RED is proven before analyzer edits. From a clean
  exact-process preflight with 84% free memory,
  `go test -v -p=1 -count=1 ./internal/cli -run
  '^TestS7ARRev23ScheduledClosureAliasRepros$'` produced the intended
  **6 RED / 7 inverse-control PASS** split in 33.00s test / 33.552s package /
  37.80s wall. Deferred alias mutation, async aliased captured sink,
  unsafe→safe post-registration reassignment, pre-registration unsafe
  reassignment, unresolved parameter alias and captured free-callable
  reassignment all falsely retained the ordinary four-write inventory instead
  of route-specific unresolved evidence. Safe→unsafe post-registration
  reassignment, inert declaration, no mutation, descriptor-only and disjoint
  backing, async no-mutation and async descriptor-reassignment controls all
  retained exact authority. This establishes both reviewer bites and the
  registration-vs-execution inverses against starting guard
  `e7ce411cc2d9241774597a830eef977b00c42e533cbf4ce390f29afa46091b3c`;
  the bounded analyzer correction begins next.
- S7 AR rev-23 bounded analyzer correction is implemented and formatted.
  `s7ARScheduledCallableLiteralTargets` now resolves each scheduled scalar
  alias from that individual registration-time reaching state, sorts its
  literal alternatives deterministically and marks absent/unresolved
  function-valued targets incomplete. `s7ARScheduleCallableCall` snapshots
  that state, unions only the resolved targets' true free callable/slice
  captures, preserves explicit argument evaluation, and keeps a resolved
  capture-free inert target complete. Execution-time state is merged only for
  the captured objects/backings, so later assignment to the scheduled
  function variable cannot replace its registered target while later
  assignment to a true free capture remains visible by reference. Async
  escape analysis now walks the same resolved alias literal bodies used by
  direct literals. No caps, ledger, production path or other file changed;
  the canonical matrix is the next gate.
- S7 AR rev-23 canonical matrix is now **GREEN** after two bounded sibling
  corrections. The first run exposed over-broad treatment of ordinary
  function-valued results (`autoDetectProvider:cancel` and
  `generatePrepareBundle:totalCancel`); alias scheduling is now restricted to
  reaching states that carry a tracked callable scalar slot, while an
  unreliable tracked slot still fails closed. The second run proved that an
  async closure's captured slice descriptor is itself read by reference:
  post-registration descriptor replacement can change the sink backing and
  is correctly unresolved, unlike post-registration reassignment of the
  scheduled function variable, which remains value-captured and exact.
  From a clean 85%-free preflight, all **14** canonical bites/inverses passed
  in 32.22s test / 32.754s package / 36.41s wall. The implementation now
  stores the deterministically resolved literal targets with each scheduled
  call and replays async target bodies against each current captured state,
  while defer mutation remains sequential LIFO. The bounded rev-23 static
  challenge is next.
- S7 AR rev-23 static self-challenge found and closed one material sibling.
  The initial 12-case run passed parenthesized aliases, branch-joined targets,
  alias chains, nested alias invocation, registration-time reassignment,
  separate call-site snapshots, async aliases and direct named/inert controls,
  but correctly exposed that a scheduled closure capturing a function
  variable did not transport the later-selected closure target's backing
  captures to an older defer. `s7ARCallableScheduledCaptureClosure` now takes
  the transitive closure of tracked literal aliases, and execution fails
  closed when a captured callable slot becomes unreliable. This preserves a
  later safe captured target as exact and transports a later unsafe target's
  callback backing mutation. After unrelated external Go workloads drained,
  the post-fix challenge passed all **12/12** cases from a clean 85%-free
  preflight in 27.86s test / 28.429s package / 31.67s wall. This was the final
  analyzer edit; complete rev-21/rev-23 matrices and inherited non-observer
  regressions begin next.
- S7 AR rev-23 final matrices and inherited repros are **PASS**. After an
  unrelated external workload drained, the exact four-test rev-21/rev-23
  selector passed all 14 rev-21 canonical, 23 rev-21 challenge, 14 rev-23
  canonical and 12 rev-23 challenge cases in 111.665s package / 112.21s wall
  from a clean 85%-free preflight. The concise exact rev-14…23 repro selector
  then passed in 601.285s package / 602.35s wall from a clean preflight with
  74% free memory. It names only the eleven reviewer/challenge test functions;
  no whole `^TestS7AR`, full suite, race, Windows execution, timing trilogy or
  observer ran. Focused ledger/runtime/report/PIB-511/519 non-observer
  regressions begin next.
- S7 AR rev-23 focused non-observer regressions are **PASS**. From a clean
  82%-free preflight, the exact ledger/empty-target, observer wrong-input and
  hosted-budget guards, PIB-508/520, real PIB-506/507 divergence, PIB-509…517
  abandon/control, real partial/completion/divergence reports and all three S4
  abandon regressions passed in 17.366s package / 18.46s wall. Separate clean
  preflights then retained 82–83% free memory while the exact PIB-511 selector
  passed in 154.374s package / 154.80s wall and the exact PIB-519 selector
  passed in 30.079s / 30.54s. These are validation runs, not the forbidden
  PIB-511/518/519 eligibility trilogy; PIB-518 was not rerun as a standalone
  timing target. Formatting, vet and host/cross-build validation begins next.
- S7 AR rev-23 formatting and build validation is **PASS**. Read-only
  `gofmt -l internal/cli/prepare_s7_ar_guard_test.go`, changed-path and
  repository `git diff --check` are clean. From separate clean exact-process
  preflights with 84–85% free memory, `go vet -p=1 ./internal/cli` passed in
  0.81s wall, `go build -p=1 -o /dev/null ./cmd/tpatch` in 0.41s, and
  `CGO_ENABLED=0 GOOS={linux,darwin} GOARCH=amd64 go test -c -p=1 -o
  /dev/null ./internal/cli` in 3.90s and 3.71s. The null-device builds created
  no new artifact. Final frozen-boundary verification and handoff transition
  remain; no further Go command is planned.
- S7 AR rev-23 is frozen and **AWAITING PRE-OBSERVER STATIC REVIEW** at guard
  SHA-256
  `62063887dc7eab05bc08a72aeb774ddcc3d815a64ed69c601e46251556d2f2d2`.
  The final correction resolves only reaching-state tracked scalar aliases,
  snapshots their registration-time literal target(s), preserves later
  execution-time state only for true/transitive captures and shared backings,
  and fails closed for unreliable tracked aliases. Thus `defer/go f()` keeps
  the function value registered at the statement, while a free callable,
  scalar or slice descriptor referenced by that closure remains by-reference
  at execution. Async alias targets are replayed against each later captured
  state; deferred targets retain sequential LIFO observation then mutation.
  Ordinary untracked function-valued results such as cancellation functions
  are intentionally outside this alias-closure route and remain unchanged.

  Final sensitive suites are
  `TestS7ARRev23ScheduledClosureAliasRepros` (**14**) and
  `TestS7ARRev23StaticChallenges` (**12**). The former covers both reviewer
  bites, target reassignment before/after registration, fail-closed
  ambiguity, inert/no-mutation/descriptor/disjoint controls and free-capture
  by-reference semantics. The latter covers parenthesized aliases,
  branch-joined alternatives, multi-hop/nested aliases, transitive captured
  callables, separate call snapshots, async backing alternatives and direct
  named/inert controls. All final runs pass, including exact rev-21/rev-23
  matrices and the exact rev-14…23 inherited selector.

  Final read-only boundary verification confirms HEAD/local/origin
  `c363ed7c1e1b43db2421c4c02bbd3c0e5d6b1430`, empty staging, empty workflow/
  assets diff, clean whitespace, exact 15-row `I6/C4/G4/S1` ledger, immutable
  64-state/32-loop caps and 85% free memory with no exact Go/compiler/linker/
  vet/CLI-test process. Runtime, renderer, ledger, observer/registration and
  AVP hashes remain
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`;
  Side Research remains `b385fe622db9926f48861105239f113e`.
  Rev-23 changed only the guard and this handoff. No observer, eligibility
  trilogy, full or whole `^TestS7AR` suite, race, Windows execution, CI,
  staging, commit, push or tag occurred.
- S7 AR rev-23 independent pre-observer static review — **NEEDS REVISION**.
  Exact refs, staging, frozen hashes, Side Research and empty workflow/assets
  diffs passed. The reviewer found one Medium PIB-518 role-collision defect:
  when a registered closure references its own scalar function variable, that
  object is both the registration-time call target and a genuine
  execution-time free capture. Capture refresh correctly updates the value
  seen inside the body, but deferred replay then re-resolves
  `scheduled.call.Fun` from the refreshed slot instead of executing the stored
  `scheduled.literals`. An unsafe closure registered before a safe
  reassignment can therefore be analyzed as safe; the reverse direction can
  be falsely rejected. Existing target-reassignment fixtures do not make the
  target a free capture. Mandatory static early-stop fired; no Go command,
  observer, timing run, build or worktree edit occurred.
- S7 AR rev-24 is dispatched as a correction and challenge revision before
  observer authorization. It may modify only
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. It must prove a
  canonical self-capturing unsafe-to-safe RED bite plus safe-to-unsafe inverse,
  then execute stored registration-time literal targets without re-resolving
  the outer scheduled call while preserving execution-time state for free
  variables used inside those bodies. Both deferred and asynchronous replay,
  LIFO, explicit-argument capture, target ambiguity, named/inert controls and
  separate call snapshots require challenge coverage. Serial `-p=1` targeted
  and inherited non-observer validation is allowed after exact-process and
  memory preflights. No observer, focused eligibility trilogy, full/race/
  Windows suite, CI, staging, commit, push or tag is authorized. The final
  guard must be hashed and frozen for a fresh independent static review.
- S7 AR rev-24 canonical RED is proven before analyzer edits against rejected
  guard SHA-256
  `62063887dc7eab05bc08a72aeb774ddcc3d815a64ed69c601e46251556d2f2d2`.
  From a clean exact-process preflight with 86% free memory,
  `go test -v -p=1 -count=1 ./internal/cli -run
  '^TestS7ARRev24ScheduledSelfCaptureRepros$'` failed exactly **2/2** cases in
  11.496s package / 14.87s wall. The self-capturing registered unsafe→safe
  bite incorrectly retained the four canonical writes, while the
  self-capturing registered safe→unsafe precision inverse incorrectly
  reported unresolved callable origins at `s7ARRev24Sink`. Both fixtures
  type-check and run the complete PIB-518 validator; route-specific unresolved
  assertions reject inventory/count-only satisfaction. The bounded analyzer
  correction begins next; no observer or other Go command ran in this phase.
- S7 AR rev-24 bounded target/capture-role correction is implemented and
  formatted. `s7ARApplyScheduledCallableCallMutation` now executes every
  nonempty stored registration-time literal set directly and reserves
  `scheduled.call` re-resolution for named/helper/sink calls with no stored
  literal. Refreshed captures/backings still form each literal body's
  execution state, and registration-time parameter bindings remain in the
  statement snapshot. `s7ARApplyDeferredScheduledCallableMutations` preserves
  sequential LIFO, observes unnamed calls before mutation, propagates each
  body effect to older defers and executes nested scheduled calls. Async
  publication plus immediate/nondeterministic mutation alternatives use the
  same stored-target path while retaining the unchanged alternative; existing
  async escape discovery already walked the stored literal body directly.
  Recursive scheduled literal replay now fails closed. The two-case canonical
  GREEN gate is next; caps, ledger and every frozen file remain untouched.
- S7 AR rev-24 canonical matrix is **GREEN**. From a clean exact-process
  preflight with 86% free memory, the unchanged exact selector
  `go test -v -p=1 -count=1 ./internal/cli -run
  '^TestS7ARRev24ScheduledSelfCaptureRepros$'` passed **2/2** in 11.289s
  package / 14.66s wall. Registered self-capturing unsafe→safe now reports
  route-specific unresolved authority at the older sink, and registered
  self-capturing safe→unsafe retains exactly four canonical writes. The
  bounded rev-24 static challenge matrix begins next.
- S7 AR rev-24 static self-challenge is **GREEN** with no sibling correction
  required. The first 16-case matrix passed, then the audit was expanded
  before inherited validation with four explicit nested scheduled-call
  bite/inverse cases. From a fresh clean exact-process preflight with 86% free
  memory, the final exact selector `go test -v -p=1 -count=1 ./internal/cli
  -run '^TestS7ARRev24StaticChallenges$'` passed **20/20** in 39.510s package /
  42.68s wall. It covers parenthesized multi-hop self-capture; deferred and
  async unsafe→safe/safe→unsafe; refreshed inner callable invocation; three-
  defer LIFO; nested defer and nested go replay; registration-frozen explicit
  parameters versus refreshed free captures; branch-joined and mixed targets;
  separate call-site snapshots; and direct/named/inert, descriptor-only and
  disjoint-backing precision controls. No analyzer edit followed this final
  challenge. Exact rev-21/rev-23/rev-24 matrices begin next.
- S7 AR rev-24 inherited validation found one precision regression and the
  final analyzer is not yet frozen. After unrelated exact Go/CLI-test
  processes drained, the six exact rev-21/rev-23/rev-24 matrix functions
  passed in 149.425s package / 149.87s wall from an 86%-free preflight. From a
  fresh clean 80%-free preflight, the 13-function exact rev-14…24 selector
  failed only rev-20 subtest
  `defer-explicit-scalar-is-value-captured`: the registration-time canonical
  scalar callback was reported unresolved at `s7ARRev20DeferScalar:callback`
  instead of retaining four writes. The run used `-p=1 -count=1 -timeout 15m`
  and stopped at 688.672s package / 689.48s wall. This is a substantive
  explicit-argument/value-capture precision regression, not an observer or
  eligibility run. Its narrow selector and the target/capture replay path are
  being audited before any further inherited gate.
- S7 AR rev-24 inherited precision regression is corrected. Stored literals
  must be *executed* from refreshed capture/backing state, but the scheduled
  invocation itself still contributes callable-demand evidence from its
  registration snapshot. `s7ARObserveScheduledCallableInvocation` now observes
  nonliteral named/helper/sink calls against execution state and stored-
  literal outer calls against the frozen statement state; mutation continues
  to execute stored literals directly. This restores explicit scalar value
  capture without permitting execution-time target reselection. From a clean
  83%-free exact-process preflight, the exact failing subtest passed in 9.690s
  package / 12.73s wall. All prior post-edit evidence is superseded; rev-24
  canonical/challenge and inherited matrices will be rerun against this final
  correction before freezing.
- S7 AR rev-24 post-final-edit matrices and inherited repros are **PASS**.
  From clean exact-process preflights, the 2 canonical + 20 static-challenge
  cases passed together in 43.467s package / 43.89s wall at 84% free memory;
  the six exact rev-21/rev-23/rev-24 matrix functions passed in 141.673s /
  142.08s at 85%; and the 13-function exact rev-14…24 reviewer/challenge
  selector passed with `-timeout 15m` in 669.467s / 670.63s at 81%. No whole
  `^TestS7AR`, full suite, observer, eligibility trilogy, race or Windows
  execution ran. Focused non-observer ledger/runtime/report/PIB-511/519 gates
  begin next.
- S7 AR rev-24 focused non-observer regressions are **PASS**. From clean
  exact-process preflights at 83–85% free memory, the exact ledger/empty-
  target, PIB-508/520, real PIB-506/507 divergence, PIB-509…517 runtime/
  archive-control, PIB-518 partial/completion/divergence report and three S4
  abandon tests passed together in 96.390s package / 97.54s wall. The exact
  PIB-511 selector then passed in 165.888s / 166.33s, and PIB-519 in 31.674s /
  32.18s. PIB-518 was not run as a standalone timing target, so this is not
  the forbidden PIB-511/518/519 eligibility trilogy. No observer or observer
  wrong-input target ran. Read-only whitespace plus vet/build/cross-build
  validation begins next.
- S7 AR rev-24 formatting/build validation is **PASS**. The changed guard is
  `gofmt`-clean, temporary diagnostic strings are absent, and changed-path plus
  repository `git diff --check` are clean. From separate exact-process
  preflights with 85% free memory, `go vet -p=1 ./internal/cli` passed in
  0.74s wall, `go build -p=1 -o /dev/null ./cmd/tpatch` in 0.42s, and
  `CGO_ENABLED=0 GOOS={linux,darwin} GOARCH=amd64 go test -c -p=1 -o
  /dev/null ./internal/cli` in 4.36s and 3.92s. Final hash/boundary
  verification and the pre-observer review transition remain; no further Go
  command is planned.
- S7 AR rev-24 is frozen and **AWAITING PRE-OBSERVER STATIC REVIEW** at guard
  SHA-256
  `e1bff6763c826949a74971a81a9ad1ac784610b11f2e546820d5c3755051bd44`.
  The final implementation structurally separates the registered outer
  literal set from refreshed execution captures: stored targets execute
  directly, true/transitive free captures and shared backings refresh inside
  their bodies, explicit parameters remain registration-bound, and the outer
  invocation is observed from its registration snapshot. Named/helper/sink
  calls without stored literals retain execution-state resolution. Deferred
  mutation remains sequential LIFO; async publication/immediate alternatives,
  nested defer/go replay and the unchanged async alternative remain covered.
  Unsupported recursion and unreliable tracked targets fail closed.

  Canonical RED was exactly **2/2 failures** on the rejected rev-23 guard;
  final canonical is **2/2 PASS** and the rev-24 challenge is **20/20 PASS**.
  The challenge covers deferred/async target-role collision, multi-hop and
  parenthesized aliases, refreshed inner calls, nested scheduled calls,
  explicit-parameter versus free-capture semantics, branch/mixed targets,
  separate call sites and precision controls. Inherited validation exposed
  and closed one precision sibling: stored outer invocations also require
  registration-state callable-demand observation. After that correction, all
  exact rev-21/rev-23/rev-24 matrices, the 13-function rev-14…24 selector,
  focused ledger/runtime/report/S4 regressions, PIB-511 and PIB-519, vet, host
  build and Linux/Darwin amd64 CLI-test cross-builds passed.

  Final boundary verification confirms HEAD/local/origin
  `c363ed7c1e1b43db2421c4c02bbd3c0e5d6b1430`, empty staging, empty workflow/
  assets diff, clean whitespace, exact 15-row `I6/C4/G4/S1` ledger, immutable
  64-state/32-loop caps and 85% free memory with no exact Go/compiler/linker/
  vet/CLI-test process. Runtime, renderer, ledger, observer/registration and
  AVP hashes remain
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`;
  Side Research remains `b385fe622db9926f48861105239f113e`.
  Rev-24 changed only the guard and this handoff. No observer, observer wrong-
  input target, standalone PIB-518 eligibility timing, eligibility trilogy,
  broad/full/race/Windows suite, CI, staging, commit, push or tag occurred.
- Independent S7 AR rev-24 static review returned **NEEDS REVISION** before
  observer authorization. Direct stored-literal execution at
  `s7ARApplyScheduledCallableCallMutation` consumes only `flow.next`, while an
  explicit `return` publishes nested schedules and yields an empty flow. A
  stored closure can therefore mutate the tracked callback, return, and lose
  its terminal state; outer LIFO replay then has no state from which to analyze
  the older sink. The rev-21 explicit-return challenge returns from the owning
  function and does not cover this stored-literal boundary. The frozen rev-24
  hash is rejected. No review command, observer, timing run, build or edit
  occurred.
- S7 AR rev-25 is dispatched as a correction and challenge revision before
  observer authorization. It may modify only
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. It must first
  prove deferred and asynchronous stored-literal explicit-return unsafe bites
  and safe inverses, then preserve terminal states separately from natural
  fallthrough through compound control flow. Nested deferred mutations must be
  applied exactly once before terminal and natural completion states are
  combined for outer effect transfer. Registration-time target/argument
  snapshots, execution-time captures/backings, outer defer LIFO, unchanged
  async alternatives, fail-closed behavior, the 64-state/32-loop caps and all
  closed PIB-511/519 behavior remain frozen. No observer, eligibility trilogy,
  full/race/Windows suite, CI, staging, commit, push or tag is authorized.
- S7 AR rev-25 canonical-RED phase started from rejected guard SHA-256
  `e1bff6763c826949a74971a81a9ad1ac784610b11f2e546820d5c3755051bd44`.
  Baseline refs are HEAD/local/origin
  `c363ed7c1e1b43db2421c4c02bbd3c0e5d6b1430`, staging and workflow/assets
  diffs are empty, the exact Go/compiler/linker/vet/CLI-test process set is
  empty, and memory is 85% free. A typed six-case complete-validator canonical
  matrix has been added without changing analyzer logic: deferred and async
  stored-literal explicit-return bites, a nested-defer-before-outer-transfer
  bite, and exact-four-write safe inverses for each route. The exact new
  selector will now be run alone to capture mandatory RED behavior.
- S7 AR rev-25 canonical RED is proven before analyzer edits. The exact
  selector
  `^TestS7ARRev25StoredLiteralReturnRepros$` ran serially with `-p=1`,
  `-count=1` and `-timeout=5m` after a clean exact-process preflight with 85%
  free memory. It produced the required **3/3 unsafe RED bites** at
  `s7ARRev25Sink` (deferred explicit return, async explicit return, and nested
  defer followed by explicit return) while all **3/3 exact-four-write safe
  inverses passed**. Package time was 18.217s and wall time 21.31s. Each bite
  failed through the complete validator's callable-origin route rather than
  an inventory-count proxy. The bounded terminal-flow correction begins now;
  no observer or prohibited command has run.
- S7 AR rev-25 bounded terminal-flow correction is implemented. A dedicated
  `returns` component now carries explicit-return states separately from
  natural `next`, `breaks` and `continues`. Blocks and every analyzed compound
  route (if/else, for, range, switch, type-switch and select clauses) propagate
  return terminals outward without feeding them through loop back-edges or
  later statements. Return expressions are evaluated and async effects
  published before terminal capture. At function/stored-literal completion,
  natural and returned states each execute their own pending defers LIFO once,
  then merge for outer effect transfer. Stored-literal, direct-literal,
  mutation-demand, async-escape and root-reaching-state consumers now use the
  common completion path. Registration snapshots, execution-time captures,
  explicit arguments, outer schedules, async unchanged alternatives and the
  immutable caps are unchanged. The exact six-case canonical selector is the
  next gate.
- S7 AR rev-25 canonical matrix is **GREEN** after the terminal-flow
  correction. An unrelated exact `go`/`cli.test` workload appeared at the
  first preflight, so no rev-25 Go command started; the implementation waited
  for PIDs 77738/77755 to drain without killing or overlapping them. A fresh
  preflight then found an empty exact-process set and 86% free memory. The
  exact `^TestS7ARRev25StoredLiteralReturnRepros$` selector passed all **6/6**
  cases with `-p=1 -count=1 -timeout=5m` in 19.512s package / 22.83s wall.
  Deferred, async and nested-defer explicit-return bites now reach
  `s7ARRev25Sink`; all safe inverses retain exactly four canonical writes.
  Bounded rev-25 compound-flow and replay self-challenges begin next.
- S7 AR rev-25 static self-challenge is **GREEN** with no sibling correction
  required. From an empty exact-process preflight with 87% free memory, exact
  selector `^TestS7ARRev25StaticChallenges$` passed **30/30** typed complete-
  validator cases using `-p=1 -count=1 -timeout=6m` in 60.621s package /
  64.47s wall. Bite/inverse pairs cover nested blocks, if/else, for, range,
  switch, type-switch and select return propagation; unreachable statements
  and loop back-edge exclusion; return-expression mutation before defers;
  multiple returns plus natural completion; nested defer LIFO/exactly-once and
  nested async publication; self-capture target reassignment; branch-joined
  stored targets; recursion/unresolved fail-closed behavior; and disjoint/
  inert precision. This was the final static challenge after the last analyzer
  edit. Exact affected rev-21/rev-23/rev-24/rev-25 matrices begin next.
- S7 AR rev-25 affected matrices are **PASS**. From an empty exact-process
  preflight with 85% free memory, the exact eight-function rev-21/rev-23/
  rev-24/rev-25 canonical/challenge selector passed with
  `-p=1 -count=1 -timeout=8m` in 252.095s package / 253.23s wall.

  The first extended 15-function rev-14…25 inherited run started from an
  empty exact-process preflight at 72% free memory, but environmental
  throughput was insufficient: the unchanged `-timeout=15m` bound fired while
  the final `TestS7ARRev21StaticChallenges` function was still running
  (`deferred-all-terminal-descriptor-replacement-is-disjoint` active).
  Package/wall times were 902.985s / 904.22s. This was a timeout, not an
  assertion failure, and no analyzer edit followed it. After the process
  drained, memory recovered to 82% with the exact-process set empty. One
  identical bounded retry under the recovered resource state will determine
  the inherited gate; the timeout is retained here rather than relabeled.
- The identical extended inherited retry also hit the unchanged 15-minute
  package timeout (902.008s package / 903.10s wall) while the final
  `TestS7ARRev21StaticChallenges` function was active; again there was no
  assertion failure. Static audit found a bounded performance sibling in the
  new common completion helper: it republished asynchronous schedules even
  though block walking publishes every natural successor and `ReturnStmt`
  publishes every terminal immediately. That duplicate whole-literal
  analysis is semantically idempotent but expensive across the inherited
  matrix. The duplicate completion-time async publication is removed;
  return-time publication and async unchanged/immediate alternatives remain
  unchanged. Because this is an analyzer edit, all prior post-edit evidence is
  superseded and the bounded canonical/challenge/inherited gates will rerun.
- After removing duplicate completion-time async publication, final rev-25
  canonical/challenge passed together in 70.060s package / 73.83s wall from
  an 82%-free preflight, and the eight affected matrices passed in 229.920s /
  230.39s at 84%. The extended inherited selector nevertheless timed out a
  third time at 902.752s package / 903.58s wall from a 77%-free preflight,
  again in the final rev-21 static-challenge function with no assertion
  failure.

  To preserve the mandated 15-minute inherited bound without weakening route
  evidence, rev-25's test-only fixture runner is now batched: the six canonical
  semantic fixtures execute as one unsafe and one safe complete-validator
  build, and the 30 challenge fixtures likewise execute as one unsafe and one
  safe build. Unsafe batches give every sink route a unique identifier and
  require every identifier (plus explicit non-sink fail-closed routes) in the
  complete-validator error; safe batches share one canonical callback and
  still require exactly four writes. All individual typed fixture bodies and
  bite/inverse semantics remain present. No analyzer behavior changed in this
  batching step. The first batched two-function run exposed only a typed-source
  fixture defect (`replacement` unused in the safe canonical batch): unsafe
  canonical and both challenge batches passed, while safe canonical failed
  type-check in 13.335s package / 17.34s wall. Adding `_ = replacement` fixed
  the fixture without changing analyzer logic. The batched exact rev-25
  selectors then ran before another inherited attempt.
- S7 AR rev-25 final matrices and inherited selector are **PASS**. The batched
  two-function rev-25 selector preserves all six canonical and 30 challenge
  semantic fixtures through two unsafe/two safe complete-validator builds and
  passed in 16.448s package / 19.94s wall from an 83%-free preflight. The
  eight affected rev-21/rev-23/rev-24/rev-25 functions then passed in
  161.350s / 161.84s at 83%. Finally, the exact 15-function rev-14…25 selector
  passed under the unchanged 15-minute bound in 724.400s package / 725.50s
  wall from a 77%-free preflight. Every final unsafe batch required all named
  route identifiers, every safe batch retained exactly four writes, and no
  inventory-count proxy satisfied an unsafe case. Focused non-observer
  ledger/runtime/report/S4 and PIB-511/519 regressions begin next.
- S7 AR rev-25 focused non-observer regressions are **PASS**. From clean
  exact-process preflights, the exact ledger/empty-target, PIB-508/520,
  PIB-506/507 divergence, PIB-509…517 runtime/archive-control, PIB-518
  purge-progress and real partial/completion/divergence reports, real owned
  divergence, and three S4 abandon functions passed together in 102.564s
  package / 103.67s wall at 83% free memory. Exact PIB-511 then passed in
  168.100s / 168.59s at 85%, and exact PIB-519 in 30.916s / 31.43s at 85%.
  PIB-518 was not run as a standalone eligibility timing target, so the
  forbidden PIB-511/518/519 trilogy did not occur. Formatting, vet, host build
  and Linux/Darwin amd64 CLI-test cross-build gates begin next.
- Final selector inventory (all `go test` commands used `-p=1 -count=1`):
  - final rev-25:
    `^(TestS7ARRev25StoredLiteralReturnRepros|TestS7ARRev25StaticChallenges)$`;
  - affected:
    `^(TestS7ARRev21ReviewerRepros|TestS7ARRev21StaticChallenges|TestS7ARRev23ScheduledClosureAliasRepros|TestS7ARRev23StaticChallenges|TestS7ARRev24ScheduledSelfCaptureRepros|TestS7ARRev24StaticChallenges|TestS7ARRev25StoredLiteralReturnRepros|TestS7ARRev25StaticChallenges)$`;
  - inherited:
    `^(TestS7ARRev14ReviewerRepro|TestS7ARRev15ReviewerRepro|TestS7ARRev16ReviewerRepro|TestS7ARRev17ReviewerRepros|TestS7ARRev18ReviewerRepros|TestS7ARRev19ReviewerRepros|TestS7ARRev20ReviewerRepros|TestS7ARRev21ReviewerRepros|TestS7ARRev21StaticChallenges|TestS7ARRev23ScheduledClosureAliasRepros|TestS7ARRev23StaticChallenges|TestS7ARRev24ScheduledSelfCaptureRepros|TestS7ARRev24StaticChallenges|TestS7ARRev25StoredLiteralReturnRepros|TestS7ARRev25StaticChallenges)$`;
  - focused:
    `^(TestS7ARCoverageLedger|TestS7ARCoverageLedgerRejectsEmptyTarget|TestS7ARExitSixRouteGuard|TestS7ARPrepareGrammarGuard|TestS7ARDivergenceContracts|TestS7ARAbandonContracts|TestS7ARArchiveControlContracts|TestS7ARPurgeProgressGuard|TestFeatureIntentArchiveRealPartialBranches|TestFeatureIntentArchivePartialDoesNotClaimRepairClassCompletion|TestFeatureIntentArchivePartialBranchesAndDivergenceReports|TestFeatureIntentArchiveRealOwnedDivergence|TestPrepareS4AbandonPreviewAndMove|TestPrepareS4AbandonRollbackPreservesConcurrentEvidence|TestPrepareS4ReportRenderingAndAbandonSecondRun)$`;
  - PIB-511: `^TestS7ARAbandonGateTableGuard$/^PIB-511$`;
  - PIB-519: `^TestS7ARPermanentBlockClaimsGuard$/^PIB-519$`.
- S7 AR rev-25 formatting/build validation is **PASS**. The changed guard is
  `gofmt`-clean, temporary rev-25 diagnostic markers are absent, and changed-
  path plus repository `git diff --check` are clean. From separate empty
  exact-process preflights with 85% free memory, `go vet -p=1 ./internal/cli`
  passed in 0.66s wall, `go build -p=1 -o /dev/null ./cmd/tpatch` in 0.40s,
  and `CGO_ENABLED=0 GOOS={linux,darwin} GOARCH=amd64 go test -c -p=1
  -count=1 -o /dev/null ./internal/cli` in 4.34s and 4.29s. Final hash and
  repository-boundary verification remains; no further Go command is planned.
- S7 AR rev-25 is frozen and **AWAITING PRE-OBSERVER STATIC REVIEW** at guard
  SHA-256
  `9dcee12aae86f2dd5c96565b5266c2ac2d823454f31c2fadc9ae77d3847ddf13`.
  The final analyzer carries explicit-return terminals independently through
  blocks, if/else, loops/ranges, switch/type-switch/select and nested
  schedules; evaluates return expressions before LIFO defers; executes nested
  defers exactly once; excludes terminals from fallthrough/back-edges; and
  merges returned/natural completion only for outer effect transfer. Frozen
  rev-24 registration-target, explicit-argument, capture/backing, LIFO, async
  alternative, fail-closed and cap semantics remain covered.

  Mandatory baseline evidence was **3/3 unsafe RED bites plus 3/3 safe
  inverses** against rejected rev-24. Final canonical coverage retains all six
  semantic fixtures in two route-sensitive complete-validator batches; final
  challenge coverage retains all 30 bite/inverse fixtures in two batches with
  every unsafe sink uniquely asserted and all safe paths at exactly four
  writes. Final rev-25 matrices, eight affected matrices, the exact
  15-function rev-14…25 selector, focused ledger/runtime/report/S4, PIB-511,
  PIB-519, vet, host build and Linux/Darwin amd64 CLI-test cross-builds all
  pass. Three earlier inherited attempts are explicitly retained as
  15-minute timeouts; batching genuine route evidence brought the unchanged
  final bound to 724.400s package / 725.50s wall without weakening assertions.

  Final boundary verification confirms HEAD/local/origin
  `c363ed7c1e1b43db2421c4c02bbd3c0e5d6b1430`, empty staging, empty workflow/
  assets diff, clean whitespace, exact 15-row `I6/C4/G4/S1` ledger, immutable
  64-state/32-loop caps and 85% free memory with no exact Go/compiler/linker/
  vet/CLI-test process. Runtime, renderer, ledger, observer/registration and
  AVP hashes remain
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`;
  Side Research remains `b385fe622db9926f48861105239f113e`.
  Rev-25 modified only the guard and this handoff. No observer, observer
  wrong-input target, standalone PIB-518 eligibility timing, eligibility
  trilogy, whole `^TestS7AR`, full/race/Windows suite, CI, staging, commit,
  push or tag occurred.
- Independent pre-observer static review rejected S7 AR rev-25 at frozen guard
  SHA-256
  `9dcee12aae86f2dd5c96565b5266c2ac2d823454f31c2fadc9ae77d3847ddf13`.
  Go evaluates every select communication's receive channel, send channel and
  send value once in source order before selecting a case. The analyzer instead
  starts each clause from the unchanged prefix, and ordinary `SendStmt`
  handling observes channel/value expressions without applying their
  mutations. A nil send-channel helper can therefore replace a callback,
  select `default`, return, and still let the analyzer replay the older sink
  against the pre-mutation callback. Existing rev-25 select fixtures mutate
  only inside a selected body and do not cover this mandatory preselection
  phase. The reviewer ran no Go command, observer, timing target, build or edit.
- S7 AR rev-26 is dispatched as a bounded correction/challenge revision before
  observer authorization. It may modify only
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. It must prove the
  reviewer canonical unsafe and restoring inverse RED against rev-25, then
  model ordinary send operands plus select preselection exactly once and in
  source order across one common state. Receive-channel operands, send-channel
  operands and send values execute before branching; selected receive
  assignment effects remain clause-local. Challenges must cover send values,
  receive operands, multiple communications and evaluation order, default
  inheritance, selected receive assignment timing and non-select sends.
  Rev-25 explicit-return propagation, nested/outer defer LIFO, async
  alternatives, registration-time targets/arguments, execution-time captures,
  fail-closed behavior, the 64-state/32-loop caps and all closed PIB-511/519
  behavior remain frozen. No observer, eligibility trilogy, full/race/Windows
  suite, CI, staging, commit, push or tag is authorized.
- S7 AR rev-26 mandatory typed RED is confirmed against rejected rev-25. From
  an empty exact Go/compiler/linker/vet/CLI-test process preflight and 85% free
  memory,
  `go test -p=1 -count=1 -run '^TestS7ARRev26SelectPreselectionRepros$' -timeout 3m ./internal/cli`
  failed both intended cases in 11.737s package / 15.29s wall. The unsafe
  nil-channel/default route falsely retained the four-write inventory instead
  of reporting unresolved origins at `s7ARRev26Sink`; the restoring inverse
  incorrectly reported unresolved origins at that sink instead of exactly
  four canonical writes. These are complementary route-specific bites of the
  missing common preselection mutation, not inventory-count proxy failures.
  Analyzer correction begins without changing or relabeling the RED fixtures.
- S7 AR rev-26 canonical correction is **GREEN**. The select walker now
  evaluates each communication operand once in source order into one shared
  preselection state: receive channels only, and send channel then value.
  Every case/default starts from that state; selected receive assignments
  evaluate LHS effects and bind only on their branch without reevaluating the
  receive channel. Ordinary sends use the same ordered operand evaluator.
  Calls are observed at their current reaching state and applied in Go
  evaluation order; bounded named helpers bind exact non-variadic arguments,
  execute with caller schedules isolated, transfer only shared backing effects
  and fail closed on recursion/unreliable backing. Rev-25 return/defer
  completion remains unchanged.

  The first correction run made the unsafe reviewer route pass but left the
  restoring inverse unresolved because the former named-helper path only
  marked its slice argument uncertain. Exact bounded helper execution closed
  that precision inverse. From empty exact-process preflights and 86% free
  memory, the unchanged exact selector
  `go test -p=1 -count=1 -run '^TestS7ARRev26SelectPreselectionRepros$' -timeout 3m ./internal/cli`
  then passed both canonical cases in 12.318s package / 16.14s wall. Static
  challenge expansion begins before inherited validation.
- The first rev-26 challenge attempt is **discarded as validation evidence**.
  Its preflight reported exact active PIDs `60296:go` and `60325:cli.test`, but
  the wrapper omitted fail-fast `set -e` and incorrectly continued instead of
  waiting. The overlapping command finished in 14.971s package / 18.48s wall;
  a subsequent exact-basename check was empty at 86% free memory. All later
  Go wrappers are fail-fast before launch. The discarded output nevertheless
  exposed two self-challenge issues before any inherited gate: the dedicated
  nested-defer unsafe route was absent from the unresolved set, and a
  restoring selector-LHS fixture remained unresolved. The nested route is
  split into its own non-masking batch for material diagnosis; the LHS inverse
  is narrowed to a disjoint backing while the relevant-backing bite remains.
- S7 AR rev-26 bounded self-challenge is **GREEN** after two sibling fixes.
  A clean 86%-free run first confirmed that a stored target with an older inert
  nested defer still dropped the new select-operand backing override
  (16.970s package / 20.69s wall). Diagnostic state tracing showed captured
  objects refreshed but their backing override maps did not: scheduled
  execution refreshed only explicit sequence arguments. The final correction
  refreshes the union of explicit-argument and true-capture backing identities,
  preserving the rev-24 execution-time capture contract. Temporary diagnostics
  were removed.

  The expanded matrix also bit named slice-method channel helpers: unsafe
  method receiver mutation was omitted and the restoring inverse remained
  unresolved (16.708s package / 20.04s wall). Exact helper execution now binds
  supported slice receivers for method values/expressions as well as ordinary
  parameters; unsupported receiver shapes, variadics, recursion and unreliable
  backing identities fail closed. Helper-local deferred completion runs under
  the callee owner before shared effects transfer.

  The final exact selector
  `go test -p=1 -count=1 -run '^(TestS7ARRev26SelectPreselectionRepros|TestS7ARRev26StaticChallenges)$' -timeout 4m ./internal/cli`
  passes in 17.314s package / 20.44s wall from an empty exact-process preflight
  and 86% free memory. Coverage is two canonical cases plus 21 challenges:
  nine unique ordinary unsafe routes, one isolated nested-defer unsafe route,
  and eleven exact-four-write inverses. It covers send channel/value order,
  receive expression/assignment operands, selector LHS branch locality,
  multiple communications/default inheritance, ordinary sends, explicit
  return with nested/outer defers, named-helper defer/method routes, `=`/`:=`
  receive forms and side-effect-free controls. Each unsafe batch requires every
  unique sink route; no fixture can pass by inventory count or sibling masking.
- S7 AR rev-26 inherited callable-demand validation is **PASS**. The exact
  affected selector covering rev-21 reviewer/challenges, rev-23 scheduled
  aliases/challenges, rev-24 self-capture/challenges, rev-25 return/challenges
  and both rev-26 matrices passed from an empty exact-process/86%-free
  preflight in 180.856s package / 181.37s wall. An initial inherited launch was
  correctly blocked before Go started because exact PIDs `31512:go` and
  `31546:cli.test` were active. After a two-minute wait they cleared; memory was
  73%, above the 70% stop threshold.

  The exact 17-function rev-14…26 selector then passed under the unchanged
  15-minute bound in 822.447s package / 823.24s wall. It included every prior
  reviewer repro from rev-14 through rev-21 plus the rev-21, rev-23, rev-24,
  rev-25 and rev-26 challenge/repro matrices. No analyzer edit has occurred
  after this inherited pass.
- S7 AR rev-26 focused non-observer regressions are **PASS**. The exact
  ledger/empty-target, PIB-508/520, PIB-506/507, PIB-509…518
  runtime/archive-control/report, real owned-divergence and three S4 abandon
  selector passed from an empty exact-process/80%-free preflight in 109.649s
  package / 110.84s wall. PIB-511 was initially blocked before launch by exact
  PIDs `82196:go`/`82234:cli.test`; after they cleared it passed at 84% free in
  175.808s package / 176.26s wall. PIB-519 was likewise delayed without
  overlap while recurring shared-worktree Go/test workloads remained active;
  after the final clear 86%-free preflight it passed in 33.086s package /
  33.76s wall. PIB-518 was not run as a standalone eligibility timing, so the
  forbidden PIB-511/518/519 eligibility trilogy did not occur.
- S7 AR rev-26 final static/format/build gates are **PASS**. Post-edit review
  traced every ordinary send, select preselection and selected-clause
  constructor; confirmed channel-before-value and clause source order, common
  correlated state, clause-local receive LHS/binding, preserved
  return/break/continue flows, exact-once nested defers, captured backing
  refresh, cap/fail-close behavior and no selected-operand reevaluation.
  `gofmt -l`, diagnostic-marker scan and `git diff --check` are clean.

  From separate empty exact-process/86%-free preflights,
  `go vet -p=1 ./internal/cli` passed in 0.80s wall, host
  `go build -p=1 -o /dev/null ./cmd/tpatch` in 0.50s, and the Linux amd64
  CLI-test cross-build in 4.67s. The first Darwin cross-build launch was
  correctly blocked before Go started by exact PIDs
  `4733:go`/`4741:cli.test`; after they cleared, the Darwin amd64 CLI-test
  cross-build passed in 4.58s. No analyzer edit followed these gates.
- S7 AR rev-26 is frozen and **AWAITING PRE-OBSERVER STATIC REVIEW** at guard
  SHA-256
  `062980bc350709c6f2b9bbffe2ebc05b8649986b79980741b4ddef277061c97e`.
  Mandatory RED remains two complementary failures against rev-25: unsafe
  send-channel nil/default falsely preserved four writes, while the restoring
  inverse remained unresolved. Final coverage is 2 canonical + 21 challenge
  fixtures: 10 route-unique unsafe bites and 11 exact-four-write inverses.

  Final selector inventory (every test used `-p=1 -count=1`):
  - rev-26:
    `^(TestS7ARRev26SelectPreselectionRepros|TestS7ARRev26StaticChallenges)$`;
  - affected:
    `^(TestS7ARRev21ReviewerRepros|TestS7ARRev21StaticChallenges|TestS7ARRev23ScheduledClosureAliasRepros|TestS7ARRev23StaticChallenges|TestS7ARRev24ScheduledSelfCaptureRepros|TestS7ARRev24StaticChallenges|TestS7ARRev25StoredLiteralReturnRepros|TestS7ARRev25StaticChallenges|TestS7ARRev26SelectPreselectionRepros|TestS7ARRev26StaticChallenges)$`;
  - inherited:
    `^(TestS7ARRev14ReviewerRepro|TestS7ARRev15ReviewerRepro|TestS7ARRev16ReviewerRepro|TestS7ARRev17ReviewerRepros|TestS7ARRev18ReviewerRepros|TestS7ARRev19ReviewerRepros|TestS7ARRev20ReviewerRepros|TestS7ARRev21ReviewerRepros|TestS7ARRev21StaticChallenges|TestS7ARRev23ScheduledClosureAliasRepros|TestS7ARRev23StaticChallenges|TestS7ARRev24ScheduledSelfCaptureRepros|TestS7ARRev24StaticChallenges|TestS7ARRev25StoredLiteralReturnRepros|TestS7ARRev25StaticChallenges|TestS7ARRev26SelectPreselectionRepros|TestS7ARRev26StaticChallenges)$`;
  - focused:
    `^(TestS7ARCoverageLedger|TestS7ARCoverageLedgerRejectsEmptyTarget|TestS7ARExitSixRouteGuard|TestS7ARPrepareGrammarGuard|TestS7ARDivergenceContracts|TestS7ARAbandonContracts|TestS7ARArchiveControlContracts|TestS7ARPurgeProgressGuard|TestFeatureIntentArchiveRealPartialBranches|TestFeatureIntentArchivePartialDoesNotClaimRepairClassCompletion|TestFeatureIntentArchivePartialBranchesAndDivergenceReports|TestFeatureIntentArchiveRealOwnedDivergence|TestPrepareS4AbandonPreviewAndMove|TestPrepareS4AbandonRollbackPreservesConcurrentEvidence|TestPrepareS4ReportRenderingAndAbandonSecondRun)$`;
  - PIB-511: `^TestS7ARAbandonGateTableGuard$/^PIB-511$`;
  - PIB-519: `^TestS7ARPermanentBlockClaimsGuard$/^PIB-519$`.

  Final boundary verification confirms HEAD/local/origin
  `c363ed7c1e1b43db2421c4c02bbd3c0e5d6b1430`, empty staging, empty workflow/
  assets diff, clean formatting/whitespace, exact 15-row `I6/C4/G4/S1`
  ledger, immutable 64-state/32-loop caps and 86% free memory with no exact
  Go/compiler/linker/vet/CLI-test process. Runtime, renderer, ledger,
  observer/registration and AVP hashes remain
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`;
  Side Research remains `b385fe622db9926f48861105239f113e`.
  Rev-26 modified only the guard and this handoff. No observer, observer
  wrong-input target, standalone PIB-518 eligibility target, eligibility
  trilogy, whole `^TestS7AR`, full/race/Windows suite, CI, staging, commit,
  amend, push or tag occurred. Next step is independent static review only.
- Independent static review rejected frozen S7 AR rev-26 at guard SHA-256
  `062980bc350709c6f2b9bbffe2ebc05b8649986b79980741b4ddef277061c97e`.
  The select preselection correction addresses the rev-25 canonical route, but
  exact named-helper execution violates call-argument value timing. It
  evaluates arguments left-to-right and then binds every parameter/backing
  from the final post-argument state. A later argument can reassign the
  expression used by an earlier slice argument after Go has already
  snapshotted that descriptor, causing helper mutation/effect transfer to
  target the wrong backing and falsely preserve four known writes.

  The reviewer supplied a typed unsafe route where `callbacks` initially
  aliases `original`, a later argument reassigns it to `other`, and the helper
  must still mutate `original`; the restoring inverse reverses the callback
  values. Existing rev-26 ordering fixtures use only stable helper arguments.
  Mandatory early-stop fired, so the rest of the adversarial review remains
  incomplete. The reviewer made no edit and ran no Go, observer or other
  prohibited command.
- S7 AR rev-27 is dispatched as a bounded correction/challenge revision before
  observer authorization. It may modify only
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. It must prove the
  reviewer unsafe/restoring pair RED against rejected rev-26, then preserve
  each receiver/argument value and backing at its own evaluation point across
  correlated alternatives while retaining later argument side effects in the
  final caller state. Parameter binding, call observation and helper-effect
  transfer must consume those snapshots rather than re-resolving earlier
  expressions after later arguments run. Challenges must cover method-value
  receivers, method-expression offsets, scalar/slice and aliased arguments,
  branching later effects and multiple ordered rebindings.

  Rev-26 select preselection, exact-once communication evaluation, selected
  receive assignment locality, return/defer/async semantics, registration-time
  targets/arguments, execution-time captures/backings, helper fail-closed
  behavior, the 64-state/32-loop caps and closed PIB-511/519 behavior remain
  frozen. No observer, eligibility trilogy, full/race/Windows suite, CI,
  staging, commit, push or tag is authorized.
- S7 AR rev-27 mandatory unchanged-logic RED fixtures are now present. The
  dedicated typed canonical pair copies the reviewer route exactly: an early
  `callbacks` slice argument is followed by a third argument that rebinds the
  variable before exact-helper execution. The unsafe assertion requires
  `unresolved callable origins` at the unique `s7ARRev27Sink` route and
  explicitly rejects an inventory-count proxy; the restoring inverse requires
  exactly four canonical writes. No analyzer logic has changed yet. The next
  phase is the exact two-case RED run against rejected rev-26.
- S7 AR rev-27 mandatory unchanged-logic RED is **CONFIRMED**. From an empty
  exact-process/86%-free preflight,
  `go test -p=1 -count=1 -timeout=2m ./internal/cli -run
  '^TestS7ARRev27CallOperandSnapshotRepros$'` failed in 13.061s package /
  17.40s wall exactly as reviewed. The unsafe bite falsely returned the
  four-write inventory (`applyIntentArchivePurgeResult` twice) instead of
  unresolved origins at `s7ARRev27Sink`; the restoring inverse returned
  unresolved routes `[s7ARRev27DescriptorSafe:s7ARRev27Sink
  s7ARRev27Sink:callbacks[0]]` instead of four writes. Both complementary
  cases therefore bite the same post-argument re-resolution defect for the
  intended route-sensitive reasons. The assertions are now frozen; analyzer
  correction may begin.
- S7 AR rev-27 operand-snapshot correction is implemented for the first
  canonical gate. Each call now carries a correlated function/receiver
  snapshot and one snapshot per argument while later operands mutate only the
  continuing caller state. Exact helper parameter/receiver binding and
  transfer-backing selection consume those snapshots; transfer starts from
  the final post-operand caller state and refreshes current overrides on each
  snapshotted backing, so descriptors/scalars are frozen but backing contents
  are not. Immediate target/demand/mutation/async observation inside the
  source-ordered evaluator also resolves from the corresponding operand
  snapshot; unrelated legacy expression paths remain unchanged.

  Defer/go registration evaluates operands through that same path, resolves
  stored targets from the function snapshot, binds exact named/literal
  parameters at registration and preserves operand side effects in the outer
  state. Supported named helpers complete under their own owner (including
  local defers); unsupported multi-operand/receiver shapes fail closed.
  Async unchanged/immediate alternatives and execution-time captured-backing
  refresh remain in place. No cap changed. The next phase is the unchanged
  two-case canonical GREEN gate before adding rev-27 challenges.
- S7 AR rev-27 canonical correction is **GREEN**. The first post-edit launch
  was correctly blocked before Go started by exact shared-worktree PIDs
  `84188:go`/`84196:cli.test`; later shared internal-suite processes also
  delayed execution without being killed. After iterative narrow diagnostics,
  the final unchanged canonical selector passed from an empty exact-process /
  85%-free preflight in 12.077s package / 15.65s wall. The unsafe case now
  reaches unresolved `s7ARRev27Sink`, and the restoring inverse sees exactly
  four canonical writes.

  One temporary diagnostic launch after a `gofmt` syntax failure was invalid:
  because that wrapper lacked `set -e`, `go test` reached setup and failed in
  0.08s without compiling or executing a test. The syntax was corrected,
  subsequent wrappers are fail-fast, and all temporary diagnostics are
  removed. This attempt is retained as invalid evidence, not counted as a
  gate.
- S7 AR rev-27 bounded static challenge matrix is now present and pending its
  exact run. It covers scalar callable snapshots, method-value and
  method-expression receivers, two aliased slice arguments with ordered
  writes, branch-correlated and repeated later rebindings, same-backing
  content mutation, disjoint backings, nested named/deferred helpers,
  recursive and variadic fail-close controls, plus deferred and asynchronous
  registration timing. Every unsafe fixture requires its unique unresolved
  route and rejects inventory-count proxy failures; every precision inverse
  requires exactly four writes. The async unchanged-state control keeps the
  accepted unchanged/immediate alternative rather than asserting an
  impossible restoring inverse.
- S7 AR rev-27 static challenge is **GREEN** after two additional
  correction-local findings. First, exact scalar callback binding had to feed
  operand-snapshotted origins into the global named-helper argument binder;
  evaluator-produced observations now carry a bounded marker and synthesize
  each distinct operand object from its own snapshot. Repeated incompatible
  objects fail closed rather than cross-product values, while unrelated
  legacy observations are unchanged. A direct callback invocation pair now
  proves unsafe unresolved versus exact-four-write safe behavior, in addition
  to the scalar-to-slice transfer pair.

  Second, an immediate function literal evaluated as a later call argument
  inherited and completed the caller's pending defer list. That made the
  earlier sink execute before the later operand and split two registered
  defers into separate states. Immediate literals now execute with a
  callee-local empty pending-defer list, complete their own defers once, then
  restore the untouched caller list while retaining body mutations. The
  deferred argument snapshot bite/inverse now pass, and the previously
  accepted nested-helper ownership rule is preserved.

  Final self-review starts observation from the final post-operand caller
  state, overlays the snapshotted function target and each correlated operand,
  and refreshes current backing contents only for snapshotted sequence
  descriptors. Scalar/index values remain value-frozen. Same-object
  incompatible snapshots fail closed; no snapshots leak across calls or
  recursion. The final exact rev-27 selector
  `^(TestS7ARRev27CallOperandSnapshotRepros|TestS7ARRev27StaticChallenges)$`
  passed from an empty exact-process / 72%-free preflight in 52.231s package /
  55.82s   wall. It contains 2 canonical cases and 26 focused challenge routes
  (route-sensitive unsafe/fail-close cases plus exact-four-write inverses and
  the async unchanged-state precision control), for 28 total rev-27 cases.
- S7 AR rev-27 affected-regression gate initially exposed a correction-local
  async regression. The rev-21 reviewer/static matrices failed nine
  async-escape bites because schedule registration no longer invoked the
  accepted async escape observer after operand evaluation. No other affected
  revision failed in the 267.762s package / 268.83s wall run. The scheduler now
  sends exact named calls through the per-operand snapshot observer and stored
  literal targets through the retained literal-body observer, preserving the
  async unchanged/immediate alternatives without re-resolving later operands.

  The immediate rev-21 recheck did not launch: its fail-fast preflight found
  65% system-wide free memory, below the mandatory 70% floor. Validation is
  paused at this resource boundary; no below-threshold Go process was started.
- S7 AR rev-27 inherited validation is now **GREEN**. Memory recovered through
  69% to the 70% launch floor without a Go process; the corrected rev-21
  reviewer/static selector then passed in 66.068s package / 69.55s wall.
  A first inherited launch at the then-final code timed out at the unchanged
  15-minute limit in `TestS7ARRev23StaticChallenges`, 902.646s package /
  903.68s wall. That result is retained as failed evidence. Profiling showed
  the new 28-fixture challenge loop rebuilt the complete typed production
  package once per fixture.

  The authorized fixture batching path now preserves sensitivity with two
  complete-validator models: every unsafe route remains a distinct function
  and the batch requires every unique outer/helper route in the unresolved
  error; every safe route starts from unresolved state, must execute its own
  restoration and route-specific sink, and all share the single canonical
  literal occurrence, so the batch can pass only at exactly four writes.
  Missing execution cannot be masked by a sibling. The final rev-27 selector
  passes in 14.618s package / 18.31s wall, and the final affected rev-21/23/
  24/25/26/27 selector passes in 188.276s package / 188.71s wall.

  After all performance edits and self-challenge were complete, the exact
  inherited rev-14…27 selector passed once at final code under the unchanged
  15-minute limit in 850.252s package / 851.39s wall, leaving about 49 seconds
  of package-time margin. Its earlier pre-optimization timeout is not counted
  as final-code evidence.
- S7 AR rev-27 focused non-observer regressions are **GREEN**. The focused AR
  ledger/runtime/report/S4 selector passed in 96.764s package / 97.71s wall;
  PIB-511 passed individually in 173.708s package / 174.18s wall; PIB-519
  passed individually in 29.330s package / 30.19s wall. The first focused and
  PIB-519 launches were blocked before Go by unrelated exact toolchain/test
  processes and retried only after clean 84%/85%-free preflights. Standalone
  PIB-518 eligibility and the PIB-511/518/519 trilogy were not run.
- S7 AR rev-27 final static/format/build gates are **GREEN**. `gofmt -l` on the
  guard, the temporary-diagnostic scan and `git diff --check` are clean.
  `go vet -p=1 ./internal/cli` passed in 0.78s wall; the host CLI build passed
  in 0.38s. From separate empty exact-process/85%-free preflights, Linux amd64
  and Darwin amd64 CLI-test cross-builds passed in 3.74s and 4.05s wall.
  No analyzer edit followed these gates.
- S7 AR rev-27 is frozen and **AWAITING INDEPENDENT STATIC REVIEW** at guard
  SHA-256
  `9c2fa6ddcc2710b6296e1a09ef838f22d2d11aa689a33b68e43110b53a240d58`.
  Mandatory unchanged-logic RED remains the complementary 13.061s package /
  17.40s wall failure against rev-26: the unsafe reviewer route falsely
  retained four writes while the restoring inverse remained unresolved.

  Final selector inventory (every test used `-p=1 -count=1`):
  - rev-27:
    `^(TestS7ARRev27CallOperandSnapshotRepros|TestS7ARRev27StaticChallenges)$`
    — 14.618s package / 18.31s wall;
  - affected rev-21/23/24/25/26/27:
    `^(TestS7ARRev21ReviewerRepros|TestS7ARRev21StaticChallenges|TestS7ARRev23ScheduledClosureAliasRepros|TestS7ARRev23StaticChallenges|TestS7ARRev24ScheduledSelfCaptureRepros|TestS7ARRev24StaticChallenges|TestS7ARRev25StoredLiteralReturnRepros|TestS7ARRev25StaticChallenges|TestS7ARRev26SelectPreselectionRepros|TestS7ARRev26StaticChallenges|TestS7ARRev27CallOperandSnapshotRepros|TestS7ARRev27StaticChallenges)$`
    — 188.276s package / 188.71s wall;
  - inherited rev-14…27:
    `^(TestS7ARRev14ReviewerRepro|TestS7ARRev15ReviewerRepro|TestS7ARRev16ReviewerRepro|TestS7ARRev17ReviewerRepros|TestS7ARRev18ReviewerRepros|TestS7ARRev19ReviewerRepros|TestS7ARRev20ReviewerRepros|TestS7ARRev21ReviewerRepros|TestS7ARRev21StaticChallenges|TestS7ARRev23ScheduledClosureAliasRepros|TestS7ARRev23StaticChallenges|TestS7ARRev24ScheduledSelfCaptureRepros|TestS7ARRev24StaticChallenges|TestS7ARRev25StoredLiteralReturnRepros|TestS7ARRev25StaticChallenges|TestS7ARRev26SelectPreselectionRepros|TestS7ARRev26StaticChallenges|TestS7ARRev27CallOperandSnapshotRepros|TestS7ARRev27StaticChallenges)$`
    — 850.252s package / 851.39s wall;
  - focused AR ledger/runtime/report/S4:
    `^(TestS7ARCoverageLedger|TestS7ARCoverageLedgerRejectsEmptyTarget|TestS7ARExitSixRouteGuard|TestS7ARPrepareGrammarGuard|TestS7ARDivergenceContracts|TestS7ARAbandonContracts|TestS7ARArchiveControlContracts|TestS7ARPurgeProgressGuard|TestFeatureIntentArchiveRealPartialBranches|TestFeatureIntentArchivePartialDoesNotClaimRepairClassCompletion|TestFeatureIntentArchivePartialBranchesAndDivergenceReports|TestFeatureIntentArchiveRealOwnedDivergence|TestPrepareS4AbandonPreviewAndMove|TestPrepareS4AbandonRollbackPreservesConcurrentEvidence|TestPrepareS4ReportRenderingAndAbandonSecondRun)$`
    — 96.764s package / 97.71s wall;
  - PIB-511: `^TestS7ARAbandonGateTableGuard$/^PIB-511$` — 173.708s
    package / 174.18s wall;
  - PIB-519: `^TestS7ARPermanentBlockClaimsGuard$/^PIB-519$` — 29.330s
    package / 30.19s wall.

  Final boundary verification confirms HEAD/local/origin
  `c363ed7c1e1b43db2421c4c02bbd3c0e5d6b1430`, empty staging, empty workflow/
  assets diff, clean formatting/whitespace, exact 15-row `I6/C4/G4/S1`
  ledger, immutable 64-state/32-loop caps and 85% free memory with no exact
  Go/compiler/linker/vet/CLI-test process. Runtime, renderer, ledger,
  observer/registration and AVP hashes remain
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`;
  Side Research remains `b385fe622db9926f48861105239f113e`.
  Rev-27 modified only the guard and this handoff relative to the rejected
  rev-26 boundary. No observer, observer wrong-input target, standalone
  PIB-518 eligibility target, eligibility trilogy, whole `^TestS7AR`,
  full/race/Windows suite, CI, staging, commit, amend, push or tag occurred.
- Independent static review rejected frozen S7 AR rev-27 at guard SHA-256
  `9c2fa6ddcc2710b6296e1a09ef838f22d2d11aa689a33b68e43110b53a240d58`.
  The operand snapshot retains an earlier slice descriptor across later
  variable rebinding, but exact-helper binding refreshes only final-state
  `backingOrigins`. A later argument can overwrite the descriptor's shared
  backing through guaranteed-reuse append, which records only
  `backingUncertain`. Because that uncertainty is not projected onto the
  snapshotted parameter and later sequence resolution does not consume it,
  the helper can observe the stale pre-append callback and falsely authorize
  four writes.

  The reviewer supplied a typed pair: the unsafe route snapshots `view`, then
  evaluates `_ = append(storage[:0:1], replacement)` before invoking the
  helper; the restoring inverse uses `storage[:0:0]` to force allocation and
  preserve the original backing. Existing same-backing tests use exact
  assignment and miss this uncertainty path. Mandatory early-stop fired, so
  observation leakage, scheduling/defer/go, select remainder, inherited
  contracts and performance edits remain unreviewed. No observer or prohibited
  command ran.
- S7 AR rev-28 is dispatched as a bounded pre-observer correction/challenge
  revision. It may modify only
  `internal/cli/prepare_s7_ar_guard_test.go` and this handoff. It must first
  prove the guaranteed-reuse unsafe route and forced-allocation inverse RED
  against unchanged rev-27 for route-sensitive reasons. The correction must
  retain the operand's frozen descriptor identity, offset and view while
  projecting final-state backing uncertainty into helper binding, observation
  and effect transfer, or fail closed when that projection is unreliable.
  Challenges must cover nonzero views, aliases, multiple uncertain indices
  and disjoint/forced-allocation backings while preserving exact-origin
  behavior.

  Rev-27 call correlation, select preselection, return/defer/async,
  registration/capture, fail-closed behavior, 64-state/32-loop caps and closed
  PIB-511/519 behavior remain frozen. All Go work stays serial with `-p=1`,
  tests use `-count=1`, and no heavy phase may launch below 70% free memory.
  No observer, wrong-input observer target, standalone PIB-518 eligibility,
  eligibility trilogy, whole `^TestS7AR`, full/race/Windows suite, CI,
  staging, commit, amend, push or tag is authorized.
- S7 AR rev-28 mandatory unchanged-logic fixtures are present before any
  analyzer edit. The typed canonical unsafe case snapshots `view`, then
  overwrites its shared backing through guaranteed-reuse
  `append(storage[:0:1], replacement)` in a later argument; it requires
  `unresolved callable origins` at the unique `s7ARRev28Invoke` route and
  rejects inventory-count proxies. The forced-allocation
  `append(storage[:0:0], replacement)` inverse requires exactly four writes.
  Two attempted RED launches were correctly blocked before Go started by
  unrelated exact processes (`84347:go`, initially also
  `84382:cli.test`). No analyzer logic has changed; the exact RED remains
  pending a clean process/memory preflight.
- S7 AR rev-28 is **BLOCKED at the mandatory unchanged-logic RED** and awaits
  parent/reviewer adjudication. From clean empty-process / 86%-free preflights,
  the exact selector
  `^TestS7ARRev28BackingUncertaintyRepros$` did not bite for the reviewed
  route. The direct typed reviewer pair completed with `err == nil` for both
  the guaranteed-reuse unsafe case and forced-allocation safe inverse
  (19.80s package / 29.82s wall). Adding the ordinary local-target/defer
  scaffold produced the same two `nil` results (17.91s / 27.26s). Deferring
  the named outer fixture produced only unrelated unresolved outer-call
  routes and still omitted `s7ARRev28Invoke` (18.54s / 27.21s). The final
  minimal inline-defer scaffold again produced `err == nil` for both cases
  (18.57s / 26.91s).

  Thus the unsafe assertion did not observe unresolved origins at the unique
  `s7ARRev28Invoke` route, while the inverse did not expose the required
  four-write inventory. This is not the complementary route-sensitive RED
  mandated by the dispatch. Per the stop condition, no analyzer logic,
  challenge matrix or regression/build gate was attempted. The only guard
  change is the dedicated rev-28 pair, currently SHA-256
  `8a8db9b3db7ccc78c91b9eb650deb7c7f752060c4ed542a108035a0252514891`;
  no observer or prohibited command ran.

  Blocked-state boundary verification confirms HEAD/local/origin
  `c363ed7c1e1b43db2421c4c02bbd3c0e5d6b1430`, empty staging, empty workflow/
  assets diff, clean tracked whitespace, exact 15-row `I6/C4/G4/S1` ledger,
  immutable 64-state/two 32-iteration caps, 86% free memory and no exact
  Go/compiler/linker/vet/CLI-test process. Runtime, renderer, ledger,
  observer/registration and AVP hashes remain
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`;
  Side Research remains `b385fe622db9926f48861105239f113e`.
- Parent adjudication confirmed that the first rev-28 pair used an unsupported
  harness edge rather than disproving the static finding. The helper called
  `callbacks[0]()` through a non-variadic slice parameter, which
  `s7ARFixedCallableSliceElementInvocation` deliberately excludes from
  callable-origin traversal. Rev-28 is re-dispatched for a test-only scaffold
  correction through the already supported variadic-sink route. Analyzer
  logic remains frozen until unchanged rev-27 falsely retains exactly four
  writes for the guaranteed-reuse branch while the forced-allocation inverse
  independently proves exactly four writes. Any other result stops the
  revision and requires withdrawing or substantially narrowing the finding.
- S7 AR rev-28 corrected-scaffold unchanged-logic RED — **unexpected semantic
  result; stopped for adjudication**. The fixture now retains the
  non-variadic `s7ARRev28Invoke(callbacks []func(), _ int)` operand-order
  boundary, forwards `callbacks...` to unique variadic
  `s7ARRev28Sink`, and uses direct named unsafe/safe outers. The unsafe later
  argument performs guaranteed-reuse `append(storage[:0:1], replacement)`;
  the safe inverse uses forced-allocation `append(storage[:0:0],
  replacement)`. The unsafe assertion requires unresolved callable origins at
  `s7ARRev28Sink` and rejects inventory-count proxies; the safe assertion
  requires exactly four writes.

  From a clean no-process / 86%-free preflight, the sole command
  `GOMAXPROCS=1 go test -p=1 -count=1 -v ./internal/cli -run
  '^TestS7ARRev28BackingUncertaintyRepros$' -timeout=5m` produced:

  ```text
  === RUN   TestS7ARRev28BackingUncertaintyRepros
  === RUN   TestS7ARRev28BackingUncertaintyRepros/guaranteed-reuse-unsafe-bite
  === RUN   TestS7ARRev28BackingUncertaintyRepros/forced-allocation-safe-inverse
      prepare_s7_ar_guard_test.go:9673: PIB-518 forced-allocation backing inverse: error=invoked function-valued result has unresolved callable origins: [s7ARRev28Invoke:s7ARRev28Sink s7ARRev28Sink:callbacks[0]] writes=0 counted=false, want four canonical writes
  --- FAIL: TestS7ARRev28BackingUncertaintyRepros (19.99s)
      --- PASS: TestS7ARRev28BackingUncertaintyRepros/guaranteed-reuse-unsafe-bite (17.34s)
      --- FAIL: TestS7ARRev28BackingUncertaintyRepros/forced-allocation-safe-inverse (2.65s)
  FAIL
  FAIL github.com/tesseracode/tesserapatch/internal/cli 20.385s
  FAIL
  real 29.31
  user 27.14
  sys 1.33
  ```

  This is not the mandatory unchanged-logic asymmetry. Unsafe did not expose
  exactly-four-write false retention: it already failed closed at the unique
  sink route and therefore passed its post-correction assertion. Safe did not
  retain four writes: it failed closed at both sink-transfer routes. Because
  this is a semantic mismatch rather than a syntax/type/harness-construction
  error, no repair/retry or analyzer edit was attempted.

  The guard is gofmt-clean, its scoped `git diff --check` is clean, and its
  SHA-256 is
  `002343ddde7a2745435530cfafd1f037e46f0eb99db87df300cb93a810874d63`.
  Analyzer logic remains unchanged from rejected rev-27; this revision changed
  only the rev-28 fixture source and assertions plus this tracking evidence.
  HEAD/local/origin remain
  `c363ed7c1e1b43db2421c4c02bbd3c0e5d6b1430`; staging and workflow/assets
  diff remain empty. Frozen runtime, renderer, ledger,
  observer/registration and AVP SHA-256 values remain
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`.
  Side Research remains `b385fe622db9926f48861105239f113e`.
  Final state at 2026-08-23T21:15:56-0700 was 86% free memory with no exact
  `go`, `compile`, `link`, `vet` or `cli.test` process. No challenge,
  inherited selector, observer, PIB-518 eligibility/trilogy, whole
  `^TestS7AR`, full/race/Windows suite, vet/build/cross-build, staging,
  commit, amend, push, tag or CI occurred.
- Parent adjudication found one final supported discriminator before the
  rev-27 finding must be withdrawn. The helper-to-variadic-sink route is
  dominated by fixed-parameter expansion uncertainty in both branches.
  Method-value receiver evaluation instead snapshots the receiver descriptor
  before later arguments, binds that receiver from `context.function`, and is
  not excluded by `s7ARFixedCallableSliceElementInvocation`. Rev-28 is
  re-dispatched for a test-only named-slice receiver pair whose later argument
  first rebinds the receiver variable away from the frozen backing and then
  mutates that backing through a retained alias. Unsafe uses guaranteed-reuse
  append; safe uses forced allocation. Both must expose exactly four writes
  under unchanged logic, making only the unsafe unresolved-origin assertion
  RED. Any other result withdraws or substantially narrows the finding.
- S7 AR rev-28 final bounded test-only adjudication — **required asymmetry not
  established; withdraw or substantially narrow the rev-27 finding**. The
  fixture now uses named slice type `s7ARRev28Callbacks` and value-receiver
  method `s7ARRev28Invoke`. Each direct outer snapshots `callbacks := storage`
  as the method receiver before its later argument first rebinds
  `callbacks = other` and then mutates the retained `storage` alias. Unsafe
  uses guaranteed-reuse `append(storage[:0:1], replacement)`; safe uses
  forced-allocation `append(storage[:0:0], replacement)`. No analyzer line
  changed.

  After gofmt, a clean exact-process / 86%-free preflight, and no overlapping
  command, the sole test launch
  `GOMAXPROCS=1 go test -p=1 -count=1 -v ./internal/cli -run
  '^TestS7ARRev28BackingUncertaintyRepros$' -timeout=5m` produced:

  ```text
  === RUN   TestS7ARRev28BackingUncertaintyRepros
  === RUN   TestS7ARRev28BackingUncertaintyRepros/guaranteed-reuse-unsafe-bite
  === RUN   TestS7ARRev28BackingUncertaintyRepros/forced-allocation-safe-inverse
      prepare_s7_ar_guard_test.go:9675: PIB-518 forced-allocation backing inverse: error=invoked function-valued result has unresolved callable origins: [s7ARRev28Invoke:callbacks[0]] writes=0 counted=false, want four canonical writes
  --- FAIL: TestS7ARRev28BackingUncertaintyRepros (19.54s)
      --- PASS: TestS7ARRev28BackingUncertaintyRepros/guaranteed-reuse-unsafe-bite (17.07s)
      --- FAIL: TestS7ARRev28BackingUncertaintyRepros/forced-allocation-safe-inverse (2.47s)
  FAIL
  FAIL	github.com/tesseracode/tesserapatch/internal/cli	19.902s
  FAIL
  ```

  Unsafe again did not RED on exactly-four-write false retention: unchanged
  rev-27 already returned route-sensitive unresolved callable origins naming
  `s7ARRev28Invoke`, and the assertion confirmed it was not an inventory-count
  proxy. Safe also returned unresolved origins at
  `s7ARRev28Invoke:callbacks[0]` rather than exactly four writes. This is an
  unexpected semantic result, so the authorized syntax/type repair was not
  used and no semantic retry occurred. The final bounded supported route
  therefore does not prove the reviewed stale-backing false authorization.
  Rev-27's actionable finding should be withdrawn, or narrowed to a static
  concern that does not authorize analyzer work unless a separately proven
  precision-preserving reachable route is supplied.

  The guard SHA-256 is
  `68c255fff9cd98070b353f5567ca6d98048cc47ca41f80ac230ffded88f10946`;
  gofmt and scoped whitespace checks are clean. HEAD/local/origin remain
  `c363ed7c1e1b43db2421c4c02bbd3c0e5d6b1430`; staging and workflow/assets
  diff remain empty. Frozen runtime, renderer, ledger,
  observer/registration and AVP SHA-256 values remain
  `012cc4d76ab5a0655a92895bbca178397eb3546dd9649c331a1fede375496d3b`,
  `07abc40fea69440345251aab997910a5d00266e16b28de2261d97df2304de7f8`,
  `2627c62b7a5fb129b01de2322baa163fb44e767aff8afa46c32ca602dbc7dbd1`,
  `783553bc55cf5a2d5572df1baf43c911d12c4e7af10dea1ae8a854e98748ee23`
  and
  `60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38`.
  Side Research remains `b385fe622db9926f48861105239f113e`.
  Final evidence at 2026-08-23T21:20:44-0700 showed 86% free memory and no
  exact `go`, `compile`, `link`, `vet` or `cli.test` process. No analyzer or
  challenge edit, inherited selector, observer, wrong-input observer, PIB-518
  eligibility/trilogy, whole S7AR/full/race/Windows suite,
  vet/build/cross-build, staging, commit, amend, push, tag or CI occurred.
- Parent adjudication **withdraws rev-27's stale-backing finding as an
  actionable analyzer defect**. Three bounded attempts failed to establish a
  supported unsafe-RED / precision-GREEN distinction; the final method-value
  receiver route failed closed at `s7ARRev28Invoke:callbacks[0]` for both
  guaranteed reuse and forced allocation. Rev-28's experimental fixture was
  removed without changing analyzer logic. After gofmt,
  `internal/cli/prepare_s7_ar_guard_test.go` is restored byte-for-byte to the
  frozen rev-27 SHA-256
  `9c2fa6ddcc2710b6296e1a09ef838f22d2d11aa689a33b68e43110b53a240d58`;
  `git diff --check` is clean. A future uncertainty concern requires a
  supported route that proves false acceptance and a green disjoint-backing
  inverse. Independent static review now resumes from frozen rev-27 and must
  cover every area skipped by the prior mandatory early-stop. Observer
  execution remains forbidden.
- Independent resumed static review of frozen rev-27 is **APPROVED**. All
  eight required areas completed: call-operand and receiver snapshots,
  observation leakage, scheduling/defer/go, select pre-evaluation,
  correlation/convergence, inherited bite/inverse contracts, protected
  runtime/ledger/renderer/registration/AVP boundaries, and performance/cache
  correctness. No defect, edit, Go command or observer occurred. Guard
  SHA-256 remains
  `9c2fa6ddcc2710b6296e1a09ef838f22d2d11aa689a33b68e43110b53a240d58`;
  every protected hash, exact `I6/C4/G4/S1` ledger, 64-state/32-loop cap,
  staging, workflow/assets boundary and Side Research hash match. The
  reviewer noted only a non-blocking lockstep invariant between
  `backingOverrideSet` and `backingOrigins`; all current mutation sites
  preserve it.
- S7 AR rev-29 is dispatched as a **validation-only** revision. No code or
  test source may change; only this handoff may record evidence. After a clean
  exact-process and memory preflight, it must run fresh serial
  PIB-518/511/519 timing eligibility with package-time sum at or below 325
  seconds. Only then may it consume one unchanged
  `TestS7ObservedARRegistrationAuthority` execution under the frozen 12m
  outer / 8m inner / 1m cleanup budgets and a 419-second external cutoff.
  Any source/hash change, competing Go process, eligibility failure or
  observer failure stops the revision. Prior observer allowances remain
  consumed.
- S7 AR rev-29 FINAL-CODE validation-only is **BLOCKED before the observer and
  AWAITING REVIEW**. The pre-Go boundary at
  `2026-08-23T21:41:12-0700` matched HEAD/local `main`/`origin/main`
  `c363ed7c1e1b43db2421c4c02bbd3c0e5d6b1430`, all six frozen SHA-256
  values, Side Research md5 `b385fe622db9926f48861105239f113e`, the exact
  15-row `I6/C4/G4/S1` ledger, 64-state/two 32-iteration caps, empty staging
  and workflow/assets diff, and clean `git diff --check`; memory was 86% free
  with no exact Go/toolchain/CLI-test process.

  Fresh serial eligibility evidence:
  - PIB-518,
    `^TestS7ARPurgeProgressGuard$/^PIB-518$`: PASS; test 149.51s,
    package 150.306s, wall 151.23s; launch preflight 85% free/no exact
    process.
  - PIB-511,
    `^TestS7ARAbandonGateTableGuard$/^PIB-511$`: PASS; test 280.45s,
    package 280.894s, wall 281.75s. The wrapper waited through unrelated
    processes and 67–68% memory, then launched at exactly 70% free with no
    exact process.
  - The two-command package sum was already **431.200s**, exceeding the
    325s eligibility cap by 106.200s (test sum 429.96s; wall sum 432.98s).
    The mandatory stop therefore fired before PIB-519. PIB-519 was not run
    and no interrupted/failed attempt was counted.

  `TestS7ObservedARRegistrationAuthority` was not started, so there is no
  observer timing or result and no 419-second cutoff machinery was created.
  Its start-consumed rev-29 allowance was therefore not consumed, but the
  observer must not run under this blocked revision without separate
  authorization.

  The read-only close at `2026-08-23T21:50:23-0700` reconfirmed all refs,
  frozen hashes, ledger/caps, Side Research hash, empty staging and
  workflow/assets diff, and clean whitespace. Terminal resources at
  `2026-08-23T21:55:18-0700` were 76% free with no exact process. The
  required residue check was not clean: ignored pre-existing `./cli.test`
  remains present (19,416,244 bytes, mtime
  `2026-08-12T07:12:00-0700`, SHA-256
  `d691e86efb64ad8b3f1ac78e8247075a046f780d27e656a2a155ed42e764cde9`).
  It was not created or removed by rev-29 because only this handoff was
  writable. No further Go command, observer, correction, suite, build, vet,
  CI, staging, commit, push or tag occurred.
- Supervisor review closes rev-29 **BLOCKED** and authorizes one rev-30
  validation-only retry under a strengthened CPU-quiet gate. Rev-29's
  431.200s partial sum is not discarded, but it is materially inconsistent
  with the identical frozen guard's prior PIB-511 package time: 280.894s
  versus 173.708s, a 107.186s / 61.7% increase without a source change.
  PIB-518 similarly rose from the earlier 102.880s reference to 150.306s.
  Post-run inspection found unrelated Go/Python workloads and a Copilot
  process above one core; the prior exact-Go/memory gate did not exclude
  non-Go CPU contention. Rev-30 must wait for both toolchain and CPU quiet,
  preserve every frozen boundary, and run the same fresh serial trilogy. If
  it exceeds 325s, no additional validation retry is authorized and AR
  returns to bounded performance correction. The observer remains unconsumed
  and may start once only after eligibility. Pre-existing ignored
  `cli.test`/`cli.test.exe` artifacts are out of scope and must not be removed.
- The first rev-30 quiet-window acquisition attempt expired at
  `2026-08-23T22:18:46-0700` after 1,200 seconds without one eligible
  60-second interval. Its final sample had 83% free memory, load 10.65 and four
  exact toolchain/test processes. Read-only inspection identified two active
  `go test ./... -count=1` runs in separate `tesseraspaces` repositories plus
  unrelated Python and Copilot CPU load. No tpatch Go command, timing selector
  or observer started; no source, hash, staging or Git boundary changed.
  Rev-30 therefore remains authorized but undispatched, and both its validation
  retry and observer allowance remain unconsumed.
- The operator cleared competing work and rev-30 passed its fresh dispatch
  gate at `2026-08-23T22:50:15-0700`: 84% free memory, load 4.29, no exact
  `go`/`compile`/`link`/`vet`/`cli.test` process, and 60 continuous eligible
  seconds. All six frozen hashes, refs, empty staging and workflow/assets
  boundary then matched. Rev-30 is dispatched for exactly one fresh serial
  PIB-518/511/519 timing trilogy. The observer remains prohibited unless the
  three package times total at most 325 seconds.
- Rev-30 PIB-518 (`^TestS7ARPurgeProgressGuard$/^PIB-518$`) passed on its
  sole fresh run: test 153.64s, package 154.307s, wall 155.44s. Its immediate
  preflight at `2026-08-23T22:51:53-0700` had 84% free memory, load 4.82 and
  no exact toolchain/test process. The running package sum is 154.307s,
  leaving 170.693s for PIB-511 plus PIB-519.
- PIB-511 did not start. After PIB-518, resources fell outside the strict
  preflight and a new 1,200-second wait never found 60 continuous eligible
  seconds. Its final sample at `2026-08-23T23:22:40-0700` had 77% free
  memory, load 10.76 and four exact toolchain/test processes. Rev-30 is
  paused with its PIB-518 result retained; PIB-518 must not be rerun.
  PIB-511, PIB-519 and the observer remain unconsumed.
- A second operator-requested PIB-511 gate attempt on `2026-08-25` also
  expired after 1,200 seconds without an eligible interval. Its final sample
  at `2026-08-25T17:15:41-0700` had 69% free memory, load 5.14 and no exact
  toolchain/test process. No selector or observer started and every frozen
  boundary remained intact.
- After operator cleanup, the PIB-511 gate passed at
  `2026-08-25T22:30:53-0700`: 87% free memory, load 2.67, zero exact
  toolchain/test processes and 60 continuous eligible seconds. All refs,
  hashes, staging, workflow/assets, Side Research and whitespace boundaries
  matched immediately before the gate. Rev-30 resumes at PIB-511 only;
  PIB-518 remains fixed at 154.307s.
- Rev-30 PIB-511 (`^TestS7ARAbandonGateTableGuard$/^PIB-511$`) passed
  functionally on its sole fresh run: test 243.55s, package 244.132s, wall
  245.78s. Its immediate preflight at `2026-08-25T22:31:25-0700` had 87%
  free memory, load 2.92 and no exact toolchain/test process.

  The retained PIB-518 package time plus PIB-511 is **398.439s**, exceeding
  the immutable 325-second trilogy cap by **73.439s before PIB-519**. The
  mandatory stop fired. PIB-519 and
  `TestS7ObservedARRegistrationAuthority` did not start, so both remain
  unconsumed. The read-only close at `2026-08-25T22:35:56-0700` reconfirmed
  HEAD/local `main`/`origin/main` at `c363ed7`, all six frozen hashes, empty
  staging and workflow/assets diff, Side Research md5
  `b385fe622db9926f48861105239f113e`, and clean whitespace. Terminal resources
  were 85% free, load 3.26 and no exact process.

  Rev-30 therefore establishes a final-code timing failure under the
  authorized CPU-quiet conditions. No further validation-only retry is
  authorized; the next revision must be a bounded performance correction.
- At `2026-08-25T22:37:30-0700`, the operator explicitly requested one fresh
  full timing remeasurement because rev-30's cumulative total combined
  PIB-518 from August 23 with PIB-511 from August 25. This is a diagnostic
  exception to the prior no-retry decision, not a relabeling of rev-30 and
  not observer authority. It must run fresh PIB-518/511/519 serially under
  the unchanged 325-second cap and mandatory early stop. A passing diagnostic
  still requires independent adjudication before any observer; a failing
  diagnostic confirms that bounded performance correction is required.
- The diagnostic quiet gate passed at `2026-08-25T22:39:10-0700` with 85%
  free memory, load 2.41, zero exact processes and 60 continuous eligible
  seconds. Fresh PIB-518 then passed on its sole diagnostic run: test 126.87s,
  package 127.329s, wall 128.44s. Its immediate preflight had 85% free memory,
  load 2.65 and no exact process. The diagnostic running sum is 127.329s,
  leaving 197.671s for PIB-511 plus PIB-519.
- Fresh PIB-511 did not start. Two consecutive 1,200-second passive recovery
  waits failed to produce 60 continuous seconds at or above 80% free memory.
  The first ended at `2026-08-25T23:01:54-0700` with 79% free memory, load
  2.29 and no exact process; the second ended at
  `2026-08-25T23:22:15-0700` with 78% free memory, load 2.89 and no exact
  process. Frozen refs, hashes, staging, workflow/assets, Side Research and
  whitespace boundaries remain intact.
- At `2026-08-25T23:30:48-0700`, the operator closed the incomplete
  diagnostic and accepted rev-30's 398.439s result as authoritative for the
  performance decision. Diagnostic PIB-518's 127.329s is retained only as
  supplemental variance evidence; it is not combined with later timings.
  Rev-31 is dispatched as a bounded test-harness performance correction.
  Production behavior, the analyzer's fail-closed semantics, the 15-row
  ledger, caps, observer budgets and 325-second threshold remain frozen.
- Rev-31's first bounded correction changes only
  `internal/cli/prepare_s6_contract_test.go`: source type models now reuse the
  exact immutable AST already parsed and checked by `go/types`, instead of
  reparsing each provided file and building five positional remap indexes for
  every mutated source set. `TestS6AnalyzerCacheSensitivity` now bite-pins
  that reuse. The four-test `go/types`/partial-type/cache sensitivity group
  passes in 0.896s package; the isolated cache test passes in 0.518s package.
  All six frozen AR hashes remain unchanged.
- Rev-31 acceptance timing has not started. Its first strict gate expired at
  `2026-08-25T23:55:10-0700` with 77% free memory, load 2.91 and no exact
  process. One PIB-511 CPU profile is authorized under the normal 70% analysis
  floor solely to identify remaining test-harness hot paths; it is not timing
  evidence and cannot authorize PIB-519 or the observer.
- The diagnostic PIB-511 profile passed functionally but was suspended or
  contended: test 523.01s, package 523.955s, wall 46,010.53s. None of those
  durations are acceptance evidence. Its 473.57s CPU sample nevertheless
  isolated `s6CloneReachingBindings` at 294.59s cumulative (62.21%), with
  allocator/GC work dominating.
- The profile exposed tuple-map loops incorrectly nested under
  `fieldsByObj`: tuple bindings were recopied once per object field-set and
  were dropped entirely when that map was empty. Rev-31 moves both loops to
  their independent level, pre-sizes every cloned map, and adds
  `TestS6ReachingBindingsCloneSensitivity` to prove tuple preservation and
  slice independence without field bindings. The five-test clone/type/cache
  guard set passes in 0.963s package under the normal resource floor.
- A second rev-31 strict acceptance gate expired at
  `2026-08-26T13:21:13-0700` with 73% free memory, load 1.95 and no exact
  process. Acceptance timing remains unstarted. One unprofiled PIB-511
  diagnostic may run under the normal 70% analysis floor to measure the
  corrected clone hot path; its duration cannot enter the 325-second ledger
  or authorize later selectors/observer.
- The first unprofiled corrected PIB-511 diagnostic passed at test 457.36s,
  package 457.824s and wall 458.24s after a 73%-free/load-2.25 preflight.
  It is non-authoritative and shows that loop relocation plus map pre-sizing
  alone is insufficient under constrained memory.
- A complete mutation-site audit found no in-place writes or sorts of the
  cloned expression-domain slices: updates replace map entries or append
  through helpers. Rev-31 now cap-clamps and shares those immutable slices so
  append operations copy on write, while tuple slices remain deep copies.
  The clone bite proves sharing, capacity clamping, append isolation, tuple
  preservation and tuple slice independence. The five-test guard set remains
  green in 0.963s package.
- Follow-up tracing found the initial typed-AST reuse optimization invalid:
  `s6ParseSource` deliberately supplies stable AST pointers across mutated
  fixture models, and those pointers key `s6BindingCache`. Replacing them with
  each new type-graph AST destroyed cross-fixture cache hits. That change and
  its bite assertion are fully reverted; only the profile-backed clone
  correction remains. The cache-preserving five-test guard set passes in
  0.983s package.
- The final-shape non-authoritative PIB-511 diagnostic passed after a
  73%-free/load-2.49 preflight: test 198.56s, package 198.907s, wall 199.33s.
  That is 200.819s / 50.2% faster than the preceding 399.726s copy-on-write
  diagnostic that lacked stable cross-fixture AST cache reuse. It remains
  analysis evidence only.
- Same-floor diagnostics for that shape were PIB-518 123.493s and PIB-519
  32.102s, for a 354.502s three-selector estimate—29.502s above the cap.
  Rev-31 therefore additionally pre-sizes every reaching-state clone map and
  each merge map from its input states, including nested alias/field maps.
  The five-test guard set remains green in 0.981s package.
- Pre-sizing reduced same-floor PIB-511 from 198.907s to 193.354s. An updated
  176.13s CPU profile then showed 83.21s cumulative in binding-cache result
  clones. Three model-construction sites only retain bindings read-only or
  immediately pass them to lexical projection, which clones before mutation.
  Rev-31 adds a read-only cache accessor for exactly those sites; mutable
  callers still receive independent states, and cache identity/isolation are
  bite-tested. The five-test guard set passes in 0.975s package.
- The read-only accessor experiment measured 196.562s, a 3.208s regression,
  and is fully reverted with its bite additions. Profiling instead showed
  unrelated named-return/model construction consuming nearly the remaining
  gap. PIB-511 mutates only `prepare_publish.go`, and all pre-abandon roots
  and followed helpers live in that file. Rev-31 now models only that file
  plus explicit `zz_s7_ar_*` fixture files while `go/types` still augments and
  checks the complete disk package. The full five-test prerequisite group
  passes in 0.965s package.
- The initial one-file boundary failed fast because
  `feature_intent_archive.go` owns `intentArchiveRefusalReport`, which the
  generic emission model otherwise restubbed before disk augmentation. Adding
  exactly that declaration-owner file closes the duplicate without restoring
  unrelated production bodies. The two-file program passes PIB-511's complete
  baseline and mutation matrix at test 102.91s, package 103.363s and wall
  110.88s after a 75%-free/load-3.17 preflight—89.991s faster than the
  pre-boundary 193.354s diagnostic. A bite proves `zz_s7_ar_*` fixture files
  remain included.
- The current rev-31 guard hash is
  `2b9649f092aaed4fdfede01d5aa566d95814beda24e13ac84d62d65109a4b4b7`;
  the shared S6 harness hash is
  `cfbf853cf2fa77a02c4327d3d7462eb753ab3ab3327cfa93eca55ee1f9f49344`.
  Runtime, renderer, ledger, registration and AVP hashes remain frozen.
  Combining the current 103.363s PIB-511 diagnostic with the conservative
  pre-pre-sizing PIB-518/519 diagnostics estimates 258.958s, 66.042s below
  the cap. Only a fresh strict trilogy can establish eligibility.
- Rev-31's fresh authoritative trilogy did not start. Its latest 1,200-second
  strict gate expired at `2026-08-26T14:25:08-0700` with 79% free memory,
  load 1.99 and no exact process—one point below the unchanged 80% floor.
  No timing selector or observer allowance was consumed.
- The next authoritative gate passed at `2026-08-26T16:16:30-0700` with 83%
  free memory, load 1.97, zero exact processes and 61 continuous eligible
  seconds. Refs, staging, current hashes, workflow/assets, Side Research and
  whitespace boundaries matched before the gate. Rev-31 may now run exactly
  one fresh serial PIB-518/511/519 trilogy under the unchanged 325-second cap.
- Authoritative rev-31 PIB-518
  (`^TestS7ARPurgeProgressGuard$/^PIB-518$`) passed on its sole fresh run:
  test 121.33s, package 121.776s, wall 129.17s. Its immediate preflight at
  `2026-08-26T16:16:51-0700` had 83% free memory, load 2.14 and no exact
  process. The running sum is 121.776s, leaving 203.224s for PIB-511 plus
  PIB-519.
- Authoritative rev-31 PIB-511
  (`^TestS7ARAbandonGateTableGuard$/^PIB-511$`) passed on its sole fresh run:
  test 101.80s, package 102.177s, wall 102.59s. Its 61-second gate ended at
  `2026-08-26T16:20:22-0700` with 83% free memory, load 2.37 and no exact
  process; immediate launch was 83%/2.29/zero. The cumulative package sum is
  223.953s, leaving 101.047s for PIB-519.
- Authoritative rev-31 PIB-519
  (`^TestS7ARPermanentBlockClaimsGuard$/^PIB-519$`) passed on its sole fresh
  run: test 31.58s, package 31.983s, wall 32.40s. Its 61-second gate ended at
  `2026-08-26T16:23:42-0700` with 83% free memory, load 1.74 and no exact
  process; immediate launch was 83%/1.73/zero.

  The fresh authoritative package sum is **255.936s**, **69.064s below** the
  immutable 325-second cap (test sum 254.71s; wall sum 264.16s). Rev-31 is
  timing-eligible. No observer has started; independent review is required
  before a new observer allowance may be consumed.
- Independent static review returned **APPROVED** with no actionable finding.
  It verified tuple independence, cap-clamped expression sharing, complete
  clone/merge state preservation, the two-file body boundary with full-package
  `go/types` authority, all protected contracts and the 255.936s arithmetic.
  No Go command or observer ran during review.
- Supervisor decision dispatches rev-32 as observer-only. After one fresh
  60-second strict gate, it may consume exactly one unchanged
  `TestS7ObservedARRegistrationAuthority` execution under the frozen 12m
  outer / 8m inner / 1m cleanup budgets and 419-second external cutoff.
  Source/test edits, a hash change, competing exact process, observer failure
  or cutoff stop the revision.
- Rev-32's sole observer allowance is consumed and **PASS**. Its strict gate
  ended at `2026-08-26T16:35:13-0700` with 84% free memory, load 2.13, zero
  exact processes and 61 continuous eligible seconds. Immediate launch at
  `2026-08-26T16:35:35-0700` reconfirmed every ref/hash/boundary at
  84%/2.37/zero, then started one new process group with a 419-second external
  cutoff. `TestS7ObservedARRegistrationAuthority` passed all registered AR
  targets in **268.44s test / 268.835s package / 269.279s monotonic wall**;
  the cutoff remained clear. The frozen 12m/8m/1m budgets are unchanged.
- Read-only close at `2026-08-26T16:40:15-0700` reconfirmed refs at
  `c363ed7`, current guard/S6 hashes
  `2b9649f092aaed4fdfede01d5aa566d95814beda24e13ac84d62d65109a4b4b7`
  and
  `cfbf853cf2fa77a02c4327d3d7462eb753ab3ab3327cfa93eca55ee1f9f49344`,
  all other frozen hashes, empty staging and workflow/assets diff, Side
  Research md5 `b385fe622db9926f48861105239f113e`, clean whitespace and
  zero exact processes. Known root binaries retain their pre-observer mtimes;
  no new residue was created.
- Independent rev-32 evidence review returned **APPROVED** with no finding.
  It verified the single-start authority, exact command and frozen budgets,
  149.721-second cutoff margin, allowance consumption, final hashes/boundaries
  and unchanged pre-existing residue. Supervisor decision accepts AR and
  authorizes explicit-path checkpointing plus blocking CI. No further local Go
  command is permitted before checkpoint.
- AR code/test checkpoint `db17262` and tracking checkpoint `1a98f4c` were
  pushed to `origin/main`. Blocking CI
  [33024637427](https://github.com/tesseracode/tesserapatch/actions/runs/33024637427)
  failed after Windows passed. macOS produced deterministic PIB-511/PIB-518
  missing-model-file failures before its 40-minute timeout:
  `s6TypeModelCache` keyed only the augmented complete graph even though each
  model exposes only the caller-provided file projection. A prior model with
  the same augmented graph but a narrower projection could therefore omit
  `feature_intent_archive.go`. Ubuntu exited 143 after about 15 minutes with
  no assertion output. Rev-33 is bounded to the projection cache key and a
  direct bite before full-suite budget reassessment. Rev-32's observer is
  historical after this harness change and cannot be relabeled.
- Rev-33 keys `s6TypeModelCache` by both the augmented complete type graph and
  the exact caller-provided source projection; `model.sourceKey` carries that
  same composite identity into dependent caches. A direct bite proves that
  one-file and two-file projections converge on the same augmented graph but
  produce distinct models and retain exactly their provided files.
  `TestS6AnalyzerCacheSensitivity` passes in 0.772s package.
- Rev-33 fresh strict eligibility passes: PIB-518 121.029s, PIB-511 103.086s,
  PIB-519 31.660s; package sum **255.775s**, 69.225s below the 325-second cap.
  Every selector launched after a 60-second gate at 83% free memory, load
  2.00–2.17 and zero exact processes. The S6 harness hash is
  `45d4bf497f6c8ec61d6567dff29dea3bfb23de49938ce7a21d52b44862363ddf`;
  all other protected hashes and boundaries remain unchanged. No observer ran.
- Independent rev-33 review returned **APPROVED**. Rev-34 retains the exact
  finite 40-minute package limit but partitions the non-Windows test step into
  two sequential blocking processes: the complete non-AR set, then the
  complete AR plus AR-observer set. The guard must prove exact complementary
  regexes, no overlap/gap, canonical order, exact package targets, one timeout
  per process and unchanged Windows ownership.
- Rev-34 first split the non-Windows step into non-AR and broad-AR processes.
  The non-AR process correctly exposed AP PIB-459's stale declaration hash for
  AR's accepted `buildIntentArchiveDivergence` history-disclosure sentence;
  that sole hash is updated and PIB-459 plus the AP observer pass (0.565s /
  135.328s package). The corrected complete non-AR partition passes in
  948.59s wall (CLI 810.608s).
- The broad AR process then independently proved too large, timing out at
  2541.79s while entering the ledger. Rev-34 now uses four exact AR shards:
  legacy rev-11…20 repros (300.96s wall), current rev-21/23…27 repros
  (216.39s), accepted core guards/runtime/ledger (266.01s), and the AR
  observer alone. The workflow guard proves five total non-Windows commands,
  exact order/argv/package/40m timeout, the broad skip union, AST-level
  one-shard ownership for every matching test, non-empty shards, and unchanged
  Windows allowed-failure ownership. All mutation and external-owner bites
  pass in 0.856s package. No fresh AR observer has run after rev-33.
- Rev-34 hashes: S6
  `45d4bf497f6c8ec61d6567dff29dea3bfb23de49938ce7a21d52b44862363ddf`,
  CI guard
  `6e3f5079a68d0817f0e2033c13a05a49b4cdd338929411601ddc17d4364fe590`,
  AP inventory
  `f9d2caf5ace2d0ff74e6d42b502f7470b733374b895e3375c0835643433ec0c9`,
  workflow
  `2a5b7f6effa74b7547dd9689c0558ffecf47a8a79f5e7dc0c7c4fd7ec3374c87`.
  AR guard/registration and all production hashes remain unchanged; staging,
  formatting, whitespace and Side Research boundaries are clean.
- Rev-34 round-0 review returned **NEEDS REVISION**: shell operators were
  discarded before filtered `go test` validation, allowing `true || shard` to
  skip execution or `shard || true` to mask failure. The bounded correction
  requires the exact canonical five-line script before parsing and adds both
  bypasses as mutations. The complete guard suite passes in 0.444s package;
  workflow commands and prior shard timings are unchanged.
- Rev-34 round-1 re-review returned **NEEDS REVISION**: a custom
  non-Windows `bash {0}` shell could remove fail-fast semantics, and Windows
  lacked the exact-script check. The workflow now pins non-Windows
  `shell: bash`; the guard requires that exact value and the exact Windows
  command, with mutations for `bash {0}`, Windows prefix skip and suffix
  masking. The complete guard suite passes in 0.448s package; shard commands,
  ownership and timings remain unchanged.
- Rev-34 round-2 re-review returned **NEEDS REVISION** on unreachable runner
  topology, inherited `BASH_ENV`, and build-ineligible ownership. The guard
  now pins the exact Ubuntu/macOS/Windows matrix and
  `runs-on: ${{ matrix.os }}`, pins `BASH_ENV=/dev/null`, requires
  `set -euo pipefail`, and applies Go filename/build-constraint matching for
  Linux amd64 and Darwin arm64. Every broad-pattern test must be eligible on
  at least one non-Windows runner, belong to exactly one shard, and every
  shard must be non-empty on both runners. Windows-only, matrix, runner,
  environment and fail-fast sensitivities all bite; the complete guard passes
  in 0.466s package. Workflow test commands and prior shard timings remain
  unchanged.
- Rev-34 round-3 re-review returned **NEEDS REVISION** because file-wide
  topology/environment substrings could be satisfied by a dead copy, folded
  `run: >` was treated as newline-preserving, and inherited
  `GOFLAGS=-list=.` could suppress test execution. The guard now parses the
  actual `jobs.test` header and actual Test step environment/run style,
  requiring the exact direct matrix topology, `BASH_ENV=/dev/null`,
  `GOFLAGS=""`, literal `run: |`, and leading `set -euo pipefail`.
  Matrix-exclude, dead/demoted job, folded-run, GOFLAGS and decoy-resistant
  environment mutations all bite. Per-Linux/macOS build eligibility remains
  enforced. The complete guard passes in 0.407s package; executable shard
  commands and evidence remain unchanged.
- Rev-34 round-4 re-review returned **NEEDS REVISION** because direct job
  controls after `steps:` escaped header validation and inert block scalars
  could impersonate a `steps:` sequence. Topology validation now parses the
  complete direct `jobs.test` mapping regardless of key order, permits only
  exact name/runs-on/strategy/steps, validates exact strategy/matrix children,
  and rejects duplicate/unknown keys. Step discovery requires a direct
  indent-4 `steps:` node. Post-steps disable/demotion and inert scalar-decoy
  mutations all bite; the complete guard passes in 0.455s package. Workflow
  commands and all executable shard evidence remain unchanged.
- Rev-34 round-5 re-review returned **NEEDS REVISION** on quoted step keys and
  standalone sequence markers. Step/environment keys are now scalar-normalized
  before matching and every bare `-` flushes/starts an independent item.
  Quoted `continue-on-error` demotion and split-step impersonation mutations
  bite; the complete guard passes in 0.407s package. Workflow commands and all
  executable evidence remain unchanged.
- Rev-34 round-6 re-review returned **NEEDS REVISION** on escaped quoted keys
  and arbitrary indentation below standalone sequence markers. The guard now
  fails closed on every quoted step/environment key and every standalone step
  marker rather than partially decoding noncanonical forms. An escaped
  `continue-on-error` key and valid four-space-deeper split-step fixture both
  bite; the complete guard passes in 0.469s package. Workflow commands and all
  executable evidence remain unchanged.
- Rev-34 round-7 re-review returned **NEEDS REVISION** on commented bare
  markers and explicit mapping keys. Direct `- # comment` markers now fail
  closed; malformed, empty or `?` step keys are rejected; exact direct-key
  counts also reject merge keys and duplicates. Commented-marker,
  explicit-key, merge-key and duplicate-condition sensitivities all bite; the
  complete guard passes in 0.441s package. Workflow commands and all
  executable evidence remain unchanged.
- Final retained review returned only `NEEDS REVISION` with no finding or
  supporting content. Supervisor fallback audit closed the remaining parser
  surface directly. Quoted/escaped/explicit/merge/duplicate keys,
  bare/commented markers, scalar decoys, complete direct job topology, actual
  step environment/run style, per-OS build eligibility and exact shard union
  are all fail-closed and bite-tested. `BASH_ENV=/dev/null`, `GOFLAGS=""` and
  `GOENV=off` are pinned; Ruby's YAML parser independently resolves exactly
  the intended three-OS job and five-command Test step. The complete guard
  passes in 0.457s package. Supervisor verdict is **APPROVED**.
- Rev-35 is observer-only and may consume one fresh unchanged AR observer
  against rev-33 after the strict 60-second gate and under the 419-second
  cutoff. No source edit is authorized.
- Rev-35's sole observer allowance is consumed and **PASS**. Its strict gate
  ended at `2026-08-26T20:25:55-0700` with 90% free memory, load 2.28, zero
  exact processes and 61 continuous eligible seconds. Immediate launch at
  `20:26:07-0700` retained 90%/2.47/zero and all frozen refs/hashes.
  `TestS7ObservedARRegistrationAuthority` passed in **270.04s test /
  270.374s package / 270.827s monotonic wall**, clearing the 419-second cutoff
  by 148.173 seconds.
- Read-only close at `20:30:46-0700` found 89% free memory, load 2.23, zero
  exact processes, HEAD/local/origin `1a98f4c`, empty staging, clean formatting
  and whitespace, Side Research md5
  `b385fe622db9926f48861105239f113e`, and final hashes: S6
  `45d4bf497f6c8ec61d6567dff29dea3bfb23de49938ce7a21d52b44862363ddf`,
  AP inventory
  `f9d2caf5ace2d0ff74e6d42b502f7470b733374b895e3375c0835643433ec0c9`,
  CI guard
  `25eca1cdf268a459e8fd32427ba6a187f4d534373a11e7ef7bf85561e20aa4fb`,
  workflow
  `3127130bac3391b37d3afd6ff1bdbc27ce47bb3af23741d877617c825532cf3a`.
  AR guard/registration and production hashes remain unchanged.
- Independent rev-35 evidence review returned **APPROVED** with no mismatch.
  Supervisor decision authorizes explicit-path rev-33/34 checkpointing and
  blocking CI. The observer allowance is consumed and no further local Go
  command is authorized before checkpoint.
- Rev-33/34 cache, AP inventory, CI partition guard and workflow corrections
  are checkpointed at `5658d5e` by explicit-path staging. Unrelated untracked
  research files remain unstaged.
- Corrected blocking CI
  [33036710574](https://github.com/tesseracode/tesserapatch/actions/runs/33036710574)
  passed Ubuntu and Windows. macOS completed the non-AR partition in
  1703.167s, then the combined rev-11…20 AR process was SIGKILLed after
  2009.735s, before its 40-minute Go timeout; no assertion failed. Rev-36 is
  bounded to finer AR process shards so analyzer caches are released between
  revision groups and heavy semantic guards. Broad skip union, exact test
  ownership, blocking authority and 40-minute per-process limits stay fixed.
- Rev-36 replaces the four AR processes with ten smaller fresh processes:
  legacy 11–15, 16–18 and 19–20; current 21/23/24 and 25/26/27; lightweight
  core; PIB-511; PIB-518; PIB-519; and observer. The broad non-AR skip remains
  unchanged. The guard now requires eleven exact ordered commands and proves
  every broad-pattern test belongs to exactly one non-empty Linux/macOS shard.
- All nine new non-observer AR shards pass locally under the exact CI
  environment: 29.29s, 113.47s, 173.08s, 187.61s, 38.09s, 8.45s, 103.83s,
  121.61s and 31.95s wall respectively. The complete guard passes in 0.503s.
  Rev-35 observer remains valid because S6, AR guard/runtime/ledger and
  registration hashes are unchanged. Current workflow hash is
  `40777cb335beabcc3abceba96497e82a0f736915026ffc43002e32fb073791d7`;
  CI guard hash is
  `e6520ff942fc8e1121de15996a60ef6aca061fbf1a724b910109e1f374d1e4b1`.
- Independent rev-36 review returned **APPROVED**. It confirmed exact complete
  disjoint ownership, per-process budgets, GitHub job feasibility, macOS
  cache-release behavior and rev-35 observer non-invalidation. Workflow/guard
  checkpoint and blocking CI are authorized.
- Rev-36 workflow and guard are checkpointed at `9cd43b9` by explicit-path
  staging.
- Finer-shard blocking CI
  [33040928741](https://github.com/tesseracode/tesserapatch/actions/runs/33040928741)
  passed Ubuntu and Windows. macOS passed non-AR and all nine non-observer AR
  shards; only the final AR observer failed when its 8-minute inner budget
  expired at 489.674s package time. No target assertion failed.
- Rev-37 raises only AR's hosted inner budget from 8m to 10m. Outer 12m,
  cleanup 1m and package 40m remain unchanged; AP/AQ budgets remain frozen.
  The exact budget guard now uses the prior 8m value as its regression bite.
- Rev-37 exact budget/wrong-input/category-binding suite passes in 4.936s
  package. Registration hash is
  `95073eed5c7fff9e85a0de7ba549fcf2ac0b1d23e246c731fac3466db40edd46`;
  AR guard, S6, workflow and CI guard remain unchanged. Staging, formatting,
  whitespace and Side Research boundaries are clean. No post-rev-37 observer
  has run.
- Independent rev-37 review returned **APPROVED**. Rev-38 may consume one
  fresh unchanged AR observer after the strict resource gate, still under the
  stronger 419-second local cutoff.
- Rev-38's sole observer allowance is consumed and **PASS**. The strict gate
  ended at `2026-08-26T23:30:35-0700` with 90% free memory, load 1.70, zero
  exact processes and 61 continuous eligible seconds. Immediate launch at
  `23:30:56-0700` retained 90%/1.79/zero and all frozen refs/hashes.
  `TestS7ObservedARRegistrationAuthority` passed in **270.85s test /
  271.178s package / 271.655s monotonic wall**, clearing the 419-second cutoff
  by 147.345 seconds.
- Read-only close at `23:35:38-0700` found 89% free memory, load 2.92, zero
  exact processes, refs `3b65e1f`, empty staging, clean formatting/whitespace,
  Side Research intact and all rev-37/38 hashes unchanged.
- Independent rev-38 evidence review returned **APPROVED**. Rev-37's one-file
  budget checkpoint and blocking CI are authorized; no further local observer
  may run.
- Rev-37's AR-only hosted observer budget is checkpointed at `16e3495` by
  explicit-path staging.
- Blocking CI
  [33046534111](https://github.com/tesseracode/tesserapatch/actions/runs/33046534111)
  passed Ubuntu, Windows and every macOS non-observer shard. The macOS AR
  observer then exhausted the 10m inner budget at 611.861s package time with
  no target assertion failure. Rev-39 replaces the non-robust increment with
  AR-only outer 20m / inner 16m / cleanup 1m, still below package 40m; AP/AQ
  stay unchanged.
- Rev-39 exact budget/category/wrong-input suite passes in 4.891s package.
  Registration hash is
  `f30cc6270f9d6b59b3feef91124f72d1c4681c08ec643222267fe896fe865e4b`;
  all other protected hashes, staging, whitespace and Side Research remain
  unchanged. No post-rev-39 observer has run.
- Independent rev-39 review returned **APPROVED**. Rev-40 may consume one
  fresh unchanged local AR observer after the strict gate, still under the
  stronger 419-second cutoff.
- Rev-40's sole observer allowance is consumed and **PASS**. Its strict gate
  ended at `2026-08-27T01:32:07-0700` with 90% free memory, load 1.66, zero
  exact processes and 61 continuous eligible seconds. Immediate launch at
  `01:32:23-0700` retained 90%/1.93/zero and all frozen boundaries.
  `TestS7ObservedARRegistrationAuthority` passed in **272.35s test /
  272.676s package / 273.123s monotonic wall**, clearing the 419-second cutoff
  by 145.877 seconds.
- Read-only close at `01:37:05-0700` found 89% free memory, load 2.12, zero
  exact processes, refs `3dbfc8b`, empty staging, clean formatting/whitespace,
  registration hash
  `f30cc6270f9d6b59b3feef91124f72d1c4681c08ec643222267fe896fe865e4b`
  and Side Research intact.
- Independent rev-40 evidence review returned **APPROVED**, confirming the
  exact 145.877-second margin and clean single-start boundaries. Rev-39
  checkpoint and blocking CI are authorized.
- Rev-39's robust AR-only hosted observer budget is checkpointed at `0c5f55f`
  by explicit-path staging.
- Blocking CI
  [33054901334](https://github.com/tesseracode/tesserapatch/actions/runs/33054901334)
  passed Ubuntu and Windows but failed before reaching AR on macOS. The
  non-AR process exposed AM–AO's legacy hardcoded 90s inner observer timeout
  at 122.36s; no target assertion failed. Rev-41 moves AM–AO's exact 65-target
  set into the category-bound table at 8m/4m/1m and shards AM–AO/AP/AQ
  observers into fresh blocking CI processes. AR 20m/16m/1m remains unchanged.
- Rev-41 adds AM–AO (`PIB-395…448`) to the category budget table with exact
  65-target multiplicity (PIB-443 owns 12 leaves) at 8m/4m/1m. The non-AR
  skip now covers AM–AO/AP/AQ/AR observers, each with an exact fresh process;
  the guard proves fourteen commands and thirteen non-empty Linux/macOS
  shards. Budget/category and full CI mutation suites pass in 4.720s package.
  Current hashes: registration
  `76a20aa482c6f75ad9f6c9b997e171e06ae85ef33646b94bf4f9553b560dda7f`,
  workflow
  `a4920dcc03aada53a636de1bfaaebdf511bd9de02cdcdd5d6c3a4fe93717bedd`,
  CI guard
  `9cd2de0e30d37f43d1e523429e68d372a9d9a6298ae44eac484f191449b5bac4`.
  AR guard/S6 and production remain unchanged; no post-rev-41 observer ran.
- Rev-41 review found only a supervisor-LOG range typo, corrected from
  PIB-402…448 to PIB-395…448. Focused re-review returned **APPROVED**.
  Rev-42 may run fresh AM–AO/AP/AQ observer processes before checkpoint.
- Rev-42 fresh isolated observers all pass: AM–AO 77.94s test / 78.354s
  package, AP 234.54s / 234.857s, AQ 70.23s / 70.638s. AM–AO and AQ retain
  robust margins; AP leaves only 5.46s under its 4m inner limit. Rev-43 raises
  AP alone to the already proven AQ envelope, outer 12m / inner 8m / cleanup
  1m. AM–AO, AQ and AR remain unchanged.
- Rev-43 exact budget/category and fourteen-command CI guard suites pass in
  4.882s package. Registration hash is
  `19814bd74c93b0d5287c95dcbfba9446aac93ca502be0076e18facdbfd817b63`;
  workflow/CI guard and all production/AR hashes remain unchanged. No
  post-rev-43 observer has run.
- Independent rev-43 review returned **APPROVED**. Rev-44 may run fresh
  isolated AM–AO/AP/AQ observers against the final table; AR evidence remains
  valid because its tuple and all observer targets/sources are unchanged.
- Rev-44 fresh isolated observers all pass after one strict gate at 89% free
  memory, load 1.70 and zero exact processes: AM–AO 80.93s test / 81.259s
  package / 81.70s wall; AP 156.12s / 156.424s / 156.85s; AQ 70.58s /
  70.950s / 71.38s. Read-only close at `2026-08-27T02:53:28-0700` retained
  89% free memory, zero exact processes, refs `c19bc45`, empty staging, clean
  formatting/whitespace, hashes registration `19814bd7`, workflow `a4920dcc`,
  CI guard `9cd2de0e`, and Side Research intact.
- Independent rev-44 evidence review returned **APPROVED** with no mismatch.
  Rev-41/43 checkpoint and blocking CI are authorized.
- Final category budgets and fourteen-command observer isolation are
  checkpointed at `27473aa` by explicit-path staging.
- Blocking CI
  [33060962809](https://github.com/tesseracode/tesserapatch/actions/runs/33060962809)
  passed Ubuntu and Windows. macOS passed non-AR, AM–AO/AP/AQ observers and
  every nine non-observer AR shard, but AR remained last after nearly two
  hours and exhausted its 16m inner budget at 974.454s. Rev-45 moves only the
  existing fresh AR observer command directly after AQ, before recursive AR
  shards; coverage, budgets, job topology and source remain unchanged.
- Rev-45 exact-order guard passes in 0.459s package. Independent review
  returned **APPROVED**, confirming complete coverage, fail-fast semantics,
  hosted-degradation rationale and final observer non-invalidation. Combined
  rev-41/43/45 checkpoint and blocking CI are authorized.
- The final AR observer reorder and exact-order guard are checkpointed at
  `021697c` by explicit-path staging; category budgets were already present in
  the preceding checkpoint lineage.
- Blocking CI
  [33069822776](https://github.com/tesseracode/tesserapatch/actions/runs/33069822776)
  passed Ubuntu and Windows. macOS started AR after about 25 minutes, but the
  observer still exhausted its 16m inner budget at 971.863s; no target
  assertion failed. Rev-46 ends incremental tuning with AR-only outer 35m /
  inner 30m / cleanup 2m under package 40m, leaving nearly 14 minutes over the
  worst hosted run. AM–AO/AP/AQ remain unchanged.
- Rev-46 exact budget/category guard passes in 4.837s package. Registration
  hash is
  `16efe5e820ec4ca5fcce2c7d85c8524d5502e98827f50ba5e1371c14920b1a26`;
  prior 20m/16m/1m values each bite independently. No post-rev-46 observer has
  run; staging, whitespace and Side Research remain clean.
- Shared-job AR still timed out at 1816.512s under 30m after non-AR plus three
  observer processes, proving runner age/degradation rather than the nominal
  envelope is the remaining constraint. Rev-48 creates a separate blocking
  Ubuntu/macOS `s7-observers` matrix job, runs AR first, then AM–AO/AP/AQ, and
  makes release depend on both `test` and `s7-observers`. The main job skips
  all four observers and retains every ordinary/AR shard.
- Dedicated-job topology, exact four-command script, environment, order,
  release dependency and expanded complete AST shard union are guarded with
  direct mutations. Combined budget + CI guards pass in 4.646s package.
  Current hashes: registration `16efe5e8`, workflow `16aba535`, CI guard
  `158eea6e`; all production/AR analyzer hashes and Side Research are unchanged.
- Rev-48 round-0 review returned **NEEDS REVISION** because AM–AO PIB-445
  requires historical `git show` evidence but the new checkout was shallow.
  The observer job now uses exact `fetch-depth: 0`; the guard parses the actual
  checkout `with:` map and rejects shallow/missing depth. Dedicated-job
  topology/script/release, full-history checkout, budget and shard-union guards
  pass together in 4.721s package.
- Focused rev-48 re-review returned **APPROVED**. Dedicated observer
  workflow/guard checkpoint and blocking CI are authorized; final local
  observer evidence remains valid because no observer source changed.
- The dedicated full-history observer job and its semantic guard are
  checkpointed at `47c4488` by explicit-path staging.
- Blocking CI
  [33080934964](https://github.com/tesseracode/tesserapatch/actions/runs/33080934964)
  proved the dedicated job but macOS AR still exhausted inner 30m at
  1818.238s. Main Ubuntu/macOS and Windows blocking surfaces also failed only
  because AVP-175's `release-needs-removed` sensitivity still matched
  `needs: test`. Rev-49 sets AR to 39m/37m/1m under process 40m and updates
  that exact AVP sensitivity to `[test, s7-observers]`.
- Rev-49 final budget/category, dedicated observer-job, full-history checkout,
  release-dependency and AVP-175 suites pass together in 5.084s package.
  Current hashes: registration `11d1ef76`, workflow `c6cb015d`, CI guard
  `f59ef102`, AVP `c584f6b7`. Staging, whitespace and Side Research are clean.
  No post-rev-49 observer has run.
- Independent rev-49 review returned **APPROVED** with no findings. The final
  authoritative local AR observer is now eligible.
- The final rev-49 AR observer passed at 271.24s package / 272.166s external
  under the 419-second process-group cutoff. Its immediate preflight reported
  89% free memory, load1 2.04 and zero exact Go-tool processes after the
  authoritative 60-second quiet gate passed at 89% free.
- Rev-49's final AR budget and bounded AVP-175 sensitivity correction are
  checkpointed at `263d6a8` by explicit-path staging with the required trailer.
- Blocking CI
  [33085315805](https://github.com/tesseracode/tesserapatch/actions/runs/33085315805)
  passed Windows and Ubuntu observers. Main Ubuntu/macOS failed only PIB-391's
  frozen AVP source hash after the authorized sensitivity edit. Fresh macOS AR
  exhausted 37m at 2237.53s with only PIB-518 RUN; every later target was
  RUN=0. Release correctly skipped. Rev-50 must optimize PIB-518 rather than
  raise the monolithic timeout again.
- Rev-50's CPU profile measured 37.77s / 33.32% in repeated stable-expression
  key generation, including 21.17s formatting the same retained AST
  expressions. A `sync.Map` now caches keys only for parser-positioned,
  immutable expressions; synthetic `NoPos` expressions remain uncached and a
  direct bite proves both cache use and non-retention. Fresh PIB-518 fell from
  125.78s to 64.60s (48.6%).
- PIB-391 now keeps the accepted routing-tip AVP hash `60e7a736...` and
  separately pins current authorized bytes at `c584f6b7...`; current drift
  remains fail-closed. The complete pre-change golden test passes.
- Final fresh eligibility under a separate authoritative 60-second gate before
  every process: PIB-518 64.826s, PIB-511 102.578s, PIB-519 31.844s package;
  total 199.248s, 125.752s below the 325-second cap. Current hashes: S6
  `a6b27da2`, PIB golden `245083d5`, registration `11d1ef76`, AVP `c584f6b7`.
  Staging, whitespace and Side Research are clean.
- Independent rev-50 review returned **APPROVED** with no findings. One final
  local AR observer is authorized under the unchanged 419-second cutoff.
- The final optimized AR observer passed at 213.12s package / 213.989s
  external after the authoritative gate passed at 85% free, load1 1.56 and
  zero Go-tool processes. Immediate preflight was 85% / 1.62 / zero.
- Rev-50's positioned-AST cache and dual provenance pin are checkpointed at
  `871d3ab` by explicit-path staging with the required trailer.
- Blocking CI
  [33091499900](https://github.com/tesseracode/tesserapatch/actions/runs/33091499900)
  is green across Ubuntu, macOS and Windows, including both dedicated
  full-history observer jobs. Release correctly skipped on the untagged push.
  AR is durably complete: 15 exact rows, I6/C4/G4/S1; cumulative S7 is 126/173.
- AS is dispatched sequentially from tracking tip `6f7171e`: ten exact rows
  `PIB-521`…`PIB-530` (I7/G3). Scope is physical residue,
  purge-owned recovery/preview totality and authority shape. No prior slice
  contract or observer budget may regress.
- Independent rev-46 review returned **APPROVED**. Rev-47 may consume one
  final local AR observer after the strict gate under the unchanged stronger
  419-second cutoff.
- Rev-47's sole observer allowance is consumed and **PASS**. Its strict gate
  ended with 89% free memory, load 1.76, zero exact processes and 61
  continuous eligible seconds. Immediate launch retained 89%/1.97/zero and
  all frozen boundaries. The AR observer passed in **270.33s test / 270.658s
  package / 271.140s monotonic wall**, clearing the 419-second cutoff by
  147.860 seconds.
- Read-only close at `2026-08-27T05:51:13-0700` found 88% free memory, load
  2.57, zero exact processes, refs `0183465`, empty staging, clean
  formatting/whitespace, hashes registration `16efe5e8`, workflow `84cd11dc`,
  CI guard `ef8b683a`, and Side Research intact.
- Independent rev-47 evidence review returned **APPROVED**. Combined
  rev-41/43/46 checkpoint and blocking CI are authorized; no further local
  observer may run.
- The final AR 35m/30m/2m hosted tuple is checkpointed at `efe7fc2` by
  explicit-path staging; observer scheduling/guard changes are already in the
  preceding checkpoint lineage.
- S7 AS rev-0 added three test files and no production changes. Executable
  review returned **NEEDS REVISION**: six ledger rows target aggregate
  wrappers; PIB-521/525 use the wrong fixture slug and reach
  `feature-not-found`; G-kind "sensitivities" never feed wrong inputs through
  the same validator; PIB-529 omits generation and exact order; PIB-530 skips
  all journal cases; doctor/decode/rehydration, exact retry and `--all`
  blast-radius assertions are incomplete. The suite compiled, failed ledger
  resolution and those runtime rows, and reported eight skips. Rev-1 is
  dispatched; no production gap is established yet.
- S7 AS rev-1 rewrites all three AS test files without production changes.
  Ledger targets are now top-level test functions with empty subtests (no
  `t.Run("PIB-NNN", ...)` wrappers). PIB-521/525 use fixture-returned slug.
  PIB-524 uses `validateS7ASX11Classification` same-validator with 18
  reachable tuples, 5 mutated classifiers and an unchanged-mutation fatality.
  PIB-526 uses typed AST call graph with reverse reachability to verify
  exactly one `intentArchiveRecoverPurge` call from confirmed purge path;
  `prepare_publish` mutation sensitivity fails the same validator. PIB-528
  builds per-selector observations for all 4 selectors with preview+yes
  through `validateS7ASSelectorTotality`; orphans→all widening and missing
  --all blast-radius disclosure fail the same validator. PIB-522 uses
  `beforePurgeBlobRemove` hook (via `//go:linkname`) to prove index decode
  precedes first removal without relying on CaptureIndex; regenerate
  rehydrates exactly after repair. PIB-529 covers all 4 selectors including
  generation, validates JSON key ordering, and exact pending-purge schema.
  PIB-530 covers clean/pending/journal × 4 selectors × preview/yes (24
  combinations, zero skips) using `s6WriteJournalFixture`. Sensitivities
  are in separate top-level tests. Ledger is I7/G3 contiguous PIB-521..530.
  No production changes; no gap established. Ready for independent re-review.
- S7 AS rev-2 closes the rev-1 review/executable residuals. A genuine
  production defect was established: successful single-class preview
  admission rebuilt the selected class as `remaining_repairs`, producing two
  retry headings. The redundant fallback is removed; refused/multi-class
  previews retain remaining-repair reports.
- PIB-522 now observes the successful production strict decode through a new
  nil-by-default store seam before the existing removal seam. PIB-526 parses
  every CLI production file, pins exact alias/store references, direct caller
  and `!options.yes` branch shape. PIB-528 uses independent pending/retained/
  orphan targets and proves only the pending blob is removed. PIB-530 covers
  all 24 combinations with local-lane and authority/cache/lock path closure.
- AS is hosted-observer wired at 8m/4m/1m for exact rows 521…530; the guarded
  main skip and dedicated observer script include it. Complete AS + store seam
  + budget/CI guards pass; full store and existing archive CLI regressions
  pass. The AS observer passes in 10.120s package / 10.57s wall. Current
  hashes: store `c1b6331a`, store seam `dc2c7829`, seam nil `b5144f14`,
  runtime `fe47d7ec`, guard `0afec4b3`, ledger `da124469`, registration
  `a6e29ef9`, CI guard `78a5dc4b`, workflow `529eb204`. Staging, whitespace
  and Side Research are clean.
- Independent rev-2 review returned **APPROVED** with no findings. Final
  provenance/aggregate/AS/registration/CI/AVP-175 guards pass in 18.840s +
  0.301s; full store and existing archive CLI regressions pass; affected
  package vet and CLI build pass. Every Go process followed a separate
  authoritative 60-second gate at 85% free memory.
- AS production, exact tests, observer budget and CI topology are checkpointed
  at `9438848` by explicit-path staging with the required trailer.
- Blocking CI
  [33102941110](https://github.com/tesseracode/tesserapatch/actions/runs/33102941110)
  passed every main platform suite and the Ubuntu observer, including AS. The
  sole failure was monolithic macOS AR: optimized PIB-518 completed, PIB-519
  began, then the shared 37m inner budget expired at 2238.33s. AS has no
  observed failure. The bounded correction splits AR into fresh processes for
  rows 506…517, 518, and 519…520 under the unchanged per-process tuple.
- Split registration/CI guards pass, including missing-target, overlap, gap,
  command removal, masking and ordering bites. Fresh local observers pass at
  113.801s (rows 506…517), 72.164s (row 518) and 39.743s (rows 519…520)
  package after separate 60-second gates at >=85% free memory. Round-0 review
  requested purge/claims masking bites; re-review is **APPROVED** with no
  residual finding. Current hashes: registration `56356e69`, CI guard
  `26661fde`, workflow `9ed5c7c7`.
- The exact split is checkpointed at `9d15cb6` by explicit-path staging with
  the required trailer.
- Blocking CI
  [33108548581](https://github.com/tesseracode/tesserapatch/actions/runs/33108548581)
  is green on Ubuntu, macOS and Windows, including both observer jobs and all
  three fresh AR processes. Release correctly skipped on the untagged push.
  AS is durably complete at ten exact rows (I7/G3); cumulative S7 is 136/173.
- AT is dispatched sequentially from `a55a0ab`: `PIB-531`…`PIB-536`
  (I4/C1/G1), covering mixed tombstone/live-reference global availability,
  exact claim/removal/tombstone repair, multi-observation list/doctor truth
  and pre-syscall directory-authority refusal.

### AT Implementation Session

AT `PIB-531`…`PIB-536` is implemented in the worktree with six exact top-level
test bodies, one per PIB row.

**Files created:**

- `internal/cli/prepare_s7_at_runtime_test.go` — PIB-531/532/534/535/536 runtime
  tests: mixed-reference prepare/regenerate/list/doctor refusal (531), exact
  claim→remove→tombstone CAS ordering spy (532), pending-recovery + multi-class
  sequential repair (534), list over five fixtures (535), beforeLockAcquire
  workspace root open failure (536).
- `internal/cli/prepare_s7_at_guard_test.go` — PIB-533 G-kind: global orphan
  predicate contracts with two runtime fixtures (orphan-only and orphan+mixed)
  and three semantic sensitivity tests (per-reference predicate, silent mixed
  omission, partial intra-class cleanup) plus unchanged-mutation fatal.
- `internal/cli/prepare_s7_at_ledger_test.go` — AT coverage ledger (6 exact rows,
  I4/C1/G1), AST target resolution, observed target provider
  `s7ObservedATTargets`, and prior-partition preservation assertion.

**Files modified:**

- `internal/cli/prepare_s7_registration_test.go` — added `s7ObservedCategoryAT`
  constant with 8m/4m/1m budget, rows 531–536, targets 6; added
  `TestS7ObservedATRegistrationAuthority`; extended wrong-budget, missing-category,
  wrong-range and callsite-binding sensitivity fixtures.
- `.github/workflows/ci.yml` — main test skip now includes `AT` in the observer
  regex; s7-observers job runs `TestS7ObservedATRegistrationAuthority` after AS;
  release dependency unchanged.

**No production files changed.** All six tests exercise existing production paths
(mixed-reference classification, CAS ordering, orphan predicate, recovery
exception, list storage rendering, authority acquisition error mapping).
No production gap was demonstrated: the existing `authorityError` in
`internal/intentlock/error.go` already maps open-root and open-directory failures
to `CodeDirectoryFlockUnavailable`, and the prepare publish path already routes
that code through `prepareAuthorityRefusal` correctly.

**Untested state (AT scope):**

- PIB-531 doctor D9 finding tag assertion is loose (matches `contains "mixed"`)
  rather than the exact tag constant, because the doctor finding tag is
  implementation-derived and was verified by visual inspection rather than
  deep-string assertion.
- PIB-532 CAS spy reads the index at `beforePurgeIndexCAS` time, which is just
  before the atomic rename. The claim CAS has already prepared the pending index
  bytes at that point. The spy reads the on-disk index (pre-CAS) rather than the
  about-to-be-written index. The assertion verifies post-CAS state via the
  afterPurgeIndexDecode seam and the final parsed index.
- PIB-535 corrupt fixture uses a hash-wrong regular file; symlink, directory,
  FIFO and device node object kinds are covered by PIB-543 (AV scope).

**Likely residuals:**

- PIB-533 fixture 1 relies on `s7ASWriteResidueFixture` producing a directory-scan
  orphan through its tombstoned replacement. Verify that the orphan hash written
  as a bare blob file (no index reference) is detected by the production orphan
  predicate as an unindexed orphan.
- PIB-534 third fixture creates a dangling reference (retained, no blob) as the
  second repair class. If the production code's class ordering puts dangling
  before mixed, the rerun assertion for "repairs mixed then reports dangling"
  might reverse. The test trusts the production ordering.

**Supervisor rev-0 verdict: NEEDS REVISION.** The package does not compile
(unused claim variable); CI semantic guards omit AT; PIB-533 has an empty
mixed-index comparison and no actual shared validator for its three wrong
inputs; PIB-532 samples disk before claim CAS and duplicates AP's linker seam;
PIB-536's syscall counters are disconnected; PIB-534 discards its preimage.
Rev-1 is dispatched with no production gap yet established.

**AT rev-1 complete and APPROVED.** PIB-532 now observes committed claim and
tombstone CAS states through existing post-rename hooks. PIB-533 uses one
shared validator over exact residue-only/residue-plus-mixed observations and
three mutated clones. PIB-536 uses a platform-split raw authority test
dependency guarded to zero production callers across open-root/open-directory
and none/journal/preimage/staging evidence.

Two bounded production-source changes were established: corrupt list
presentation now states the exact-hash restoration alternative before its
final executable `rm -rf` line, and a raw test dependency composes copied
authority ops without changing production `Acquire`. AT is wired into
8m/4m/1m hosted observation and exact guarded CI.

Complete AT/archive/intentlock suites pass; the six-target observer passes in
9.413s package / 9.86s wall. AP/AR frozen checks pass in 47.728s after updating
only `intentArchiveCorruptClassPrerequisite`'s owned hash. AVP-175, vet, build,
Windows and FreeBSD intentlock compile pass. Final review is **APPROVED**.
Current hashes: presentation `a14869e1`, AP hash guard `8180c43c`, runtime
`659f419b`, G guard `6f22f0cc`, ledger `93cb3ebb`, registration `48bf9aaf`,
CI guard `65a16685`, workflow `38754ccc`, common authority dependency
`2c9e5ea2`, dependency guard `bbbf24e1`, supported/unsupported halves
`abd604d2`/`39c6ee74`. Staging, whitespace and Side Research are clean.
- AT production/test/observer/CI code is checkpointed at `496016a` by
  explicit-path staging with the required trailer.
- Blocking CI
  [33117127691](https://github.com/tesseracode/tesserapatch/actions/runs/33117127691)
  is green on every main platform and both observer jobs, including AT's six
  exact targets. Release correctly skipped on the untagged push. AT is durably
  complete at I4/C1/G1; cumulative S7 is 142/173.
- AU is dispatched sequentially from `c14f790`: `PIB-537`…`PIB-545`
  (I6/C2/G1), covering abandon boolean semantics, manual archive precedence,
  global pending ownership, claimed-hash crash/insertion windows, coexisting
  observation/admission behavior and derived claim/removal authority.
- External Group A+B review at baseline `81885a9` returned **APPROVED WITH
  NOTES**. Its two LOW findings were sensitivity gaps, not production defects.
  Direct owning-package tests now pin semantic-no-op CAS drift to exit 5
  `CodeEntryChanged` / `no-op-cas` while preserving editor bytes, and reject a
  mismatched artifact `Rel` as `CodeInvalidPlan` / `canonical-path`. Both pass
  together in 0.555s package. Independent review is **APPROVED** with no
  findings; the dedicated test-only checkpoint is `87c8763`.
- AU rev-0 added three test files and observer wiring but is **NEEDS
  REVISION**. It does not compile; six row targets are aggregate wrappers;
  PIB-540 is panic/recover rather than process-crash evidence; PIB-544 hook
  timing is not authoritative; PIB-545 lacks semantic claim/removal proof; and
  PIB-537 is not bound to PIB-511. Rev-1 is dispatched.
- AU rev-1 removes aggregate targets and panic modeling, but review returned
  **NEEDS REVISION**: PIB-545's in-memory AST path tries to reread synthetic
  filenames, so its baseline fails and mutations bite vacuously; two
  sensitivities also use raw sentinel strings. Rev-2 is bounded to a genuine
  same-validator fix.
- AU rev-2 static review is **APPROVED**, including the seven PIB-545
  mutations and four subprocess crash exits. Go validation is resource-blocked:
  a 10-minute gate never reached 80% free memory and ended at 68%; follow-up
  samples remained 66–71%. No Go process is active. Another independently
  owned `copilot --yolo --resume` process uses about 2.1 GB RSS and Chrome
  renderers use several GB; this session will not terminate them.
- After memory cleanup, the authoritative gate passed at 84% free. Executable
  rev-2 is **NEEDS REVISION** on seven concrete defects: PIB-544 wrong
  insertion linearization, PIB-545 empty baseline extraction, PIB-537 broken
  Cobra import in its synthetic graph, PIB-539 foreign-index fixture, PIB-542
  admitted `--all` fixture, PIB-543 wrong owned-path invocation, and missing AU
  CI owner-union registration. Rev-3 must correct fixtures, not expectations.
- Executable rev-3 is **NEEDS REVISION** on seven further roots, all fixed in
  rev-4 without weakening an expectation: the AU observer belonged to zero CI
  shards; PIB-545's baseline rejected shipped source because it looked for a
  lexical `allPending` in the removal's own function (and its mutation source
  could not parse); PIB-544 injected window 2 at `afterPurgeBlobRevalidate`,
  which is the PIB-550 final-syscall residual, not one of §9.7.2's four
  windows; PIB-537 previewed an abandon with no journal evidence; PIB-539
  expected exit 3 from a `list` that correctly exits 0 with `pending-remove`;
  PIB-542's clean controls reused the mixed fixture's selector identifiers on a
  shared, mutated archive; and PIB-543's restore fixture wrote to another
  slug's managed blob path. See "AU Implementation Session — rev-4".
- The rev-3 `PIB-542` "admitted `--all` fixture" finding is now traced to the
  PRD row itself, not to the test: the row's first fixture contradicted the
  accepted ADR-035 D16 sole-class admission rule. PRD rev-17 + ADR-035 rev-17
  (**no-decision erratum, acceptance pending joint review**) correct the row;
  the AU fixture must follow the corrected row — three no-class selectors
  refuse exit 3 zero-write, `--all --yes` is admitted with its blast-radius
  disclosure and exits 0 `purged`. The rev-3 production `SelectorAll` guard and
  its store-test edit are **reverted**.
- Rev-4 passed the parent's run: the AU suite in **12.081s** and the AU
  observer in **15.019s**. Joint review then returned six findings — a
  tautological PIB-538 post-command snapshot, an unarmed PIB-542 decode spy and
  an under-asserted admitted `--all` branch, an unverified claim about the
  PIB-543 mutating-refusal presentation, a rev-17 erratum scoped to one row
  when three are wrong, stale CURRENT files/evidence, and dead residual
  claims. Rev-5 folds all six. See "AU Implementation Session — rev-5".

**Suggested exact selectors:**

```
go test ./internal/cli -count=1 -timeout 8m -run '^TestS7ATMixedResiduePrepareRefusalContracts$'
go test ./internal/cli -count=1 -timeout 8m -run '^TestS7ATMixedResidueBlobRepairContracts$'
go test ./internal/cli -count=1 -timeout 8m -run '^TestS7ATGlobalOrphanPredicateContracts$'
go test ./internal/cli -count=1 -timeout 8m -run '^TestS7ATPendingRepairMultiClassContracts$'
go test ./internal/cli -count=1 -timeout 8m -run '^TestS7ATListStorageContracts$'
go test ./internal/cli -count=1 -timeout 8m -run '^TestS7ATWorkspaceRootOpenFailureContracts$'
go test ./internal/cli -count=1 -timeout 8m -run '^TestS7ATCoverageLedger$'
go test ./internal/cli -count=1 -timeout 8m -run '^TestS7ObservedATRegistrationAuthority$'
```

### AU Implementation Session — rev-1

AU `PIB-537`…`PIB-545` rev-1 addresses all rev-0 NEEDS REVISION findings.
Nine exact top-level nonaggregate test targets (I6/C2/G1) across three files
plus infrastructure updates to registration, CI guard and CI workflow:

- `internal/cli/prepare_s7_au_ledger_test.go` — 9-row ledger, AU kind counts
  I6/C2/G1, prior AM–AT partition invariants, observed-target uniqueness.
  Unchanged from rev-0.
- `internal/cli/prepare_s7_au_runtime_test.go` — PIB-537/538/539/541/542/543
  I-kind runtime tests. Rev-1 changes:
  - PIB-537: structurally binds to PIB-511 via `validateS7ARAbandonGateTable`
    (the same domain predicate/table parser/validator used by the AR guard),
    not identifier presence. Straight loop with labels, no nested `t.Run`.
  - PIB-538: straight loop, no nested `t.Run`. Five fixture classes asserted.
  - PIB-539: unchanged (already direct, no `t.Run`).
  - PIB-541: fixed `IntentArchiveRepairOrphanedResidue` →
    `IntentArchiveRepairUnreferencedResidue` (the undefined constant from
    rev-0). Straight loop, no nested `t.Run`.
  - PIB-542: straight loops in all selector iterations, no `t.Run`.
  - PIB-543: straight loop over five kinds, no nested `t.Run`. Device-node
    uses the accepted `s7ARInstallDeviceProbe` seam and is asserted, not
    skipped. FIFO creation failure is `t.Fatalf`, not `t.Skipf`. Both repair
    routes (remove+purge and restore-correct-blob) are executed and asserted
    for every kind.
- `internal/cli/prepare_s7_au_guard_test.go` — Rev-1 changes:
  - PIB-540: real subprocess crash via `TestS7AUCrashFixtureHelper` (child
    process reads env vars, installs the exact documented `go:linkname` store
    hooks — `beforePurgeIndexCAS`, `beforePurgeBlobRemove`,
    `afterPurgeBlobRemove`, `beforePendingTombstoneCAS` — and calls
    `os.Exit(97)` at each seam). Parent inspects disk invariants and reruns
    the same command. Four exact crash points, next-action asserted, invariant
    checked, no repeat removal needed. No `panic`/`recover`.
  - PIB-544: uses exact documented `go:linkname` hooks at four linearization
    points (`afterPurgeIndexDecode`, `afterPurgeBlobRevalidate`,
    `afterPurgeIndexRename` post-CAS, `afterPurgeBlobRemove`). Straight loop,
    no nested `t.Run`. Fifth fixture (new-selection) is inline.
  - PIB-545: combined validator performs real AST analysis:
    `s7AUExtractClassificationBranches` walks `ClassifyIntentArchiveTuple`'s
    AST return statements; `s7AUValidateClaimCASCoverage` inspects claim
    function AST for tombstoned-skip branch statements;
    `s7AUValidateRemovalDomination` finds `RemoveBlob` call sites and verifies
    all-pending guard co-location. Seven wrong-input sensitivity fixtures
    through the same validator; unchanged mutation fatal. No shallow string
    alternatives for structural properties.
- `internal/cli/prepare_s7_registration_test.go` — AU category constant,
  hosted budget (8m/4m/1m, rows 537–545, targets 9), observer test, budget
  validation want map, callsite-category binding, wrong-input mutations
  (wrong range, missing category, cross-category target swap). Unchanged from
  rev-0.
- `internal/cli/prepare_s7_ci_timeout_guard_test.go` — AU observer pattern and
  command constants, skip pattern extended, observer script updated, want list
  updated. Unchanged from rev-0.
- `.github/workflows/ci.yml` — skip pattern extended with `|AU`, AU observer
  command added to s7-observers job. Unchanged from rev-0.

**Rev-1 status: UNTESTED.** Files are written but `go test` has not been run.
The parent must validate compilation and all selectors before review.

**Residual design notes:**
- PIB-543 device-node kind reuses `s7ARInstallDeviceProbe` /
  `s7ARDeviceProbeStorage` from the AR runtime test.
- PIB-537 now depends on `validateS7ARAbandonGateTable` from the AR guard;
  if the gate table structure changes, both PIB-511 and PIB-537 update
  together.

### AU Implementation Session — rev-2 (bounded PIB-545)

Rev-2 is a bounded fix to `internal/cli/prepare_s7_au_guard_test.go` only,
addressing three PIB-545 findings from rev-1 review:

**1. PIB-545 `s7AUNodeString` synthetic filename read failure (F1).**
The old `s7AUNodeString` called `os.ReadFile(start.Filename)` using the
line/column position from `fset.Position()`. When called from the
`combinedValidator` path (which parses synthetic source strings via
`s7AUParseSource` with filenames `"store.go"` / `"cli.go"`), the read
would fail because no such file exists on disk, and the error was silently
converted to a `<read error: ...>` string instead of propagated.

Fix: Introduced `s7AUSourceRegistry` — a `map[string][]byte` keyed by
`token.File.Name()`. All parse functions (`s7AUParsePackage`,
`s7AUParseSource`) register their source bytes in the registry.
`s7AUNodeString` now uses `fset.File(node.Pos())` to get the `token.File`,
looks up bytes from the registry by `tokFile.Name()`, and extracts the exact
byte slice `[tokFile.Offset(node.Pos()):tokFile.Offset(node.End())]`.
Returns `(string, error)` — read errors are explicit, never converted to
strings. All callers updated to handle the error.

**2. Raw `strings.Contains` sentinel detection removed (F2).**
Rev-1 mutations 6 (`claim-skips-tombstoned-references`) and 7
(`absent-blob-path-leaves-retained`) replaced Go identifiers with
`SKIP_TOMBSTONED_WIRE` / `ONLY_PENDING_WIRE` sentinel strings, then the
`combinedValidator` detected those sentinels via `strings.Contains(storeSrc, ...)`.
This is raw string matching, not structural validation.

Fix: Both sentinels removed. Two new AST validators added:
- `s7AUValidateClaimIteratesAllWireStates` — walks
  `setIntentArchiveHashState`'s range body and verifies that no
  `if-continue` branch references a specific `IntentArchiveWire*` constant
  beyond the legitimate `== state` comparison. Detects mutations that add
  tombstoned-skip branches.
- `s7AUValidateAbsentPathTombstonesAll` — walks the same function and
  verifies no skip branch references `IntentArchiveWireRetained`, which
  would cause the tombstone path to leave retained refs behind.

Mutations 6 and 7 changed from sentinel-injection to structural mutations:
- Mutation 6 now appends `|| replacement.WireState() == IntentArchiveWireTombstoned`
  to the skip condition, which the AST validator detects.
- Mutation 7 now appends `|| replacement.WireState() == IntentArchiveWireRetained`
  to the skip condition, which the AST validator detects.
Both produce valid compilable source shapes that fail structural checks.

**3. Mutation verification and error class assertions (F3).**
- Every mutation verifies `mutatedStore != storeSource` (or CLI equivalent)
  before running the validator — unchanged source is fatal.
- Baseline positive runs before the first mutation.
- Post-mutation unchanged baseline positive runs after all mutations.
- Each `sensitivityFixture` has a `wantClass` field; after the validator
  rejects the mutation, the test asserts the error message contains the
  expected class substring (`"missing classification"`, `"all-pending"`,
  `"claim CAS"`, `"absent-blob"`).

**Files changed:**
- `internal/cli/prepare_s7_au_guard_test.go` — all changes confined to
  `TestS7AUClassificationAuthorityGuard` and its helpers.

**Rev-2 status: UNTESTED.** Syntax-checked (brace/paren balance verified)
but `go test` has not been run per dispatch constraints (no Go commands).
The parent must validate compilation and the
`TestS7AUClassificationAuthorityGuard` selector before review.

**Residuals:**
- The structural validators for mutations 6/7 are specific to
  `setIntentArchiveHashState`'s current control-flow shape. A refactor that
  moves the skip logic to a helper function would require validator updates.
- PIB-545 sensitivity mutations still use `strings.Replace` for mutations
  1–5; those target classification disposition names which are unlikely to
  change identifier shape.

**Suggested exact selector:**

```
go test ./internal/cli -count=1 -timeout 8m -run '^TestS7AUClassificationAuthorityGuard$'
```

**Suggested exact selectors:**

```
go test ./internal/cli -count=1 -timeout 8m -run '^TestS7AUAbandonBooleanDomainContracts$'
go test ./internal/cli -count=1 -timeout 8m -run '^TestS7AUManualModeRefusalContracts$'
go test ./internal/cli -count=1 -timeout 8m -run '^TestS7AURetainedPendingSameHashContracts$'
go test ./internal/cli -count=1 -timeout 8m -run '^TestS7AUCrashInjectionContracts$'
go test ./internal/cli -count=1 -timeout 8m -run '^TestS7AUListDoctorMultiObservationContracts$'
go test ./internal/cli -count=1 -timeout 8m -run '^TestS7AUDisjointSelectorRefusalContracts$'
go test ./internal/cli -count=1 -timeout 8m -run '^TestS7AUCorruptBlobRouteContracts$'
go test ./internal/cli -count=1 -timeout 8m -run '^TestS7AUSameHashInsertionWindowContracts$'
go test ./internal/cli -count=1 -timeout 8m -run '^TestS7AUClassificationAuthorityGuard$'
go test ./internal/cli -count=1 -timeout 8m -run '^TestS7AUCoverageLedger$'
go test ./internal/cli -count=1 -timeout 8m -run '^TestS7ObservedAURegistrationAuthority$'
```

### AU Implementation Session — rev-3 (seven concrete defect fixes)

Rev-3 addresses the seven concrete defects from rev-2 executable review.

**1. PIB-545 empty baseline extraction (F1).**
`s7AUExtractClassificationBranches` looked for `*ast.ReturnStmt` but
`ClassifyIntentArchiveTuple` assigns fields to a local `result` variable and
returns it. Branches extracted = 0. Fix: extract `*ast.AssignStmt` nodes where
the LHS is `result.Disposition`, matching the production code's assignment
pattern. Required disposition check list updated from Code names
(RecoveryPending, BlobDangling, BlobCorrupt, PurgeEvidenceDivergent) to
Disposition names (PendingRemove, CorruptObject, DanglingReference). Sensitivity
mutations updated to target Disposition constants. All seven same-validator
classes use the same combined validator. In-memory source registry preserved.

**2. PIB-537 broken Cobra import (F2).**
`s7AUInstallDeterministicProvider` changes HOME to a temp dir. The type graph
builder's `go list -export` for cobra fails with GOPROXY=off because the Go
cache is unreachable. Fix: move `validateS7ARAbandonGateTable` call before
`s7AUInstallDeterministicProvider(t)` so HOME is still real during type graph
construction. The gate table validation doesn't need the provider. Shared AR
authority (`validateS7ARAbandonGateTable`, `s7ARBuildPreAbandonProgram`) is
unchanged.

**3. PIB-539 foreign-index fixture (F3).**
The test built the archive under `slug2` (from `intentArchiveCLIWorkspace`),
then copied it to `prepSlug` (from `prepareS4Workspace`). The index contained
`slug2`'s identity, which the validator rejected as `archive-index-foreign`.
Fix: build the retained+pending archive directly under `prepSlug` using
`intentArchiveCLIGeneration(t, prepSlug, ...)`. No cross-slug copy; no foreign
index.

**4. PIB-542 admitted --all fixture (F4).** **Superseded by rev-5 — this
diagnosis and its production change are withdrawn.** Rev-3 read the admitted
`--all` as a production gap and added an
`equalStringSets(selected, inspection.Classes[0].Hashes)` guard to the
`SelectorAll` admission case, with the store test
`TestIntentArchiveAllSelectsEveryReferenceAndBlob` updated to match. ADR-035
D16 admits `--all --yes` exactly when the chosen class is the archive's **only**
class, which is what the shipped code already did; the defect was in the
`PIB-542` row, not in `admitIntentArchiveRepair`. Rev-5 reverted both the
production guard and the store-test edit; the PRD/ADR rev-17 erratum corrects
the row instead.

**5. PIB-543 wrong owned-path invocation (F5).**
Prepare/list readers route owned state as recovery-pending (exit 3). The test
incorrectly expected exit 6 (divergent) from `prepare` on owned corrupt blob.
Exit 6 divergent is the response from `purge --yes` on a corrupt owned blob.
Fix: assert `prepare` exits 3 with `recovery-pending`, then separately assert
`purge --yes` exits 6 with `archive-purge-evidence-divergent`. Doctor observer
now captures actual exit code from `runDoctorCLI` instead of hard-coding
`obsCode = 3`.

**6. PIB-544 wrong insertion linearization (F6).**
Hooks did not match the four PRD-documented linearization points. The
`between-reread-and-CAS` window used `afterPurgeBlobRevalidate` (post-
revalidation) instead of `beforePurgeIndexCAS` (pre-CAS). Fix: four windows now
use `beforePurgeIndexCAS`, `afterPurgeBlobRevalidate`, `beforePurgeBlobRemove`,
`beforePendingTombstoneCAS` exactly as documented. Fifth fixture (new-selection)
uses `beforePurgeIndexCAS`. References to `afterPurgeIndexDecode`,
`afterPurgeIndexRename`, `afterPurgeBlobRemove` removed from PIB-544 code;
those hooks remain available for PIB-540 crash injection.

**7. CI owner-union registration.**
Already present: AU observer pattern, CI workflow entry, skip pattern, and
registration test were all implemented in rev-0. No change needed.

**Files changed:**
- `internal/store/intent_archive.go` — PIB-542 SelectorAll admission fix
  (**reverted in rev-5**)
- `internal/store/intent_archive_purge_test.go` — Updated
  `TestIntentArchiveAllSelectsEveryReferenceAndBlob` for new admission behavior
  (**reverted in rev-5**)
- `internal/cli/prepare_s7_au_guard_test.go` — PIB-544 hook alignment, PIB-545
  branch extraction rewrite
- `internal/cli/prepare_s7_au_runtime_test.go` — PIB-537 gate validation
  ordering, PIB-539 direct archive build, PIB-543 owned-path fix

**Rev-3 status: UNTESTED.** The parent must validate compilation and all
selectors before review. No Go commands were run.

**Residuals:**
- `copyArchiveTree` in `prepare_s7_au_runtime_test.go` was left dead after the
  PIB-539 fix; rev-4 removed it, so this residual is closed.
- PIB-545 mutations 1-5 now target Disposition constant names rather than
  Code/RepairClass names. The mutation strings are chosen so each replaces a
  unique required disposition.
- The store test update in `intent_archive_purge_test.go` is **withdrawn** with
  the rev-3 admission change; `internal/store/intent_archive_purge_test.go` is
  byte-identical to `origin/main` again after rev-5.

### AU Implementation Session — rev-4 (seven executable failure roots)

Rev-4 addresses the runtime failures rev-3 left open. Every fix corrects the
fixture, seam or guard that produced the failure; no expectation was weakened,
and no product behaviour changed except the PIB-542 `SelectorAll` admission
already landed in rev-3.

**1. AU observer owned by zero CI shards.**
`validateS7ARPartitionTestOwners` builds its shard list from the observer
patterns, but `s7CIAUObserverPattern` was never added, so
`TestS7ObservedAURegistrationAuthority` matched `s7CIARPartitionPattern` and
belonged to 0 shards. Fix: the AU observer regexp is now a shard, between AT
and the AR legacy shards. The workflow command and skip pattern were already
present, so only the owner union changed.

**2. PIB-545 baseline rejected the shipped source ("no blob removal guarded
all-pending").**
`s7AUValidateRemovalDomination` scanned the *enclosing function's source text*
for the literal `allPending`. Production spells the predicate
`allIntentArchiveReferencesPending` and puts the syscall behind the
`removeIntentArchiveBlob` helper, so the lexical scan could never match, and
the paired mutation (`allPending` → `true`) changed nothing. The old validator
also concatenated whole packages into one synthetic file, which cannot parse
(repeated `package` clauses), so the mutation arm was unreachable anyway.

Rev-4 replaces it with a structural authority walk over two real single-file
sources (`internal/store/intent_archive.go`, `internal/cli/prepare_publish.go`):

- the removal syscall (`storage.RemoveBlob`) is located by call graph; any call
  site outside the store authority is a failure;
- its enclosing wrapper must gate on a boolean parameter whose negation
  terminates before the syscall (`removeIntentArchiveBlob`'s `authorized`);
- every call site of that wrapper must pass an argument that resolves to a
  total authority predicate (`allIntentArchiveReferencesPending` or
  `intentArchiveHashUnreferenced`) directly, through its binding, or — for the
  literal `true` — through a terminating `if !<authority>(…)` guard that
  dominates the call in the enclosing function's statement tree;
- both predicates must iterate generations and replacements and return false.

The combined validator also derives the classification map from
`ClassifyIntentArchiveTuple` assignments, asserts the ownership branch returns
before any non-owned classification and routes unidentifiable blobs, asserts
the owned unidentifiable arm of `intentArchiveUnidentifiablePurgeError` carries
only exit-6 `archive-purge-evidence-divergent`, and asserts
`setIntentArchiveHashState`'s skip condition excludes neither tombstoned (claim
totality) nor retained (absent-blob totality) references.

Seven same-validator mutations, all valid Go against real source:
owned→mixed disposition, both all-pending gates degraded to
`intentArchiveHashHasState`, corrupt→dangling disposition, the orphan path's
`authorized :=` bound to `true`, the owned divergent constructor rewritten to
exit-3 `archive-blob-corrupt`, and the two wire-state skip exclusions. Their
expected classes are `missing classification`, `blob removal in
executeIntentArchivePurgeHash`, `missing classification`, `blob removal in
executeIntentArchiveOrphanPurge`, `exit-6 route`, `claim CAS` and
`absent-blob`. Both baselines (before and after the mutation sweep) must pass.

**3. PIB-544 windows were injected at the wrong linearization points.**
Rev-3 drove window 2 through `afterPurgeBlobRevalidate`, which fires *after*
the pre-removal revalidation: the removal legitimately proceeds there, so
asserting "no removal" contradicted the disclosed final-syscall residual the
PRD assigns to PIB-550. Rev-4 mirrors the store fixture
`TestRecoverPendingPurgeInsertionWindowsPIB544` seam for seam and outcome for
outcome:

| window | CLI seam | store subtest | outcome |
|---|---|---|---|
| before the claim's re-read | `CaptureIndex` spy armed after `PreflightIndexCAS` | `before-claim-reread-is-included` | exit 0 `recovered`, blob removed, all three references tombstoned |
| between the re-read and its CAS | `beforePurgeIndexCAS` | `between-reread-and-claim-cas-is-partial` | exit 5 `purge-partial`, `pending-recovery-then-completion`, `pending_hash`, no removal |
| between the CAS and the revalidation | `afterPurgeIndexRename` | `between-claim-cas-and-revalidate-is-partial` | exit 5, same progress record, no removal |
| after the removal | `beforePendingTombstoneCAS` | `after-removal-before-tombstone-retries-absent-path` | exit 5, blob already unlinked, no completed hash; retry takes the absent path with zero removals |

The window-1 seam is a storage spy rather than a hook because the store fixture
inserts before the *second* capture — after the recovery preflight — and an
`afterPurgeIndexDecode` insertion would land before `PreflightIndexCAS` and
change the outcome. Each partial window's sanitized retry must reach exit 0
with every reference (including the inserted one) tombstoned. The fifth fixture
now uses a **new selection** on a clean archive and asserts exit 3
`archive-purge-index-changed` with a tree byte-identical to the externally
inserted index.

**4. PIB-537 bare abandon had no evidence.**
`prepare <slug> --abandon-transaction` correctly exits 3 `no-pending-transaction`
when the lane is empty; §6.6's preview contract needs journal evidence. Rev-4
writes a real journal with `s6WriteJournalFixture` for the bare and `=true`
arms and asserts exit 0, mode `abandon`, outcome `abandon-planned`, `moved`
exactly `[journal.json]`, a byte-identical tree and no `abandoned-` directory.
`--abandon-transaction=false` runs on a separate clean workspace (exit 0, mode
`generate`, outcome `no-op`, `beforeAbandonBranch` never fires). The
`--check --abandon-transaction=false` mutex asserts pflag's own
`[check manual regenerate abandon-transaction]` text and binds its expected
exit to the abandon gate table's own row 1 rather than to prose; the structural
PIB-511 binding through `validateS7ARAbandonGateTable` is unchanged and still
runs before the provider fixture changes `HOME`.

**5. PIB-539 expected the wrong list exit.**
An owned hash produces no repair class (`case owned:` files it under
`PendingHashes`), so `intentArchiveListExit` returns 0. Rev-4 expects exit 0
with both same-hash entries rendered as `pending-remove` carrying the exact
`--blob <h> --yes` repair, retry and `workspace-root` retry CWD, no orphans and
no corrupt objects. The raw-substring "no dangling/mixed/corrupt" scan was
replaced by exact refusal-code and decoded-field checks, because the list
schema always emits a `corrupt_objects` array. `prepare` (exit 3
`recovery-pending`) and the purge preview (exit 0 `recovery-required`) are
unchanged.

**6. PIB-542 clean controls were not clean.**
The control loop reused the *mixed* fixture's `h₃` hash and `id₃` generation
against a different archive and ran all four mutating selectors over one
workspace. Rev-4 builds each control independently with unique canonical
artifact and generation data, proves the control is clean with a `list` before
any selector runs, derives every selector argument from that control, and
asserts all four succeed — plus that the disjoint `h₂` blob survives the
`--blob`/`--generation` controls.

**7. PIB-543 restore fixture pointed at another slug's blob path.**
The restore-alternative workspace reused `blobRel` from the first slug, so the
managed blob parent did not exist under the alternative root and the symlink
kind failed with ENOENT. Rev-4 derives `blobRel` from the alternative slug and
ensures the parent exists. The owned half now asserts the confirmed purge exits
exactly 6 with refusal code `archive-purge-evidence-divergent`, a divergence
record naming the hash and blob, and no exit-3 code anywhere; the doctor
observer runs the real command and asserts its actual exit code (0 —
warning-only) with exactly one `corrupt-object` D9 finding whose remediation
carries the `rm -rf -- '<blob>'` line and the retry command. The forbidden-word
scan is now a case-sensitive word-boundary check over the procedure text with
the blob path elided, so it cannot be satisfied by padding and does not confuse
the required "Git history" caveat with the `git` command.

**8. Dead code.** `copyArchiveTree` and the now-unused
`s7AUAfterPurgeBlobRevalidate` linkname were removed, along with the stale
error-class comment block in the old PIB-545 validator. No `t.Run`, skip or
panic was introduced; all nine ledger targets remain top-level test bodies.

**Files changed:**
- `internal/cli/prepare_s7_ci_timeout_guard_test.go` — AU observer shard
- `internal/cli/prepare_s7_au_runtime_test.go` — PIB-537/539/542/543 fixtures
  and expectations, clean-control builder, forbidden-word helper
- `internal/cli/prepare_s7_au_guard_test.go` — PIB-544 window realignment,
  PIB-545 structural authority validator and its seven mutations

**Rev-4 status: UNTESTED.** No Go command was run in this session (no build, no
vet, no gofmt, no test) and nothing was staged, committed or pushed. Symbols
were verified by static scan only: every helper, hook, constant and report
field referenced by the AU files was grepped to its declaration.

**Residuals:**
- The whole AU suite still needs a resource-gated `go test ./internal/cli`
  run plus `gofmt -l`; formatting of the rewritten blocks is hand-maintained.
  (Superseded by rev-5: the parent ran the AU suite and the observer; see
  "AU Implementation Session — rev-5".)
- PIB-543's PRD row says every observer "exits 3"; the product's doctor is
  warning-only (`DoctorExitCode` returns 0 when only warnings exist), so rev-4
  asserts the real exit 0 and the rendered corrupt route. **Closed by the
  rev-17 erratum**, which corrects the row to product truth.
- PIB-543's PRD row also expects the printed `rm -rf --` procedure from every
  observer. Rev-4 recorded this as "`prepare`/`--regenerate`/`--manual` emit
  the short `prepareArchiveRepairText` remediation instead". **That reading is
  wrong and is withdrawn by rev-5**: `prepareArchiveRepairText`'s short string
  is the `refusePrepare` default, but `prepareStoreArchiveFailure`'s
  `IntentArchiveCodeBlobCorrupt` arm overwrites it with the full destructive
  warning + `rm -rf -- <blobRel>` + literal purge retry + restore alternative +
  Git-history caveat (`internal/cli/prepare_publish.go`). Rev-5 asserts that
  exact string on all three surfaces; the rev-17 erratum records the
  presentation difference (quoting and the absent `retry_cwd` pair) instead of
  a missing route.
- PIB-544's PRD row names `afterPurgeBlobRevalidate` among the four drivers;
  rev-4 deliberately does not use it, because that seam is the PIB-550
  final-syscall residual. Window 1 uses a `CaptureIndex` storage spy for the
  same reason the store fixture uses `before:capture-index`. **Closed by the
  rev-17 erratum**, which replaces the row's driver list with the store
  fixture's authoritative seam mapping.
- PIB-545's guard is source-structural: it proves the removal gate exists and
  dominates, not that the predicate is semantically correct at runtime. The
  runtime proof of that property stays with PIB-539/PIB-540/PIB-544.

### AU Implementation Session — rev-5 (joint review findings)

Rev-5 is a bounded fold of the joint review of rev-4. It changes one test file
(three test bodies), three tracking/contract documents, and reverts the last
two rev-3 store edits; it weakens no expectation and adds no product behaviour.

**1. PIB-538 tautological post-command snapshot removed.**
`TestS7AUManualModeRefusalContracts` re-read the feature directory twice
*after* the command and compared the two reads, which can never fail. The
whole-`.tpatch` pre/post byte comparison two statements above it is already the
exact zero-write proof, so the block is deleted rather than repaired. The
now-dead `report.Refusal != nil` guard in the forbidden-word loop — the nil
case already fatals above it — is dropped with it. The write-spy count, the
refusal code per fixture class, the forbidden-word ban and the leak check are
unchanged.

**2. PIB-542 decode spy armed and the admitted branch asserted.**
`TestS7AUDisjointSelectorRefusalContracts` now arms the existing
`s7ASAfterPurgeIndexDecode` linkname seam immediately before each of the four
main selector invocations and restores the previous hook on the next statement,
with no failure path in between; the counter only counts decodes of this slug's
own `index.json` rel path, and a selector that decides without one is fatal.
That is the row's "a decode spy confirms the whole-index X11 scan ran before
each of the three refusals **and** before the admission".
The corrected `--all` branch now asserts the whole admitted outcome: exit 0,
`outcome: "purged"`, `action: "none"`, no refusal, `remaining_repairs` absent,
`blast_radius` byte-equal to the shipped constant
(`s7AUAllBlastRadius` — tombstones every reference/removes every blob, the
unconfirmed preview default and the narrower repeated-`--blob` alternative in
one string), both the `h₂` and `h₃` blobs gone from disk, and no workspace-root
leak. The disclosure is carried by the JSON `blast_radius` field, so no human
(non-JSON) preview run is needed to observe it. The three refusal branches keep
their whole-tree byte equality and now also assert that both managed blob paths
still exist.

**3. PIB-543 bound to the observed refusal presentation.**
The three mutating readers' actual `remediation` was read out of
`prepareStoreArchiveFailure` rather than assumed: they emit the full route.
`TestS7AUCorruptBlobRouteContracts` now asserts the exact composed string —
destructive warning, `rm -rf -- <repo-relative blobRel>` (unquoted), the
literal `tpatch feature intent-archive purge <slug> --blob <h> --yes`, the
restore alternative and the Git-history caveat — for `prepare`,
`--regenerate` and `--manual`, and asserts those refusals carry no
`retry`/`retry_cwd` pair, which is where they differ from `list`'s structured
`retry_cwd: "workspace-root"`. `list` keeps its quoted-procedure, retry and
`retry_cwd` assertions; `doctor` keeps its real exit code (0, warning-only)
with exactly one `corrupt-object` D9 finding carrying the quoted `rm -rf --`
line and the retry. A `--orphans` scan now runs over **every** observer's
stdout, not just the procedure prose. The rev-17 erratum records the
presentation split so the row and the product agree.

**4. PRD/ADR rev-17 erratum widened to its true scope.**
The erratum now covers exactly `PIB-542`, `PIB-543` and `PIB-544` — not one
row. `PIB-543` is corrected for the doctor warning-only exit 0 beside the
exit-3 `prepare`/`--regenerate`/`--manual`/`list` observers and for which
surface carries which presentation of the one route; `PIB-544`'s incorrect
driver list is replaced by the authoritative seam mapping the store fixture
`TestRecoverPendingPurgeInsertionWindowsPIB544` already drives, with
`afterPurgeBlobRevalidate` excluded as PIB-550's fifth-window seam. No decision,
normative rule, admission condition, precedence rank, emitted exit code, state,
vocabulary token or count changes; ADR-035's D16 normative text is untouched
and only its companion-row record moves. The matrix stays at **567** rows with
**thirty-six** semantic guards, and the §18.1 amendment ledger, the revision
rows, both headers and both bylines were updated together.

**5. Store surface reduced to one uncommitted production change.**
The rev-3 `SelectorAll` admission guard was already reverted; rev-5 also
reverted the whitespace-only churn in
`internal/store/intent_archive_purge_test.go`, so that file is byte-identical
to `origin/main`. The **only** surviving store change is the uncommitted
three-line clamp in `publishIntentArchiveIndex`
(`internal/store/intent_archive.go`): an index CAS that did **not** commit and
whose inherited typed error carries `ExitClass 5` is clamped to **3** before
the code is rewritten to `archive-purge-index-changed`. Without it the
PIB-544 fifth fixture — an external insertion before a new selection's first
write — inherits `intentpub`'s `entry-changed` exit 5 although nothing was
written, contradicting the row's exit 3 whole-tree-snapshot expectation;
`intentArchiveExitAfterMutation(false)` is already 3, so the clamp restores the
store's own uncommitted-failure exit class rather than inventing one.

**Files changed in rev-5:**
- `internal/cli/prepare_s7_au_runtime_test.go` — PIB-538 snapshot block
  removed; PIB-542 decode spy, admitted-branch and blob-path assertions plus
  the `s7AUAllBlastRadius` constant; PIB-543 exact mutating-route assertions,
  structured-retry check and per-observer `--orphans` scan.
- `docs/prds/PRD-prepare-intent-bundle.md` — rev-17 status/acceptance/byline,
  the rev-17 revision row, the §18.1 amendment ledger entry, and the `PIB-543`
  and `PIB-544` matrix rows.
- `docs/adrs/ADR-035-intent-bundle-publication-and-history.md` — rev-17 status,
  acceptance note, byline and revision row, all companion-row only.
- `docs/handoff/CURRENT.md` — this section, the erratum intake paragraph, the
  rev-3/rev-4 superseded claims and the Status/Next Steps lines.
- `internal/store/intent_archive_purge_test.go` — reverted to `origin/main`.

**Rev-5 status: TESTED AND APPROVED.** The full AU/store/CI suite passes in
12.291s / 0.251s; the nine-target observer passes in 15.398s package. `gofmt`,
affected vet and CLI build pass. Final review confirms every prior finding is
closed and explicitly approves the `publishIntentArchiveIndex` clamp.
- AU production/test/observer/CI code is checkpointed at `bbf093e` by
  explicit-path staging with the required trailer.
- CI 33151661769 passed Windows and both observer jobs; Ubuntu/macOS failed
  only rev-16's stale document-region hashes after rev-17. The guard now
  allowlists exact rev-17 header/history/ledger/AU regions and expects matrix
  changes `PIB-402/403/425/542/543/544`; baseline plus all sensitivities pass.
- Corrected CI 33153117728 is green on Ubuntu, macOS, Windows and both
  observer jobs. AU is durably complete at I6/C2/G1; cumulative S7 is
  151/173. AV (`PIB-546`…`PIB-551`) is implemented at rev-1 in the worktree
  and awaiting review; with it S7 would be 157/173.

**Residuals:**
- PIB-543's exact-string assertion binds the three mutating surfaces to
  `prepareStoreArchiveFailure`'s current composition. If that message is ever
  reworded, the test fails loudly rather than silently, which is the intent,
  but the reword must then be reflected in the `PIB-543` row too.
- The `publishIntentArchiveIndex` clamp is approved production code for
  PIB-544's zero-write new-selection CAS race.

### AV Implementation Session — rev-1

**Scope**: `PIB-546`…`PIB-551` (PRD §18.49), dispatched from the post-AU
baseline `d6aa482` (`docs: close S7 AU and dispatch AV`). Manifest kinds parsed
from the PRD are `546 G`, `547 I`, `548 I`, `549 G`, `550 C`, `551 G` — the
ledger asserts `I` 2 / `C` 1 / `G` 3, which matches §18.52's "rev-11 added
`I` 2, `C` 1 and `G` 3". Six direct top-level exact targets, no skips and no
proxies; every guard reuses the AU/AT/AS helpers and the shipped injection
seams.

**Ledger, observer and CI**

- `internal/cli/prepare_s7_av_ledger_test.go` registers the six rows against
  six top-level bodies, resolves each through the shared AST resolver, asserts
  the kind counts and re-asserts that the accepted AM–AU partitions
  (54/12/34/23/15/10/6/9) are unchanged.
- `s7ObservedCategoryAV` is added at **8m outer / 4m inner / 1m cleanup**,
  `first: 546`, `last: 551`, `targets: 6`, in both the live budget map and the
  independent `want` map the budget validator compares against. Three new
  wrong-input arms (`wrong-av-range`, `wrong-av-target-count`,
  `missing-av-category`) plus a cross-category arm (AV must reject AU's
  targets) keep the budget guard sensitive.
- CI: `AV` joins the full-suite `-skip` partition pattern, a dedicated
  `TestS7ObservedAVRegistrationAuthority` step joins the `s7-observers` job,
  and the guard gains `s7CIAVObserverPattern` /
  `s7CINonWindowsAVObserverCommand`, the observer-script tail, the
  observer-command partition list, the **owner** regexp set, the
  **recursion** exclusion list (the observed-registration regex must not match
  the AV observer itself) and two new workflow sensitivities
  (`observer-av-command-removed`, `observer-av-command-failure-masked`).

**PIB-546 — `TestS7AVRecoveryBeforeGlobalScanGuard` (G)**

`s7AVValidateRecoveryOrdering` derives the ordering from control flow, not
prose. It parses `internal/store/intent_archive.go`,
`internal/cli/feature_intent_archive.go` and `internal/cli/prepare_publish.go`;
derives the archive-mutating primitives as the three writing storage methods
(`PublishBlob`, `CASIndex`, `RemoveBlob`) and the global X11 scan as
`InspectIntentArchive`; resolves the CLI's package-level store indirection vars;
derives the command entry points as the `run*` runners no other runner
delegates to; and then threads a "has the global scan already run on this path"
fact through the structured control flow (if/else, switch/select, loops,
returns, inlined callees, cycle-guarded and memoised). Every mutation the fact
does not dominate is a finding attributed to the outermost store API on the
stack. The baseline yields **exactly one** route — `RecoverPendingPurge` — from
**exactly one** entry point, the confirmed purge. The guard additionally proves
the recovery is restricted to `PendingIntentArchiveHashes` and iterates that
set, that the `recovered` branch of `runFeatureIntentArchivePurgeConfirmed`
terminates with a return, and that the single `intentArchivePlanPurge` call sits
after it.

The runtime half runs the derived property end to end on an archive holding a
removal-pending `h₁` and an unrelated mixed `h₂`: `purge <slug> --orphans --yes`
and `purge <slug> --blob <h₂> --yes` both exit **0** `recovered`, finalize `h₁`
(blob gone, every reference tombstoned), leave `h₂`'s blob byte-identical and
its retained/tombstoned pair untouched, and carry the
`recovered-prior-transaction` advisory rather than processing the selector. The
`--orphans` rerun then performs the full global scan and refuses exit 3
`archive-index-storage-inconsistent`; the `--blob <h₂>` rerun is admitted and
repairs it. A mutating `prepare` over the same archive refuses exit 3
`recovery-pending` before each run.

Three same-validator semantic sensitivities, all valid Go with correct names:
running `store.CaptureIntentArchive` before the recovery (→ "no path mutates
before the global scan"), demoting the recovered branch's `return` to `_ =`
(→ "continues into the selector"), and granting the same exception to the
mutating prepare by calling `store.RecoverPendingPurge` before its
`recovery-pending` refusal (→ two entry points).

**PIB-547 — `TestS7AVPrintedRemovalProcedureContracts` (I)**

Both printed procedures are executed verbatim from the workspace root, over
five object kinds at the managed blob path: a hash-wrong regular file, a symlink
to a file **outside** `.tpatch/`, a directory containing **two** files, a real
FIFO, and a device node through PIB-560's injected file-kind seam
(`s7ARInstallDeviceProbe`). The divergent-blob procedure is driven through the
owned exit-6 route (`s7AROwnedDivergenceFixture` + `s7ARReplaceArchiveBlobKind`);
the corrupt-object procedure is driven through `list` on an independent
non-owned fixture of the same kind.

`s7AVValidatePrintedRemoval` is the single validator for the ten real
observations, the must-pass caveat fixture and the three must-fail fixtures. It
tokenises rather than scans: the removal must split to exactly
`["rm","-rf","--",<managed blob path>]` (no wildcard, no second path, no
directory prefix above the blob, no `-i`/`-I`/`--interactive`), the destructive
warning must appear above it, §10.7 rule 2's closed `argv[0]` allowlist
`{tpatch, rm}` is applied to every structural command line **and** to any
remaining line in invocation shape, and §10.7 rule 3 fails a forbidden token
only as inline code or immediately followed by an option- or path-shaped word.
Each printed line is then run with `sh -c` from the workspace root: it exits 0
silently, the path is absent afterwards, the symlink's target file is
byte-identical, and the sibling blob is byte-identical. The index-divergence
form names no removal command, no blob field, no `rm -rf` and no managed blob
path on either surface.

Sensitivities: rev-10's `cp` + plain `rm` pair, `rm -rf <path>` without the `--`
terminator, and a correct `rm -rf --` beside rev-11's own
`cp -R … cp -P … git show` prose. The must-pass fixture is §9.6.2's caveat
"it is still in this repository's Git history", which rev-12's prose-substring
rule would have failed.

**PIB-548 — `TestS7AVMultiInstanceRepairClassContracts` (I)**

Five real archives built by one spec-driven fixture builder. (a) three globally
unreferenced tombstone-beside-blob residues: `list` renders all six
observations (three generation entries + three orphan rows) carrying the
**identical** class repair `purge <slug> --orphans --yes`, `doctor --check D9`
emits exactly **one** `unreferenced-residue` finding naming all three paths with
that same remediation, and one `--orphans --yes` exits 0 removing all three with
`index.json` byte-identical. (b) two dangling hashes and (c) two mixed hashes:
each instance renders the same class-total repeated-`--blob` command, `doctor`
emits one finding per class, and one invocation completes (b) with **zero**
removals and (c) with **two** removals (proved by a `RemoveBlob` spy), after
which `list` exits 0 and an ordinary mutating `prepare` proceeds. (d) classes
(a)+(c) together at the **rev-12 sequential outcome**: `--generation <id> --yes`,
`--all --yes` and the partial `--blob <h₁> --yes` each refuse exit 3 with a
byte-identical `.tpatch` tree and a `remaining_repairs` object rendering **both**
routes in rank order; `--orphans --yes` is then admitted at exit 0
(`purged`/`none`) with the three residue blobs gone, `index.json` byte-identical,
both mixed blobs byte-identical, `repaired_class: unreferenced-residue`,
`stages_remaining: 1` and a single `mixed-reference` stage; the rerun over both
mixed hashes exits 0 with nothing remaining. (e) the same archive plus one real
`corrupt-object` (a directory at a managed blob path under a retained
reference): **every** selector — `--orphans --yes` included — refuses exit 3
zero-write with `archive-blob-corrupt` and a `next_stage` of kind
`manual-prerequisite` / class `corrupt-object`.

**PIB-549 — `TestS7AVRepairAdmissionPredicateGuard` (G)**

`s7AVValidateAdmissionPredicate` derives §9.3.1's predicate from
`admitIntentArchiveRepair` and `InspectIntentArchive`: the corrupt-first
precondition must exist, must not be conditioned on the observed class set's
cardinality, must terminate and must carry no `else`; no gate may compare the
class set against any cardinality other than zero (the "nothing is wrong" early
exit); the single closed selector switch must admit exactly
`unreferenced-residue` for `--orphans`, exactly `{dangling, mixed}` for
`--blob`, nothing for `--generation`, and only a **sole** observed class for
`--all`; the `--orphans`/`--blob` arms must require set **equality** against the
class report's hashes and must not condition on an instance count; and the
class-collapse switch must decide ownership first and the unidentifiable
observation second, unconditioned on liveness, assigning at most one class per
hash.

Six same-validator semantic sensitivities: rev-10's sole-inconsistency rule,
rev-11's sole-class rule, a partial-coverage admission, `--all` admitted beside
a second class, a precedence change that routes a mixed-plus-unidentifiable hash
to `mixed-reference`, and rev-12's rule that a corrupt object does not block the
other classes.

**PIB-550 — `TestS7AVRevalidateUnlinkWindowContracts` (C)**

The window is driven exactly where the row specifies: an external writer
replaces the managed object at `beforePurgeBlobRemove`, gated on
`afterPurgeBlobRevalidate` having already fired for the same path, over both a
replacement **regular file** and a replacement **directory**. The guard half is
implemented as specified — `s7AVValidateWindowDisclosure` proves the PRD §9.7.2
five-window table and ADR-035's residual paragraph both disclose the
revalidate→unlink replacement **beside** the post-CAS final-syscall race (and in
that order), that both still deny closure, and that no normative sentence and no
shipped surface **asserts** closure. Claim detection is sentence-scoped so the
documents' own denials ("No shipped message claims this window is closed") pass,
and alternatives-table rows (`| Claiming …`) and acceptance-matrix rows
(`| PIB-…`) are excluded as quotation rather than assertion. Four sensitivities:
the closure claim added to the PRD's normative row, to ADR-035's normative
paragraph, to a shipped report string, and the final-syscall race dropped from
the disclosure.

**PIB-550 behaviour divergence — recorded at rev-1, adjudicated at rev-2 by the
paired rev-18 erratum (see the rev-2 section below).** The rev-11 row asserted
that the unlink removes the replacement and the invocation ends at exit 0
`recovered` with the byte-level loss undetected. That is **not** what the
shipped product does at this seam, and the test asserts the shipped behaviour
instead. `prepareArchiveStorage.RemoveBlob` calls `PreflightBlobRemove`, which
re-probes the managed object's identity token immediately before `root.Remove`,
so a replacement landing at `beforePurgeBlobRemove` is **detected**: both kinds
end at exit **5** `archive-purge-partial` with `resume:
pending-recovery-then-completion`, `pending_hash: h`, zero completed hashes, the
replacement object byte-identical (no byte-level loss at all), the hash still
removal-pending, and the rerun routing to exit 6
`archive-purge-evidence-divergent` naming the managed blob path. The genuinely
open residual therefore narrows to the probe→unlink syscall gap, which has no
injection seam and is exactly the already-disclosed final-syscall race. **This
is a PRD-row/implementation divergence for the supervisor to adjudicate** — the
honest fix is a rev-N erratum to PIB-550's asserted observable, not a
weakening of the shipped identity re-probe. No production code was changed.
**Rev-2 took exactly that route**: the rev-18 erratum now states the shipped
observable, and the test asserts it against the corrected contract rather than
against a contradicted one.

**PIB-551 — `TestS7AVDispositionTableTotalityGuard` (G)**

`s7AVValidateDispositionTotality` derives both sides. The domain comes from the
declared `IntentArchiveWireState` and `IntentArchiveBlobState` constants
(3 × 3 × 2 × 2 = **36**); reachability is computed independently from §9.3's
three dependencies and the **18** ruled-out tuples are asserted by partition
(**6** retained × unreferenced, **9** removal-pending, **3** tombstoned × owned ×
unreferenced), leaving **18** reachable. A bounded interpreter over the real
source of `IntentArchiveTupleReachable` and `ClassifyIntentArchiveTuple`
produces the mapping, and its faithfulness is proved by cross-checking all 36
tuples against the live shipped functions before any fixture is judged. The
shipped §9.3 table is parsed from the PRD: exactly **18** rows, contiguous
ordinals, a bijection onto the reachable set, no duplicated tuple, no row over an
unreachable tuple, and each row naming exactly the one repair family its
interpreted classification assigns (with the corrupt row permitted to name the
dangling follow-up its own prerequisite produces, and never `--orphans`). The
two rev-12 splits — rows 10/11 on liveness and rows 13/14 on hash-correctness —
are asserted directly as four distinct rows. Ownership precedence is asserted
over every reachable owned tuple: no repair class, no code, no exit class, and
action `route-pending-owner`.

Four same-validator sensitivities: dropping the retained/absent-and-owned tuple
so an owned dangling reference is classified `archive-blob-dangling`; adding a
second route (`--orphans --yes` beside the corrupt procedure) to the
tombstoned/present-unidentifiable row; re-collapsing rows 13 and 14 into
rev-11's single row (17 rows); and duplicating a row so the count disagrees with
the stated domain (19 rows over 18 tuples) — which a count-free totality check
would pass.

**AV rev-1 test results (macOS, local):**

- `go test ./internal/cli -run '^(TestS7AV.*|TestS7CI.*|TestS7ObservedRegistrationWrongInputs)$'`
  — pass, 5.881s package.
- `go test ./internal/cli -run '^TestS7ObservedAVRegistrationAuthority$'` —
  pass, 4.748s package (six-target overlay observer).
- `gofmt -l .` clean; `go vet ./...` clean; `go build ./cmd/tpatch` clean.
- Full non-observer suite with the CI skip pattern
  (`go test ./... -count=1 -timeout 40m -skip '…|AS|AT|AU|AV)RegistrationAuthority)$'`)
  — every package **ok**; `internal/cli` 366.230s, `internal/workflow` 104.890s,
  `internal/store` 10.533s. No regression.

**AV rev-1 residuals (see the rev-2 section for their disposition):**

- **PIB-550's asserted observable is falsified by the shipped storage layer**
  (see above). **Closed at rev-2** by the paired rev-18 no-decision erratum;
  no production behaviour changed.
- **PIB-547's caveat wording is split across the two procedures.** The exact
  rev-13 sentence "it is still in this repository's Git history" ships on the
  divergent-blob (exit-6) surface. The corrupt-object procedure printed by
  `list`/`doctor` carries §9.6.2's caveat in its own shipped wording
  (`intentArchiveHistoryDisclosure`, "A committed blob remains in Git
  history…"). The test asserts the exact sentence where it ships and a
  Git-history caveat on the other surface, and both are required to pass the
  tokenizer — which is the property the rev-13 correction is actually about.
  Whether the two surfaces should be unified on one sentence is a contract
  question, not an implementation gap.
- PIB-551's table parser is deliberately strict: an unrecognised wire-state or
  blob-observation phrase in a §9.3 row is an error, not a default. Rewording a
  row without updating the parser fails loudly, which is the intent.
- PIB-546's flow walker treats a loop body as executing zero or more times, so
  it is conservative in the "not yet scanned" direction. It reports a possible
  pre-scan mutation rather than proving one on a concrete path; the runtime
  half is what proves the concrete path. **Still true at rev-2**, and
  deliberately so.
- PIB-546's rev-1 memo was keyed on `function|scanned` alone and cached only
  the resulting scan fact, so a helper first walked under one route reported
  nothing when a second route reached it. **Closed at rev-2.**
- `internal/cli` is now 366s locally for the non-observer suite. It remains
  under the 40m CI package budget with wide margin, but it is the slowest
  package by an order of magnitude.

### AV Implementation Session — rev-2 (five review findings)

**Scope**: the five findings raised against AV rev-1. No production source or
asset is touched, no Go command was run in this session (documents and tests
only), and the six AV ledger rows keep the same six exact top-level targets.

**F1 — `PIB-550`: adjudicate the divergence with a paired erratum.** Rev-1
recorded that the shipped storage detects the replacement instead of removing
it. Rev-2 amends the contract rather than the product, as a **rev-18
no-decision erratum** dated 2026-08-28, **acceptance pending joint review**, in
both documents:

- PRD header (Status, a `**Rev-18 acceptance**` block, Byline and the
  Architecture companion note), a `| rev-18 |` revision-history row, and the
  §18.1 amendment-ledger paragraph;
- PRD §9.7.2's **fourth window row**, which now states that the removal carries
  step 2's identity, that `prepareArchiveStorage.RemoveBlob` re-probes the
  managed object immediately before the syscall and refuses a mismatch, that the
  replacement is preserved at exit **5** `archive-purge-partial`
  (`resume: pending-recovery-then-completion`, `pending_hash: h`, zero completed
  hashes) with the rerun at exit **6** `archive-purge-evidence-divergent`, and
  that the residual narrows to the probe→unlink syscall gap, which is **not**
  detected and not claimed to be;
- PRD §9.7.2's closing two-residual paragraph and its exit-6 atomicity note,
  PRD §21's alternatives row (the re-check ships; it narrows rather than closes
  the window), and matrix row `PIB-550`;
- PRD §18.53's sensitivity entries for `PIB-546` and `PIB-551`, which gain the
  fixtures F2 and F3 required;
- ADR-035's header (Status, `**Rev-18 acceptance**`, Byline, companion note), a
  `| rev-18 |` revision-history row, D16's residual paragraph, D16's closing
  atomicity paragraph and the matching alternatives row.

Counts are unchanged: **567** matrix rows, **thirty-six** semantic guards,
§18.52's category/kind/slice arithmetic untouched, and **no ADR decision
changes** — D1–D21 stand as accepted.

The disclosure validator was rebuilt around the corrected text. Each document
now carries **three ordered anchors** — the revalidate→unlink window, the
residual probe→unlink gap, and the post-CAS final-syscall race — each of which
must occur **exactly once** and in that order, plus a unique denial sentence and
a list of **retired** rev-11 readings that must be absent. The old contradictory
anchors are gone: the PRD no longer keys on `**Not detected, and not claimed to
be.**` and the ADR no longer keys on `between the pre-removal revalidation and
the` or `disclosed rather than claimed closed`, both of which occurred three and
two times respectively and could not order anything. Ten sensitivity fixtures
cover claim injection into each document and into a report, dropping either
residual from either document, reordering the two residuals, duplicating the
window row, dropping the denial, and restoring the retired reading in each
document.

**F2 — `PIB-546`: the flow walker's memo dropped findings.** `walkFunction`
memoised `name|scanned` → `after` and recorded findings only as a side effect,
so the first route to reach a shared helper consumed it: a second route hitting
the cache reported nothing. Rev-2 keys the memo on
`entry|routeContext|name|scanned`, caches the findings recorded during the
sub-walk, and **replays them on a cache hit**, so every route/context that
reaches a mutation reports it. A walk truncated by the recursion guard is not
cached at all, because its result depends on the caller chain the key does not
carry. `route()` and the new `routeContext()` share one attribution table.

The new sensitivity proves the fix bites where rev-1 was blind: the ordinary
selector execution is hoisted ahead of the plan **and** `ExecuteIntentArchivePurge`
is made to trust the planned selection, so the ordinary route reaches
`executeIntentArchivePurgeHash` — the same helper `RecoverPendingPurge`
reaches — with no scan before it. The validator must then report **two**
mutation routes. Two deltas are needed because the ordinary route is dominated
by two independent scans (`PlanIntentArchivePurge` in the CLI and
`CaptureIntentArchive`/`BuildIntentArchivePurgePlan` inside Execute); the
fixture table now carries an ordered delta list for that reason, and every other
fixture stays a single delta.

**F3 — `PIB-551`: derive row 14's exit 6 from the purge machine.** The
classifier deliberately gives an owned tuple no code and no exit class, so
rev-1's split assertion could only distinguish rows 13 and 14 by row count. Rev-2
adds a second authority derived from the shipped machine:
`s7AVDeriveOwnedUnidentifiableRoute` requires `executeIntentArchivePurgeHash`'s
evidence check, `RecoverPendingPurge`'s owned preflight and
`intentArchiveUnidentifiablePurgeError`'s owned arm to **agree** on one code and
one exit class, marks the refusal post-mutation, and requires the not-owned arm
to stay a distinct exit-3 classification;
`s7AVDeriveOwnedCompletionRoute` requires the hash-correct branch to reach
`removeIntentArchiveBlob`, publish the tombstone after it, construct no
divergent error and end the function at `return execution, nil`. Every owned
row is then held to its machine outcome — an unidentifiable observation must
name the derived code and exactly the derived exit class, and every other owned
observation must name neither — and rows 13 and 14 are additionally asserted as
exit **0** under the owner versus exit **6** divergent.

Four sensitivities bite the route rather than the count: row 14 given row 13's
owner-sweep wording with all eighteen rows intact; row 13 given row 14's exit-6
divergent route; the shared owned arm demoted to the not-owned exit-3
classification; and the machine's evidence check demoted to exit 3 while its
owned recovery preflight still refuses exit 6.

**F4 — global hook replacements now register `t.Cleanup` immediately.**
`s7AVRestoreSeam(t, restore)` registers the restore with `t.Cleanup` at install
time and returns an **idempotent** restore the caller runs explicitly before any
later phase. All four AV seam sites use it: the `afterPurgeBlobRevalidate` and
`beforePurgeBlobRemove` pair in the replacement window (restored before the
rerun and disclosure phases), the `afterPurgeIndexRename` index-divergence
driver, the `intentArchiveNewStorage` removal spy, and both
`s7ARInstallDeviceProbe` installs. A `t.Fatal` between install and the explicit
restore can no longer leak a seam into a later test.

**F5 — this handoff.** Updated above and below; the stale rev-1 residual and
blocker are marked closed with their disposition rather than deleted, and the
rev-2 record states exactly what changed.

**Erratum evidence guard.** `internal/cli/prepare_s7_rev16_test.go` pins both
documents against base `2d9492cbf6fd9c69c5aa75d64d05983c05e1563f`, so the
erratum required it to learn: five new allowlisted regions (PRD §9.7.2, §18.49,
§18.53, §21 and ADR `## Alternatives considered`), updated current hashes for
the five regions the erratum edits (PRD header, revision history, §18.1; ADR
header, D16), `PIB-550` as the seventh changed matrix row in both the parsed
matrix and the `git diff` row set, a 16/17/18 revision tail, rev-18 disposition
and no-decision token checks, PRD rev-18 row tokens (`PIB-550`, row, kind,
count, 567, thirty-six), and an absence check for the retired readings in both
documents' current normative text. Two rev-18 sensitivity fixtures join the
existing seven.

**AV rev-2 test results:** AV suite **PASS** in 6.263s; exact six-target
observer **PASS** in 9.928s; exact full non-observer command **PASS** with CLI
at 471.544s. Formatting, vet and CLI build pass.
- AV exact tests, observer and CI guards are checkpointed at `cf2389f`.

**Final GH #23 validation:**

- AW accepted at `31192a0`; CI 33177631054 green on all five blocking jobs.
- AX/rev-19 accepted at `51382fc`; CI 33198502426 green on all five blocking
  jobs. S7 is 173/173.
- Aggregate acceptance committed at `7d4981a`: 567/567 body-sensitive rows,
  123/123 G sensitivities, zero blocked escapes.
- Full uncached non-observer suite **PASS** (`internal/cli` 512.183s);
  aggregate/S5/golden race shard **PASS** (901.022s); shared S6 guard race
  shard **PASS** (307.001s); other touched-package race suites **PASS**.
- Vet/build, Linux/Darwin/Windows/BSD CLI test-compiles and Linux amd64/arm64,
  Darwin amd64 and Windows amd64 binary builds **PASS**.
- Aggregate CI 33230665925 is green at `6eb51c0`; final post-fix CI
  33236724647 is green at `30dbdba`.
- Production review found the `Pread` EINTR count-underflow panic. Fix
  `1c5ad8b` and its focused non-race/race/Linux/Windows tests are **APPROVED**.
- Rev-17, rev-18 and rev-19 are accepted jointly on 2026-08-29. Changelog
  graduation, ADR-index, authorization, ROADMAP and doctor-panic record guards
  pass locally.
- Side Research md5 remains `b385fe622db9926f48861105239f113e`.

## Next Steps

1. Checkpoint and push rev-20, then require green blocking CI and wave close.
2. Wait for explicit user approval.
3. After approval: graduate CHANGELOG to `v0.16.0`, update
   version metadata, commit the release, create/push the annotated tag, and
   create the GitHub release.

## Blockers

- Version bump, CHANGELOG graduation, tag, tag push and GitHub release remain
  blocked on explicit user approval by design.
- Release mechanics wait for rev-20's green close gates.

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
- GH #23's explicit file partition is authoritative. Stage exact files only;
  never use `git add .`, `git add -A`, directory-scope adds or `git commit -a`.
- Rev-11 must consume method-expression receivers before binding callback
  parameters (with value- and pointer-receiver bite-proofs), invalidate every
  affected package claim dependent when a cross-file constant changes, use
  exact selector-specific blob/generation value domains, and treat
  negative-quantifier block/strand forms as positive guarantees requiring an
  authoritative route. The fresh focused package sum remains capped at 325
  seconds; the unchanged observer retains 12m/8m/1m budgets and a 420-second
  target.
- Rev-15 changes only `s7ARSignatureCarriesCallable`: the fixed tuple scan is
  untouched, and the additional predicate can return true only for a variadic
  signature whose final slice element is function-valued. Reviewer attacks
  should therefore focus on alias/named element identity, fixed container
  exclusion, MethodExpr receiver offset and invocation gating; the complete
  PIB-518 cases bind each of those through the production-source validator.
- Rev-17 replaces rev-16's lossy one-object union with alternatives of ordered
  callback origins. Constant selection is exact; dynamic index/range unions
  only possible elements. The local forward interpreter shares direct slice
  aliases and applies exact sequential index/copy overwrites; conditional,
  append, reslice and package-scope routes preserve candidates but fail
  closed. Expansion evidence is gated by direct/transitive variadic-parameter
  invocation, while the main resolver remains authoritative for unresolved
  interface/literal dispatch. Reviewer attacks should focus on multiple call
  alternatives, alias reassignment versus shared mutation, nested
  control-flow joins and transitive forwarding. No spelling/path/count/store
  special case exists.
- Rev-18 adds one demand abstraction shared by explicit and expanded callback
  routes: constant indices remain exact, while negative/unknown/out-of-range,
  dynamic/range, recursive or unresolved forwarding demand all possible
  elements. Mutation uncertainty is index-addressed when helper/alias/append
  effects are statically bounded and all-addressed otherwise; it is consulted
  only for demanded elements. Empty transient forwarding states are skipped
  rather than irreversibly poisoning the monotone fixpoint. Reviewer attacks
  should focus on multiple slice parameters, nested MethodExpr helper effects,
  conditional capacity aliases and mixed resolved/unresolved forwarding;
  those routes conservatively fail closed where exact identity cannot be
  proved.
- Rev-19 makes demand and callable binding program-point aware with correlated
  whole-state alternatives. Straight-line assignments replace; branch/switch/
  select joins retain only reachable alternatives; returns remove fallthrough;
  loops include zero iterations and a bounded backedge fixpoint. Exact backing
  identities keep alias reassignment distinct from shared element writes, and
  the same walker computes helper-mutation summaries. Scalar call-target
  resolution, expanded-sequence binding and final unresolved audit all consume
  the call's reaching state rather than whole-body unions. Labeled control
  transfer, state overflow beyond 64 alternatives and an unstabilized
  32-iteration loop deliberately fail closed. Reviewer attacks should focus
  on nested labeled control, multi-result assignments, recursive callable
  aliases and complex append/reslice capacity joins; no spelling, source-path,
  callback-count or canonical-store special case exists.
- Rev-20 schedules callable-slice defer/go calls in the correlated state.
  Deferred execution replays every reachable return-state override against the
  captured descriptor; asynchronous execution includes statement-time and
  every later mutation state. Scalar arguments and call targets remain
  value-captured. Direct/copy origin overrides are exact where proved;
  helper, append, derived-reslice and deferred-mutator uncertainty is scoped
  by backing identity, offset and affected element. Every active function now
  receives reaching state, while fixed `[]func()` direct element invocation
  remains excluded and fixed slices forwarded into accepted variadic sinks
  retain exact flow. State alternatives cap at 64, loops at 32 iterations and
  unsupported labeled flow fails closed. Reviewer attacks should focus on
  nested deferred mutators with mixed backings, loop-scheduled calls,
  function literals with callable parameters and complex multi-result
  factories; those unsupported/ambiguous routes must reject rather than lose
  an alternative.

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

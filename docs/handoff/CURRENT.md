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

## Active Task

- **Task ID**: `implement-prepare-intent-bundle`
- **Issue**: [GH #23](https://github.com/tesseracode/tesserapatch/issues/23)
- **Description**: Implement the mutating `tpatch prepare <slug>` intent-bundle
  contract from the accepted `PRD-prepare-intent-bundle` rev-15 +
  `ADR-035-intent-bundle-publication-and-history` rev-15 (ADR-035 normative
  where they overlap).
- **Status**: **In Progress — S7 AQ observer approved; AR blocked on CI**
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

## Test Results

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

## Next Steps

1. Commit/push the approved observer correction and require green blocking CI.
2. Then implement AR–AX, remaining sensitivities and the full 567 ledger from exact
   runtime/document observables; obtain clean review.
3. Run joint internal/external review to acceptance; only then select the
   release tag carrying `prepare --check` plus mutating prepare.

## Blockers

- AR–AX remain procedurally blocked until AQ is checkpointed and green in CI.

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

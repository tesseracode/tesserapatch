# ADR-034 — Rooted Filesystem Inspection Boundary

**Status**: Proposed — Awaiting Review (rev-0)
**Date**: 2026-08-13 (Proposed)
**Owner**: Core (planning lane)
**Byline**: writer sub-agent, rev-0 at HEAD `5a678b5`
**Cluster**: WP-005 spec-driven workflows / GH #10
**Supersedes**: none
**Superseded by**: none
**Depends on**: [ADR-027](./ADR-027-capture-context-privacy-boundary.md) (D2 no raw
context, D6 no wall-clock), [ADR-033](./ADR-033-resource-capture-boundary.md)
(D10 no tracked timestamps, D11 no Go map in a wire schema)
**Companion**: [PRD-artifact-validation-and-provenance](../prds/PRD-artifact-validation-and-provenance.md)
(rev-3, Draft — Awaiting Review). **The two documents are reviewed together.**
Read the PRD for the full design and its acceptance matrix; this ADR states the
decisions the PRD's §7 depends on, and where the two overlap **this ADR is
normative**.
**Blocks**: implementation of `tpatch prepare <slug> --check` (PRD §17 slices
S1–S5). No implementation is authorized until both documents are accepted.

---

## Context

`tpatch prepare <slug> --check` (PRD rev-3) is a **read-only** inspector that
classifies four intent artifacts plus `status.json` under
`.tpatch/features/<slug>/` and reports a structural-readiness verdict. It
writes nothing, advances no state, and adds no lifecycle state.

Its whole value is *truthfulness*, which makes the filesystem-access model a
first-class architectural question rather than an implementation detail. The
inspector must survive a hostile or corrupted `.tpatch/` tree: a `spec.md`
symlinked outside the repository, a `status.json` replaced by a Windows
junction, an `exploration.md` replaced by a FIFO with no writer, an ancestor
directory swapped for a symlink between the check and the open, and a file that
grows during the read.

Three prior revisions of the companion PRD proposed three different mechanisms:

- **rev-0**: `os.Lstat` then an ordinary `os.Open`. Follows a symlink that
  appears after the `Lstat`; blocks forever on a FIFO.
- **rev-1**: absolute pathnames composed with `filepath.Join`, a
  component-by-component `os.Lstat` walk, `openNoFollow`, and a
  descriptor-identity check — i.e. a hand-rolled reproduction of
  `internal/rescap`'s pathname gate. Every check and the final open resolve the
  *name* independently, so the kernel re-resolves the ancestor chain at open
  time with whatever the tree looks like then. Its Windows half cited
  `internal/rescap/pathopen_windows.go:1-20`, which is an explicitly
  unsupported compile-only stub.
- **rev-2**: one held Go 1.26 `*os.Root` for the repository root, with every
  `Lstat` and open handle-relative to it.

The rev-2 review confirmed `os.Root` as the right mechanism and simultaneously
found that rev-2 **overstated what it provides**: it claimed physical
"nothing outside the repository is ever read" confinement, used a fail-open
platform denylist, mis-stated the Windows reparse-tag mapping, kept an inert
`O_NOFOLLOW`, promised substitution detection stronger than `os.SameFile` can
deliver, and made per-capture allocation claims it never totalled.

Adopting a held `*os.Root` as a new read-only rooted namespace — alongside, and
deliberately not replacing, the shipped `rescap.GatePath` pathname model — is a
non-obvious architecture choice that sets platform, confinement and identity
policy for a new package. `AGENTS.md` → Context Preservation Rule 7 ("ADR on
every architecture decision") therefore requires this record. The rev-2
supervisor adjudication dispatched it explicitly.

**This ADR is paper-only.** It changes no code, schema, CLI behavior, asset,
PRD body, handoff entry, release artifact or `CHANGELOG.md` entry.

### What this ADR is *not*

It is **not** the provenance ADR. The companion PRD emits the constant
`provenance: unknown` and selects **no** persistent representation, so the
WP-005 provenance ADR trigger (PRD §11.4) remains **unfired**. A future PRD
that selects a persistent provenance representation must write that ADR
separately.

---

## Decisions

### D1 — One held `*os.Root`, opened once per invocation, owned by the CLI layer

The CLI layer calls `os.OpenRoot(repoRoot)` **exactly once** per invocation,
after workspace discovery, wraps it in the D3 adapter, passes that to
`Inspect`, and closes it **exactly once** after the report is rendered.
`Inspect` never opens and never closes a root.

Every filesystem operation the inspector performs — the four artifact captures
**and** the `status.json` capture whose value reaches output — goes through that
one root. `store.LoadFeatureStatus`, `os.ReadFile`, `os.Open`, `os.OpenFile`,
`os.Stat`, `os.Lstat`, `os.ReadDir` and `filepath.Walk` are forbidden in the
inspector and in the `prepare` command file, and the prohibition is
AST-guarded with sensitivity fixtures.

**Rationale.** A rooted namespace resolves one component at a time relative to
a held descriptor/handle and refuses any resolution whose name would leave it.
That removes the entire class of "the kernel re-resolves the name at open time"
defects that rev-1's pathname model was exposed to, on every target the project
ships to, without a hand-rolled per-platform resolver.

### D2 — The guarantee is **logical rooted pathname confinement**, not physical confinement

`os.Root` bounds the **name** the process resolves. It does **not** bound the
physical filesystem, the physical device, or the semantics of the object a name
finally reaches. The `Root` doc comment says so in its own words:

> Methods on `Root` do not prohibit traversal of filesystem boundaries, Linux
> bind mounts, /proc special files, or access to Unix device files.
>
> — Go 1.26 `os` package, `Root` doc comment (`$GOROOT/src/os/root.go`)

Therefore:

- **Permitted and reachable through the root**: mount points, Linux bind
  mounts, `/proc` special files, Unix device files, and any untracked or
  Git-ignored file — provided each is nameable from inside the root.
- **The bytes a leaf yields may physically originate outside the repository's
  own filesystem.** No document, output string, help text, doc page or skill
  surface may claim otherwise.
- **What closes the leaf cases is the kind gate and the bounded read**, not the
  root: a non-regular leaf is refused (`ModeSymlink`/`ModeIrregular` refusal,
  then `!IsRegular()`), and a leaf that reports as regular but streams
  indefinitely is bounded by D9's fixed buffer and classified `unstable` on the
  byte-count disagreement.

**Three sentences are prohibited by name**, because rev-2 shipped them: "no
path outside the repository is ever opened, read, or named"; "no operation can
resolve outside the repository"; and any bounding of a raced link's blast
radius to "some other object inside this repository". The permitted form
qualifies the claim as **logical**, **pathname** or **name** confinement.

A mechanical guard (PRD AVP-189) scans every shipped string, the companion PRD
and **this ADR** for the prohibited forms, with a sensitivity fixture that
reinserts rev-2's exact sentence.

### D3 — Workspace discovery is outside the rooted capture, and `--path` is trusted input

`store.FindProjectRoot` (`internal/store/store.go:23-40`) resolves the `--path`
value with `filepath.Abs` and walks upward for a `.tpatch/` directory. It is an
ordinary, symlink-following pathname traversal in the shipped store package, it
runs **before** `os.OpenRoot` — it is what *produces* the directory the root is
opened on — and it is therefore **outside** every confinement claim this ADR
makes.

Consequences, stated rather than implied:

- The companion PRD's guarantee is scoped to "every filesystem operation
  **after workspace discovery**". rev-2's unqualified "every filesystem
  operation" is withdrawn.
- An operator who points `--path` at a hostile directory gets a root opened on
  that directory. The command then confines itself to it and reports about it.
  Nothing in this slice hardens the discovery walk, and nothing claims to.
- Workspace-discovery failure is a **precondition abort**
  (`workspace-not-initialized`, exit 3) decided at PRD §9.3 step 5, not a cobra
  usage error.

### D4 — Root-relative names only, canonical `fs.ValidPath`; `EnsureSafeRepoPath` is not used

Every name handed to a root method is relative, slash-separated, and composed
**only** from compile-time constants and the validated canonical slug. No
absolute path is composed inside the inspector; no flag, config key or artifact
content contributes a component.

Each composed name — the four artifact names, the status name, and every prefix
used by the component walk — is asserted to satisfy `fs.ValidPath`: unrooted,
non-empty, valid UTF-8, no `.` or `..` element, no empty element, no leading or
trailing slash.

**`safety.EnsureSafeRepoPath` is removed from this design entirely.** rev-2 kept
it as a "defence in depth" pre-filter. That was incoherent: it is a lexical test
defined against an absolute repository *prefix string*, applied here to a
*root-relative* name in a design whose containment primitive is a *handle* —
and under `--path <dir>` from an unrelated cwd, the prefix it wants to test
against is not a value the inspector holds. It could never fire, so it proved
nothing, while implying a containment contribution it does not make.
`fs.ValidPath` is the property that actually constrains a root method argument.

This decision is scoped to `internal/intent`. `safety.EnsureSafeRepoPath`
remains correct and in use elsewhere; nothing about it changes.

### D5 — Platform support is a fail-closed allowlist copied verbatim from the standard library

`internal/intent` carries a build-tagged `rootConfinementSupported` constant in
exactly two files:

| File | `//go:build` expression | Value |
|---|---|---|
| `confine_supported.go` | `unix \|\| windows \|\| wasip1` | `true` |
| `confine_unsupported.go` | `!(unix \|\| windows \|\| wasip1)` | `false` |

The `true` expression is **byte-identical to `$GOROOT/src/os/root_openat.go`'s
own build tag**, and the `false` expression is its exact syntactic negation.
The inspector's allowlist *is* the standard library's allowlist, not a
paraphrase of it, and a guard compares the two texts.

When the constant is `false` the command aborts `workspace-unsupported-platform`
(exit 3) **before** `os.OpenRoot` is called and before any name is composed.
This is a refusal, not a degraded mode.

**Why an allowlist and not a denylist.** rev-2 used
`(js && wasm) || plan9` → `false` with its negation → `true`. That is
**fail-open**: it hard-codes the claim that today's two unconfined targets are
the only ones there will ever be. A `GOOS` added by a future Go release, or an
existing one Go later reclassifies, lands silently in the `true` branch and the
command asserts a confinement guarantee nobody checked. `js/wasm`, `plan9` and
any future unmatched `GOOS` must all abort, and only the allowlist form
delivers that.

### D6 — The open carries `O_NONBLOCK` on Unix and nothing on Windows; caller-side `O_NOFOLLOW` is removed

`openFlags()` is build-tagged and returns:

- non-Windows: `syscall.O_NONBLOCK`, and nothing else;
- Windows: `0`.

No write, create, truncate or append bit ever appears, on any target. There is
no raw `syscall.CreateFile`, no `windows.Openat`, and no call into
`internal/rescap`'s open helpers.

**`O_NONBLOCK` is load-bearing but is an implementation dependency, not a
contract.** It covers exactly the raced window in which a FIFO or blocking
character device replaces the leaf after the pre-open kind check refused the
stable case. Nothing in `os`'s documentation promises that caller flags reach
`openat`; the behavior is read from unexported code
(`root_unix.go` → `rootOpenFileNolog` → `unix.Openat` /
`unix.HasNonblockFlag`). It is therefore recorded as a **tripwire** (D10) with
a dedicated Go-upgrade test that opens a real writer-less FIFO through a real
`os.Root` under a hard deadline.

**Caller-supplied `O_NOFOLLOW` is removed.** `Root` already ORs it internally
on the final component and then converts the resulting signal into an in-root
symlink *resolution* rather than a refusal. rev-2 documented that and passed the
flag anyway, "belt-and-braces". A flag whose own specification says it produces
no observable effect is not defence in depth — it is a false affordance that
invites the next reader to infer a no-follow guarantee that does not exist.
**No test, message or document may claim that a caller-supplied `O_NOFOLLOW`
produces a refusal through `Root`.**

### D7 — The Windows kind mapping is stated exactly, and `winsymlink=1` is pinned in `package main`

Under the current `(*fileStat).mode` (`$GOROOT/src/os/types_windows.go`):

| Reparse tag | Name surrogate? | Mode bits | Refused by the `ModeSymlink\|ModeIrregular` predicate? |
|---|---|---|---|
| `IO_REPARSE_TAG_SYMLINK` | yes | `ModeSymlink` | yes |
| `IO_REPARSE_TAG_MOUNT_POINT` (junction) | yes | `ModeIrregular` via the `default` branch; `ModeDir` and `GetFileType`-derived bits suppressed | yes |
| `IO_REPARSE_TAG_AF_UNIX` | no | `ModeSocket` | **no** — closed by the `!IsRegular()` kind gate |
| `IO_REPARSE_TAG_DEDUP` | no | **no type bit**; deliberately treated as a regular file by Go, with an explanatory comment | **no, and deliberately so** — read as the regular file Go says it is |
| any other tag | no | `ModeIrregular` via `default` | yes |

Two decisions follow:

1. **`ModeSymlink|ModeIrregular` is a *refusal* predicate, not the kind
   policy.** It is necessary and **not sufficient**. rev-2's "every other
   reparse tag sets `ModeIrregular`" is withdrawn: AF_UNIX sets `ModeSocket`
   and DEDUP sets no type bit at all. The `!IsRegular()` gate (applied both
   pre-open from `Lstat` and post-open from the descriptor stat) is what makes
   the kind policy total.
2. **A DEDUP reparse point is inspected as an ordinary regular file.** This
   accepts the standard library's own documented reasoning — DEDUP files
   support ordinary random-access reads and should not flip from regular to
   irregular when the Data Deduplication job runs — and it is recorded here as
   a decision rather than left as an omission.

**`//go:debug winsymlink=1` is pinned in `cmd/tpatch/main.go`.**
`(*fileStat).Mode` consults the `winsymlink` GODEBUG setting and, when it is
`0`, recomputes the mode with `modePreGo1_23`, under which a junction maps to
`ModeSymlink` instead of `ModeIrregular`. The refusal outcome is stable across
both values; the mode bits an assertion observes are not, and any future
narrowing of the predicate would not be. A `//go:debug` directive is honored
only in the **main** package, which is where it goes. The module already
declares `go 1.26.1` and therefore already defaults to `1` — the directive
makes the dependency explicit and reviewable rather than inherited, so a
`go.mod` language downgrade, a vendored build, or a copy of the main package
into another module cannot silently move the mapping.

**Honest limit, recorded rather than papered over**: `//go:debug` sets the
*default*; the `GODEBUG` environment variable still overrides it at process
start. The directive pins and documents the default; the behavioral test
detects a change in the effective value.

### D8 — Symlink refusal is *observation*-based; the identity promise is "observed as different"

**Component policy.** A component **observed** to be a symlink or a refused
reparse point is refused without being followed, resolved or named. The walk
runs **twice per capture** — once before the open and once after the read. A
component observed as changed by the post-capture walk makes the result
`unstable`; the captured bytes are discarded and no content state is reported.

**Identity policy.** Before the first byte is read, the leaf's pre-open
`Root.Lstat` `FileInfo` and the opened descriptor's `File.Stat` `FileInfo` are
compared with `os.SameFile`. A mismatch ends the capture with `unstable` and
**zero bytes read**.

**The promise, stated exactly:**

> An object **observed as different** is never read.

rev-2 said "a *different* object is never read". That is withdrawn as
unprovable. `os.SameFile` compares identity *numbers*, and five cases separate
"different" from "observed as different". All five are documented in the PRD
(§7.4.4, §8.3), asserted as *limits* rather than capabilities, and may not be
claimed away anywhere:

1. **Same-length in-place rewrite** — undetectable by size, kind or identity.
2. **Same-identity alias** — a raced in-root link resolving to the very object
   observed. The report remains true of the object read; no refusal is claimed.
3. **Hard-link alias** — undetectable **by construction**: a hard link *is* the
   same inode / file index. No additional check inside this design could
   distinguish the two names.
4. **Inode / NTFS file-ID reuse** — an unlinked object's identity number can be
   reassigned, so two genuinely different objects can compare equal. Inherent
   to identity-by-number.
5. **Swap-and-restore between probes** — the capture has finitely many
   observation points; an adversary with arbitrary timing control can restore
   the intended object before each one. The post-capture walk narrows the
   window; the residual walk→`Lstat`→open TOCTOU window is **not** closed, and
   is named rather than implied.

A sixth, structural limit is also recorded: a second `fstat` on a **held**
descriptor is a tautology for pathname questions — it detects a change to the
object it holds, never a name swap.

### D9 — One reused `MaxArtifactBytes+1` scratch buffer per invocation

The CLI layer allocates **exactly one** data buffer per invocation:
`make([]byte, MaxArtifactBytes+1)` — `MaxArtifactBytes = 4 MiB`, so
**4,194,305 bytes**. It is passed to `Inspect` and reused **sequentially** by
the `status.json` capture and all four artifact captures. The status capture
uses `scratch[:MaxStatusBytes+1]`, a sub-slice of the same array, never its
own.

Properties:

- Reuse is safe without a lock, pool or copy because the captures are strictly
  sequential — there is no goroutine anywhere in this command — and each
  classification consumes `scratch[:n]` before the next capture begins.
- `MaxStatusBytes < MaxArtifactBytes` becomes a **structural invariant**,
  asserted at compile time. The two remain distinct, independently revisable
  constants; the reuse imposes only the ordering.
- `io.ReadAll`, `io.LimitReader`, `os.ReadFile` and `bufio.Scanner` are
  forbidden. rev-1's `io.ReadAll(io.LimitReader(f, Max+1))` bounded the result
  *length* but not the *allocation*, and rev-1's "exact allocation ceiling"
  claim was false.

**The cost is stated honestly rather than hidden.** The command pays a flat
4,194,305 bytes for **every** invocation, including a run that aborts before
capturing anything and a run against four 12-byte files. This is a deliberate
trade: a fixed, predictable ~4 MiB against five variable allocations and any
possibility of growth during a read. rev-2's per-capture form had a ~20 MiB
worst case it never totalled. The companion PRD's Q9 records lazy allocation as
a revisable alternative.

**Cap values are coupled to frozen message text.** `4 MiB` appears verbatim in
the `analysis-sidecar-oversize` advisory and the oversize remediation; `1 MiB`
appears verbatim in the `status-oversize` abort template. Changing a constant
without changing its message would ship a diagnostic stating a limit the binary
does not enforce, so a guard derives each unit string from its constant and
fails on disagreement in either direction.

### D10 — Standard-library claims are split into contract and tripwire, and tripwires gate Go upgrades

Every claim this design makes about the Go standard library is classified:

- **`contract`** — stated in the package's own documentation (a doc comment on
  an exported identifier, or the registered `GODEBUG` history). Go's
  compatibility promise covers it.
- **`tripwire`** — read from **unexported** code in the pinned toolchain
  (`go1.26.5`, for a module declaring `go 1.26.1`). Go promises nothing.

Resting on a tripwire is permitted; **claiming a tripwire as a guarantee is
not**. The companion PRD's §23.2 carries the full classified table (21 claims:
8 contract, 13 tripwire). The tripwires this design actually depends on are:

| Dependency | Runtime tripwire test |
|---|---|
| `Root.OpenFile` forwards caller flags to `openat` (`O_NONBLOCK` no-hang, D6) | real writer-less FIFO opened through a real `os.Root` under a hard deadline |
| Windows `Root.Lstat` and `File.Stat` are both handle-derived and `os.SameFile` does not re-fetch by pathname (D8) | native Windows identity test: true for an unchanged file, false after an injected replacement |
| The Windows reparse-tag → mode mapping and the `winsymlink` fallback (D7) | injected-`FileInfo` mapping test on all targets + native junction test |
| `os.Root`'s escape sentinel is unexported and undiscriminable | out-of-root raced-link test landing on `unreadable` |
| The stdlib build-tag expression the allowlist copies (D5) | text comparison against `$GOROOT/src/os/root_openat.go`'s tag |

**Maintenance obligation.** A Go **minor**-version bump requires re-reading
every `tripwire` row before the bump lands. A red tripwire test at upgrade time
is the designed failure mode; a silent behavior change in the field is the
failure this decision exists to prevent.

### D11 — `internal/rescap` is not migrated, and its `GatePath` is not replaced

This slice introduces a rooted namespace for a **new**, read-only inspection
package. It does **not**:

- migrate `rescap.GatePath` onto `os.Root`;
- replace, deprecate or modify the shipped pathname gate;
- change any `rescap` refusal vocabulary, lock/scratch lifecycle, or cap;
- touch `internal/rescap/pathopen_windows.go`'s compile-only stub.

**Rationale.** `rescap` is a *capture* domain with a lock and scratch-tree
lifecycle, a missing-path-**is**-a-refusal policy, and its own refusal
vocabulary and ADR lineage (ADR-033). `prepare --check` needs the opposite leaf
policy — "missing" is an ordinary reportable state, not a refusal — and needs
no lock at all. Converting `rescap` would be a behavior change to a shipped
surface, in a different domain, with its own review obligations.

What this slice **does** take from `rescap` is *policy and reasoning*, cited
rather than imported: the ancestor-walk refusal policy, and the cap-plus-one
bounded-read discipline whose stated reason is that a pre-read `Stat().Size()`
check is bypassed by a file that grows. `rescap`'s own `readBounded`
accumulates into a growable `append` buffer, which is precisely the allocation
shape D9 replaces — the discipline is inherited, the allocation shape is not.

A wave that "helpfully" migrates `rescap` while implementing this PRD has
exceeded its scope. Any future migration needs its own PRD and its own ADR.

### D12 — A narrow, single-implementation operation seam exists for tests, and nothing else

`Inspect` takes an interface pair rather than a concrete `*os.Root`:

```go
type RootOps interface {
    Lstat(name string) (fs.FileInfo, error)
    OpenFile(name string, flag int, perm fs.FileMode) (FileOps, error)
}

type FileOps interface {
    Stat() (fs.FileInfo, error)
    Read(p []byte) (int, error)
    Close() error
}
```

Constraints, all mechanically asserted:

1. **Exactly one non-test implementation of each interface exists**, an
   unexported adapter over `*os.Root` / `*os.File`. Every other implementation
   lives in a `_test.go` file.
2. **The interfaces are five methods wide in total**, and none takes or returns
   an absolute path. There is no `ReadDir`, no `Walk`, no `Readlink`, no
   `Create` and no pathname-taking method of any kind — which makes "the
   inspector cannot mutate and cannot enumerate" a *type-level* property rather
   than a convention.
3. **Deterministic before/after hooks belong to the test implementations
   only.** The production adapter struct has exactly one field and carries no
   hook.
4. **The seam does not weaken the forbidden-reader guards.** Source scans cover
   the whole package including test files; a test may call `os.WriteFile` /
   `os.Symlink` / `os.Mkdir` to *build* a fixture tree, but no file — production
   or test — may read an intent artifact or `status.json` through a pathname
   reader.

**Rationale.** Without this seam roughly two dozen acceptance rows — every
injected `fstat` failure, every injected read error, and every deterministic
"replace the object between step N and step N+1" race — are unimplementable
except through a real hostile filesystem or a production-visible package
variable. rev-2 specified those rows against a concrete `*os.Root` and so
specified tests that cannot be written. The seam is the minimum surface that
makes them deterministic, and constraint 1 is what keeps it from becoming a
general-purpose injection point.

### D13 — Native Windows CI is mandatory, and its junction fixtures must fail rather than skip

The CI matrix is `[ubuntu-latest, macos-latest]` today. `windows-latest` is a
**required** addition, landing in the same slice as the Windows-relevant code
(PRD §17 S1), not deferred to a docs pass. Cross-building for `GOOS=windows`
proves the code compiles and proves nothing about D7's mapping, D8's
handle-derived identity, or the `GetFileType`-derived kinds.

**Junction fixtures use `cmd /c mklink /J` and must `t.Fatal`, never
`t.Skip`.** `os.Symlink` creates a symlink (`IO_REPARSE_TAG_SYMLINK`), which is
the case the predicate would catch even if D7's `ModeIrregular` half were
wrong — so a symlink fixture proves nothing about junctions. `mklink /J`
creates a directory junction and, unlike `mklink /D`, requires neither
`SeCreateSymbolicLinkPrivilege` nor Developer Mode, so an ordinary unprivileged
runner can create one.

If junction creation is unavailable or fails, the test **fails**. `t.Skip`,
`t.Skipf` and `t.SkipNow` are forbidden in the junction-fixture helper and in
any test consuming it; the only permitted guard is `runtime.GOOS != "windows"`,
which is a *platform* skip on non-Windows runners rather than a *capability*
skip on Windows. A skipped Windows test is indistinguishable from a passing one
in a green CI log, and silent non-execution of the Windows path is the exact
risk this decision exists to remove.

### D14 — No provenance persistence, and the provenance ADR trigger stays unfired

This ADR authorizes **no** persistent per-artifact provenance representation.
The companion PRD emits the constant `provenance: unknown` for every artifact,
with the fixed meaning "no trustworthy provenance for this artifact is
available from an accepted source", and evaluates four persistence
alternatives without selecting any.

Consequences:

- **Nothing this ADR decides writes a byte anywhere.** The inspector opens the
  root read-only, the operation seam exposes no mutator (D12 constraint 2), and
  no provenance record, sidecar, status sub-record or attestation is created,
  read or migrated.
- **The WP-005 provenance ADR trigger remains unfired.** It fires on
  *selection of a persistent provenance representation* and on nothing else. A
  future PRD that selects one must write
  `ADR-0NN-intent-artifact-provenance-representation` and have it accepted
  **before** that PRD is accepted for implementation.
- **No backfill, ever, by this slice or by a future migration acting on it.**
  An artifact written before any provenance representation existed has no
  accepted source, so `unknown` is its correct final answer rather than a
  placeholder.
- **This ADR must not be cited as provenance precedent.** It is a filesystem
  *access* boundary. It says nothing about what may be persisted about an
  artifact's authorship.

---

## Consequences

**Accepted costs**

- A flat 4,194,305-byte allocation on every invocation, including aborts (D9).
- A mandatory native Windows runner, roughly a third more CI minutes (D13).
- A maintenance obligation on every Go minor-version bump: re-read thirteen
  tripwire claims (D10).
- Two build-tagged constant files and two build-tagged `openFlags()` files in a
  package that is otherwise platform-neutral (D5, D6).
- An interface indirection on the hottest path of a command whose whole job is
  five stats and five reads (D12).

**Accepted limits, stated as decisions rather than gaps**

- Bytes read may physically originate outside the repository's filesystem (D2).
- Workspace discovery is unhardened and `--path` is trusted (D3).
- Five identity-observation limits are permanent properties of the mechanism,
  not backlog items (D8).
- The design is confined on `unix`, `windows` and `wasip1`, and **refuses**
  everywhere else (D5).

**Obligations created**

- `cmd/tpatch/main.go` gains `//go:debug winsymlink=1` (D7).
- `.github/workflows/ci.yml` gains `windows-latest` (D13).
- No shipped string, doc page or skill surface may restate a confinement,
  identity or no-follow claim more strongly than D2, D6 and D8 permit; a
  mechanical guard covers the strings, this ADR and the companion PRD.

**Explicitly unchanged**

- `internal/rescap` and its `GatePath` (D11).
- `safety.EnsureSafeRepoPath` everywhere outside `internal/intent` (D4).
- Every `--manual` gate, `next`, `cycle`, `verify`, `status` and `doctor`
  behavior — this ADR authorizes a read-only inspector and nothing else.
- Provenance: the companion PRD emits the constant `unknown` and selects no
  persistent representation, so the provenance ADR trigger stays unfired.

---

## Alternatives considered

| Alternative | Verdict |
|---|---|
| **Keep the `rescap` pathname `GatePath` model** (rev-1) | Rejected. Every check and the final open re-resolve the *name* independently, so the kernel re-resolves the ancestor chain at open time with whatever the tree looks like then. Its Windows half is an explicitly unsupported compile-only stub with a `isSymlinkLoopError` that "always reports false on this target". Reusable as *policy*, not as *mechanism* — which is exactly how D11 reuses it. |
| **Hand-rolled `openat` / `NtCreateFile` resolver** | Rejected. It is precisely the code `os.Root` is: a per-component, descriptor-relative resolver with symlink re-resolution, a `..`-escape check and a symlink-count limit, in two platform halves. Writing it ourselves means owning the TOCTOU-correctness of a security primitive, per platform, forever, with no upstream review — for a read-only inspector. The `rescap` Windows stub is the empirical evidence that this project does not have the appetite to maintain that. |
| **Refuse to ship on any platform where confinement is uncertain (compile-time refusal)** | Rejected as the *default*, retained as an open question (PRD §21 Q8). A build constraint would break `go build ./...` for anyone cross-compiling the whole module to a non-allowlisted target, which the repo does not currently forbid. D5's runtime abort is total and testable, and with the allowlist form it also covers unknown future targets — which was the compile-time option's real attraction. |
| **`os.Root` (chosen)** | **Chosen.** A standard-library, reviewed, per-component rooted resolver available on every target this project ships to, with an explicit and *documented* statement of what it does not cover (D2's quoted paragraph) — which is what makes an honest scope statement possible at all. The residual work is stating the boundary correctly, not implementing the boundary. |
| **A denylist of unconfined platforms** (rev-2) | Rejected. Fail-open: a future `GOOS`, or an existing one Go later reclassifies, lands silently in the confined branch. D5 inverts it. |
| **Passing caller-side `O_NOFOLLOW` "belt-and-braces"** (rev-2) | Rejected. `Root` consumes it and converts the signal into an in-root resolution, so it produces no observable effect — a false affordance that invites the next reader to infer a guarantee that does not exist. D6 removes it and keeps the explanation. |
| **One buffer per capture** (rev-2) | Rejected. Correct on shape, wrong on total: five captures means a ~20 MiB worst case that rev-2 never totalled. D9 reuses one buffer sequentially and states the flat cost. |
| **Claiming "a different object is never read"** (rev-2) | Rejected as unprovable. `os.SameFile` compares identity numbers; hard-link aliases, ID reuse and swap-and-restore all defeat it, and the first is undetectable by construction. D8 weakens the promise to "observed as different" and enumerates the limits. |
| **Testing `Inspect` against a concrete `*os.Root`** (rev-2) | Rejected. It leaves every injected-failure and deterministic-race row unimplementable. D12 defines the narrowest seam that makes them writable and constrains it to one production implementation. |
| **Migrating `rescap` onto `os.Root` in the same slice** | Rejected and locked out by D11. Different domain, different leaf policy, shipped surface, separate ADR lineage. |

---

## Acceptance dependencies

This ADR is **Proposed**. It becomes **Accepted** only when both it and the
companion PRD rev-3 pass review. Its decisions are verified by the companion
PRD's acceptance matrix; the mapping is:

| Decision | Verified by (PRD §18) |
|---|---|
| D1 one root, opened/closed once, owned by the CLI | AVP-141, AVP-142, AVP-143 |
| D2 logical, not physical, confinement | AVP-189 (over-claim guard incl. this ADR), AVP-190 (the four leaf shapes), AVP-149 |
| D3 discovery outside the capture; `--path` trusted | AVP-008, AVP-183, AVP-184 |
| D4 `fs.ValidPath` names; no `EnsureSafeRepoPath` | AVP-144, AVP-089 |
| D5 fail-closed platform allowlist | AVP-191 (tag text + sensitivity), AVP-177 (runtime abort), AVP-178, AVP-179 |
| D6 `O_NONBLOCK` only; no caller `O_NOFOLLOW` | AVP-118, AVP-107, AVP-200 (tripwire) |
| D7 Windows tag mapping + `winsymlink=1` | AVP-198, AVP-176, AVP-146 |
| D8 observed-symlink refusal, pre/post walks, identity limits | AVP-145, AVP-148, AVP-151, AVP-152, AVP-195, AVP-196 |
| D9 one reused scratch buffer, stated cost, cap↔message coupling | AVP-197, AVP-170, AVP-171, AVP-174, AVP-201 |
| D10 contract/tripwire split and upgrade gate | AVP-176, AVP-198, AVP-200, and §23.2's classified table |
| D11 no `rescap` migration | AVP-180, AVP-089, AVP-150 |
| D12 single-implementation seam | AVP-194 |
| D13 native Windows CI; fail-not-skip junctions | AVP-175, AVP-176, AVP-199 |
| D14 no provenance persistence; trigger unfired | AVP-059, AVP-060, AVP-063, AVP-129 |

**Status of the artifacts, and of the whole slice:** the status inspection
(`status.json`) is captured under the **same** boundary as the four artifacts —
same root, same component policy with pre/post walks, same `openFlags()`, same
identity and kind rechecks, same shared scratch buffer, its own cap — and is
subject to the same valid-state gate before any value is echoed. There is no
second, weaker path for metadata.

**No implementation is authorized by this document.** It records decisions for
a planning slice; PRD §17's S1–S5 remain gated on acceptance of both documents,
and `PRD-prepare-intent-bundle.md` remains blocked.

---

## References

- [PRD-artifact-validation-and-provenance](../prds/PRD-artifact-validation-and-provenance.md)
  rev-3 — companion; §7 (path policy and capture), §14 (security), §18 (matrix),
  §23 (claims audit).
- [WP-005 Spec-driven workflows](../whitepapers/WP-005-spec-driven-workflows.md)
  — `## Agreed`, Turns 2–4.
- [ADR-027 — Capture context privacy boundary](./ADR-027-capture-context-privacy-boundary.md).
- [ADR-033 — Resource capture boundary](./ADR-033-resource-capture-boundary.md).
- GH #10 — define truthful intent-artifact validation and provenance.
- Go 1.26 `os` package: `root.go` (`Root` doc comment, `OpenRoot`,
  `rootMaxSymlinks`), `root_openat.go` (`doInRoot`, build tag),
  `root_unix.go` (`rootStat`, `rootOpenFileNolog`, `checkSymlink`),
  `root_windows.go` (`rootStat`), `root_noopenat.go` (build tag),
  `stat_windows.go` (`statHandle`, `(*File).Stat`),
  `types_windows.go` (`(*fileStat).mode`, `Mode`, `modePreGo1_23`,
  `isReparseTagNameSurrogate`, `newFileStatFromGetFileInformationByHandle`,
  `sameFile`, `loadFileId`), `types_unix.go` (`sameFile`), `file.go`
  (`errPathEscapes`); `io/fs` (`ValidPath`); `io` (`ReadFull`);
  `internal/godebugs/table.go` (`winsymlink`).
- `internal/rescap/pathgate.go:50-54,68-83,97-120,133-155,181-190`;
  `internal/rescap/content.go:9-11,29-32,50-70`;
  `internal/rescap/pathopen_unix.go:20-28`;
  `internal/rescap/pathopen_windows.go:1-20`.
- `internal/store/store.go:23-40` (`FindProjectRoot`), `:351-361`
  (`LoadFeatureStatus`); `internal/safety/safety.go:11-28`;
  `cmd/tpatch/main.go:1-11`; `.github/workflows/ci.yml:20-25,37-58`;
  `go.mod:3`.

# ADR-034 — Rooted Filesystem Inspection Boundary

**Status**: Proposed — Awaiting Review (rev-2)
**Date**: 2026-08-13 (Proposed)
**Owner**: Core (planning lane)
**Byline**: writer sub-agent, rev-2 at HEAD `70876c1`
**Cluster**: WP-005 spec-driven workflows / GH #10
**Supersedes**: none
**Superseded by**: none
**Depends on**: [ADR-027](./ADR-027-capture-context-privacy-boundary.md) (D2 no raw
context, D6 no wall-clock), [ADR-033](./ADR-033-resource-capture-boundary.md)
(D10 no tracked timestamps, D11 no Go map in a wire schema)
**Companion**: [PRD-artifact-validation-and-provenance](../prds/PRD-artifact-validation-and-provenance.md)
(rev-5, Draft — Awaiting Review). **The two documents are reviewed together.**
Read the PRD for the full design and its acceptance matrix; this ADR states the
decisions the PRD's §7 depends on, and where the two overlap **this ADR is
normative**.
**Blocks**: implementation of `tpatch prepare <slug> --check` (PRD §17 slices
S1–S5). No implementation is authorized until both documents are accepted.

**Revision history**

| Rev | Disposition | What changed |
|---|---|---|
| rev-0 | NEEDS REVISION (internal and external) | First draft: D1–D14. |
| rev-1 | NEEDS REVISION (internal), APPROVED WITH NOTES (external) | **D5** narrows the confinement allowlist from `unix \|\| windows \|\| wasip1` to **`unix \|\| windows`**; `wasip1` becomes an unsupported target that aborts, the "byte-identical to the stdlib tag" justification is withdrawn in favour of an asserted **subset** relation, and **no split implementation** is authorized. **D6** scopes `O_NONBLOCK` to the *open*. **D7**'s name-surrogate column is corrected to the `0x20000000` bit test rather than a two-tag list. **D8** records that bytes read through an unobserved consistent alias are **attributed to the canonical name**. **D9** corrects the `io.ReadAll(io.LimitReader(...))` rationale — it is bounded but variable and repeated, not unbounded — and states the one-time ~4 MiB zeroing cost. **D12** adds an injectable `SameFile` to the seam, making it three + three methods with **exactly two** production adapters, one per interface. **Four new decisions**: **D15** descriptor `Close` contract and close-failure precedence; **D16** withdrawal of every bounded-runtime guarantee; **D17** Cobra parse-error ownership under `SilenceUsage`/`SilenceErrors`; **D18** the deliberate exit-3 workspace divergence. |
| rev-2 | this document | Final no-decision-change correction: advisory catalog count ten; companion PRD rev-5 labels synchronized. |

---

## Context

`tpatch prepare <slug> --check` (PRD rev-5) is a **read-only** inspector that
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
  appears after the `Lstat`; its **open** of a writer-less FIFO blocks.
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
reinserts rev-2's exact sentence. Its timing-dimension sibling is PRD AVP-207
(D16), which owns the complementary set of termination claims; the two guards
partition the over-claim surface and do not overlap.

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
used by the component walk — is asserted to satisfy `fs.ValidPath`. The
property that predicate enforces is defined by the **`io/fs` package
documentation's `# Path Names` section** (UTF-8-encoded, unrooted,
slash-separated, no `.`/`..`/empty element, no leading or trailing slash), with
the documented special case that the bare name `"."` is valid and denotes the
root directory. `ValidPath`'s own doc comment states only that the name is
valid for `Open` and links to that section; rev-0 of this ADR and PRD rev-3
attributed the whole property list to the function comment, which is the
anchoring error PRD G18 now corrects. No name this design composes is `"."`, so
the special case changes nothing here — it is recorded so a future refactor
does not assume `fs.ValidPath` refuses it.

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

### D5 — Platform support is a fail-closed allowlist, and a deliberate strict subset of the standard library's confined set

`internal/intent` carries a build-tagged `rootConfinementSupported` constant in
exactly two files:

| File | `//go:build` expression | Value |
|---|---|---|
| `confine_supported.go` | `unix \|\| windows` | `true` |
| `confine_unsupported.go` | `!(unix \|\| windows)` | `false` |

When the constant is `false` the command aborts `workspace-unsupported-platform`
(exit 3) **before** `os.OpenRoot` is called and before any name is composed.
This is a refusal, not a degraded mode.

**Why an allowlist and not a denylist.** rev-2 of the companion PRD used
`(js && wasm) || plan9` → `false` with its negation → `true`. That is
**fail-open**: it hard-codes the claim that today's two unconfined targets are
the only ones there will ever be. A `GOOS` added by a future Go release, or an
existing one Go later reclassifies, lands silently in the `true` branch and the
command asserts a confinement guarantee nobody checked. `js/wasm`, `plan9` and
any future unmatched `GOOS` must all abort, and only the allowlist form
delivers that.

**Why `unix || windows` and not the stdlib's `unix || windows || wasip1`
(rev-1 narrowing).** ADR rev-0 admitted `wasip1` on one ground: the expression
was byte-identical to `$GOROOT/src/os/root_openat.go`'s own build tag, so "the
inspector's allowlist *is* the standard library's allowlist". **That
justification is withdrawn.** It reasons about the stdlib's implementation set
when the question is *this design's* implementation set, and the two are not
the same:

1. **This design declares exactly two implementation halves.** D6's
   `openFlags()` is build-tagged `!windows` / `windows`, and the non-Windows
   half returns `syscall.O_NONBLOCK` with FIFO/character-device semantics that
   D10's tripwire test (PRD AVP-200) exercises on a real Unix FIFO. A WASI
   preview-1 host does not provide those semantics, so admitting `wasip1` to
   the `true` branch would assert a property the `!windows` half does not
   deliver there.
2. **No `wasip1` runner, fixture or cross-build is proposed.** D13 makes a
   native Windows runner mandatory precisely because an unexecuted platform
   path degrades silently. Claiming a third confined target while proposing no
   runner for it reproduces that defect one platform over.
3. **Splitting the implementation is out of scope of this ADR.** A `wasip1`
   half would need its own `openFlags()` file, its own kind story and its own
   tripwire. **No split implementation is authorized here.** A future ADR may
   add one; until then the target is refused.

**The asserted property is therefore subset-ness, not textual identity:**

- the pair is exactly `unix || windows` and its exact syntactic negation;
- the two sets are exhaustive and disjoint over every `GOOS` in
  `go tool dist list`;
- every `GOOS` in the `true` set is **also** matched by
  `$GOROOT/src/os/root_openat.go`'s build tag — the design never claims
  confinement where the standard library does not implement the confined
  resolver;
- the subset is **proper**: `wasip1` is matched by the stdlib tag and not by
  ours, and that gap is a recorded refusal rather than an oversight.

PRD AVP-191 owns the tag texts and the exhaustive/disjoint property; **PRD
AVP-208 (new in rev-1)** owns the subset relation, the proper-subset assertion
for `wasip1`, and the sensitivity fixture that fails when `wasip1` is re-added
without a `wasip1` `openFlags()` half and a runner.

### D6 — The open carries `O_NONBLOCK` on Unix and nothing on Windows; caller-side `O_NOFOLLOW` is removed

`openFlags()` is build-tagged and returns:

- non-Windows: `syscall.O_NONBLOCK`, and nothing else;
- Windows: `0`.

No write, create, truncate or append bit ever appears, on any target. There is
no raw `syscall.CreateFile`, no `windows.Openat`, and no call into
`internal/rescap`'s open helpers.

**`O_NONBLOCK` is load-bearing for the *open*, and is an implementation
dependency rather than a contract.** It covers exactly the raced window in
which a FIFO or blocking character device replaces the leaf after the pre-open
kind check refused the stable case. Nothing in `os`'s documentation promises
that caller flags reach `openat`; the behavior is read from unexported code
(`root_unix.go` → `rootOpenFileNolog` → `unix.Openat` /
`unix.HasNonblockFlag`). It is therefore recorded as a **tripwire** (D10) with
a dedicated Go-upgrade test that opens a real writer-less FIFO through a real
`os.Root` under a hard deadline.

**Its scope is the open and nothing else (rev-1 clarification).** `O_NONBLOCK`
does not bound a subsequent `read(2)` and does not bound `Root.Lstat`. The
property this flag buys is "a FIFO with no writer cannot wedge the open"; it is
**not** "this command terminates". D16 states the general withdrawal, and no
document, message or test may present this flag as a termination guarantee.

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

**The "name surrogate" column is a *bit test*, not a tag list (rev-1
correction).** `isReparseTagNameSurrogate()` is
`FileAttributes&FILE_ATTRIBUTE_REPARSE_POINT != 0 && ReparseTag&0x20000000 != 0`.
Its source comment says "True for `IO_REPARSE_TAG_SYMLINK` and
`IO_REPARSE_TAG_MOUNT_POINT`", which is true of today's common tags but is not
the predicate. rev-0's table reproduced the comment as if it were the rule and
so answered "no" for *any other tag*, which is wrong: any tag carrying the
`0x20000000` bit is a surrogate. The distinction is observable — a surrogate
has its `ModeDir` and `GetFileType`-derived bits **suppressed**, a
non-surrogate keeps them.

| Reparse tag | Name surrogate? (`0x20000000` bit) | Mode bits | Refused by the `ModeSymlink\|ModeIrregular` predicate? |
|---|---|---|---|
| `IO_REPARSE_TAG_SYMLINK` (`0xA000000C`) | **yes** | `ModeSymlink`; `ModeDir`/`GetFileType` bits suppressed | yes |
| `IO_REPARSE_TAG_MOUNT_POINT` (junction, `0xA0000003`) | **yes** | `ModeIrregular` via the `default` branch; `ModeDir` and `GetFileType`-derived bits suppressed | yes |
| `IO_REPARSE_TAG_AF_UNIX` (`0x80000023`) | **no** — surrogate bit clear | `ModeSocket`, **and** its `ModeDir`/`GetFileType` bits are **not** suppressed | **no** — closed by the `!IsRegular()` kind gate |
| `IO_REPARSE_TAG_DEDUP` (`0x80000013`) | **no** — surrogate bit clear | **no type bit**; deliberately treated as a regular file by Go, with an explanatory comment; keeps its `GetFileType`-derived bits | **no, and deliberately so** — read as the regular file Go says it is |
| any other tag | **depends on that tag's own `0x20000000` bit** — a future or vendor tag may be either | `ModeIrregular` via `default` in both cases; only the suppression differs | yes |

Three decisions follow:

1. **`ModeSymlink|ModeIrregular` is a *refusal* predicate, not the kind
   policy.** It is necessary and **not sufficient**. rev-2 of the PRD's "every
   other reparse tag sets `ModeIrregular`" is withdrawn: AF_UNIX sets
   `ModeSocket` and DEDUP sets no type bit at all. The `!IsRegular()` gate
   (applied both pre-open from `Lstat` and post-open from the descriptor stat)
   is what makes the kind policy total.
2. **A DEDUP reparse point is inspected as an ordinary regular file.** This
   accepts the standard library's own documented reasoning — DEDUP files
   support ordinary random-access reads and should not flip from regular to
   irregular when the Data Deduplication job runs — and it is recorded here as
   a decision rather than left as an omission.
3. **The two exceptions are exceptions to the *surrogate* rule as well as to
   the *mode* rule.** Because AF_UNIX and DEDUP are not surrogates, a directory
   carrying either tag still reports `ModeDir`, and a pipe/char handle still
   reports its `GetFileType` bits. That is the reason the kind gate — not the
   refusal predicate — is what must be total.

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
compared through the seam's `SameFile` operation (D12), whose one production
implementation delegates to `os.SameFile`. A mismatch ends the capture with
`unstable` and **zero bytes read**.

**The promise, stated exactly:**

> An object **observed as different** is never read.

rev-2 of the PRD said "a *different* object is never read". That is withdrawn
as unprovable. `os.SameFile` compares identity *numbers*, and five cases
separate "different" from "observed as different". All five are documented in
the PRD (§7.4.4, §8.3), asserted as *limits* rather than capabilities, and may
not be claimed away anywhere:

1. **Same-length in-place rewrite** — undetectable by size, kind or identity.
2. **Same-identity alias** — a raced in-root link resolving to the very object
   observed. The report remains true of the object read; no refusal is claimed.
   **Attribution, stated in rev-1**: those bytes are labelled in the report
   with the **canonical artifact path**, so a consistent in-root alias is
   attributed to the canonical name. The claim is "the object I classified is
   the object I read", **not** "the canonical name designated this object at
   every instant".
3. **Hard-link alias** — undetectable **by construction**: a hard link *is* the
   same inode / file index. No additional check inside this design could
   distinguish the two names, and the same attribution note applies.
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
  forbidden.

**Why they are forbidden, stated correctly (rev-1 correction).** rev-0 of this
ADR wrote that rev-1 of the PRD's `io.ReadAll(io.LimitReader(f, Max+1))`
"bounded the result *length* but not the *allocation*". **That is false, and it
over-corrected an earlier false claim in the opposite direction.** The limit
reader caps the result at `Max+1` bytes, so the total allocation is `O(Max)` —
**bounded**. What it is not is *fixed*: `io.ReadAll` grows by `append`, so one
capture performs a sequence of increasing allocations with copies between them,
and the sequence is paid again on each of the five sequential captures. The
rejection is therefore about cost **shape**, not boundedness:

- one allocation for the whole invocation instead of a growth sequence per
  capture;
- a flat footprint that does not vary with artifact size, so the worst case
  equals the common case;
- no copy-on-grow inside a capture.

What *was* false in PRD rev-1 was its "exact allocation ceiling" claim, which
`ReadAll` does not provide. No document may now claim `ReadAll` over a
`LimitReader` is unbounded (PRD G20, G21).

**The cost is stated honestly rather than hidden.** The command pays a flat
4,194,305 bytes for **every** invocation, including a run that aborts before
capturing anything and a run against four 12-byte files. Because
`make([]byte, n)` yields a **zeroed** slice (PRD G22), it also pays one ~4 MiB
zeroing pass — once per invocation, never per capture. This is a deliberate
trade: a fixed, predictable ~4 MiB plus one zeroing against five variable
allocation sequences with copy-on-grow. rev-2 of the PRD's per-capture form had
a ~20 MiB worst case it never totalled. The companion PRD's Q9 records lazy
allocation as a revisable alternative.

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
not**. The companion PRD's §23.2 carries the full classified table (24 claims:
10 contract, 14 tripwire). The tripwires this design actually depends on are:

| Dependency | Runtime tripwire test |
|---|---|
| `Root.OpenFile` forwards caller flags to `openat` (`O_NONBLOCK` non-wedging **open**, D6) | real writer-less FIFO opened through a real `os.Root` under a hard deadline |
| Windows `Root.Lstat` and `File.Stat` are both handle-derived and `os.SameFile` does not re-fetch by pathname (D8) | native Windows identity test: true for an unchanged file, false after an injected replacement |
| The Windows reparse-tag → mode mapping, the `0x20000000` surrogate bit test and the `winsymlink` fallback (D7) | injected-`FileInfo` mapping test on all targets + native junction test |
| `os.Root`'s escape sentinel is unexported and undiscriminable | out-of-root raced-link test landing on `unreadable` |
| The stdlib build-tag expression the allowlist is a subset of (D5) | subset comparison against `$GOROOT/src/os/root_openat.go`'s live tag |
| `io.ReadAll`'s growth sequence (D9's cost-shape rationale) | allocation-counting fixture over a full run |

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

### D12 — A narrow operation seam exists for tests, with exactly one production implementation per interface, and nothing else

`Inspect` takes an interface pair rather than a concrete `*os.Root`:

```go
type RootOps interface {
    Lstat(name string) (fs.FileInfo, error)
    OpenFile(name string, flag int, perm fs.FileMode) (FileOps, error)
    SameFile(a, b fs.FileInfo) bool
}

type FileOps interface {
    Stat() (fs.FileInfo, error)
    Read(p []byte) (int, error)
    Close() error
}
```

Constraints, all mechanically asserted:

1. **Exactly two non-test implementations exist — one per interface**, both
   unexported adapters declared in `internal/intent`: `osRootOps` over
   `*os.Root`, and `osFileOps` over `*os.File`. `OpenFile` returns
   `osFileOps`, **not** a bare `*os.File`: rev-0 of this ADR returned the
   `*os.File` directly, which made the production `FileOps` a type declared in
   `os` and left PRD AVP-194's "declared outside a `_test.go` file" assertion
   with nothing in this package to point at. Every other implementation of
   either interface lives in a `_test.go` file.
2. **The interfaces are six methods wide in total** — three and three — and
   none takes or returns an absolute path. There is no `ReadDir`, no `Walk`,
   no `Readlink`, no `Create` and no pathname-taking method of any kind — which
   makes "the inspector cannot mutate and cannot enumerate" a *type-level*
   property rather than a convention.
3. **`SameFile` is part of the seam (rev-1 addition), and this is the point of
   the rev-1 change.** `os.SameFile` is only meaningful over the **unexported**
   `*os.fileStat` values `os` produces — it type-asserts and compares
   `Dev`/`Ino` or `vol`/`idxhi`/`idxlo` — so no test outside `os` can construct
   an input that yields a chosen verdict. With a package-level `os.SameFile`
   call in the capture path, every acceptance row needing a *chosen* identity
   answer (PRD AVP-084, AVP-151, AVP-160, AVP-196 (b)) is unwritable except
   against a real hostile filesystem, which is exactly the defect this ADR
   fixed for `Lstat`/`OpenFile` and left in place one line later. The
   production body is exactly `return os.SameFile(a, b)`, so production
   behavior is unchanged, and PRD AVP-206 asserts `os.SameFile` appears at
   exactly one production call site.
4. **`Close` is part of the seam because its failure is a classified
   outcome**, not a discarded error — see D15.
5. **Deterministic before/after hooks belong to the test implementations
   only.** Each production adapter struct has exactly one field (`root`,
   `file`) and carries no hook.
6. **The seam does not weaken the forbidden-reader guards.** Source scans cover
   the whole package including test files; a test may call `os.WriteFile` /
   `os.Symlink` / `os.Mkdir` to *build* a fixture tree, but no file — production
   or test — may read an intent artifact or `status.json` through a pathname
   reader.

**Rationale.** Without this seam roughly two dozen acceptance rows — every
injected `fstat` failure, every injected read error, every injected close
error, every chosen identity verdict, and every deterministic "replace the
object between step N and step N+1" race — are unimplementable except through a
real hostile filesystem or a production-visible package variable. rev-2 of the
PRD specified those rows against a concrete `*os.Root` and so specified tests
that cannot be written. The seam is the minimum surface that makes them
deterministic, and constraint 1 is what keeps it from becoming a
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

### D15 — The descriptor is closed exactly once per open, and a close failure is a classified outcome

rev-0 of this ADR declared `Close` on `FileOps` and then never used it: no
decision, capture step or ladder row consumed it. rev-1 closes that gap.

1. **Exactly one `Close` per successful `OpenFile`, on every path.** The close
   is unconditional — never inside a success-only branch — and covers every
   path that abandons a capture after the open, including every `unstable` and
   `unreadable` outcome and every abort. PRD AVP-205 counts opens against
   closes over every post-open ladder row and asserts zero descriptors
   outstanding when `Inspect` returns, with sensitivity fixtures for a skipped
   close and a double close.
2. **The close is the *last* filesystem operation of the capture**, after the
   post-capture component walk (D8). Holding the descriptor across that walk
   keeps the object pinned so it cannot be unlinked and its identity reclaimed
   while the ancestors are being re-observed — which is the window the walk
   exists to inspect.
3. **A close failure is a capture-level failure, not a content fact**, and is
   evaluated **after** the last descriptor-scoped probe and **before** any
   content classification: PRD §7.5 row 20c for an artifact, §9.4.2 row 16a for
   `status.json`. The captured bytes are discarded and no content state is
   reported.
4. **First-match-wins is preserved.** A capture that already matched an earlier
   ladder row keeps that row; the close row neither overwrites nor suppresses
   an upstream verdict.
5. **No new state, reason code, advisory or abort code is minted.** A close
   failure is `unreadable` / `artifact-unreadable` for an artifact,
   `analysis-sidecar-unreadable` for the sidecar advisory, and
   `status-unreadable` for `status.json`. The abort catalog stays at thirteen
   codes, the reason catalog at ten and the advisory catalog at ten, so
   every totality assertion keeps its arithmetic. A code earns its place by
   changing the remediation, and "could not be read and closed cleanly — check
   permissions and the filesystem, then run `tpatch doctor`" is the same
   remediation whichever syscall failed. The `status-unreadable` message and
   lifecycle annotation are widened to that wording so they stay true of all
   six of their rows.

### D16 — No bounded-runtime guarantee: the cost guarantees are about space, not time

rev-0 of this ADR and rev-3 of the companion PRD asserted, in several places,
that the command "has no unbounded wait anywhere", that "no leaf kind can hang
it" and that "nothing hangs". **All such claims are withdrawn.**

**What is guaranteed:**

- **Bounded allocation** — exactly one data buffer per invocation (D9).
- **Bounded bytes requested** — at most `MaxArtifactBytes+1` from any one
  descriptor, `MaxStatusBytes+1` from the status descriptor.
- **A bounded number of operations** — five captures, each with a fixed number
  of `Lstat`s, one open, one bounded read and one close; no retry, no spin, no
  lock (D8, D15).
- **A non-wedging open on Unix** — `O_NONBLOCK` keeps a FIFO or blocking
  character device from wedging `open(2)` (D6).

**What is not guaranteed: termination.** `O_NONBLOCK` bounds the open, not the
read. The Windows kind refusal handles the *stable* non-regular kinds, not a
slow regular file. Neither addresses an ordinary regular file served by a
stalled NFS/SMB mount, a wedged FUSE server, a `/proc`-style provider or an
unresponsive device driver, on which `read(2)` — and even `Lstat` — can block
indefinitely. v1 adds no deadline, no `context.Context` and no watchdog.

**Consequences, stated rather than implied:**

- The security property is **confidentiality and integrity, not
  availability**. This command is not hardened against denial of service.
- The absence of `--timeout` is justified by "no provider, network or
  subprocess wait to bound, and no cancellation contract defined in v1" — not
  by "nothing can wait". Go's `os` file reads are not context-cancellable and
  `SetReadDeadline` does not apply to ordinary files, so a `--timeout` that
  could not interrupt a read would be the same class of false affordance as
  D6's removed `O_NOFOLLOW`. The companion PRD's Q11 records the additive
  alternative.
- **No shipped string, doc page, skill surface, PRD sentence or ADR sentence
  may claim the command cannot hang, always terminates, has bounded runtime,
  or is safe in a non-cancellable preflight step.** PRD AVP-207 is the
  mechanical guard, with sensitivity fixtures drawn from rev-3's own removed
  sentences. It is AVP-189's timing-dimension sibling and the two do not
  overlap: AVP-189 owns confinement claims, AVP-207 owns termination claims.

### D17 — Cobra parse errors are printed by this repository, from third-party text, and are outside the command's schema and security guarantee

The root command is built with `SilenceUsage: true` and `SilenceErrors: true`
(`internal/cli/cobra.go:56-62`). Two consequences follow, and rev-0 of this ADR
and rev-3 of the PRD stated both incorrectly:

1. **Cobra prints no usage block.** rev-3's "the usage block cobra prints with
   them" describes behavior this binary does not have. No document, test or
   acceptance row may expect one.
2. **Cobra prints no error either.** It returns the parse error from
   `rootCmd.Execute()`; **this repository's own printer** writes
   `error: %v` to stderr (`internal/cli/cobra.go:33-39`), after which
   `exitCodeFor` finds no `*ExitCodeError` and returns `1`
   (`internal/cli/cobra.go:43-52`).

So the exit-1 population is a **repository-emitted line wrapping third-party
error text**. The text is produced by `spf13/pflag` or `spf13/cobra` from raw
`os.Args` before `RunE` runs, and pflag interpolates the offending argument
verbatim.

**The decision:**

- Those bytes are **outside** the prepare report schema and **outside** the
  §14.3.3 byte rules. The PRD does not claim they are sanitized, and the fact
  that the repository's own `Fprintf` carries them does not make them the
  command's: the printer is a shared, pre-existing surface applying no
  filtering, and changing it would be a cross-command behavior change no single
  command's PRD may make.
- **The command installs no interception point.** No `FlagErrorFunc`, no
  `SetFlagErrorFunc`, no `SetErr`/`SetOut`, no custom `Args` validator that
  formats its own message, and no local `SilenceUsage`/`SilenceErrors`
  assignment. Installing any of these would move third-party bytes into this
  command's ownership without a schema decision.
- **The guard is scoped and real.** rev-3's AVP-193 sensitivity fixture — "a
  fixture that intercepts and re-renders cobra's error inside `RunE`" — is
  unconstructible, because a parse error is raised before `RunE`; a guard whose
  negative fixture cannot exist proves nothing. rev-1 replaces it with an AST
  half (the absences above, sensitivity = adding a `FlagErrorFunc`) plus a
  behavior half over the five exit-1 inputs (exit 1, empty stdout, exactly one
  `error:` line matching no §9.5 template, no report, `.tpatch/`
  byte-identical).
- **The hostile-slug case is unaffected.** A hostile slug with a well-formed
  flag set parses fine, reaches `RunE` and is refused by `CanonicalSlug` with
  the argument withheld.

### D18 — `prepare --check`'s exit-3 workspace refusal deliberately diverges from every other command's generic exit 1

Every existing command that opens a workspace does so through
`openStoreFromCmd` (`internal/cli/cobra.go:3782-3793`), which returns
`store.FindProjectRoot`'s plain `errors.New("could not find .tpatch in this
directory or any parent")` (`internal/store/store.go:23-40`). That is not an
`*ExitCodeError`, so it becomes generic **exit 1**. `tpatch prepare <slug>
--check` binds the same condition to **exit 3** with abort code
`workspace-not-initialized` and a full report.

This divergence is deliberate and legitimate:

1. **`SPEC.md:135-141` makes exit codes per-command contracts**, not a global
   enum; `verify` already binds a non-1 meaning to `2`.
2. **This command's exit code is a *verdict*, not an error class.** `0`/`2`/`3`
   are the three values of `structural_readiness`, and all three emit the same
   report schema. Collapsing the missing-workspace case to generic `1` would
   make it the only nonzero path with no report, breaking the
   `artifacts` ⇔ `abort` invariant consumers are told to rely on.
3. **It is additive and reversible.** No existing command's exit code changes,
   `openStoreFromCmd` is not modified, and this command does not call it —
   it calls `FindProjectRoot` directly and never opens a `*store.Store`.

The divergence is **disclosed, not claimed away**: a harness author who greps
for exit 1 as "no workspace" across all tpatch commands will not get that
answer here, and the companion PRD's §16.1 requires the exit envelope —
including this row — to be documented in `SPEC.md` alongside the existing
per-command envelopes. A future cross-command convention would need its own
enumerated behavior delta (PRD Q2 tracks the analogous question for exit `4`).

## Consequences

**Accepted costs**

- A flat 4,194,305-byte allocation plus one ~4 MiB zeroing pass on every
  invocation, including aborts (D9).
- A mandatory native Windows runner, roughly a third more CI minutes (D13).
- A maintenance obligation on every Go minor-version bump: re-read fourteen
  tripwire claims (D10).
- Two build-tagged constant files and two build-tagged `openFlags()` files in a
  package that is otherwise platform-neutral (D5, D6).
- An interface indirection on the hottest path of a command whose whole job is
  five stats, five reads and five closes (D12, D15).
- One more interface method than strictly needed for production
  (`RootOps.SameFile`, whose production body is a one-line delegation), paid so
  that a dozen identity rows are writable at all (D12).
- A per-command exit-code contract that differs from every other command's
  handling of a missing workspace (D18).

**Accepted limits, stated as decisions rather than gaps**

- Bytes read may physically originate outside the repository's filesystem (D2).
- Workspace discovery is unhardened and `--path` is trusted (D3).
- Five identity-observation limits are permanent properties of the mechanism,
  not backlog items; bytes read through an unobserved consistent alias are
  attributed to the canonical name (D8).
- The design is confined on `unix` and `windows`, and **refuses** everywhere
  else — including `wasip1`, which the standard library confines but this
  design does not implement (D5).
- **The command has no bounded runtime and no cancellation.** An ordinary read
  can block indefinitely on a stalled filesystem; the security property is
  confidentiality and integrity, not availability (D16).
- The exit-1 parse-error population carries unsanitized third-party text
  through this repository's shared printer (D17).

**Obligations created**

- `cmd/tpatch/main.go` gains `//go:debug winsymlink=1` (D7).
- `.github/workflows/ci.yml` gains `windows-latest` (D13).
- No shipped string, doc page or skill surface may restate a confinement,
  identity, no-follow or **termination** claim more strongly than D2, D6, D8
  and D16 permit; mechanical guards cover the strings, this ADR and the
  companion PRD (PRD AVP-189, AVP-207).
- `SPEC.md` must document this command's exit envelope including the exit-3
  workspace row, so D18's divergence is discoverable (D18).

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
| **A denylist of unconfined platforms** (PRD rev-2) | Rejected. Fail-open: a future `GOOS`, or an existing one Go later reclassifies, lands silently in the confined branch. D5 inverts it. |
| **Including `wasip1` in the allowlist because the stdlib tag does** (ADR rev-0 / PRD rev-3) | Rejected in rev-1. It reasons about the *stdlib's* implementation set when the question is *this design's*: `openFlags()` has two halves, not three; the `O_NONBLOCK` FIFO tripwire is a Unix property a WASI preview-1 host does not provide; and no `wasip1` runner or fixture is proposed, which would reproduce D13's unexecuted-platform defect one target over. **No split implementation is authorized**, so `wasip1` is simply refused, and the asserted property becomes a proper-subset relation (PRD AVP-208). |
| **Claiming the command "has no unbounded wait anywhere"** (ADR rev-0 / PRD rev-3) | Rejected as false. `O_NONBLOCK` bounds the FIFO/device *open*, not any read; the Windows kind refusal handles stable non-regular kinds, not a slow regular file; neither addresses a regular file on a stalled NFS/SMB mount, a wedged FUSE server or an unresponsive driver. D16 withdraws the claim and states what is actually bounded — allocation, bytes requested, operation count and the Unix open. PRD AVP-207 guards it with fixtures drawn from the removed sentences. |
| **Discarding the descriptor `Close` error** (ADR rev-0, by omission) | Rejected. rev-0 declared `Close` on `FileOps` and never called it, leaving both an unspecified descriptor lifetime and a silently swallowed error on captures that otherwise reported a content state. D15 makes the close unconditional and exactly-once, classifies its failure before any content interpretation, and mints no new code. |
| **A fourteenth abort code for a `status.json` close failure** | Rejected. A code earns its place by changing the remediation, and a close failure shares one remediation with a read failure. A separate code would add a fourteenth `--quiet` token, `error:` template and lifecycle line and break four totality assertions, to name a syscall the operator cannot act on differently. D15 routes it to `status-unreadable` and widens that message to stay truthful. |
| **Intercepting cobra's parse error to re-render it** | Rejected, and the rev-0 sensitivity fixture for it was unconstructible: a parse error is raised before `RunE`, so no `RunE` body can intercept it. The real interception point is a `FlagErrorFunc`, and installing one would move third-party bytes into this command's ownership without a schema decision. D17 forbids it and replaces the fixture with an AST absence check plus a behavior half. |
| **Passing caller-side `O_NOFOLLOW` "belt-and-braces"** (PRD rev-2) | Rejected. `Root` consumes it and converts the signal into an in-root resolution, so it produces no observable effect — a false affordance that invites the next reader to infer a guarantee that does not exist. D6 removes it and keeps the explanation. |
| **One buffer per capture** (PRD rev-2) | Rejected. Correct on shape, wrong on total: five captures means a ~20 MiB worst case that rev-2 never totalled. D9 reuses one buffer sequentially and states the flat cost. |
| **`io.ReadAll(io.LimitReader(f, Max+1))`** (PRD rev-1) | Rejected on cost **shape**, not boundedness. The limit reader caps the result, so total allocation is `O(Max)` — bounded. What it is not is fixed: `ReadAll` grows by `append`, so each of five captures pays a variable sequence of allocations and copies. rev-0 of this ADR wrote "bounded the result length but not the allocation", which over-corrected PRD rev-1's false exact-ceiling claim into a second false claim; D9 states both accurately. |
| **Claiming "a different object is never read"** (PRD rev-2) | Rejected as unprovable. `os.SameFile` compares identity numbers; hard-link aliases, ID reuse and swap-and-restore all defeat it, and the first is undetectable by construction. D8 weakens the promise to "observed as different", enumerates the limits, and records that an unobserved consistent alias is attributed to the canonical name. |
| **Testing `Inspect` against a concrete `*os.Root`** (PRD rev-2) | Rejected. It leaves every injected-failure and deterministic-race row unimplementable. D12 defines the narrowest seam that makes them writable and constrains it to one production implementation per interface. |
| **Calling `os.SameFile` directly from the capture path** (ADR rev-0 / PRD rev-3) | Rejected in rev-1. `os.SameFile` is only meaningful over unexported `*os.fileStat` values no test outside `os` can construct, so every row needing a chosen identity verdict was unwritable except against a real hostile filesystem — the same defect D12 fixed for `Lstat`/`OpenFile` and left in place one line later. D12 moves the comparison behind the seam with a one-line production delegation. |
| **Returning a bare `*os.File` as the production `FileOps`** (ADR rev-0) | Rejected in rev-1. It puts the production `FileOps` implementation in package `os`, where the "exactly one implementation declared outside a `_test.go` file" assertion has nothing in `internal/intent` to point at. D12 requires an explicit `osFileOps` adapter so the property is mechanically checkable. |
| **Migrating `rescap` onto `os.Root` in the same slice** | Rejected and locked out by D11. Different domain, different leaf policy, shipped surface, separate ADR lineage. |

---

## Acceptance dependencies

This ADR is **Proposed**. It becomes **Accepted** only when both it and the
companion PRD rev-5 pass review. Its decisions are verified by the companion
PRD's acceptance matrix; the mapping is:

| Decision | Verified by (PRD §18) |
|---|---|
| D1 one root, opened/closed once, owned by the CLI | AVP-141, AVP-142, AVP-143 |
| D2 logical, not physical, confinement | AVP-189 (over-claim guard incl. this ADR), AVP-190 (the four leaf shapes), AVP-149 |
| D3 discovery outside the capture; `--path` trusted | AVP-008, AVP-183, AVP-184 |
| D4 `fs.ValidPath` names; no `EnsureSafeRepoPath` | AVP-144, AVP-089 |
| D5 fail-closed allowlist, `unix \|\| windows`, proper subset of the stdlib set | AVP-191 (tag text + denylist sensitivity), AVP-208 (subset relation + `wasip1` gap), AVP-177 (runtime abort), AVP-178, AVP-179 |
| D6 `O_NONBLOCK` only, scoped to the open; no caller `O_NOFOLLOW` | AVP-118, AVP-107, AVP-200 (tripwire), AVP-207 (scope) |
| D7 Windows tag mapping, surrogate bit test + `winsymlink=1` | AVP-198, AVP-176, AVP-146 |
| D8 observed-symlink refusal, pre/post walks, identity limits, alias attribution | AVP-145, AVP-148, AVP-151, AVP-152, AVP-195, AVP-196 |
| D9 one reused scratch buffer, stated cost incl. zeroing, cap↔message coupling | AVP-197, AVP-170, AVP-171, AVP-174, AVP-201 |
| D10 contract/tripwire split and upgrade gate | AVP-176, AVP-198, AVP-200, and §23.2's classified table |
| D11 no `rescap` migration | AVP-180, AVP-089, AVP-150 |
| D12 two-adapter seam, one production implementation per interface, injectable `SameFile` | AVP-194, AVP-206 |
| D13 native Windows CI; fail-not-skip junctions | AVP-175, AVP-176, AVP-199 |
| D14 no provenance persistence; trigger unfired | AVP-059, AVP-060, AVP-063, AVP-129 |
| D15 exactly-once `Close`, close-failure precedence, zero leaks, no new code | AVP-203, AVP-204, AVP-205 |
| D16 no bounded-runtime guarantee | AVP-207, and the scope halves of AVP-118 and AVP-200 |
| D17 Cobra parse-error ownership; no interception point | AVP-193, AVP-184 |
| D18 exit-3 workspace divergence, disclosed | AVP-183, AVP-184 |

**Status of the artifacts, and of the whole slice:** the status inspection
(`status.json`) is captured under the **same** boundary as the four artifacts —
same root, same component policy with pre/post walks, same `openFlags()`, same
identity and kind rechecks through the same seam, same shared scratch buffer,
same exactly-once `Close` with its outcome classified before any parse, its own
cap — and is subject to the same valid-state gate before any value is echoed.
There is no second, weaker path for metadata.

**No implementation is authorized by this document.** It records decisions for
a planning slice; PRD §17's S1–S5 remain gated on acceptance of both documents,
and `PRD-prepare-intent-bundle.md` remains blocked.

---

## References

- [PRD-artifact-validation-and-provenance](../prds/PRD-artifact-validation-and-provenance.md)
  rev-5 — companion; §7 (path policy and capture), §14 (security), §18 (matrix),
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
  (`errPathEscapes`); `io/fs` (package doc `# Path Names` section, `ValidPath`);
  `io` (`ReadFull`, `LimitReader`, `ReadAll`);
  `internal/godebugs/table.go` (`winsymlink`).
- `internal/rescap/pathgate.go:50-54,68-83,97-120,133-155,181-190`;
  `internal/rescap/content.go:9-11,29-32,50-70`;
  `internal/rescap/pathopen_unix.go:20-28`;
  `internal/rescap/pathopen_windows.go:1-20`.
- `internal/store/store.go:23-40` (`FindProjectRoot`), `:351-361`
  (`LoadFeatureStatus`); `internal/safety/safety.go:11-28`;
  `internal/cli/cobra.go:33-39` (root error printer), `:43-52`
  (`exitCodeFor`), `:56-62` (`SilenceUsage`/`SilenceErrors`), `:3782-3793`
  (`openStoreFromCmd`); `cmd/tpatch/main.go:1-11`;
  `.github/workflows/ci.yml:20-25,37-58`; `go.mod:3`; `SPEC.md:135-141`.

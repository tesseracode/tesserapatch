// CLI surface for typed feature resources
// (PRD-feature-resource-claims-and-capture-adapters §3, ADR-033).
//
//	tpatch feature resource add        <slug> --kind <kind> --selector <sel> [--adapter a --capability c --arg k=v ...] [--trust-current-dolt] [--json]
//	tpatch feature resource list       <slug> [--json]
//	tpatch feature resource remove     <slug> <resource-id-or-prefix> [--json]
//	tpatch feature resource clear      <slug> [--json]
//	tpatch feature resource trust-dolt <slug> <resource-id-or-prefix> --binary-sha256 <64hex> [--json]
//	tpatch feature resource capture    <slug> [--resource <id>] [--dry-run] [--json]
//	tpatch feature resource diff       <slug> [--resource <id>] [--json]
//
// Exit codes are binding: 1 internal/host, 2 validation, 3 state/policy
// refusal. Every refusal is surfaced through ExitCodeError so those
// semantics are real at the process boundary, not just in prose.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tesseracode/tesserapatch/internal/buildinfo"
	"github.com/tesseracode/tesserapatch/internal/redact"
	"github.com/tesseracode/tesserapatch/internal/rescap"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// resourceExit converts a rescap refusal into an ExitCodeError so the
// process exit code matches the design's taxonomy. Any other error
// keeps the legacy exit-1 behaviour.
func resourceExit(err error) error {
	if err == nil {
		return nil
	}
	if r := rescap.AsRefusal(err); r != nil {
		return &ExitCodeError{Code: r.ExitCode(), Message: r.Error()}
	}
	if m, ok := err.(*store.ResourceManifestError); ok {
		return &ExitCodeError{Code: rescap.ExitRefusal, Message: m.Error()}
	}
	if p, ok := err.(*store.PublicationError); ok {
		return &ExitCodeError{Code: rescap.ExitRefusal, Message: p.Error()}
	}
	return err
}

func featureResourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resource",
		Short: "Declare and capture typed non-Git feature resources",
		Long: `Manage the per-feature resource declaration manifest at
.tpatch/features/<slug>/artifacts/resources.json and its tracked capture
store at artifacts/resource-captures/.

Resources are audit sidecars: they record structural facts about state a
feature depends on that is not a Git blob — a deliberately gitignored
config template, a logical Git-metadata view, or a Dolt diff summary.
They are never canonical patch or lifecycle truth, and no tracked
resource artifact ever contains raw bytes or a wall-clock timestamp.`,
	}
	cmd.AddCommand(
		featureResourceAddCmd(),
		featureResourceListCmd(),
		featureResourceRemoveCmd(),
		featureResourceClearCmd(),
		featureResourceTrustDoltCmd(),
		featureResourceCaptureCmd(),
		featureResourceDiffCmd(),
	)
	return cmd
}

// resourceContext bundles the store, slug and loaded manifest every
// verb needs, and enforces the shared preconditions.
type resourceContext struct {
	Store    *store.Store
	Slug     string
	Manifest store.ResourcesManifest
}

func openResourceContext(cmd *cobra.Command, slug string) (*resourceContext, error) {
	s, err := openStoreFromCmd(cmd)
	if err != nil {
		return nil, err
	}
	if !s.FeatureExists(slug) {
		return nil, fmt.Errorf("no such feature: %s", slug)
	}
	manifest, err := store.LoadResources(s, slug)
	if err != nil {
		return nil, err
	}
	return &resourceContext{Store: s, Slug: slug, Manifest: manifest}, nil
}

// withMutatorLock runs fn under the shared per-slug flock, after the
// two-target local ignore/untracked gate. Every mutating verb —
// add/remove/clear/trust-dolt/capture/record --resources — routes
// through here, so the gate and the lock can never be skipped for one
// verb but not another.
func withMutatorLock(ctx *resourceContext, fn func() error) error {
	if err := rescap.EnsureLocalContract(ctx.Store.Root, ctx.Slug); err != nil {
		return err
	}
	lock, err := rescap.AcquireLock(rescap.ScratchRoot(ctx.Store.Root, ctx.Slug), ctx.Store.Root)
	if err != nil {
		return err
	}
	defer func() { _ = lock.Release() }()
	return fn()
}

// ─── add ─────────────────────────────────────────────────────────────────────

func featureResourceAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <slug>",
		Short: "Declare a typed resource for a feature",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return resourceExit(runResourceAdd(cmd, args[0]))
		},
	}
	cmd.Flags().String("kind", "", "Resource kind: ignored-file, git-metadata, or adapter-snapshot")
	cmd.Flags().String("selector", "", "Kind-specific selector")
	cmd.Flags().String("adapter", "", "Adapter name (adapter-snapshot only; v1 accepts only 'dolt')")
	cmd.Flags().String("capability", "", "Adapter capability, or the git-metadata view (ref, index-entry, config)")
	cmd.Flags().StringArray("arg", nil, "Declared argument as key=value (repeatable)")
	cmd.Flags().Bool("trust-current-dolt", false, "Pin the currently-resolved dolt binary's SHA-256; required to declare an adapter=dolt resource")
	cmd.Flags().Bool("json", false, "Emit the result as JSON")
	return cmd
}

func runResourceAdd(cmd *cobra.Command, slug string) error {
	kind, _ := cmd.Flags().GetString("kind")
	selector, _ := cmd.Flags().GetString("selector")
	adapter, _ := cmd.Flags().GetString("adapter")
	capability, _ := cmd.Flags().GetString("capability")
	rawArgs, _ := cmd.Flags().GetStringArray("arg")
	trustCurrent, _ := cmd.Flags().GetBool("trust-current-dolt")

	declared, err := parseDeclaredArgs(rawArgs)
	if err != nil {
		return err
	}
	for _, field := range []string{slug, kind, selector, adapter, capability} {
		if store.HasControlBytes(field) {
			return rescap.Invalid(rescap.ReasonInvalidDeclaration,
				"a declared field contains a NUL or C0 control byte")
		}
	}
	for _, a := range declared {
		if store.HasControlBytes(a.Key) || store.HasControlBytes(a.Value) {
			return rescap.Invalid(rescap.ReasonInvalidDeclaration,
				"--arg %s contains a NUL or C0 control byte", a.Key)
		}
	}

	// PRD §8.3: the selector and every args value are scanned **before
	// they are written anywhere**. This runs before the store is even
	// opened, so a refusal cannot have created resources.json, the
	// per-slug .lock, any ephemeral scratch, current.json or a batch
	// file — there is nothing to roll back because nothing was made.
	// It also runs before the add-time Dolt TOFU bootstrap, so a
	// refused declaration never opens or hashes a binary either.
	if err := scanDeclarationForRedaction(selector, declared); err != nil {
		return err
	}

	ctx, err := openResourceContext(cmd, slug)
	if err != nil {
		return err
	}

	effectiveCapability, err := validateDeclaration(ctx, kind, selector, adapter, capability, declared, trustCurrent)
	if err != nil {
		return err
	}

	var trust *store.ResourceTrust
	if kind == store.ResourceKindAdapterSnapshot && adapter == store.ResourceAdapterDolt {
		digest, err := bootstrapDoltTrust(ctx.Store.Root)
		if err != nil {
			return err
		}
		trust = &store.ResourceTrust{BinarySHA256: digest}
	}

	// Identity is derived from the NORMALIZED capability, never the raw
	// flag, so one semantic declaration has exactly one resource_id.
	candidateID := store.DeriveResourceID(slug, kind, selector, adapter, effectiveCapability, declared)
	candidatePayload := store.ResourceIdentityPayload(slug, kind, selector, adapter, effectiveCapability, declared)

	out := cmd.OutOrStdout()
	asJSON, _ := cmd.Flags().GetBool("json")

	return withMutatorLock(ctx, func() error {
		manifest, err := store.LoadResources(ctx.Store, slug)
		if err != nil {
			return err
		}
		for _, existing := range manifest.Resources {
			if existing.ResourceID != candidateID {
				continue
			}
			existingPayload, _ := existing.Identity(slug)
			if existingPayload == candidatePayload {
				// Strict idempotent no-op: an existing entry's trust
				// pin is left byte-for-byte unchanged even when
				// --trust-current-dolt is re-passed. Only trust-dolt
				// may re-pin after the initial add.
				return reportResource(out, asJSON, "already-declared", existing)
			}
			return rescap.Refuse(rescap.ReasonResourceIDCollision,
				"a different declaration already holds %s", candidateID)
		}
		entry := store.Resource{
			ResourceID:         candidateID,
			Kind:               kind,
			Selector:           selector,
			Adapter:            adapter,
			Capability:         effectiveCapability,
			Args:               declared,
			Trust:              trust,
			AddedByToolVersion: "tpatch/" + buildinfo.String(),
		}
		manifest.Feature = slug
		manifest.Resources = append(manifest.Resources, entry)
		if err := store.SaveResources(ctx.Store, manifest); err != nil {
			return err
		}
		return reportResource(out, asJSON, "added", entry)
	})
}

// scanDeclarationForRedaction applies PRD §8.3's unconditional pre-write
// scan to a declaration's selector and every args value, for **every**
// kind. A match on any of the six closed classes hard-refuses the whole
// invocation (`redaction-refused`, exit 3) — never a partial
// scrub-and-continue, and never a partially-written declaration.
//
// Args *keys* are deliberately not scanned: they are a closed,
// design-owned vocabulary (`contract`/`db_path`/`from`/`table`/`to`)
// validated separately, and the control-byte rules above already bound
// them. Only caller-supplied content is scanned.
func scanDeclarationForRedaction(selector string, args []store.ResourceArg) error {
	if findings := redact.ScanString(selector); len(findings) > 0 {
		return rescap.Refuse(rescap.ReasonRedactionRefused,
			"--selector matched forbidden content classes %s; nothing was written",
			strings.Join(findings, ", "))
	}
	for _, a := range args {
		if findings := redact.ScanString(a.Value); len(findings) > 0 {
			return rescap.Refuse(rescap.ReasonRedactionRefused,
				"--arg %s matched forbidden content classes %s; nothing was written",
				a.Key, strings.Join(findings, ", "))
		}
	}
	return nil
}

// parseDeclaredArgs converts repeated --arg key=value flags into the
// sorted array form, refusing a duplicate key.
func parseDeclaredArgs(raw []string) ([]store.ResourceArg, error) {
	out := make([]store.ResourceArg, 0, len(raw))
	seen := map[string]struct{}{}
	for _, item := range raw {
		key, value, ok := strings.Cut(item, "=")
		if !ok || key == "" {
			return nil, rescap.Invalid(rescap.ReasonInvalidDeclaration,
				"--arg %q must have the shape key=value", item)
		}
		if _, dup := seen[key]; dup {
			return nil, rescap.Invalid(rescap.ReasonInvalidDeclaration, "duplicate --arg key %q", key)
		}
		seen[key] = struct{}{}
		out = append(out, store.ResourceArg{Key: key, Value: value})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// validateDeclaration applies every add-time validation rule for the
// three closed kinds and returns the **normalized effective
// capability** — the single canonical spelling that is hashed into
// resource_id and persisted.
//
// Normalization exists because a capability that is merely *validated*
// but stored raw lets two spellings of one semantic resource produce
// two different resource_ids. Rev-1 closes that: for every kind there
// is exactly one stored capability for a given semantic declaration,
// regardless of whether the caller spelled it out.
func validateDeclaration(ctx *resourceContext, kind, selector, adapter, capability string, args []store.ResourceArg, trustCurrent bool) (string, error) {
	if selector == "" {
		return "", rescap.Invalid(rescap.ReasonInvalidDeclaration, "--selector is required")
	}
	switch kind {
	case store.ResourceKindIgnoredFile:
		if adapter != "" || capability != "" {
			return "", rescap.Invalid(rescap.ReasonInvalidDeclaration,
				"--adapter/--capability do not apply to kind %s", kind)
		}
		if len(args) != 0 {
			return "", rescap.Invalid(rescap.ReasonInvalidDeclaration, "kind %s declares no --arg keys", kind)
		}
		// ignored-file has no capability concept at all.
		return "", validateIgnoredFileSelector(ctx.Store.Root, selector)
	case store.ResourceKindGitMetadata:
		if adapter != "" {
			return "", rescap.Invalid(rescap.ReasonInvalidDeclaration,
				"--adapter does not apply to kind %s", kind)
		}
		if len(args) != 0 {
			return "", rescap.Invalid(rescap.ReasonInvalidDeclaration, "kind %s declares no --arg keys", kind)
		}
		return normalizeGitMetadataCapability(ctx.Store.Root, selector, capability)
	case store.ResourceKindAdapterSnapshot:
		if adapter != store.ResourceAdapterDolt {
			return "", rescap.Invalid(rescap.ReasonInvalidDeclaration,
				"adapter %q is not supported; v1 accepts only %q", adapter, store.ResourceAdapterDolt)
		}
		declaredCapability, table, err := rescap.ParseDoltSelector(selector)
		if err != nil {
			return "", err
		}
		if capability != "" && capability != declaredCapability {
			return "", rescap.Invalid(rescap.ReasonInvalidDeclaration,
				"--capability %q does not match the selector's %q", capability, declaredCapability)
		}
		if err := rescap.ValidateDoltArgs(args, table); err != nil {
			return "", err
		}
		if !trustCurrent {
			return "", rescap.Invalid(rescap.ReasonDoltTrustFlagRequired,
				"declaring an adapter=dolt resource requires --trust-current-dolt; there is no default-trust fallback")
		}
		dbPath, _ := argValue(args, "db_path")
		gated, err := rescap.GatePath(ctx.Store.Root, dbPath)
		if err != nil {
			return "", err
		}
		if closeErr := gated.Close(); closeErr != nil {
			return "", rescap.Internal(rescap.ReasonAdapterCopyFailed,
				"releasing the db_path descriptor: %v", closeErr)
		}
		// The selector is authoritative: `diff-summary` is stored and
		// hashed whether the caller passed --capability or not, so the
		// documented CLI form reaches the same identity as the explicit
		// one (§5.3, §13.3 Vector 2/3).
		return declaredCapability, nil
	default:
		return "", rescap.Invalid(rescap.ReasonInvalidDeclaration,
			"unknown --kind %q; expected one of ignored-file, git-metadata, adapter-snapshot", kind)
	}
}

func argValue(args []store.ResourceArg, key string) (string, bool) {
	for _, a := range args {
		if a.Key == key {
			return a.Value, true
		}
	}
	return "", false
}

// validateIgnoredFileSelector runs §5.1's two gates plus the path gate
// at add time. All three are re-checked at every capture.
func validateIgnoredFileSelector(repoRoot, selector string) error {
	if _, err := rescap.LexicalContainment(repoRoot, selector); err != nil {
		return err
	}
	ignored, err := rescap.IsIgnored(repoRoot, selector)
	if err != nil {
		return err
	}
	if !ignored {
		return rescap.Refuse(rescap.ReasonNotIgnored, "%s is not ignored by git", selector)
	}
	tracked, err := rescap.IsTracked(repoRoot, selector)
	if err != nil {
		return err
	}
	if tracked {
		return rescap.Refuse(rescap.ReasonTrackedAndIgnored,
			"%s reports ignored but is also tracked by git", selector)
	}
	gated, err := rescap.GatePath(repoRoot, selector)
	if err != nil {
		return err
	}
	return gated.Close()
}

// normalizeGitMetadataCapability validates the view against its
// selector and returns the single canonical stored capability.
//
// The `head` view is self-identifying via `--selector head`, and its
// canonical stored capability is the empty string (golden Vector 1,
// §13.3). Passing `--capability head` is the *same* semantic
// declaration, so it normalizes to the identical empty capability
// rather than minting a second identity for one resource. Every other
// view is not inferable from its selector — a repo-relative path or a
// config key says nothing about which view is meant — so those keep
// their view name as the canonical stored capability.
func normalizeGitMetadataCapability(repoRoot, selector, view string) (string, error) {
	if view == "" {
		if selector != store.GitMetadataViewHead {
			return "", rescap.Invalid(rescap.ReasonInvalidDeclaration,
				"git-metadata requires --capability (ref, index-entry, config) unless --selector is exactly %q",
				store.GitMetadataViewHead)
		}
		return "", nil
	}
	switch view {
	case store.GitMetadataViewHead:
		if selector != store.GitMetadataViewHead {
			return "", rescap.Invalid(rescap.ReasonInvalidDeclaration,
				"the head view takes selector %q", store.GitMetadataViewHead)
		}
		// Converges on the omitted spelling's identity.
		return "", nil
	case store.GitMetadataViewRef:
		if _, err := rescap.RunGit(repoRoot, "rev-parse", "--symbolic-full-name", selector); err != nil {
			return "", rescap.Invalid(rescap.ReasonInvalidDeclaration, "ref %q does not resolve", selector)
		}
		return store.GitMetadataViewRef, nil
	case store.GitMetadataViewIndexEntry:
		_, ok, err := rescap.LookupIndexEntry(repoRoot, selector)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", rescap.Invalid(rescap.ReasonInvalidDeclaration,
				"%s has no index entry", selector)
		}
		return store.GitMetadataViewIndexEntry, nil
	case store.GitMetadataViewConfig:
		if !rescap.IsAllowedConfigKey(selector) {
			return "", rescap.Invalid(rescap.ReasonInvalidDeclaration,
				"config key %q is not one of the four allowed keys: %s",
				selector, strings.Join(rescap.AllowedConfigKeys, ", "))
		}
		return store.GitMetadataViewConfig, nil
	default:
		return "", rescap.Invalid(rescap.ReasonInvalidDeclaration,
			"unknown git-metadata view %q", view)
	}
}

// bootstrapDoltTrust runs §6.1's add-time TOFU: resolve, open, hash the
// opened descriptor directly. Zero Dolt processes are started and zero
// scratch directories or files are created.
func bootstrapDoltTrust(repoRoot string) (string, error) {
	resolved, err := rescap.ResolveExternalExecutable(repoRoot, store.ResourceAdapterDolt,
		rescap.Invalid(rescap.ReasonAdapterMissingAtAdd,
			"no dolt executable was found on PATH while computing the bootstrap pin"))
	if err != nil {
		return "", err
	}
	return rescap.HashExecutableDescriptor(resolved)
}

// ─── list ────────────────────────────────────────────────────────────────────

func featureResourceListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <slug>",
		Short: "List a feature's declared resources and their current capture state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return resourceExit(runResourceList(cmd, args[0]))
		},
	}
	cmd.Flags().Bool("json", false, "Emit the full listing as JSON")
	return cmd
}

// resourceListEntry is the JSON shape `list --json` emits. It is a CLI
// projection, not a tracked wire schema.
type resourceListEntry struct {
	ResourceID string               `json:"resource_id"`
	Kind       string               `json:"kind"`
	Selector   string               `json:"selector"`
	Adapter    string               `json:"adapter"`
	Capability string               `json:"capability"`
	Args       []store.ResourceArg  `json:"args"`
	Trust      *store.ResourceTrust `json:"trust"`
	BatchID    string               `json:"batch_id"`
	State      string               `json:"state"`
}

func runResourceList(cmd *cobra.Command, slug string) error {
	// list never acquires the lock: it is a pure read of whatever
	// content is currently visible, which — because every writer uses
	// temp-then-atomic-rename — is always either the fully-prior or the
	// fully-new file, never a torn read.
	ctx, err := openResourceContext(cmd, slug)
	if err != nil {
		return err
	}
	pointer, err := ctx.Store.LoadCurrentPointer(slug)
	if err != nil {
		return err
	}
	entries := make([]resourceListEntry, 0, len(ctx.Manifest.Resources))
	var failures []batchLoadFailure
	for _, r := range ctx.Manifest.Resources {
		entry := resourceListEntry{
			ResourceID: r.ResourceID,
			Kind:       r.Kind,
			Selector:   r.Selector,
			Adapter:    r.Adapter,
			Capability: r.Capability,
			Args:       r.Args,
			Trust:      r.Trust,
			State:      "no-capture-yet",
		}
		if batchID, ok := pointer.BatchFor(r.ResourceID); ok {
			entry.BatchID = batchID
			if _, err := ctx.Store.LoadBatch(slug, batchID); err != nil {
				failure := classifyBatchLoadError(r.ResourceID, err)
				// The per-resource state carries the store's own reason,
				// so `list --json` distinguishes a corrupt batch from an
				// absent one rather than labelling both "missing".
				entry.State = failure.Reason
				failures = append(failures, failure)
			} else {
				entry.State = "captured"
			}
		}
		entries = append(entries, entry)
	}

	out := cmd.OutOrStdout()
	asJSON, _ := cmd.Flags().GetBool("json")
	if asJSON {
		payload := struct {
			Feature   string              `json:"feature"`
			Resources []resourceListEntry `json:"resources"`
		}{Feature: slug, Resources: entries}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(data))
	} else if len(entries) == 0 {
		fmt.Fprintf(out, "Resources for %s: (none)\n", slug)
	} else {
		fmt.Fprintf(out, "Resources for %s:\n", slug)
		for _, e := range entries {
			fmt.Fprintf(out, "  %s  %s  %s  %s\n", e.ResourceID, e.Kind, e.State, e.Selector)
		}
	}
	return aggregateBatchFailures(failures)
}

// batchLoadFailure records one resource's batch-load failure with the
// STORE's own reason and exit code preserved.
//
// Rev-1 collapsed every batch-load error into `tracked-batch-missing`
// (exit 1), which masked a present-but-corrupt or identity-invalid
// batch behind the reason and exit code reserved for an absent file.
// The store already distinguishes them; the CLI's job is to carry that
// distinction to the process boundary, not to flatten it.
type batchLoadFailure struct {
	ResourceID string
	Reason     string
	Code       int
	Detail     string
}

// classifyBatchLoadError maps a store error onto its CLI-facing reason
// and exit code. A *store.PublicationError keeps its own reason;
// `tracked-batch-missing` stays exit 1 (a data-integrity condition
// distinct from "no capture yet"), and every other named batch failure
// — notably `batch-file-corrupt` — is an exit-3 state refusal.
func classifyBatchLoadError(resourceID string, err error) batchLoadFailure {
	if p, ok := err.(*store.PublicationError); ok {
		code := rescap.ExitRefusal
		if p.Reason == store.ReasonTrackedBatchMissing {
			code = rescap.ExitInternal
		}
		return batchLoadFailure{ResourceID: resourceID, Reason: p.Reason, Code: code, Detail: p.Detail}
	}
	return batchLoadFailure{
		ResourceID: resourceID,
		Reason:     store.ReasonTrackedBatchMissing,
		Code:       rescap.ExitInternal,
		Detail:     err.Error(),
	}
}

// aggregateBatchFailures folds per-resource failures into the single
// error the command returns.
//
// Every resource's own state is still reported individually in the
// JSON/text output before this fires, so a healthy resource is never
// hidden by a sick sibling. When resources fail for different reasons,
// the most severe exit code wins (3 outranks 1) and every distinct
// reason is named, so a caller is never told a corrupt batch is merely
// missing.
func aggregateBatchFailures(failures []batchLoadFailure) error {
	if len(failures) == 0 {
		return nil
	}
	byReason := map[string][]string{}
	code := rescap.ExitInternal
	for _, f := range failures {
		byReason[f.Reason] = append(byReason[f.Reason], f.ResourceID)
		if f.Code > code {
			code = f.Code
		}
	}
	reasons := make([]string, 0, len(byReason))
	for reason := range byReason {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	parts := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		ids := byReason[reason]
		sort.Strings(ids)
		parts = append(parts, fmt.Sprintf("%s: %s", reason, strings.Join(ids, ", ")))
	}
	// Name the most severe class first so the reported reason and the
	// exit code agree.
	primary := reasons[0]
	for _, reason := range reasons {
		if reason != store.ReasonTrackedBatchMissing {
			primary = reason
			break
		}
	}
	return &rescap.Refusal{
		Reason: primary,
		Code:   code,
		Detail: strings.Join(parts, "; "),
	}
}

// ─── remove / clear ──────────────────────────────────────────────────────────

func featureResourceRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <slug> <resource-id-or-prefix>",
		Short: "Remove one declared resource",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return resourceExit(runResourceRemove(cmd, args[0], args[1]))
		},
	}
	cmd.Flags().Bool("json", false, "Emit the result as JSON")
	return cmd
}

func runResourceRemove(cmd *cobra.Command, slug, target string) error {
	ctx, err := openResourceContext(cmd, slug)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	asJSON, _ := cmd.Flags().GetBool("json")
	return withMutatorLock(ctx, func() error {
		manifest, err := store.LoadResources(ctx.Store, slug)
		if err != nil {
			return err
		}
		match, ok, err := store.FindResource(&manifest, target)
		if err != nil {
			return rescap.Invalid(rescap.ReasonAmbiguousResourcePrefix, "%v", err)
		}
		if !ok {
			return rescap.Invalid(rescap.ReasonNoSuchResource, "no such resource: %s", target)
		}
		// remove only ever mutates resources.json. A current.json entry
		// from a prior capture simply becomes orphaned — harmless,
		// permanent history, exactly like a batch file that outlives its
		// resource's declaration.
		store.RemoveResource(&manifest, match.ResourceID)
		if err := store.SaveResources(ctx.Store, manifest); err != nil {
			return err
		}
		return reportResource(out, asJSON, "removed", match)
	})
}

func featureResourceClearCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear <slug>",
		Short: "Remove all declared resources (the file is kept with resources: [])",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return resourceExit(runResourceClear(cmd, args[0]))
		},
	}
	cmd.Flags().Bool("json", false, "Emit the result as JSON")
	return cmd
}

func runResourceClear(cmd *cobra.Command, slug string) error {
	ctx, err := openResourceContext(cmd, slug)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	asJSON, _ := cmd.Flags().GetBool("json")
	return withMutatorLock(ctx, func() error {
		manifest := store.ResourcesManifest{
			Version:   store.ResourcesManifestVersion,
			Feature:   slug,
			Resources: []store.Resource{},
		}
		if err := store.SaveResources(ctx.Store, manifest); err != nil {
			return err
		}
		if asJSON {
			return emitJSON(out, map[string]string{"feature": slug, "action": "cleared"})
		}
		fmt.Fprintf(out, "cleared all resources for %s\n", slug)
		return nil
	})
}

// ─── trust-dolt ──────────────────────────────────────────────────────────────

func featureResourceTrustDoltCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trust-dolt <slug> <resource-id-or-prefix>",
		Short: "Re-pin an already-declared Dolt resource's trusted binary digest",
		Long: `Re-pin an adapter-snapshot/dolt resource's trust.binary_sha256.

trust-dolt does not resolve or hash a live dolt binary: it takes the
caller's asserted hash directly, so an operator can pre-approve a hash
before installing the corresponding binary, or pin one obtained from a
separate out-of-band verification step. Only trust.binary_sha256 changes
— resource_id, args, capture history and current.json are untouched.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return resourceExit(runResourceTrustDolt(cmd, args[0], args[1]))
		},
	}
	cmd.Flags().String("binary-sha256", "", "The 64-lowercase-hex digest to pin")
	cmd.Flags().Bool("json", false, "Emit the result as JSON")
	return cmd
}

func runResourceTrustDolt(cmd *cobra.Command, slug, target string) error {
	digest, _ := cmd.Flags().GetString("binary-sha256")
	// Validated before the lock is ever acquired.
	if !rescap.IsValidBinarySHA256(digest) {
		return rescap.Invalid(rescap.ReasonInvalidDeclaration,
			"--binary-sha256 must be exactly 64 lowercase hex characters")
	}
	ctx, err := openResourceContext(cmd, slug)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	asJSON, _ := cmd.Flags().GetBool("json")
	return withMutatorLock(ctx, func() error {
		manifest, err := store.LoadResources(ctx.Store, slug)
		if err != nil {
			return err
		}
		match, ok, err := store.FindResource(&manifest, target)
		if err != nil {
			return rescap.Invalid(rescap.ReasonAmbiguousResourcePrefix, "%v", err)
		}
		if !ok {
			return rescap.Invalid(rescap.ReasonNoSuchResource, "no such resource: %s", target)
		}
		if !match.IsDoltAdapter() {
			return rescap.Invalid(rescap.ReasonResourceNotDoltAdapter,
				"%s is kind %q/adapter %q; trust-dolt applies only to adapter-snapshot/dolt",
				match.ResourceID, match.Kind, match.Adapter)
		}
		store.SetResourceTrust(&manifest, match.ResourceID, digest)
		if err := store.SaveResources(ctx.Store, manifest); err != nil {
			return err
		}
		updated, _, _ := store.FindResource(&manifest, match.ResourceID)
		return reportResource(out, asJSON, "re-pinned", updated)
	})
}

// ─── capture ─────────────────────────────────────────────────────────────────

func featureResourceCaptureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "capture <slug>",
		Short: "Capture declared resources and publish one immutable batch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return resourceExit(runResourceCapture(cmd, args[0]))
		},
	}
	cmd.Flags().String("resource", "", "Capture only this resource id or unambiguous prefix")
	cmd.Flags().Bool("dry-run", false, "Run the entire pipeline for real but write no tracked batch or pointer")
	cmd.Flags().Bool("json", false, "Emit the result as JSON")
	return cmd
}

func runResourceCapture(cmd *cobra.Command, slug string) error {
	resourceArg, _ := cmd.Flags().GetString("resource")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	asJSON, _ := cmd.Flags().GetBool("json")
	out := cmd.OutOrStdout()

	ctx, err := openResourceContext(cmd, slug)
	if err != nil {
		return err
	}
	targets, err := selectResources(&ctx.Manifest, resourceArg)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return rescap.Internal(rescap.ReasonNoResourcesDeclared,
			"%s has no declared resources to capture", slug)
	}

	return withMutatorLock(ctx, func() error {
		engine := rescap.NewEngine(ctx.Store, slug)
		if !dryRun {
			// Both sweeps run only after this invocation has itself
			// acquired the live lock, so neither can race a different,
			// concurrently-running mutator's in-flight scratch content.
			engine.Diagnostics = append(engine.Diagnostics, rescap.SweepLocalOrphans(ctx.Store.Root, slug, "")...)
			engine.Diagnostics = append(engine.Diagnostics, ctx.Store.SweepTrackedTempArtifacts(slug)...)
		}
		scratch, err := rescap.EphemeralScratch(ctx.Store.Root, slug)
		if err != nil {
			return err
		}
		defer engine.RemoveScratch(scratch)

		staged, err := engine.Stage(targets, scratch)
		if err != nil {
			engine.WriteLocalDiagnostics(scratch)
			return err
		}
		if dryRun {
			return reportCapture(out, asJSON, slug, staged.Batch, store.PublishOutcome{BatchID: staged.Batch.BatchID}, true)
		}
		outcome, err := engine.Publish(staged)
		if err != nil {
			engine.WriteLocalDiagnostics(scratch)
			return err
		}
		return reportCapture(out, asJSON, slug, staged.Batch, outcome, false)
	})
}

// selectResources resolves the optional --resource subset.
func selectResources(manifest *store.ResourcesManifest, arg string) ([]store.Resource, error) {
	if arg == "" {
		return manifest.Resources, nil
	}
	match, ok, err := store.FindResource(manifest, arg)
	if err != nil {
		return nil, rescap.Invalid(rescap.ReasonAmbiguousResourcePrefix, "%v", err)
	}
	if !ok {
		return nil, rescap.Invalid(rescap.ReasonNoSuchResource, "no such resource: %s", arg)
	}
	return []store.Resource{match}, nil
}

// ─── diff ────────────────────────────────────────────────────────────────────

func featureResourceDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <slug>",
		Short: "Compare current resource state against the last tracked batch",
		Long: `Recompute each resource's structural result and compare it against the
last tracked batch.

diff never executes the Dolt adapter, never writes tracked state, and
never acquires the per-slug lock. For an ignored-file resource it does
read current file content — through the same bounded in-memory scanner
capture uses, to recompute a real hash — but it never produces a textual
line-level diff of that content.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return resourceExit(runResourceDiff(cmd, args[0]))
		},
	}
	cmd.Flags().String("resource", "", "Diff only this resource id or unambiguous prefix")
	cmd.Flags().Bool("json", false, "Emit the report as JSON")
	return cmd
}

// resourceDiffEntry is one resource's comparison outcome.
type resourceDiffEntry struct {
	ResourceID  string   `json:"resource_id"`
	Kind        string   `json:"kind"`
	Selector    string   `json:"selector"`
	Status      string   `json:"status"`
	Differences []string `json:"differences"`
}

func runResourceDiff(cmd *cobra.Command, slug string) error {
	resourceArg, _ := cmd.Flags().GetString("resource")
	asJSON, _ := cmd.Flags().GetBool("json")
	out := cmd.OutOrStdout()

	ctx, err := openResourceContext(cmd, slug)
	if err != nil {
		return err
	}
	targets, err := selectResources(&ctx.Manifest, resourceArg)
	if err != nil {
		return err
	}
	pointer, err := ctx.Store.LoadCurrentPointer(slug)
	if err != nil {
		return err
	}

	entries := make([]resourceDiffEntry, 0, len(targets))
	var failures []batchLoadFailure
	for _, r := range targets {
		entry := resourceDiffEntry{
			ResourceID:  r.ResourceID,
			Kind:        r.Kind,
			Selector:    r.Selector,
			Differences: []string{},
		}
		batchID, ok := pointer.BatchFor(r.ResourceID)
		if !ok {
			entry.Status = "no-capture-yet"
			entries = append(entries, entry)
			continue
		}
		batch, err := ctx.Store.LoadBatch(slug, batchID)
		if err != nil {
			failure := classifyBatchLoadError(r.ResourceID, err)
			entry.Status = failure.Reason
			failures = append(failures, failure)
			entries = append(entries, entry)
			continue
		}
		var recorded store.CanonNode
		found := false
		for _, res := range batch.Results {
			if res.ResourceID == r.ResourceID {
				recorded = res.Result
				found = true
				break
			}
		}
		if !found {
			// The batch loaded authentically but does not carry an entry
			// for this resource, which is a pointer/batch disagreement
			// rather than a corrupt file.
			failure := batchLoadFailure{
				ResourceID: r.ResourceID,
				Reason:     store.ReasonTrackedBatchMissing,
				Code:       rescap.ExitInternal,
				Detail:     fmt.Sprintf("batch %s carries no result for this resource", batchID),
			}
			entry.Status = failure.Reason
			failures = append(failures, failure)
			entries = append(entries, entry)
			continue
		}
		fresh, err := recomputeForDiff(ctx.Store.Root, r)
		if err != nil {
			return err
		}
		diffs := rescap.CompareResults(recorded, fresh)
		if len(diffs) == 0 {
			entry.Status = "unchanged"
		} else {
			entry.Status = "changed"
			entry.Differences = diffs
		}
		entries = append(entries, entry)
	}

	if asJSON {
		payload := struct {
			Feature   string              `json:"feature"`
			Resources []resourceDiffEntry `json:"resources"`
		}{Feature: slug, Resources: entries}
		if err := emitJSON(out, payload); err != nil {
			return err
		}
	} else {
		for _, e := range entries {
			if len(e.Differences) == 0 {
				fmt.Fprintf(out, "  %s  %s  %s\n", e.ResourceID, e.Status, e.Selector)
				continue
			}
			fmt.Fprintf(out, "  %s  %s  %s  [%s]\n", e.ResourceID, e.Status, e.Selector, strings.Join(e.Differences, "; "))
		}
	}
	return aggregateBatchFailures(failures)
}

// recomputeForDiff recomputes a resource's result without executing any
// adapter. An adapter-snapshot resource reports its recorded result
// unchanged, since diff never runs Dolt.
func recomputeForDiff(repoRoot string, r store.Resource) (store.CanonNode, error) {
	switch r.Kind {
	case store.ResourceKindIgnoredFile:
		result, _, err := rescap.CaptureIgnoredFile(repoRoot, r.Selector)
		return result, err
	case store.ResourceKindGitMetadata:
		view := r.Capability
		if view == "" {
			view = r.Selector
		}
		return rescap.CaptureGitMetadata(repoRoot, view, r.Selector)
	default:
		return store.CanonNull(), nil
	}
}

// ─── shared output helpers ───────────────────────────────────────────────────

func emitJSON(out io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(data))
	return err
}

func reportResource(out io.Writer, asJSON bool, action string, r store.Resource) error {
	if asJSON {
		return emitJSON(out, struct {
			Action   string         `json:"action"`
			Resource store.Resource `json:"resource"`
		}{Action: action, Resource: r})
	}
	fmt.Fprintf(out, "%s %s  %s  %s\n", action, r.ResourceID, r.Kind, r.Selector)
	return nil
}

func reportCapture(out io.Writer, asJSON bool, slug string, batch store.Batch, outcome store.PublishOutcome, dryRun bool) error {
	ids := make([]string, 0, len(batch.Results))
	for _, r := range batch.Results {
		ids = append(ids, r.ResourceID)
	}
	if asJSON {
		return emitJSON(out, struct {
			Feature      string   `json:"feature"`
			BatchID      string   `json:"batch_id"`
			Resources    []string `json:"resources"`
			DryRun       bool     `json:"dry_run"`
			WroteBatch   bool     `json:"wrote_batch"`
			DriftIgnored bool     `json:"drift_ignored"`
		}{
			Feature:      slug,
			BatchID:      batch.BatchID,
			Resources:    ids,
			DryRun:       dryRun,
			WroteBatch:   outcome.WroteBatch,
			DriftIgnored: outcome.DriftIgnored,
		})
	}
	if dryRun {
		fmt.Fprintf(out, "dry-run: %s would publish %s for %d resource(s); nothing was written\n",
			slug, batch.BatchID, len(batch.Results))
		return nil
	}
	verb := "published"
	if !outcome.WroteBatch {
		verb = "re-published (identical content already on disk)"
	}
	fmt.Fprintf(out, "%s %s for %s (%d resource(s))\n", verb, batch.BatchID, slug, len(batch.Results))
	return nil
}

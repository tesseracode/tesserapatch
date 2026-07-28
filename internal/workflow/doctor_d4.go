package workflow

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/safety"
	"github.com/tesseracode/tesserapatch/internal/store"
)

const doctorD4Remediation = "inspect .tpatch/upstream.lock manually; only equivalent format normalization is available via 'tpatch doctor --fix --check D4'"

var doctorLockSHARe = regexp.MustCompile(`^[0-9a-f]{40}$`)

type doctorParsedLock struct {
	lock        store.UpstreamLock
	seen        map[string]bool
	unknown     []string
	malformed   []string
	wrongType   []string
	oldFormat   bool
	canonical   []byte
	canonicalOK bool
}

func runDoctorD4(ctx *doctorContext) {
	path := ctx.store.UpstreamLockPath()
	if err := safety.EnsureSafeRepoPath(ctx.root, path); err != nil {
		ctx.addFinding(DoctorFinding{
			CheckID:  "D4",
			Code:     "lock-unsafe-path",
			Severity: "error",
			Path:     relOrAbs(ctx.root, path),
			Message:  fmt.Sprintf("upstream.lock path is unsafe: %v", err),
			Fixable:  false,
		})
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			ctx.addFinding(DoctorFinding{
				CheckID:     "D4",
				Code:        "lock-missing",
				Severity:    "warning",
				Path:        relOrAbs(ctx.root, path),
				Message:     "upstream.lock is missing; commands that need an upstream baseline may warn or refuse until reconcile repopulates it",
				Fixable:     false,
				Remediation: "run tpatch reconcile <slug> to repopulate upstream.lock when reconciling a feature",
			})
			return
		}
		ctx.addFinding(DoctorFinding{
			CheckID:  "D4",
			Code:     "lock-unreadable",
			Severity: "error",
			Path:     relOrAbs(ctx.root, path),
			Message:  fmt.Sprintf("cannot read upstream.lock: %v", err),
			Fixable:  false,
		})
		return
	}
	loaded, err := store.LoadUpstreamLock(ctx.store)
	if err != nil {
		ctx.addFinding(DoctorFinding{
			CheckID:  "D4",
			Code:     "lock-unreadable",
			Severity: "error",
			Path:     relOrAbs(ctx.root, path),
			Message:  fmt.Sprintf("production upstream.lock loader failed: %v", err),
			Fixable:  false,
		})
		return
	}
	parsed := parseDoctorUpstreamLock(data, loaded)
	if bytes.Equal(bytes.TrimSpace(data), nil) {
		ctx.addFinding(DoctorFinding{
			CheckID:  "D4",
			Code:     "lock-empty",
			Severity: "warning",
			Path:     relOrAbs(ctx.root, path),
			Message:  "upstream.lock is empty; no upstream baseline is pinned yet",
			Fixable:  false,
		})
		return
	}
	if reportDoctorD4SchemaFindings(ctx, path, parsed) {
		return
	}
	if parsed.oldFormat {
		reportDoctorD4OldFormat(ctx, path, data, parsed)
		if !ctx.options.Fix {
			return
		}
	}
	if !parsed.oldFormat && parsed.canonicalOK && !bytes.Equal(data, parsed.canonical) {
		reportDoctorD4CanonicalDrift(ctx, path, data, parsed)
		if ctx.options.Fix {
			return
		}
	}
	reportDoctorD4Reachability(ctx, path, parsed.lock)
}

func parseDoctorUpstreamLock(data []byte, loaded store.UpstreamLock) doctorParsedLock {
	parsed := doctorParsedLock{lock: loaded, seen: map[string]bool{}}
	for i, line := range strings.Split(string(data), "\n") {
		lineNo := i + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		idx := strings.IndexByte(trimmed, ':')
		if idx <= 0 {
			parsed.malformed = append(parsed.malformed, fmt.Sprintf("line %d: expected key: value", lineNo))
			continue
		}
		key := strings.TrimSpace(trimmed[:idx])
		val := strings.TrimSpace(trimmed[idx+1:])
		if i := strings.Index(val, " #"); i >= 0 {
			val = strings.TrimSpace(val[:i])
		}
		switch key {
		case "remote", "branch", "commit", "url":
		default:
			parsed.unknown = append(parsed.unknown, key)
			continue
		}
		parsed.seen[key] = true
		if val == "" {
			continue
		}
		if strings.HasPrefix(val, "[") || strings.HasPrefix(val, "{") {
			parsed.wrongType = append(parsed.wrongType, key)
			continue
		}
		if n := len(val); n >= 1 && (val[0] == '"' || val[0] == '\'') {
			if n < 2 || val[n-1] != val[0] {
				parsed.malformed = append(parsed.malformed, fmt.Sprintf("line %d: field %q has unterminated quoted value", lineNo, key))
				continue
			}
			val = val[1 : n-1]
		}
		switch key {
		case "remote":
			parsed.lock.Remote = val
		case "branch":
			parsed.lock.Branch = val
		case "commit":
			parsed.lock.Commit = val
		case "url":
			parsed.lock.URL = val
		}
	}
	if parsed.lock.Remote != "" && strings.HasPrefix(parsed.lock.Branch, parsed.lock.Remote+"/") {
		parsed.oldFormat = true
		parsed.lock.Branch = strings.TrimPrefix(parsed.lock.Branch, parsed.lock.Remote+"/")
	}
	if parsed.lock.Remote != "" && parsed.lock.Branch != "" && parsed.lock.Commit != "" {
		parsed.canonical = canonicalDoctorUpstreamLock(parsed.lock)
		parsed.canonicalOK = true
	}
	return parsed
}

func reportDoctorD4SchemaFindings(ctx *doctorContext, path string, parsed doctorParsedLock) bool {
	hasError := false
	for _, key := range parsed.unknown {
		hasError = true
		ctx.addFinding(DoctorFinding{CheckID: "D4", Code: "lock-unknown-field", Severity: doctorD4ReadOnlySeverity(ctx), Path: relOrAbs(ctx.root, path), Field: key, Message: fmt.Sprintf("upstream.lock contains unknown field %q; refusing --fix because removing fields could hide operator intent", key), Fixable: false, Remediation: doctorD4Remediation})
	}
	for _, field := range parsed.wrongType {
		hasError = true
		ctx.addFinding(DoctorFinding{CheckID: "D4", Code: "lock-field-wrong-type", Severity: doctorD4ReadOnlySeverity(ctx), Path: relOrAbs(ctx.root, path), Field: field, Message: fmt.Sprintf("upstream.lock field %q must be a scalar string; refusing --fix because type coercion could change lock meaning", field), Fixable: false, Remediation: doctorD4Remediation})
	}
	for _, msg := range parsed.malformed {
		hasError = true
		ctx.addFinding(DoctorFinding{CheckID: "D4", Code: "lock-malformed", Severity: doctorD4ReadOnlySeverity(ctx), Path: relOrAbs(ctx.root, path), Message: msg + "; refusing --fix because malformed lock content is not equivalent format normalization", Fixable: false, Remediation: doctorD4Remediation})
	}
	if parsed.lock.Remote == "" && parsed.lock.Branch == "" && parsed.lock.Commit == "" && len(parsed.unknown) == 0 && len(parsed.wrongType) == 0 && len(parsed.malformed) == 0 {
		ctx.addFinding(DoctorFinding{CheckID: "D4", Code: "lock-empty", Severity: "warning", Path: relOrAbs(ctx.root, path), Message: "upstream.lock has no remote, branch, or commit baseline yet", Fixable: false})
		return true
	}
	for _, req := range []struct {
		field string
		value string
	}{
		{"remote", parsed.lock.Remote},
		{"branch", parsed.lock.Branch},
		{"commit", parsed.lock.Commit},
	} {
		if strings.TrimSpace(req.value) == "" {
			hasError = true
			ctx.addFinding(DoctorFinding{CheckID: "D4", Code: "lock-missing-field", Severity: doctorD4ReadOnlySeverity(ctx), Path: relOrAbs(ctx.root, path), Field: req.field, Message: fmt.Sprintf("upstream.lock missing required field %q; refusing --fix because doctor must not advance commits or guess branches", req.field), Fixable: false, Remediation: doctorD4Remediation})
		}
	}
	if parsed.lock.Commit != "" && !doctorLockSHARe.MatchString(parsed.lock.Commit) {
		hasError = true
		ctx.addFinding(DoctorFinding{CheckID: "D4", Code: "lock-malformed-sha", Severity: doctorD4ReadOnlySeverity(ctx), Path: relOrAbs(ctx.root, path), Field: "commit", Message: "upstream.lock commit must be a 40-character lowercase hexadecimal SHA; refusing --fix because doctor must not choose a replacement commit", Fixable: false, Remediation: doctorD4Remediation})
	}
	return hasError
}

func doctorD4ReadOnlySeverity(ctx *doctorContext) string {
	if ctx.options.Fix {
		return "error"
	}
	return "drift"
}

func reportDoctorD4OldFormat(ctx *doctorContext, path string, data []byte, parsed doctorParsedLock) {
	if ctx.options.Fix {
		fixDoctorD4Canonical(ctx, path, data, parsed, "lock-old-format-fixed", "normalized legacy upstream.lock branch field without changing remote or commit")
		return
	}
	ctx.addFinding(DoctorFinding{
		CheckID:     "D4",
		Code:        "lock-old-format",
		Severity:    "drift",
		Path:        relOrAbs(ctx.root, path),
		Field:       "branch",
		Message:     "upstream.lock uses legacy branch=<remote>/<branch>; current format stores remote and branch separately",
		Fixable:     true,
		Remediation: "run 'tpatch doctor --fix --check D4' to strip the duplicate remote prefix without changing the locked commit",
		BackupPath:  relOrAbs(ctx.root, BackupPathForOverwrite(path)),
	})
}

func reportDoctorD4CanonicalDrift(ctx *doctorContext, path string, data []byte, parsed doctorParsedLock) {
	if ctx.options.Fix {
		fixDoctorD4Canonical(ctx, path, data, parsed, "lock-format-normalized", "normalized upstream.lock key order, quoting, and line endings without changing remote, branch, commit, or url")
		return
	}
	ctx.addFinding(DoctorFinding{
		CheckID:     "D4",
		Code:        "lock-format-noncanonical",
		Severity:    "drift",
		Path:        relOrAbs(ctx.root, path),
		Message:     "upstream.lock is semantically current but not in canonical key order / quoting / line-ending form",
		Fixable:     true,
		Remediation: "run 'tpatch doctor --fix --check D4' to normalize formatting without changing the locked commit or branch",
		BackupPath:  relOrAbs(ctx.root, BackupPathForOverwrite(path)),
	})
}

func fixDoctorD4Canonical(ctx *doctorContext, path string, data []byte, parsed doctorParsedLock, code, message string) {
	if !parsed.canonicalOK {
		ctx.addFinding(DoctorFinding{CheckID: "D4", Code: "lock-normalization-refused", Severity: "error", Path: relOrAbs(ctx.root, path), Message: "refusing D4 --fix because the lock does not contain an unambiguous remote, branch, and commit", Fixable: false, Remediation: doctorD4Remediation})
		return
	}
	if parsed.lock.Commit == "" || !doctorLockSHARe.MatchString(parsed.lock.Commit) {
		ctx.addFinding(DoctorFinding{CheckID: "D4", Code: "lock-normalization-refused", Severity: "error", Path: relOrAbs(ctx.root, path), Field: "commit", Message: "refusing D4 --fix because normalizing would require choosing or changing the locked commit", Fixable: false, Remediation: doctorD4Remediation})
		return
	}
	backup, err := prepareDoctorD4Backup(ctx.root, path, data)
	if err != nil {
		ctx.addFinding(DoctorFinding{CheckID: "D4", Code: "lock-backup-collision", Severity: "error", Path: relOrAbs(ctx.root, path), Message: fmt.Sprintf("refusing to normalize upstream.lock because backup cannot be safely created: %v", err), Fixable: false, Remediation: "move or inspect the existing .orig backup manually before running tpatch doctor --fix --check D4", BackupPath: relOrAbs(ctx.root, BackupPathForOverwrite(path))})
		return
	}
	if backup != "" {
		if err := os.WriteFile(backup, data, 0o644); err != nil {
			ctx.addFinding(DoctorFinding{CheckID: "D4", Code: "lock-backup-failed", Severity: "error", Path: relOrAbs(ctx.root, path), Message: fmt.Sprintf("failed to write upstream.lock backup: %v", err), Fixable: false, Remediation: doctorD4Remediation, BackupPath: relOrAbs(ctx.root, backup)})
			return
		}
	}
	if err := os.WriteFile(path, parsed.canonical, 0o644); err != nil {
		ctx.addFinding(DoctorFinding{CheckID: "D4", Code: "lock-normalization-failed", Severity: "error", Path: relOrAbs(ctx.root, path), Message: fmt.Sprintf("failed to normalize upstream.lock: %v", err), Fixable: true, Remediation: doctorD4Remediation, BackupPath: relOrAbs(ctx.root, BackupPathForOverwrite(path))})
		return
	}
	ctx.addFinding(DoctorFinding{CheckID: "D4", Code: code, Severity: "fixed", Path: relOrAbs(ctx.root, path), Message: message, Fixable: false, BackupPath: relOrAbs(ctx.root, BackupPathForOverwrite(path))})
}

func prepareDoctorD4Backup(root, path string, current []byte) (string, error) {
	backup, err := EnsureDoctorBackup(root, path)
	if err == nil {
		return backup, nil
	}
	backup = BackupPathForOverwrite(path)
	if safeErr := safety.EnsureSafeRepoPath(root, backup); safeErr != nil {
		return "", safeErr
	}
	existing, readErr := os.ReadFile(backup)
	if readErr != nil {
		return "", err
	}
	if bytes.Equal(existing, current) {
		return "", nil
	}
	return "", fmt.Errorf("backup target already exists with different content: %s", relOrAbs(root, backup))
}

func canonicalDoctorUpstreamLock(lock store.UpstreamLock) []byte {
	return []byte(fmt.Sprintf(`# Upstream Lock
# Normalized by tpatch doctor; remote, branch, and commit are preserved.
remote: %q
branch: %q
commit: %q
url: %q
`, lock.Remote, lock.Branch, lock.Commit, lock.URL))
}

func reportDoctorD4Reachability(ctx *doctorContext, path string, lock store.UpstreamLock) {
	ref := lock.Remote + "/" + lock.Branch
	if _, err := runDoctorD4Git(ctx.root, "rev-parse", "--verify", ref+"^{commit}"); err != nil {
		ctx.addFinding(DoctorFinding{CheckID: "D4", Code: "lock-stale-ref", Severity: "drift", Path: relOrAbs(ctx.root, path), Field: "branch", Message: fmt.Sprintf("upstream.lock names local ref %q, but it does not resolve in this repository", ref), Fixable: false, Remediation: "run tpatch reconcile <slug> after restoring or choosing the correct local upstream ref"})
		return
	}
	if _, err := runDoctorD4Git(ctx.root, "cat-file", "-e", lock.Commit+"^{commit}"); err != nil {
		ctx.addFinding(DoctorFinding{CheckID: "D4", Code: "lock-unreachable-commit", Severity: "drift", Path: relOrAbs(ctx.root, path), Field: "commit", Message: "upstream.lock commit object is not present in local git object storage", Fixable: false, Remediation: "restore the missing commit outside doctor, then rerun tpatch doctor"})
		return
	}
	refsOut, err := runDoctorD4Git(ctx.root, "for-each-ref", "--format=%(objectname)")
	if err != nil {
		ctx.addFinding(DoctorFinding{CheckID: "D4", Code: "lock-git-unavailable", Severity: "error", Path: relOrAbs(ctx.root, path), Message: fmt.Sprintf("cannot inspect local git refs for upstream.lock reachability: %v", err), Fixable: false})
		return
	}
	for _, refSHA := range strings.Fields(refsOut) {
		if _, err := runDoctorD4Git(ctx.root, "merge-base", "--is-ancestor", lock.Commit, refSHA); err == nil {
			return
		}
	}
	ctx.addFinding(DoctorFinding{CheckID: "D4", Code: "lock-unreachable-commit", Severity: "drift", Path: relOrAbs(ctx.root, path), Field: "commit", Message: "upstream.lock commit exists locally but is not reachable from any local ref", Fixable: false, Remediation: "restore a local ref containing the locked commit or rerun reconcile outside doctor"})
}

func runDoctorD4Git(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

package workflow

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tesseracode/tesserapatch/assets"
	"github.com/tesseracode/tesserapatch/internal/safety"
)

const doctorD3Remediation = "run 'tpatch doctor --fix --check D3'"

type doctorSkillAsset struct {
	Name string
	Src  string
	Dst  string
}

func doctorSkillAssets(root string) []doctorSkillAsset {
	return []doctorSkillAsset{
		{Name: "Claude skill", Src: "skills/claude/tessera-patch/SKILL.md", Dst: filepath.Join(root, ".claude", "skills", "tessera-patch", "SKILL.md")},
		{Name: "Copilot skill", Src: "skills/copilot/tessera-patch/SKILL.md", Dst: filepath.Join(root, ".github", "skills", "tessera-patch", "SKILL.md")},
		{Name: "Copilot prompt", Src: "prompts/copilot/tessera-patch-apply.prompt.md", Dst: filepath.Join(root, ".github", "prompts", "tessera-patch-apply.prompt.md")},
		{Name: "Cursor rules", Src: "skills/cursor/tessera-patch.mdc", Dst: filepath.Join(root, ".cursor", "rules", "tessera-patch.mdc")},
		{Name: "Windsurf rules", Src: "skills/windsurf/windsurfrules", Dst: filepath.Join(root, ".windsurfrules")},
		{Name: "Generic workflow", Src: "workflows/tessera-patch-generic.md", Dst: filepath.Join(root, ".tpatch", "workflows", "tessera-patch-generic.md")},
	}
}

func runDoctorD3(ctx *doctorContext) {
	for _, asset := range doctorSkillAssets(ctx.root) {
		if err := safety.EnsureSafeRepoPath(ctx.root, asset.Dst); err != nil {
			ctx.addFinding(DoctorFinding{
				CheckID:  "D3",
				Code:     "skill-asset-unsafe-path",
				Severity: "error",
				Path:     relOrAbs(ctx.root, asset.Dst),
				Message:  fmt.Sprintf("managed skill asset path is unsafe: %v", err),
				Fixable:  false,
			})
			continue
		}
		bundled, err := assets.Skills.ReadFile(asset.Src)
		if err != nil {
			ctx.addFinding(DoctorFinding{
				CheckID:  "D3",
				Code:     "bundled-skill-asset-unreadable",
				Severity: "error",
				Path:     asset.Src,
				Message:  fmt.Sprintf("cannot read bundled %s: %v", asset.Name, err),
				Fixable:  false,
			})
			continue
		}
		installed, err := os.ReadFile(asset.Dst)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			ctx.addFinding(DoctorFinding{
				CheckID:  "D3",
				Code:     "skill-asset-unreadable",
				Severity: "error",
				Path:     relOrAbs(ctx.root, asset.Dst),
				Message:  fmt.Sprintf("cannot read installed %s: %v", asset.Name, err),
				Fixable:  false,
			})
			continue
		}
		if bytes.Equal(installed, bundled) {
			continue
		}
		expectedSHA := shortSHA256(bundled)
		actualSHA := shortSHA256(installed)
		backupPath := BackupPathForOverwrite(asset.Dst)
		if !looksLikeTpatchSkillAsset(installed, bundled) {
			severity := "drift"
			if ctx.options.Fix {
				severity = "error"
			}
			ctx.addFinding(DoctorFinding{
				CheckID:     "D3",
				Code:        "skill-asset-unrecognized",
				Severity:    severity,
				Path:        relOrAbs(ctx.root, asset.Dst),
				Message:     fmt.Sprintf("%s differs from bundled asset but lacks a tpatch marker; installed sha256=%s, bundled sha256=%s", asset.Name, actualSHA, expectedSHA),
				Fixable:     false,
				Remediation: "move or delete this file manually before running tpatch doctor --fix --check D3",
			})
			continue
		}
		if !ctx.options.Fix {
			ctx.addFinding(DoctorFinding{
				CheckID:     "D3",
				Code:        "stale-skill-asset",
				Severity:    "drift",
				Path:        relOrAbs(ctx.root, asset.Dst),
				Message:     fmt.Sprintf("%s differs from bundled asset; installed sha256=%s, bundled sha256=%s", asset.Name, actualSHA, expectedSHA),
				Fixable:     true,
				Remediation: doctorD3Remediation,
				BackupPath:  relOrAbs(ctx.root, backupPath),
			})
			continue
		}
		backup, err := prepareDoctorD3Backup(ctx.root, asset.Dst, installed)
		if err != nil {
			ctx.addFinding(DoctorFinding{
				CheckID:     "D3",
				Code:        "skill-asset-backup-collision",
				Severity:    "error",
				Path:        relOrAbs(ctx.root, asset.Dst),
				Message:     fmt.Sprintf("refusing to overwrite %s because backup cannot be safely created: %v", asset.Name, err),
				Fixable:     false,
				Remediation: "move or inspect the existing .orig backup manually before running tpatch doctor --fix --check D3",
				BackupPath:  relOrAbs(ctx.root, backupPath),
			})
			continue
		}
		if backup != "" {
			if err := os.WriteFile(backup, installed, 0o644); err != nil {
				ctx.addFinding(DoctorFinding{
					CheckID:     "D3",
					Code:        "skill-asset-backup-failed",
					Severity:    "error",
					Path:        relOrAbs(ctx.root, asset.Dst),
					Message:     fmt.Sprintf("failed to write backup before replacing %s: %v", asset.Name, err),
					Fixable:     false,
					Remediation: doctorD3Remediation,
					BackupPath:  relOrAbs(ctx.root, backup),
				})
				continue
			}
		}
		if err := os.WriteFile(asset.Dst, bundled, 0o644); err != nil {
			ctx.addFinding(DoctorFinding{
				CheckID:     "D3",
				Code:        "skill-asset-fix-failed",
				Severity:    "error",
				Path:        relOrAbs(ctx.root, asset.Dst),
				Message:     fmt.Sprintf("failed to replace %s with bundled asset: %v", asset.Name, err),
				Fixable:     true,
				Remediation: doctorD3Remediation,
				BackupPath:  relOrAbs(ctx.root, BackupPathForOverwrite(asset.Dst)),
			})
			continue
		}
		ctx.addFinding(DoctorFinding{
			CheckID:    "D3",
			Code:       "stale-skill-asset-fixed",
			Severity:   "fixed",
			Path:       relOrAbs(ctx.root, asset.Dst),
			Message:    fmt.Sprintf("replaced stale %s with bundled asset; previous sha256=%s, bundled sha256=%s", asset.Name, actualSHA, expectedSHA),
			Fixable:    false,
			BackupPath: relOrAbs(ctx.root, BackupPathForOverwrite(asset.Dst)),
		})
	}
}

func prepareDoctorD3Backup(root, path string, installed []byte) (string, error) {
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
	if bytes.Equal(existing, installed) {
		return "", nil
	}
	return "", fmt.Errorf("backup target already exists with different content: %s", relOrAbs(root, backup))
}

func looksLikeTpatchSkillAsset(installed, bundled []byte) bool {
	prefix := string(installed[:minInt(len(installed), 256)])
	firstLine := strings.ToLower(strings.TrimSpace(strings.SplitN(prefix, "\n", 2)[0]))
	if strings.Contains(firstLine, "tessera-patch") || strings.Contains(firstLine, "tpatch") {
		return true
	}
	bundledFirst := strings.ToLower(strings.TrimSpace(strings.SplitN(string(bundled[:minInt(len(bundled), 256)]), "\n", 2)[0]))
	if bundledFirst != "" && firstLine == bundledFirst {
		return true
	}
	lowerPrefix := strings.ToLower(prefix)
	return strings.Contains(lowerPrefix, "tessera-patch") || strings.Contains(lowerPrefix, "tpatch")
}

func shortSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum)[:12]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

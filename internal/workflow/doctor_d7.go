package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tesseracode/tesserapatch/assets"
)

const doctorD7Remediation = "hand-fix apply-recipe.json to the current workflow.ApplyRecipe schema, verify with 'tpatch verify <slug>', or regenerate with 'tpatch implement <slug>'"

var doctorJSONCodeBlockRe = regexp.MustCompile("(?s)```json\\s*\\n(.*?)\\n```")

func runDoctorD7(ctx *doctorContext) {
	for _, feature := range ctx.features {
		path := filepath.Join(feature.Dir, "artifacts", "apply-recipe.json")
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			ctx.addFinding(DoctorFinding{
				CheckID:     "D7",
				Code:        "recipe-unreadable",
				Severity:    "error",
				Feature:     feature.Slug,
				Path:        relOrAbs(ctx.root, path),
				Message:     fmt.Sprintf("cannot read apply-recipe.json: %v", err),
				Fixable:     false,
				Remediation: doctorD7RemediationForFeature(feature.Slug),
			})
			continue
		}
		if _, err := DecodeApplyRecipeStrict(data); err != nil {
			ctx.addFinding(DoctorFinding{
				CheckID:     "D7",
				Code:        doctorD7CodeForError(err),
				Severity:    "drift",
				Feature:     feature.Slug,
				Path:        relOrAbs(ctx.root, path),
				Line:        doctorD7LineForError(data, err),
				Message:     fmt.Sprintf("apply-recipe.json does not match the current workflow.ApplyRecipe schema: %v", err),
				Fixable:     false,
				Remediation: doctorD7RemediationForFeature(feature.Slug),
			})
		}
	}
	for _, asset := range doctorSkillAssets(ctx.root) {
		data, err := os.ReadFile(asset.Dst)
		if err != nil {
			continue
		}
		bundled, _ := assets.Skills.ReadFile(asset.Src)
		if !looksLikeTpatchSkillAsset(data, bundled) {
			continue
		}
		runDoctorD7SkillExamples(ctx, asset, data)
	}
}

func runDoctorD7SkillExamples(ctx *doctorContext, asset doctorSkillAsset, data []byte) {
	matches := doctorJSONCodeBlockRe.FindAllSubmatchIndex(data, -1)
	for _, m := range matches {
		if len(m) < 4 || m[2] < 0 || m[3] < 0 {
			continue
		}
		block := data[m[2]:m[3]]
		if !strings.Contains(string(block), "\"operations\"") {
			continue
		}
		if _, err := DecodeApplyRecipeStrict(block); err != nil {
			line := bytesLineNumber(data, m[2]) + doctorD7LineForError(block, err) - 1
			ctx.addFinding(DoctorFinding{
				CheckID:     "D7",
				Code:        "skill-recipe-schema-drift",
				Severity:    "drift",
				Path:        relOrAbs(ctx.root, asset.Dst),
				Line:        line,
				Message:     fmt.Sprintf("%s recipe example does not match the current workflow.ApplyRecipe schema: %v", asset.Name, err),
				Fixable:     false,
				Remediation: "refresh the installed skill asset with 'tpatch doctor --fix --check D3' or hand-edit the recipe example to match workflow.ApplyRecipe",
			})
		}
	}
}

func doctorD7RemediationForFeature(slug string) string {
	return strings.ReplaceAll(doctorD7Remediation, "<slug>", slug)
}

func doctorD7CodeForError(err error) string {
	if err == nil {
		return "recipe-schema-drift"
	}
	var syntax *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntax):
		return "recipe-parse-error"
	case errors.Is(err, io.ErrUnexpectedEOF):
		return "recipe-parse-error"
	case errors.As(err, &typeErr):
		return "recipe-field-type-invalid"
	case strings.Contains(err.Error(), "unknown field"):
		return "recipe-unknown-field"
	case strings.Contains(err.Error(), "missing top-level feature"):
		return "recipe-missing-feature"
	case strings.Contains(err.Error(), "missing type"):
		return "recipe-operation-missing-type"
	case strings.Contains(err.Error(), "unknown type"):
		return "recipe-operation-unknown-type"
	default:
		return "recipe-schema-drift"
	}
}

func doctorD7LineForError(data []byte, err error) int {
	if line := lineForJSONErrorBytes(data, err); line > 1 {
		return line
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) && typeErr.Offset > 0 {
		return bytesLineNumber(data, int(typeErr.Offset))
	}
	if field := unknownJSONFieldName(err); field != "" {
		if idx := strings.Index(string(data), `"`+field+`"`); idx >= 0 {
			return bytesLineNumber(data, idx)
		}
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return strings.Count(string(data), "\n")
	}
	return 1
}

func unknownJSONFieldName(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	const prefix = "json: unknown field "
	idx := strings.Index(msg, prefix)
	if idx < 0 {
		return ""
	}
	field := strings.TrimPrefix(msg[idx:], prefix)
	return strings.Trim(field, `"`)
}

func bytesLineNumber(data []byte, offset int) int {
	if offset < 0 {
		return 1
	}
	if offset > len(data) {
		offset = len(data)
	}
	return strings.Count(string(data[:offset]), "\n") + 1
}

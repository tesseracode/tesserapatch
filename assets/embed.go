// Package assets embeds skill, prompt, workflow, and template files into the binary.
package assets

import "embed"

// Skills contains the embedded skill/workflow/template files.
//
//go:embed prompts skills templates workflows
var Skills embed.FS

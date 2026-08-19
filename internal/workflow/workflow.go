// Package workflow orchestrates the 7-phase lifecycle:
// analyse → define → explore → implement → test → record → reconcile.
package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

type legacyProviderAttempt struct {
	response string
	err      error
}

// legacyPhaseRecordingProvider is used only by Run* compatibility wrappers.
// Pure generators receive it as an ordinary provider and remain sink-free.
type legacyPhaseRecordingProvider struct {
	provider.Provider
	logPrefix string
	attempts  []legacyProviderAttempt
}

func (p *legacyPhaseRecordingProvider) Generate(ctx context.Context, cfg provider.Config, req provider.GenerateRequest) (string, error) {
	spinner := NewSpinnerIfTTY(os.Stderr, spinnerMessage(p.logPrefix))
	response, err := p.Provider.Generate(ctx, cfg, req)
	spinner.Stop()
	p.attempts = append(p.attempts, legacyProviderAttempt{response: response, err: err})
	return response, err
}

// RunAnalysis executes the analysis phase for a feature.
func RunAnalysis(ctx context.Context, s *store.Store, slug string, prov provider.Provider, cfg provider.Config) (*AnalysisResult, error) {
	request, err := s.ReadFeatureFile(slug, "request.md")
	if err != nil {
		return nil, fmt.Errorf("cannot read feature request: %w", err)
	}

	fileTree := captureFileTree(s.Root, 3)
	guidance := readGuidanceFiles(s.Root)

	maxRetries := 0
	if prov != nil && cfg.Configured() {
		storeCfg, _ := s.LoadConfig()
		maxRetries = storeCfg.MaxRetries
	}
	generationProvider, recorder := legacyRecordingProvider(prov, cfg, "analyze")
	result, note, err := GenerateAnalysis(ctx, AnalysisInput{
		Slug:       slug,
		Request:    request,
		FileTree:   fileTree,
		Guidance:   guidance,
		Provider:   generationProvider,
		Config:     cfg,
		MaxRetries: maxRetries,
		Authority: GeneratorAuthorityPolicy{
			LegacyContextSemantics: true,
		},
	})
	replayLegacyPhaseResponses(s, slug, "analyze", recorder)
	if err != nil {
		return nil, err
	}
	if recorder != nil && result.HeuristicMode && note.ErrorClass != GenErrorNone {
		if legacyErr := legacyAnalysisGenerationError(ctx, recorder); legacyErr != nil {
			result.UnresolvedQuestions = append(result.UnresolvedQuestions, fmt.Sprintf("Provider error: %v", legacyErr))
		} else {
			result.ImplementationNotes = append(result.ImplementationNotes, "Raw LLM response available in artifacts")
		}
		if response := recorder.lastResponse(); response != "" {
			_ = s.WriteArtifact(slug, "raw-analysis-response.txt", response)
		}
	}

	analysisJSON, _ := json.MarshalIndent(result, "", "  ")
	if err := s.WriteArtifact(slug, "analysis.json", string(analysisJSON)+"\n"); err != nil {
		return nil, err
	}

	analysisMD := RenderAnalysisMD(result, slug)
	if err := s.WriteFeatureFile(slug, "analysis.md", analysisMD); err != nil {
		return nil, err
	}

	state := store.StateAnalyzed
	notes := result.Summary
	if err := s.MarkFeatureState(slug, state, "analyze", notes); err != nil {
		return nil, err
	}

	return &result, nil
}

// RunDefine generates acceptance criteria and implementation plan.
func RunDefine(ctx context.Context, s *store.Store, slug string, prov provider.Provider, cfg provider.Config) error {
	request, err := s.ReadFeatureFile(slug, "request.md")
	if err != nil {
		return fmt.Errorf("cannot read feature request: %w", err)
	}

	analysisJSON, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "analysis.json"))
	analysisTxt := ""
	if err == nil {
		analysisTxt = analysisJSON
	}

	analysisMD, _ := s.ReadFeatureFile(slug, "analysis.md")

	maxRetries := 0
	if prov != nil && cfg.Configured() {
		storeCfg, _ := s.LoadConfig()
		maxRetries = storeCfg.MaxRetries
	}
	generationProvider, recorder := legacyRecordingProvider(prov, cfg, "define")
	specContent, _, err := GenerateSpec(ctx, DefineInput{
		Slug:         slug,
		Request:      request,
		AnalysisMD:   analysisMD,
		AnalysisJSON: analysisTxt,
		Provider:     generationProvider,
		Config:       cfg,
		MaxRetries:   maxRetries,
		Authority: GeneratorAuthorityPolicy{
			LegacyContextSemantics: true,
		},
	})
	replayLegacyPhaseResponses(s, slug, "define", recorder)
	if err != nil {
		return err
	}

	if err := s.WriteFeatureFile(slug, "spec.md", specContent); err != nil {
		return err
	}

	return s.MarkFeatureState(slug, store.StateDefined, "define", "Acceptance criteria and plan generated")
}

// RunExplore reads relevant codebase files and produces an exploration log.
func RunExplore(ctx context.Context, s *store.Store, slug string, prov provider.Provider, cfg provider.Config) error {
	request, err := s.ReadFeatureFile(slug, "request.md")
	if err != nil {
		return fmt.Errorf("cannot read feature request: %w", err)
	}

	analysisMD, _ := s.ReadFeatureFile(slug, "analysis.md")
	specMD, _ := s.ReadFeatureFile(slug, "spec.md")

	fileTree := captureFileTree(s.Root, 4)

	maxRetries := 0
	if prov != nil && cfg.Configured() {
		storeCfg, _ := s.LoadConfig()
		maxRetries = storeCfg.MaxRetries
	}
	generationProvider, recorder := legacyRecordingProvider(prov, cfg, "explore")
	explorationContent, _, err := GenerateExploration(ctx, ExploreInput{
		Slug:       slug,
		Request:    request,
		AnalysisMD: analysisMD,
		SpecMD:     specMD,
		FileTree:   fileTree,
		Provider:   generationProvider,
		Config:     cfg,
		MaxRetries: maxRetries,
		Authority: GeneratorAuthorityPolicy{
			LegacyContextSemantics: true,
		},
	})
	replayLegacyPhaseResponses(s, slug, "explore", recorder)
	if err != nil {
		return err
	}

	if err := s.WriteFeatureFile(slug, "exploration.md", explorationContent); err != nil {
		return err
	}

	return s.MarkFeatureState(slug, store.StateDefined, "explore", "Exploration complete")
}

func legacyRecordingProvider(prov provider.Provider, cfg provider.Config, logPrefix string) (provider.Provider, *legacyPhaseRecordingProvider) {
	if prov == nil || !cfg.Configured() {
		return prov, nil
	}
	recorder := &legacyPhaseRecordingProvider{Provider: prov, logPrefix: logPrefix}
	return recorder, recorder
}

func replayLegacyPhaseResponses(s *store.Store, slug, prefix string, recorder *legacyPhaseRecordingProvider) {
	if recorder == nil {
		return
	}
	success := 0
	for _, attempt := range recorder.attempts {
		if attempt.err != nil {
			continue
		}
		success++
		name := fmt.Sprintf("raw-%s-response-%d.txt", prefix, success)
		_ = s.WriteArtifact(slug, name, attempt.response)
	}
}

func legacyAnalysisGenerationError(ctx context.Context, recorder *legacyPhaseRecordingProvider) error {
	if len(recorder.attempts) == 0 {
		return retryContextError(ctx)
	}
	last := recorder.attempts[len(recorder.attempts)-1]
	if last.err != nil {
		return last.err
	}

	var target AnalysisResult
	if validationErr := JSONObjectValidator(&target)(last.response); validationErr != nil {
		return fmt.Errorf("validation failed after %d attempt(s): %w", len(recorder.attempts), validationErr)
	}
	return nil
}

func (p *legacyPhaseRecordingProvider) lastResponse() string {
	if p == nil || len(p.attempts) == 0 {
		return ""
	}
	return p.attempts[len(p.attempts)-1].response
}

// Capture file tree up to maxDepth levels deep.
func captureFileTree(root string, maxDepth int) string {
	var b strings.Builder
	walkTree(&b, root, "", 0, maxDepth)
	return b.String()
}

func walkTree(b *strings.Builder, path, prefix string, depth, maxDepth int) {
	if depth >= maxDepth {
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		// Skip common non-essential directories
		if entry.IsDir() && (name == "node_modules" || name == ".git" || name == ".tpatch" || name == "dist" || name == "build" || name == "__pycache__" || name == ".next") {
			continue
		}
		b.WriteString(prefix + name)
		if entry.IsDir() {
			b.WriteString("/\n")
			walkTree(b, filepath.Join(path, name), prefix+"  ", depth+1, maxDepth)
		} else {
			b.WriteString("\n")
		}
	}
}

// Read guidance files (PATCHING.md, CONTRIBUTING.md, etc.)
func readGuidanceFiles(root string) string {
	candidates := []string{"PATCHING.md", "CONTRIBUTING.md", "AGENTS.md", "CLAUDE.md"}
	var parts []string
	for _, name := range candidates {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err == nil && len(data) > 0 {
			parts = append(parts, fmt.Sprintf("### %s\n\n%s", name, string(data)))
		}
	}
	return strings.Join(parts, "\n\n---\n\n")
}

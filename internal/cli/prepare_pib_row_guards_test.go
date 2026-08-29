//go:build (linux && !android) || (darwin && !ios)

package cli

// Per-row S6 guard tests — GH #23 aggregate acceptance closure.
//
// `TestS6PrepareContractRows` registers each of these rows as
// `t.Run("PIB-NNN-…", runS6Guard("PIB-NNN"))`. That callback is a *call
// expression*, so the accepted AST resolver cannot select a body for it: the row
// had no independently addressable, body-sensitive target. Rather than point the
// aggregate ledger at the parent wrapper — the exact false positive the ledger
// exists to reject — each row gets its own narrow test here.
//
// Every test below is simultaneously the row's acceptance target and its §18.53
// sensitivity fixture: it runs the row's own validator over the shipped input
// (which must pass) and then over the row's registered wrong-input fixture
// (which the *same* validator must reject). Nothing is synthetic — the fixture is
// `s6GuardSpecs[id].sensitivity`, the mutation shipped with S6.

// Each row asserts in its own body — no shared runner — so the aggregate ledger
// resolves a distinct, row-specific identity for every one of them.

import "testing"

// TestS6RowPIB155GuardAndSensitivity is the acceptance target and the
// sensitivity fixture for PIB-155 — the multi-file atomic over-claim guard.
func TestS6RowPIB155GuardAndSensitivity(t *testing.T) {
	spec, registered := s6GuardSpecs["PIB-155"]
	if !registered || spec.run == nil || spec.sensitivity == nil || spec.fixture == "" {
		t.Fatal("PIB-155 has no registered S6 guard spec")
	}
	if err := spec.run(t); err != nil {
		t.Fatalf("PIB-155 guard rejected the shipped input: %v", err)
	}
	if err := spec.sensitivity(t); err == nil {
		t.Fatalf("PIB-155 sensitivity fixture %q was accepted by its own validator", spec.fixture)
	}
}

// TestS6RowPIB158GuardAndSensitivity is the acceptance target and the
// sensitivity fixture for PIB-158 — the recursive forbidden-JSON-key walk.
func TestS6RowPIB158GuardAndSensitivity(t *testing.T) {
	spec, registered := s6GuardSpecs["PIB-158"]
	if !registered || spec.run == nil || spec.sensitivity == nil || spec.fixture == "" {
		t.Fatal("PIB-158 has no registered S6 guard spec")
	}
	if err := spec.run(t); err != nil {
		t.Fatalf("PIB-158 guard rejected the shipped input: %v", err)
	}
	if err := spec.sensitivity(t); err == nil {
		t.Fatalf("PIB-158 sensitivity fixture %q was accepted by its own validator", spec.fixture)
	}
}

// TestS6RowPIB159GuardAndSensitivity is the acceptance target and the
// sensitivity fixture for PIB-159 — the created_at wire-key sensitivity.
func TestS6RowPIB159GuardAndSensitivity(t *testing.T) {
	spec, registered := s6GuardSpecs["PIB-159"]
	if !registered || spec.run == nil || spec.sensitivity == nil || spec.fixture == "" {
		t.Fatal("PIB-159 has no registered S6 guard spec")
	}
	if err := spec.run(t); err != nil {
		t.Fatalf("PIB-159 guard rejected the shipped input: %v", err)
	}
	if err := spec.sensitivity(t); err == nil {
		t.Fatalf("PIB-159 sensitivity fixture %q was accepted by its own validator", spec.fixture)
	}
}

// TestS6RowPIB215GuardAndSensitivity is the acceptance target and the
// sensitivity fixture for PIB-215 — the SPEC.md exit-code table guard.
func TestS6RowPIB215GuardAndSensitivity(t *testing.T) {
	spec, registered := s6GuardSpecs["PIB-215"]
	if !registered || spec.run == nil || spec.sensitivity == nil || spec.fixture == "" {
		t.Fatal("PIB-215 has no registered S6 guard spec")
	}
	if err := spec.run(t); err != nil {
		t.Fatalf("PIB-215 guard rejected the shipped input: %v", err)
	}
	if err := spec.sensitivity(t); err == nil {
		t.Fatalf("PIB-215 sensitivity fixture %q was accepted by its own validator", spec.fixture)
	}
}

// TestS6RowPIB216GuardAndSensitivity is the acceptance target and the
// sensitivity fixture for PIB-216 — the six-skill command parity guard.
func TestS6RowPIB216GuardAndSensitivity(t *testing.T) {
	spec, registered := s6GuardSpecs["PIB-216"]
	if !registered || spec.run == nil || spec.sensitivity == nil || spec.fixture == "" {
		t.Fatal("PIB-216 has no registered S6 guard spec")
	}
	if err := spec.run(t); err != nil {
		t.Fatalf("PIB-216 guard rejected the shipped input: %v", err)
	}
	if err := spec.sensitivity(t); err == nil {
		t.Fatalf("PIB-216 sensitivity fixture %q was accepted by its own validator", spec.fixture)
	}
}

// TestS6RowPIB217GuardAndSensitivity is the acceptance target and the
// sensitivity fixture for PIB-217 — the skill phase-ordering placement guard.
func TestS6RowPIB217GuardAndSensitivity(t *testing.T) {
	spec, registered := s6GuardSpecs["PIB-217"]
	if !registered || spec.run == nil || spec.sensitivity == nil || spec.fixture == "" {
		t.Fatal("PIB-217 has no registered S6 guard spec")
	}
	if err := spec.run(t); err != nil {
		t.Fatalf("PIB-217 guard rejected the shipped input: %v", err)
	}
	if err := spec.sensitivity(t); err == nil {
		t.Fatalf("PIB-217 sensitivity fixture %q was accepted by its own validator", spec.fixture)
	}
}

// TestS6RowPIB218GuardAndSensitivity is the acceptance target and the
// sensitivity fixture for PIB-218 — the skill provenance/certification claim guard.
func TestS6RowPIB218GuardAndSensitivity(t *testing.T) {
	spec, registered := s6GuardSpecs["PIB-218"]
	if !registered || spec.run == nil || spec.sensitivity == nil || spec.fixture == "" {
		t.Fatal("PIB-218 has no registered S6 guard spec")
	}
	if err := spec.run(t); err != nil {
		t.Fatalf("PIB-218 guard rejected the shipped input: %v", err)
	}
	if err := spec.sensitivity(t); err == nil {
		t.Fatalf("PIB-218 sensitivity fixture %q was accepted by its own validator", spec.fixture)
	}
}

// TestS6RowPIB219GuardAndSensitivity is the acceptance target and the
// sensitivity fixture for PIB-219 — the docs/feature-layout.md contract guard.
func TestS6RowPIB219GuardAndSensitivity(t *testing.T) {
	spec, registered := s6GuardSpecs["PIB-219"]
	if !registered || spec.run == nil || spec.sensitivity == nil || spec.fixture == "" {
		t.Fatal("PIB-219 has no registered S6 guard spec")
	}
	if err := spec.run(t); err != nil {
		t.Fatalf("PIB-219 guard rejected the shipped input: %v", err)
	}
	if err := spec.sensitivity(t); err == nil {
		t.Fatalf("PIB-219 sensitivity fixture %q was accepted by its own validator", spec.fixture)
	}
}

// TestS6RowPIB220GuardAndSensitivity is the acceptance target and the
// sensitivity fixture for PIB-220 — the docs/agent-as-provider.md manual-note guard.
func TestS6RowPIB220GuardAndSensitivity(t *testing.T) {
	spec, registered := s6GuardSpecs["PIB-220"]
	if !registered || spec.run == nil || spec.sensitivity == nil || spec.fixture == "" {
		t.Fatal("PIB-220 has no registered S6 guard spec")
	}
	if err := spec.run(t); err != nil {
		t.Fatalf("PIB-220 guard rejected the shipped input: %v", err)
	}
	if err := spec.sensitivity(t); err == nil {
		t.Fatalf("PIB-220 sensitivity fixture %q was accepted by its own validator", spec.fixture)
	}
}

// TestS6RowPIB224GuardAndSensitivity is the acceptance target and the
// sensitivity fixture for PIB-224 — the artifact-disposition totality guard.
func TestS6RowPIB224GuardAndSensitivity(t *testing.T) {
	spec, registered := s6GuardSpecs["PIB-224"]
	if !registered || spec.run == nil || spec.sensitivity == nil || spec.fixture == "" {
		t.Fatal("PIB-224 has no registered S6 guard spec")
	}
	if err := spec.run(t); err != nil {
		t.Fatalf("PIB-224 guard rejected the shipped input: %v", err)
	}
	if err := spec.sensitivity(t); err == nil {
		t.Fatalf("PIB-224 sensitivity fixture %q was accepted by its own validator", spec.fixture)
	}
}

// TestS6RowPIB225GuardAndSensitivity is the acceptance target and the
// sensitivity fixture for PIB-225 — the feature-state totality guard.
func TestS6RowPIB225GuardAndSensitivity(t *testing.T) {
	spec, registered := s6GuardSpecs["PIB-225"]
	if !registered || spec.run == nil || spec.sensitivity == nil || spec.fixture == "" {
		t.Fatal("PIB-225 has no registered S6 guard spec")
	}
	if err := spec.run(t); err != nil {
		t.Fatalf("PIB-225 guard rejected the shipped input: %v", err)
	}
	if err := spec.sensitivity(t); err == nil {
		t.Fatalf("PIB-225 sensitivity fixture %q was accepted by its own validator", spec.fixture)
	}
}

// TestS6RowPIB226GuardAndSensitivity is the acceptance target and the
// sensitivity fixture for PIB-226 — the closed-vocabulary order guard.
func TestS6RowPIB226GuardAndSensitivity(t *testing.T) {
	spec, registered := s6GuardSpecs["PIB-226"]
	if !registered || spec.run == nil || spec.sensitivity == nil || spec.fixture == "" {
		t.Fatal("PIB-226 has no registered S6 guard spec")
	}
	if err := spec.run(t); err != nil {
		t.Fatalf("PIB-226 guard rejected the shipped input: %v", err)
	}
	if err := spec.sensitivity(t); err == nil {
		t.Fatalf("PIB-226 sensitivity fixture %q was accepted by its own validator", spec.fixture)
	}
}

// TestS6RowPIB227GuardAndSensitivity is the acceptance target and the
// sensitivity fixture for PIB-227 — the advisory catalog totality guard.
func TestS6RowPIB227GuardAndSensitivity(t *testing.T) {
	spec, registered := s6GuardSpecs["PIB-227"]
	if !registered || spec.run == nil || spec.sensitivity == nil || spec.fixture == "" {
		t.Fatal("PIB-227 has no registered S6 guard spec")
	}
	if err := spec.run(t); err != nil {
		t.Fatalf("PIB-227 guard rejected the shipped input: %v", err)
	}
	if err := spec.sensitivity(t); err == nil {
		t.Fatalf("PIB-227 sensitivity fixture %q was accepted by its own validator", spec.fixture)
	}
}

// TestS6RowPIB228GuardAndSensitivity is the acceptance target and the
// sensitivity fixture for PIB-228 — the refusal catalog totality guard.
func TestS6RowPIB228GuardAndSensitivity(t *testing.T) {
	spec, registered := s6GuardSpecs["PIB-228"]
	if !registered || spec.run == nil || spec.sensitivity == nil || spec.fixture == "" {
		t.Fatal("PIB-228 has no registered S6 guard spec")
	}
	if err := spec.run(t); err != nil {
		t.Fatalf("PIB-228 guard rejected the shipped input: %v", err)
	}
	if err := spec.sensitivity(t); err == nil {
		t.Fatalf("PIB-228 sensitivity fixture %q was accepted by its own validator", spec.fixture)
	}
}

// TestS6RowPIB229GuardAndSensitivity is the acceptance target and the
// sensitivity fixture for PIB-229 — the prose PIB-citation totality guard.
func TestS6RowPIB229GuardAndSensitivity(t *testing.T) {
	spec, registered := s6GuardSpecs["PIB-229"]
	if !registered || spec.run == nil || spec.sensitivity == nil || spec.fixture == "" {
		t.Fatal("PIB-229 has no registered S6 guard spec")
	}
	if err := spec.run(t); err != nil {
		t.Fatalf("PIB-229 guard rejected the shipped input: %v", err)
	}
	if err := spec.sensitivity(t); err == nil {
		t.Fatalf("PIB-229 sensitivity fixture %q was accepted by its own validator", spec.fixture)
	}
}

// TestS6RowPIB230GuardAndSensitivity is the acceptance target and the
// sensitivity fixture for PIB-230 — the AST acceptance-ledger resolver guard.
func TestS6RowPIB230GuardAndSensitivity(t *testing.T) {
	spec, registered := s6GuardSpecs["PIB-230"]
	if !registered || spec.run == nil || spec.sensitivity == nil || spec.fixture == "" {
		t.Fatal("PIB-230 has no registered S6 guard spec")
	}
	if err := spec.run(t); err != nil {
		t.Fatalf("PIB-230 guard rejected the shipped input: %v", err)
	}
	if err := spec.sensitivity(t); err == nil {
		t.Fatalf("PIB-230 sensitivity fixture %q was accepted by its own validator", spec.fixture)
	}
}

// TestS6RowPIB231GuardAndSensitivity is the acceptance target and the
// sensitivity fixture for PIB-231 — the Kind-column sensitivity meta guard.
func TestS6RowPIB231GuardAndSensitivity(t *testing.T) {
	spec, registered := s6GuardSpecs["PIB-231"]
	if !registered || spec.run == nil || spec.sensitivity == nil || spec.fixture == "" {
		t.Fatal("PIB-231 has no registered S6 guard spec")
	}
	if err := spec.run(t); err != nil {
		t.Fatalf("PIB-231 guard rejected the shipped input: %v", err)
	}
	if err := spec.sensitivity(t); err == nil {
		t.Fatalf("PIB-231 sensitivity fixture %q was accepted by its own validator", spec.fixture)
	}
}

// TestS6RowPIB232GuardAndSensitivity is the acceptance target and the
// sensitivity fixture for PIB-232 — the injection-seam nil/unassigned guard.
func TestS6RowPIB232GuardAndSensitivity(t *testing.T) {
	spec, registered := s6GuardSpecs["PIB-232"]
	if !registered || spec.run == nil || spec.sensitivity == nil || spec.fixture == "" {
		t.Fatal("PIB-232 has no registered S6 guard spec")
	}
	if err := spec.run(t); err != nil {
		t.Fatalf("PIB-232 guard rejected the shipped input: %v", err)
	}
	if err := spec.sensitivity(t); err == nil {
		t.Fatalf("PIB-232 sensitivity fixture %q was accepted by its own validator", spec.fixture)
	}
}

// TestS6RowPIB388GuardAndSensitivity is the acceptance target and the
// sensitivity fixture for PIB-388 — the redaction-message non-echo guard.
func TestS6RowPIB388GuardAndSensitivity(t *testing.T) {
	spec, registered := s6GuardSpecs["PIB-388"]
	if !registered || spec.run == nil || spec.sensitivity == nil || spec.fixture == "" {
		t.Fatal("PIB-388 has no registered S6 guard spec")
	}
	if err := spec.run(t); err != nil {
		t.Fatalf("PIB-388 guard rejected the shipped input: %v", err)
	}
	if err := spec.sensitivity(t); err == nil {
		t.Fatalf("PIB-388 sensitivity fixture %q was accepted by its own validator", spec.fixture)
	}
}

// TestS6RowPIB389GuardAndSensitivity is the acceptance target and the
// sensitivity fixture for PIB-389 — the archive-as-history/undo claim guard.
func TestS6RowPIB389GuardAndSensitivity(t *testing.T) {
	spec, registered := s6GuardSpecs["PIB-389"]
	if !registered || spec.run == nil || spec.sensitivity == nil || spec.fixture == "" {
		t.Fatal("PIB-389 has no registered S6 guard spec")
	}
	if err := spec.run(t); err != nil {
		t.Fatalf("PIB-389 guard rejected the shipped input: %v", err)
	}
	if err := spec.sensitivity(t); err == nil {
		t.Fatalf("PIB-389 sensitivity fixture %q was accepted by its own validator", spec.fixture)
	}
}

// TestS6RowPIB390GuardAndSensitivity is the acceptance target and the
// sensitivity fixture for PIB-390 — the regenerate provider-requirement guard.
func TestS6RowPIB390GuardAndSensitivity(t *testing.T) {
	spec, registered := s6GuardSpecs["PIB-390"]
	if !registered || spec.run == nil || spec.sensitivity == nil || spec.fixture == "" {
		t.Fatal("PIB-390 has no registered S6 guard spec")
	}
	if err := spec.run(t); err != nil {
		t.Fatalf("PIB-390 guard rejected the shipped input: %v", err)
	}
	if err := spec.sensitivity(t); err == nil {
		t.Fatalf("PIB-390 sensitivity fixture %q was accepted by its own validator", spec.fixture)
	}
}

// TestS6RowPIB391GuardAndSensitivity is the acceptance target and the
// sensitivity fixture for PIB-391 — the frozen golden provenance wiring guard.
func TestS6RowPIB391GuardAndSensitivity(t *testing.T) {
	spec, registered := s6GuardSpecs["PIB-391"]
	if !registered || spec.run == nil || spec.sensitivity == nil || spec.fixture == "" {
		t.Fatal("PIB-391 has no registered S6 guard spec")
	}
	if err := spec.run(t); err != nil {
		t.Fatalf("PIB-391 guard rejected the shipped input: %v", err)
	}
	if err := spec.sensitivity(t); err == nil {
		t.Fatalf("PIB-391 sensitivity fixture %q was accepted by its own validator", spec.fixture)
	}
}

package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// GH #25 W2 (ADR-013 Amendment 2 D20) moves the verify report schema from
// 1.1 to 1.2. The frozen compat-verify golden predates the bump, so the
// final comparator applies this one independently expected delta after the
// S2/S4/S5 rows. The fixture's failure is not a blocked anchor-C ladder on a
// landed target, so no overlap_assessment object may appear.
const (
	ovGoldenSchemaOld = "stdout:\n{\n  \"schema_version\": \"1.1\",\n"
	ovGoldenSchemaNew = "stdout:\n{\n  \"schema_version\": \"1.2\",\n"
)

func ovExpectedVerifySchemaGolden(name string, previous []byte) ([]byte, error) {
	if name != "compat-verify.txt" {
		return previous, nil
	}
	old := string(previous)
	if strings.Count(old, ovGoldenSchemaOld) != 1 || strings.Count(old, `"schema_version": "1.`) != 1 ||
		strings.Contains(old, `"overlap_assessment"`) {
		return nil, fmt.Errorf("W2 verify fixture lost its single schema 1.1 report or already carries the bump")
	}
	return []byte(strings.Replace(old, ovGoldenSchemaOld, ovGoldenSchemaNew, 1)), nil
}

// GH #25 A1 (ADR-013 Amendment 3 D32) moves the schema from 1.2 to 1.3.
// The fixture is a forward-mode report, so nothing else in it may change.
const a1GoldenSchemaNew = "stdout:\n{\n  \"schema_version\": \"1.3\",\n"

func a1ExpectedVerifySchemaGolden(name string, previous []byte) ([]byte, error) {
	if name != "compat-verify.txt" {
		return previous, nil
	}
	old := string(previous)
	if strings.Count(old, ovGoldenSchemaNew) != 1 || strings.Count(old, `"schema_version": "1.`) != 1 ||
		strings.Contains(old, `"overlap_assessment"`) {
		return nil, fmt.Errorf("A1 verify fixture lost its single schema 1.2 report or already carries the bump")
	}
	return []byte(strings.Replace(old, ovGoldenSchemaNew, a1GoldenSchemaNew, 1)), nil
}

// verifySchemaGoldenChain applies every verify schema bump after the
// S2/S4/S5 rows, in order.
func verifySchemaGoldenChain(name string, previous []byte) ([]byte, error) {
	out, err := ovExpectedVerifySchemaGolden(name, previous)
	if err != nil {
		return nil, err
	}
	return a1ExpectedVerifySchemaGolden(name, out)
}

func ovGoldenBeforeSchemaBumps(t *testing.T, name string) []byte {
	t.Helper()
	frozen, err := os.ReadFile(filepath.Join(preparePIBGoldenDir, name))
	if err != nil {
		t.Fatal(err)
	}
	previous := frozen
	for _, step := range []func(string, []byte) ([]byte, error){
		rgaS2ExpectedProducerGolden, rgaS4ExpectedPublicationGolden,
		rgaS5ExpectedCaptureEventGolden, rgaS5ExpectedVerifyGolden,
	} {
		if previous, err = step(name, previous); err != nil {
			t.Fatal(err)
		}
	}
	return previous
}

func TestA1VerifyGoldenSchemaDeltaAndSensitivities(t *testing.T) {
	const name = "compat-verify.txt"
	w2, err := ovExpectedVerifySchemaGolden(name, ovGoldenBeforeSchemaBumps(t, name))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := a1ExpectedVerifySchemaGolden(name, w2)
	if err != nil || preparePIBGoldenDelta(name, string(expected)) != nil {
		t.Fatalf("exact schema 1.3 bump was rejected: %v", err)
	}
	current := string(expected)
	for label, wrong := range map[string]string{
		"missing-bump":  string(w2),
		"wrong-version": strings.Replace(current, `"schema_version": "1.3"`, `"schema_version": "1.4"`, 1),
	} {
		t.Run(label, func(t *testing.T) {
			if wrong == current || preparePIBGoldenDelta(name, wrong) == nil {
				t.Fatal("same final comparator accepted the wrong verify report")
			}
		})
	}
	if _, err := a1ExpectedVerifySchemaGolden(name, expected); err == nil {
		t.Fatal("schema adapter accepted an already-applied bump")
	}
	unaffected := []byte("stdout:\n{\n  \"schema_version\": \"1.2\",\n}\n")
	if got, err := a1ExpectedVerifySchemaGolden("compat-record.txt", unaffected); err != nil || !bytes.Equal(got, unaffected) {
		t.Fatal("schema adapter changed a non-verify fixture")
	}
}

func TestOVVerifyGoldenSchemaDeltaAndSensitivities(t *testing.T) {
	const name = "compat-verify.txt"
	frozen, err := os.ReadFile(filepath.Join(preparePIBGoldenDir, name))
	if err != nil {
		t.Fatal(err)
	}
	previous := frozen
	for _, step := range []func(string, []byte) ([]byte, error){
		rgaS2ExpectedProducerGolden, rgaS4ExpectedPublicationGolden,
		rgaS5ExpectedCaptureEventGolden, rgaS5ExpectedVerifyGolden,
	} {
		if previous, err = step(name, previous); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := ovExpectedVerifySchemaGolden(name, previous)
	if err != nil {
		t.Fatalf("exact schema 1.2 bump was rejected: %v", err)
	}
	// A1's 1.3 bump applies on top; W2's sensitivities are judged against
	// the final expected report.
	final, err := a1ExpectedVerifySchemaGolden(name, expected)
	if err != nil || preparePIBGoldenDelta(name, string(final)) != nil {
		t.Fatalf("composed schema bumps were rejected: %v", err)
	}
	current := string(final)
	for label, wrong := range map[string]string{
		"missing-bump":   string(previous),
		"assessment-key": strings.Replace(current, a1GoldenSchemaNew, a1GoldenSchemaNew+"  \"overlap_assessment\": null,\n", 1),
	} {
		t.Run(label, func(t *testing.T) {
			if wrong == current || preparePIBGoldenDelta(name, wrong) == nil {
				t.Fatal("same final comparator accepted the wrong verify report")
			}
		})
	}
	if _, err := ovExpectedVerifySchemaGolden(name, expected); err == nil {
		t.Fatal("schema adapter accepted an already-applied bump")
	}
	if _, err := ovExpectedVerifySchemaGolden(name, []byte(strings.Replace(string(previous), ovGoldenSchemaOld, "stdout:\n{\n", 1))); err == nil {
		t.Fatal("schema adapter accepted a report without its schema line")
	}
	unaffected := []byte("stdout:\n{\n  \"schema_version\": \"1.1\",\n}\n")
	if got, err := ovExpectedVerifySchemaGolden("compat-record.txt", unaffected); err != nil || !bytes.Equal(got, unaffected) {
		t.Fatal("schema adapter changed a non-verify fixture")
	}
}

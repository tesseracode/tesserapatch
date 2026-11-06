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
	if err != nil || preparePIBGoldenDelta(name, string(expected)) != nil {
		t.Fatalf("exact schema 1.2 bump was rejected: %v", err)
	}
	current := string(expected)
	for label, wrong := range map[string]string{
		"missing-bump":   string(previous),
		"wrong-version":  strings.Replace(current, `"schema_version": "1.2"`, `"schema_version": "1.3"`, 1),
		"assessment-key": strings.Replace(current, ovGoldenSchemaNew, ovGoldenSchemaNew+"  \"overlap_assessment\": null,\n", 1),
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

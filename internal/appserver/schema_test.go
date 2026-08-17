package appserver

import (
	"os"
	"testing"
)

func TestValidateStableFixture(t *testing.T) {
	b, err := os.ReadFile("testdata/stable-schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateStableFixture(b); err != nil {
		t.Fatal(err)
	}
}
func TestValidateStableFixtureRejectsExperimental(t *testing.T) {
	if err := ValidateStableFixture([]byte(`{"cliVersion":"0.147.0","transport":"stdio-jsonl","initialize":"initialize","experimentalApi":true}`)); err == nil {
		t.Fatal("want error")
	}
}

package appserver

import (
	"os"
	"testing"
)

func TestParseCodexVersionAcceptsAnySemanticVersion(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    string
		wantErr bool
	}{
		{name: "reference version", output: "codex-cli 0.147.0\n", want: "0.147.0"},
		{name: "newer version", output: "codex-cli 0.149.0\n", want: "0.149.0"},
		{name: "future major version", output: "codex-cli 1.0.0", want: "1.0.0"},
		{name: "malformed output", output: "codex 0.149.0", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCodexVersion(tt.output)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseCodexVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("parseCodexVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

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

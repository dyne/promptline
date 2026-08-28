package appserver

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
		{name: "empty output", output: "", wantErr: true},
		{name: "trailing data", output: "codex-cli 0.149.0\nextra", wantErr: true},
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

func TestResolveExecutableValidatesAndRevalidatesIdentity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "codex")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho 'codex-cli 0.149.0'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	executable, err := ResolveExecutable(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = executable.Close() })
	if err := executable.Revalidate(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := executable.Revalidate(); err == nil {
		t.Fatal("non-executable replacement accepted")
	}
	if _, err := ResolveExecutable(dir); err == nil {
		t.Fatal("directory accepted as executable")
	}
	if _, err := ResolveExecutable(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing executable accepted")
	}
	notExecutable := filepath.Join(dir, "not-executable")
	if err := os.WriteFile(notExecutable, []byte("plain"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveExecutable(notExecutable); err == nil {
		t.Fatal("non-executable file accepted")
	}
}

func TestExecutableWithoutDescriptorFailsClosed(t *testing.T) {
	if (Executable{}).Path() != "" {
		t.Fatal("zero executable path")
	}
	if err := (Executable{}).Revalidate(); err == nil {
		t.Fatal("descriptor-less executable accepted")
	}
	if err := (Executable{}).Close(); err != nil {
		t.Fatal(err)
	}
}

func TestProbeAndRPCErrorContracts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho 'codex-cli 0.149.0'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	version, err := Probe(context.Background(), path)
	if err != nil || version != "0.149.0" {
		t.Fatalf("Probe() = %q, %v", version, err)
	}
	executable, err := ResolveExecutable(path)
	if err != nil {
		t.Fatal(err)
	}
	if executable.Path() == "" {
		t.Fatal("empty executable path")
	}
	if err := executable.Close(); err != nil {
		t.Fatal(err)
	}
	rpc := &RPCError{Code: -32001, Message: "overloaded"}
	if !errors.Is(rpc, ErrOverloaded) || rpc.Error() == "" {
		t.Fatal("RPC overload contract lost")
	}
	if errors.Is(rpc, ErrClosed) {
		t.Fatal("RPC error matched unrelated sentinel")
	}
}

func TestLimitsNormalizeEveryResourceBudget(t *testing.T) {
	limits := (Limits{}).normalized()
	if limits.MaxFrameBytes == 0 || limits.MaxPending == 0 || limits.MaxEvents == 0 || limits.MaxServerRequests == 0 || limits.MaxQueuedBytes == 0 || limits.MaxRepliedIDs == 0 {
		t.Fatalf("incomplete default limits: %+v", limits)
	}
	if got := (Limits{MaxFrameBytes: 9}).normalized(); got.MaxFrameBytes != 9 {
		t.Fatalf("explicit frame budget changed: %+v", got)
	}
}

func TestProbeRejectsMalformedAndExecutableCloseFailsAfterClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho malformed\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Probe(context.Background(), path); err == nil {
		t.Fatal("malformed version accepted")
	}
	executable, err := ResolveExecutable(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := executable.Close(); err != nil {
		t.Fatal(err)
	}
	if err := executable.Revalidate(); err == nil {
		t.Fatal("closed executable revalidated")
	}
	if _, err := ProbeExecutable(context.Background(), executable); err == nil {
		t.Fatal("closed executable was probed")
	}
}

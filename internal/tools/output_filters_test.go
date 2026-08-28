package tools

import (
	"context"
	"testing"
)

func TestOutputFilterSanitizesControlsANSIAndRunes(t *testing.T) {
	got, truncated := sanitizeToolOutputWithConfig("ok\x00\x1b[31mred\x1b[0m\n", OutputFilterConfig{MaxChars: 6, StripANSI: true, StripControl: true})
	if got != "okred\n" || truncated {
		t.Fatalf("filtered=%q truncated=%t", got, truncated)
	}
	got, truncated = sanitizeToolOutputWithConfig("αβγ", OutputFilterConfig{MaxChars: 2})
	if got != "αβ" || !truncated {
		t.Fatalf("rune truncation=%q %t", got, truncated)
	}
	if got := stripControlChars("a\x7fb\tc"); got != "ab\tc" {
		t.Fatalf("controls=%q", got)
	}
	if config := normalizeOutputFilterConfig(OutputFilterConfig{}); config.MaxChars != defaultMaxOutputChars {
		t.Fatal("default size")
	}
}

func TestBuiltinPathAndContextValidation(t *testing.T) {
	for _, path := range []string{"", "/absolute", "safe/file"} {
		err := validatePath(path)
		if path == "safe/file" && err != nil {
			t.Fatalf("safe path: %v", err)
		}
		if path != "safe/file" && err == nil {
			t.Fatalf("unsafe path accepted: %q", path)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := getCurrentDatetime(ctx, nil); err == nil {
		t.Fatal("cancelled clock accepted")
	}
	if value, err := getCurrentDatetime(context.Background(), nil); err != nil || value == "" {
		t.Fatalf("clock=%q %v", value, err)
	}
}

func TestFilterHiddenOutputKeepsVisiblePaths(t *testing.T) {
	root := t.TempDir()
	output := root + "/visible:\n" + root + "/.hidden:\nrelative/.secret\nplain\n\n"
	got := filterHiddenOutput(output, root)
	if got != root+"/visible:\nplain\n\n" {
		t.Fatalf("filtered listing=%q", got)
	}
	if !containsHiddenSegment(".hidden/file") || containsHiddenSegment("visible/file") {
		t.Fatal("hidden segment classification")
	}
}

func TestTruncateArgumentValidation(t *testing.T) {
	if err := validateTruncateArgs(map[string]interface{}{"path": "file", "size": 1}); err != nil {
		t.Fatal(err)
	}
	if err := validateTruncateArgs(map[string]interface{}{"size": 1}); err == nil {
		t.Fatal("missing path accepted")
	}
	if err := validateTruncateArgs(map[string]interface{}{"path": "file", "size": "bad"}); err == nil {
		t.Fatal("bad size accepted")
	}
}

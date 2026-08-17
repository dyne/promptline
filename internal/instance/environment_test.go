package instance

import (
	"reflect"
	"testing"
)

func TestChildEnvironmentPolicy(t *testing.T) {
	parent := []string{"LANG=C", "TERM=xterm", "OPENAI_API_KEY=secret", "DATABASE_URL=omit", "HTTP_PROXY=http://u:p@proxy"}
	overrides := map[string]string{"TERM": "screen", "ADMIN_FLAG": "enabled", "bad-key": "ignored"}
	got := ChildEnvironment(EnvironmentPolicy{Parent: parent, Overrides: overrides, Remove: []string{"OPENAI_API_KEY"}})
	want := []string{"ADMIN_FLAG=enabled", "HTTP_PROXY=http://u:p@proxy", "LANG=C", "TERM=screen"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("environment = %#v, want %#v", got, want)
	}
	if parent[1] != "TERM=xterm" || overrides["TERM"] != "screen" {
		t.Fatal("input environment mutated")
	}
}

func TestEnvironmentForChildUsesPrivateCodexHome(t *testing.T) {
	i := testInstance(t)
	got := i.EnvironmentForChild([]string{"CODEX_HOME=/shared", "LANG=C"}, map[string]string{}, nil)
	want := []string{"CODEX_HOME=" + i.CodexHome(), "LANG=C"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("environment = %#v, want %#v", got, want)
	}
}

func TestRedactEnvironmentValue(t *testing.T) {
	for _, tc := range []struct{ key, value, want string }{
		{"OPENAI_API_KEY", "sk-secret", "[REDACTED]"}, {"Authorization", "Bearer abc", "[REDACTED]"},
		{"COOKIE", "sid=abc", "[REDACTED]"}, {"HTTP_PROXY", "http://user:pass@proxy", "[REDACTED]"},
		{"LANG", "C.UTF-8", "C.UTF-8"}, {"TERM", "screen", "screen"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			if got := RedactEnvironmentValue(tc.key, tc.value); got != tc.want {
				t.Fatalf("got %q", got)
			}
		})
	}
}

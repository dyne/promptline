package runtime

import (
	"io"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"valid", []string{"--instance", "ops", "--cwd", ".", "--new"}, false},
		{"conflicting thread choices", []string{"--cwd", ".", "--new", "--resume", "x"}, true},
		{"unknown flag", []string{"--nope"}, true},
		{"missing cwd", []string{"--instance", "ops"}, true},
		{"version has no side effects", []string{"--version"}, false},
		{"toolbox serve", []string{"toolbox", "serve", "--cwd", "."}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.args, io.Discard)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

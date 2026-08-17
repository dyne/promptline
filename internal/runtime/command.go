package runtime

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"promptline/internal/instance"
)

type Command struct {
	Instance     instance.Config
	New          bool
	ResumeID     string
	Debug        bool
	Version      bool
	ToolboxServe bool
}

func Parse(args []string, stderr io.Writer) (Command, error) {
	toolboxServe := len(args) >= 2 && args[0] == "toolbox" && args[1] == "serve"
	if toolboxServe {
		args = args[2:]
	}
	fs := flag.NewFlagSet("promptline", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c Command
	fs.StringVar(&c.Instance.Name, "instance", "default", "named instance")
	fs.StringVar(&c.Instance.WorkingDirectory, "cwd", "", "working directory")
	fs.StringVar(&c.Instance.StateRoot, "state-root", "", "private state root")
	fs.StringVar(&c.Instance.CodexExecutable, "codex", "codex", "Codex executable")
	fs.StringVar(&c.Instance.Model, "model", "", "Codex model")
	fs.BoolVar(&c.New, "new", false, "start a new primary thread")
	fs.StringVar(&c.ResumeID, "resume", "", "resume this primary thread ID")
	fs.BoolVar(&c.Debug, "debug", false, "enable terminal diagnostics")
	fs.BoolVar(&c.Version, "version", false, "print version")
	if err := fs.Parse(args); err != nil {
		return Command{}, err
	}
	if fs.NArg() != 0 {
		return Command{}, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if c.New && c.ResumeID != "" {
		return Command{}, errors.New("--new and --resume are mutually exclusive")
	}
	if c.Version {
		return c, nil
	}
	if c.Instance.WorkingDirectory == "" {
		return Command{}, errors.New("--cwd is required")
	}
	cwd, err := filepath.Abs(c.Instance.WorkingDirectory)
	if err != nil {
		return Command{}, err
	}
	c.Instance.WorkingDirectory, c.Instance.WorkingRoot = cwd, cwd
	c.ToolboxServe = toolboxServe
	return c, nil
}

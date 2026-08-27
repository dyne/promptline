package runtime

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"promptline/internal/instance"
)

type Command struct {
	Instance     instance.Config
	New          bool
	ResumeID     string
	Debug        bool
	Version      bool
	ToolboxServe bool
	ListSkills   bool
	SkillFiles   string
	Materialize  string
	MockCodex    bool
}

func Parse(args []string, stderr io.Writer) (Command, error) {
	command, commandArgument, remaining, err := splitCommand(args)
	if err != nil {
		return Command{}, err
	}
	if command == "help" {
		remaining = append([]string{"--help"}, remaining...)
	}

	fs := flag.NewFlagSet("promptline", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { printUsage(fs, stderr) }
	var c Command
	fs.StringVar(&c.Instance.Name, "instance", "default", "named instance")
	fs.StringVar(&c.Instance.WorkingDirectory, "cwd", "", "working directory")
	fs.StringVar(&c.Instance.WorkingDirectory, "C", "", "working directory (alias for --cwd)")
	fs.StringVar(&c.Instance.WorkingDirectory, "cd", "", "working directory (alias for --cwd)")
	fs.StringVar(
		&c.Instance.StateRoot,
		"state-root",
		"",
		"private state root (default: ~/.promptline/instances; root: /var/lib/promptline/instances)",
	)
	fs.StringVar(&c.Instance.CodexExecutable, "codex", "codex", "Codex executable")
	mockCodexExecutable := ""
	fs.StringVar(&mockCodexExecutable, "mock-codex", "", "mock Codex executable (tests only)")
	fs.StringVar(&c.Instance.Model, "model", "", "Codex model (default: "+instance.DefaultModel+")")
	fs.StringVar(&c.Instance.Model, "m", "", "Codex model (alias for --model)")
	fs.BoolVar(&c.Instance.ToolboxEnabled, "toolbox", true, "enable the instance toolbox MCP server")
	approvalMode := string(instance.ApprovalDeny)
	fs.StringVar(&approvalMode, "approval", approvalMode, "approval mode: deny or ask")
	fs.StringVar(&approvalMode, "a", approvalMode, "approval mode (alias for --approval)")
	fs.BoolVar(&c.Debug, "debug", false, "enable terminal diagnostics")
	fs.BoolVar(&c.Version, "version", false, "print version")
	fs.BoolVar(&c.Version, "V", false, "print version (alias for --version)")
	if err := fs.Parse(remaining); err != nil {
		return Command{}, err
	}
	if fs.NArg() != 0 {
		return Command{}, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}

	c.New = command != "resume"
	if command == "resume" {
		c.ResumeID = commandArgument
	}
	c.ToolboxServe = command == "mcp-server"
	c.ListSkills = command == "list-skills"
	if command == "list-skill-files" {
		c.SkillFiles = commandArgument
	}
	if command == "materialize-skill" {
		c.Materialize = commandArgument
	}
	if (command == "list-skill-files" || command == "materialize-skill") && commandArgument == "" {
		return Command{}, fmt.Errorf("%s requires an argument", command)
	}
	if c.ToolboxServe && (command == "resume" || command == "new") {
		return Command{}, errors.New("mcp-server cannot be combined with a thread command")
	}
	if mockCodexExecutable != "" {
		if c.Instance.CodexExecutable != "codex" {
			return Command{}, errors.New("--codex and --mock-codex are mutually exclusive")
		}
		c.Instance.CodexExecutable = mockCodexExecutable
		c.MockCodex = true
	}
	c.Instance.ApprovalMode = instance.ApprovalMode(approvalMode)
	if c.Instance.ApprovalMode != instance.ApprovalDeny && c.Instance.ApprovalMode != instance.ApprovalAsk {
		return Command{}, fmt.Errorf("invalid approval mode %q", approvalMode)
	}
	if c.Version {
		return c, nil
	}
	if c.ListSkills || c.SkillFiles != "" || c.Materialize != "" {
		return c, nil
	}
	if c.Instance.WorkingDirectory == "" {
		if !c.ToolboxServe {
			return Command{}, errors.New("--cwd is required")
		}
		cwd, err := os.Getwd()
		if err != nil {
			return Command{}, fmt.Errorf("determine working directory: %w", err)
		}
		c.Instance.WorkingDirectory = cwd
	}
	cwd, err := filepath.Abs(c.Instance.WorkingDirectory)
	if err != nil {
		return Command{}, err
	}
	c.Instance.WorkingDirectory, c.Instance.WorkingRoot = cwd, cwd
	return c, nil
}

func splitCommand(args []string) (command, argument string, remaining []string, err error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", "", args, nil
	}
	switch args[0] {
	case "new", "mcp-server", "help", "list-skills":
		return args[0], "", args[1:], nil
	case "resume", "list-skill-files", "materialize-skill":
		remaining = args[1:]
		if len(remaining) > 0 && !strings.HasPrefix(remaining[0], "-") {
			argument = remaining[0]
			remaining = remaining[1:]
		}
		return args[0], argument, remaining, nil
	default:
		return "", "", nil, fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage(fs *flag.FlagSet, output io.Writer) {
	fmt.Fprintln(output, "Promptline CLI")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Usage: promptline [OPTIONS]")
	fmt.Fprintln(output, "       promptline new [OPTIONS]")
	fmt.Fprintln(output, "       promptline resume [THREAD_ID] [OPTIONS]")
	fmt.Fprintln(output, "       promptline mcp-server [OPTIONS]")
	fmt.Fprintln(output, "       promptline list-skills")
	fmt.Fprintln(output, "       promptline list-skill-files SKILL")
	fmt.Fprintln(output, "       promptline materialize-skill DIRECTORY")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Commands:")
	fmt.Fprintln(output, "  new         Start a new interactive thread (default)")
	fmt.Fprintln(output, "  resume      Resume THREAD_ID, or the last saved thread when omitted")
	fmt.Fprintln(output, "  mcp-server  Run only the u-root toolbox MCP server on stdio")
	fmt.Fprintln(output, "  list-skills List embedded skills")
	fmt.Fprintln(output, "  list-skill-files List public files in an embedded skill")
	fmt.Fprintln(output, "  materialize-skill Export embedded skills without overwriting")
	fmt.Fprintln(output, "  help        Print this help")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Options:")
	fs.PrintDefaults()
}

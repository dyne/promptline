// Package instance defines private, provider-neutral Promptline instances.
package instance

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	defaultRootStateDirectory = "/var/lib/promptline/instances"
	defaultUserStateDirectory = ".promptline/instances"
	DefaultModel              = "gpt-5.6-terra"
)

var instanceNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// ApprovalMode is the policy Promptline applies to app-server approval requests.
type ApprovalMode string

const (
	ApprovalAsk  ApprovalMode = "ask"
	ApprovalDeny ApprovalMode = "deny"
)

// Timeouts bounds child process operations. Zero values receive conservative defaults.
type Timeouts struct {
	Startup  time.Duration
	Shutdown time.Duration
}

// OutputCaps bounds child output retained by later runtime layers.
type OutputCaps struct {
	StdoutBytes int
	StderrBytes int
}

// Config is input to New. StateRoot defaults according to the effective user.
type Config struct {
	Name             string
	StateRoot        string
	WorkingRoot      string
	WorkingDirectory string
	CodexExecutable  string
	Model            string
	ReasoningEffort  string
	ApprovalMode     ApprovalMode
	ToolboxEnabled   bool
	Timeouts         Timeouts
	OutputCaps       OutputCaps
	StateRename      func(string, string) error
}

// Instance is immutable configuration and its private filesystem layout.
type Instance struct {
	name             string
	stateRoot        string
	stateDir         string
	workingRoot      string
	workingDirectory string
	codexExecutable  string
	codexHome        string
	model            string
	reasoningEffort  string
	approvalMode     ApprovalMode
	toolboxEnabled   bool
	timeouts         Timeouts
	outputCaps       OutputCaps
	stateRename      func(string, string) error
}

func (i *Instance) Name() string               { return i.name }
func (i *Instance) StateRoot() string          { return i.stateRoot }
func (i *Instance) StateDir() string           { return i.stateDir }
func (i *Instance) WorkingRoot() string        { return i.workingRoot }
func (i *Instance) WorkingDirectory() string   { return i.workingDirectory }
func (i *Instance) CodexExecutable() string    { return i.codexExecutable }
func (i *Instance) CodexHome() string          { return i.codexHome }
func (i *Instance) Model() string              { return i.model }
func (i *Instance) ReasoningEffort() string    { return i.reasoningEffort }
func (i *Instance) ApprovalMode() ApprovalMode { return i.approvalMode }
func (i *Instance) ToolboxEnabled() bool       { return i.toolboxEnabled }
func (i *Instance) Timeouts() Timeouts         { return i.timeouts }
func (i *Instance) OutputCaps() OutputCaps     { return i.outputCaps }

// New validates cfg and prepares an isolated private instance directory.
func New(cfg Config) (*Instance, error) {
	if !instanceNamePattern.MatchString(cfg.Name) {
		return nil, fmt.Errorf("invalid instance name %q", cfg.Name)
	}
	if strings.ContainsRune(cfg.Name, '\x00') {
		return nil, errors.New("instance name contains NUL")
	}
	homeDirectory := ""
	if cfg.StateRoot == "" && os.Geteuid() != 0 {
		resolvedHome, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home directory for state root: %w", err)
		}
		homeDirectory = resolvedHome
	}
	root, err := resolveStateRoot(os.Geteuid(), cfg.StateRoot, homeDirectory)
	if err != nil {
		return nil, err
	}
	if err := ensurePrivateDirectory(root); err != nil {
		return nil, fmt.Errorf("state root: %w", err)
	}
	stateDir := filepath.Join(root, cfg.Name)
	if filepath.Dir(stateDir) != root {
		return nil, errors.New("instance path escapes state root")
	}
	if err := ensurePrivateDirectory(stateDir); err != nil {
		return nil, fmt.Errorf("instance state directory: %w", err)
	}
	codexHome := filepath.Join(stateDir, "codex-home")
	if err := ensurePrivateDirectory(codexHome); err != nil {
		return nil, fmt.Errorf("CODEX_HOME: %w", err)
	}

	workingRoot, err := absoluteDirectory(cfg.WorkingRoot)
	if err != nil {
		return nil, fmt.Errorf("working root: %w", err)
	}
	workingDirectory := cfg.WorkingDirectory
	if workingDirectory == "" {
		workingDirectory = workingRoot
	}
	workingDirectory, err = absoluteDirectory(workingDirectory)
	if err != nil {
		return nil, fmt.Errorf("working directory: %w", err)
	}
	rel, err := filepath.Rel(workingRoot, workingDirectory)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, errors.New("working directory escapes working root")
	}

	mode := cfg.ApprovalMode
	if mode == "" {
		mode = ApprovalDeny
	}
	if mode != ApprovalAsk && mode != ApprovalDeny {
		return nil, fmt.Errorf("invalid approval mode %q", mode)
	}
	executable := cfg.CodexExecutable
	if executable == "" {
		executable = "codex"
	}
	model := cfg.Model
	if model == "" {
		model = DefaultModel
	}
	timeouts := cfg.Timeouts
	if timeouts.Startup <= 0 {
		timeouts.Startup = 15 * time.Second
	}
	if timeouts.Shutdown <= 0 {
		timeouts.Shutdown = 10 * time.Second
	}
	caps := cfg.OutputCaps
	if caps.StdoutBytes <= 0 {
		caps.StdoutBytes = 4 << 20
	}
	if caps.StderrBytes <= 0 {
		caps.StderrBytes = 1 << 20
	}
	renameState := cfg.StateRename
	if renameState == nil {
		renameState = os.Rename
	}
	return &Instance{name: cfg.Name, stateRoot: root, stateDir: stateDir, workingRoot: workingRoot,
		workingDirectory: workingDirectory, codexExecutable: executable, codexHome: codexHome,
		model: model, reasoningEffort: cfg.ReasoningEffort, approvalMode: mode,
		toolboxEnabled: cfg.ToolboxEnabled,
		timeouts:       timeouts, outputCaps: caps, stateRename: renameState}, nil
}

func resolveStateRoot(euid int, supplied, homeDirectory string) (string, error) {
	if supplied == "" {
		if euid == 0 {
			supplied = defaultRootStateDirectory
		} else {
			if homeDirectory == "" {
				return "", errors.New("home directory is required for the default state root")
			}
			supplied = filepath.Join(homeDirectory, defaultUserStateDirectory)
		}
	}
	if !filepath.IsAbs(supplied) {
		return "", errors.New("state root must be absolute")
	}
	return filepath.Clean(supplied), nil
}

func absoluteDirectory(path string) (string, error) {
	if path == "" {
		return "", errors.New("is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("is not a directory")
	}
	return filepath.Clean(abs), nil
}

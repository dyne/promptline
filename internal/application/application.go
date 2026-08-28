// Package application owns Promptline resource composition and cleanup.
package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"promptline/internal/appserver"
	"promptline/internal/governance"
	"promptline/internal/instance"
	"promptline/internal/mcp"
	pruntime "promptline/internal/runtime"
	"promptline/internal/tools"
)

// Factories are the small construction seams used by composition tests and
// embedders. Nil functions receive the production implementation.
type Factories struct {
	NewInstance  func(instance.Config) (*instance.Instance, error)
	Toolbox      func(string, string) (*tools.Registry, error)
	AcquireLock  func(*instance.Instance) (*instance.Lock, error)
	StartProcess func(context.Context, *instance.Instance) (pruntime.Process, ReplyClient, error)
	NewClient    func(ReplyClient) (ReplyClient, error)
	OpenJournal  func(governance.JournalConfig) (*governance.Journal, error)
}

func (f Factories) newInstance(config instance.Config) (*instance.Instance, error) {
	if f.NewInstance != nil {
		return f.NewInstance(config)
	}
	return instance.New(config)
}

type ReplyClient interface {
	pruntime.Client
	ReplyRequest(context.Context, uint64, any) error
}

// cleanupStack makes partial construction failures auditable: every registered
// resource is closed once, in reverse acquisition order.
type cleanupStack struct{ closers []func() error }

func (s *cleanupStack) add(close func() error) { s.closers = append(s.closers, close) }
func (s *cleanupStack) close() error {
	var errs []error
	for i := len(s.closers) - 1; i >= 0; i-- {
		if err := s.closers[i](); err != nil {
			errs = append(errs, err)
		}
	}
	s.closers = nil
	return errors.Join(errs...)
}

func (f Factories) toolbox(workingDirectory, workingRoot string) (*tools.Registry, error) {
	if f.Toolbox != nil {
		return f.Toolbox(workingDirectory, workingRoot)
	}
	return Toolbox(workingDirectory, workingRoot)
}

func (f Factories) acquireLock(in *instance.Instance) (*instance.Lock, error) {
	if f.AcquireLock != nil {
		return f.AcquireLock(in)
	}
	return in.AcquireLock()
}
func (f Factories) startProcess(ctx context.Context, in *instance.Instance) (pruntime.Process, ReplyClient, error) {
	if f.StartProcess != nil {
		return f.StartProcess(ctx, in)
	}
	process, err := appserver.Start(ctx, in)
	if err != nil {
		return nil, nil, err
	}
	return process, pruntime.AppServer{API: appserver.NewAPI(process.Client), Client: process.Client}, nil
}
func (f Factories) client(client ReplyClient) (ReplyClient, error) {
	if f.NewClient != nil {
		return f.NewClient(client)
	}
	return client, nil
}
func (f Factories) openJournal(config governance.JournalConfig) (*governance.Journal, error) {
	if f.OpenJournal != nil {
		return f.OpenJournal(config)
	}
	return governance.OpenJournal(config)
}

// Run assembles one command invocation. Resources are acquired in dependency
// order and released by their immediate owner on every construction failure.
func Run(ctx context.Context, cmd pruntime.Command, input io.Reader, output io.Writer, version string) error {
	return RunWithFactories(ctx, cmd, input, output, version, Factories{})
}

// RunWithFactories composes a command invocation with explicitly injectable
// construction seams. Production callers use Run.
func RunWithFactories(ctx context.Context, cmd pruntime.Command, input io.Reader, output io.Writer, version string, factories Factories) error {
	restoreUmask := instance.SetPrivateUmaskBeforeConcurrency()
	defer restoreUmask()
	if cmd.ToolboxServe {
		registry, err := factories.toolbox(cmd.Instance.WorkingDirectory, cmd.Instance.WorkingRoot)
		if err != nil {
			return err
		}
		defer registry.Close()
		server, err := mcp.NewServer(registry, input, output, 4<<20)
		if err != nil {
			return err
		}
		return server.Serve(ctx)
	}
	in, err := factories.newInstance(cmd.Instance)
	if err != nil {
		return err
	}
	lock, err := factories.acquireLock(in)
	if err != nil {
		return err
	}
	locked := true
	defer func() {
		if locked {
			_ = lock.Close()
		}
	}()

	var dynamicTools []appserver.DynamicToolNamespace
	if in.ToolboxEnabled() {
		registry, err := factories.toolbox(in.WorkingDirectory(), in.WorkingRoot())
		if err != nil {
			return fmt.Errorf("describe toolbox tools: %w", err)
		}
		dynamicTools = []appserver.DynamicToolNamespace{mcp.DynamicToolbox(registry)}
		_ = registry.Close()
		executable, err := os.Executable()
		if err != nil {
			return fmt.Errorf("resolve Promptline executable: %w", err)
		}
		executable, err = filepath.EvalSymlinks(executable)
		if err != nil {
			return fmt.Errorf("resolve Promptline executable symlinks: %w", err)
		}
		if err := mcp.InstallCodexConfig(executable, in); err != nil {
			return err
		}
	}
	process, client, err := factories.startProcess(ctx, in)
	if err != nil {
		return err
	}
	client, err = factories.client(client)
	if err != nil {
		_ = process.Close(context.Background())
		return err
	}
	r, err := pruntime.New(in, client, process, lock)
	if err != nil {
		_ = process.Close(context.Background())
		return err
	}
	locked = false
	journal, err := factories.openJournal(governance.JournalConfig{Directory: filepath.Join(in.StateDir(), "audit")})
	if err != nil {
		_ = r.Close(context.Background())
		return fmt.Errorf("open audit journal: %w", err)
	}
	defer journal.Close()
	r.SetRequestHandler(func(requestCtx context.Context, request appserver.ServerRequest, approvalInput io.Reader) error {
		decision, decisionErr := governance.HandleServerRequest(requestCtx, governance.Policy{Roots: []string{in.WorkingRoot()}}, ApprovalPrompt(in.ApprovalMode(), approvalInput, output), journal, request)
		if decisionErr != nil {
			decision = map[string]string{"decision": string(governance.DecisionDecline)}
		}
		if err := client.ReplyRequest(requestCtx, request.ID, decision); err != nil {
			return err
		}
		return decisionErr
	})
	if err := r.Start(ctx, pruntime.Options{Resume: !cmd.New, ResumeID: cmd.ResumeID, DynamicTools: dynamicTools}, version); err != nil {
		_ = r.Close(context.Background())
		return err
	}
	return r.Run(ctx, input, pruntime.Terminal{Out: output})
}

// Toolbox is the shared catalog factory for standalone and dynamic-tool modes.
func Toolbox(workingDirectory, workingRoot string) (*tools.Registry, error) {
	toolConfig := tools.DefaultConfig()
	toolConfig.WorkingDirectory = workingDirectory
	toolConfig.Roots = []string{workingRoot}
	toolConfig.Policy = mcp.ReadOnlyToolPolicy()
	return tools.NewRegistryWithConfig(toolConfig)
}

func ApprovalPrompt(mode instance.ApprovalMode, input io.Reader, output io.Writer) governance.Prompt {
	if mode != instance.ApprovalAsk {
		return nil
	}
	return governance.TerminalPrompt{Input: input, Output: output}
}

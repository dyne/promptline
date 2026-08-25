package application

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"promptline/internal/governance"
	"promptline/internal/instance"
	pruntime "promptline/internal/runtime"
	"promptline/internal/tools"
)

type compositionProcess struct{ closed int }

func (p *compositionProcess) CodexVersion() string        { return "" }
func (p *compositionProcess) Close(context.Context) error { p.closed++; return nil }

type compositionClient struct{ pruntime.Client }

func (compositionClient) ReplyRequest(context.Context, uint64, any) error { return nil }
func (compositionClient) Unsubscribe(context.Context, string) error       { return nil }

func TestRunWithFactoriesUsesSharedToolboxFactoryForStandaloneAndInteractive(t *testing.T) {
	var calls [][2]string
	factories := Factories{Toolbox: func(directory, root string) (*tools.Registry, error) {
		calls = append(calls, [2]string{directory, root})
		return nil, errors.New("injected toolbox failure")
	}}
	standalone := pruntime.Command{ToolboxServe: true}
	workingDirectory := t.TempDir()
	standalone.Instance.WorkingDirectory, standalone.Instance.WorkingRoot = workingDirectory, workingDirectory
	if err := RunWithFactories(context.Background(), standalone, nil, io.Discard, "test", factories); err == nil {
		t.Fatal("standalone composition accepted a failed toolbox factory")
	}
	interactive := pruntime.Command{}
	interactive.Instance.Name, interactive.Instance.StateRoot = "test", t.TempDir()
	interactive.Instance.WorkingDirectory, interactive.Instance.WorkingRoot = workingDirectory, workingDirectory
	interactive.Instance.ToolboxEnabled = true
	if err := RunWithFactories(context.Background(), interactive, bytes.NewBuffer(nil), io.Discard, "test", factories); err == nil {
		t.Fatal("interactive composition accepted a failed toolbox factory")
	}
	if len(calls) != 2 || calls[0] != calls[1] {
		t.Fatalf("toolbox factory calls = %#v, want matching standalone and interactive catalog inputs", calls)
	}
}

func TestCleanupStackClosesReverseOrderExactlyOnce(t *testing.T) {
	var got []string
	stack := &cleanupStack{}
	for _, name := range []string{"lock", "process", "journal"} {
		name := name
		stack.add(func() error { got = append(got, name); return nil })
	}
	if err := stack.close(); err != nil {
		t.Fatal(err)
	}
	if err := stack.close(); err != nil {
		t.Fatal(err)
	}
	if want := "journal,process,lock"; strings.Join(got, ",") != want {
		t.Fatalf("cleanup = %v, want %s", got, want)
	}
}

func TestRunWithFactoriesPropagatesInstanceFailure(t *testing.T) {
	want := errors.New("instance")
	err := RunWithFactories(context.Background(), pruntime.Command{}, nil, io.Discard, "test", Factories{NewInstance: func(instance.Config) (*instance.Instance, error) { return nil, want }})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestRunWithFactoriesClosesProcessOnLaterFailure(t *testing.T) {
	for _, journal := range []bool{false, true} {
		t.Run("failure", func(t *testing.T) {
			p := &compositionProcess{}
			cmd := pruntime.Command{}
			d := t.TempDir()
			cmd.Instance = instance.Config{Name: "test", StateRoot: d, WorkingDirectory: d, WorkingRoot: d}
			f := Factories{StartProcess: func(context.Context, *instance.Instance) (pruntime.Process, ReplyClient, error) {
				return p, compositionClient{}, nil
			}}
			if journal {
				f.OpenJournal = func(governance.JournalConfig) (*governance.Journal, error) { return nil, errors.New("journal") }
			} else {
				f.NewClient = func(ReplyClient) (ReplyClient, error) { return nil, errors.New("client") }
			}
			if err := RunWithFactories(context.Background(), cmd, nil, io.Discard, "test", f); err == nil || p.closed != 1 {
				t.Fatalf("err=%v closes=%d", err, p.closed)
			}
		})
	}
}

//go:build unix

package instance

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestSaveStatePropagatesInjectedRenameFault(t *testing.T) {
	fault := errors.New("rename fault")
	i, err := New(Config{Name: "fault", StateRoot: t.TempDir(), WorkingRoot: t.TempDir(), StateRename: func(string, string) error { return fault }})
	if err != nil {
		t.Fatal(err)
	}
	if err := i.SaveState(State{}); !errors.Is(err, fault) {
		t.Fatalf("SaveState error = %v", err)
	}
}

func testInstance(t *testing.T) *Instance {
	t.Helper()
	i, err := New(Config{Name: "instance", StateRoot: t.TempDir(), WorkingRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	return i
}

func TestLockExclusiveAndReleased(t *testing.T) {
	i := testInstance(t)
	first, err := i.AcquireLock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := i.AcquireLock(); !errors.Is(err, ErrInstanceLocked) {
		t.Fatalf("second lock error = %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := i.AcquireLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestStaleLockArtifactDoesNotLock(t *testing.T) {
	i := testInstance(t)
	if err := os.WriteFile(i.lockPath(), []byte("stale"), privateFileMode); err != nil {
		t.Fatal(err)
	}
	lock, err := i.AcquireLock()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
}

func TestStateAtomicReplacementAndValidation(t *testing.T) {
	i := testInstance(t)
	if err := i.SaveState(State{LastPrimaryThreadID: "one", CodexVersion: "0.147.0"}); err != nil {
		t.Fatal(err)
	}
	if err := i.SaveState(State{LastPrimaryThreadID: "two"}); err != nil {
		t.Fatal(err)
	}
	state, err := i.LoadState()
	if err != nil || state.LastPrimaryThreadID != "two" {
		t.Fatalf("state = %+v, %v", state, err)
	}
	info, err := os.Stat(i.statePath())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != privateFileMode {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	if err := os.WriteFile(i.statePath(), []byte(`{"schemaVersion":`), privateFileMode); err != nil {
		t.Fatal(err)
	}
	if _, err := i.LoadState(); err == nil {
		t.Fatal("truncated state accepted")
	}
	if err := os.WriteFile(i.statePath(), []byte(`{"schemaVersion":2}`), privateFileMode); err != nil {
		t.Fatal(err)
	}
	if _, err := i.LoadState(); err == nil {
		t.Fatal("forward schema accepted")
	}
	if matches, err := filepath.Glob(filepath.Join(i.StateDir(), ".state-*.tmp")); err != nil || len(matches) != 0 {
		t.Fatalf("temporary artifacts = %v, %v", matches, err)
	}
}

func TestStateSaveFailurePreservesPreviousStateAndCleansTemporaryFile(t *testing.T) {
	i := testInstance(t)
	if err := i.SaveState(State{LastPrimaryThreadID: "previous"}); err != nil {
		t.Fatal(err)
	}
	fault := errors.New("rename fault")
	i.stateRename = func(string, string) error { return fault }
	if err := i.SaveState(State{LastPrimaryThreadID: "replacement"}); !errors.Is(err, fault) {
		t.Fatalf("SaveState error = %v, want rename fault", err)
	}
	state, err := i.LoadState()
	if err != nil || state.LastPrimaryThreadID != "previous" {
		t.Fatalf("state after failed save = %+v, %v", state, err)
	}
	matches, err := filepath.Glob(filepath.Join(i.StateDir(), ".state-*.tmp"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary artifacts = %v, %v", matches, err)
	}
}

func TestLoadStateRejectsCorruptionAndOversizedRecords(t *testing.T) {
	i := testInstance(t)
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{name: "zero schema", data: []byte(`{"schemaVersion":0}`)},
		{name: "unknown schema", data: []byte(`{"schemaVersion":99}`)},
		{name: "truncated json", data: []byte(`{"schemaVersion":`)},
		{name: "oversized", data: append([]byte(`{"schemaVersion":1,"lastPrimaryThreadId":"`), append(make([]byte, 1<<20), []byte(`"}`)...)...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(i.statePath(), tc.data, privateFileMode); err != nil {
				t.Fatal(err)
			}
			if _, err := i.LoadState(); err == nil {
				t.Fatal("LoadState unexpectedly accepted corrupt state")
			}
		})
	}
}

func TestLockCloseIsIdempotentAndReleasesForReacquisition(t *testing.T) {
	i := testInstance(t)
	lock, err := i.AcquireLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		t.Fatalf("second Close() = %v", err)
	}
	reacquired, err := i.AcquireLock()
	if err != nil {
		t.Fatalf("AcquireLock after Close = %v", err)
	}
	defer reacquired.Close()
}

func TestConcurrentLockRace(t *testing.T) {
	i := testInstance(t)
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan bool, 8)
	release := make(chan struct{})
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			lock, err := i.AcquireLock()
			results <- err == nil
			if err == nil {
				<-release
				defer lock.Close()
			}
		}()
	}
	close(start)
	acquired := 0
	for range 8 {
		if <-results {
			acquired++
		}
	}
	close(release)
	wg.Wait()
	if acquired != 1 {
		t.Fatalf("acquired %d locks, want 1", acquired)
	}
}

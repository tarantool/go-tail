// Copyright (c) 2026 FOSS contributors of https://github.com/tarantool/go-tail

package tail

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-tail/watch"
)

func stallBeforeRecheck(t *testing.T) {
	t.Helper()

	delayBeforeRecheck = watchDelay
	t.Cleanup(func() { delayBeforeRecheck = 0 })
}

// appendLine is an error-returning counterpart of appendFile for use
// outside of the test goroutine.
func appendLine(path, line string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(line)
	return err
}

// The file is replaced after the watch was armed on the old file but
// before the tailer compared the descriptor with the name. By the time
// the tailer switches to the new file the watcher is already dead: the
// rename made it drop the watch and quit, leaving a latched Deleted
// notification behind. The tailer used to keep that dead subscription
// and honour the stale notification, reopening the already-reopened
// file a second time and re-delivering everything read in between.
func TestReplaceWhileRecheckingThenAppend(t *testing.T) {
	eachWatcher(t, func(t *testing.T, poll bool) {
		stallBeforeRecheck(t)

		dir := t.TempDir()
		path := filepath.Join(dir, "rotated.log")
		writeFile(t, path, "before rotation\n")

		tailer := startTail(t, path, Config{Follow: true, ReOpen: true, Poll: poll})
		expectLine(t, tailer, "before rotation")

		// Give the tailer time to arm the watch and enter the stall,
		// then rotate inside the arm-to-recheck window.
		time.Sleep(windowSettle)
		require.NoError(t, os.Rename(path, path+".bak"))
		writeFile(t, path, "n1\n")
		expectLine(t, tailer, "n1")
		expectNoLine(t, tailer, dupSettle)

		appendFile(t, path, "n2\n")
		expectLine(t, tailer, "n2")
		expectNoLine(t, tailer, dupSettle)
	})
}

// Someone removes the watch out from under an armed tailer (Cleanup is
// documented as a process-exit helper, but nothing stops it from being
// called earlier). The watcher goroutine used to treat the closed events
// channel as a silent exit: it removed the already-removed watch again,
// corrupting the shared refcount, and told nobody, leaving the tailer
// blocked forever on a subscription that could not fire anymore. It must
// report the file as gone instead, so the tailer re-arms and keeps
// delivering lines. Only the inotify watcher shares state this way.
func TestExternalWatchRemovalDoesNotHang(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cleaned.log")
	writeFile(t, path, "seed\n")

	tailer := startTail(t, path, Config{Follow: true, ReOpen: true})
	expectLine(t, tailer, "seed")

	// Let the tailer arm the watch, then yank the watch away.
	time.Sleep(windowSettle)
	require.NoError(t, watch.Cleanup(path))

	appendFile(t, path, "n1\n")

	// The recovery reopen legitimately re-reads the file from the
	// start; the only requirement is that n1 arrives instead of the
	// tailer hanging forever.
	deadline := time.After(lineWait)
	for {
		select {
		case line, ok := <-tailer.Lines:
			require.True(t, ok, "Lines channel closed while waiting for %q", "n1")
			require.NoError(t, line.Err)
			if line.Text == "n1" {
				return
			}
		case <-deadline:
			t.Fatal("tailer hung after external watch removal")
		}
	}
}

// Rotation stress: lines are appended in small batches with a rotation
// after every batch, racing the tailer's EOF/arm/recheck cycle. Every
// line must be delivered exactly once, in order.
func TestRotationStressExactlyOnce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	writeFile(t, path, "seed\n")

	tailer := startTail(t, path, Config{Follow: true, ReOpen: true})
	expectLine(t, tailer, "seed")

	const (
		rotations = 20
		batch     = 5
	)

	var expected []string
	for r := 0; r < rotations; r++ {
		for l := 0; l < batch; l++ {
			expected = append(expected, fmt.Sprintf("r%02d-l%02d", r, l))
		}
	}
	expected = append(expected, "final")

	writeErr := make(chan error, 1)
	go func() {
		writeErr <- func() error {
			for r := 0; r < rotations; r++ {
				for l := 0; l < batch; l++ {
					if err := appendLine(path, fmt.Sprintf("r%02d-l%02d\n", r, l)); err != nil {
						return err
					}
					time.Sleep(5 * time.Millisecond)
				}
				if err := os.Rename(path, fmt.Sprintf("%s.%d", path, r)); err != nil {
					return err
				}
				if err := os.WriteFile(path, nil, 0o644); err != nil {
					return err
				}
			}
			return appendLine(path, "final\n")
		}()
	}()

	var got []string
	deadline := time.After(30 * time.Second)
loop:
	for {
		select {
		case line, ok := <-tailer.Lines:
			require.True(t, ok, "Lines channel closed mid-stress")
			require.NoError(t, line.Err)
			got = append(got, line.Text)
			if line.Text == "final" {
				break loop
			}
		case <-deadline:
			require.Failf(t, "stress timed out", "delivered %d lines so far, want %d",
				len(got), len(expected))
		}
	}

	require.NoError(t, <-writeErr)
	require.Equal(t, expected, got)
}

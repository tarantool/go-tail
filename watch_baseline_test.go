// Copyright (c) 2026 FOSS contributors of https://github.com/tarantool/go-tail

package tail

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// dupSettle is how long the tests wait for a reopen to settle or for a
// spurious duplicate line to show up.
const dupSettle = time.Second

// expectNoLine asserts that no line arrives on the tailer within d.
func expectNoLine(t *testing.T, tailer *Tail, d time.Duration) {
	t.Helper()

	select {
	case line, ok := <-tailer.Lines:
		require.True(t, ok, "Lines channel closed unexpectedly")
		require.Failf(t, "unexpected extra line", "got %q (err=%v)", line.Text, line.Err)
	case <-time.After(d):
	}
}

// eachWatcher runs the test against both watcher implementations.
func eachWatcher(t *testing.T, test func(t *testing.T, poll bool)) {
	t.Helper()

	for _, tc := range []struct {
		name string
		poll bool
	}{
		{name: "inotify", poll: false},
		{name: "polling", poll: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			test(t, tc.poll)
		})
	}
}

// The file is replaced while the watch is being armed, so the watch ends
// up on the file that took over the name. A later append to that file
// must not be mistaken for a truncation: the watcher used to seed its
// size baseline with the tailer's offset in the old file, and the
// smaller new file looked truncated on its first write, triggering a
// reopen and a re-read from offset zero that duplicated every line
// delivered from the new file so far.
func TestReplaceWhileArmingThenAppend(t *testing.T) {
	eachWatcher(t, func(t *testing.T, poll bool) {
		stallBeforeWatch(t)

		dir := t.TempDir()
		path := filepath.Join(dir, "rotated.log")
		writeFile(t, path, "a rather long line that moves the offset far ahead\n")

		tailer := startTail(t, path, Config{Follow: true, ReOpen: true, Poll: poll})
		expectLine(t, tailer, "a rather long line that moves the offset far ahead")

		time.Sleep(windowSettle)
		require.NoError(t, os.Rename(path, path+".bak"))
		writeFile(t, path, "n1\n")
		expectLine(t, tailer, "n1")

		// Let the reopen settle, then append: the append must produce
		// exactly one new line and no replay of the lines above.
		time.Sleep(dupSettle)
		appendFile(t, path, "n2\n")
		expectLine(t, tailer, "n2")
		expectNoLine(t, tailer, dupSettle)
	})
}

// Same stale-baseline scenario as above, but the file is truncated and
// rewritten in place instead of being replaced.
func TestTruncateWhileArmingThenAppend(t *testing.T) {
	eachWatcher(t, func(t *testing.T, poll bool) {
		stallBeforeWatch(t)

		path := filepath.Join(t.TempDir(), "truncated.log")
		writeFile(t, path, "a rather long line that moves the offset far ahead\n")

		tailer := startTail(t, path, Config{Follow: true, ReOpen: true, Poll: poll})
		expectLine(t, tailer, "a rather long line that moves the offset far ahead")

		time.Sleep(windowSettle)
		writeFile(t, path, "short\n")
		expectLine(t, tailer, "short")

		time.Sleep(dupSettle)
		appendFile(t, path, "next\n")
		expectLine(t, tailer, "next")
		expectNoLine(t, tailer, dupSettle)
	})
}

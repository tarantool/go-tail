// Copyright (c) 2026 FOSS contributors of https://github.com/tarantool/go-tail

package tail

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	watchDelay   = time.Second
	windowSettle = 200 * time.Millisecond
	lineWait     = 10 * time.Second
)

func stallBeforeWatch(t *testing.T) {
	t.Helper()

	delayBeforeWatch = watchDelay
	t.Cleanup(func() { delayBeforeWatch = 0 })
}

func startTail(t *testing.T, path string, config Config) *Tail {
	t.Helper()

	config.Logger = DiscardingLogger
	tailer, err := TailFile(path, config)
	if err != nil {
		t.Fatalf("failed to tail %s: %v", path, err)
	}
	t.Cleanup(func() { tailer.Stop() })

	return tailer
}

func expectLine(t *testing.T, tailer *Tail, expected string) {
	t.Helper()

	select {
	case line, ok := <-tailer.Lines:
		if !ok {
			t.Fatalf("Lines channel closed while waiting for %q", expected)
		}
		if line.Err != nil {
			t.Fatalf("error while waiting for %q: %v", expected, line.Err)
		}
		if line.Text != expected {
			t.Fatalf("got line %q, want %q", line.Text, expected)
		}
	case <-time.After(lineWait):
		t.Fatalf("timeout waiting for line %q", expected)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func appendFile(t *testing.T, path, content string) {
	t.Helper()

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("failed to open %s for append: %v", path, err)
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		t.Fatalf("failed to append to %s: %v", path, err)
	}
}

func TestAppendWhileWatchIsNotArmed(t *testing.T) {
	stallBeforeWatch(t)

	path := filepath.Join(t.TempDir(), "appended.log")
	writeFile(t, path, "first\n")

	tailer := startTail(t, path, Config{Follow: true, ReOpen: true})
	expectLine(t, tailer, "first")

	time.Sleep(windowSettle)
	appendFile(t, path, "second\nthird\n")

	expectLine(t, tailer, "second")
	expectLine(t, tailer, "third")
}

func TestTruncateWhileWatchIsNotArmed(t *testing.T) {
	stallBeforeWatch(t)

	path := filepath.Join(t.TempDir(), "truncated.log")
	writeFile(t, path, "a long first line\n")

	tailer := startTail(t, path, Config{Follow: true, ReOpen: true})
	expectLine(t, tailer, "a long first line")

	time.Sleep(windowSettle)
	writeFile(t, path, "short\n")

	expectLine(t, tailer, "short")
}

func TestReplaceWhileWatchIsNotArmed(t *testing.T) {
	stallBeforeWatch(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "rotated.log")
	writeFile(t, path, "before rotation\n")

	tailer := startTail(t, path, Config{Follow: true, ReOpen: true})
	expectLine(t, tailer, "before rotation")

	time.Sleep(windowSettle)
	appendFile(t, path, "written just before the rename\n")
	if err := os.Rename(path, path+".bak"); err != nil {
		t.Fatalf("failed to rotate %s: %v", path, err)
	}
	writeFile(t, path, "after rotation\n")

	expectLine(t, tailer, "written just before the rename")
	expectLine(t, tailer, "after rotation")
}

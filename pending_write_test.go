// Copyright (c) 2026 FOSS contributors of https://github.com/tarantool/go-tail

package tail

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/tarantool/go-tail/watch"
)

func TestDeleteWaitsForPendingWrites(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "pending-write-*.log")
	if err != nil {
		t.Fatalf("failed to create temporary log: %v", err)
	}
	defer file.Close()

	if _, err := file.WriteString("line written immediately before rename\n"); err != nil {
		t.Fatalf("failed to write pending line: %v", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("failed to seek temporary log: %v", err)
	}

	tailer := &Tail{
		Filename: file.Name(),
		Config: Config{
			Follow: true,
		},
		file:    file,
		reader:  bufio.NewReader(file),
		changes: watch.NewFileChanges(),
	}
	tailer.Logger = DiscardingLogger
	tailer.changes.NotifyDeleted()

	if err := tailer.waitForChanges(); err != nil {
		t.Fatalf("delete was handled before pending writes: %v", err)
	}

	line, err := tailer.readLine()
	if err != nil {
		t.Fatalf("failed to read pending line: %v", err)
	}
	if expected := "line written immediately before rename"; line != expected {
		t.Fatalf("unexpected pending line: got %q, want %q", line, expected)
	}

	if _, err := tailer.readLine(); !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF after pending line, got %v", err)
	}
	if err := tailer.waitForChanges(); !errors.Is(err, ErrStop) {
		t.Fatalf("delete was not handled after EOF: %v", err)
	}
}

// TestReOpenWaitsForPendingWrites checks the same drain-before-rename
// behaviour on the ReOpen path: the rotated-away file is read to EOF first
// and only then the recreated file is picked up.
func TestReOpenWaitsForPendingWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rotated.log")

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create temporary log: %v", err)
	}
	defer file.Close()

	if _, err := file.WriteString("line written immediately before rename\n"); err != nil {
		t.Fatalf("failed to write pending line: %v", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("failed to seek temporary log: %v", err)
	}

	tailer := &Tail{
		Filename: path,
		Config: Config{
			Follow: true,
			ReOpen: true,
		},
		file:    file,
		reader:  bufio.NewReader(file),
		changes: watch.NewFileChanges(),
	}
	tailer.Logger = DiscardingLogger

	// Rotate the file away and put a fresh one in its place, as a log
	// rotator would do, then notify the tailer about the rename.
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatalf("failed to rotate log: %v", err)
	}
	if err := os.WriteFile(path, []byte("line written after rename\n"), 0o600); err != nil {
		t.Fatalf("failed to create rotated log: %v", err)
	}
	tailer.changes.NotifyDeleted()

	if err := tailer.waitForChanges(); err != nil {
		t.Fatalf("rename was handled before pending writes: %v", err)
	}

	// Still reading the rotated-away file.
	line, err := tailer.readLine()
	if err != nil {
		t.Fatalf("failed to read pending line: %v", err)
	}
	if expected := "line written immediately before rename"; line != expected {
		t.Fatalf("unexpected pending line: got %q, want %q", line, expected)
	}
	if _, err := tailer.readLine(); !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF after pending line, got %v", err)
	}

	// EOF reached: the rename is handled now and the new file is reopened.
	if err := tailer.waitForChanges(); err != nil {
		t.Fatalf("failed to reopen after EOF: %v", err)
	}
	defer tailer.closeFile()

	line, err = tailer.readLine()
	if err != nil {
		t.Fatalf("failed to read line from reopened file: %v", err)
	}
	if expected := "line written after rename"; line != expected {
		t.Fatalf("unexpected line from reopened file: got %q, want %q", line, expected)
	}
}

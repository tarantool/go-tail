// Copyright (c) 2026 FOSS contributors of https://github.com/tarantool/go-tail
// Copyright (c) 2019 FOSS contributors of https://github.com/nxadm/tail
// Copyright (c) 2015 HPE Software Inc. All rights reserved.
// Copyright (c) 2013 ActiveState Software Inc. All rights reserved.

package watch

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/tarantool/go-tail/util"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/tomb.v1"
)

// InotifyFileWatcher uses inotify to monitor file changes.
type InotifyFileWatcher struct {
	Filename string
	Size     int64
	// inodeId record cur watch file id
	inodeId uint64
}

func NewInotifyFileWatcher(filename string) *InotifyFileWatcher {
	fw := &InotifyFileWatcher{filepath.Clean(filename), 0, 0}
	return fw
}

func (fw *InotifyFileWatcher) BlockUntilExists(t *tomb.Tomb) error {
	err := WatchCreate(fw.Filename)
	if err != nil {
		return err
	}
	defer RemoveWatchCreate(fw.Filename)

	// Do a real check now as the file might have been created before
	// calling `WatchFlags` above.
	if _, err = os.Stat(fw.Filename); !os.IsNotExist(err) {
		// file exists, or stat returned an error.
		return err
	}

	events := Events(fw.Filename)

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return fmt.Errorf("inotify watcher has been closed")
			}
			evtName, err := filepath.Abs(evt.Name)
			if err != nil {
				return err
			}
			fwFilename, err := filepath.Abs(fw.Filename)
			if err != nil {
				return err
			}
			if evtName == fwFilename {
				return nil
			}
		case <-t.Dying():
			return tomb.ErrDying
		}
	}
	panic("unreachable")
}

func (fw *InotifyFileWatcher) ChangeEvents(t *tomb.Tomb, pos int64) (*FileChanges, error) {
	err := Watch(fw.Filename)
	if err != nil {
		return nil, err
	}

	changes := NewFileChanges()

	// Seed the size baseline and the inode from the file that occupies
	// the name right now, not from the caller's offset: after a rotation
	// in the unwatched window the name already points at a different,
	// usually smaller file, and a stale baseline would make the first
	// write to it look like a truncation, causing a spurious reopen that
	// re-delivers already-sent lines. Changes that land before the watch
	// is armed are the caller's recheck's job to detect, not ours.
	fw.Size = 0
	fw.inodeId = 0
	var stat syscall.Stat_t
	if err := syscall.Stat(fw.Filename, &stat); err == nil {
		fw.Size = stat.Size
		fw.inodeId = stat.Ino
	}

	changes.hasProducer = true
	go func() {
		defer close(changes.stopped)

		events := Events(fw.Filename)

		for {
			prevSize := fw.Size

			var evt fsnotify.Event
			var ok bool

			select {
			case evt, ok = <-events:
				if !ok {
					// The events channel is closed by an external
					// RemoveWatch (e.g. Cleanup): the watch is gone
					// already and removing it again would corrupt
					// the shared refcount. Report the file as gone
					// so the consumer re-arms instead of blocking
					// forever on a subscription that cannot fire.
					changes.NotifyDeleted()
					return
				}
			case <-t.Dying():
				RemoveWatch(fw.Filename)
				return
			case <-changes.stop:
				RemoveWatch(fw.Filename)
				return
			}

			switch {
			case evt.Op&fsnotify.Remove == fsnotify.Remove:
				fallthrough

			case evt.Op&fsnotify.Rename == fsnotify.Rename:
				RemoveWatch(fw.Filename)
				changes.NotifyDeleted()
				return

			//With an open fd, unlink(fd) - inotify returns IN_ATTRIB (==fsnotify.Chmod)
			case evt.Op&fsnotify.Chmod == fsnotify.Chmod:
				fallthrough

			case evt.Op&fsnotify.Write == fsnotify.Write:
				fi, err := os.Stat(fw.Filename)
				if err != nil {
					if os.IsNotExist(err) {
						RemoveWatch(fw.Filename)
						changes.NotifyDeleted()
						return
					}
					// XXX: report this error back to the user
					util.Fatal("Failed to stat file %v: %v", fw.Filename, err)
				}
				fw.Size = fi.Size()

				// No matter what, it is necessary to notify a write event.
				changes.NotifyModified()

				if prevSize > 0 && prevSize > fw.Size {
					// File change; if file inodeId changed, notify delete event.
					if fw.inodeId > 0 && fi.Sys() != nil {
						if statT, fok := fi.Sys().(*syscall.Stat_t); fok {
							if statT.Ino != fw.inodeId {
								RemoveWatch(fw.Filename)
								changes.NotifyDeleted()
								return
							}
						}
					}
					changes.NotifyTruncated()
				}
			}
		}
	}()

	return changes, nil
}

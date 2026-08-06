// Copyright (c) 2026 FOSS contributors of https://github.com/tarantool/go-tail
// Copyright (c) 2019 FOSS contributors of https://github.com/nxadm/tail
// Copyright (c) 2015 HPE Software Inc. All rights reserved.
// Copyright (c) 2013 ActiveState Software Inc. All rights reserved.

package watch

import "gopkg.in/tomb.v1"

// FileWatcher monitors file-level events.
type FileWatcher interface {
	// BlockUntilExists blocks until the file comes into existence.
	BlockUntilExists(*tomb.Tomb) error

	// ChangeEvents reports on changes to a file, be it modification,
	// deletion, renames or truncations. The watcher takes its size
	// baseline from the file that occupies the name at arm time, so
	// changes that happened before the watch was armed are the
	// caller's job to detect. After a deletion event the producing
	// goroutine quits and the FileChanges must be discarded;
	// FileChanges.Stop releases the producer explicitly when the
	// caller wants to re-arm the watch.
	// The offset argument is unused and kept for compatibility.
	ChangeEvents(*tomb.Tomb, int64) (*FileChanges, error)
}

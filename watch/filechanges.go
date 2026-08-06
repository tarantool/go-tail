// Copyright (c) 2026 FOSS contributors of https://github.com/tarantool/go-tail
// Copyright (c) 2019 FOSS contributors of https://github.com/nxadm/tail
package watch

import "sync"

type FileChanges struct {
	Modified  chan bool // Channel to get notified of modifications
	Truncated chan bool // Channel to get notified of truncations
	Deleted   chan bool // Channel to get notified of deletions/renames

	// stop asks the producing watcher goroutine to quit; stopped is
	// closed by the producer once it has. Together they let a consumer
	// drop its subscription synchronously, so that a fresh watch can
	// be armed without the old producer competing for the same shared
	// events channel.
	stop        chan struct{}
	stopped     chan struct{}
	stopOnce    sync.Once
	hasProducer bool
}

func NewFileChanges() *FileChanges {
	return &FileChanges{
		Modified:  make(chan bool, 1),
		Truncated: make(chan bool, 1),
		Deleted:   make(chan bool, 1),
		stop:      make(chan struct{}),
		stopped:   make(chan struct{}),
	}
}

// Stop asks the producing watcher goroutine to quit and waits until it
// has done so. It is safe to call Stop multiple times and after the
// producer has already quit on its own. A FileChanges that never had a
// producer attached stops right away.
func (fc *FileChanges) Stop() {
	fc.stopOnce.Do(func() { close(fc.stop) })
	if fc.hasProducer {
		<-fc.stopped
	}
}

func (fc *FileChanges) NotifyModified() {
	sendOnlyIfEmpty(fc.Modified)
}

func (fc *FileChanges) NotifyTruncated() {
	sendOnlyIfEmpty(fc.Truncated)
}

func (fc *FileChanges) NotifyDeleted() {
	sendOnlyIfEmpty(fc.Deleted)
}

// sendOnlyIfEmpty sends on a bool channel only if the channel has no
// backlog to be read by other goroutines. This concurrency pattern
// can be used to notify other goroutines if and only if they are
// looking for it (i.e., subsequent notifications can be compressed
// into one).
func sendOnlyIfEmpty(ch chan bool) {
	select {
	case ch <- true:
	default:
	}
}

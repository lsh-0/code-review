package main

import (
	"context"
	"os"
	"time"

	"github.com/bep/debounce"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// how often the state file is polled for external modification. A poll (rather
// than fsnotify) keeps the watcher dependency-free and entirely backend-side,
// which suits watching a single known file.
const watchInterval = 500 * time.Millisecond

// coalesce a burst of external writes (an agent or CLI may write several times
// in quick succession) into one notification.
const watchDebounce = 200 * time.Millisecond

// how often the working tree is polled for uncommitted changes. As with the
// state-file poll, a poll (rather than fsnotify) keeps the watcher
// dependency-free; here it re-runs one cheap `git status --porcelain` per tick.
const worktreeWatchInterval = 1000 * time.Millisecond

// poll the state file for modifications by a writer other than this GUI. On a
// genuine external change the review state is reloaded and a `review:changed`
// event is emitted so the frontend can offer a refresh; the GUI's own writes,
// recorded via `markSaved`, are ignored. Runs until `ctx` is cancelled. The
// emitted event carries no payload — the frontend reacts by showing a banner,
// not by reading data from the event.
func (a *App) watchStateFile(ctx context.Context) {
	debounced := debounce.New(watchDebounce)
	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if a.stateChangedExternally() {
				debounced(func() {
					if err := a.ReloadReview(); err != nil {
						return
					}
					// adopt the external file's mtime as the new baseline so the
					// same change is not reported again on the next poll.
					a.markSaved()
					runtime.EventsEmit(a.ctx, "review:changed")
				})
			}
		}
	}
}

// poll the working tree for uncommitted changes so the banners appear the
// moment a tracked file changes on disk, without the reviewer triggering a
// refresh. Each tick re-runs `GetWorkingTreeStatus` and, when the reported
// status differs from the previous poll (per `WorkingTreeStatusEqual`), emits a
// `worktree:changed` event; the frontend reacts by re-fetching the status and
// re-rendering both banners. A steady dirty or clean tree emits nothing. A
// query error skips the tick, keeping the last-seen status as the baseline.
// Runs until `ctx` is cancelled, mirroring `watchStateFile`. The emitted event
// carries no payload — the frontend reads the fresh status over the bridge.
func (a *App) watchWorkingTree(ctx context.Context) {
	ticker := time.NewTicker(worktreeWatchInterval)
	defer ticker.Stop()

	previous, err := GetWorkingTreeStatus(a.repoPath)
	if err != nil {
		// leave the baseline empty; the first successful poll seeds it and only a
		// genuine later change emits.
		previous = WorkingTreeStatus{}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current, err := GetWorkingTreeStatus(a.repoPath)
			if err != nil {
				continue
			}
			if !WorkingTreeStatusEqual(previous, current) {
				previous = current
				runtime.EventsEmit(a.ctx, "worktree:changed")
			}
		}
	}
}

// report whether the state file's modification time is newer than this GUI's
// own last recorded write, i.e. another process has written it.
func (a *App) stateChangedExternally() bool {
	info, err := os.Stat(a.statePath)
	if err != nil {
		return false
	}

	a.savedMu.Lock()
	last := a.lastSavedMtime
	a.savedMu.Unlock()

	return info.ModTime().After(last)
}

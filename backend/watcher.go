package main

import (
	"context"
	"os"
	"time"

	"github.com/bep/debounce"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// how often every watcher polls its subject. A poll (rather than fsnotify) keeps
// the watchers dependency-free and entirely backend-side; for the branch ref it
// is also the more correct choice, since `git rev-parse` is the only thing that
// reliably knows whether a ref is loose, packed, or in another worktree's
// directory.
//
// One interval serves all three rather than tuning each to its subject, because
// a watcher only polls while nothing is being reported: once it has alerted it
// suspends until the reviewer clears the alert. A shorter interval therefore
// buys nothing across a session, only latency on the single quiet-to-alerted
// transition — and the reviewer is away for that one by definition.
const watchInterval = 5000 * time.Millisecond

// coalesce a burst of external writes (an agent or CLI may write several times
// in quick succession) into one notification.
const watchDebounce = 200 * time.Millisecond

// poll the state file for modifications by a writer other than this GUI. On a
// genuine external change the review state is reloaded and a `review:changed`
// event is emitted so the frontend can offer a refresh; the GUI's own writes,
// recorded via `markSaved`, are ignored. Emitting suspends this watcher until
// the reviewer clears the banner, so a change already reported is not reloaded
// and re-reported behind a prompt they have not yet acted on. Runs until `ctx`
// is cancelled. The emitted event carries no payload — the frontend reacts by
// showing a banner, not by reading data from the event.
func (a *App) watchStateFile(ctx context.Context) {
	debounced := debounce.New(watchDebounce)
	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// suspended means the banner is already up for this watcher: the
			// reviewer has been told and has not yet acted, so there is nothing
			// to add by reloading and re-reporting.
			if a.watchSuspended(watchNameState) {
				continue
			}
			if a.stateChangedExternally() {
				debounced(func() {
					if err := a.ReloadReview(); err != nil {
						return
					}
					// adopt the external file's mtime as the new baseline so the
					// same change is not reported again on the next poll.
					a.markSaved()
					a.SuspendWatch(watchNameState)
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
//
// While the banner is raised this watcher is suspended, which for it means
// reporting only the return to a clean tree rather than skipping the read
// outright: its alert has no dismiss control, so that return is what clears it.
// See `shouldReportWorktree`.
// Runs until `ctx` is cancelled, mirroring `watchStateFile`. The emitted event
// carries no payload — the frontend reads the fresh status over the bridge.
func (a *App) watchWorkingTree(ctx context.Context) {
	ticker := time.NewTicker(watchInterval)
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
			// unlike the other two, this watcher keeps reading while suspended:
			// its alert has no buttons, so the return to a clean tree is what
			// clears it and must still be noticed. `previous` advances either
			// way, so an absorbed dirty state does not fire as a stale
			// transition once the watcher resumes.
			report := shouldReportWorktree(a.watchSuspended(watchNameWorktree), previous, current)
			previous = current
			if report {
				runtime.EventsEmit(a.ctx, "worktree:changed")
			}
		}
	}
}

// whether a working-tree poll should report `current`.
//
// While the banner is up (`suspended`) only the return to a clean tree is news:
// further dirty states are absorbed, because the banner reports the bare
// condition "uncommitted changes exist" and no additional dirty file changes
// what it says. Unsuspended, any change in the reported status is news.
//
// A clean tree reports on every tick while suspended, rather than only on the
// dirty-to-clean edge. That repeat is deliberate: the frontend resumes this
// watcher as it hides the banner, so a repeat only happens when that resume did
// not arrive (its status fetch failed), and re-emitting retries until it does.
// Reporting the edge alone would instead leave the watcher suspended for good,
// blind to every later change.
func shouldReportWorktree(suspended bool, previous, current WorkingTreeStatus) bool {
	if suspended {
		return WorkingTreeStatusClean(current)
	}
	return !WorkingTreeStatusEqual(previous, current)
}

// the last commit SHA seen on a branch, and the rule for deciding whether a
// freshly read SHA is news. Kept as a value type so the rule is unit-testable
// without a ticker or a git repository, as `stateChangedExternally` is.
type commitBaseline struct {
	sha string
}

// whether `current` is a change worth reporting, adopting it as the new
// baseline either way.
//
// An empty baseline means "not yet seeded" and reports nothing. This is why the
// baseline is not simply compared from a zero value the way `watchWorkingTree`
// compares from a zero `WorkingTreeStatus`: an empty status is a meaningful
// value ("clean"), but an empty SHA is not, so comparing a real SHA against it
// would report a change and raise the banner at startup for no reason. The same
// property makes `reset` the mechanism for re-seeding a resumed watcher.
func (b *commitBaseline) observe(current string) bool {
	if b.sha == "" || b.sha == current {
		b.sha = current
		return false
	}
	b.sha = current
	return true
}

// forget the baseline, so the next observation re-seeds without reporting.
func (b *commitBaseline) reset() {
	b.sha = ""
}

// poll the source branch's HEAD commit so a commit made underneath the
// reviewer — by an agent, or by the reviewer in another terminal — raises the
// same refresh banner an external state edit does. Without it a new commit
// leaves the diff, file list, and marked files describing a revision that is no
// longer HEAD, with no prompt.
//
// This poller only detects: it neither reloads state nor recomputes the diff,
// leaving the view exactly as it was until the reviewer chooses to refresh. A
// failed read skips the tick, keeping the baseline, so a transient git error
// does not raise a false alert. Emitting suspends the watcher until the reviewer
// clears the banner. Runs until `ctx` is cancelled, mirroring its two
// neighbours; the emitted event carries no payload.
//
// The branch is passed in rather than read from `a.review` on each tick: a
// review's branches are fixed for the life of the app, whereas `a.review` itself
// is reassigned wholesale by `ReloadReview` from another goroutine, so reading
// through it here would be a data race for a value that never changes.
func (a *App) watchSourceBranch(ctx context.Context, branch string) {
	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if a.watchSuspended(watchNameCommits) {
				continue
			}
			current, err := CommitSHA(a.repoPath, branch)
			if err != nil {
				continue
			}
			if a.observeCommit(current) {
				a.SuspendWatch(watchNameCommits)
				runtime.EventsEmit(a.ctx, "commits:changed")
			}
		}
	}
}

// observe a freshly read commit SHA against the guarded baseline, reporting
// whether it is a change. Guarded because `ResumeWatch` resets the baseline from
// the frontend while this poller reads it.
func (a *App) observeCommit(current string) bool {
	a.watchMu.Lock()
	defer a.watchMu.Unlock()
	return a.commitSeen.observe(current)
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

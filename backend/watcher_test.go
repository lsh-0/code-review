package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"code-review/model"
)

func TestStateChangedExternally(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	app := &App{
		review:    model.NewReview("/repo", "feature", "main"),
		repoPath:  "/repo",
		statePath: statePath,
	}

	// a GUI-originated save records the mtime, so it must not look external.
	if err := app.persist(); err != nil {
		t.Fatalf("persist failed: %v", err)
	}
	if app.stateChangedExternally() {
		t.Error("expected GUI's own save to not be seen as external")
	}

	// an external write (another process) bumps the mtime past the recorded one.
	// advance the timestamp explicitly so the test does not depend on filesystem
	// mtime resolution.
	later := time.Now().Add(2 * time.Second)
	if err := os.WriteFile(statePath, []byte("{}"), 0644); err != nil {
		t.Fatalf("external write failed: %v", err)
	}
	if err := os.Chtimes(statePath, later, later); err != nil {
		t.Fatalf("chtimes failed: %v", err)
	}

	if !app.stateChangedExternally() {
		t.Error("expected an external write to be seen as external")
	}

	// after the watcher would reload and re-baseline, it is no longer external.
	app.markSaved()
	if app.stateChangedExternally() {
		t.Error("expected re-baselined state to not be seen as external")
	}
}

func TestCommitBaseline(t *testing.T) {
	var b commitBaseline

	// the first read seeds: there is no previous SHA to have moved from, so a
	// startup read must not raise the banner.
	if b.observe("aaa") {
		t.Error("expected the first read to seed without reporting")
	}
	if b.observe("aaa") {
		t.Error("expected an unchanged read to report nothing")
	}

	// a moved SHA is news exactly once; the baseline advances with it.
	if !b.observe("bbb") {
		t.Error("expected a differing read to report a change")
	}
	if b.observe("bbb") {
		t.Error("expected the same change to not be reported twice")
	}

	// resetting is how a resumed watcher re-baselines: the reviewer has either
	// absorbed or declined the change, so the next read must be silent.
	b.reset()
	if b.observe("ccc") {
		t.Error("expected a reset baseline to re-seed without reporting")
	}
	if !b.observe("ddd") {
		t.Error("expected a change after re-seeding to be reported")
	}
}

func TestShouldReportWorktree(t *testing.T) {
	clean := makeStatus(nil, nil)
	dirty := makeStatus([]string{"a.go"}, nil)
	dirtier := makeStatus([]string{"a.go", "b.go"}, nil)

	// unsuspended, any change in the reported status is news.
	if !shouldReportWorktree(false, clean, dirty) {
		t.Error("expected a tree going dirty to be reported")
	}
	if shouldReportWorktree(false, dirty, dirty) {
		t.Error("expected an unchanged status to report nothing")
	}

	// suspended, the banner is already up and says only that uncommitted
	// changes exist, so a further dirty file changes nothing it claims.
	if shouldReportWorktree(true, dirty, dirtier) {
		t.Error("expected a further dirty state to be absorbed while suspended")
	}

	// but the return to clean must still be noticed: it is what clears the
	// banner and resumes the watcher.
	if !shouldReportWorktree(true, dirtier, clean) {
		t.Error("expected the return to clean to be reported while suspended")
	}
}

func TestSuspendAndResumeWatch(t *testing.T) {
	app := &App{}

	if app.watchSuspended(watchNameCommits) {
		t.Error("expected a fresh app to have no watcher suspended")
	}

	app.SuspendWatch(watchNameCommits)
	if !app.watchSuspended(watchNameCommits) {
		t.Error("expected the commit watcher to be suspended")
	}
	// suspension is per watcher: one raised alert must not blind the others.
	if app.watchSuspended(watchNameState) || app.watchSuspended(watchNameWorktree) {
		t.Error("expected suspending one watcher to leave the others polling")
	}

	// a suspended commit watcher takes no reading, so its baseline is whatever
	// it was when it alerted; resuming must drop it so the cleared change is
	// not immediately re-reported.
	app.observeCommit("aaa")
	app.observeCommit("bbb")
	app.ResumeWatch(watchNameCommits)
	if app.watchSuspended(watchNameCommits) {
		t.Error("expected resume to clear the suspension")
	}
	if app.observeCommit("ccc") {
		t.Error("expected resume to re-seed the baseline rather than re-report")
	}
}

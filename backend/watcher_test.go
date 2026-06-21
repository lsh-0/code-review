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

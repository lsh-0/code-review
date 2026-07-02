package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenInPreferredApp(t *testing.T) {
	t.Run("composes target path and runs opener", func(t *testing.T) {
		// stand in a recorded path for the real opener: a script that writes
		// its argument to a file, letting the test assert the composed target.
		tmp_dir := t.TempDir()
		record := filepath.Join(tmp_dir, "opened.txt")
		script := filepath.Join(tmp_dir, "fake-opener")
		body := "#!/bin/sh\nprintf '%s' \"$1\" > " + record + "\n"
		if err := os.WriteFile(script, []byte(body), 0755); err != nil {
			t.Fatalf("failed to write fake opener: %v", err)
		}

		original := openerCommand
		openerCommand = script
		defer func() { openerCommand = original }()

		err := OpenInPreferredApp("/repo/root", "sub/file.go")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		actual, err := os.ReadFile(record)
		if err != nil {
			t.Fatalf("opener did not run: %v", err)
		}
		expected := filepath.Join("/repo/root", "sub/file.go")
		if string(actual) != expected {
			t.Errorf("expected target %q, got %q", expected, string(actual))
		}
	})

	t.Run("propagates error when opener is absent", func(t *testing.T) {
		original := openerCommand
		openerCommand = "/nonexistent/opener-binary"
		defer func() { openerCommand = original }()

		err := OpenInPreferredApp("/repo/root", "file.go")
		if err == nil {
			t.Error("expected error for missing opener, got nil")
		}
	})
}

func TestOpenDiffTool(t *testing.T) {
	t.Run("invokes git difftool with the repo, no-prompt, and path", func(t *testing.T) {
		// stand in a recorded command for `git`: a script that writes all its
		// arguments to a file, letting the test assert the difftool invocation.
		// The tool is launched detached, so the record is polled briefly.
		tmp_dir := t.TempDir()
		record := filepath.Join(tmp_dir, "args.txt")
		script := filepath.Join(tmp_dir, "fake-git")
		body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + record + "\n"
		if err := os.WriteFile(script, []byte(body), 0755); err != nil {
			t.Fatalf("failed to write fake git: %v", err)
		}

		original := diffToolCommand
		diffToolCommand = script
		defer func() { diffToolCommand = original }()

		if err := OpenDiffTool("/repo/root", "sub/file.go"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		actual := waitForFile(t, record)
		expected := "-C\n/repo/root\ndifftool\n--no-prompt\n--\nsub/file.go\n"
		if actual != expected {
			t.Errorf("expected args %q, got %q", expected, actual)
		}
	})

	t.Run("propagates error when git is absent", func(t *testing.T) {
		original := diffToolCommand
		diffToolCommand = "/nonexistent/git-binary"
		defer func() { diffToolCommand = original }()

		err := OpenDiffTool("/repo/root", "file.go")
		if err == nil {
			t.Error("expected error for missing git, got nil")
		}
	})
}

// poll for the detached tool's record file to appear and return its contents.
// The tool launched by `OpenDiffTool` runs asynchronously, so the file may not
// exist the instant the call returns; this waits up to a short deadline.
func waitForFile(t *testing.T, path string) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil {
			return string(data)
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("diff tool did not run: %s not written", path)
	return ""
}

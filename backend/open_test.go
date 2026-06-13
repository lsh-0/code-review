package main

import (
	"os"
	"path/filepath"
	"testing"
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

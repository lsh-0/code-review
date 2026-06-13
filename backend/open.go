package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// the opener command used to launch a file in its preferred application.
// Overridable in tests; defaults to the Linux `xdg-open`.
var openerCommand = "xdg-open"

// open a working-tree file in the OS-preferred application via `xdg-open`.
// `path` is relative to the repository root. Returns the exec error on
// failure (for example a missing opener or absent file).
func OpenInPreferredApp(repoPath, path string) error {
	target := filepath.Join(repoPath, path)
	cmd := exec.Command(openerCommand, target)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to open %s: %w", target, err)
	}
	return nil
}

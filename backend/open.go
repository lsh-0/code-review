package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// the opener command used to launch a file in its preferred application.
// Overridable in tests; defaults to the Linux `xdg-open`.
var openerCommand = "xdg-open"

// the command used to launch the reviewer's configured diff tool. Overridable
// in tests; defaults to `git`, whose `difftool` subcommand honours `diff.tool`.
var diffToolCommand = "git"

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

// open a single working-tree file in the reviewer's configured diff tool via
// `git difftool`, which honours their `diff.tool` (for example meld). `path` is
// relative to the repository root. `--no-prompt` skips the per-file
// confirmation; the tool diffs the working-tree change against the index for
// that one path.
//
// The tool is launched detached and not waited on: a GUI diff tool stays open
// for the duration of the review, so blocking on it would hang the calling
// bridge method. A failure to *start* (for example git absent) is returned; a
// failure that surfaces only once the detached tool runs is not observed here.
func OpenDiffTool(repoPath, path string) error {
	cmd := exec.Command(diffToolCommand, "-C", repoPath, "difftool", "--no-prompt", "--", path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch diff tool for %s: %w", path, err)
	}
	// release the process resources without blocking on the tool's lifetime.
	go func() { _ = cmd.Wait() }()
	return nil
}
